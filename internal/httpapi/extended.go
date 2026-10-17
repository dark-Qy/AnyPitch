package httpapi

import (
	"encoding/csv"
	"errors"
	"net/http"
	"path"
	"strings"

	"anypitch/internal/media"
	"anypitch/internal/specialactivity"
	"anypitch/internal/stats"
)

func (h *Handler) eventCancel(w http.ResponseWriter, r *http.Request, ctx requestContext, eventID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	event, err := h.events.Cancel(ctx.teamID, eventID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event", "Event could not be cancelled.")
		return
	}
	writeSuccess(w, map[string]any{"event": event})
}

func (h *Handler) eventRestore(w http.ResponseWriter, r *http.Request, ctx requestContext, eventID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	event, err := h.events.Restore(ctx.teamID, eventID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event", "Event could not be restored.")
		return
	}
	writeSuccess(w, map[string]any{"event": event})
}

func (h *Handler) termsCollection(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	switch r.Method {
	case http.MethodGet:
		terms, err := h.stats.ListTerms(ctx.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list terms.")
			return
		}
		writeSuccess(w, map[string]any{"terms": terms})
	case http.MethodPost:
		var input stats.TermInput
		if !decodeJSON(w, r, &input) {
			return
		}
		if !h.checkText(w, r, 3, input) {
			return
		}
		term, err := h.stats.CreateTerm(ctx.teamID, input)
		if err != nil {
			writeStatsError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"term": term})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) termDetail(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	id := strings.TrimPrefix(r.URL.Path, "/api/terms/")
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "Term not found.")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var input stats.TermUpdateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		if !h.checkText(w, r, 3, input) {
			return
		}
		term, err := h.stats.UpdateTerm(ctx.teamID, id, input)
		if err != nil {
			writeStatsError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"term": term})
	case http.MethodDelete:
		if err := h.stats.DeleteTerm(ctx.teamID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete term.")
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) coachLeaderboard(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	leaderboard, err := h.stats.Leaderboard(ctx.teamID, leaderboardQuery(r))
	if err != nil {
		writeStatsError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"leaderboard": leaderboard})
}

func (h *Handler) playerLeaderboard(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	leaderboard, err := h.stats.Leaderboard(ctx.teamID, leaderboardQuery(r))
	if err != nil {
		writeStatsError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"leaderboard": leaderboard})
}

func (h *Handler) playerTerms(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	terms, err := h.stats.ListTerms(ctx.teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list terms.")
		return
	}
	writeSuccess(w, map[string]any{"terms": terms})
}

func (h *Handler) coachHistory(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	history, err := h.stats.History(ctx.teamID, historyFilter(r))
	if err != nil {
		writeStatsError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"history": history})
}

func (h *Handler) playerStatsMe(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	filter := historyFilter(r)
	filter.PlayerID = ctx.player.ID
	filter.Personal = true
	history, err := h.stats.History(ctx.teamID, filter)
	if err != nil {
		writeStatsError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"history": history})
}

