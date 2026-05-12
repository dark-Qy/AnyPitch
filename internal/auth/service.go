package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
)

const (
	DefaultCoachEmail    = "coach@anypitch.local"
	DefaultCoachPassword = "AnyPitch@2026"
	CoachPasswordEnv     = "ANYPITCH_COACH_PASSWORD"
)

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type Session struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type PlayerIdentity struct {
	ID        string   `json:"id"`
	TeamID    string   `json:"team_id,omitempty"`
	Name      string   `json:"name"`
	Number    *int     `json:"number"`
	Positions []string `json:"positions"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

type PlayerSession struct {
	Token  string         `json:"token"`
	Player PlayerIdentity `json:"player"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) EnsureDefaultCoach() (User, error) {
	email := normalizeEmail(DefaultCoachEmail)
	password := defaultCoachPassword()
	user, passwordHash, err := s.userWithPasswordByEmail(email)
	if err == nil {
		if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return User{}, err
			}
			if _, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), user.ID); err != nil {
				return User{}, err
			}
		}
		return user, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	now := nowISO()
	user = User{
		ID:        uuid.NewString(),
		Email:     email,
		Status:    "active",
		CreatedAt: now,
	}
	_, err = s.db.Exec(
		`INSERT INTO users (id, email, password_hash, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		user.ID,
		user.Email,
		string(hash),
		user.Status,
		user.CreatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return s.userByEmail(email)
		}
		return User{}, err
	}
	return user, nil
}

func (s *Service) Login(email, password string) (Session, error) {
	email = normalizeEmail(email)
	user, passwordHash, err := s.userWithPasswordByEmail(email)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return Session{}, ErrInvalidCredentials
	}
	return s.createSession(user)
}

func (s *Service) LoginPlayerByName(name string) (PlayerSession, error) {
	playerName := strings.TrimSpace(name)
	if playerName == "" {
		return PlayerSession{}, ErrInvalidCredentials
	}
	player, err := s.playerByName(playerName)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerSession{}, ErrInvalidCredentials
	}
	if err != nil {
		return PlayerSession{}, err
	}
	return s.createPlayerSession(player)
}

func (s *Service) userByEmail(email string) (User, error) {
	var user User
	err := s.db.QueryRow(
		`SELECT id, email, status, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Email, &user.Status, &user.CreatedAt)
	return user, err
}

func (s *Service) userWithPasswordByEmail(email string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.db.QueryRow(
		`SELECT id, email, password_hash, status, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Email, &passwordHash, &user.Status, &user.CreatedAt)
	return user, passwordHash, err
}

func (s *Service) UserByToken(token string) (User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return User{}, ErrUnauthorized
	}

	var user User
	err := s.db.QueryRow(
		`SELECT u.id, u.email, u.status, u.created_at
		 FROM auth_sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token = ? AND s.expires_at > ?`,
		token,
		nowISO(),
	).Scan(&user.ID, &user.Email, &user.Status, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) PlayerByToken(token string) (PlayerIdentity, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return PlayerIdentity{}, ErrUnauthorized
	}

	player, err := scanPlayerIdentity(s.db.QueryRow(
		`SELECT p.id, p.team_id, p.name, p.number, p.positions_json, p.status, p.created_at, p.updated_at
		 FROM player_sessions s
		 JOIN players p ON p.id = s.player_id AND p.team_id = s.team_id
		 WHERE s.token = ? AND s.expires_at > ? AND p.status = 'active'`,
		token,
		nowISO(),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerIdentity{}, ErrUnauthorized
	}
	if err != nil {
		return PlayerIdentity{}, err
	}
	return player, nil
}

func (s *Service) Logout(token string) error {
	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE token = ?`, strings.TrimSpace(token))
	return err
}

func (s *Service) LogoutPlayer(token string) error {
	_, err := s.db.Exec(`DELETE FROM player_sessions WHERE token = ?`, strings.TrimSpace(token))
	return err
}

func (s *Service) createSession(user User) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	createdAt := nowISO()
	expiresAt := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO auth_sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		token,
		user.ID,
		createdAt,
		expiresAt,
	)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, User: user}, nil
}

func (s *Service) createPlayerSession(player PlayerIdentity) (PlayerSession, error) {
	token, err := randomToken()
	if err != nil {
		return PlayerSession{}, err
	}
	createdAt := nowISO()
	expiresAt := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	_, err = s.db.Exec(
		`INSERT INTO player_sessions (token, team_id, player_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		token,
		player.TeamID,
		player.ID,
		createdAt,
		expiresAt,
	)
	if err != nil {
		return PlayerSession{}, err
	}
	return PlayerSession{Token: token, Player: player}, nil
}

func (s *Service) playerByName(name string) (PlayerIdentity, error) {
	return scanPlayerIdentity(s.db.QueryRow(
		`SELECT p.id, p.team_id, p.name, p.number, p.positions_json, p.status, p.created_at, p.updated_at
		 FROM players p
		 JOIN teams t ON t.id = p.team_id
		 WHERE p.name = ? AND p.status = 'active'
		 ORDER BY t.created_at, p.created_at
		 LIMIT 1`,
		name,
	))
}

type playerIdentityScanner interface {
	Scan(dest ...any) error
}

func scanPlayerIdentity(scanner playerIdentityScanner) (PlayerIdentity, error) {
	var player PlayerIdentity
	var number sql.NullInt64
	var positionsJSON string
	err := scanner.Scan(
		&player.ID,
		&player.TeamID,
		&player.Name,
		&number,
		&positionsJSON,
		&player.Status,
		&player.CreatedAt,
		&player.UpdatedAt,
	)
	if err != nil {
		return PlayerIdentity{}, err
	}
	if number.Valid {
		value := int(number.Int64)
		player.Number = &value
	}
	if err := json.Unmarshal([]byte(positionsJSON), &player.Positions); err != nil {
		return PlayerIdentity{}, err
	}
	return player, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func defaultCoachPassword() string {
	password := strings.TrimSpace(os.Getenv(CoachPasswordEnv))
	if password == "" {
		return DefaultCoachPassword
	}
	return password
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func nowISO() string {
	return time.Now().Format(time.RFC3339)
}
