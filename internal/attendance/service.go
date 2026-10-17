package attendance

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidAttendance = errors.New("invalid attendance")
	ErrEventCancelled    = errors.New("event cancelled")
)

type Record struct {
	PlayerID  string `json:"player_id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	UpdatedAt string `json:"updated_at"`
}

type PlayerEventRecord struct {
	EventID   string `json:"event_id"`
	PlayerID  string `json:"player_id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	UpdatedAt string `json:"updated_at"`
}

type MemberRecord struct {
	PlayerID  string `json:"player_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at,omitempty"`
	HasAvatar bool   `json:"has_avatar"`
}

type Summary struct {
	Available   int `json:"available"`
	Unavailable int `json:"unavailable"`
	Tentative   int `json:"tentative"`
	Unknown     int `json:"unknown"`
}

type UpdateInput struct {
	Records []Record `json:"records"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) List(teamID, eventID string) ([]Record, error) {
	if _, _, err := s.eventTiming(teamID, eventID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT a.player_id, a.status, a.note, a.updated_at FROM attendance_records a JOIN players p ON p.id = a.player_id WHERE a.event_id = ? AND p.team_id = ? AND p.id IN (SELECT player_id FROM wechat_members WHERE profile_complete = 1) ORDER BY p.name, p.id`, eventID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []Record{}
	for rows.Next() {
		var record Record
		if err := rows.Scan(&record.PlayerID, &record.Status, &record.Note, &record.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *Service) ListMembers(teamID, eventID string) ([]MemberRecord, error) {
	var exists int
	if err := s.db.QueryRow(`SELECT 1 FROM events WHERE id = ? AND team_id = ?`, eventID, teamID).Scan(&exists); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT p.id, p.name, COALESCE(a.status, 'unknown'), COALESCE(a.updated_at, ''), w.avatar_file_name <> ''
		FROM players p
		JOIN wechat_members w ON w.player_id = p.id
		LEFT JOIN attendance_records a ON a.player_id = p.id AND a.event_id = ?
		WHERE p.team_id = ? AND p.status = 'active' AND w.profile_complete = 1
		ORDER BY CASE COALESCE(a.status, 'unknown') WHEN 'available' THEN 0 WHEN 'tentative' THEN 1 WHEN 'unavailable' THEN 2 ELSE 3 END, p.name, p.id`, eventID, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []MemberRecord{}
	for rows.Next() {
		var member MemberRecord
		if err := rows.Scan(&member.PlayerID, &member.Name, &member.Status, &member.UpdatedAt, &member.HasAvatar); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Service) Update(teamID, eventID string, input UpdateInput) ([]Record, error) {
	if _, _, err := s.eventTiming(teamID, eventID); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	records := make([]Record, 0, len(input.Records))
	for _, record := range input.Records {
		if record.PlayerID == "" || !validStatus(record.Status) {
			return nil, ErrInvalidAttendance
		}
		if err := ensurePlayer(tx, teamID, record.PlayerID); err != nil {
			return nil, err
		}
		record.Note = strings.TrimSpace(record.Note)
		record.UpdatedAt = nowISO()
		_, err := tx.Exec(`INSERT INTO attendance_records (event_id, player_id, status, note, updated_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(event_id, player_id) DO UPDATE SET status = excluded.status, note = excluded.note, updated_at = excluded.updated_at`, eventID, record.PlayerID, record.Status, record.Note, record.UpdatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Service) GetForPlayer(teamID, eventID, playerID string) (Record, error) {
	if _, _, err := s.eventTiming(teamID, eventID); err != nil {
		return Record{}, err
	}
	var record Record
	err := s.db.QueryRow(`SELECT a.player_id, a.status, a.note, a.updated_at FROM attendance_records a JOIN players p ON p.id = a.player_id WHERE a.event_id = ? AND a.player_id = ? AND p.team_id = ?`, eventID, playerID, teamID).Scan(&record.PlayerID, &record.Status, &record.Note, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{PlayerID: playerID, Status: "unknown"}, nil
	}
	return record, err
}

func (s *Service) SaveForPlayer(teamID, eventID, playerID, status string) (Record, error) {
	if !validStatus(status) {
		return Record{}, ErrInvalidAttendance
	}
	if _, _, err := s.eventTiming(teamID, eventID); err != nil {
		return Record{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	if err := ensurePlayer(tx, teamID, playerID); err != nil {
		return Record{}, err
	}
	record := Record{PlayerID: playerID, Status: status, UpdatedAt: nowISO()}
	_, err = tx.Exec(`INSERT INTO attendance_records (event_id, player_id, status, note, updated_at) VALUES (?, ?, ?, '', ?) ON CONFLICT(event_id, player_id) DO UPDATE SET status = excluded.status, updated_at = excluded.updated_at`, eventID, record.PlayerID, record.Status, record.UpdatedAt)
	if err != nil {
		return Record{}, err
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) ListForPlayer(teamID, playerID string) ([]PlayerEventRecord, Summary, error) {
	if err := s.ensurePlayerInDB(teamID, playerID); err != nil {
		return nil, Summary{}, err
	}
	rows, err := s.db.Query(`SELECT e.id, a.status, a.note, a.updated_at FROM events e LEFT JOIN event_cancellations c ON c.event_id = e.id LEFT JOIN attendance_records a ON a.event_id = e.id AND a.player_id = ? WHERE e.team_id = ? AND c.event_id IS NULL ORDER BY e.starts_at`, playerID, teamID)
	if err != nil {
		return nil, Summary{}, err
	}
	defer rows.Close()
	records := []PlayerEventRecord{}
	var summary Summary
	totalEvents := 0
	for rows.Next() {
		totalEvents++
		var eventID string
		var status, note, updatedAt sql.NullString
		if err := rows.Scan(&eventID, &status, &note, &updatedAt); err != nil {
			return nil, Summary{}, err
		}
		if !status.Valid {
			continue
		}
		record := PlayerEventRecord{EventID: eventID, PlayerID: playerID, Status: status.String, Note: note.String, UpdatedAt: updatedAt.String}
		records = append(records, record)
		addToSummary(&summary, record.Status)
	}
	if err := rows.Err(); err != nil {
		return nil, Summary{}, err
	}
	known := summary.Available + summary.Unavailable + summary.Tentative
	if totalEvents > known {
		summary.Unknown = totalEvents - known
	}
	return records, summary, nil
}

func (s *Service) Summary(teamID, eventID string) (Summary, error) {
	if _, _, err := s.eventTiming(teamID, eventID); err != nil {
		return Summary{}, err
	}
	var totalPlayers int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM players WHERE team_id = ? AND status = 'active' AND id IN (SELECT player_id FROM wechat_members WHERE profile_complete = 1)`, teamID).Scan(&totalPlayers); err != nil {
		return Summary{}, err
	}
	rows, err := s.db.Query(`SELECT a.status FROM attendance_records a JOIN players p ON p.id = a.player_id WHERE a.event_id = ? AND p.team_id = ? AND p.status = 'active' AND p.id IN (SELECT player_id FROM wechat_members WHERE profile_complete = 1)`, eventID, teamID)
	if err != nil {
		return Summary{}, err
	}
	defer rows.Close()
	var summary Summary
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return Summary{}, err
		}
		addToSummary(&summary, status)
	}
	if err := rows.Err(); err != nil {
		return Summary{}, err
	}
	known := summary.Available + summary.Unavailable + summary.Tentative
	if totalPlayers > known {
		summary.Unknown = totalPlayers - known
	}
	return summary, nil
}

