package app

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/D9veth/RBPO/internal/security"
	"github.com/jackc/pgx/v5"
)

type Pass struct {
	ID            string     `json:"id"`
	EventID       string     `json:"event_id"`
	HolderID      *string    `json:"holder_id"`
	HolderName    string     `json:"holder_name"`
	Status        string     `json:"status"`
	IssuedAt      time.Time  `json:"issued_at"`
	IssuedBy      string     `json:"issued_by"`
	RedeemedAt    *time.Time `json:"redeemed_at"`
	RedeemedBy    *string    `json:"redeemed_by"`
	RevokedAt     *time.Time `json:"revoked_at"`
	RevokeReason  *string    `json:"revoke_reason"`
	CodeHash      string     `json:"-"`
	RequestHash   string     `json:"-"`
	EventTitle    string     `json:"event_title"`
	EventLocation string     `json:"event_location"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        time.Time  `json:"ends_at"`
	EventStatus   string     `json:"event_status"`
	Color         string     `json:"color"`
}

const passColumns = `p.id,p.event_id,p.holder_id,p.holder_name,p.status,p.issued_at,p.issued_by,p.redeemed_at,p.redeemed_by,p.revoked_at,p.revoke_reason,p.code_hash,p.request_hash,e.title,e.location,e.starts_at,e.ends_at,e.status,e.color`

func scanPass(row pgx.Row) (Pass, error) {
	var p Pass
	err := row.Scan(&p.ID, &p.EventID, &p.HolderID, &p.HolderName, &p.Status, &p.IssuedAt, &p.IssuedBy, &p.RedeemedAt, &p.RedeemedBy, &p.RevokedAt, &p.RevokeReason, &p.CodeHash, &p.RequestHash, &p.EventTitle, &p.EventLocation, &p.StartsAt, &p.EndsAt, &p.EventStatus, &p.Color)
	return p, err
}
func (a *App) pass(r *http.Request, q queryer, id string) (Pass, error) {
	return scanPass(q.QueryRow(r.Context(), `SELECT `+passColumns+` FROM passes p JOIN events e ON e.id=p.event_id WHERE p.id=$1`, id))
}
func (a *App) passPayload(p Pass) map[string]any {
	return map[string]any{"pass": p, "code": security.PassCode(p.ID, a.Config.Secret), "qr_payload": "CAMPUS:" + security.PassCode(p.ID, a.Config.Secret)}
}
func key(r *http.Request) (string, error) {
	k := r.Header.Get("Idempotency-Key")
	if !security.ValidRequestKey(k) {
		return "", problem(400, "IDEMPOTENCY_REQUIRED", "Для операции нужен уникальный идентификатор запроса.")
	}
	return k, nil
}
func (a *App) issuePass(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	k, err := key(r)
	if err != nil {
		return err
	}
	if err = a.rate(r.Context(), "issue:"+u.ID, 60, time.Minute); err != nil {
		return err
	}
	var in struct {
		GuestName string `json:"guest_name"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	in.GuestName = strings.TrimSpace(in.GuestName)
	if in.GuestName != "" && (utf8.RuneCountInString(in.GuestName) < 2 || utf8.RuneCountInString(in.GuestName) > 120) {
		return problem(422, "VALIDATION", "Имя гостя: от 2 до 120 символов.")
	}
	eventID := r.PathValue("event")
	requestHash := security.Hash(eventID + "\x00" + in.GuestName)
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "issue:"+u.ID+":"+k); err != nil {
		return err
	}
	existing, err := scanPass(tx.QueryRow(r.Context(), `SELECT `+passColumns+` FROM passes p JOIN events e ON e.id=p.event_id WHERE p.issued_by=$1 AND p.request_id=$2`, u.ID, k))
	if err == nil {
		if existing.RequestHash != requestHash {
			return problem(409, "IDEMPOTENCY_CONFLICT", "Этот идентификатор уже использован для другой операции.")
		}
		if existing.HolderID == nil || *existing.HolderID != u.ID {
			e, err := a.event(r.Context(), tx, eventID, u.ID, "FOR SHARE OF e")
			if err != nil {
				return err
			}
			if err = permission(e, "organizer"); err != nil {
				return err
			}
		}
		write(w, 200, a.passPayload(existing))
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	e, err := a.event(r.Context(), tx, eventID, u.ID, "FOR UPDATE OF e")
	if err != nil {
		return err
	}
	if e.Status != "published" {
		return problem(409, "REGISTRATION_CLOSED", "Регистрация на это мероприятие недоступна.")
	}
	if !time.Now().Before(e.RegistrationEndsAt) || !time.Now().Before(e.EndsAt) {
		return problem(409, "REGISTRATION_CLOSED", "Регистрация уже закончилась.")
	}
	if e.Issued >= e.Capacity {
		return problem(409, "SOLD_OUT", "Свободных мест больше нет.")
	}
	var holderID *string
	name := in.GuestName
	if name == "" {
		holderID = &u.ID
		name = u.Name
		var exists bool
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM passes WHERE event_id=$1 AND holder_id=$2 AND status<>'revoked')`, e.ID, u.ID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return problem(409, "ALREADY_REGISTERED", "У вас уже есть пропуск на это мероприятие.")
		}
	} else {
		if err = permission(e, "organizer"); err != nil {
			return err
		}
	}
	id := security.ID()
	code, _ := security.NormalizeCode(security.PassCode(id, a.Config.Secret))
	_, err = tx.Exec(r.Context(), `INSERT INTO passes(id,event_id,holder_id,holder_name,code_hash,issued_by,request_id,request_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, e.ID, holderID, name, security.Hash(code), u.ID, k, requestHash)
	if err != nil {
		return err
	}
	if err = record(r.Context(), tx, e.ID, u.ID, "pass.issued", id, map[string]bool{"guest": holderID == nil}); err != nil {
		return err
	}
	p, err := a.pass(r, tx, id)
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 201, a.passPayload(p))
	return nil
}
func (a *App) getPass(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	p, err := a.pass(r, a.DB, r.PathValue("pass"))
	if err != nil {
		return err
	}
	if p.HolderID == nil || *p.HolderID != u.ID {
		e, err := a.event(r.Context(), a.DB, p.EventID, u.ID, "")
		if err != nil {
			return err
		}
		if e.Role != "organizer" {
			return problem(404, "NOT_FOUND", "Пропуск не найден.")
		}
	}
	write(w, 200, a.passPayload(p))
	return nil
}
func (a *App) myPasses(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	page := security.LimitInt(r.URL.Query().Get("page"), 1, 10000)
	var total int
	if err = a.DB.QueryRow(r.Context(), `SELECT count(*) FROM passes WHERE holder_id=$1`, u.ID).Scan(&total); err != nil {
		return err
	}
	rows, err := a.DB.Query(r.Context(), `SELECT `+passColumns+` FROM passes p JOIN events e ON e.id=p.event_id WHERE p.holder_id=$1 ORDER BY e.starts_at DESC,p.issued_at DESC LIMIT 24 OFFSET $2`, u.ID, (page-1)*24)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []Pass{}
	for rows.Next() {
		p, e := scanPass(rows)
		if e != nil {
			return e
		}
		items = append(items, p)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	write(w, 200, map[string]any{"items": items, "total": total, "page": page, "limit": 24})
	return nil
}
func (a *App) listEventPasses(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	e, err := a.event(r.Context(), a.DB, r.PathValue("event"), u.ID, "")
	if err != nil {
		return err
	}
	if err = permission(e, "organizer"); err != nil {
		return err
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 200 {
		return problem(422, "VALIDATION", "Слишком длинный запрос.")
	}
	status := r.URL.Query().Get("status")
	page := security.LimitInt(r.URL.Query().Get("page"), 1, 10000)
	var total int
	if err = a.DB.QueryRow(r.Context(), `SELECT count(*) FROM passes WHERE event_id=$1 AND ($2='' OR holder_name ILIKE '%'||$2||'%') AND ($3='' OR status=$3)`, e.ID, q, status).Scan(&total); err != nil {
		return err
	}
	rows, err := a.DB.Query(r.Context(), `SELECT `+passColumns+` FROM passes p JOIN events e ON e.id=p.event_id WHERE p.event_id=$1 AND ($2='' OR p.holder_name ILIKE '%'||$2||'%') AND ($3='' OR p.status=$3) ORDER BY p.issued_at DESC,p.id LIMIT 50 OFFSET $4`, e.ID, q, status, (page-1)*50)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []Pass{}
	for rows.Next() {
		p, e := scanPass(rows)
		if e != nil {
			return e
		}
		items = append(items, p)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	write(w, 200, map[string]any{"items": items, "total": total, "page": page, "limit": 50})
	return nil
}
func (a *App) revokePass(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if utf8.RuneCountInString(in.Reason) < 3 || utf8.RuneCountInString(in.Reason) > 240 {
		return problem(422, "VALIDATION", "Укажите причину отмены: от 3 до 240 символов.")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	p, err := a.pass(r, tx, r.PathValue("pass"))
	if err != nil {
		return err
	}
	e, err := a.event(r.Context(), tx, p.EventID, u.ID, "FOR UPDATE OF e")
	if err != nil {
		return err
	}
	self := p.HolderID != nil && *p.HolderID == u.ID
	if !self && e.Role != "organizer" {
		return problem(404, "NOT_FOUND", "Пропуск не найден.")
	}
	if self && e.Role != "organizer" && !time.Now().Before(e.DoorsAt) {
		return problem(409, "CANCELLATION_CLOSED", "После открытия входа отменить пропуск может организатор.")
	}
	p, err = a.pass(r, tx, p.ID)
	if err != nil {
		return err
	}
	if p.Status == "revoked" {
		write(w, 200, p)
		return nil
	}
	if p.Status == "redeemed" {
		return problem(409, "ALREADY_REDEEMED", "Погашенный пропуск нельзя отменить.")
	}
	if _, err = tx.Exec(r.Context(), `UPDATE passes SET status='revoked',revoked_at=now(),revoke_reason=$2 WHERE id=$1 AND status='active'`, p.ID, in.Reason); err != nil {
		return err
	}
	if err = record(r.Context(), tx, e.ID, u.ID, "pass.revoked", p.ID, map[string]string{"reason": in.Reason}); err != nil {
		return err
	}
	p, err = a.pass(r, tx, p.ID)
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, p)
	return nil
}
func csvCell(s string) string {
	if strings.ContainsAny(strings.TrimLeft(s, " \t\r\n")[:min(1, len(strings.TrimLeft(s, " \t\r\n")))], "=+-@") {
		return "'" + s
	}
	return s
}
func (a *App) exportPasses(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	e, err := a.event(r.Context(), a.DB, r.PathValue("event"), u.ID, "")
	if err != nil {
		return err
	}
	if err = permission(e, "organizer"); err != nil {
		return err
	}
	if err = a.rate(r.Context(), "export:"+u.ID, 10, time.Hour); err != nil {
		return err
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id,holder_name,status,issued_at,redeemed_at FROM passes WHERE event_id=$1 ORDER BY issued_at`, e.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	// Build a bounded export before sending headers, so failures stay readable.
	var buffer strings.Builder
	buffer.WriteString("\ufeff")
	writer := csv.NewWriter(&buffer)
	writer.Comma = ';'
	_ = writer.Write([]string{"ID", "Участник", "Статус", "Выдан", "Вход"})
	for rows.Next() {
		var id, name, status string
		var issued time.Time
		var redeemed *time.Time
		if err = rows.Scan(&id, &name, &status, &issued, &redeemed); err != nil {
			return err
		}
		at := ""
		if redeemed != nil {
			at = redeemed.Format(time.RFC3339)
		}
		_ = writer.Write([]string{id, csvCell(name), status, issued.Format(time.RFC3339), at})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	writer.Flush()
	if err = writer.Error(); err != nil {
		return err
	}
	if err = record(r.Context(), a.DB, e.ID, u.ID, "participants.exported", e.ID, map[string]string{"format": "csv"}); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="participants-`+e.ID+`.csv"`)
	w.Header().Set("Content-Length", strconv.Itoa(buffer.Len()))
	_, err = w.Write([]byte(buffer.String()))
	return err
}
