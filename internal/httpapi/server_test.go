package httpapi_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

	player := createFixturePlayer(t, handler, map[string]any{
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

	lockedPlayerDelete := performJSONRequest(t, handler, http.MethodDelete, "/api/players/"+playerID, token, nil)
	assertStatus(t, lockedPlayerDelete, http.StatusConflict)

	logout := performJSONRequest(t, handler, http.MethodPost, "/api/auth/logout", token, nil)
	assertStatus(t, logout, http.StatusOK)
}

func TestNewWeChatMemberCanBrowseBeforeCompletingProfile(t *testing.T) {
	handler := newTestHandler(t)
	coach := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	coachToken := jsonPath(t, coach, "data.token").(string)
	event := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type": "friendly", "title": "周末友谊赛", "starts_at": "2099-10-13T20:00:00+08:00",
	})
	assertStatus(t, event, http.StatusOK)
	eventID := jsonPath(t, event, "data.event.id").(string)
	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": "new-member"})
	assertStatus(t, login, http.StatusOK)
	token := jsonPath(t, login, "data.token").(string)
	playerID := jsonPath(t, login, "data.player.id").(string)
	assertJSONEquals(t, login, "data.player.profile_complete", false)
	detail := performJSONRequest(t, handler, http.MethodGet, "/api/player/events/"+eventID, token, nil)
	assertStatus(t, detail, http.StatusOK)
	assertJSONEquals(t, detail, "data.record.status", "unknown")
	if members := jsonPath(t, detail, "data.members").([]any); len(members) != 0 {
		t.Fatalf("unfinished profile appeared in event roster: %v", members)
	}
	blocked := performJSONRequest(t, handler, http.MethodPut, "/api/player/events/"+eventID+"/attendance", token, map[string]any{"status": "available"})
	assertStatus(t, blocked, http.StatusForbidden)
	assertJSONEquals(t, blocked, "error.code", "profile_incomplete")
	coachSignup := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+eventID+"/attendance", coachToken, map[string]any{"records": []map[string]any{{"player_id": playerID, "status": "available"}}})
	assertStatus(t, coachSignup, http.StatusBadRequest)
	assertStatus(t, performJSONRequest(t, handler, http.MethodPost, "/api/player/events", token, map[string]any{"type": "pickup", "title": "周末野球", "starts_at": "2026-10-14T20:00:00+08:00"}), http.StatusForbidden)
	profile := performJSONRequest(t, handler, http.MethodPatch, "/api/player/profile", token, map[string]any{"name": "新队员"})
	assertStatus(t, profile, http.StatusOK)
	detail = performJSONRequest(t, handler, http.MethodGet, "/api/player/events/"+eventID, token, nil)
	assertJSONEquals(t, detail, "data.members.0.name", "新队员")
	assertJSONEquals(t, detail, "data.members.0.status", "unknown")
	assertStatus(t, performJSONRequest(t, handler, http.MethodPost, "/api/player/events", token, map[string]any{"type": "training", "title": "越权训练", "starts_at": "2026-10-14T20:00:00+08:00"}), http.StatusForbidden)
	pickup := performJSONRequest(t, handler, http.MethodPost, "/api/player/events", token, map[string]any{"type": "pickup", "title": "周末野球", "starts_at": "2026-10-14T20:00:00+08:00"})
	assertStatus(t, pickup, http.StatusForbidden)
	assertJSONEquals(t, pickup, "error.code", "forbidden")
	joined := performJSONRequest(t, handler, http.MethodPut, "/api/player/events/"+eventID+"/attendance", token, map[string]any{"status": "available"})
	assertStatus(t, joined, http.StatusOK)
	assertJSONEquals(t, joined, "data.record.status", "available")
}

