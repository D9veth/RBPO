package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/D9veth/RBPO/internal/security"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
type EventInput struct {
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	Category           string    `json:"category"`
	Location           string    `json:"location"`
	StartsAt           time.Time `json:"starts_at"`
	EndsAt             time.Time `json:"ends_at"`
	DoorsAt            time.Time `json:"doors_at"`
	RegistrationEndsAt time.Time `json:"registration_ends_at"`
	Capacity           int       `json:"capacity"`
	Color              string    `json:"color"`
}
type Event struct {
	EventInput
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Status    string    `json:"status"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Issued    int       `json:"issued"`
	CheckedIn int       `json:"checked_in"`
	Role      string    `json:"role"`
}

const eventColumns = `e.id,e.owner_id,e.title,e.description,e.category,e.location,e.starts_at,e.ends_at,e.doors_at,e.registration_ends_at,e.capacity,e.status,e.color,e.version,e.created_at,e.updated_at,
 (SELECT count(*) FROM passes p WHERE p.event_id=e.id AND p.status<>'revoked'),
 (SELECT count(*) FROM passes p WHERE p.event_id=e.id AND p.status='redeemed'),
 CASE WHEN e.owner_id=$1 THEN 'organizer' ELSE COALESCE((SELECT s.role FROM event_staff s WHERE s.event_id=e.id AND s.user_id=$1 AND s.revoked_at IS NULL),'') END`

func scanEvent(row pgx.Row) (Event, error) {
	var e Event
	err := row.Scan(&e.ID, &e.OwnerID, &e.Title, &e.Description, &e.Category, &e.Location, &e.StartsAt, &e.EndsAt, &e.DoorsAt, &e.RegistrationEndsAt, &e.Capacity, &e.Status, &e.Color, &e.Version, &e.CreatedAt, &e.UpdatedAt, &e.Issued, &e.CheckedIn, &e.Role)
	return e, err
}
func (a *App) event(ctx context.Context, q queryer, id, user, lock string) (Event, error) {
	if lock != "" {
		var locked string
		if err := q.QueryRow(ctx, `SELECT e.id FROM events e WHERE e.id=$1 `+lock, id).Scan(&locked); err != nil {
			return Event{}, err
		}
	}
	// A fresh statement after acquiring the lock observes committed pass counts.
	return scanEvent(q.QueryRow(ctx, `SELECT `+eventColumns+` FROM events e WHERE e.id=$2`, user, id))
}
func validateEvent(in *EventInput) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Location = strings.TrimSpace(in.Location)
	if in.Color == "" {
		in.Color = "blue"
	}
	fields := map[string]string{}
	length := func(k, v string, min, max int) {
		n := utf8.RuneCountInString(v)
		if n < min || n > max {
			fields[k] = "Длина: от " + strconv.Itoa(min) + " до " + strconv.Itoa(max) + " символов"
		}
	}
	length("title", in.Title, 4, 120)
	length("description", in.Description, 20, 5000)
	length("location", in.Location, 3, 240)
	if !contains([]string{"Лекции", "Карьера", "Культура", "Спорт", "Сообщества"}, in.Category) {
		fields["category"] = "Выберите категорию"
	}
	if !contains([]string{"blue", "violet", "orange", "green", "pink"}, in.Color) {
		fields["color"] = "Выберите цвет"
	}
	if in.Capacity < 1 || in.Capacity > 10000 {
		fields["capacity"] = "От 1 до 10 000 мест"
	}
	if in.StartsAt.IsZero() || in.StartsAt.Year() < 2020 || in.StartsAt.Year() > 2100 {
		fields["starts_at"] = "Укажите корректную дату начала"
	}
	if !in.EndsAt.After(in.StartsAt) || in.EndsAt.Sub(in.DoorsAt) > 14*24*time.Hour {
		fields["ends_at"] = "Окончание должно быть позже начала; длительность — до 14 дней"
	}
	if in.DoorsAt.IsZero() || in.DoorsAt.After(in.StartsAt) {
		fields["doors_at"] = "Вход должен открыться не позже начала"
	}
	if in.RegistrationEndsAt.IsZero() || in.RegistrationEndsAt.After(in.EndsAt) {
		fields["registration_ends_at"] = "Регистрация должна закончиться не позже мероприятия"
	}
	if len(fields) > 0 {
		return &APIError{Status: 422, Code: "VALIDATION", Message: "Проверьте поля мероприятия.", Fields: fields}
	}
	return nil
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func permission(e Event, required string) error {
	if e.Role == "organizer" || (required == "controller" && e.Role == "controller") {
		return nil
	}
	return problem(403, "FORBIDDEN", "У вас нет прав для этого действия.")
}
func record(ctx context.Context, q queryer, event, user, action, target string, details any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO audit_log(event_id,actor_id,action,target_id,details) VALUES($1,$2,$3,NULLIF($4,''),$5)`, event, user, action, target, raw)
	return err
}
func (a *App) listEvents(w http.ResponseWriter, r *http.Request) error {
	user := ""
	if u := optionalActor(r); u != nil {
		user = u.ID
	}
	scope := r.URL.Query().Get("scope")
	if scope != "" && scope != "discover" && scope != "manage" && scope != "control" {
		return problem(422, "VALIDATION", "Неизвестный список мероприятий.")
	}
	if (scope == "manage" || scope == "control") && user == "" {
		return problem(401, "UNAUTHENTICATED", "Войдите в аккаунт.")
	}
	where := `e.status='published'`
	if scope == "manage" {
		where = `(e.owner_id=$1 OR EXISTS(SELECT 1 FROM event_staff s WHERE s.event_id=e.id AND s.user_id=$1 AND s.role='organizer' AND s.revoked_at IS NULL))`
	}
	if scope == "control" {
		where = `e.status='published' AND (e.owner_id=$1 OR EXISTS(SELECT 1 FROM event_staff s WHERE s.event_id=e.id AND s.user_id=$1 AND s.revoked_at IS NULL))`
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 200 {
		return problem(422, "VALIDATION", "Поисковый запрос слишком длинный.")
	}
	category := r.URL.Query().Get("category")
	period := r.URL.Query().Get("period")
	where += ` AND ($2='' OR e.title ILIKE '%'||$2||'%' OR e.location ILIKE '%'||$2||'%') AND ($3='' OR e.category=$3) AND ($4<>'upcoming' OR e.ends_at>now()) AND ($4<>'past' OR e.ends_at<=now())`
	page := security.LimitInt(r.URL.Query().Get("page"), 1, 10000)
	limit := security.LimitInt(r.URL.Query().Get("limit"), 24, 100)
	var total int
	// Keep $1 typed even in the public scope where ownership is not a predicate.
	if err := a.DB.QueryRow(r.Context(), `SELECT count(*) FROM events e WHERE ($1::text IS NOT NULL) AND `+where, user, q, category, period).Scan(&total); err != nil {
		return err
	}
	order := "DESC"
	if period == "upcoming" {
		order = "ASC"
	}
	rows, err := a.DB.Query(r.Context(), `SELECT `+eventColumns+` FROM events e WHERE `+where+` ORDER BY e.starts_at `+order+`,e.id LIMIT $5 OFFSET $6`, user, q, category, period, limit, (page-1)*limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []Event{}
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return err
		}
		items = append(items, e)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	write(w, 200, map[string]any{"items": items, "total": total, "page": page, "limit": limit})
	return nil
}
func (a *App) createEvent(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	if err = a.rate(r.Context(), "event-create:"+u.ID, 20, time.Hour); err != nil {
		return err
	}
	var in EventInput
	if err = decode(r, &in); err != nil {
		return err
	}
	if err = validateEvent(&in); err != nil {
		return err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	id := security.ID()
	_, err = tx.Exec(r.Context(), `INSERT INTO events(id,owner_id,title,description,category,location,starts_at,ends_at,doors_at,registration_ends_at,capacity,color) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, u.ID, in.Title, in.Description, in.Category, in.Location, in.StartsAt, in.EndsAt, in.DoorsAt, in.RegistrationEndsAt, in.Capacity, in.Color)
	if err != nil {
		return err
	}
	if err = record(r.Context(), tx, id, u.ID, "event.created", id, map[string]string{"title": in.Title}); err != nil {
		return err
	}
	e, err := a.event(r.Context(), tx, id, u.ID, "")
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 201, e)
	return nil
}
func (a *App) getEvent(w http.ResponseWriter, r *http.Request) error {
	uid := ""
	if u := optionalActor(r); u != nil {
		uid = u.ID
	}
	e, err := a.event(r.Context(), a.DB, r.PathValue("event"), uid, "")
	if err != nil {
		return err
	}
	if e.Status == "draft" && e.Role == "" {
		return problem(404, "NOT_FOUND", "Мероприятие не найдено.")
	}
	write(w, 200, e)
	return nil
}
func (a *App) updateEvent(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	var in struct {
		EventInput
		Version int `json:"version"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	if err = validateEvent(&in.EventInput); err != nil {
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
	if err = permission(e, "organizer"); err != nil {
		return err
	}
	if e.Status == "cancelled" {
		return problem(409, "EVENT_CANCELLED", "Отменённое мероприятие нельзя редактировать.")
	}
	if e.Version != in.Version {
		return problem(409, "VERSION_CONFLICT", "Мероприятие уже изменилось. Обновите данные перед сохранением.")
	}
	if in.Capacity < e.Issued {
		return problem(409, "CAPACITY_CONFLICT", "Вместимость не может быть меньше числа действующих и погашенных пропусков.")
	}
	_, err = tx.Exec(r.Context(), `UPDATE events SET title=$2,description=$3,category=$4,location=$5,starts_at=$6,ends_at=$7,doors_at=$8,registration_ends_at=$9,capacity=$10,color=$11,version=version+1,updated_at=now() WHERE id=$1`, e.ID, in.Title, in.Description, in.Category, in.Location, in.StartsAt, in.EndsAt, in.DoorsAt, in.RegistrationEndsAt, in.Capacity, in.Color)
	if err != nil {
		return err
	}
	if err = record(r.Context(), tx, e.ID, u.ID, "event.updated", e.ID, map[string]int{"version": e.Version + 1}); err != nil {
		return err
	}
	e, err = a.event(r.Context(), tx, e.ID, u.ID, "")
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, e)
	return nil
}
func (a *App) changeEventStatus(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	var in struct {
		Status  string `json:"status"`
		Version int    `json:"version"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	if !contains([]string{"published", "draft", "cancelled"}, in.Status) {
		return problem(422, "VALIDATION", "Неизвестный статус.")
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
	if err = permission(e, "organizer"); err != nil {
		return err
	}
	if e.Version != in.Version {
		return problem(409, "VERSION_CONFLICT", "Мероприятие уже изменилось. Обновите страницу.")
	}
	if e.Status == "cancelled" {
		return problem(409, "EVENT_CANCELLED", "Отмена мероприятия окончательна.")
	}
	if in.Status == "published" && !e.EndsAt.After(time.Now()) {
		return problem(409, "EVENT_ENDED", "Нельзя опубликовать завершённое мероприятие.")
	}
	if in.Status == "draft" && e.Issued > 0 {
		return problem(409, "HAS_PASSES", "У мероприятия уже есть пропуска. Вместо снятия с публикации используйте отмену.")
	}
	_, err = tx.Exec(r.Context(), `UPDATE events SET status=$2,version=version+1,updated_at=now() WHERE id=$1`, e.ID, in.Status)
	if err != nil {
		return err
	}
	if err = record(r.Context(), tx, e.ID, u.ID, "event."+in.Status, e.ID, map[string]string{"previous_status": e.Status}); err != nil {
		return err
	}
	e, err = a.event(r.Context(), tx, e.ID, u.ID, "")
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, e)
	return nil
}
