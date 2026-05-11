package event

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidEvent = errors.New("invalid event")

type Event struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	StartsAt  string `json:"starts_at"`
	Location  string `json:"location"`
	Opponent  string `json:"opponent"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreateInput struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	StartsAt string `json:"starts_at"`
	Location string `json:"location"`
	Opponent string `json:"opponent"`
	Notes    string `json:"notes"`
}

type UpdateInput struct {
	Type     *string `json:"type"`
	Title    *string `json:"title"`
	StartsAt *string `json:"starts_at"`
	Location *string `json:"location"`
	Opponent *string `json:"opponent"`
	Notes    *string `json:"notes"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(teamID string) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, type, title, starts_at, location, opponent, notes, created_at, updated_at
		 FROM events
		 WHERE team_id = ?
		 ORDER BY starts_at`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Service) Create(teamID string, input CreateInput) (Event, error) {
	event, err := normalizeCreate(input)
	if err != nil {
		return Event{}, err
	}
	now := nowISO()
	event.ID = uuid.NewString()
	event.CreatedAt = now
	event.UpdatedAt = now
	_, err = s.db.Exec(
		`INSERT INTO events (id, team_id, type, title, starts_at, location, opponent, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		teamID,
		event.Type,
		event.Title,
		event.StartsAt,
		event.Location,
		event.Opponent,
		event.Notes,
		event.CreatedAt,
		event.UpdatedAt,
	)
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

func (s *Service) Update(teamID, id string, input UpdateInput) (Event, error) {
	current, err := s.Get(teamID, id)
	if err != nil {
		return Event{}, err
	}
	if input.Type != nil {
		current.Type = strings.TrimSpace(*input.Type)
	}
	if input.Title != nil {
		current.Title = strings.TrimSpace(*input.Title)
	}
	if input.StartsAt != nil {
		current.StartsAt = strings.TrimSpace(*input.StartsAt)
	}
	if input.Location != nil {
		current.Location = strings.TrimSpace(*input.Location)
	}
	if input.Opponent != nil {
		current.Opponent = strings.TrimSpace(*input.Opponent)
	}
	if input.Notes != nil {
		current.Notes = strings.TrimSpace(*input.Notes)
	}
	if err := validateEvent(current); err != nil {
		return Event{}, err
	}
	current.UpdatedAt = nowISO()
	_, err = s.db.Exec(
		`UPDATE events SET type = ?, title = ?, starts_at = ?, location = ?, opponent = ?, notes = ?, updated_at = ?
		 WHERE id = ? AND team_id = ?`,
		current.Type,
		current.Title,
		current.StartsAt,
		current.Location,
		current.Opponent,
		current.Notes,
		current.UpdatedAt,
		id,
		teamID,
	)
	if err != nil {
		return Event{}, err
	}
	return current, nil
}

func (s *Service) Delete(teamID, id string) error {
	_, err := s.db.Exec(`DELETE FROM events WHERE id = ? AND team_id = ?`, id, teamID)
	return err
}

func (s *Service) Get(teamID, id string) (Event, error) {
	row := s.db.QueryRow(
		`SELECT id, type, title, starts_at, location, opponent, notes, created_at, updated_at
		 FROM events
		 WHERE id = ? AND team_id = ?`,
		id,
		teamID,
	)
	return scanEvent(row)
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner eventScanner) (Event, error) {
	var event Event
	err := scanner.Scan(&event.ID, &event.Type, &event.Title, &event.StartsAt, &event.Location, &event.Opponent, &event.Notes, &event.CreatedAt, &event.UpdatedAt)
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

func normalizeCreate(input CreateInput) (Event, error) {
	event := Event{
		Type:     strings.TrimSpace(input.Type),
		Title:    strings.TrimSpace(input.Title),
		StartsAt: strings.TrimSpace(input.StartsAt),
		Location: strings.TrimSpace(input.Location),
		Opponent: strings.TrimSpace(input.Opponent),
		Notes:    strings.TrimSpace(input.Notes),
	}
	if err := validateEvent(event); err != nil {
		return Event{}, err
	}
	return event, nil
}

func validateEvent(event Event) error {
	if event.Type != "training" && event.Type != "friendly" {
		return ErrInvalidEvent
	}
	if event.Title == "" || event.StartsAt == "" {
		return ErrInvalidEvent
	}
	if _, err := time.Parse(time.RFC3339, event.StartsAt); err != nil {
		return ErrInvalidEvent
	}
	return nil
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
