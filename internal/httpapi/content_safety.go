package httpapi

import (
	"anypitch/internal/contentsafety"
	"anypitch/internal/media"
	"bytes"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

func (h *Handler) miniMember(r *http.Request) string {
	p, e := h.auth.PlayerByToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if e != nil {
		return ""
	}
	return p.ID
}
func (h *Handler) openID(id string) (string, error) {
	var v string
	e := h.db.QueryRow(`SELECT open_id FROM wechat_members WHERE player_id=?`, id).Scan(&v)
	return v, e
}
func textValues(v reflect.Value) string {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return ""
		}
		return textValues(v.Elem())
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Struct:
		var ss []string
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			tag := strings.Split(v.Type().Field(i).Tag.Get("json"), ",")[0]
			// Check editable text, not IDs, timestamps or fixed RSVP/type enums.
			switch tag {
			case "name", "title", "location", "opponent", "note", "notes", "summary", "description", "label":
				ss = append(ss, textValues(field))
			default:
				if field.Kind() == reflect.Slice || field.Kind() == reflect.Struct {
					ss = append(ss, textValues(field))
				}
			}
		}
		return strings.Join(ss, "\n")
	case reflect.Slice:
		var ss []string
		for i := 0; i < v.Len(); i++ {
			ss = append(ss, textValues(v.Index(i)))
		}
		return strings.Join(ss, "\n")
	}
	return ""
}
func (h *Handler) checkText(w http.ResponseWriter, r *http.Request, scene int, value any) bool {
	id := h.miniMember(r)
	if id == "" {
		return true
	} // password-only web management is not a mini-program UGC identity
	openid, err := h.openID(id)
	if err != nil {
		return safetyError(w, contentsafety.ErrUnavailable)
	}
	text := []rune(textValues(reflect.ValueOf(value)))
	for len(text) > 0 {
		n := len(text)
		if n > 2500 {
			n = 2500
		}
		if err = h.safety.Text(r.Context(), openid, string(text[:n]), scene); err != nil {
			return safetyError(w, err)
		}
		text = text[n:]
	}
	return true
}
func safetyError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, contentsafety.ErrRejected) {
		writeError(w, 400, "content_rejected", err.Error())
	} else {
		writeError(w, 503, "content_check_unavailable", contentsafety.ErrUnavailable.Error())
	}
	return false
}

type reviewJob struct {
	ID      string          `json:"id"`
	Kind    string          `json:"kind"`
	Status  string          `json:"status"`
	Result  json.RawMessage `json:"result,omitempty"`
	Message string          `json:"message,omitempty"`
}

var reviewMu sync.Mutex

