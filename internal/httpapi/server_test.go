package httpapi_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"anypitch/internal/httpapi"
)

func TestCoachCanManagePlayersEventsAttendanceAndTactics(t *testing.T) {
	handler := newTestHandler(t)

	health := performJSONRequest(t, handler, http.MethodGet, "/api/healthz", "", nil)
	assertStatus(t, health, http.StatusOK)
	assertJSONEquals(t, health, "data.ok", true)

	registerAttempt := performJSONRequest(t, handler, http.MethodPost, "/api/auth/register", "", map[string]any{
		"email":    "someone@example.com",
		"password": "SomePassword2026",
	})
	assertStatus(t, registerAttempt, http.StatusNotFound)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	token := jsonPath(t, login, "data.token").(string)
	if token == "" {
		t.Fatal("expected login token")
	}

	me := performJSONRequest(t, handler, http.MethodGet, "/api/auth/me", token, nil)
	assertStatus(t, me, http.StatusOK)
	assertJSONEquals(t, me, "data.user.email", "coach@anypitch.local")
	if jsonPath(t, me, "data.team_id").(string) == "" {
		t.Fatal("expected default team id")
	}

	locations := performJSONRequest(t, handler, http.MethodGet, "/api/locations", token, nil)
	assertStatus(t, locations, http.StatusOK)
	assertJSONEquals(t, locations, "data.locations.0.name", "北京邮电大学（海淀校区）")

	location := performJSONRequest(t, handler, http.MethodPost, "/api/locations", token, map[string]any{
		"name": "北京邮电大学（沙河校区）",
	})
	assertStatus(t, location, http.StatusOK)
	locationID := jsonPath(t, location, "data.location.id").(string)
	assertJSONEquals(t, location, "data.location.name", "北京邮电大学（沙河校区）")

	deleteLocation := performJSONRequest(t, handler, http.MethodDelete, "/api/locations/"+locationID, token, nil)
	assertStatus(t, deleteLocation, http.StatusOK)

	player := performJSONRequest(t, handler, http.MethodPost, "/api/players", token, map[string]any{
		"name":      "林海",
		"number":    10,
		"positions": []string{"前腰", "前锋"},
	})
	assertStatus(t, player, http.StatusOK)
	playerID := jsonPath(t, player, "data.player.id").(string)
	assertJSONEquals(t, player, "data.player.positions.0", "前腰")

	training := performJSONRequest(t, handler, http.MethodPost, "/api/events", token, map[string]any{
		"type":      "training",
		"title":     "周三控球训练",
		"starts_at": "2026-05-13T20:00:00+08:00",
		"opponent":  "",
		"notes":     "小场压迫与转移",
	})
	assertStatus(t, training, http.StatusOK)
	eventID := jsonPath(t, training, "data.event.id").(string)
	assertJSONEquals(t, training, "data.event.ends_at", "2026-05-13T22:00:00+08:00")
	assertJSONEquals(t, training, "data.event.location", "北京邮电大学（海淀校区）")
	assertJSONEquals(t, training, "data.event.notes", "小场压迫与转移")

	attendance := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+eventID+"/attendance", token, map[string]any{
		"records": []map[string]any{
			{"player_id": playerID, "status": "available", "note": "准时"},
		},
	})
	assertStatus(t, attendance, http.StatusOK)
	assertJSONEquals(t, attendance, "data.records.0.status", "available")

	templates := performJSONRequest(t, handler, http.MethodGet, "/api/tactics/templates", token, nil)
	assertStatus(t, templates, http.StatusOK)
	assertJSONEquals(t, templates, "data.templates.0.id", "f5-121-press")
	assertJSONEquals(t, templates, "data.templates.0.format", float64(5))
	assertJSONEquals(t, templates, "data.templates.0.slots.0.label", "门将")
	assertJSONEquals(t, templates, "data.templates.1.format", float64(5))
	assertJSONEquals(t, templates, "data.templates.2.format", float64(8))
	assertJSONEquals(t, templates, "data.templates.4.format", float64(11))

	board := performJSONRequest(t, handler, http.MethodPost, "/api/tactics/boards", token, map[string]any{
		"name":                 "五人制高位压迫",
		"template_id":          "f5-121-press",
		"opponent_template_id": "f5-211-counter",
		"format":               5,
		"formation":            "1-2-1",
		"opponent_formation":   "2-1-1",
		"slots": []map[string]any{
			{"slot_id": "home:gk", "label": "门将", "x": 50, "y": 91, "player_id": playerID},
			{"slot_id": "opponent:gk", "label": "门将", "side": "opponent", "x": 50, "y": 9, "player_id": ""},
		},
	})
	assertStatus(t, board, http.StatusOK)
	assertJSONEquals(t, board, "data.board.format", float64(5))
	assertJSONEquals(t, board, "data.board.template_id", "f5-121-press")
	assertJSONEquals(t, board, "data.board.opponent_formation", "2-1-1")
	assertJSONEquals(t, board, "data.board.slots.0.side", "home")
	assertJSONEquals(t, board, "data.board.slots.1.side", "opponent")

	boards := performJSONRequest(t, handler, http.MethodGet, "/api/tactics/boards", token, nil)
	assertStatus(t, boards, http.StatusOK)
	assertJSONEquals(t, boards, "data.boards.0.name", "五人制高位压迫")

	logout := performJSONRequest(t, handler, http.MethodPost, "/api/auth/logout", token, nil)
	assertStatus(t, logout, http.StatusOK)
}

