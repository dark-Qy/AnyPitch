package specialactivity

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidActivity = errors.New("invalid special activity")
var ErrInvalidEntry = errors.New("invalid special activity entry")

type Activity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartsOn  string `json:"starts_on"`
	EndsOn    string `json:"ends_on"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ActivityInput struct {
	Name     string `json:"name"`
	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
}

type ActivityUpdateInput struct {
	Name     *string `json:"name"`
	StartsOn *string `json:"starts_on"`
	EndsOn   *string `json:"ends_on"`
	Summary  *string `json:"summary"`
	Status   *string `json:"status"`
}

type Entry struct {
	ID                string `json:"id"`
	SpecialActivityID string `json:"special_activity_id"`
	Title             string `json:"title"`
	OccursAt          string `json:"occurs_at"`
	Location          string `json:"location"`
	Opponent          string `json:"opponent"`
	HomeScore         *int   `json:"home_score"`
	AwayScore         *int   `json:"away_score"`
	Notes             string `json:"notes"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type EntryInput struct {
	Title     string `json:"title"`
	OccursAt  string `json:"occurs_at"`
	Location  string `json:"location"`
	Opponent  string `json:"opponent"`
	HomeScore *int   `json:"home_score"`
	AwayScore *int   `json:"away_score"`
	Notes     string `json:"notes"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) List(teamID string) ([]Activity, error) {
	rows, err := s.db.Query(`SELECT id, name, starts_on, ends_on, summary, status, created_at, updated_at FROM special_activities WHERE team_id = ? ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END, starts_on DESC, id`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	activities := []Activity{}
	for rows.Next() {
		activity, err := scanActivity(rows)
		if err != nil {
			return nil, err
		}
		activities = append(activities, activity)
	}
	return activities, rows.Err()
}

func (s *Service) Get(teamID, id string) (Activity, error) {
	activity, err := scanActivity(s.db.QueryRow(`SELECT id, name, starts_on, ends_on, summary, status, created_at, updated_at FROM special_activities WHERE id = ? AND team_id = ?`, id, teamID))
	if errors.Is(err, sql.ErrNoRows) {
		return Activity{}, ErrInvalidActivity
	}
	return activity, err
}

func (s *Service) Create(teamID string, input ActivityInput) (Activity, error) {
	activity, err := normalizeActivity(Activity{Name: input.Name, StartsOn: input.StartsOn, EndsOn: input.EndsOn, Summary: input.Summary, Status: input.Status})
	if err != nil {
		return Activity{}, err
	}
	now := nowISO()
	activity.ID, activity.CreatedAt, activity.UpdatedAt = uuid.NewString(), now, now
	_, err = s.db.Exec(`INSERT INTO special_activities (id, team_id, name, starts_on, ends_on, summary, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, activity.ID, teamID, activity.Name, activity.StartsOn, activity.EndsOn, activity.Summary, activity.Status, now, now)
	return activity, err
}

func (s *Service) Update(teamID, id string, input ActivityUpdateInput) (Activity, error) {
	activity, err := s.Get(teamID, id)
	if err != nil {
		return Activity{}, err
	}
	if input.Name != nil {
		activity.Name = *input.Name
	}
	if input.StartsOn != nil {
		activity.StartsOn = *input.StartsOn
	}
	if input.EndsOn != nil {
		activity.EndsOn = *input.EndsOn
	}
	if input.Summary != nil {
		activity.Summary = *input.Summary
	}
	if input.Status != nil {
		activity.Status = *input.Status
	}
	activity, err = normalizeActivity(activity)
	if err != nil {
		return Activity{}, err
	}
	activity.UpdatedAt = nowISO()
	_, err = s.db.Exec(`UPDATE special_activities SET name = ?, starts_on = ?, ends_on = ?, summary = ?, status = ?, updated_at = ? WHERE id = ? AND team_id = ?`, activity.Name, activity.StartsOn, activity.EndsOn, activity.Summary, activity.Status, activity.UpdatedAt, id, teamID)
	return activity, err
}

