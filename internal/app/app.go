package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/D9veth/RBPO/internal/config"
	"github.com/D9veth/RBPO/internal/security"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	DB            *pgxpool.Pool
	Config        config.Config
	Logger        *slog.Logger
	passwordSlots chan struct{}
}
type ctxKey int

const actorKey ctxKey = 1
const requestIDKey ctxKey = 2
const sessionKey ctxKey = 3
const sessionCookie = "campus_session"

type Actor struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}
type APIError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *APIError) Error() string { return e.Message }
func problem(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}
func New(db *pgxpool.Pool, c config.Config, logger *slog.Logger) *App {
	return &App{DB: db, Config: c, Logger: logger, passwordSlots: make(chan struct{}, 4)}
}

type handler func(http.ResponseWriter, *http.Request) error

func (a *App) wrap(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			a.respondError(w, r, err)
		}
	}
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", a.wrap(func(w http.ResponseWriter, r *http.Request) error {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := a.DB.Ping(ctx); err != nil {
			return problem(503, "NOT_READY", "Сервис временно недоступен.")
		}
		write(w, 200, map[string]string{"status": "ready"})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/auth/register", a.wrap(a.register))
	mux.HandleFunc("POST /api/v1/auth/login", a.wrap(a.login))
	mux.HandleFunc("POST /api/v1/auth/logout", a.wrap(a.logout))
	mux.HandleFunc("POST /api/v1/auth/password", a.wrap(a.changePassword))
	mux.HandleFunc("GET /api/v1/me", a.wrap(a.me))
	mux.HandleFunc("PATCH /api/v1/me", a.wrap(a.updateProfile))
	mux.HandleFunc("GET /api/v1/events", a.wrap(a.listEvents))
	mux.HandleFunc("POST /api/v1/events", a.wrap(a.createEvent))
	mux.HandleFunc("GET /api/v1/events/{event}", a.wrap(a.getEvent))
	mux.HandleFunc("PATCH /api/v1/events/{event}", a.wrap(a.updateEvent))
	mux.HandleFunc("POST /api/v1/events/{event}/status", a.wrap(a.changeEventStatus))
	mux.HandleFunc("POST /api/v1/events/{event}/passes", a.wrap(a.issuePass))
	mux.HandleFunc("GET /api/v1/events/{event}/passes", a.wrap(a.listEventPasses))
	mux.HandleFunc("GET /api/v1/events/{event}/export", a.wrap(a.exportPasses))
	mux.HandleFunc("GET /api/v1/passes", a.wrap(a.myPasses))
	mux.HandleFunc("GET /api/v1/passes/{pass}", a.wrap(a.getPass))
	mux.HandleFunc("POST /api/v1/passes/{pass}/revoke", a.wrap(a.revokePass))
	mux.HandleFunc("POST /api/v1/events/{event}/check-in", a.wrap(a.checkIn))
	mux.HandleFunc("GET /api/v1/events/{event}/activity", a.wrap(a.activity))
	mux.HandleFunc("GET /api/v1/events/{event}/staff", a.wrap(a.listStaff))
	mux.HandleFunc("POST /api/v1/events/{event}/staff", a.wrap(a.inviteStaff))
	mux.HandleFunc("DELETE /api/v1/events/{event}/staff/{staff}", a.wrap(a.revokeStaff))
	mux.HandleFunc("POST /api/v1/invitations/inspect", a.wrap(a.inspectInvite))
	mux.HandleFunc("POST /api/v1/invitations/accept", a.wrap(a.acceptInvite))
	mux.HandleFunc("/api/", a.wrap(func(w http.ResponseWriter, r *http.Request) error {
		return problem(404, "NOT_FOUND", "Этот адрес API не существует.")
	}))
	mux.Handle("/", a.static())
	return a.middleware(mux)
}
func write(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}
func (a *App) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var api *APIError
	if !errors.As(err, &api) {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) && pgerr.Code == "23505" {
			api = problem(409, "CONFLICT", "Такая запись уже существует. Обновите страницу.")
		} else if errors.Is(err, pgx.ErrNoRows) {
			api = problem(404, "NOT_FOUND", "Запись не найдена.")
		} else {
			api = problem(500, "INTERNAL", "Не удалось выполнить действие. Попробуйте ещё раз.")
			// Never log request bodies, credentials, or PostgreSQL error details containing values.
			a.Logger.Error("request failed", "request_id", r.Context().Value(requestIDKey), "route", r.Pattern, "error_type", fmt.Sprintf("%T", err))
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(api.Status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": api, "request_id": r.Context().Value(requestIDKey)})
}
func decode(r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return problem(415, "JSON_REQUIRED", "Нужен запрос в формате JSON.")
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return problem(400, "INVALID_JSON", "Не удалось прочитать данные запроса.")
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return problem(400, "INVALID_JSON", "Запрос должен содержать один JSON-объект.")
	}
	return nil
}
func actor(r *http.Request) (*Actor, error) {
	v, _ := r.Context().Value(actorKey).(*Actor)
	if v == nil {
		return nil, problem(401, "UNAUTHENTICATED", "Войдите в аккаунт, чтобы продолжить.")
	}
	return v, nil
}
func optionalActor(r *http.Request) *Actor { v, _ := r.Context().Value(actorKey).(*Actor); return v }
func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rid := security.ID()
		ctx := context.WithValue(r.Context(), requestIDKey, rid)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", rid)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; media-src 'self' blob:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'")
		if a.Config.SecureCookies {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		defer func() {
			if rec := recover(); rec != nil {
				a.Logger.Error("panic", "request_id", rid, "type", fmt.Sprintf("%T", rec))
				a.respondError(w, r, problem(500, "INTERNAL", "Не удалось обработать запрос."))
			}
			a.Logger.Info("http", "method", r.Method, "route", r.Pattern, "duration_ms", time.Since(started).Milliseconds(), "request_id", rid)
		}()
		r.Body = http.MaxBytesReader(w, r.Body, 32768)
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if r.Header.Get("Origin") != a.Config.AppURL {
				a.respondError(w, r, problem(403, "ORIGIN_REJECTED", "Запрос отправлен с другого сайта."))
				return
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				a.respondError(w, r, problem(403, "ORIGIN_REJECTED", "Запрос отправлен с другого сайта."))
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if c, err := r.Cookie(sessionCookie); err == nil && len(c.Value) == 43 {
				var user Actor
				err = a.DB.QueryRow(ctx, `SELECT u.id,u.email,u.name FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now()`, security.Hash(c.Value)).Scan(&user.ID, &user.Email, &user.Name)
				if err == nil {
					ctx = context.WithValue(ctx, actorKey, &user)
					ctx = context.WithValue(ctx, sessionKey, c.Value)
					r = r.WithContext(ctx)
				} else if !errors.Is(err, pgx.ErrNoRows) {
					a.respondError(w, r, err)
					return
				}
			}
			if r.Method != "GET" && r.Method != "HEAD" && optionalActor(r) != nil && r.URL.Path != "/api/v1/auth/login" && r.URL.Path != "/api/v1/auth/register" {
				raw, _ := r.Context().Value(sessionKey).(string)
				if !security.Equal(r.Header.Get("X-CSRF-Token"), security.MAC(a.Config.Secret, "csrf:"+raw)) {
					a.respondError(w, r, problem(403, "CSRF_REJECTED", "Сессия изменилась. Обновите страницу и повторите действие."))
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) static() http.Handler {
	files := http.FileServer(http.Dir(a.Config.StaticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		if strings.HasPrefix(clean, "/.") {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(a.Config.StaticDir + clean); err == nil && !info.IsDir() {
			if strings.HasPrefix(clean, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, a.Config.StaticDir+"/index.html")
	})
}
func (a *App) rate(ctx context.Context, key string, max int, window time.Duration) error {
	start := time.Now().UTC().Truncate(window)
	var hits int
	err := a.DB.QueryRow(ctx, `INSERT INTO rate_limits(key,window_start,hits) VALUES($1,$2,1) ON CONFLICT(key) DO UPDATE SET hits=CASE WHEN rate_limits.window_start=$2 THEN rate_limits.hits+1 ELSE 1 END,window_start=$2 RETURNING hits`, security.Hash(key), start).Scan(&hits)
	if err != nil {
		return err
	}
	if hits > max {
		return problem(429, "RATE_LIMIT", "Слишком много запросов. Подождите немного и повторите.")
	}
	return nil
}
func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
func (a *App) passwordWork(ctx context.Context, f func() error) error {
	select {
	case a.passwordSlots <- struct{}{}:
		defer func() { <-a.passwordSlots }()
		return f()
	case <-ctx.Done():
		return ctx.Err()
	default:
		return problem(503, "AUTH_BUSY", "Сервис входа занят. Повторите через несколько секунд.")
	}
}
