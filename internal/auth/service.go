package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrEmailExists        = errors.New("email already exists")
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

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Register(email, password string) (Session, error) {
	email = normalizeEmail(email)
	if email == "" || len(password) < 8 {
		return Session{}, ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, err
	}

	now := nowISO()
	user := User{
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
			return Session{}, ErrEmailExists
		}
		return Session{}, err
	}
	return s.createSession(user)
}

func (s *Service) Login(email, password string) (Session, error) {
	email = normalizeEmail(email)
	var user User
	var passwordHash string
	err := s.db.QueryRow(
		`SELECT id, email, password_hash, status, created_at FROM users WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Email, &passwordHash, &user.Status, &user.CreatedAt)
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

func (s *Service) Logout(token string) error {
	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE token = ?`, strings.TrimSpace(token))
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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