func (h *Handler) expireReviews() {
	rows, e := h.db.Query(`SELECT id FROM content_reviews WHERE status='pending' AND created_at < ?`, time.Now().Add(-35*time.Minute).Unix())
	if e != nil {
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		h.db.Exec(`UPDATE content_reviews SET status='error' WHERE id=? AND status='pending'`, id)
		os.Remove(filepath.Join(h.reviewDir, id+".jpg"))
	}
	h.db.Exec(`DELETE FROM content_reviews WHERE status<>'pending' AND created_at < ?`, time.Now().Add(-24*time.Hour).Unix())
}
func (h *Handler) submitImage(w http.ResponseWriter, r *http.Request, kind, album, caption string, source io.Reader) bool {
	id := h.miniMember(r)
	if id == "" {
		return false
	}
	if !h.checkText(w, r, 2, caption) {
		return true
	}
	base := strings.TrimRight(os.Getenv("ANYPITCH_PUBLIC_URL"), "/")
	if !strings.HasPrefix(base, "https://") || os.Getenv("ANYPITCH_WECHAT_CALLBACK_TOKEN") == "" {
		safetyError(w, contentsafety.ErrUnavailable)
		return true
	}
	openid, e := h.openID(id)
	if e != nil {
		safetyError(w, contentsafety.ErrUnavailable)
		return true
	}
	data, e := media.ReviewJPEG(source)
	if e != nil {
		if kind == "avatar" && errors.Is(e, media.ErrInvalidPhoto) {
			writeError(w, http.StatusBadRequest, "invalid_avatar", "请选择不超过 6 MB、2500 万像素的 JPEG、PNG 或 WebP 图片")
		} else {
			writeMediaError(w, e)
		}
		return true
	}
	if kind == "photo" {
		if _, e = h.media.ListPhotos(h.playerTeam(id), album); e != nil {
			writeMediaError(w, e)
			return true
		}
	}
	reviewMu.Lock()
	h.expireReviews()
	reviewMu.Unlock()
	job := uuid.NewString()
	path := filepath.Join(h.reviewDir, job+".jpg")
	if e = os.WriteFile(path, data, 0600); e != nil {
		writeMediaError(w, e)
		return true
	}
	_, e = h.db.Exec(`INSERT INTO content_reviews(id,player_id,kind,album_id,caption,status,created_at) VALUES(?,?,?,?,?,'pending',?)`, job, id, kind, album, caption, time.Now().Unix())
	if e != nil {
		os.Remove(path)
		writeMediaError(w, e)
		return true
	}
	if kind == "avatar" {
		reviewMu.Lock()
		rows, err := h.db.Query(`SELECT id FROM content_reviews WHERE player_id=? AND kind='avatar' AND status='pending' AND id<>?`, id, job)
		if err == nil {
			var old []string
			for rows.Next() {
				var v string
				if rows.Scan(&v) == nil {
					old = append(old, v)
				}
			}
			rows.Close()
			for _, v := range old {
				h.db.Exec(`UPDATE content_reviews SET status='error' WHERE id=?`, v)
				os.Remove(filepath.Join(h.reviewDir, v+".jpg"))
			}
		}
		reviewMu.Unlock()
	}
	scene := 4
	if kind == "avatar" {
		scene = 1
	}
	trace, e := h.safety.Image(r.Context(), openid, base+"/api/wechat/review-media/"+job, scene)
	if e == nil {
		_, e = h.db.Exec(`UPDATE content_reviews SET trace_id=? WHERE id=?`, trace, job)
	}
	if e != nil {
		h.db.Exec(`UPDATE content_reviews SET status='error' WHERE id=?`, job)
		os.Remove(path)
		safetyError(w, e)
		return true
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"data": map[string]any{"review": reviewJob{ID: job, Kind: kind, Status: "pending"}}})
	return true
}
func (h *Handler) playerTeam(id string) string {
	var team string
	h.db.QueryRow(`SELECT team_id FROM players WHERE id=? AND status='active'`, id).Scan(&team)
	return team
}
func (h *Handler) reviewMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/wechat/review-media/")
	var status string
	var created int64
	if h.db.QueryRow(`SELECT status,created_at FROM content_reviews WHERE id=?`, id).Scan(&status, &created) != nil || status != "pending" || time.Now().Unix()-created > 35*60 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, filepath.Join(h.reviewDir, id+".jpg"))
}
func (h *Handler) playerReviews(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	reviewMu.Lock()
	h.expireReviews()
	reviewMu.Unlock()
	id := strings.TrimPrefix(r.URL.Path, "/api/player/reviews/")
	var job reviewJob
	var result string
	err := h.db.QueryRow(`SELECT id,kind,status,result FROM content_reviews WHERE id=? AND player_id=?`, id, ctx.player.ID).Scan(&job.ID, &job.Kind, &job.Status, &result)
	if err != nil {
		writeError(w, 404, "not_found", "检测记录不存在")
		return
	}
	if result != "" {
		job.Result = json.RawMessage(result)
	}
	if job.Status == "rejected" {
		job.Message = contentsafety.ErrRejected.Error()
	}
	if job.Status == "error" {
		job.Message = "图片审核失败或超时，请重新上传"
	}
	writeSuccess(w, map[string]any{"review": job})
}
func callbackValid(r *http.Request) bool {
	token := os.Getenv("ANYPITCH_WECHAT_CALLBACK_TOKEN")
	if token == "" {
		return false
	}
	q := r.URL.Query()
	parts := []string{token, q.Get("timestamp"), q.Get("nonce")}
	if parts[1] == "" || parts[2] == "" {
		return false
	}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(q.Get("signature"))) == 1
}
func (h *Handler) safetyCallback(w http.ResponseWriter, r *http.Request) {
	if !callbackValid(r) {
		w.WriteHeader(403)
		return
	}
	if r.Method == "GET" {
		io.WriteString(w, r.URL.Query().Get("echostr"))
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var event struct {
		Event  string `json:"Event" xml:"Event"`
		AppID  string `json:"appid" xml:"appid"`
		Trace  string `json:"trace_id" xml:"trace_id"`
		Code   int    `json:"errcode" xml:"errcode"`
		Result struct {
			Suggest string `json:"suggest" xml:"suggest"`
		} `json:"result" xml:"result"`
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if e != nil {
		w.WriteHeader(400)
		return
	}
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte("<")) {
		e = xml.Unmarshal(body, &event)
	} else {
		e = json.Unmarshal(body, &event)
	}
	if e != nil || event.Event != "wxa_media_check" || event.AppID != os.Getenv("ANYPITCH_WECHAT_APP_ID") || event.Trace == "" {
		w.WriteHeader(400)
		return
	}
	reviewMu.Lock()
	defer reviewMu.Unlock()
	var id, member, kind, album, caption, status string
	e = h.db.QueryRow(`SELECT id,player_id,kind,album_id,caption,status FROM content_reviews WHERE trace_id=?`, event.Trace).Scan(&id, &member, &kind, &album, &caption, &status)
	if e != nil {
		w.WriteHeader(503)
		return
	} // also asks WeChat to retry a callback arriving before trace persistence
	if status != "pending" {
		io.WriteString(w, "success")
		return
	}
	path := filepath.Join(h.reviewDir, id+".jpg")
	var result []byte
	if event.Code != 0 {
		status = "error"
	} else if event.Result.Suggest == "risky" || event.Result.Suggest == "review" {
		status = "rejected"
	} else if event.Result.Suggest != "pass" {
		status = "error"
	} else {
		// Never publish after expiry or a member has been disabled. Keep the previous avatar.
		var created int64
		h.db.QueryRow(`SELECT created_at FROM content_reviews WHERE id=?`, id).Scan(&created)
		team := h.playerTeam(member)
		if team == "" || time.Now().Unix()-created > 35*60 {
			status = "error"
		} else {
			file, err := os.Open(path)
			if err == nil {
				if kind == "avatar" {
					err = h.media.StoreAvatar(team, member, file)
					result = []byte(`{"avatar":true}`)
				} else {
					var photo media.Photo
					photo, err = h.media.StoreReviewedPhoto(team, album, media.Actor{Role: "player", PlayerID: member}, caption, file, id)
					result, _ = json.Marshal(map[string]any{"photo": photo})
				}
				file.Close()
			}
			if err != nil {
				status = "error"
			} else {
				status = "approved"
			}
		}
	}
	if _, e = h.db.Exec(`UPDATE content_reviews SET status=?,result=? WHERE id=?`, status, string(result), id); e != nil {
		w.WriteHeader(500)
		return
	}
	os.Remove(path)
	io.WriteString(w, "success")
}
