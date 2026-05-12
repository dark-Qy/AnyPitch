package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"anypitch/internal/attendance"
	"anypitch/internal/auth"
	"anypitch/internal/db"
	teamevent "anypitch/internal/event"
	"anypitch/internal/player"
	"anypitch/internal/tactics"
)

type Handler struct {
	auth       *auth.Service
	players    *player.Service
	events     *teamevent.Service
	attendance *attendance.Service
	tactics    *tactics.Service
	db         *sql.DB
}

func NewHandler(conn *sql.DB) (http.Handler, error) {
	if err := db.Migrate(conn); err != nil {
		return nil, err
	}
	authService := auth.NewService(conn)
	handler := &Handler{
		auth:       authService,
		players:    player.NewService(conn),
		events:     teamevent.NewService(conn),
		attendance: attendance.NewService(conn),
		tactics:    tactics.NewService(conn),
		db:         conn,
	}
	defaultCoach, err := handler.auth.EnsureDefaultCoach()
	if err != nil {
		return nil, err
	}
	if _, err := handler.ensureDefaultTeam(defaultCoach.ID); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", publicHealth)
	mux.HandleFunc("/api/healthz", apiHealth)
	mux.HandleFunc("/api/auth/login", handler.login)
	mux.Handle("/api/auth/logout", handler.withAuth(handler.logout))
	mux.Handle("/api/auth/me", handler.withAuth(handler.me))
	mux.HandleFunc("/api/player/login", handler.playerLogin)
	mux.Handle("/api/player/logout", handler.withPlayer(handler.playerLogout))
	mux.Handle("/api/player/me", handler.withPlayer(handler.playerMe))
	mux.Handle("/api/player/events", handler.withPlayer(handler.playerEventsCollection))
	mux.Handle("/api/player/events/", handler.withPlayer(handler.playerEventNested))
	mux.Handle("/api/players", handler.withAuth(handler.playersCollection))
	mux.Handle("/api/players/", handler.withAuth(handler.playerDetail))
	mux.Handle("/api/locations", handler.withAuth(handler.locationsCollection))
	mux.Handle("/api/locations/", handler.withAuth(handler.locationDetail))
	mux.Handle("/api/events", handler.withAuth(handler.eventsCollection))
	mux.Handle("/api/events/", handler.withAuth(handler.eventNested))
	mux.Handle("/api/tactics/templates", handler.withAuth(handler.tacticsTemplates))
	mux.Handle("/api/tactics/boards", handler.withAuth(handler.tacticsBoardsCollection))
	mux.Handle("/api/tactics/boards/", handler.withAuth(handler.tacticsBoardDetail))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "API route not found.")
	})
	if frontend, ok := resolveFrontend(); ok {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if shouldServeSPA(r.URL.Path) {
				http.ServeFile(w, r, frontend.indexPath)
				return
			}
			frontend.fileServer.ServeHTTP(w, r)
		})
	}
	return mux, nil
}

func publicHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func apiHealth(w http.ResponseWriter, r *http.Request) {
	writeSuccess(w, map[string]any{"ok": true})
}

type frontendServer struct {
	fileServer http.Handler
	indexPath  string
}

func resolveFrontend() (frontendServer, bool) {
	dist := filepath.Join("web", "dist")
	indexPath := filepath.Join(dist, "index.html")
	if !fileExists(indexPath) {
		return frontendServer{}, false
	}
	return frontendServer{
		fileServer: http.FileServer(http.Dir(dist)),
		indexPath:  indexPath,
	}, true
}

func shouldServeSPA(requestPath string) bool {
	if requestPath == "/" {
		return true
	}
	if strings.HasPrefix(requestPath, "/api/") || requestPath == "/api" {
		return false
	}
	return filepath.Ext(requestPath) == ""
}

func fileExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir()
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := h.auth.Login(input.Password)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	if _, err := h.ensureDefaultTeam(session.User.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to prepare team workspace.")
		return
	}
	writeSuccess(w, session)
}

func (h *Handler) playerLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	session, err := h.auth.LoginPlayerByName(input.Name)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	writeSuccess(w, session)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	if err := h.auth.Logout(ctx.token); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to logout.")
		return
	}
	writeSuccess(w, map[string]any{"ok": true})
}

func (h *Handler) playerLogout(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	if err := h.auth.LogoutPlayer(ctx.token); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to logout.")
		return
	}
	writeSuccess(w, map[string]any{"ok": true})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	writeSuccess(w, map[string]any{"user": ctx.user, "team_id": ctx.teamID})
}