func TestPlayerCanLoginByNameAndOnlyManageOwnAttendance(t *testing.T) {
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	coachToken := jsonPath(t, login, "data.token").(string)

	player := performJSONRequest(t, handler, http.MethodPost, "/api/players", coachToken, map[string]any{
		"name":      "林海",
		"number":    10,
		"positions": []string{"前腰", "前锋"},
	})
	assertStatus(t, player, http.StatusOK)
	playerID := jsonPath(t, player, "data.player.id").(string)

	otherPlayer := performJSONRequest(t, handler, http.MethodPost, "/api/players", coachToken, map[string]any{
		"name":      "周舟",
		"number":    7,
		"positions": []string{"边锋"},
	})
	assertStatus(t, otherPlayer, http.StatusOK)
	otherPlayerID := jsonPath(t, otherPlayer, "data.player.id").(string)

	thirdPlayer := performJSONRequest(t, handler, http.MethodPost, "/api/players", coachToken, map[string]any{
		"name":      "小白",
		"number":    3,
		"positions": []string{"后卫"},
	})
	assertStatus(t, thirdPlayer, http.StatusOK)
	inactivePlayer := performJSONRequest(t, handler, http.MethodPost, "/api/players", coachToken, map[string]any{
		"name":      "停用队员",
		"number":    99,
		"positions": []string{"后勤"},
	})
	assertStatus(t, inactivePlayer, http.StatusOK)
	inactivePlayerID := jsonPath(t, inactivePlayer, "data.player.id").(string)
	inactiveStatus := performJSONRequest(t, handler, http.MethodPatch, "/api/players/"+inactivePlayerID, coachToken, map[string]any{
		"status": "inactive",
	})
	assertStatus(t, inactiveStatus, http.StatusOK)

	training := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type":      "training",
		"title":     "周三控球训练",
		"starts_at": "2026-05-13T20:00:00+08:00",
	})
	assertStatus(t, training, http.StatusOK)
	eventID := jsonPath(t, training, "data.event.id").(string)

	playerLogin := performJSONRequest(t, handler, http.MethodPost, "/api/player/login", "", map[string]any{
		"name": "林海",
	})
	assertStatus(t, playerLogin, http.StatusOK)
	playerToken := jsonPath(t, playerLogin, "data.token").(string)
	assertJSONEquals(t, playerLogin, "data.player.id", playerID)

	playerMe := performJSONRequest(t, handler, http.MethodGet, "/api/player/me", playerToken, nil)
	assertStatus(t, playerMe, http.StatusOK)
	assertJSONEquals(t, playerMe, "data.player.name", "林海")

	events := performJSONRequest(t, handler, http.MethodGet, "/api/player/events", playerToken, nil)
	assertStatus(t, events, http.StatusOK)
	assertJSONEquals(t, events, "data.events.0.title", "周三控球训练")

	coachStatus := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+eventID+"/attendance", coachToken, map[string]any{
		"records": []map[string]any{
			{"player_id": otherPlayerID, "status": "unavailable", "note": ""},
		},
	})
	assertStatus(t, coachStatus, http.StatusOK)

	status := performJSONRequest(t, handler, http.MethodPut, "/api/player/events/"+eventID+"/attendance", playerToken, map[string]any{
		"status": "tentative",
	})
	assertStatus(t, status, http.StatusOK)
	assertJSONEquals(t, status, "data.record.player_id", playerID)
	assertJSONEquals(t, status, "data.record.status", "tentative")

	ownStatus := performJSONRequest(t, handler, http.MethodGet, "/api/player/events/"+eventID+"/attendance", playerToken, nil)
	assertStatus(t, ownStatus, http.StatusOK)
	assertJSONEquals(t, ownStatus, "data.record.status", "tentative")
	assertJSONEquals(t, ownStatus, "data.summary.available", float64(0))
	assertJSONEquals(t, ownStatus, "data.summary.unavailable", float64(1))
	assertJSONEquals(t, ownStatus, "data.summary.tentative", float64(1))
	assertJSONEquals(t, ownStatus, "data.summary.unknown", float64(1))

	forbidden := performJSONRequest(t, handler, http.MethodGet, "/api/players", playerToken, nil)
	assertStatus(t, forbidden, http.StatusUnauthorized)
}

