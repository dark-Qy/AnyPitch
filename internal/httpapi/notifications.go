package httpapi

import (
	"net/http"
	"strings"
)

func (h *Handler) playerNotifications(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if !h.notifications.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "notifications_unavailable", "新活动提醒暂未开放")
		return
	}
	switch r.Method {
	case http.MethodGet:
		remaining, err := h.notifications.Remaining(ctx.player.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "读取提醒状态失败")
			return
		}
		writeSuccess(w, map[string]any{"template_id": h.notifications.TemplateID(), "remaining": remaining})
	case http.MethodPost:
		var input struct {
			RequestID string `json:"request_id"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		input.RequestID = strings.TrimSpace(input.RequestID)
		if input.RequestID == "" || len(input.RequestID) > 100 {
			writeError(w, http.StatusBadRequest, "invalid_request", "提醒授权编号无效")
			return
		}
		remaining, err := h.notifications.Grant(ctx.player.ID, input.RequestID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "保存提醒授权失败")
			return
		}
		writeSuccess(w, map[string]any{"remaining": remaining})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}
