package contentsafety

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var ErrRejected = errors.New("所发布内容含违规信息")
var ErrUnavailable = errors.New("内容检测暂不可用，请稍后重试")

type Client struct {
	mu      sync.Mutex
	token   string
	expires time.Time
	HTTP    *http.Client
}

func New() *Client { return &Client{HTTP: &http.Client{Timeout: 10 * time.Second}} }
func (c *Client) call(ctx context.Context, endpoint string, payload any, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result) != nil {
		return ErrUnavailable
	}
	return nil
}
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}
	appID, secret := os.Getenv("ANYPITCH_WECHAT_APP_ID"), os.Getenv("ANYPITCH_WECHAT_APP_SECRET")
	if appID == "" || secret == "" {
		return "", ErrUnavailable
	}
	var out struct {
		Token   string `json:"access_token"`
		Expires int    `json:"expires_in"`
		Code    int    `json:"errcode"`
	}
	err := c.call(ctx, "https://api.weixin.qq.com/cgi-bin/stable_token", map[string]any{"grant_type": "client_credential", "appid": appID, "secret": secret, "force_refresh": false}, &out)
	if err != nil || out.Code != 0 || out.Token == "" || out.Expires <= 120 {
		return "", ErrUnavailable
	}
	c.token = out.Token
	c.expires = time.Now().Add(time.Duration(out.Expires-120) * time.Second)
	return c.token, nil
}
func (c *Client) endpoint(ctx context.Context, path string) (string, error) {
	t, e := c.accessToken(ctx)
	return "https://api.weixin.qq.com" + path + "?access_token=" + url.QueryEscape(t), e
}
func (c *Client) Text(ctx context.Context, openid, content string, scene int) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	endpoint, err := c.endpoint(ctx, "/wxa/msg_sec_check")
	if err != nil {
		return err
	}
	var out struct {
		Code   int `json:"errcode"`
		Result struct {
			Suggest string `json:"suggest"`
		} `json:"result"`
	}
	err = c.call(ctx, endpoint, map[string]any{"openid": openid, "content": content, "version": 2, "scene": scene}, &out)
	if err != nil {
		return err
	}
	if out.Code == 87014 {
		return ErrRejected
	}
	if out.Code != 0 {
		return ErrUnavailable
	}
	if out.Result.Suggest == "risky" || out.Result.Suggest == "review" {
		return ErrRejected
	}
	if out.Result.Suggest != "pass" {
		return ErrUnavailable
	}
	return nil
}
func (c *Client) Image(ctx context.Context, openid, mediaURL string, scene int) (string, error) {
	endpoint, err := c.endpoint(ctx, "/wxa/media_check_async")
	if err != nil {
		return "", err
	}
	var out struct {
		Code  int    `json:"errcode"`
		Trace string `json:"trace_id"`
	}
	err = c.call(ctx, endpoint, map[string]any{"openid": openid, "media_url": mediaURL, "media_type": 2, "version": 2, "scene": scene}, &out)
	if err != nil || out.Code != 0 || out.Trace == "" {
		return "", ErrUnavailable
	}
	return out.Trace, nil
}
