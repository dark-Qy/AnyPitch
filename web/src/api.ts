import type { AttendanceDecisionSummary, AttendanceRecord, AttendanceStatus, PlayerEventAttendanceRecord } from "./domain";

export type EventType = "training" | "friendly" | "pickup";
export type User = { id: string; email: string; status: string; created_at: string };
export type Session = { token: string; user: User };
export type Player = { id: string; name: string; number: number | null; positions: string[]; status: "active" | "inactive"; created_at: string; updated_at: string };
export type PlayerSession = { token: string; player: Player };
export type TeamEvent = { id: string; type: EventType; format?: number; title: string; starts_at: string; ends_at: string; location: string; opponent: string; notes: string; cancelled_at?: string; created_at: string; updated_at: string };
export type EventLocation = { id: string; name: string; created_at: string; updated_at: string };
export type Term = { id: string; name: string; starts_on: string; ends_on: string; created_at: string; updated_at: string };
export type LeaderboardEntry = { rank: number; player_id: string; name: string; status: "active" | "inactive"; count: number };
export type Leaderboard = { label: string; starts_at: string; ends_at: string; entries: LeaderboardEntry[] };
export type HistoryRow = { event_id: string; type: EventType; title: string; starts_at: string; ends_at: string; cancelled_at?: string; player_id: string; player_name: string; player_status: "active" | "inactive"; rsvp_status: AttendanceStatus; rsvp_note: string };
export type SpecialActivity = { id: string; name: string; starts_on: string; ends_on: string; summary: string; status: "active" | "archived"; created_at: string; updated_at: string };
export type SpecialActivityEntry = { id: string; special_activity_id: string; title: string; occurs_at: string; location: string; opponent: string; home_score: number | null; away_score: number | null; notes: string; created_at: string; updated_at: string };
export type Album = { id: string; event_id?: string; special_activity_id?: string; source_kind: "event" | "special_activity"; source_title: string; photo_count: number; created_at: string; updated_at: string };
export type Photo = { id: string; album_id: string; caption: string; uploader_player_id?: string; uploader_name: string; uploader_role: "coach" | "player"; content_type: string; width: number; height: number; byte_size: number; created_at: string };
export type TacticSlot = { slot_id: string; label: string; side?: "home" | "opponent"; x: number; y: number; player_id: string };
export type TacticTemplate = { id: string; format: 5 | 8 | 11; name: string; formation: string; slots: TacticSlot[] };
export type TacticBoard = { id: string; name: string; template_id?: string; opponent_template_id?: string; format: 5 | 8 | 11; formation: string; opponent_formation?: string; slots: TacticSlot[]; created_at: string; updated_at: string };

type APIEnvelope<T> = { data?: T; error?: { code: string; message: string } };
type Query = Record<string, string | undefined>;

export class APIClient {
  constructor(private token: string | null) {}
  setToken(token: string | null) { this.token = token; }

  login(password: string) { return this.request<Session>("/api/auth/login", { method: "POST", body: { password } }); }
  logout() { return this.request<{ ok: boolean }>("/api/auth/logout", { method: "POST" }); }
  playerLogout() { return this.request<{ ok: boolean }>("/api/player/logout", { method: "POST" }); }
  me() { return this.request<{ user: User; team_id: string }>("/api/auth/me"); }
  playerMe() { return this.request<{ player: Player }>("/api/player/me"); }

  listPlayers() { return this.request<{ players: Player[] }>("/api/players"); }
  updatePlayer(playerID: string, input: { name?: string; number?: number | null; positions?: string[]; status?: "active" | "inactive" }) { return this.request<{ player: Player }>(`/api/players/${playerID}`, { method: "PATCH", body: input }); }
  deletePlayer(playerID: string) { return this.request<{ ok: boolean }>(`/api/players/${playerID}`, { method: "DELETE" }); }

