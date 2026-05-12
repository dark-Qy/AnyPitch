package attendance

import (
	"database/sql"
	"errors"
	"time"
)

var ErrInvalidAttendance = errors.New("invalid attendance")

type Record struct {
	PlayerID  string `json:"player_id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
	UpdatedAt string `json:"updated_at"`
}

type ReplaceInput struct {
	Records []Record `json:"records"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(teamID, eventID string) ([]Record, error) {
	if err := s.ensureEvent(teamID, eventID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT a.player_id, a.status, a.note, a.updated_at
		 FROM attendance_records a
		 JOIN players p ON p.id = a.player_id
		 WHERE a.event_id = ? AND p.team_id = ?
		 ORDER BY p.name`,
		eventID,
		teamID,
	)
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

func (s *Service) Replace(teamID, eventID string, input ReplaceInput) ([]Record, error) {
	if err := s.ensureEvent(teamID, eventID); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	records := make([]Record, 0, len(input.Records))
	for _, record := range input.Records {
		if !validStatus(record.Status) || record.PlayerID == "" {
			return nil, ErrInvalidAttendance
		}
		if err := ensurePlayer(tx, teamID, record.PlayerID); err != nil {
			return nil, err
		}
		record.UpdatedAt = nowISO()
		_, err := tx.Exec(
			`INSERT INTO attendance_records (event_id, player_id, status, note, updated_at)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(event_id, player_id) DO UPDATE SET
			 status = excluded.status,
			 note = excluded.note,
			 updated_at = excluded.updated_at`,
			eventID,
			record.PlayerID,
			record.Status,
			record.Note,
			record.UpdatedAt,
		)
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
	if err := s.ensureEvent(teamID, eventID); err != nil {
		return Record{}, err
	}
	var record Record
	err := s.db.QueryRow(
		`SELECT a.player_id, a.status, a.note, a.updated_at
		 FROM attendance_records a
		 JOIN players p ON p.id = a.player_id
		 WHERE a.event_id = ? AND a.player_id = ? AND p.team_id = ?`,
		eventID,
		playerID,
		teamID,
	).Scan(&record.PlayerID, &record.Status, &record.Note, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{PlayerID: playerID, Status: "unknown", Note: "", UpdatedAt: ""}, nil
	}
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) SaveForPlayer(teamID, eventID, playerID, status string) (Record, error) {
	if !validPlayerStatus(status) {
		return Record{}, ErrInvalidAttendance
	}
	if err := s.ensureEvent(teamID, eventID); err != nil {
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
	record := Record{PlayerID: playerID, Status: status, Note: "", UpdatedAt: nowISO()}
	_, err = tx.Exec(
		`INSERT INTO attendance_records (event_id, player_id, status, note, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(event_id, player_id) DO UPDATE SET
		 status = excluded.status,
		 note = excluded.note,
		 updated_at = excluded.updated_at`,
		eventID,
		record.PlayerID,
		record.Status,
		record.Note,
		record.UpdatedAt,
	)
	if err != nil {
		return Record{}, err
	}
	if err := tx.Commit(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) ensureEvent(teamID, eventID string) error {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM events WHERE id = ? AND team_id = ?`, eventID, teamID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidAttendance
	}
	return err
}

func ensurePlayer(tx *sql.Tx, teamID, playerID string) error {
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM players WHERE id = ? AND team_id = ?`, playerID, teamID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidAttendance
	}
	return err
}

func validStatus(status string) bool {
	switch status {
	case "unknown", "available", "unavailable", "late", "injured", "present", "absent", "excused":
		return true
	default:
		return false
	}
}

func validPlayerStatus(status string) bool {
	switch status {
	case "unknown", "available", "unavailable":
		return true
	default:
		return false
	}
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
