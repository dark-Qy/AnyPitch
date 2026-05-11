package tactics

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidBoard = errors.New("invalid tactic board")

type Slot struct {
	SlotID   string  `json:"slot_id"`
	Label    string  `json:"label"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	PlayerID string  `json:"player_id"`
}

type Template struct {
	Format    int    `json:"format"`
	Name      string `json:"name"`
	Formation string `json:"formation"`
	Slots     []Slot `json:"slots"`
}

type Board struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Format    int    `json:"format"`
	Formation string `json:"formation"`
	Slots     []Slot `json:"slots"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type BoardInput struct {
	Name      string `json:"name"`
	Format    int    `json:"format"`
	Formation string `json:"formation"`
	Slots     []Slot `json:"slots"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func Templates() []Template {
	return []Template{
		{
			Format:    5,
			Name:      "五人制高位压迫",
			Formation: "1-2-1",
			Slots: []Slot{
				{SlotID: "gk", Label: "门将", X: 50, Y: 91},
				{SlotID: "ld", Label: "左后", X: 28, Y: 65},
				{SlotID: "rd", Label: "右后", X: 72, Y: 65},
				{SlotID: "am", Label: "前腰", X: 50, Y: 41},
				{SlotID: "fw", Label: "前锋", X: 50, Y: 18},
			},
		},
		{
			Format:    8,
			Name:      "八人制平衡推进",
			Formation: "2-3-2",
			Slots: []Slot{
				{SlotID: "gk", Label: "门将", X: 50, Y: 93},
				{SlotID: "lb", Label: "左后", X: 32, Y: 73},
				{SlotID: "rb", Label: "右后", X: 68, Y: 73},
				{SlotID: "lm", Label: "左前卫", X: 25, Y: 49},
				{SlotID: "cm", Label: "中场", X: 50, Y: 50},
				{SlotID: "rm", Label: "右前卫", X: 75, Y: 49},
				{SlotID: "lf", Label: "左锋", X: 38, Y: 22},
				{SlotID: "rf", Label: "右锋", X: 62, Y: 22},
			},
		},
		{
			Format:    11,
			Name:      "十一人制 4-3-3",
			Formation: "4-3-3",
			Slots: []Slot{
				{SlotID: "gk", Label: "门将", X: 50, Y: 95},
				{SlotID: "lb", Label: "左后卫", X: 18, Y: 76},
				{SlotID: "lcb", Label: "左中卫", X: 39, Y: 78},
				{SlotID: "rcb", Label: "右中卫", X: 61, Y: 78},
				{SlotID: "rb", Label: "右后卫", X: 82, Y: 76},
				{SlotID: "dm", Label: "后腰", X: 50, Y: 57},
				{SlotID: "lcm", Label: "左中场", X: 34, Y: 46},
				{SlotID: "rcm", Label: "右中场", X: 66, Y: 46},
				{SlotID: "lw", Label: "左边锋", X: 22, Y: 22},
				{SlotID: "st", Label: "中锋", X: 50, Y: 16},
				{SlotID: "rw", Label: "右边锋", X: 78, Y: 22},
			},
		},
	}
}

func (s *Service) List(teamID string) ([]Board, error) {
	rows, err := s.db.Query(
		`SELECT id, name, format, formation, slots_json, created_at, updated_at
		 FROM tactic_boards
		 WHERE team_id = ?
		 ORDER BY updated_at DESC`,
		teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	boards := []Board{}
	for rows.Next() {
		board, err := scanBoard(rows)
		if err != nil {
			return nil, err
		}
		boards = append(boards, board)
	}
	return boards, rows.Err()
}

func (s *Service) Create(teamID string, input BoardInput) (Board, error) {
	board, err := normalizeBoard(input)
	if err != nil {
		return Board{}, err
	}
	now := nowISO()
	board.ID = uuid.NewString()
	board.CreatedAt = now
	board.UpdatedAt = now
	slotsJSON, err := json.Marshal(board.Slots)
	if err != nil {
		return Board{}, err
	}
	_, err = s.db.Exec(
		`INSERT INTO tactic_boards (id, team_id, name, format, formation, slots_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		board.ID,
		teamID,
		board.Name,
		board.Format,
		board.Formation,
		string(slotsJSON),
		board.CreatedAt,
		board.UpdatedAt,
	)
	if err != nil {
		return Board{}, err
	}
	return board, nil
}

func (s *Service) Update(teamID, id string, input BoardInput) (Board, error) {
	board, err := normalizeBoard(input)
	if err != nil {
		return Board{}, err
	}
	board.ID = id
	board.UpdatedAt = nowISO()
	var createdAt string
	err = s.db.QueryRow(`SELECT created_at FROM tactic_boards WHERE id = ? AND team_id = ?`, id, teamID).Scan(&createdAt)
	if err != nil {
		return Board{}, err
	}
	board.CreatedAt = createdAt
	slotsJSON, err := json.Marshal(board.Slots)
	if err != nil {
		return Board{}, err
	}
	_, err = s.db.Exec(
		`UPDATE tactic_boards SET name = ?, format = ?, formation = ?, slots_json = ?, updated_at = ?
		 WHERE id = ? AND team_id = ?`,
		board.Name,
		board.Format,
		board.Formation,
		string(slotsJSON),
		board.UpdatedAt,
		id,
		teamID,
	)
	if err != nil {
		return Board{}, err
	}
	return board, nil
}

func (s *Service) Delete(teamID, id string) error {
	_, err := s.db.Exec(`DELETE FROM tactic_boards WHERE id = ? AND team_id = ?`, id, teamID)
	return err
}

type boardScanner interface {
	Scan(dest ...any) error
}

func scanBoard(scanner boardScanner) (Board, error) {
	var board Board
	var slotsJSON string
	err := scanner.Scan(&board.ID, &board.Name, &board.Format, &board.Formation, &slotsJSON, &board.CreatedAt, &board.UpdatedAt)
	if err != nil {
		return Board{}, err
	}
	if err := json.Unmarshal([]byte(slotsJSON), &board.Slots); err != nil {
		return Board{}, err
	}
	return board, nil
}

func normalizeBoard(input BoardInput) (Board, error) {
	board := Board{
		Name:      strings.TrimSpace(input.Name),
		Format:    input.Format,
		Formation: strings.TrimSpace(input.Formation),
		Slots:     input.Slots,
	}
	if board.Name == "" || board.Formation == "" || !validFormat(board.Format) {
		return Board{}, ErrInvalidBoard
	}
	for _, slot := range board.Slots {
		if strings.TrimSpace(slot.SlotID) == "" || slot.X < 0 || slot.X > 100 || slot.Y < 0 || slot.Y > 100 {
			return Board{}, ErrInvalidBoard
		}
	}
	return board, nil
}

func validFormat(format int) bool {
	return format == 5 || format == 8 || format == 11
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
