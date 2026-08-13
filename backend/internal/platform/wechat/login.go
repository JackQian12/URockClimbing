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
	"sync"
	"time"
)

const code2SessionEndpoint = "https://api.weixin.qq.com/sns/jscode2session"
const stableTokenEndpoint = "https://api.weixin.qq.com/cgi-bin/stable_token"
const phoneNumberEndpoint = "https://api.weixin.qq.com/wxa/business/getuserphonenumber"

type Session struct {
	OpenID  string
	UnionID string
}

type LoginExchanger interface {
	ExchangeCode(context.Context, string) (Session, error)
}

type PhoneNumber struct {
	PureNumber  string
	CountryCode string
}

type PhoneNumberExchanger interface {
	ExchangePhoneNumber(context.Context, string) (PhoneNumber, error)
}

type LoginClient struct {
	appID      string
	appSecret  string
	mock       bool
	httpClient *http.Client
	tokenMu    sync.Mutex
	token      string
	tokenUntil time.Time
}

func (c *LoginClient) ExchangePhoneNumber(ctx context.Context, code string) (PhoneNumber, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return PhoneNumber{}, errors.New("empty phone code")
	}
	if c.mock {
		return PhoneNumber{PureNumber: "13800000000", CountryCode: "86"}, nil
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return PhoneNumber{}, err
	}
	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return PhoneNumber{}, fmt.Errorf("encode phone request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, phoneNumberEndpoint+"?access_token="+url.QueryEscape(token), strings.NewReader(string(body)))
	if err != nil {
		return PhoneNumber{}, fmt.Errorf("build phone request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return PhoneNumber{}, fmt.Errorf("request phone number: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return PhoneNumber{}, fmt.Errorf("phone number returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		ErrCode   int `json:"errcode"`
		PhoneInfo struct {
			PureNumber  string `json:"purePhoneNumber"`
			CountryCode string `json:"countryCode"`
		} `json:"phone_info"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return PhoneNumber{}, fmt.Errorf("decode phone response: %w", err)
	}
	if payload.ErrCode != 0 || payload.PhoneInfo.PureNumber == "" {
		return PhoneNumber{}, fmt.Errorf("phone number rejected code: %d", payload.ErrCode)
	}
	return PhoneNumber(payload.PhoneInfo), nil
}

func (c *LoginClient) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenUntil) {
		return c.token, nil
	}
	if c.appID == "" || c.appSecret == "" {
		return "", errors.New("wechat phone number is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"grant_type":    "client_credential",
		"appid":         c.appID,
		"secret":        c.appSecret,
		"force_refresh": false,
	})
	if err != nil {
		return "", fmt.Errorf("encode stable token request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, stableTokenEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("build stable token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("request stable token: %w", err)
	}
	defer response.Body.Close()
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stable token returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode stable token response: %w", err)
	}
	if payload.ErrCode != 0 || payload.AccessToken == "" {
		return "", fmt.Errorf("stable token rejected request: %d", payload.ErrCode)
	}
	ttl := time.Duration(payload.ExpiresIn) * time.Second
	if ttl > 5*time.Minute {
		ttl -= 5 * time.Minute
	}
	c.token = payload.AccessToken
	c.tokenUntil = time.Now().Add(ttl)
	return c.token, nil
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
