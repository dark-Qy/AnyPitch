package httpapi

import (
	"anypitch/internal/media"
	"net/http"
	"os"
	"strings"
)

func (h *Handler) playerAvatarUpload(w http.ResponseWriter, r *http.Request, _ playerRequestContext) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+64*1024)
	if err := r.ParseMultipartForm(media.MaxUploadBytes + 64*1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_photo", "请选择不超过 6 MB 的 JPEG、PNG 或 WebP 图片")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_photo", "请选择一张头像图片")
		return
	}
	defer file.Close()
	h.submitImage(w, r, "avatar", "", "", file)
}

func (h *Handler) playerAvatarImage(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "Method not allowed.")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/player/avatars/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, 404, "not_found", "头像不存在")
		return
	}
	path, err := h.media.AvatarPath(ctx.player.TeamID, id)
	if err != nil {
		writeError(w, 404, "not_found", "头像不存在")
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeError(w, 404, "not_found", "头像不存在")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeFile(w, r, path)
}
