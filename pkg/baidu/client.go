package baidu

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const maxBaiduResponseLen = 5 * 1024 * 1024

type Client struct {
	AppID       string
	AppKey      string
	SecretKey   string
	RedirectURI string
	HTTPClient  *http.Client
}

func NewClient(appID, appKey, secretKey, redirectURI string) *Client {
	return &Client{
		AppID:       appID,
		AppKey:      appKey,
		SecretKey:   secretKey,
		RedirectURI: redirectURI,
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) GetAuthURL(state string, isMobile bool) string {
	u, _ := url.Parse("https://openapi.baidu.com/oauth/2.0/authorize")
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.AppKey)
	q.Set("redirect_uri", c.RedirectURI)
	q.Set("scope", "basic")
	if c.AppID != "" {
		q.Set("device_id", c.AppID)
	}
	if state != "" {
		q.Set("state", state)
	}
	if isMobile {
		q.Set("display", "mobile")
	} else {
		q.Set("display", "page")
		q.Set("qrcode", "1")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

type OauthTokenResponse struct {
	AccessToken   string `json:"access_token"`
	ExpiresIn     int    `json:"expires_in"`
	RefreshToken  string `json:"refresh_token"`
	Scope         string `json:"scope"`
	SessionKey    string `json:"session_key"`
	SessionSecret string `json:"session_secret"`
	Error         string `json:"error"`
	ErrorDesc     string `json:"error_description"`
}

func (c *Client) GetOauthToken(code string) (*OauthTokenResponse, error) {
	u, _ := url.Parse("https://openapi.baidu.com/oauth/2.0/token")
	q := u.Query()
	q.Set("grant_type", "authorization_code")
	q.Set("code", code)
	q.Set("client_id", c.AppKey)
	q.Set("client_secret", c.SecretKey)
	q.Set("redirect_uri", c.RedirectURI)
	u.RawQuery = q.Encode()

	slog.Info("Baidu token request", "url", "https://openapi.baidu.com/oauth/2.0/token")
	resp, err := c.httpClient().Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("baidu token http status %d", resp.StatusCode)
	}

	var out OauthTokenResponse
	if err := decodeLimitedJSON(resp.Body, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return nil, fmt.Errorf("baidu api error: %s - %s", out.Error, out.ErrorDesc)
	}
	return &out, nil
}

type UserInfoResponse struct {
	UserID   string `json:"userid"`
	OpenID   string `json:"openid"`
	Username string `json:"username"`
	Portrait string `json:"portrait"`
	ErrorMsg string `json:"error_msg"`
	Error    string `json:"error"`
}

func (c *Client) GetUserInfo(accessToken string) (*UserInfoResponse, error) {
	u, _ := url.Parse("https://openapi.baidu.com/rest/2.0/passport/users/getInfo")
	q := u.Query()
	q.Set("access_token", accessToken)
	u.RawQuery = q.Encode()

	slog.Info("Baidu user info request", "url", "https://openapi.baidu.com/rest/2.0/passport/users/getInfo")
	resp, err := c.httpClient().Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("baidu user info http status %d", resp.StatusCode)
	}

	var out UserInfoResponse
	if err := decodeLimitedJSON(resp.Body, &out); err != nil {
		return nil, err
	}
	if out.Error != "" || out.ErrorMsg != "" {
		return nil, fmt.Errorf("baidu api error: %s - %s", out.Error, out.ErrorMsg)
	}
	return &out, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func decodeLimitedJSON(r io.Reader, out interface{}) error {
	bodyBytes, err := io.ReadAll(io.LimitReader(r, maxBaiduResponseLen))
	if err != nil {
		return fmt.Errorf("read body failed: %w", err)
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("json unmarshal failed: %w", err)
	}
	return nil
}
