package httpapi_test

import (
	"net/http"
	"testing"
)

func TestRejectedNicknameDoesNotChangeProfile(t *testing.T) {
	h := newTestHandler(t)
	login := performJSONRequest(t, h, "POST", "/api/auth/wechat", "", map[string]any{"code": "safety-user"})
	token := jsonPath(t, login, "data.token").(string)
	bad := performJSONRequest(t, h, "PATCH", "/api/player/profile", token, map[string]any{"name": "违规测试"})
	assertStatus(t, bad, 400)
	assertJSONEquals(t, bad, "error.message", "所发布内容含违规信息")
	me := performJSONRequest(t, h, "GET", "/api/player/me", token, nil)
	assertJSONEquals(t, me, "data.player.profile_complete", false)
}

func TestOnlyWebAdministratorCanPublishAlbumPhotos(t *testing.T) {
	h := newTestHandler(t)
	coach := performJSONRequest(t, h, "POST", "/api/auth/login", "", map[string]any{"password": "AnyPitch@2026"})
	coachToken := jsonPath(t, coach, "data.token").(string)
	event := performJSONRequest(t, h, "POST", "/api/events", coachToken, map[string]any{"type": "training", "title": "审核测试训练", "starts_at": "2026-10-20T19:00:00+08:00", "ends_at": "2026-10-20T21:00:00+08:00"})
	assertStatus(t, event, 200)
	eventID := jsonPath(t, event, "data.event.id").(string)
	member := performJSONRequest(t, h, "POST", "/api/auth/wechat", "", map[string]any{"code": "photo-owner"})
	token := jsonPath(t, member, "data.token").(string)
	assertStatus(t, performJSONRequest(t, h, "POST", "/api/player/events", token, map[string]any{"type": "pickup", "title": "自由野球", "starts_at": "2026-10-20T19:00:00+08:00"}), http.StatusForbidden)
	assertStatus(t, performJSONRequest(t, h, "POST", "/api/player/albums", token, map[string]any{"event_id": eventID}), http.StatusForbidden)
	album := performJSONRequest(t, h, "POST", "/api/albums", coachToken, map[string]any{"event_id": eventID})
	assertStatus(t, album, 200)
	albumID := jsonPath(t, album, "data.album.id").(string)
	assertStatus(t, performImageUpload(t, h, "/api/player/albums/"+albumID+"/photos", token), http.StatusForbidden)
	upload := performImageUpload(t, h, "/api/albums/"+albumID+"/photos", coachToken)
	assertStatus(t, upload, 200)
	photoID := jsonPath(t, upload, "data.photo.id").(string)
	assertStatus(t, performJSONRequest(t, h, "PATCH", "/api/player/photos/"+photoID, token, map[string]any{"caption": "私自编辑"}), http.StatusForbidden)
	assertStatus(t, performJSONRequest(t, h, "DELETE", "/api/player/photos/"+photoID, token, nil), http.StatusForbidden)
	assertStatus(t, performJSONRequest(t, h, "GET", "/api/player/photos/"+photoID, token, nil), http.StatusOK)
	assertStatus(t, performJSONRequest(t, h, "GET", "/api/player/albums/"+albumID+"/photos", token, nil), http.StatusOK)
	assertStatus(t, performJSONRequest(t, h, "DELETE", "/api/photos/"+photoID, coachToken, nil), http.StatusOK)
}
