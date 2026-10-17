package contentsafety

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func TestChecksFailClosedAndReuseToken(t *testing.T) {
	t.Setenv("ANYPITCH_WECHAT_APP_ID", "appid")
	t.Setenv("ANYPITCH_WECHAT_APP_SECRET", "secret")
	c := New()
	tokenCalls := 0
	answer := `{"errcode":0,"result":{"suggest":"pass"}}`
	c.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		body := answer
		if r.URL.Path == "/cgi-bin/stable_token" {
			tokenCalls++
			body = `{"access_token":"test","expires_in":7200}`
		} else {
			var input map[string]any
			json.NewDecoder(r.Body).Decode(&input)
			if input["version"] != float64(2) || input["openid"] != "member" {
				t.Fatal("expected v2 member-scoped check")
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	ctx := context.Background()
	if err := c.Text(ctx, "member", "测试昵称", 1); err != nil {
		t.Fatal(err)
	}
	for _, response := range []string{`{"errcode":0,"result":{"suggest":"risky"}}`, `{"errcode":0,"result":{"suggest":"review"}}`, `{"errcode":87014}`} {
		answer = response
		if c.Text(ctx, "member", "测试", 1) != ErrRejected {
			t.Fatal("unsafe response must reject")
		}
	}
	for _, response := range []string{`{}`, `{"errcode":40003}`, `not-json`} {
		answer = response
		if c.Text(ctx, "member", "测试", 1) != ErrUnavailable {
			t.Fatal("missing or failed result must reject")
		}
	}
	answer = `{"errcode":0,"trace_id":"job"}`
	if id, e := c.Image(ctx, "member", "https://example.test/image", 4); e != nil || id != "job" {
		t.Fatal(id, e)
	}
	answer = `{"errcode":0}`
	if _, e := c.Image(ctx, "member", "https://example.test/image", 4); e != ErrUnavailable {
		t.Fatal("missing trace must reject")
	}
	if tokenCalls != 1 {
		t.Fatal("token should be cached")
	}
}
