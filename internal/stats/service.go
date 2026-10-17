package stats

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidTerm = errors.New("invalid term")
var ErrOverlappingTerm = errors.New("overlapping term")
var ErrInvalidRange = errors.New("invalid statistics range")

var chinaLocation = mustChinaLocation()

type Term struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartsOn  string `json:"starts_on"`
	EndsOn    string `json:"ends_on"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type TermInput struct {
	Name     string `json:"name"`
	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on"`
}

type TermUpdateInput struct {
	Name     *string `json:"name"`
	StartsOn *string `json:"starts_on"`
	EndsOn   *string `json:"ends_on"`
}

type LeaderboardQuery struct {
	Month  string
	Year   string
	Half   string
	TermID string
}

type LeaderboardEntry struct {
	Rank      int    `json:"rank"`
	PlayerID  string `json:"player_id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	HasAvatar bool   `json:"has_avatar"`
	Count     int    `json:"count"`
}

type Leaderboard struct {
	Label    string             `json:"label"`
	StartsAt string             `json:"starts_at"`
	EndsAt   string             `json:"ends_at"`
	Entries  []LeaderboardEntry `json:"entries"`
}

type HistoryFilter struct {
	PlayerID string
	Type     string
	From     string
	To       string
}

type HistoryRow struct {
	EventID      string `json:"event_id"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	StartsAt     string `json:"starts_at"`
	EndsAt       string `json:"ends_at"`
	CancelledAt  string `json:"cancelled_at,omitempty"`
	PlayerID     string `json:"player_id"`
	PlayerName   string `json:"player_name"`
	PlayerStatus string `json:"player_status"`
	RSVPStatus   string `json:"rsvp_status"`
	RSVPNote     string `json:"rsvp_note"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) ListTerms(teamID string) ([]Term, error) {
	rows, err := s.db.Query(`SELECT id, name, starts_on, ends_on, created_at, updated_at FROM terms WHERE team_id = ? ORDER BY starts_on DESC, id`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	terms := []Term{}
	for rows.Next() {
		var term Term
		if err := rows.Scan(&term.ID, &term.Name, &term.StartsOn, &term.EndsOn, &term.CreatedAt, &term.UpdatedAt); err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	return terms, rows.Err()
}

func (s *Service) CreateTerm(teamID string, input TermInput) (Term, error) {
	term, err := normalizeTerm(Term{Name: input.Name, StartsOn: input.StartsOn, EndsOn: input.EndsOn})
	if err != nil {
		return Term{}, err
	}
	if err := s.ensureNoOverlap(teamID, "", term.StartsOn, term.EndsOn); err != nil {
		return Term{}, err
	}
	now := nowISO()
	term.ID, term.CreatedAt, term.UpdatedAt = uuid.NewString(), now, now
	_, err = s.db.Exec(`INSERT INTO terms (id, team_id, name, starts_on, ends_on, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, term.ID, teamID, term.Name, term.StartsOn, term.EndsOn, now, now)
	return term, err
}

func (s *Service) UpdateTerm(teamID, id string, input TermUpdateInput) (Term, error) {
	term, err := s.term(teamID, id)
	if err != nil {
		return Term{}, err
	}
	if input.Name != nil {
		term.Name = *input.Name
	}
	if input.StartsOn != nil {
		term.StartsOn = *input.StartsOn
	}
	if input.EndsOn != nil {
		term.EndsOn = *input.EndsOn
	}
	term, err = normalizeTerm(term)
	if err != nil {
		return Term{}, err
	}
	if err := s.ensureNoOverlap(teamID, id, term.StartsOn, term.EndsOn); err != nil {
		return Term{}, err
	}
	term.UpdatedAt = nowISO()
	_, err = s.db.Exec(`UPDATE terms SET name = ?, starts_on = ?, ends_on = ?, updated_at = ? WHERE id = ? AND team_id = ?`, term.Name, term.StartsOn, term.EndsOn, term.UpdatedAt, id, teamID)
	return term, err
}

func (s *Service) DeleteTerm(teamID, id string) error {
	_, err := s.db.Exec(`DELETE FROM terms WHERE id = ? AND team_id = ?`, id, teamID)
	return err
}

