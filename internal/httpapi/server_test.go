package httpapi_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"anypitch/internal/httpapi"
)

func TestCoachCanManagePlayersEventsAttendanceAndTactics(t *testing.T) {
	handler := newTestHandler(t)

	health := performJSONRequest(t, handler, http.MethodGet, "/api/healthz", "", nil)
	assertStatus(t, health, http.StatusOK)
	assertJSONEquals(t, health, "data.ok", true)

	register := performJSONRequest(t, handler, http.MethodPost, "/api/auth/register", "", map[string]any{
		"email":    "coach@example.com",
		"password": "correct horse battery staple",
	})
	assertStatus(t, register, http.StatusOK)
	token := jsonPath(t, register, "data.token").(string)
	if token == "" {
		t.Fatal("expected register token")
	}

	me := performJSONRequest(t, handler, http.MethodGet, "/api/auth/me", token, nil)
	assertStatus(t, me, http.StatusOK)
	assertJSONEquals(t, me, "data.user.email", "coach@example.com")

	player := performJSONRequest(t, handler, http.MethodPost, "/api/players", token, map[string]any{
		"name":      "林海",
		"number":    10,
		"positions": []string{"AM", "FW"},
	})
	assertStatus(t, player, http.StatusOK)
	playerID := jsonPath(t, player, "data.player.id").(string)

	training := performJSONRequest(t, handler, http.MethodPost, "/api/events", token, map[string]any{
		"type":      "training",
		"title":     "周三控球训练",
		"starts_at": "2026-05-13T20:00:00+08:00",
		"location":  "东区球场",
		"opponent":  "",
		"notes":     "小场压迫与转移",
	})
	assertStatus(t, training, http.StatusOK)
	eventID := jsonPath(t, training, "data.event.id").(string)

	attendance := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+eventID+"/attendance", token, map[string]any{
		"records": []map[string]any{
			{"player_id": playerID, "status": "available", "note": "准时"},
		},
	})
	assertStatus(t, attendance, http.StatusOK)
	assertJSONEquals(t, attendance, "data.records.0.status", "available")

	templates := performJSONRequest(t, handler, http.MethodGet, "/api/tactics/templates", token, nil)
	assertStatus(t, templates, http.StatusOK)
	assertJSONEquals(t, templates, "data.templates.0.format", float64(5))
	assertJSONEquals(t, templates, "data.templates.1.format", float64(8))
	assertJSONEquals(t, templates, "data.templates.2.format", float64(11))

	board := performJSONRequest(t, handler, http.MethodPost, "/api/tactics/boards", token, map[string]any{
		"name":      "五人制高位压迫",
		"format":    5,
		"formation": "1-2-1",
		"slots": []map[string]any{
			{"slot_id": "gk", "label": "GK", "x": 50, "y": 91, "player_id": playerID},
		},
	})
	assertStatus(t, board, http.StatusOK)
	assertJSONEquals(t, board, "data.board.format", float64(5))

	boards := performJSONRequest(t, handler, http.MethodGet, "/api/tactics/boards", token, nil)
	assertStatus(t, boards, http.StatusOK)
	assertJSONEquals(t, boards, "data.boards.0.name", "五人制高位压迫")

	logout := performJSONRequest(t, handler, http.MethodPost, "/api/auth/logout", token, nil)
	assertStatus(t, logout, http.StatusOK)
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "anypitch-test.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	handler, err := httpapi.NewHandler(conn)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	return handler
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("expected status %d, got %d: %s", want, response.Code, response.Body.String())
	}
}

func assertJSONEquals(t *testing.T, response *httptest.ResponseRecorder, path string, want any) {
	t.Helper()
	got := jsonPath(t, response, path)
	if got != want {
		t.Fatalf("expected %s = %#v, got %#v in %s", path, want, got, response.Body.String())
	}
}

func jsonPath(t *testing.T, response *httptest.ResponseRecorder, path string) any {
	t.Helper()

	var payload any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	current := payload
	for _, part := range splitPath(path) {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[part]
		case []any:
			index := int(part[0] - '0')
			current = typed[index]
		default:
			t.Fatalf("cannot descend into %T for %q in %s", current, part, response.Body.String())
		}
	}
	return current
}

func splitPath(path string) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	return append(parts, path[start:])
}
