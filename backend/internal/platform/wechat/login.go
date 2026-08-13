// Package wechat contains adapters for WeChat login, payments, and refunds.
package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const code2SessionEndpoint = "https://api.weixin.qq.com/sns/jscode2session"

type Session struct {
	OpenID  string
	UnionID string
}

type LoginExchanger interface {
	ExchangeCode(context.Context, string) (Session, error)
}

type LoginClient struct {
	appID      string
	appSecret  string
	mock       bool
	httpClient *http.Client
}

func NewLoginClient(appID, appSecret string, mock bool) *LoginClient {
	return &LoginClient{
		appID: appID, appSecret: appSecret, mock: mock,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *LoginClient) ExchangeCode(ctx context.Context, code string) (Session, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return Session{}, errors.New("empty login code")
	}
	if c.mock {
		return Session{OpenID: "mock-" + code}, nil
	}
	if c.appID == "" || c.appSecret == "" {
		return Session{}, errors.New("wechat login is not configured")
	}

	query := url.Values{
		"appid":      {c.appID},
		"secret":     {c.appSecret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, code2SessionEndpoint+"?"+query.Encode(), nil)
	if err != nil {
		return Session{}, fmt.Errorf("build code2session request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Session{}, fmt.Errorf("request code2session: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Session{}, fmt.Errorf("code2session returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Session{}, fmt.Errorf("decode code2session response: %w", err)
	}
	if payload.ErrCode != 0 || payload.OpenID == "" {
		return Session{}, fmt.Errorf("code2session rejected code: %d", payload.ErrCode)
	}
	return Session{OpenID: payload.OpenID, UnionID: payload.UnionID}, nil
}
