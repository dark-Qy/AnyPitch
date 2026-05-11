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
	Side     string  `json:"side,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	PlayerID string  `json:"player_id"`
}

type Template struct {
	ID        string `json:"id"`
	Format    int    `json:"format"`
	Name      string `json:"name"`
	Formation string `json:"formation"`
	Slots     []Slot `json:"slots"`
}

type Board struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	TemplateID         string `json:"template_id,omitempty"`
	OpponentTemplateID string `json:"opponent_template_id,omitempty"`
	Format             int    `json:"format"`
	Formation          string `json:"formation"`
	OpponentFormation  string `json:"opponent_formation,omitempty"`
	Slots              []Slot `json:"slots"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type BoardInput struct {
	Name               string `json:"name"`
	TemplateID         string `json:"template_id"`
	OpponentTemplateID string `json:"opponent_template_id"`
	Format             int    `json:"format"`
	Formation          string `json:"formation"`
	OpponentFormation  string `json:"opponent_formation"`
	Slots              []Slot `json:"slots"`
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
			ID:        "f5-121-press",
			Format:    5,
			Name:      "五人制高位压迫",
			Formation: "1-2-1",
			Slots: []Slot{
				homeSlot("gk", "门将", 50, 91),
				homeSlot("ld", "左后", 28, 65),
				homeSlot("rd", "右后", 72, 65),
				homeSlot("am", "前腰", 50, 41),
				homeSlot("fw", "前锋", 50, 18),
			},
		},
		{
			ID:        "f5-211-counter",
			Format:    5,
			Name:      "五人制稳守反击",
			Formation: "2-1-1",
			Slots: []Slot{
				homeSlot("gk", "门将", 50, 91),
				homeSlot("lb", "左后", 35, 68),
				homeSlot("rb", "右后", 65, 68),
				homeSlot("cm", "中场", 50, 45),
				homeSlot("fw", "前锋", 50, 20),
			},
		},
		{
			ID:        "f8-232-balance",
			Format:    8,
			Name:      "八人制平衡推进",
			Formation: "2-3-2",
			Slots: []Slot{
				homeSlot("gk", "门将", 50, 93),
				homeSlot("lb", "左后", 32, 73),
				homeSlot("rb", "右后", 68, 73),
				homeSlot("lm", "左前卫", 25, 49),
				homeSlot("cm", "中场", 50, 50),
				homeSlot("rm", "右前卫", 75, 49),
				homeSlot("lf", "左锋", 38, 22),
				homeSlot("rf", "右锋", 62, 22),
			},
		},
		{
			ID:        "f8-322-press",
			Format:    8,
			Name:      "八人制三后卫压迫",
			Formation: "3-2-2",
			Slots: []Slot{
				homeSlot("gk", "门将", 50, 93),
				homeSlot("lcb", "左中卫", 30, 74),
				homeSlot("cb", "中卫", 50, 76),
				homeSlot("rcb", "右中卫", 70, 74),
				homeSlot("lm", "左前卫", 35, 51),
				homeSlot("rm", "右前卫", 65, 51),
				homeSlot("lf", "左锋", 40, 24),
				homeSlot("rf", "右锋", 60, 24),
			},
		},
		{
			ID:        "f11-433-wide",
			Format:    11,
			Name:      "十一人制 4-3-3",
			Formation: "4-3-3",
			Slots: []Slot{
				homeSlot("gk", "门将", 50, 95),
				homeSlot("lb", "左后卫", 18, 76),
				homeSlot("lcb", "左中卫", 39, 78),
				homeSlot("rcb", "右中卫", 61, 78),
				homeSlot("rb", "右后卫", 82, 76),
				homeSlot("dm", "后腰", 50, 57),
				homeSlot("lcm", "左中场", 34, 46),
				homeSlot("rcm", "右中场", 66, 46),
				homeSlot("lw", "左边锋", 22, 22),
				homeSlot("st", "中锋", 50, 16),
				homeSlot("rw", "右边锋", 78, 22),
			},
		},
		{
			ID:        "f11-442-flat",
			Format:    11,
			Name:      "十一人制 4-4-2",
			Formation: "4-4-2",
			Slots: []Slot{
				homeSlot("gk", "门将", 50, 95),
				homeSlot("lb", "左后卫", 18, 76),
				homeSlot("lcb", "左中卫", 39, 78),
				homeSlot("rcb", "右中卫", 61, 78),
				homeSlot("rb", "右后卫", 82, 76),
				homeSlot("lm", "左前卫", 20, 49),
				homeSlot("lcm", "左中场", 42, 50),
				homeSlot("rcm", "右中场", 58, 50),
				homeSlot("rm", "右前卫", 80, 49),
				homeSlot("ls", "左前锋", 42, 19),
				homeSlot("rs", "右前锋", 58, 19),
			},
		},
		{
			ID:        "f11-352-wingback",
			Format:    11,
			Name:      "十一人制 3-5-2",
			Formation: "3-5-2",
			Slots: []Slot{
				homeSlot("gk", "门将", 50, 95),
				homeSlot("lcb", "左中卫", 32, 78),
				homeSlot("cb", "中卫", 50, 80),
				homeSlot("rcb", "右中卫", 68, 78),
				homeSlot("lwb", "左翼卫", 16, 55),
				homeSlot("dm", "后腰", 50, 58),
				homeSlot("rwb", "右翼卫", 84, 55),
				homeSlot("lcm", "左中场", 38, 43),
				homeSlot("rcm", "右中场", 62, 43),
				homeSlot("ls", "左前锋", 42, 18),
				homeSlot("rs", "右前锋", 58, 18),
			},
		},
	}
}

