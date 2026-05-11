package event

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidEvent = errors.New("invalid event")
var ErrInvalidLocation = errors.New("invalid event location")

const DefaultLocationName = "北京邮电大学（海淀校区）"

type Event struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
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
	EndsAt   string `json:"ends_at"`
	Location string `json:"location"`
	Opponent string `json:"opponent"`
	Notes    string `json:"notes"`
}

type UpdateInput struct {
	Type     *string `json:"type"`
	Title    *string `json:"title"`
	StartsAt *string `json:"starts_at"`
	EndsAt   *string `json:"ends_at"`
	Location *string `json:"location"`
	Opponent *string `json:"opponent"`
	Notes    *string `json:"notes"`
}

type Location struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type LocationInput struct {
	Name string `json:"name"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(teamID string) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, type, title, starts_at, ends_at, location, opponent, notes, created_at, updated_at
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
		`INSERT INTO events (id, team_id, type, title, starts_at, ends_at, location, opponent, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID,
		teamID,
		event.Type,
		event.Title,
		event.StartsAt,
		event.EndsAt,
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
	if input.EndsAt != nil {
		current.EndsAt = strings.TrimSpace(*input.EndsAt)
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
	current, err = normalizeEvent(current)
	if err != nil {
		return Event{}, err
	}
	current.UpdatedAt = nowISO()
	_, err = s.db.Exec(
		`UPDATE events SET type = ?, title = ?, starts_at = ?, ends_at = ?, location = ?, opponent = ?, notes = ?, updated_at = ?
		 WHERE id = ? AND team_id = ?`,
		current.Type,
		current.Title,
		current.StartsAt,
		current.EndsAt,
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
		`SELECT id, type, title, starts_at, ends_at, location, opponent, notes, created_at, updated_at
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
	err := scanner.Scan(
		&event.ID,
		&event.Type,
		&event.Title,
		&event.StartsAt,
		&event.EndsAt,
		&event.Location,
		&event.Opponent,
		&event.Notes,
		&event.CreatedAt,
		&event.UpdatedAt,
	)
	if err != nil {
		return Event{}, err
	}
	event, err = normalizeEvent(event)
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
		EndsAt:   strings.TrimSpace(input.EndsAt),
		Location: strings.TrimSpace(input.Location),
		Opponent: strings.TrimSpace(input.Opponent),
		Notes:    strings.TrimSpace(input.Notes),
	}
	return normalizeEvent(event)
}

func normalizeEvent(event Event) (Event, error) {
	event.Type = strings.TrimSpace(event.Type)
	event.Title = strings.TrimSpace(event.Title)
	event.StartsAt = strings.TrimSpace(event.StartsAt)
	event.EndsAt = strings.TrimSpace(event.EndsAt)
	event.Location = strings.TrimSpace(event.Location)
	event.Opponent = strings.TrimSpace(event.Opponent)
	event.Notes = strings.TrimSpace(event.Notes)
	if event.Type != "training" && event.Type != "friendly" {
		return Event{}, ErrInvalidEvent
	}
	if event.Title == "" || event.StartsAt == "" {
		return Event{}, ErrInvalidEvent
	}
	start, err := time.Parse(time.RFC3339, event.StartsAt)
	if err != nil {
		return Event{}, ErrInvalidEvent
	}
	if event.EndsAt == "" {
		event.EndsAt = start.Add(2 * time.Hour).Format(time.RFC3339)
	}
	end, err := time.Parse(time.RFC3339, event.EndsAt)
	if err != nil || !end.After(start) {
		return Event{}, ErrInvalidEvent
	}
	if event.Location == "" {
		event.Location = DefaultLocationName
	}
	return event, nil
}

func (s *Service) EnsureDefaultLocation(teamID string) error {
	_, err := s.CreateLocation(teamID, LocationInput{Name: DefaultLocationName})
	if errors.Is(err, ErrInvalidLocation) {
		return err
	}
	return nil
}

func (s *Service) ListLocations(teamID string) ([]Location, error) {
	rows, err := s.db.Query(
		`SELECT id, name, created_at, updated_at
		 FROM event_locations
		 WHERE team_id = ?
		 ORDER BY created_at`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locations := []Location{}
	for rows.Next() {
		var location Location
		if err := rows.Scan(&location.ID, &location.Name, &location.CreatedAt, &location.UpdatedAt); err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	return locations, rows.Err()
}

func (s *Service) CreateLocation(teamID string, input LocationInput) (Location, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Location{}, ErrInvalidLocation
	}
	now := nowISO()
	location := Location{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO event_locations (id, team_id, name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		location.ID,
		teamID,
		location.Name,
		location.CreatedAt,
		location.UpdatedAt,
	)
	if err != nil {
		return Location{}, err
	}
	var existing Location
	err = s.db.QueryRow(
		`SELECT id, name, created_at, updated_at FROM event_locations WHERE team_id = ? AND name = ?`,
		teamID,
		name,
	).Scan(&existing.ID, &existing.Name, &existing.CreatedAt, &existing.UpdatedAt)
	if err != nil {
		return Location{}, err
	}
	return existing, nil
}

func (s *Service) DeleteLocation(teamID, id string) error {
	_, err := s.db.Exec(`DELETE FROM event_locations WHERE id = ? AND team_id = ?`, id, teamID)
	return err
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