func (s *Service) Leaderboard(teamID string, query LeaderboardQuery) (Leaderboard, error) {
	startsAt, endsAt, label, err := s.rangeFor(teamID, query)
	if err != nil {
		return Leaderboard{}, err
	}
	counts, err := s.attendanceCounts(teamID, startsAt, endsAt)
	if err != nil {
		return Leaderboard{}, err
	}
	rows, err := s.db.Query(`SELECT p.id, p.name, p.status, m.avatar_file_name <> '' FROM players p JOIN wechat_members m ON m.player_id=p.id WHERE p.team_id = ? AND m.profile_complete=1 ORDER BY p.name, p.id`, teamID)
	if err != nil {
		return Leaderboard{}, err
	}
	defer rows.Close()
	entries := []LeaderboardEntry{}
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.PlayerID, &entry.Name, &entry.Status, &entry.HasAvatar); err != nil {
			return Leaderboard{}, err
		}
		entry.Count = counts[entry.PlayerID]
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return Leaderboard{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].PlayerID < entries[j].PlayerID
	})
	rank, previousCount := 0, -1
	for index := range entries {
		if entries[index].Count != previousCount {
			rank = index + 1
			previousCount = entries[index].Count
		}
		entries[index].Rank = rank
	}
	return Leaderboard{Label: label, StartsAt: startsAt.Format(time.RFC3339), EndsAt: endsAt.Format(time.RFC3339), Entries: entries}, nil
}

func (s *Service) History(teamID string, filter HistoryFilter) ([]HistoryRow, error) {
	if filter.Type != "" && filter.Type != "training" && filter.Type != "friendly" && filter.Type != "pickup" {
		return nil, ErrInvalidRange
	}
	from, to, err := historyRange(filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	args := []any{teamID}
	query := `SELECT e.id, e.type, e.title, e.starts_at, e.ends_at, COALESCE(cancelled.cancelled_at, ''),
        p.id, p.name, p.status, a.status, a.note
        FROM attendance_records a JOIN events e ON e.id=a.event_id
        JOIN players p ON p.id=a.player_id AND p.team_id=e.team_id
        JOIN wechat_members m ON m.player_id=p.id
        LEFT JOIN event_cancellations cancelled ON cancelled.event_id=e.id
        WHERE e.team_id = ?`
	if filter.PlayerID != "" {
		query += ` AND p.id = ?`
		args = append(args, filter.PlayerID)
	}
	query += ` ORDER BY e.starts_at DESC, p.name, p.id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := []HistoryRow{}
	for rows.Next() {
		var row HistoryRow
		if err := rows.Scan(&row.EventID, &row.Type, &row.Title, &row.StartsAt, &row.EndsAt, &row.CancelledAt, &row.PlayerID, &row.PlayerName, &row.PlayerStatus, &row.RSVPStatus, &row.RSVPNote); err != nil {
			return nil, err
		}
		startsAt, parseErr := time.Parse(time.RFC3339, row.StartsAt)
		if parseErr != nil {
			return nil, parseErr
		}
		if filter.Type != "" && row.Type != filter.Type {
			continue
		}
		if !from.IsZero() && startsAt.Before(from) {
			continue
		}
		if !to.IsZero() && !startsAt.Before(to) {
			continue
		}
		history = append(history, row)
	}
	return history, rows.Err()
}

func (s *Service) rangeFor(teamID string, query LeaderboardQuery) (time.Time, time.Time, string, error) {
	choices := 0
	for _, value := range []string{query.Month, query.TermID, query.Year} {
		if value != "" {
			choices++
		}
	}
	if choices != 1 || (query.Half != "" && (query.Year == "" || (query.Half != "1" && query.Half != "2"))) {
		return time.Time{}, time.Time{}, "", ErrInvalidRange
	}
	if query.Year != "" {
		start, err := time.ParseInLocation("2006", query.Year, chinaLocation)
		if err != nil || start.Year() < 1 {
			return time.Time{}, time.Time{}, "", ErrInvalidRange
		}
		if query.Half == "1" {
			return start, start.AddDate(0, 6, 0), query.Year + "年上半年", nil
		}
		if query.Half == "2" {
			return start.AddDate(0, 6, 0), start.AddDate(1, 0, 0), query.Year + "年下半年", nil
		}
		return start, start.AddDate(1, 0, 0), query.Year + "年度", nil
	}
	if query.Month != "" {
		start, err := time.ParseInLocation("2006-01", query.Month, chinaLocation)
		if err != nil {
			return time.Time{}, time.Time{}, "", ErrInvalidRange
		}
		return start, start.AddDate(0, 1, 0), start.Format("2006年1月"), nil
	}
	term, err := s.term(teamID, query.TermID)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	start, _ := time.ParseInLocation("2006-01-02", term.StartsOn, chinaLocation)
	end, _ := time.ParseInLocation("2006-01-02", term.EndsOn, chinaLocation)
	return start, end.AddDate(0, 0, 1), term.Name, nil
}

func (s *Service) attendanceCounts(teamID string, startsAt, endsAt time.Time) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT r.player_id, e.starts_at FROM attendance_records r JOIN events e ON e.id = r.event_id LEFT JOIN event_cancellations cancelled ON cancelled.event_id = e.id WHERE e.team_id = ? AND e.type IN ('training', 'friendly') AND r.status = 'available' AND cancelled.event_id IS NULL`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var playerID, started string
		if err := rows.Scan(&playerID, &started); err != nil {
			return nil, err
		}
		occurred, err := time.Parse(time.RFC3339, started)
		if err != nil {
			return nil, err
		}
		if !occurred.Before(startsAt) && occurred.Before(endsAt) {
			counts[playerID]++
		}
	}
	return counts, rows.Err()
}