func homeSlot(slotID, label string, x, y float64) Slot {
	return Slot{SlotID: slotID, Label: label, Side: "home", X: x, Y: y}
}

func (s *Service) List(teamID string) ([]Board, error) {
	rows, err := s.db.Query(
		`SELECT id, name, template_id, opponent_template_id, format, formation, opponent_formation, slots_json, created_at, updated_at
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
		`INSERT INTO tactic_boards (id, team_id, name, template_id, opponent_template_id, format, formation, opponent_formation, slots_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		board.ID,
		teamID,
		board.Name,
		board.TemplateID,
		board.OpponentTemplateID,
		board.Format,
		board.Formation,
		board.OpponentFormation,
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
		`UPDATE tactic_boards SET name = ?, template_id = ?, opponent_template_id = ?, format = ?, formation = ?, opponent_formation = ?, slots_json = ?, updated_at = ?
		 WHERE id = ? AND team_id = ?`,
		board.Name,
		board.TemplateID,
		board.OpponentTemplateID,
		board.Format,
		board.Formation,
		board.OpponentFormation,
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
	err := scanner.Scan(
		&board.ID,
		&board.Name,
		&board.TemplateID,
		&board.OpponentTemplateID,
		&board.Format,
		&board.Formation,
		&board.OpponentFormation,
		&slotsJSON,
		&board.CreatedAt,
		&board.UpdatedAt,
	)
	if err != nil {
		return Board{}, err
	}
	if err := json.Unmarshal([]byte(slotsJSON), &board.Slots); err != nil {
		return Board{}, err
	}
	board.Slots = normalizeStoredSlots(board.Slots)
	return board, nil
}

func normalizeBoard(input BoardInput) (Board, error) {
	board := Board{
		Name:               strings.TrimSpace(input.Name),
		TemplateID:         strings.TrimSpace(input.TemplateID),
		OpponentTemplateID: strings.TrimSpace(input.OpponentTemplateID),
		Format:             input.Format,
		Formation:          strings.TrimSpace(input.Formation),
		OpponentFormation:  strings.TrimSpace(input.OpponentFormation),
		Slots:              input.Slots,
	}
	if board.Name == "" || board.Formation == "" || !validFormat(board.Format) {
		return Board{}, ErrInvalidBoard
	}
	for index, slot := range board.Slots {
		normalized, err := normalizeSlot(slot)
		if err != nil {
			return Board{}, ErrInvalidBoard
		}
		board.Slots[index] = normalized
	}
	return board, nil
}

func normalizeStoredSlots(slots []Slot) []Slot {
	normalized := make([]Slot, 0, len(slots))
	for _, slot := range slots {
		next, err := normalizeSlot(slot)
		if err != nil {
			normalized = append(normalized, slot)
			continue
		}
		normalized = append(normalized, next)
	}
	return normalized
}

func normalizeSlot(slot Slot) (Slot, error) {
	slot.SlotID = strings.TrimSpace(slot.SlotID)
	slot.Label = strings.TrimSpace(slot.Label)
	slot.Side = strings.TrimSpace(slot.Side)
	if slot.Side == "" {
		slot.Side = "home"
	}
	if slot.SlotID == "" || slot.Label == "" || (slot.Side != "home" && slot.Side != "opponent") {
		return Slot{}, ErrInvalidBoard
	}
	if slot.X < 0 || slot.X > 100 || slot.Y < 0 || slot.Y > 100 {
		return Slot{}, ErrInvalidBoard
	}
	return slot, nil
}

func validFormat(format int) bool {
	return format == 5 || format == 8 || format == 11
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