func (s *Service) Delete(teamID, id string) error {
	var references int
	err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM special_activity_entries WHERE special_activity_id = ?) + (SELECT COUNT(*) FROM albums WHERE special_activity_id = ?)`, id, id).Scan(&references)
	if err != nil {
		return err
	}
	if references > 0 {
		return ErrInvalidActivity
	}
	_, err = s.db.Exec(`DELETE FROM special_activities WHERE id = ? AND team_id = ?`, id, teamID)
	return err
}

func (s *Service) ListEntries(teamID, activityID string) ([]Entry, error) {
	if _, err := s.Get(teamID, activityID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id, special_activity_id, title, occurs_at, location, opponent, home_score, away_score, notes, created_at, updated_at FROM special_activity_entries WHERE special_activity_id = ? ORDER BY occurs_at, id`, activityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []Entry{}
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *Service) CreateEntry(teamID, activityID string, input EntryInput) (Entry, error) {
	if _, err := s.Get(teamID, activityID); err != nil {
		return Entry{}, err
	}
	entry, err := normalizeEntry(Entry{SpecialActivityID: activityID, Title: input.Title, OccursAt: input.OccursAt, Location: input.Location, Opponent: input.Opponent, HomeScore: input.HomeScore, AwayScore: input.AwayScore, Notes: input.Notes})
	if err != nil {
		return Entry{}, err
	}
	now := nowISO()
	entry.ID, entry.CreatedAt, entry.UpdatedAt = uuid.NewString(), now, now
	_, err = s.db.Exec(`INSERT INTO special_activity_entries (id, special_activity_id, title, occurs_at, location, opponent, home_score, away_score, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, entry.ID, activityID, entry.Title, entry.OccursAt, entry.Location, entry.Opponent, entry.HomeScore, entry.AwayScore, entry.Notes, now, now)
	return entry, err
}

func (s *Service) UpdateEntry(teamID, activityID, id string, input EntryInput) (Entry, error) {
	if _, err := s.Get(teamID, activityID); err != nil {
		return Entry{}, err
	}
	entry, err := s.entry(activityID, id)
	if err != nil {
		return Entry{}, err
	}
	entry.Title, entry.OccursAt, entry.Location, entry.Opponent, entry.HomeScore, entry.AwayScore, entry.Notes = input.Title, input.OccursAt, input.Location, input.Opponent, input.HomeScore, input.AwayScore, input.Notes
	entry, err = normalizeEntry(entry)
	if err != nil {
		return Entry{}, err
	}
	entry.UpdatedAt = nowISO()
	_, err = s.db.Exec(`UPDATE special_activity_entries SET title = ?, occurs_at = ?, location = ?, opponent = ?, home_score = ?, away_score = ?, notes = ?, updated_at = ? WHERE id = ? AND special_activity_id = ?`, entry.Title, entry.OccursAt, entry.Location, entry.Opponent, entry.HomeScore, entry.AwayScore, entry.Notes, entry.UpdatedAt, id, activityID)
	return entry, err
}

func (s *Service) DeleteEntry(teamID, activityID, id string) error {
	if _, err := s.Get(teamID, activityID); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM special_activity_entries WHERE id = ? AND special_activity_id = ?`, id, activityID)
	return err
}

type scanner interface{ Scan(...any) error }

func scanActivity(row scanner) (Activity, error) {
	var activity Activity
	err := row.Scan(&activity.ID, &activity.Name, &activity.StartsOn, &activity.EndsOn, &activity.Summary, &activity.Status, &activity.CreatedAt, &activity.UpdatedAt)
	return activity, err
}

func scanEntry(row scanner) (Entry, error) {
	var entry Entry
	var home, away sql.NullInt64
	err := row.Scan(&entry.ID, &entry.SpecialActivityID, &entry.Title, &entry.OccursAt, &entry.Location, &entry.Opponent, &home, &away, &entry.Notes, &entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		return Entry{}, err
	}
	if home.Valid {
		value := int(home.Int64)
		entry.HomeScore = &value
	}
	if away.Valid {
		value := int(away.Int64)
		entry.AwayScore = &value
	}
	return entry, nil
}

func (s *Service) entry(activityID, id string) (Entry, error) {
	entry, err := scanEntry(s.db.QueryRow(`SELECT id, special_activity_id, title, occurs_at, location, opponent, home_score, away_score, notes, created_at, updated_at FROM special_activity_entries WHERE id = ? AND special_activity_id = ?`, id, activityID))
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, ErrInvalidEntry
	}
	return entry, err
}

func normalizeActivity(activity Activity) (Activity, error) {
	activity.Name, activity.StartsOn, activity.EndsOn = strings.TrimSpace(activity.Name), strings.TrimSpace(activity.StartsOn), strings.TrimSpace(activity.EndsOn)
	activity.Summary, activity.Status = strings.TrimSpace(activity.Summary), strings.TrimSpace(activity.Status)
	if activity.Status == "" {
		activity.Status = "active"
	}
	if activity.Name == "" || (activity.Status != "active" && activity.Status != "archived") {
		return Activity{}, ErrInvalidActivity
	}
	start, err := time.Parse("2006-01-02", activity.StartsOn)
	if err != nil {
		return Activity{}, ErrInvalidActivity
	}
	end, err := time.Parse("2006-01-02", activity.EndsOn)
	if err != nil || end.Before(start) {
		return Activity{}, ErrInvalidActivity
	}
	return activity, nil
}

func normalizeEntry(entry Entry) (Entry, error) {
	entry.Title, entry.OccursAt, entry.Location = strings.TrimSpace(entry.Title), strings.TrimSpace(entry.OccursAt), strings.TrimSpace(entry.Location)
	entry.Opponent, entry.Notes = strings.TrimSpace(entry.Opponent), strings.TrimSpace(entry.Notes)
	if entry.Title == "" || entry.OccursAt == "" {
		return Entry{}, ErrInvalidEntry
	}
	if _, err := time.Parse(time.RFC3339, entry.OccursAt); err != nil {
		return Entry{}, ErrInvalidEntry
	}
	if (entry.HomeScore == nil) != (entry.AwayScore == nil) {
		return Entry{}, ErrInvalidEntry
	}
	if entry.HomeScore != nil && (*entry.HomeScore < 0 || *entry.AwayScore < 0) {
		return Entry{}, ErrInvalidEntry
	}
	return entry, nil
}

func nowISO() string { return time.Now().Format(time.RFC3339) }