func (s *Service) term(teamID, id string) (Term, error) {
	var term Term
	err := s.db.QueryRow(`SELECT id, name, starts_on, ends_on, created_at, updated_at FROM terms WHERE id = ? AND team_id = ?`, id, teamID).Scan(&term.ID, &term.Name, &term.StartsOn, &term.EndsOn, &term.CreatedAt, &term.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Term{}, ErrInvalidTerm
	}
	return term, err
}

func (s *Service) ensureNoOverlap(teamID, excludedID, startsOn, endsOn string) error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM terms WHERE team_id = ? AND id <> ? AND NOT (ends_on < ? OR starts_on > ?)`, teamID, excludedID, startsOn, endsOn).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrOverlappingTerm
	}
	return nil
}

func normalizeTerm(term Term) (Term, error) {
	term.Name = strings.TrimSpace(term.Name)
	term.StartsOn = strings.TrimSpace(term.StartsOn)
	term.EndsOn = strings.TrimSpace(term.EndsOn)
	if term.Name == "" {
		return Term{}, ErrInvalidTerm
	}
	startsAt, err := time.ParseInLocation("2006-01-02", term.StartsOn, chinaLocation)
	if err != nil {
		return Term{}, ErrInvalidTerm
	}
	endsAt, err := time.ParseInLocation("2006-01-02", term.EndsOn, chinaLocation)
	if err != nil || endsAt.Before(startsAt) {
		return Term{}, ErrInvalidTerm
	}
	return term, nil
}

func historyRange(fromValue, toValue string) (time.Time, time.Time, error) {
	if fromValue == "" && toValue == "" {
		return time.Time{}, time.Time{}, nil
	}
	var from, to time.Time
	var err error
	if fromValue != "" {
		from, err = time.ParseInLocation("2006-01-02", fromValue, chinaLocation)
		if err != nil {
			return time.Time{}, time.Time{}, ErrInvalidRange
		}
	}
	if toValue != "" {
		to, err = time.ParseInLocation("2006-01-02", toValue, chinaLocation)
		if err != nil {
			return time.Time{}, time.Time{}, ErrInvalidRange
		}
		to = to.AddDate(0, 0, 1)
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return time.Time{}, time.Time{}, ErrInvalidRange
	}
	return from, to, nil
}

func mustChinaLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(fmt.Sprintf("load Asia/Shanghai: %v", err))
	}
	return location
}

func nowISO() string { return time.Now().Format(time.RFC3339) }
