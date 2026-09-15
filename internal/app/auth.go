package app

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/D9veth/RBPO/internal/security"
	"github.com/jackc/pgx/v5"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func validateEmail(v string) bool {
	m, e := mail.ParseAddress(v)
	return e == nil && m.Address == v && len(v) <= 254 && strings.Contains(v, "@")
}
func validatePassword(v string) bool { return utf8.RuneCountInString(v) >= 12 && len(v) <= 128 }
func (a *App) register(w http.ResponseWriter, r *http.Request) error {
	var in credentials
	if err := decode(r, &in); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	fields := map[string]string{}
	if !validateEmail(in.Email) {
		fields["email"] = "Введите корректный email"
	}
	if n := utf8.RuneCountInString(in.Name); n < 2 || n > 120 {
		fields["name"] = "Имя: от 2 до 120 символов"
	}
	if !validatePassword(in.Password) {
		fields["password"] = "Пароль: минимум 12 символов, максимум 128 байт"
	}
	if len(fields) > 0 {
		return &APIError{Status: 422, Code: "VALIDATION", Message: "Проверьте поля формы.", Fields: fields}
	}
	if err := a.rate(r.Context(), "register:"+clientIP(r), 10, 15*time.Minute); err != nil {
		return err
	}
	hash := ""
	if err := a.passwordWork(r.Context(), func() error { var e error; hash, e = security.HashPassword(in.Password); return e }); err != nil {
		return err
	}
	user := Actor{ID: security.ID(), Email: in.Email, Name: in.Name}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `INSERT INTO users(id,email,name,password_hash) VALUES($1,$2,$3,$4)`, user.ID, user.Email, user.Name, hash)
	if err != nil {
		return err
	}
	token := security.Token()
	if _, err = tx.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, security.Hash(token), user.ID, time.Now().Add(7*24*time.Hour)); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	a.setCookie(w, token)
	write(w, 201, map[string]any{"user": user, "csrf_token": security.MAC(a.Config.Secret, "csrf:"+token)})
	return nil
}
func (a *App) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if len(in.Password) > 128 || len(in.Email) > 254 {
		return problem(401, "INVALID_CREDENTIALS", "Неверный email или пароль.")
	}
	if err := a.rate(r.Context(), "login-ip:"+clientIP(r), 60, 15*time.Minute); err != nil {
		return err
	}
	if err := a.rate(r.Context(), "login-account:"+in.Email, 15, 15*time.Minute); err != nil {
		return err
	}
	var u Actor
	var hash string
	err := a.DB.QueryRow(r.Context(), `SELECT id,email,name,password_hash FROM users WHERE email=$1`, in.Email).Scan(&u.ID, &u.Email, &u.Name, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	found := err == nil
	if !found {
		hash = "$argon2id$v=19$m=65536,t=3,p=2$MDEyMzQ1Njc4OWFiY2RlZg$MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"
	}
	valid := false
	if err = a.passwordWork(r.Context(), func() error { valid = security.VerifyPassword(in.Password, hash); return nil }); err != nil {
		return err
	}
	if !found || !valid {
		return problem(401, "INVALID_CREDENTIALS", "Неверный email или пароль.")
	}
	token := security.Token()
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	// Serialise session changes with password changes for this account.
	if _, err = tx.Exec(r.Context(), `SELECT id FROM users WHERE id=$1 FOR UPDATE`, u.ID); err != nil {
		return err
	}
	var currentHash string
	if err = tx.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&currentHash); err != nil {
		return err
	}
	if currentHash != hash {
		return problem(401, "INVALID_CREDENTIALS", "Пароль изменился. Войдите ещё раз.")
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash IN (SELECT token_hash FROM sessions WHERE user_id=$1 ORDER BY created_at DESC OFFSET 9) OR (user_id=$1 AND expires_at<=now())`, u.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, security.Hash(token), u.ID, time.Now().Add(7*24*time.Hour)); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	a.setCookie(w, token)
	write(w, 200, map[string]any{"user": u, "csrf_token": security.MAC(a.Config.Secret, "csrf:"+token)})
	return nil
}
func (a *App) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: a.Config.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 86400})
}
func (a *App) me(w http.ResponseWriter, r *http.Request) error {
	u := optionalActor(r)
	csrf := ""
	if u != nil {
		raw, _ := r.Context().Value(sessionKey).(string)
		csrf = security.MAC(a.Config.Secret, "csrf:"+raw)
	}
	write(w, 200, map[string]any{"user": u, "csrf_token": csrf})
	return nil
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) error {
	raw, _ := r.Context().Value(sessionKey).(string)
	if raw != "" {
		if _, err := a.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash=$1`, security.Hash(raw)); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", Value: "", MaxAge: -1, HttpOnly: true, Secure: a.Config.SecureCookies, SameSite: http.SameSiteLaxMode})
	write(w, 200, map[string]bool{"signed_out": true})
	return nil
}
func (a *App) updateProfile(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	var in struct {
		Name string `json:"name"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(in.Name); n < 2 || n > 120 {
		return problem(422, "VALIDATION", "Имя должно содержать от 2 до 120 символов.")
	}
	if _, err = a.DB.Exec(r.Context(), `UPDATE users SET name=$1 WHERE id=$2`, in.Name, u.ID); err != nil {
		return err
	}
	u.Name = in.Name
	write(w, 200, u)
	return nil
}
func (a *App) changePassword(w http.ResponseWriter, r *http.Request) error {
	u, err := actor(r)
	if err != nil {
		return err
	}
	var in struct {
		Current  string `json:"current_password"`
		Password string `json:"password"`
	}
	if err = decode(r, &in); err != nil {
		return err
	}
	if !validatePassword(in.Password) || len(in.Current) > 128 {
		return problem(422, "VALIDATION", "Новый пароль должен содержать не менее 12 символов и не более 128 байт.")
	}
	if err = a.rate(r.Context(), "password:"+u.ID, 5, 15*time.Minute); err != nil {
		return err
	}
	var old string
	if err = a.DB.QueryRow(r.Context(), `SELECT password_hash FROM users WHERE id=$1`, u.ID).Scan(&old); err != nil {
		return err
	}
	var next string
	if err = a.passwordWork(r.Context(), func() error {
		if !security.VerifyPassword(in.Current, old) {
			return problem(401, "INVALID_CREDENTIALS", "Текущий пароль неверен.")
		}
		var e error
		next, e = security.HashPassword(in.Password)
		return e
	}); err != nil {
		return err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	result, err := tx.Exec(r.Context(), `UPDATE users SET password_hash=$1 WHERE id=$2 AND password_hash=$3`, next, u.ID, old)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return problem(409, "CONFLICT", "Пароль уже изменился. Повторите вход.")
	}
	raw, _ := r.Context().Value(sessionKey).(string)
	if _, err = tx.Exec(r.Context(), `DELETE FROM sessions WHERE user_id=$1 AND token_hash<>$2`, u.ID, security.Hash(raw)); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	write(w, 200, map[string]bool{"changed": true})
	return nil
}