func (s *Service) eventTiming(teamID, eventID string) (string, string, error) {
	var startsAt, endsAt string
	var cancelledAt sql.NullString
	err := s.db.QueryRow(`SELECT e.starts_at, e.ends_at, c.cancelled_at FROM events e LEFT JOIN event_cancellations c ON c.event_id = e.id WHERE e.id = ? AND e.team_id = ?`, eventID, teamID).Scan(&startsAt, &endsAt, &cancelledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrInvalidAttendance
	}
	if err != nil {
		return "", "", err
	}
	if cancelledAt.Valid && cancelledAt.String != "" {
		return "", "", ErrEventCancelled
	}
	return startsAt, endsAt, nil
}

func (s *Service) ensurePlayerInDB(teamID, playerID string) error {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM players WHERE id = ? AND team_id = ?`, playerID, teamID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidAttendance
	}
	return err
}

func ensurePlayer(tx *sql.Tx, teamID, playerID string) error {
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM players WHERE id = ? AND team_id = ? AND NOT EXISTS (SELECT 1 FROM wechat_members WHERE player_id = players.id AND profile_complete = 0)`, playerID, teamID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidAttendance
	}
	return err
}

func addToSummary(summary *Summary, status string) {
	switch status {
	case "available":
		summary.Available++
	case "unavailable":
		summary.Unavailable++
	case "tentative":
		summary.Tentative++
	}
}

func validStatus(status string) bool {
	return status == "unknown" || status == "available" || status == "unavailable" || status == "tentative"
}
func nowISO() string { return time.Now().Format(time.RFC3339) }