  listEvents() { return this.request<{ events: TeamEvent[] }>("/api/events"); }
  listPlayerEvents() { return this.request<{ events: TeamEvent[]; attendance_records: PlayerEventAttendanceRecord[]; attendance_summary: AttendanceDecisionSummary }>("/api/player/events"); }
  createEvent(input: Pick<TeamEvent, "type" | "format" | "title" | "starts_at" | "ends_at" | "location" | "opponent" | "notes">) { return this.request<{ event: TeamEvent }>("/api/events", { method: "POST", body: input }); }
  updateEvent(eventID: string, input: Partial<Pick<TeamEvent, "type" | "format" | "title" | "starts_at" | "ends_at" | "location" | "opponent" | "notes">>) { return this.request<{ event: TeamEvent }>(`/api/events/${eventID}`, { method: "PATCH", body: input }); }
  deleteEvent(eventID: string) { return this.request<{ ok: boolean }>(`/api/events/${eventID}`, { method: "DELETE" }); }
  cancelEvent(eventID: string) { return this.request<{ event: TeamEvent }>(`/api/events/${eventID}/cancel`, { method: "POST" }); }
  restoreEvent(eventID: string) { return this.request<{ event: TeamEvent }>(`/api/events/${eventID}/restore`, { method: "POST" }); }
  listLocations() { return this.request<{ locations: EventLocation[] }>("/api/locations"); }
  createLocation(input: { name: string }) { return this.request<{ location: EventLocation }>("/api/locations", { method: "POST", body: input }); }
  deleteLocation(locationID: string) { return this.request<{ ok: boolean }>(`/api/locations/${locationID}`, { method: "DELETE" }); }

  listAttendance(eventID: string) { return this.request<{ records: AttendanceRecord[] }>(`/api/events/${eventID}/attendance`); }
  saveAttendance(eventID: string, record: Pick<AttendanceRecord, "player_id" | "status" | "note">) { return this.request<{ records: AttendanceRecord[] }>(`/api/events/${eventID}/attendance`, { method: "PUT", body: { records: [record] } }); }
  getPlayerAttendance(eventID: string) { return this.request<{ record: AttendanceRecord; summary: AttendanceDecisionSummary }>(`/api/player/events/${eventID}/attendance`); }
  savePlayerAttendance(eventID: string, status: AttendanceStatus) { return this.request<{ record: AttendanceRecord; summary: AttendanceDecisionSummary }>(`/api/player/events/${eventID}/attendance`, { method: "PUT", body: { status } }); }

  listTerms(player = false) { return this.request<{ terms: Term[] }>(player ? "/api/player/terms" : "/api/terms"); }
  createTerm(input: Pick<Term, "name" | "starts_on" | "ends_on">) { return this.request<{ term: Term }>("/api/terms", { method: "POST", body: input }); }
  updateTerm(id: string, input: Partial<Pick<Term, "name" | "starts_on" | "ends_on">>) { return this.request<{ term: Term }>(`/api/terms/${id}`, { method: "PATCH", body: input }); }
  deleteTerm(id: string) { return this.request<{ ok: boolean }>(`/api/terms/${id}`, { method: "DELETE" }); }
  leaderboard(query: { month?: string; term_id?: string; year?: string; half?: string }, player = false) { return this.request<{ leaderboard: Leaderboard }>(this.withQuery(player ? "/api/player/stats/leaderboard" : "/api/stats/leaderboard", query)); }
  history(query: Query = {}, player = false) { return this.request<{ history: HistoryRow[] }>(this.withQuery(player ? "/api/player/stats/me" : "/api/stats/history", query)); }
  downloadHistory(query: Query = {}) { return this.raw(this.withQuery("/api/stats/history.csv", query)); }

  listSpecialActivities(player = false) { return this.request<{ activities: SpecialActivity[] }>(player ? "/api/player/special-activities" : "/api/special-activities"); }
  getSpecialActivity(id: string, player = false) { return this.request<{ activity: SpecialActivity }>(`${player ? "/api/player/special-activities" : "/api/special-activities"}/${id}`); }
  createSpecialActivity(input: Pick<SpecialActivity, "name" | "starts_on" | "ends_on" | "summary" | "status">) { return this.request<{ activity: SpecialActivity }>("/api/special-activities", { method: "POST", body: input }); }
  updateSpecialActivity(id: string, input: Partial<Pick<SpecialActivity, "name" | "starts_on" | "ends_on" | "summary" | "status">>) { return this.request<{ activity: SpecialActivity }>(`/api/special-activities/${id}`, { method: "PATCH", body: input }); }
  deleteSpecialActivity(id: string) { return this.request<{ ok: boolean }>(`/api/special-activities/${id}`, { method: "DELETE" }); }
  listSpecialEntries(id: string, player = false) { return this.request<{ entries: SpecialActivityEntry[] }>(`${player ? "/api/player/special-activities" : "/api/special-activities"}/${id}/entries`); }
  createSpecialEntry(id: string, input: Pick<SpecialActivityEntry, "title" | "occurs_at" | "location" | "opponent" | "home_score" | "away_score" | "notes">) { return this.request<{ entry: SpecialActivityEntry }>(`/api/special-activities/${id}/entries`, { method: "POST", body: input }); }
  updateSpecialEntry(activityID: string, id: string, input: Pick<SpecialActivityEntry, "title" | "occurs_at" | "location" | "opponent" | "home_score" | "away_score" | "notes">) { return this.request<{ entry: SpecialActivityEntry }>(`/api/special-activities/${activityID}/entries/${id}`, { method: "PUT", body: input }); }
  deleteSpecialEntry(activityID: string, id: string) { return this.request<{ ok: boolean }>(`/api/special-activities/${activityID}/entries/${id}`, { method: "DELETE" }); }

