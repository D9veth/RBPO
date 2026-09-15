package app

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/D9veth/RBPO/internal/security"
	"github.com/jackc/pgx/v5"
)

type Staff struct {
	ID         string     `json:"id"`
	EventID    string     `json:"event_id"`
	Label      string     `json:"label"`
	Role       string     `json:"role"`
	UserID     *string    `json:"user_id"`
	Name       *string    `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

func (a *App) listStaff(w http.ResponseWriter, r *http.Request) error {
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
	rows, err := a.DB.Query(r.Context(), `SELECT s.id,s.event_id,s.label,s.role,s.user_id,u.name,s.created_at,s.expires_at,s.accepted_at,s.revoked_at FROM event_staff s LEFT JOIN users u ON u.id=s.user_id WHERE s.event_id=$1 ORDER BY s.created_at DESC`, e.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []Staff{}
	for rows.Next() {
		var s Staff
		if err = rows.Scan(&s.ID, &s.EventID, &s.Label, &s.Role, &s.UserID, &s.Name, &s.CreatedAt, &s.ExpiresAt, &s.AcceptedAt, &s.RevokedAt); err != nil {
			return err
		}
		items = append(items, s)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	var ownerName string
	if err = a.DB.QueryRow(r.Context(), `SELECT name FROM users WHERE id=$1`, e.OwnerID).Scan(&ownerName); err != nil {
		return err
	}
	write(w, 200, map[string]any{"items": items, "owner_id": e.OwnerID, "owner_name": ownerName})
	return nil
}
func (a *App) inviteStaff(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	var in struct {
		Label string `json:"label"`
		Role  string `json:"role"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	in.Label = strings.TrimSpace(in.Label)
	if utf8.RuneCountInString(in.Label) < 2 || utf8.RuneCountInString(in.Label) > 120 || !contains([]string{"controller", "organizer"}, in.Role) {
		return problem(422, "VALIDATION", "Укажите имя приглашения и роль.")
	}
	if err = a.rate(r.Context(), "invite:"+u.ID, 30, time.Hour); err != nil {
		return err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	e, err := a.event(r.Context(), tx, r.PathValue("event"), u.ID, "FOR UPDATE OF e")
	if err != nil {
		return err
	}
	if e.OwnerID != u.ID {
		return problem(403, "OWNER_REQUIRED", "Состав команды изменяет владелец мероприятия.")
	}
	var count int
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM event_staff WHERE event_id=$1 AND revoked_at IS NULL AND (accepted_at IS NOT NULL OR expires_at>now())`, e.ID).Scan(&count); err != nil {
		return err
	}
	if count >= 50 {
		return problem(409, "STAFF_LIMIT", "В команде может быть до 50 приглашений и участников.")
	}
	token := security.Token()
	id := security.ID()
	expires := time.Now().Add(7 * 24 * time.Hour)
	if _, err = tx.Exec(r.Context(), `INSERT INTO event_staff(id,event_id,label,role,token_hash,invited_by,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, e.ID, in.Label, in.Role, security.Hash(token), u.ID, expires); err != nil {
		return err
	}
	if err = record(r.Context(), tx, e.ID, u.ID, "staff.invited", id, map[string]string{"role": in.Role}); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 201, map[string]any{"id": id, "token": token, "expires_at": expires, "role": in.Role})
	return nil
}
func (a *App) revokeStaff(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	e, err := a.event(r.Context(), tx, r.PathValue("event"), u.ID, "FOR UPDATE OF e")
	if err != nil {
		return err
	}
	if e.OwnerID != u.ID {
		return problem(403, "OWNER_REQUIRED", "Состав команды изменяет владелец мероприятия.")
	}
	result, err := tx.Exec(r.Context(), `UPDATE event_staff SET revoked_at=now() WHERE id=$1 AND event_id=$2 AND revoked_at IS NULL`, r.PathValue("staff"), e.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return problem(404, "NOT_FOUND", "Действующее приглашение не найдено.")
	}
	if err = record(r.Context(), tx, e.ID, u.ID, "staff.revoked", r.PathValue("staff"), map[string]string{}); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"revoked": true})
	return nil
}
func inviteToken(r *http.Request) (string, error) {
	var in struct {
		Token string `json:"token"`
	}
	if err := decode(r, &in); err != nil {
		return "", err
	}
	if len(in.Token) != 43 {
		return "", problem(404, "INVITE_INVALID", "Приглашение не найдено или больше не действует.")
	}
	return in.Token, nil
}
func (a *App) inspectInvite(w http.ResponseWriter, r *http.Request) error {
	token, err := inviteToken(r)
	if err != nil {
		return err
	}
	if err = a.rate(r.Context(), "invite-inspect:"+clientIP(r), 60, time.Minute); err != nil {
		return err
	}
	var title, role, label string
	var expires time.Time
	err = a.DB.QueryRow(r.Context(), `SELECT e.title,s.role,s.label,s.expires_at FROM event_staff s JOIN events e ON e.id=s.event_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.accepted_at IS NULL AND s.expires_at>now()`, security.Hash(token)).Scan(&title, &role, &label, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "INVITE_INVALID", "Приглашение не найдено или больше не действует.")
	}
	if err != nil {
		return err
	}
	write(w, 200, map[string]any{"event_title": title, "role": role, "label": label, "expires_at": expires})
	return nil
}
func (a *App) acceptInvite(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	token, err := inviteToken(r)
	if err != nil {
		return err
	}
	if err = a.rate(r.Context(), "invite-accept:"+u.ID, 20, time.Minute); err != nil {
		return err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var id, eventID, role string
	err = tx.QueryRow(r.Context(), `SELECT id,event_id,role FROM event_staff WHERE token_hash=$1`, security.Hash(token)).Scan(&id, &eventID, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "INVITE_INVALID", "Приглашение недействительно.")
	}
	if err != nil {
		return err
	}
	e, err := a.event(r.Context(), tx, eventID, u.ID, "FOR UPDATE OF e")
	if err != nil {
		return err
	}
	if e.OwnerID == u.ID {
		return problem(409, "ALREADY_OWNER", "Вы уже владелец этого мероприятия.")
	}
	var existing bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM event_staff WHERE event_id=$1 AND user_id=$2 AND revoked_at IS NULL)`, eventID, u.ID).Scan(&existing); err != nil {
		return err
	}
	if existing {
		return problem(409, "ALREADY_STAFF", "Вы уже состоите в команде этого мероприятия.")
	}
	result, err := tx.Exec(r.Context(), `UPDATE event_staff SET user_id=$2,accepted_at=now() WHERE id=$1 AND revoked_at IS NULL AND accepted_at IS NULL AND expires_at>now()`, id, u.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return problem(409, "INVITE_INVALID", "Приглашение использовано, отозвано или истекло.")
	}
	if err = record(r.Context(), tx, eventID, u.ID, "staff.accepted", id, map[string]string{"role": role}); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]string{"event_id": eventID, "role": role})
	return nil
}
