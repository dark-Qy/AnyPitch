package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrWeChatUnavailable = errors.New("微信登录暂不可用，请联系管理员完成配置")
var ErrInvalidProfile = errors.New("请选择昵称，长度不超过40个字")

// Only the server exchanges the single-use code. OpenID and session_key never
// cross the application API boundary; the client receives our session only.
func (s *Service) LoginWeChat(ctx context.Context, code string) (PlayerSession, error) {
	appID, secret := os.Getenv("ANYPITCH_WECHAT_APP_ID"), os.Getenv("ANYPITCH_WECHAT_APP_SECRET")
	if appID == "" || secret == "" {
		return PlayerSession{}, ErrWeChatUnavailable
	}
	if strings.TrimSpace(code) == "" || len(code) > 512 {
		return PlayerSession{}, ErrInvalidCredentials
	}
	params := url.Values{"appid": {appID}, "secret": {secret}, "js_code": {code}, "grant_type": {"authorization_code"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.weixin.qq.com/sns/jscode2session?"+params.Encode(), nil)
	if err != nil {
		return PlayerSession{}, ErrWeChatUnavailable
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return PlayerSession{}, ErrWeChatUnavailable
	}
	defer resp.Body.Close()
	var result struct {
		OpenID    string `json:"openid"`
		ErrorCode int    `json:"errcode"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result) != nil {
		return PlayerSession{}, ErrWeChatUnavailable
	}
	if result.ErrorCode != 0 || result.OpenID == "" {
		return PlayerSession{}, ErrInvalidCredentials
	}
	return s.sessionForWeChat(appID, result.OpenID)
}

func (s *Service) sessionForWeChat(appID, openID string) (PlayerSession, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerSession{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRow(`SELECT player_id FROM wechat_members WHERE app_id=? AND open_id=?`, appID, openID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var teamID string
		if err = tx.QueryRow(`SELECT t.id FROM teams t JOIN users u ON u.id=t.user_id WHERE u.email=? ORDER BY t.created_at LIMIT 1`, DefaultCoachEmail).Scan(&teamID); err != nil {
			return PlayerSession{}, err
		}
		id = uuid.NewString()
		now := nowISO()
		if _, err = tx.Exec(`INSERT INTO players(id,team_id,name,number,positions_json,status,created_at,updated_at) VALUES(?,?,?,NULL,'[]','active',?,?)`, id, teamID, "球友"+id[:6], now, now); err != nil {
			return PlayerSession{}, err
		}
		if _, err = tx.Exec(`INSERT INTO wechat_members(player_id,app_id,open_id) VALUES(?,?,?)`, id, appID, openID); err != nil {
			return PlayerSession{}, err
		}
	} else if err != nil {
		return PlayerSession{}, err
	}
	player, err := scanPlayerIdentity(tx.QueryRow(`SELECT p.id,p.team_id,p.name,p.number,p.positions_json,p.status,p.created_at,p.updated_at,m.role,m.profile_complete,m.avatar_file_name <> '' FROM players p JOIN wechat_members m ON m.player_id=p.id WHERE p.id=?`, id))
	if err != nil {
		return PlayerSession{}, err
	}
	if player.Status != "active" {
		return PlayerSession{}, ErrUnauthorized
	}
	if err = tx.Commit(); err != nil {
		return PlayerSession{}, err
	}
	return s.createPlayerSession(player)
}

func (s *Service) UpdateProfile(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 40 || strings.ContainsAny(name, "\r\n\x00") {
		return ErrInvalidProfile
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE players SET name=?,updated_at=? WHERE id=? AND id IN (SELECT player_id FROM wechat_members)`, name, nowISO(), id); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE wechat_members SET profile_complete=1 WHERE player_id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) CoachByToken(token string) (User, error) {
	// The web password session remains a separate management entry.
	user, err := s.UserByToken(token)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrUnauthorized) {
		return User{}, err
	}
	member, err := s.PlayerByToken(token)
	if err != nil || (member.Role != "coach" && member.Role != "owner") {
		return User{}, ErrUnauthorized
	}
	err = s.db.QueryRow(`SELECT u.id,u.email,u.status,u.created_at FROM users u JOIN teams t ON t.user_id=u.id WHERE t.id=?`, member.TeamID).Scan(&user.ID, &user.Email, &user.Status, &user.CreatedAt)
	return user, err
}
