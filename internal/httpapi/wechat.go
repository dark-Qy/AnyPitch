package httpapi

import (
	"anypitch/internal/auth"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

func (h *Handler) playerMemberProfile(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/player/members/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found", "队员不存在")
		return
	}
	var name string
	var hasAvatar bool
	err := h.db.QueryRow(`SELECT p.name, COALESCE(m.avatar_file_name, '') <> '' FROM players p LEFT JOIN wechat_members m ON m.player_id=p.id WHERE p.id=? AND p.team_id=?`, id, ctx.teamID).Scan(&name, &hasAvatar)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "队员不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取队员资料失败")
		return
	}
	count, err := h.stats.ParticipationCount(ctx.teamID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取参与次数失败")
		return
	}
	writeSuccess(w, map[string]any{"member": map[string]any{"id": id, "name": name, "has_avatar": hasAvatar, "participation_count": count}})
}

func (h *Handler) wechatLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, 405, "method_not_allowed", "Method not allowed.")
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := h.auth.LoginWeChat(r.Context(), input.Code)
	if errors.Is(err, auth.ErrWeChatUnavailable) {
		writeError(w, 503, "wechat_unavailable", err.Error())
		return
	}
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, 400, "invalid_credentials", "微信登录失败，请重新登录")
		return
	}
	if err != nil {
		handleAuthError(w, err)
		return
	}
	writeSuccess(w, session)
}
func (h *Handler) playerProfile(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodPatch {
		writeError(w, 405, "method_not_allowed", "Method not allowed.")
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !h.checkText(w, r, 1, input) {
		return
	}
	if err := h.auth.UpdateProfile(ctx.player.ID, input.Name); err != nil {
		if errors.Is(err, auth.ErrInvalidProfile) {
			writeError(w, 400, "invalid_profile", err.Error())
		} else {
			writeError(w, 500, "internal_error", "保存昵称失败")
		}
		return
	}
	member, err := h.auth.PlayerByToken(ctx.token)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"player": member})
}
func (h *Handler) coachMembers(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodGet {
		writeError(w, 405, "method_not_allowed", "Method not allowed.")
		return
	}
	rows, err := h.db.Query(`SELECT p.id,p.name,p.status,m.role FROM players p JOIN wechat_members m ON m.player_id=p.id WHERE p.team_id=? AND m.profile_complete=1 ORDER BY p.created_at,p.id`, ctx.teamID)
	if err != nil {
		writeError(w, 500, "internal_error", "读取成员失败")
		return
	}
	defer rows.Close()
	members := []map[string]string{}
	for rows.Next() {
		var id, name, status, role string
		if rows.Scan(&id, &name, &status, &role) != nil {
			writeError(w, 500, "internal_error", "读取成员失败")
			return
		}
		members = append(members, map[string]string{"id": id, "name": name, "status": status, "role": role})
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal_error", "读取成员失败")
		return
	}
	writeSuccess(w, map[string]any{"members": members})
}
func (h *Handler) coachMemberDetail(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodPatch {
		writeError(w, 405, "method_not_allowed", "Method not allowed.")
		return
	}
	actor, err := h.auth.PlayerByToken(ctx.token)
	if err != nil || actor.Role != "owner" {
		writeError(w, 403, "forbidden", "仅球队负责人可指定教练")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/coach/members/")
	var input struct {
		Role string `json:"role"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Role != "coach" && input.Role != "member" {
		writeError(w, 400, "invalid_role", "请选择队员或教练")
		return
	}
	result, err := h.db.Exec(`UPDATE wechat_members SET role=? WHERE player_id=? AND role<>'owner' AND player_id IN (SELECT id FROM players WHERE team_id=? AND status='active')`, input.Role, id, ctx.teamID)
	if err != nil {
		writeError(w, 500, "internal_error", "保存权限失败")
		return
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		writeError(w, 404, "not_found", "成员不存在或不可修改")
		return
	}
	writeSuccess(w, map[string]any{"ok": true})
}