func TestPlayerCanRecordOneTimeNewEventReminderAuthorization(t *testing.T) {
	t.Setenv("ANYPITCH_WECHAT_NEW_EVENT_TEMPLATE_ID", "template")
	handler := newTestHandler(t)
	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": "notice-member"})
	assertStatus(t, login, http.StatusOK)
	token := jsonPath(t, login, "data.token").(string)
	state := performJSONRequest(t, handler, http.MethodGet, "/api/player/notifications", token, nil)
	assertStatus(t, state, http.StatusOK)
	assertJSONEquals(t, state, "data.template_id", "template")
	assertJSONEquals(t, state, "data.remaining", float64(0))
	for i := 0; i < 2; i++ {
		grant := performJSONRequest(t, handler, http.MethodPost, "/api/player/notifications", token, map[string]any{"request_id": "accepted-once"})
		assertStatus(t, grant, http.StatusOK)
		assertJSONEquals(t, grant, "data.remaining", float64(1))
	}
}

func TestWeChatMemberOnlyManagesOwnAttendance(t *testing.T) {
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	coachToken := jsonPath(t, login, "data.token").(string)

	player := createFixturePlayer(t, handler, map[string]any{
		"name":      "林海",
		"number":    10,
		"positions": []string{"前腰", "前锋"},
	})
	assertStatus(t, player, http.StatusOK)
	playerID := jsonPath(t, player, "data.player.id").(string)

	otherPlayer := createFixturePlayer(t, handler, map[string]any{
		"name":      "周舟",
		"number":    7,
		"positions": []string{"边锋"},
	})
	assertStatus(t, otherPlayer, http.StatusOK)
	otherPlayerID := jsonPath(t, otherPlayer, "data.player.id").(string)

	thirdPlayer := createFixturePlayer(t, handler, map[string]any{
		"name":      "小白",
		"number":    3,
		"positions": []string{"后卫"},
	})
	assertStatus(t, thirdPlayer, http.StatusOK)
	inactivePlayer := createFixturePlayer(t, handler, map[string]any{
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
		"starts_at": "2099-05-13T20:00:00+08:00",
	})
	assertStatus(t, training, http.StatusOK)
	eventID := jsonPath(t, training, "data.event.id").(string)

	playerLogin := performJSONRequest(t, handler, http.MethodPost, "/api/auth/wechat", "", map[string]any{
		"code": playerID,
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

	detail := performJSONRequest(t, handler, http.MethodGet, "/api/player/events/"+eventID, playerToken, nil)
	assertStatus(t, detail, http.StatusOK)
	assertJSONEquals(t, detail, "data.event.title", "周三控球训练")
	assertJSONEquals(t, detail, "data.record.status", "tentative")
	assertJSONEquals(t, detail, "data.members.0.name", "林海")
	assertJSONEquals(t, detail, "data.members.0.status", "tentative")
	assertJSONEquals(t, detail, "data.members.1.name", "周舟")
	assertJSONEquals(t, detail, "data.members.1.status", "unavailable")
	assertJSONEquals(t, detail, "data.members.2.name", "小白")
	assertJSONEquals(t, detail, "data.members.2.status", "unknown")
	assertJSONEquals(t, detail, "data.album", nil)

	forbidden := performJSONRequest(t, handler, http.MethodGet, "/api/players", playerToken, nil)
	assertStatus(t, forbidden, http.StatusUnauthorized)
}

func TestMemberProfileIsTeamScopedAndUsesTrainingLeaderboardCount(t *testing.T) {
	h := newTestHandler(t)
	first := createFixturePlayer(t, h, map[string]any{"name": "小王"})
	second := createFixturePlayer(t, h, map[string]any{"name": "小李"})
	firstID := jsonPath(t, first, "data.player.id").(string)
	secondID := jsonPath(t, second, "data.player.id").(string)
	memberLogin := performJSONRequest(t, h, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": firstID})
	memberToken := jsonPath(t, memberLogin, "data.token").(string)
	coachLogin := performJSONRequest(t, h, http.MethodPost, "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	coachToken := jsonPath(t, coachLogin, "data.token").(string)
	for _, eventType := range []string{"training", "friendly", "pickup"} {
		event := performJSONRequest(t, h, http.MethodPost, "/api/events", coachToken, map[string]any{"type": eventType, "title": "队内活动", "starts_at": "2026-10-22T19:00:00+08:00"})
		assertStatus(t, event, http.StatusOK)
		id := jsonPath(t, event, "data.event.id").(string)
		assertStatus(t, performJSONRequest(t, h, http.MethodPut, "/api/events/"+id+"/attendance", coachToken, map[string]any{"records": []map[string]any{{"player_id": secondID, "status": "available"}}}), http.StatusOK)
	}
	profile := performJSONRequest(t, h, http.MethodGet, "/api/player/members/"+secondID, memberToken, nil)
	assertStatus(t, profile, http.StatusOK)
	assertJSONEquals(t, profile, "data.member.name", "小李")
	assertJSONEquals(t, profile, "data.member.participation_count", float64(2))
	if _, err := h.(*fixtureHandler).db.Exec(`INSERT INTO players (id,team_id,name,positions_json,status,created_at,updated_at) SELECT 'legacy-member',team_id,'老队员','[]','active',created_at,updated_at FROM players WHERE id=?`, firstID); err != nil {
		t.Fatal(err)
	}
	legacy := performJSONRequest(t, h, http.MethodGet, "/api/player/members/legacy-member", memberToken, nil)
	assertStatus(t, legacy, http.StatusOK)
	assertJSONEquals(t, legacy, "data.member.name", "老队员")
	assertJSONEquals(t, legacy, "data.member.has_avatar", false)
	assertStatus(t, performJSONRequest(t, h, http.MethodGet, "/api/player/members/"+secondID, "", nil), http.StatusUnauthorized)
	assertStatus(t, performJSONRequest(t, h, http.MethodGet, "/api/player/members/missing", memberToken, nil), http.StatusNotFound)
}

func TestPersonalHistoryUsesCurrentDatabaseStatusAndCoachCanEditPastEvents(t *testing.T) {
	h := newTestHandler(t)
	coach := performJSONRequest(t, h, http.MethodPost, "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	coachToken := jsonPath(t, coach, "data.token").(string)
	joinedAt := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	createEvent := func(title string, startsAt time.Time) string {
		t.Helper()
		response := performJSONRequest(t, h, http.MethodPost, "/api/events", coachToken, map[string]any{
			"type": "training", "title": title, "starts_at": startsAt.Format(time.RFC3339),
		})
		assertStatus(t, response, http.StatusOK)
		return jsonPath(t, response, "data.event.id").(string)
	}
	beforeID := createEvent("入队前活动", joinedAt.Add(-24*time.Hour))
	afterID := createEvent("入队后活动", joinedAt.Add(12*time.Hour))
	unknownID := createEvent("未确认活动", joinedAt.Add(24*time.Hour))

	member := createFixturePlayer(t, h, map[string]any{"name": "队员"})
	assertStatus(t, member, http.StatusOK)
	playerID := jsonPath(t, member, "data.player.id").(string)
	if _, err := h.(*fixtureHandler).db.Exec(`UPDATE players SET created_at = ? WHERE id = ?`, joinedAt.Format(time.RFC3339), playerID); err != nil {
		t.Fatal(err)
	}
	login := performJSONRequest(t, h, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": playerID})
	assertStatus(t, login, http.StatusOK)
	playerToken := jsonPath(t, login, "data.token").(string)

	setStatus := func(eventID, status string) {
		t.Helper()
		response := performJSONRequest(t, h, http.MethodPut, "/api/events/"+eventID+"/attendance", coachToken, map[string]any{
			"records": []map[string]any{{"player_id": playerID, "status": status}},
		})
		assertStatus(t, response, http.StatusOK)
	}
	setStatus(beforeID, "available")
	setStatus(afterID, "tentative")
	setStatus(unknownID, "unknown")

	all := performJSONRequest(t, h, http.MethodGet, "/api/player/events", playerToken, nil)
	assertStatus(t, all, http.StatusOK)
	if got := len(jsonPath(t, all, "data.events").([]any)); got != 3 {
		t.Fatalf("expected complete team history, got %d events", got)
	}
	personal := performJSONRequest(t, h, http.MethodGet, "/api/player/stats/me", playerToken, nil)
	assertStatus(t, personal, http.StatusOK)
	if got := len(jsonPath(t, personal, "data.history").([]any)); got != 1 {
		t.Fatalf("expected one post-join confirmed event, got %d: %s", got, personal.Body.String())
	}
	assertJSONEquals(t, personal, "data.history.0.event_id", afterID)

	edited := performJSONRequest(t, h, http.MethodPatch, "/api/events/"+afterID, coachToken, map[string]any{"title": "教练修改的历史活动"})
	assertStatus(t, edited, http.StatusOK)
	setStatus(afterID, "unknown")
	personal = performJSONRequest(t, h, http.MethodGet, "/api/player/stats/me", playerToken, nil)
	assertStatus(t, personal, http.StatusOK)
	if got := len(jsonPath(t, personal, "data.history").([]any)); got != 0 {
		t.Fatalf("expected current unknown status to disappear, got %s", personal.Body.String())
	}
	setStatus(afterID, "available")
	personal = performJSONRequest(t, h, http.MethodGet, "/api/player/stats/me", playerToken, nil)
	assertJSONEquals(t, personal, "data.history.0.title", "教练修改的历史活动")
	assertJSONEquals(t, personal, "data.history.0.rsvp_status", "available")

	blocked := performJSONRequest(t, h, http.MethodPut, "/api/player/events/"+afterID+"/attendance", playerToken, map[string]any{"status": "unavailable"})
	assertStatus(t, blocked, http.StatusConflict)
	assertJSONEquals(t, blocked, "error.code", "event_ended")
}

func TestPlayerEventsIncludeOwnAttendanceOverview(t *testing.T) {
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	coachToken := jsonPath(t, login, "data.token").(string)

	player := createFixturePlayer(t, handler, map[string]any{
		"name":      "林海",
		"number":    10,
		"positions": []string{"前腰"},
	})
	assertStatus(t, player, http.StatusOK)
	playerID := jsonPath(t, player, "data.player.id").(string)

	otherPlayer := createFixturePlayer(t, handler, map[string]any{
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

	playerLogin := performJSONRequest(t, handler, http.MethodPost, "/api/auth/wechat", "", map[string]any{
		"code": playerID,
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

func TestCoachCanUpdateEventDateTime(t *testing.T) {
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	coachToken := jsonPath(t, login, "data.token").(string)

	training := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type":      "training",
		"title":     "周三控球训练",
		"starts_at": "2026-05-13T20:00:00+08:00",
		"ends_at":   "2026-05-13T22:00:00+08:00",
	})
	assertStatus(t, training, http.StatusOK)
	eventID := jsonPath(t, training, "data.event.id").(string)

	update := performJSONRequest(t, handler, http.MethodPatch, "/api/events/"+eventID, coachToken, map[string]any{
		"starts_at": "2026-05-14T19:30:00+08:00",
		"ends_at":   "2026-05-14T21:30:00+08:00",
		"notes":     "改到周四晚",
	})
	assertStatus(t, update, http.StatusOK)
	assertJSONEquals(t, update, "data.event.starts_at", "2026-05-14T19:30:00+08:00")
	assertJSONEquals(t, update, "data.event.ends_at", "2026-05-14T21:30:00+08:00")
	assertJSONEquals(t, update, "data.event.notes", "改到周四晚")

	invalid := performJSONRequest(t, handler, http.MethodPatch, "/api/events/"+eventID, coachToken, map[string]any{
		"starts_at": "2026-05-15T20:00:00+08:00",
		"ends_at":   "2026-05-15T19:00:00+08:00",
	})
	assertStatus(t, invalid, http.StatusBadRequest)
	assertJSONEquals(t, invalid, "error.code", "invalid_event")
}

func TestCoachCanDeleteEvent(t *testing.T) {
	handler := newTestHandler(t)

	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{
		"password": "AnyPitch@2026",
	})
	assertStatus(t, login, http.StatusOK)
	coachToken := jsonPath(t, login, "data.token").(string)

	training := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type":      "training",
		"title":     "待删除训练",
		"starts_at": "2026-05-13T20:00:00+08:00",
		"ends_at":   "2026-05-13T22:00:00+08:00",
	})
	assertStatus(t, training, http.StatusOK)
	eventID := jsonPath(t, training, "data.event.id").(string)

	deleted := performJSONRequest(t, handler, http.MethodDelete, "/api/events/"+eventID, coachToken, nil)
	assertStatus(t, deleted, http.StatusOK)
	assertJSONEquals(t, deleted, "data.ok", true)

	events := performJSONRequest(t, handler, http.MethodGet, "/api/events", coachToken, nil)
	assertStatus(t, events, http.StatusOK)
	if got := jsonPath(t, events, "data.events"); len(got.([]any)) != 0 {
		t.Fatalf("expected deleted event to be absent, got %s", events.Body.String())
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

func TestTrainingSignupsRankMembersAndPhotosRemainPrivate(t *testing.T) {
	handler := newTestHandler(t)
	login := performJSONRequest(t, handler, http.MethodPost, "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	assertStatus(t, login, http.StatusOK)
	coachToken := jsonPath(t, login, "data.token").(string)

	first := createFixturePlayer(t, handler, map[string]any{"name": "林海", "number": 10, "positions": []string{"前腰"}})
	second := createFixturePlayer(t, handler, map[string]any{"name": "周舟", "number": 7, "positions": []string{"边锋"}})
	assertStatus(t, first, http.StatusOK)
	assertStatus(t, second, http.StatusOK)
	firstID := jsonPath(t, first, "data.player.id").(string)
	secondID := jsonPath(t, second, "data.player.id").(string)

	training := performJSONRequest(t, handler, http.MethodPost, "/api/events", coachToken, map[string]any{
		"type": "training", "title": "九月传接训练", "starts_at": "2026-09-01T19:00:00+08:00", "ends_at": "2026-09-01T21:00:00+08:00",
	})
	assertStatus(t, training, http.StatusOK)
	eventID := jsonPath(t, training, "data.event.id").(string)

	rsvp := performJSONRequest(t, handler, http.MethodPut, "/api/events/"+eventID+"/attendance", coachToken, map[string]any{"records": []map[string]any{
		{"player_id": firstID, "status": "available", "note": ""},
		{"player_id": secondID, "status": "unavailable", "note": ""},
	}})
	assertStatus(t, rsvp, http.StatusOK)

	removed := performJSONRequest(t, handler, http.MethodGet, "/api/events/"+eventID+"/checkin", coachToken, nil)
	assertStatus(t, removed, http.StatusNotFound)
	playerLogin := performJSONRequest(t, handler, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": firstID})
	assertStatus(t, playerLogin, http.StatusOK)
	playerToken := jsonPath(t, playerLogin, "data.token").(string)

	term := performJSONRequest(t, handler, http.MethodPost, "/api/terms", coachToken, map[string]any{"name": "2026年秋季学期", "starts_on": "2026-09-01", "ends_on": "2027-01-31"})
	assertStatus(t, term, http.StatusOK)
	termID := jsonPath(t, term, "data.term.id").(string)
	monthRank := performJSONRequest(t, handler, http.MethodGet, "/api/stats/leaderboard?month=2026-09", coachToken, nil)
	assertStatus(t, monthRank, http.StatusOK)
	assertJSONEquals(t, monthRank, "data.leaderboard.entries.0.name", "林海")
	assertJSONEquals(t, monthRank, "data.leaderboard.entries.0.count", float64(1))
	termRank := performJSONRequest(t, handler, http.MethodGet, "/api/stats/leaderboard?term_id="+termID, coachToken, nil)
	assertStatus(t, termRank, http.StatusOK)
	assertJSONEquals(t, termRank, "data.leaderboard.entries.0.count", float64(1))

	special := performJSONRequest(t, handler, http.MethodPost, "/api/special-activities", coachToken, map[string]any{"name": "校长杯", "starts_on": "2026-09-10", "ends_on": "2026-09-30", "summary": "秋季专题"})
	assertStatus(t, special, http.StatusOK)
	specialID := jsonPath(t, special, "data.activity.id").(string)
	entry := performJSONRequest(t, handler, http.MethodPost, "/api/special-activities/"+specialID+"/entries", coachToken, map[string]any{"title": "小组赛第一轮", "occurs_at": "2026-09-18T18:00:00+08:00", "location": "西土城操场", "opponent": "软件学院", "home_score": 2, "away_score": 1, "notes": ""})
	assertStatus(t, entry, http.StatusOK)

	album := performJSONRequest(t, handler, http.MethodPost, "/api/albums", coachToken, map[string]any{"special_activity_id": specialID})
	assertStatus(t, album, http.StatusOK)
	albumID := jsonPath(t, album, "data.album.id").(string)
	upload := performImageUpload(t, handler, "/api/albums/"+albumID+"/photos", coachToken)
	assertStatus(t, upload, http.StatusOK)
	photoID := jsonPath(t, upload, "data.photo.id").(string)
	photo := performJSONRequest(t, handler, http.MethodGet, "/api/photos/"+photoID+"?variant=thumb", coachToken, nil)
	if photo.Code != http.StatusOK || photo.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("expected private JPEG photo, got %d %q", photo.Code, photo.Header().Get("Content-Type"))
	}
	playerAlbums := performJSONRequest(t, handler, http.MethodGet, "/api/player/albums", playerToken, nil)
	assertStatus(t, playerAlbums, http.StatusOK)
	assertJSONEquals(t, playerAlbums, "data.albums.0.photo_count", float64(1))
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "anypitch-test.db")
	t.Setenv("ANYPITCH_UPLOAD_DIR", filepath.Join(t.TempDir(), "uploads"))
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
	t.Setenv("ANYPITCH_WECHAT_APP_ID", "test-app")
	t.Setenv("ANYPITCH_WECHAT_APP_SECRET", "test-secret")
	t.Setenv("ANYPITCH_PUBLIC_URL", "https://example.test")
	t.Setenv("ANYPITCH_WECHAT_CALLBACK_TOKEN", "test-callback-token")
	previous := http.DefaultTransport
	http.DefaultTransport = weChatTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api.weixin.qq.com" && r.URL.Path != "/sns/jscode2session" {
			var input map[string]any
			json.NewDecoder(r.Body).Decode(&input)
			var response any
			switch r.URL.Path {
			case "/cgi-bin/stable_token":
				response = map[string]any{"access_token": "test-token", "expires_in": 7200}
			case "/wxa/msg_sec_check":
				suggest := "pass"
				if strings.Contains(fmt.Sprint(input["content"]), "违规测试") {
					suggest = "risky"
				}
				response = map[string]any{"errcode": 0, "result": map[string]any{"suggest": suggest}}
			case "/wxa/media_check_async":
				response = map[string]any{"errcode": 0, "trace_id": strings.TrimPrefix(fmt.Sprint(input["media_url"]), "https://example.test/api/wechat/review-media/")}
			default:
				t.Fatalf("unexpected WeChat API: %s", r.URL.Path)
			}
			payload, _ := json.Marshal(response)
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload))}, nil
		}
		if r.URL.Host != "api.weixin.qq.com" || r.URL.Path != "/sns/jscode2session" {
			return previous.RoundTrip(r)
		}
		payload, _ := json.Marshal(map[string]any{"openid": r.URL.Query().Get("js_code"), "session_key": "never-return-this"})
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previous })
	return &fixtureHandler{Handler: handler, db: conn}
}

type fixtureHandler struct {
	http.Handler
	db *sql.DB
}
type weChatTransport func(*http.Request) (*http.Response, error)

func (f weChatTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func createFixturePlayer(t *testing.T, h http.Handler, input map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	code := uuid.NewString()
	login := performJSONRequest(t, h, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": code})
	assertStatus(t, login, 200)
	id := jsonPath(t, login, "data.player.id").(string)
	token := jsonPath(t, login, "data.token").(string)
	// Fixture uses member ID as its upstream ID so tests can log in again.
	if _, err := h.(*fixtureHandler).db.Exec(`UPDATE wechat_members SET open_id=? WHERE player_id=?`, id, id); err != nil {
		t.Fatal(err)
	}
	profile := performJSONRequest(t, h, http.MethodPatch, "/api/player/profile", token, map[string]any{"name": input["name"]})
	assertStatus(t, profile, 200)
	coach := performJSONRequest(t, h, http.MethodPost, "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	return performJSONRequest(t, h, http.MethodPatch, "/api/players/"+id, jsonPath(t, coach, "data.token").(string), input)
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

func performImageUpload(t *testing.T, handler http.Handler, requestPath, token string) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	file, err := writer.CreateFormFile("file", "training.png")
	if err != nil {
		t.Fatalf("create upload field: %v", err)
	}
	imageValue := image.NewRGBA(image.Rect(0, 0, 24, 16))
	imageValue.Set(0, 0, color.RGBA{R: 22, G: 130, B: 71, A: 255})
	if err := png.Encode(file, imageValue); err != nil {
		t.Fatalf("encode image: %v", err)
	}
	if err := writer.WriteField("caption", "训练结束后的合照"); err != nil {
		t.Fatalf("write caption: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, requestPath, &payload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+token)
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

func TestWeChatIdentityRoleAndRevocation(t *testing.T) {
	h := newTestHandler(t)
	login := func(code string) *httptest.ResponseRecorder {
		return performJSONRequest(t, h, "POST", "/api/auth/wechat", "", map[string]any{"code": code})
	}
	first := login("alice")
	assertStatus(t, first, 200)
	token := jsonPath(t, first, "data.token").(string)
	id := jsonPath(t, first, "data.player.id").(string)
	again := login("alice")
	assertStatus(t, again, 200)
	assertJSONEquals(t, again, "data.player.id", id)
	second := login("bob")
	assertStatus(t, second, 200)
	other := jsonPath(t, second, "data.player.id").(string)
	otherToken := jsonPath(t, second, "data.token").(string)
	if id == other {
		t.Fatal("different WeChat accounts share a member")
	}
	for _, tok := range []string{token, otherToken} {
		assertStatus(t, performJSONRequest(t, h, "PATCH", "/api/player/profile", tok, map[string]any{"name": "同名队友"}), 200)
	}
	for _, hidden := range []string{"openid", "open_id", "session_key", "never-return-this", "test-secret"} {
		if strings.Contains(first.Body.String(), hidden) {
			t.Fatalf("leaked %s", hidden)
		}
	}
	assertStatus(t, performJSONRequest(t, h, "POST", "/api/player/login", "", map[string]any{"name": "同名队友"}), 404)
	assertStatus(t, performJSONRequest(t, h, "GET", "/api/events", token, nil), 401)
	conn := h.(*fixtureHandler).db
	if _, err := conn.Exec(`UPDATE wechat_members SET role='owner' WHERE player_id=?`, id); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, performJSONRequest(t, h, "PATCH", "/api/coach/members/"+other, token, map[string]any{"role": "coach"}), 200)
	assertStatus(t, performJSONRequest(t, h, "GET", "/api/events", otherToken, nil), 200)
	assertStatus(t, performJSONRequest(t, h, "PATCH", "/api/coach/members/"+id, otherToken, map[string]any{"role": "member"}), 403)
	assertStatus(t, performJSONRequest(t, h, "PATCH", "/api/coach/members/"+other, token, map[string]any{"role": "owner"}), 400)
	assertStatus(t, performJSONRequest(t, h, "PATCH", "/api/coach/members/"+other, token, map[string]any{"role": "member"}), 200)
	assertStatus(t, performJSONRequest(t, h, "GET", "/api/events", otherToken, nil), 401)
	assertStatus(t, performJSONRequest(t, h, "GET", "/api/player/events", otherToken, nil), 200)
	if _, err := conn.Exec(`UPDATE players SET status='inactive' WHERE id=?`, other); err != nil {
		t.Fatal(err)
	}
	assertStatus(t, login("bob"), 401)
	assertStatus(t, performJSONRequest(t, h, "GET", "/api/player/events", otherToken, nil), 401)
	t.Setenv("ANYPITCH_WECHAT_APP_SECRET", "")
	assertStatus(t, login("new"), 503)
}

func TestTrainingRankingCalendarBoundariesAndSignupChanges(t *testing.T) {
	h := newTestHandler(t)
	p := createFixturePlayer(t, h, map[string]any{"name": "球友"})
	assertStatus(t, p, 200)
	id := jsonPath(t, p, "data.player.id").(string)
	login := performJSONRequest(t, h, "POST", "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	token := jsonPath(t, login, "data.token").(string)
	cases := []struct {
		date, kind, status string
		cancel             bool
	}{
		{"2025-12-31T15:59:59Z", "training", "available", false},
		{"2025-12-31T16:00:00Z", "training", "available", false},
		{"2026-06-30T15:59:59Z", "training", "available", false},
		{"2026-06-30T16:00:00Z", "training", "available", false},
		{"2026-12-31T16:00:00Z", "training", "available", false},
		{"2026-08-01T12:00:00Z", "pickup", "available", false},
		{"2026-08-02T12:00:00Z", "friendly", "available", false},
		{"2026-08-03T12:00:00Z", "training", "tentative", false},
		{"2026-08-04T12:00:00Z", "training", "available", true},
		{"2026-08-05T12:00:00Z", "friendly", "available", true},
	}
	var july string
	for i, c := range cases {
		e := performJSONRequest(t, h, "POST", "/api/events", token, map[string]any{"type": c.kind, "title": fmt.Sprintf("活动%d", i), "starts_at": c.date})
		assertStatus(t, e, 200)
		eid := jsonPath(t, e, "data.event.id").(string)
		if i == 3 {
			july = eid
		}
		a := performJSONRequest(t, h, "PUT", "/api/events/"+eid+"/attendance", token, map[string]any{"records": []map[string]any{{"player_id": id, "status": c.status}}})
		assertStatus(t, a, 200)
		if c.cancel {
			assertStatus(t, performJSONRequest(t, h, "POST", "/api/events/"+eid+"/cancel", token, nil), 200)
		}
	}
	for query, count := range map[string]int{"year=2026": 4, "year=2026&half=1": 2, "year=2026&half=2": 2, "month=2026-07": 1} {
		r := performJSONRequest(t, h, "GET", "/api/stats/leaderboard?"+query, token, nil)
		assertStatus(t, r, 200)
		assertJSONEquals(t, r, "data.leaderboard.entries.0.count", float64(count))
	}
	for _, q := range []string{"year=2026&half=3", "half=1", "year=2026&month=2026-01", "year=no"} {
		assertStatus(t, performJSONRequest(t, h, "GET", "/api/stats/leaderboard?"+q, token, nil), 400)
	}
	a := performJSONRequest(t, h, "PUT", "/api/events/"+july+"/attendance", token, map[string]any{"records": []map[string]any{{"player_id": id, "status": "unavailable"}}})
	assertStatus(t, a, 200)
	r := performJSONRequest(t, h, "GET", "/api/stats/leaderboard?year=2026&half=2", token, nil)
	assertJSONEquals(t, r, "data.leaderboard.entries.0.count", float64(1))
}

func TestFriendlyFormatAndFreeTextLocation(t *testing.T) {
	h := newTestHandler(t)
	login := performJSONRequest(t, h, "POST", "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	token := jsonPath(t, login, "data.token").(string)
	before := performJSONRequest(t, h, "GET", "/api/locations", token, nil)
	e := performJSONRequest(t, h, "POST", "/api/events", token, map[string]any{"type": "friendly", "title": "Format test", "format": 5, "starts_at": "2026-09-25T19:00:00+08:00", "location": "One-off pitch"})
	assertStatus(t, e, 200)
	assertJSONEquals(t, e, "data.event.format", float64(5))
	id := jsonPath(t, e, "data.event.id").(string)
	edit := performJSONRequest(t, h, "PATCH", "/api/events/"+id, token, map[string]any{"format": 8})
	assertStatus(t, edit, 200)
	assertJSONEquals(t, edit, "data.event.format", float64(8))
	persisted := performJSONRequest(t, h, "GET", "/api/events", token, nil)
	assertJSONEquals(t, persisted, "data.events.0.format", float64(8))
	assertStatus(t, performJSONRequest(t, h, "PATCH", "/api/events/"+id, token, map[string]any{"format": 6}), 400)
	after := performJSONRequest(t, h, "GET", "/api/locations", token, nil)
	if fmt.Sprint(jsonPath(t, before, "data.locations")) != fmt.Sprint(jsonPath(t, after, "data.locations")) {
		t.Fatal("event location added to saved locations")
	}
	training := performJSONRequest(t, h, "PATCH", "/api/events/"+id, token, map[string]any{"type": "training", "location": ""})
	assertJSONEquals(t, training, "data.event.format", float64(0))
	assertJSONEquals(t, training, "data.event.location", "北京邮电大学（海淀校区）")
}
