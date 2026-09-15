//go:build integration

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/D9veth/RBPO/internal/config"
	"github.com/D9veth/RBPO/internal/database"
	"github.com/D9veth/RBPO/internal/security"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testResponse struct {
	Status int
	Data   json.RawMessage
	Error  *APIError
	Header http.Header
	Raw    string
}
type testClient struct {
	client     *http.Client
	base, csrf string
	user       Actor
}

func (c *testClient) request(method, path string, data any, key string) testResponse {
	var body io.Reader
	if data != nil {
		raw, _ := json.Marshal(data)
		body = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, c.base+path, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.base)
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return testResponse{Status: 0, Raw: err.Error()}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var parsed struct {
		Data  json.RawMessage `json:"data"`
		Error *APIError       `json:"error"`
	}
	_ = json.Unmarshal(raw, &parsed)
	return testResponse{Status: res.StatusCode, Data: parsed.Data, Error: parsed.Error, Header: res.Header, Raw: string(raw)}
}
func expect(t *testing.T, r testResponse, status int) {
	t.Helper()
	if r.Status != status {
		t.Fatalf("want status %d, got %d: %s", status, r.Status, r.Raw)
	}
}
func parse[T any](t *testing.T, r testResponse) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(r.Data, &v); err != nil {
		t.Fatalf("decode: %v (%s)", err, r.Raw)
	}
	return v
}
func fixture(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL must point to an isolated test database")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + security.ID()
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 20
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	app := New(pool, config.Config{Secret: strings.Repeat("integration-secret", 4), StaticDir: "../../dist"}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	server := httptest.NewUnstartedServer(app.Handler())
	app.Config.AppURL = "http://" + server.Listener.Addr().String()
	server.Start()
	t.Cleanup(func() {
		server.Close()
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	})
	return pool, server.URL
}
func newClient(base string) *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{base: base, client: &http.Client{Jar: jar, Timeout: 20 * time.Second}}
}
func registerClient(t *testing.T, base, name string) *testClient {
	t.Helper()
	c := newClient(base)
	res := c.request("POST", "/api/v1/auth/register", map[string]string{"email": name + "@example.test", "name": name, "password": "a long secure test passphrase"}, "")
	expect(t, res, 201)
	var auth struct {
		User Actor  `json:"user"`
		CSRF string `json:"csrf_token"`
	}
	auth = parse[struct {
		User Actor  `json:"user"`
		CSRF string `json:"csrf_token"`
	}](t, res)
	c.csrf = auth.CSRF
	c.user = auth.User
	return c
}
func eventPayload(capacity int) EventInput {
	now := time.Now().UTC().Truncate(time.Second)
	return EventInput{Title: "Студенческие проекты", Description: "Обсуждаем проекты и встречаем участников студенческого сообщества.", Category: "Сообщества", Location: "Кампус, аудитория 101", StartsAt: now.Add(-30 * time.Minute), EndsAt: now.Add(time.Hour), DoorsAt: now.Add(-time.Hour), RegistrationEndsAt: now.Add(45 * time.Minute), Capacity: capacity, Color: "blue"}
}
func createPublished(t *testing.T, c *testClient, capacity int) Event {
	t.Helper()
	res := c.request("POST", "/api/v1/events", eventPayload(capacity), "")
	expect(t, res, 201)
	e := parse[Event](t, res)
	res = c.request("POST", "/api/v1/events/"+e.ID+"/status", map[string]any{"status": "published", "version": e.Version}, "")
	expect(t, res, 200)
	return parse[Event](t, res)
}
func issue(t *testing.T, c *testClient, e Event, name string) Pass {
	t.Helper()
	res := c.request("POST", "/api/v1/events/"+e.ID+"/passes", map[string]string{"guest_name": name}, security.ID())
	expect(t, res, 201)
	return parse[struct {
		Pass Pass `json:"pass"`
	}](t, res).Pass
}
func codeFor(t *testing.T, c *testClient, p Pass) string {
	t.Helper()
	res := c.request("GET", "/api/v1/passes/"+p.ID, nil, "")
	expect(t, res, 200)
	return parse[struct {
		Code string `json:"code"`
	}](t, res).Code
}
func invite(t *testing.T, owner, other *testClient, e Event) string {
	t.Helper()
	res := owner.request("POST", "/api/v1/events/"+e.ID+"/staff", map[string]string{"label": "Главный вход", "role": "controller"}, "")
	expect(t, res, 201)
	v := parse[struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}](t, res)
	res = other.request("POST", "/api/v1/invitations/accept", map[string]string{"token": v.Token}, "")
	expect(t, res, 200)
	expect(t, other.request("POST", "/api/v1/invitations/accept", map[string]string{"token": v.Token}, ""), 409)
	return v.ID
}
func TestFlowsAndInvariants(t *testing.T) {
	pool, base := fixture(t)
	owner := registerClient(t, base, "Организатор")
	student := registerClient(t, base, "Студент")
	other := registerClient(t, base, "Другой")
	controller := registerClient(t, base, "Контролёр")
	anonymous := newClient(base)
	t.Run("authentication-and-request-boundaries", func(t *testing.T) {
		expect(t, anonymous.request("POST", "/api/v1/events", eventPayload(10), ""), 401)
		saved := owner.csrf
		owner.csrf = ""
		expect(t, owner.request("POST", "/api/v1/events", eventPayload(10), ""), 403)
		owner.csrf = saved
		req, _ := http.NewRequest("POST", base+"/api/v1/auth/login", strings.NewReader(`{"email":"x","password":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://attacker.invalid")
		res, err := owner.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatal("cross-site POST allowed")
		}
		me := owner.request("GET", "/api/v1/me", nil, "")
		expect(t, me, 200)
		if me.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("private data cacheable")
		}
	})
	e := createPublished(t, owner, 40)
	staffID := invite(t, owner, controller, e)
	t.Run("draft-and-role-isolation", func(t *testing.T) {
		draft := parse[Event](t, owner.request("POST", "/api/v1/events", eventPayload(10), ""))
		expect(t, anonymous.request("GET", "/api/v1/events/"+draft.ID, nil, ""), 404)
		expect(t, student.request("POST", "/api/v1/events/"+e.ID+"/status", map[string]any{"status": "cancelled", "version": e.Version}, ""), 403)
		expect(t, controller.request("POST", "/api/v1/events/"+e.ID+"/passes", map[string]string{"guest_name": "Гость"}, security.ID()), 403)
		expect(t, controller.request("GET", "/api/v1/events/"+e.ID+"/passes", nil, ""), 403)
	})
	p := issue(t, student, e, "")
	code := codeFor(t, student, p)
	t.Run("personal-pass-and-code-confidentiality", func(t *testing.T) {
		expect(t, student.request("POST", "/api/v1/events/"+e.ID+"/passes", map[string]string{}, security.ID()), 409)
		expect(t, other.request("GET", "/api/v1/passes/"+p.ID, nil, ""), 404)
		expect(t, controller.request("GET", "/api/v1/passes/"+p.ID, nil, ""), 404)
		expect(t, anonymous.request("GET", "/api/v1/passes/"+p.ID, nil, ""), 401)
		var stored string
		if err := pool.QueryRow(context.Background(), `SELECT code_hash FROM passes WHERE id=$1`, p.ID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		norm, _ := security.NormalizeCode(code)
		if stored != security.Hash(norm) || strings.Contains(stored, code) {
			t.Fatal("code not hashed")
		}
		res := student.request("GET", "/api/v1/passes", nil, "")
		if strings.Contains(res.Raw, "code_hash") || strings.Contains(res.Raw, code) {
			t.Fatal("list leaks secret")
		}
	})
	t.Run("last-seat-is-atomic", func(t *testing.T) {
		last := createPublished(t, owner, 1)
		var wg sync.WaitGroup
		responses := make([]testResponse, 2)
		for i, c := range []*testClient{student, other} {
			wg.Add(1)
			go func(i int, c *testClient) {
				defer wg.Done()
				responses[i] = c.request("POST", "/api/v1/events/"+last.ID+"/passes", map[string]string{}, security.ID())
			}(i, c)
		}
		wg.Wait()
		accepted := 0
		for _, r := range responses {
			if r.Status == 201 {
				accepted++
			} else {
				expect(t, r, 409)
			}
		}
		if accepted != 1 {
			t.Fatalf("issued %d passes for one seat", accepted)
		}
		var count int
		_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM passes WHERE event_id=$1`, last.ID).Scan(&count)
		if count != 1 {
			t.Fatal("oversold event")
		}
	})
	t.Run("issue-retries-are-idempotent", func(t *testing.T) {
		k := security.ID()
		var wg sync.WaitGroup
		responses := make([]testResponse, 2)
		for i := range responses {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				responses[i] = owner.request("POST", "/api/v1/events/"+e.ID+"/passes", map[string]string{"guest_name": "Приглашённый гость"}, k)
			}(i)
		}
		wg.Wait()
		first := parse[struct {
			Pass Pass `json:"pass"`
		}](t, responses[0])
		second := parse[struct {
			Pass Pass `json:"pass"`
		}](t, responses[1])
		if first.Pass.ID == "" || first.Pass.ID != second.Pass.ID {
			t.Fatal("retry created distinct passes")
		}
		expect(t, owner.request("POST", "/api/v1/events/"+e.ID+"/passes", map[string]string{"guest_name": "Другой гость"}, k), 409)
	})
	t.Run("two-gates-redeem-only-once-and-retry-safely", func(t *testing.T) {
		var wg sync.WaitGroup
		responses := make([]testResponse, 2)
		keys := []string{security.ID(), security.ID()}
		clients := []*testClient{owner, controller}
		for i, c := range clients {
			wg.Add(1)
			go func(i int, c *testClient) {
				defer wg.Done()
				responses[i] = c.request("POST", "/api/v1/events/"+e.ID+"/check-in", map[string]string{"code": code}, keys[i])
			}(i, c)
		}
		wg.Wait()
		accepted := 0
		winner := -1
		for i, r := range responses {
			result := parse[Admission](t, r)
			if result.Result == "accepted" {
				expect(t, r, 200)
				accepted++
				winner = i
			} else {
				expect(t, r, 409)
				if result.Result != "already_used" {
					t.Fatalf("unexpected %s", r.Raw)
				}
			}
		}
		if accepted != 1 {
			t.Fatalf("accepted %d requests", accepted)
		}
		replay := clients[winner].request("POST", "/api/v1/events/"+e.ID+"/check-in", map[string]string{"code": code}, keys[winner])
		expect(t, replay, 200)
		if !parse[Admission](t, replay).Replay {
			t.Fatal("retry not recognised")
		}
		var count int
		_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM redemptions WHERE pass_id=$1`, p.ID).Scan(&count)
		if count != 1 {
			t.Fatal("duplicate redemption")
		}
		_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM admission_attempts WHERE pass_id=$1`, p.ID).Scan(&count)
		if count != 2 {
			t.Fatalf("replay produced audit entry: %d", count)
		}
	})
	t.Run("wrong-event-revocation-and-event-cancellation", func(t *testing.T) {
		event2 := createPublished(t, owner, 20)
		guest := issue(t, owner, event2, "Гость второго события")
		guestCode := codeFor(t, owner, guest)
		result := controller.request("POST", "/api/v1/events/"+e.ID+"/check-in", map[string]string{"code": guestCode}, security.ID())
		expect(t, result, 409)
		if parse[Admission](t, result).Result != "wrong_event" {
			t.Fatal(result.Raw)
		}
		expect(t, other.request("POST", "/api/v1/events/"+event2.ID+"/check-in", map[string]string{"code": guestCode}, security.ID()), 403)
		expect(t, owner.request("POST", "/api/v1/passes/"+guest.ID+"/revoke", map[string]string{"reason": "Планы изменились"}, ""), 200)
		res := owner.request("POST", "/api/v1/events/"+event2.ID+"/check-in", map[string]string{"code": guestCode}, security.ID())
		expect(t, res, 409)
		if parse[Admission](t, res).Result != "revoked" {
			t.Fatal(res.Raw)
		}
		another := issue(t, owner, event2, "Ещё один гость")
		anotherCode := codeFor(t, owner, another)
		expect(t, owner.request("POST", "/api/v1/events/"+event2.ID+"/status", map[string]any{"status": "cancelled", "version": event2.Version}, ""), 200)
		res = owner.request("POST", "/api/v1/events/"+event2.ID+"/check-in", map[string]string{"code": anotherCode}, security.ID())
		expect(t, res, 409)
		if parse[Admission](t, res).Result != "cancelled" {
			t.Fatal(res.Raw)
		}
	})
	t.Run("versioning-and-capacity-update", func(t *testing.T) {
		input := e.EventInput
		input.Capacity = 1
		payload := map[string]any{}
		raw, _ := json.Marshal(input)
		_ = json.Unmarshal(raw, &payload)
		payload["version"] = e.Version
		expect(t, owner.request("PATCH", "/api/v1/events/"+e.ID, payload, ""), 409)
		payload["capacity"] = 100
		expect(t, owner.request("PATCH", "/api/v1/events/"+e.ID, payload, ""), 200)
		expect(t, owner.request("PATCH", "/api/v1/events/"+e.ID, payload, ""), 409)
	})
	t.Run("csv-formulas-escaped-and-audit-readable", func(t *testing.T) {
		issue(t, owner, e, "=HYPERLINK(\"https://evil.invalid\")")
		export := owner.request("GET", "/api/v1/events/"+e.ID+"/export", nil, "")
		expect(t, export, 200)
		if !strings.Contains(export.Raw, "'=HYPERLINK") {
			t.Fatal("CSV formula was not escaped")
		}
		log := owner.request("GET", "/api/v1/events/"+e.ID+"/activity", nil, "")
		expect(t, log, 200)
		if !strings.Contains(log.Raw, "participants.exported") {
			t.Fatal("export not audited")
		}
		if strings.Contains(log.Raw, code) {
			t.Fatal("audit leaks code")
		}
	})
	t.Run("revoked-staff-loses-access-immediately", func(t *testing.T) {
		expect(t, owner.request("DELETE", "/api/v1/events/"+e.ID+"/staff/"+staffID, nil, ""), 200)
		expect(t, controller.request("POST", "/api/v1/events/"+e.ID+"/check-in", map[string]string{"code": code}, security.ID()), 403)
	})
	t.Run("revoked-organizer-cannot-replay-guest-issue", func(t *testing.T) {
		res := owner.request("POST", "/api/v1/events/"+e.ID+"/staff", map[string]string{"label": "Соорганизатор", "role": "organizer"}, "")
		expect(t, res, 201)
		invitation := parse[struct{ ID, Token string }](t, res)
		expect(t, other.request("POST", "/api/v1/invitations/accept", map[string]string{"token": invitation.Token}, ""), 200)
		k := security.ID()
		payload := map[string]string{"guest_name": "Гость соорганизатора"}
		expect(t, other.request("POST", "/api/v1/events/"+e.ID+"/passes", payload, k), 201)
		expect(t, owner.request("DELETE", "/api/v1/events/"+e.ID+"/staff/"+invitation.ID, nil, ""), 200)
		expect(t, other.request("POST", "/api/v1/events/"+e.ID+"/passes", payload, k), 403)
	})
	t.Run("password-change-and-session-revocation", func(t *testing.T) {
		second := newClient(base)
		login := second.request("POST", "/api/v1/auth/login", map[string]string{"email": owner.user.Email, "password": "a long secure test passphrase"}, "")
		expect(t, login, 200)
		second.csrf = parse[struct {
			CSRF string `json:"csrf_token"`
		}](t, login).CSRF
		expect(t, owner.request("POST", "/api/v1/auth/password", map[string]string{"current_password": "a long secure test passphrase", "password": "a different secure passphrase"}, ""), 200)
		me := parse[struct {
			User *Actor `json:"user"`
		}](t, second.request("GET", "/api/v1/me", nil, ""))
		if me.User != nil {
			t.Fatal("old session still active")
		}
		expect(t, owner.request("POST", "/api/v1/auth/logout", map[string]string{}, ""), 200)
		expect(t, owner.request("GET", "/api/v1/passes", nil, ""), 401)
	})
}
func TestCSVCell(t *testing.T) {
	for _, s := range []string{"=1+1", "  +cmd", "\t@SUM(1)", "-4"} {
		if !strings.HasPrefix(csvCell(s), "'") {
			t.Fatalf("not escaped: %q", s)
		}
	}
	if csvCell("Обычное имя") != "Обычное имя" {
		t.Fatal("ordinary value changed")
	}
}
func Example_validateEvent() {
	v := eventPayload(0)
	fmt.Println(validateEvent(&v) != nil) // Output: true
}