  listAlbums(player = false) { return this.request<{ albums: Album[] }>(player ? "/api/player/albums" : "/api/albums"); }
  ensureAlbum(input: { event_id?: string; special_activity_id?: string }, player = false) { return this.request<{ album: Album }>(player ? "/api/player/albums" : "/api/albums", { method: "POST", body: input }); }
  listPhotos(albumID: string, player = false) { return this.request<{ photos: Photo[] }>(`${player ? "/api/player/albums" : "/api/albums"}/${albumID}/photos`); }
  uploadPhoto(albumID: string, file: File, caption: string, player = false) { const form = new FormData(); form.set("file", file); form.set("caption", caption); return this.multipart<{ photo: Photo }>(`${player ? "/api/player/albums" : "/api/albums"}/${albumID}/photos`, form); }
  photoBlob(photoID: string, thumbnail = false, player = false) { return this.raw(`${player ? "/api/player/photos" : "/api/photos"}/${photoID}${thumbnail ? "?variant=thumb" : ""}`); }
  updatePhotoCaption(photoID: string, caption: string, player = false) { return this.request<{ photo: Photo }>(`${player ? "/api/player/photos" : "/api/photos"}/${photoID}`, { method: "PATCH", body: { caption } }); }
  deletePhoto(photoID: string, player = false) { return this.request<{ ok: boolean }>(`${player ? "/api/player/photos" : "/api/photos"}/${photoID}`, { method: "DELETE" }); }

  templates() { return this.request<{ templates: TacticTemplate[] }>("/api/tactics/templates"); }
  listBoards() { return this.request<{ boards: TacticBoard[] }>("/api/tactics/boards"); }
  createBoard(input: { name: string; template_id?: string; opponent_template_id?: string; format: 5 | 8 | 11; formation: string; opponent_formation?: string; slots: TacticSlot[] }) { return this.request<{ board: TacticBoard }>("/api/tactics/boards", { method: "POST", body: input }); }

  private withQuery(path: string, query: Query) { const params = new URLSearchParams(); Object.entries(query).forEach(([key, value]) => { if (value) params.set(key, value); }); return params.size ? `${path}?${params.toString()}` : path; }
  private headers(json = true) { const headers: Record<string, string> = {}; if (json) headers["Content-Type"] = "application/json"; if (this.token) headers.Authorization = `Bearer ${this.token}`; return headers; }
  private async raw(path: string) { const response = await fetch(path, { headers: this.headers(false) }); if (!response.ok) { const envelope = (await response.json().catch(() => ({}))) as APIEnvelope<never>; throw new Error(envelope.error?.message ?? `Request failed: ${response.status}`); } return response.blob(); }
  private async multipart<T>(path: string, body: FormData): Promise<T> { const response = await fetch(path, { method: "POST", headers: this.headers(false), body }); return this.decode<T>(response); }
  private async request<T>(path: string, init: { method?: string; body?: unknown } = {}): Promise<T> { const response = await fetch(path, { method: init.method ?? "GET", headers: this.headers(), body: init.body === undefined ? undefined : JSON.stringify(init.body) }); return this.decode<T>(response); }
  private async decode<T>(response: Response): Promise<T> { const envelope = (await response.json()) as APIEnvelope<T>; if (!response.ok || envelope.error) throw new Error(envelope.error?.message ?? `Request failed: ${response.status}`); if (envelope.data === undefined) throw new Error("Malformed API response"); return envelope.data; }
}
