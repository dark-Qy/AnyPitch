package notification

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Service struct {
	db         *sql.DB
	templateID string
	http       *http.Client
	mu         sync.Mutex
	token      string
	expires    time.Time
}

func New(db *sql.DB, templateID string) *Service {
	return &Service{db: db, templateID: strings.TrimSpace(templateID), http: &http.Client{Timeout: 10 * time.Second}}
}

func (s *Service) Enabled() bool      { return s.templateID != "" }
func (s *Service) TemplateID() string { return s.templateID }

func (s *Service) Remaining(playerID string) (int, error) {
	var remaining int
	err := s.db.QueryRow(`SELECT remaining FROM event_notice_grants WHERE player_id=?`, playerID).Scan(&remaining)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return remaining, err
}

func (s *Service) Grant(playerID, requestID string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().Format(time.RFC3339)
	result, err := tx.Exec(`INSERT OR IGNORE INTO event_notice_authorizations(player_id,request_id,created_at) VALUES(?,?,?)`, playerID, requestID, now)
	if err != nil {
		return 0, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if inserted > 0 {
		_, err = tx.Exec(`INSERT INTO event_notice_grants(player_id,remaining,updated_at) VALUES(?,1,?)
			ON CONFLICT(player_id) DO UPDATE SET remaining=remaining+1,updated_at=excluded.updated_at`, playerID, now)
		if err != nil {
			return 0, err
		}
	}
	var remaining int
	if err = tx.QueryRow(`SELECT remaining FROM event_notice_grants WHERE player_id=?`, playerID).Scan(&remaining); err != nil {
		return 0, err
	}
	return remaining, tx.Commit()
}

// Run drains committed event notices. An interrupted in-flight send is marked
// unknown instead of retried, because WeChat may already have accepted it.
func (s *Service) Run(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE event_notice_deliveries SET status='unknown' WHERE status='sending'`); err != nil {
		log.Printf("event notice recovery failed: %v", err)
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := s.Drain(ctx); err != nil && ctx.Err() == nil {
			log.Printf("event notice worker: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type delivery struct {
	eventID  string
	playerID string
	openID   string
	title    string
	startsAt string
	location string
	inactive bool
}

// Drain sends a bounded batch so new work and transient failures cannot monopolize the server.
func (s *Service) Drain(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	for i := 0; i < 30; i++ {
		job, err := s.next(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		result, err := s.db.ExecContext(ctx, `UPDATE event_notice_deliveries SET status='sending',updated_at=? WHERE event_id=? AND player_id=? AND status='pending'`, time.Now().Format(time.RFC3339), job.eventID, job.playerID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			continue
		}
		if job.inactive {
			if err := s.finish(ctx, job, "skipped", 0); err != nil {
				return err
			}
			continue
		}
		status, code := s.send(ctx, job)
		if status == "pending" {
			if err := s.finish(ctx, job, status, code); err != nil {
				return err
			}
			return nil // Retry after the next tick; no message was sent.
		}
		if err := s.finish(ctx, job, status, code); err != nil {
			return err
		}
		if status != "sent" {
			log.Printf("event notice delivery %s: WeChat code %d", status, code)
		}
	}
	return nil
}

func (s *Service) next(ctx context.Context) (delivery, error) {
	var job delivery
	err := s.db.QueryRowContext(ctx, `SELECT d.event_id,d.player_id,m.open_id,e.title,e.starts_at,e.location,
		(p.status<>'active' OR c.event_id IS NOT NULL)
		FROM event_notice_deliveries d
		JOIN events e ON e.id=d.event_id
		JOIN players p ON p.id=d.player_id
		JOIN wechat_members m ON m.player_id=p.id
		LEFT JOIN event_cancellations c ON c.event_id=e.id
		WHERE d.status='pending' ORDER BY d.created_at,d.event_id,d.player_id LIMIT 1`).Scan(
		&job.eventID, &job.playerID, &job.openID, &job.title, &job.startsAt, &job.location, &job.inactive)
	return job, err
}

func (s *Service) finish(ctx context.Context, job delivery, status string, code int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE event_notice_deliveries SET status=?,error_code=?,updated_at=? WHERE event_id=? AND player_id=? AND status='sending'`, status, code, time.Now().Format(time.RFC3339), job.eventID, job.playerID)
	return err
}

func (s *Service) send(ctx context.Context, job delivery) (status string, code int) {
	token, err := s.accessToken(ctx)
	if err != nil {
		return "pending", 0
	}
	for attempt := 0; attempt < 2; attempt++ {
		code, err = s.sendWithToken(ctx, token, job)
		if err != nil {
			return "unknown", 0
		}
		if code == 0 {
			return "sent", 0
		}
		if (code != 40001 && code != 42001) || attempt == 1 {
			return "failed", code
		}
		s.mu.Lock()
		s.token = ""
		s.mu.Unlock()
		token, err = s.accessToken(ctx)
		if err != nil {
			return "pending", code
		}
	}
	return "failed", code
}

func (s *Service) sendWithToken(ctx context.Context, token string, job delivery) (int, error) {
	start, err := time.Parse(time.RFC3339, job.startsAt)
	if err != nil {
		return -1, err
	}
	beijing, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return -1, err
	}
	location := strings.TrimSpace(job.location)
	if location == "" {
		location = "地点待定"
	}
	payload := map[string]any{
		"touser":      job.openID,
		"template_id": s.templateID,
		"page":        "pages/event-detail/index?id=" + url.QueryEscape(job.eventID),
		"data": map[string]any{
			"thing6":  map[string]string{"value": firstRunes(job.title, 20)},
			"date2":   map[string]string{"value": start.In(beijing).Format("2006年01月02日 15:04")},
			"thing4":  map[string]string{"value": firstRunes(location, 20)},
			"thing11": map[string]string{"value": "点击查看活动并报名"},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return -1, err
	}
	endpoint := "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return -1, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	var result struct {
		Code int `json:"errcode"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result) != nil {
		return -1, fmt.Errorf("invalid WeChat send response")
	}
	return result.Code, nil
}

func (s *Service) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.expires) {
		return s.token, nil
	}
	appID, secret := os.Getenv("ANYPITCH_WECHAT_APP_ID"), os.Getenv("ANYPITCH_WECHAT_APP_SECRET")
	if appID == "" || secret == "" {
		return "", errors.New("WeChat credentials unavailable")
	}
	data, err := json.Marshal(map[string]any{"grant_type": "client_credential", "appid": appID, "secret": secret, "force_refresh": false})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.weixin.qq.com/cgi-bin/stable_token", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		Token   string `json:"access_token"`
		Expires int    `json:"expires_in"`
		Code    int    `json:"errcode"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result) != nil || result.Code != 0 || result.Token == "" || result.Expires <= 120 {
		return "", errors.New("WeChat token unavailable")
	}
	s.token = result.Token
	s.expires = time.Now().Add(time.Duration(result.Expires-120) * time.Second)
	return s.token, nil
}

func firstRunes(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}