func TestPlayerEventsIncludeOwnAttendanceOverview(t *testing.T) {
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	coachToken := jsonPath(t, login, "data.token").(string)

	player := performJSONRequest(t, handler, http.MethodPost, "/api/players", coachToken, map[string]any{
		"name":      "林海",
		"number":    10,
		"positions": []string{"前腰"},
	})
	assertStatus(t, player, http.StatusOK)
	playerID := jsonPath(t, player, "data.player.id").(string)

	otherPlayer := performJSONRequest(t, handler, http.MethodPost, "/api/players", coachToken, map[string]any{
		"name":      "周舟",
		"number":    7,
		"positions": []string{"边锋"},
	})
	assertStatus(t, otherPlayer, http.StatusOK)
	otherPlayerID := jsonPath(t, otherPlayer, "data.player.id").(string)

	pastTraining := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type":      "training",
		"title":     "五一恢复训练",
		"starts_at": "2026-05-01T20:00:00+08:00",
	})
	assertStatus(t, pastTraining, http.StatusOK)
	pastEventID := jsonPath(t, pastTraining, "data.event.id").(string)

	futureTraining := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type":      "training",
		"title":     "周三控球训练",
		"starts_at": "2026-05-13T20:00:00+08:00",
	})
	assertStatus(t, futureTraining, http.StatusOK)
	futureEventID := jsonPath(t, futureTraining, "data.event.id").(string)

	unconfirmedFriendly := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type":      "friendly",
		"title":     "周末友谊赛",
		"starts_at": "2026-05-16T18:00:00+08:00",
	})
	assertStatus(t, unconfirmedFriendly, http.StatusOK)
	unconfirmedEventID := jsonPath(t, unconfirmedFriendly, "data.event.id").(string)

	pastAttendance := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+pastEventID+"/attendance", coachToken, map[string]any{
		"records": []map[string]any{
			{"player_id": playerID, "status": "available", "note": ""},
		},
	})
	assertStatus(t, pastAttendance, http.StatusOK)

	futureAttendance := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+futureEventID+"/attendance", coachToken, map[string]any{
		"records": []map[string]any{
			{"player_id": playerID, "status": "tentative", "note": ""},
			{"player_id": otherPlayerID, "status": "unavailable", "note": ""},
		},
	})
	assertStatus(t, futureAttendance, http.StatusOK)

	otherOnlyAttendance := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+unconfirmedEventID+"/attendance", coachToken, map[string]any{
		"records": []map[string]any{
			{"player_id": otherPlayerID, "status": "unavailable", "note": ""},
		},
	})
	assertStatus(t, otherOnlyAttendance, http.StatusOK)

	playerLogin := performJSONRequest(t, handler, http.MethodPost, "/api/player/login", "", map[string]any{
		"name": "林海",
	})
	assertStatus(t, playerLogin, http.StatusOK)
	playerToken := jsonPath(t, playerLogin, "data.token").(string)

	response := performJSONRequest(t, handler, http.MethodGet, "/api/player/events", playerToken, nil)
	assertStatus(t, response, http.StatusOK)

	var payload struct {
		Data struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
			AttendanceRecords []struct {
				EventID  string `json:"event_id"`
				PlayerID string `json:"player_id"`
				Status   string `json:"status"`
			} `json:"attendance_records"`
			AttendanceSummary struct {
				Available   int `json:"available"`
				Unavailable int `json:"unavailable"`
				Tentative   int `json:"tentative"`
				Unknown     int `json:"unknown"`
			} `json:"attendance_summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode player events overview: %v; body=%s", err, response.Body.String())
	}

	if len(payload.Data.Events) != 3 {
		t.Fatalf("expected 3 player events, got %d in %s", len(payload.Data.Events), response.Body.String())
	}
	if len(payload.Data.AttendanceRecords) != 2 {
		t.Fatalf("expected only current player's 2 attendance records, got %d in %s", len(payload.Data.AttendanceRecords), response.Body.String())
	}

	recordsByEvent := map[string]string{}
	for _, record := range payload.Data.AttendanceRecords {
		if record.PlayerID != playerID {
			t.Fatalf("leaked attendance for player %q in %s", record.PlayerID, response.Body.String())
		}
		recordsByEvent[record.EventID] = record.Status
	}
	if recordsByEvent[pastEventID] != "available" {
		t.Fatalf("expected past event status available, got %#v in %s", recordsByEvent[pastEventID], response.Body.String())
	}
	if recordsByEvent[futureEventID] != "tentative" {
		t.Fatalf("expected future event status tentative, got %#v in %s", recordsByEvent[futureEventID], response.Body.String())
	}
	if _, ok := recordsByEvent[unconfirmedEventID]; ok {
		t.Fatalf("expected unconfirmed event to have no saved player record in %s", response.Body.String())
	}
	if payload.Data.AttendanceSummary.Available != 1 ||
		payload.Data.AttendanceSummary.Unavailable != 0 ||
		payload.Data.AttendanceSummary.Tentative != 1 ||
		payload.Data.AttendanceSummary.Unknown != 1 {
		t.Fatalf("unexpected attendance summary: %#v in %s", payload.Data.AttendanceSummary, response.Body.String())
	}
}

func TestDefaultCoachPasswordCanComeFromEnvironment(t *testing.T) {
	t.Setenv("ANYPITCH_COACH_PASSWORD", "CoachSecret@2026")
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "CoachSecret@2026",
	})
	assertStatus(t, login, http.StatusOK)
	if jsonPath(t, login, "data.token").(string) == "" {
		t.Fatal("expected login token")
	}
}

func TestCoachBootstrapEndpointsCanLoadConcurrentlyAfterPasswordLogin(t *testing.T) {
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	token := jsonPath(t, login, "data.token").(string)

	paths := []string{
		"/api/auth/me",
		"/api/players",
		"/api/events",
		"/api/locations",
		"/api/tactics/templates",
		"/api/tactics/boards",
	}
	failures := make(chan string, len(paths)*8)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, path := range paths {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				<-start
				response := performJSONRequest(t, handler, http.MethodGet, path, token, nil)
				if response.Code != http.StatusOK {
					failures <- fmt.Sprintf("%s returned %d: %s", path, response.Code, response.Body.String())
				}
			}(path)
		}
	}
	close(start)
	wg.Wait()
	close(failures)

	for failure := range failures {
		t.Error(failure)
	}
}

func TestDefaultCoachEnvironmentPasswordRotatesExistingCoach(t *testing.T) {
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
	oldLogin := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, oldLogin, http.StatusOK)

	t.Setenv("ANYPITCH_COACH_PASSWORD", "RotatedCoach@2026")
	handler, err = httpapi.NewHandler(conn)
	if err != nil {
		t.Fatalf("new handler after rotation: %v", err)
	}
	newLogin := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "RotatedCoach@2026",
	})
	assertStatus(t, newLogin, http.StatusOK)

	staleLogin := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, staleLogin, http.StatusBadRequest)
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
