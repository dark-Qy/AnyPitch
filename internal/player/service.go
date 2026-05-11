package player

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidPlayer = errors.New("invalid player")

type Player struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Number    *int     `json:"number"`
	Positions []string `json:"positions"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

type CreateInput struct {
	Name      string   `json:"name"`
	Number    *int     `json:"number"`
	Positions []string `json:"positions"`
}

type UpdateInput struct {
	Name      *string  `json:"name"`
	Number    *int     `json:"number"`
	Positions []string `json:"positions"`
	Status    *string  `json:"status"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(teamID string) ([]Player, error) {
	rows, err := s.db.Query(
		`SELECT id, name, number, positions_json, status, created_at, updated_at
		 FROM players
		 WHERE team_id = ?
		 ORDER BY COALESCE(number, 999), name`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	players := []Player{}
	for rows.Next() {
		player, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, player)
	}
	return players, rows.Err()
}

func (s *Service) Create(teamID string, input CreateInput) (Player, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Player{}, ErrInvalidPlayer
	}
	positions := cleanPositions(input.Positions)
	positionsJSON, err := json.Marshal(positions)
	if err != nil {
		return Player{}, err
	}
	now := nowISO()
	player := Player{
		ID:        uuid.NewString(),
		Name:      name,
		Number:    input.Number,
		Positions: positions,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = s.db.Exec(
		`INSERT INTO players (id, team_id, name, number, positions_json, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		player.ID,
		teamID,
		player.Name,
		player.Number,
		string(positionsJSON),
		player.Status,
		player.CreatedAt,
		player.UpdatedAt,
	)
	if err != nil {
		return Player{}, err
	}
	return player, nil
}

func (s *Service) Update(teamID, id string, input UpdateInput) (Player, error) {
	current, err := s.Get(teamID, id)
	if err != nil {
		return Player{}, err
	}
	if input.Name != nil {
		current.Name = strings.TrimSpace(*input.Name)
	}
	if input.Number != nil {
		current.Number = input.Number
	}
	if input.Positions != nil {
		current.Positions = cleanPositions(input.Positions)
	}
	if input.Status != nil {
		current.Status = strings.TrimSpace(*input.Status)
	}
	if current.Name == "" || (current.Status != "active" && current.Status != "inactive") {
		return Player{}, ErrInvalidPlayer
	}
	positionsJSON, err := json.Marshal(current.Positions)
	if err != nil {
		return Player{}, err
	}
	current.UpdatedAt = nowISO()
	_, err = s.db.Exec(
		`UPDATE players SET name = ?, number = ?, positions_json = ?, status = ?, updated_at = ?
		 WHERE id = ? AND team_id = ?`,
		current.Name,
		current.Number,
		string(positionsJSON),
		current.Status,
		current.UpdatedAt,
		id,
		teamID,
	)
	if err != nil {
		return Player{}, err
	}
	return current, nil
}

func (s *Service) Delete(teamID, id string) error {
	_, err := s.db.Exec(`DELETE FROM players WHERE id = ? AND team_id = ?`, id, teamID)
	return err
}

func (s *Service) Get(teamID, id string) (Player, error) {
	row := s.db.QueryRow(
		`SELECT id, name, number, positions_json, status, created_at, updated_at
		 FROM players
		 WHERE id = ? AND team_id = ?`,
		id,
		teamID,
	)
	return scanPlayer(row)
}

type playerScanner interface {
	Scan(dest ...any) error
}

func scanPlayer(scanner playerScanner) (Player, error) {
	var player Player
	var number sql.NullInt64
	var positionsJSON string
	err := scanner.Scan(&player.ID, &player.Name, &number, &positionsJSON, &player.Status, &player.CreatedAt, &player.UpdatedAt)
	if err != nil {
		return Player{}, err
	}
	if number.Valid {
		value := int(number.Int64)
		player.Number = &value
	}
	if err := json.Unmarshal([]byte(positionsJSON), &player.Positions); err != nil {
		return Player{}, err
	}
	return player, nil
}

func cleanPositions(raw []string) []string {
	cleaned := []string{}
	seen := map[string]bool{}
	for _, value := range raw {
		position := strings.TrimSpace(value)
		if position == "" || seen[position] {
			continue
		}
		seen[position] = true
		cleaned = append(cleaned, position)
	}
	return cleaned
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