func (h *Handler) coachHistoryCSV(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	history, err := h.stats.History(ctx.teamID, historyFilter(r))
	if err != nil {
		writeStatsError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="anypitch-history.csv"`)
	w.WriteHeader(http.StatusOK)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"活动日期", "类型", "活动", "队员", "报名", "报名备注", "活动状态"})
	for _, row := range history {
		state := "正常"
		if row.CancelledAt != "" {
			state = "已取消"
		}
		values := []string{row.StartsAt, row.Type, row.Title, row.PlayerName, row.RSVPStatus, row.RSVPNote, state}
		for i := range values {
			values[i] = csvCell(values[i])
		}
		_ = writer.Write(values)
	}
	writer.Flush()
}

func (h *Handler) specialActivitiesCollection(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	switch r.Method {
	case http.MethodGet:
		activities, err := h.specialActivities.List(ctx.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list special activities.")
			return
		}
		writeSuccess(w, map[string]any{"activities": activities})
	case http.MethodPost:
		var input specialactivity.ActivityInput
		if !decodeJSON(w, r, &input) {
			return
		}
		if !h.checkText(w, r, 3, input) {
			return
		}
		activity, err := h.specialActivities.Create(ctx.teamID, input)
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"activity": activity})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) specialActivityNested(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	parts := nestedParts(r.URL.Path, "/api/special-activities/")
	if len(parts) == 1 {
		h.specialActivityDetail(w, r, ctx, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "entries" {
		h.specialActivityEntries(w, r, ctx, parts[0])
		return
	}
	if len(parts) == 3 && parts[1] == "entries" {
		h.specialActivityEntryDetail(w, r, ctx, parts[0], parts[2])
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Special activity route not found.")
}

func (h *Handler) specialActivityDetail(w http.ResponseWriter, r *http.Request, ctx requestContext, id string) {
	switch r.Method {
	case http.MethodGet:
		activity, err := h.specialActivities.Get(ctx.teamID, id)
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"activity": activity})
	case http.MethodPatch:
		var input specialactivity.ActivityUpdateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		if !h.checkText(w, r, 3, input) {
			return
		}
		activity, err := h.specialActivities.Update(ctx.teamID, id, input)
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"activity": activity})
	case http.MethodDelete:
		if err := h.specialActivities.Delete(ctx.teamID, id); err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) specialActivityEntries(w http.ResponseWriter, r *http.Request, ctx requestContext, activityID string) {
	switch r.Method {
	case http.MethodGet:
		entries, err := h.specialActivities.ListEntries(ctx.teamID, activityID)
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"entries": entries})
	case http.MethodPost:
		var input specialactivity.EntryInput
		if !decodeJSON(w, r, &input) {
			return
		}
		if !h.checkText(w, r, 3, input) {
			return
		}
		entry, err := h.specialActivities.CreateEntry(ctx.teamID, activityID, input)
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"entry": entry})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) specialActivityEntryDetail(w http.ResponseWriter, r *http.Request, ctx requestContext, activityID, entryID string) {
	switch r.Method {
	case http.MethodPut:
		var input specialactivity.EntryInput
		if !decodeJSON(w, r, &input) {
			return
		}
		if !h.checkText(w, r, 3, input) {
			return
		}
		entry, err := h.specialActivities.UpdateEntry(ctx.teamID, activityID, entryID, input)
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"entry": entry})
	case http.MethodDelete:
		if err := h.specialActivities.DeleteEntry(ctx.teamID, activityID, entryID); err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) playerSpecialActivities(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	activities, err := h.specialActivities.List(ctx.teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list special activities.")
		return
	}
	writeSuccess(w, map[string]any{"activities": activities})
}

func (h *Handler) playerSpecialActivityNested(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	parts := nestedParts(r.URL.Path, "/api/player/special-activities/")
	if len(parts) == 1 {
		activity, err := h.specialActivities.Get(ctx.teamID, parts[0])
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"activity": activity})
		return
	}
	if len(parts) == 2 && parts[1] == "entries" {
		entries, err := h.specialActivities.ListEntries(ctx.teamID, parts[0])
		if err != nil {
			writeSpecialActivityError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"entries": entries})
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Special activity route not found.")
}

func (h *Handler) coachAlbumsCollection(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	h.albumsCollection(w, r, ctx.teamID, media.Actor{Role: "coach"})
}
func (h *Handler) playerAlbumsCollection(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusForbidden, "forbidden", "相册由网页管理端维护")
		return
	}
	h.albumsCollection(w, r, ctx.teamID, media.Actor{Role: "player", PlayerID: ctx.player.ID})
}

func (h *Handler) albumsCollection(w http.ResponseWriter, r *http.Request, teamID string, actor media.Actor) {
	switch r.Method {
	case http.MethodGet:
		albums, err := h.media.ListAlbums(teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list albums.")
			return
		}
		writeSuccess(w, map[string]any{"albums": albums})
	case http.MethodPost:
		var input struct {
			EventID           string `json:"event_id"`
			SpecialActivityID string `json:"special_activity_id"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		album, err := h.media.EnsureAlbum(teamID, input.EventID, input.SpecialActivityID)
		if err != nil {
			writeMediaError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"album": album})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) coachAlbumNested(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	h.albumNested(w, r, ctx.teamID, media.Actor{Role: "coach"}, "/api/albums/")
}
func (h *Handler) playerAlbumNested(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusForbidden, "forbidden", "相册由网页管理端维护")
		return
	}
	h.albumNested(w, r, ctx.teamID, media.Actor{Role: "player", PlayerID: ctx.player.ID}, "/api/player/albums/")
}