func (h *Handler) playerMe(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	writeSuccess(w, map[string]any{"player": ctx.player})
}

func (h *Handler) playersCollection(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	switch r.Method {
	case http.MethodGet:
		players, err := h.players.List(ctx.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list players.")
			return
		}
		writeSuccess(w, map[string]any{"players": players})
	case http.MethodPost:
		var input player.CreateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		created, err := h.players.Create(ctx.teamID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_player", "Player name is required.")
			return
		}
		writeSuccess(w, map[string]any{"player": created})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) playerDetail(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	id := strings.TrimPrefix(r.URL.Path, "/api/players/")
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "Player not found.")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var input player.UpdateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		updated, err := h.players.Update(ctx.teamID, id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_player", "Player update is invalid.")
			return
		}
		writeSuccess(w, map[string]any{"player": updated})
	case http.MethodDelete:
		if err := h.players.Delete(ctx.teamID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete player.")
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) eventsCollection(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	switch r.Method {
	case http.MethodGet:
		events, err := h.events.List(ctx.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list events.")
			return
		}
		writeSuccess(w, map[string]any{"events": events})
	case http.MethodPost:
		var input teamevent.CreateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		created, err := h.events.Create(ctx.teamID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_event", "Event type, title, and RFC3339 starts_at are required; ends_at must be after starts_at when provided.")
			return
		}
		writeSuccess(w, map[string]any{"event": created})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) locationsCollection(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	switch r.Method {
	case http.MethodGet:
		locations, err := h.events.ListLocations(ctx.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list locations.")
			return
		}
		writeSuccess(w, map[string]any{"locations": locations})
	case http.MethodPost:
		var input teamevent.LocationInput
		if !decodeJSON(w, r, &input) {
			return
		}
		location, err := h.events.CreateLocation(ctx.teamID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_location", "Location name is required.")
			return
		}
		writeSuccess(w, map[string]any{"location": location})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) locationDetail(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	id := strings.TrimPrefix(r.URL.Path, "/api/locations/")
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "Location not found.")
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := h.events.DeleteLocation(ctx.teamID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete location.")
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) eventNested(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/api/events/"))
	parts := strings.Split(clean, "/")
	if len(parts) == 1 {
		h.eventDetail(w, r, ctx, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "attendance" {
		h.eventAttendance(w, r, ctx, parts[0])
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Event route not found.")
}

func (h *Handler) eventDetail(w http.ResponseWriter, r *http.Request, ctx requestContext, id string) {
	switch r.Method {
	case http.MethodPatch:
		var input teamevent.UpdateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		updated, err := h.events.Update(ctx.teamID, id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_event", "Event update is invalid.")
			return
		}
		writeSuccess(w, map[string]any{"event": updated})
	case http.MethodDelete:
		if err := h.events.Delete(ctx.teamID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete event.")
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) eventAttendance(w http.ResponseWriter, r *http.Request, ctx requestContext, eventID string) {
	switch r.Method {
	case http.MethodGet:
		records, err := h.attendance.List(ctx.teamID, eventID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_attendance", "Event not found.")
			return
		}
		writeSuccess(w, map[string]any{"records": records})
	case http.MethodPut:
		var input attendance.ReplaceInput
		if !decodeJSON(w, r, &input) {
			return
		}
		records, err := h.attendance.Replace(ctx.teamID, eventID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_attendance", "Attendance records are invalid.")
			return
		}
		writeSuccess(w, map[string]any{"records": records})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) playerEventsCollection(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	events, err := h.events.List(ctx.teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list events.")
		return
	}
	writeSuccess(w, map[string]any{"events": events})
}

func (h *Handler) playerEventNested(w http.ResponseWriter, r *http.Request, ctx playerRequestContext) {
	clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/api/player/events/"))
	parts := strings.Split(clean, "/")
	if len(parts) == 2 && parts[1] == "attendance" {
		h.playerEventAttendance(w, r, ctx, parts[0])
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "Player event route not found.")
}

func (h *Handler) playerEventAttendance(w http.ResponseWriter, r *http.Request, ctx playerRequestContext, eventID string) {
	switch r.Method {
	case http.MethodGet:
		record, err := h.attendance.GetForPlayer(ctx.teamID, eventID, ctx.player.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_attendance", "Event not found.")
			return
		}
		summary, err := h.attendance.Summary(ctx.teamID, eventID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_attendance", "Event not found.")
			return
		}
		writeSuccess(w, map[string]any{"record": record, "summary": summary})
	case http.MethodPut:
		var input struct {
			Status string `json:"status"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		record, err := h.attendance.SaveForPlayer(ctx.teamID, eventID, ctx.player.ID, input.Status)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_attendance", "Attendance status is invalid.")
			return
		}
		summary, err := h.attendance.Summary(ctx.teamID, eventID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_attendance", "Event not found.")
			return
		}
		writeSuccess(w, map[string]any{"record": record, "summary": summary})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) tacticsTemplates(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	writeSuccess(w, map[string]any{"templates": tactics.Templates()})
}

func (h *Handler) tacticsBoardsCollection(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	switch r.Method {
	case http.MethodGet:
		boards, err := h.tactics.List(ctx.teamID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list tactic boards.")
			return
		}
		writeSuccess(w, map[string]any{"boards": boards})
	case http.MethodPost:
		var input tactics.BoardInput
		if !decodeJSON(w, r, &input) {
			return
		}
		board, err := h.tactics.Create(ctx.teamID, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tactic_board", "Tactic board is invalid.")
			return
		}
		writeSuccess(w, map[string]any{"board": board})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

func (h *Handler) tacticsBoardDetail(w http.ResponseWriter, r *http.Request, ctx requestContext) {
	id := strings.TrimPrefix(r.URL.Path, "/api/tactics/boards/")
	switch r.Method {
	case http.MethodPatch:
		var input tactics.BoardInput
		if !decodeJSON(w, r, &input) {
			return
		}
		board, err := h.tactics.Update(ctx.teamID, id, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_tactic_board", "Tactic board update is invalid.")
			return
		}
		writeSuccess(w, map[string]any{"board": board})
	case http.MethodDelete:
		if err := h.tactics.Delete(ctx.teamID, id); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete tactic board.")
			return
		}
		writeSuccess(w, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
	}
}

type requestContext struct {
	user   auth.User
	token  string
	teamID string
}

type playerRequestContext struct {
	player auth.PlayerIdentity
	token  string
	teamID string
}

func (h *Handler) withAuth(next func(http.ResponseWriter, *http.Request, requestContext)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		user, err := h.auth.UserByToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		teamID, err := h.defaultTeamID(user.ID)
		if errors.Is(err, sql.ErrNoRows) {
			teamID, err = h.ensureDefaultTeam(user.ID)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load team workspace.")
			return
		}
		next(w, r, requestContext{user: user, token: token, teamID: teamID})
	})
}

func (h *Handler) withPlayer(next func(http.ResponseWriter, *http.Request, playerRequestContext)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		player, err := h.auth.PlayerByToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Player access is required.")
			return
		}
		next(w, r, playerRequestContext{player: player, token: token, teamID: player.TeamID})
	})
}

func (h *Handler) ensureDefaultTeam(userID string) (string, error) {
	var teamID string
	err := h.db.QueryRow(`SELECT id FROM teams WHERE user_id = ? ORDER BY created_at LIMIT 1`, userID).Scan(&teamID)
	if err == nil {
		if _, err := h.db.Exec(`UPDATE teams SET name = ?, updated_at = ? WHERE id = ?`, "西土城FC", time.Now().Format(time.RFC3339), teamID); err != nil {
			return "", err
		}
		if h.events != nil {
			if err := h.events.EnsureDefaultLocation(teamID); err != nil {
				return "", err
			}
		}
		return teamID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	now := time.Now().Format(time.RFC3339)
	teamID = uuid.NewString()
	_, err = h.db.Exec(
		`INSERT INTO teams (id, user_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		teamID,
		userID,
		"西土城FC",
		now,
		now,
	)
	if err != nil {
		return "", err
	}
	if h.events != nil {
		if err := h.events.EnsureDefaultLocation(teamID); err != nil {
			return "", err
		}
	}
	return teamID, nil
}

func (h *Handler) defaultTeamID(userID string) (string, error) {
	var teamID string
	err := h.db.QueryRow(`SELECT id FROM teams WHERE user_id = ? ORDER BY created_at LIMIT 1`, userID).Scan(&teamID)
	return teamID, err
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return ""
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return false
	}
	return true
}

func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusBadRequest, "invalid_credentials", "Password is invalid.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Authentication failed.")
	}
}
