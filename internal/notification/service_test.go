package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"anypitch/internal/db"
	_ "modernc.org/sqlite"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixture(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "notice.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO users(id,email,password_hash,status,created_at) VALUES('u','u@example.com','x','active','now')`,
		`INSERT INTO teams(id,user_id,name,created_at,updated_at) VALUES('team','u','Team','now','now')`,
		`INSERT INTO players(id,team_id,name,positions_json,status,created_at,updated_at) VALUES('p','team','Player','[]','active','now','now')`,
		`INSERT INTO wechat_members(player_id,app_id,open_id) VALUES('p','app','openid')`,
	} {
		if _, err := conn.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return conn, New(conn, "template")
}

func addEvent(t *testing.T, conn *sql.DB, id string) {
	t.Helper()
	_, err := conn.Exec(`INSERT INTO events(id,team_id,type,title,starts_at,location,opponent,notes,created_at,updated_at)
		VALUES(?,'team','training','周三训练','2026-10-28T19:00:00+08:00','北京邮电大学','', '',?,?)`, id, id, id)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGrantIsIdempotentAndEventCreationReservesOnePermission(t *testing.T) {
	conn, service := fixture(t)
	for _, requestID := range []string{"first", "first", "second"} {
		if _, err := service.Grant("p", requestID); err != nil {
			t.Fatal(err)
		}
	}
	if remaining, _ := service.Remaining("p"); remaining != 2 {
		t.Fatalf("remaining=%d", remaining)
	}
	addEvent(t, conn, "event-1")
	if remaining, _ := service.Remaining("p"); remaining != 1 {
		t.Fatalf("remaining after event=%d", remaining)
	}
	addEvent(t, conn, "event-2")
	addEvent(t, conn, "event-3")
	if remaining, _ := service.Remaining("p"); remaining != 0 {
		t.Fatalf("remaining after two events=%d", remaining)
	}
	var count int
	if err := conn.QueryRow(`SELECT count(*) FROM event_notice_deliveries`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("deliveries=%d", count)
	}
}

func TestDrainSendsTemplateAndMarksCancelledEventSkipped(t *testing.T) {
	conn, service := fixture(t)
	if _, err := service.Grant("p", "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Grant("p", "second"); err != nil {
		t.Fatal(err)
	}
	addEvent(t, conn, "event-1")
	addEvent(t, conn, "event-2")
	if _, err := conn.Exec(`INSERT INTO event_cancellations(event_id,cancelled_at) VALUES('event-2','now')`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANYPITCH_WECHAT_APP_ID", "app")
	t.Setenv("ANYPITCH_WECHAT_APP_SECRET", "secret")
	sends := 0
	service.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/stable_token") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"token","expires_in":7200}`))}, nil
		}
		if !strings.HasSuffix(req.URL.Path, "/subscribe/send") {
			t.Fatalf("unexpected URL %s", req.URL)
		}
		sends++
		var body struct {
			To       string                       `json:"touser"`
			Template string                       `json:"template_id"`
			Page     string                       `json:"page"`
			Data     map[string]map[string]string `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.To != "openid" || body.Template != "template" || body.Page != "pages/event-detail/index?id=event-1" || body.Data["thing6"]["value"] != "周三训练" || body.Data["date2"]["value"] != "2026年10月28日 19:00" {
			t.Fatalf("unexpected payload: %+v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"errcode":0}`))}, nil
	})
	if err := service.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Fatalf("sends=%d", sends)
	}
	for eventID, expected := range map[string]string{"event-1": "sent", "event-2": "skipped"} {
		var status string
		if err := conn.QueryRow(`SELECT status FROM event_notice_deliveries WHERE event_id=?`, eventID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != expected {
			t.Fatalf("%s status=%s", eventID, status)
		}
	}
}
