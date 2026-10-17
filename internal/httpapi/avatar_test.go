package httpapi_test

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
)

func uploadAvatarBytes(t *testing.T, h http.Handler, token string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "avatar.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/player/avatar", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func reviewAvatar(t *testing.T, h http.Handler, job, suggest string) {
	t.Helper()
	parts := []string{"test-callback-token", "123", "nonce"}
	sort.Strings(parts)
	digest := sha1.Sum([]byte(strings.Join(parts, "")))
	query := url.Values{"timestamp": {"123"}, "nonce": {"nonce"}, "signature": {hex.EncodeToString(digest[:])}}
	payload, _ := json.Marshal(map[string]any{"Event": "wxa_media_check", "appid": "test-app", "trace_id": job, "errcode": 0, "result": map[string]any{"suggest": suggest}})
	req := httptest.NewRequest(http.MethodPost, "/api/wechat/content-callback?"+query.Encode(), bytes.NewReader(payload))
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	assertStatus(t, res, http.StatusOK)
}

func TestPlayerAvatarUploadValidatesAndWaitsForApproval(t *testing.T) {
	h := newTestHandler(t)
	login := performJSONRequest(t, h, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": "avatar-owner"})
	assertStatus(t, login, http.StatusOK)
	token := jsonPath(t, login, "data.token").(string)
	playerID := jsonPath(t, login, "data.player.id").(string)
	assertStatus(t, uploadAvatarBytes(t, h, "", []byte("bad")), http.StatusUnauthorized)
	assertStatus(t, uploadAvatarBytes(t, h, token, []byte("not an image")), http.StatusBadRequest)
	assertStatus(t, uploadAvatarBytes(t, h, token, bytes.Repeat([]byte("x"), (6<<20)+1)), http.StatusBadRequest)
	assertStatus(t, performJSONRequest(t, h, http.MethodGet, "/api/player/avatar", token, nil), http.StatusMethodNotAllowed)
	upload := performImageUpload(t, h, "/api/player/avatar", token)
	assertStatus(t, upload, http.StatusAccepted)
	job := jsonPath(t, upload, "data.review.id").(string)
	assertJSONEquals(t, upload, "data.review.status", "pending")
	before := performJSONRequest(t, h, http.MethodGet, "/api/player/me", token, nil)
	assertJSONEquals(t, before, "data.player.has_avatar", false)
	assertStatus(t, performJSONRequest(t, h, http.MethodGet, "/api/player/avatars/"+playerID, token, nil), http.StatusNotFound)
	reviewAvatar(t, h, job, "pass")
	review := performJSONRequest(t, h, http.MethodGet, "/api/player/reviews/"+job, token, nil)
	assertJSONEquals(t, review, "data.review.status", "approved")
	after := performJSONRequest(t, h, http.MethodGet, "/api/player/me", token, nil)
	assertJSONEquals(t, after, "data.player.has_avatar", true)
	avatar := performJSONRequest(t, h, http.MethodGet, "/api/player/avatars/"+playerID, token, nil)
	assertStatus(t, avatar, http.StatusOK)
	if avatar.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("unexpected content type: %s", avatar.Header().Get("Content-Type"))
	}
	assertStatus(t, performJSONRequest(t, h, http.MethodGet, "/api/player/avatars/not-a-member", "", nil), http.StatusUnauthorized)
	assertStatus(t, performJSONRequest(t, h, http.MethodGet, "/api/player/avatars/not-a-member", token, nil), http.StatusNotFound)
}

func TestRejectedAvatarKeepsCurrentPicture(t *testing.T) {
	h := newTestHandler(t)
	login := performJSONRequest(t, h, http.MethodPost, "/api/auth/wechat", "", map[string]any{"code": "avatar-review"})
	token := jsonPath(t, login, "data.token").(string)
	first := performImageUpload(t, h, "/api/player/avatar", token)
	reviewAvatar(t, h, jsonPath(t, first, "data.review.id").(string), "pass")
	second := performImageUpload(t, h, "/api/player/avatar", token)
	job := jsonPath(t, second, "data.review.id").(string)
	reviewAvatar(t, h, job, "risky")
	review := performJSONRequest(t, h, http.MethodGet, "/api/player/reviews/"+job, token, nil)
	assertJSONEquals(t, review, "data.review.status", "rejected")
	me := performJSONRequest(t, h, http.MethodGet, "/api/player/me", token, nil)
	assertJSONEquals(t, me, "data.player.has_avatar", true)
}