func (h *Handler) albumNested(w http.ResponseWriter, r *http.Request, teamID string, actor media.Actor, prefix string) {
	parts := nestedParts(r.URL.Path, prefix)
	if len(parts) != 2 || parts[1] != "photos" {
		writeError(w, http.StatusNotFound, "not_found", "Album route not found.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		photos, err := h.media.ListPhotos(teamID, parts[0])
		if err != nil {
			writeMediaError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"photos": photos})
	case http.MethodPost:
		h.uploadPhoto(w, r, teamID, parts[0], actor)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) uploadPhoto(w http.ResponseWriter, r *http.Request, teamID, albumID string, actor media.Actor) {
	r.Body = http.MaxBytesReader(w, r.Body, media.MaxUploadBytes+64*1024)
	if err := r.ParseMultipartForm(media.MaxUploadBytes + 64*1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_photo", "Choose a JPEG, PNG, or WebP photo under 6 MB.")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_photo", "Choose one photo to upload.")
		return
	}
	defer file.Close()
	if h.submitImage(w, r, "photo", albumID, r.FormValue("caption"), file) {
		return
	}
	photo, err := h.media.StorePhoto(teamID, albumID, actor, r.FormValue("caption"), file)
	if err != nil {
		writeMediaError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"photo": photo})
}

func (h *Handler) coachPhotoDetail(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	h.photoDetail(w, r, ctx.teamID, media.Actor{Role: "coach"}, "/api/photos/")
}
func (h *Handler) playerPhotoDetail(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusForbidden, "forbidden", "照片由网页管理端维护")
		return
	}
	h.photoDetail(w, r, ctx.teamID, media.Actor{Role: "player", PlayerID: ctx.player.ID}, "/api/player/photos/")
}

func (h *Handler) photoDetail(w http.ResponseWriter, r *http.Request, teamID string, actor media.Actor, prefix string) {
	photoID := strings.TrimPrefix(r.URL.Path, prefix)
	if photoID == "" || strings.Contains(photoID, "/") {
		writeError(w, http.StatusNotFound, "not_found", "Photo not found.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		file, contentType, err := h.media.OpenPhoto(teamID, photoID, r.URL.Query().Get("variant") == "thumb")
		if err != nil {
			writeMediaError(w, err)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			writeMediaError(w, err)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "private, max-age=300")
		http.ServeContent(w, r, "", info.ModTime(), file)
	case http.MethodPatch:
		var input struct {
			Caption string `json:"caption"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		if !h.checkText(w, r, 3, input) {
			return
		}
		photo, err := h.media.UpdateCaption(teamID, photoID, actor, input.Caption)
		if err != nil {
			writeMediaError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"photo": photo})
	case http.MethodDelete:
		if err := h.media.DeletePhoto(teamID, photoID, actor); err != nil {
			writeMediaError(w, err)
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func leaderboardQuery(r *http.Request) stats.LeaderboardQuery {
	return stats.LeaderboardQuery{Month: r.URL.Query().Get("month"), TermID: r.URL.Query().Get("term_id"), Year: r.URL.Query().Get("year"), Half: r.URL.Query().Get("half")}
}
func historyFilter(r *http.Request) stats.HistoryFilter {
	return stats.HistoryFilter{PlayerID: r.URL.Query().Get("player_id"), Type: r.URL.Query().Get("type"), From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to")}
}
func nestedParts(value, prefix string) []string {
	clean := path.Clean(strings.TrimPrefix(value, prefix))
	if clean == "." || clean == "" {
		return nil
	}
	return strings.Split(clean, "/")
}
func csvCell(value string) string {
	if value != "" && strings.ContainsAny(value[:1], "=+-@") {
		return "'" + value
	}
	return value
}

func writeStatsError(w http.ResponseWriter, err error) {
	if errors.Is(err, stats.ErrOverlappingTerm) {
		writeError(w, http.StatusConflict, "overlapping_term", "Terms cannot overlap.")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_statistics", "Statistics request is invalid.")
}

func writeSpecialActivityError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadRequest, "invalid_special_activity", "Special activity request is invalid.")
}
func writeMediaError(w http.ResponseWriter, err error) {
	if errors.Is(err, media.ErrForbidden) {
		writeError(w, http.StatusForbidden, "forbidden", "You cannot change this photo.")
		return
	}
	if errors.Is(err, media.ErrStorageFull) {
		writeError(w, http.StatusInsufficientStorage, "media_storage_full", "Team photo storage is full. Ask the coach to remove photos before uploading more.")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_photo", "Photo request is invalid.")
}
