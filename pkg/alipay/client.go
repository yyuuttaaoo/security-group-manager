package alipay

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	gatewayURL           = "https://openapi.alipay.com/gateway.do"
	maxAlipayResponseLen = 5 * 1024 * 1024
)

type Client struct {
	AppID              string
	PrivateKey         *rsa.PrivateKey
	AlipayPublicKey    *rsa.PublicKey
	AppCertSN          string
	AlipayRootCertSN   string
	AlipayPublicCertSN string
	HTTPClient         *http.Client
}

func NewClient(appID, privateKeyPath, appCertPath, alipayPublicCertPath, alipayRootCertPath string) (*Client, error) {
	privBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key failed: %w", err)
	}

	privKey, err := parsePrivateKey(privBytes)
	if err != nil {
		return nil, err
	}

	appCert, err := ParseCertFile(appCertPath)
	if err != nil {
		return nil, fmt.Errorf("read app cert failed: %w", err)
	}

	aliCert, err := ParseCertFile(alipayPublicCertPath)
	if err != nil {
		return nil, fmt.Errorf("read alipay public cert failed: %w", err)
	}
	aliPubKey, ok := aliCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("alipay public key is not an RSA key")
	}

	rootBytes, err := os.ReadFile(alipayRootCertPath)
	if err != nil {
		return nil, fmt.Errorf("read alipay root cert failed: %w", err)
	}
	rootSN, err := GetRootCertSN(rootBytes)
	if err != nil {
		return nil, fmt.Errorf("get root SN failed: %w", err)
	}

	return &Client{
		AppID:              appID,
		PrivateKey:         privKey,
		AlipayPublicKey:    aliPubKey,
		AppCertSN:          GetCertSN(appCert),
		AlipayRootCertSN:   rootSN,
		AlipayPublicCertSN: GetCertSN(aliCert),
		HTTPClient:         http.DefaultClient,
	}, nil
}

func parsePrivateKey(privBytes []byte) (*rsa.PrivateKey, error) {
	var derBytes []byte
	privBlock, _ := pem.Decode(privBytes)
	if privBlock == nil {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(privBytes)))
		if err != nil {
			return nil, fmt.Errorf("failed to decode private key")
		}
		derBytes = decoded
	} else {
		derBytes = privBlock.Bytes
	}

	if key, err := x509.ParsePKCS8PrivateKey(derBytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not an RSA key")
		}
		return rsaKey, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(derBytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("failed to parse private key")
}

func (c *Client) Sign(params map[string]string) (string, error) {
	content := canonicalContent(params)
	hashed := sha256.Sum256([]byte(content))

	signature, err := rsa.SignPKCS1v15(nil, c.PrivateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func (c *Client) Verify(content, sign string) error {
	signBytes, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return err
	}

	hashed := sha256.Sum256([]byte(content))
	return rsa.VerifyPKCS1v15(c.AlipayPublicKey, crypto.SHA256, hashed[:], signBytes)
}

func (c *Client) DoRequest(method string, systemParams map[string]string, bizContent map[string]interface{}, responseNode string, out interface{}) error {
	params := map[string]string{
		"app_id":              c.AppID,
		"method":              method,
		"format":              "JSON",
		"charset":             "UTF-8",
		"sign_type":           "RSA2",
		"timestamp":           time.Now().Format("2006-01-02 15:04:05"),
		"version":             "1.0",
		"app_cert_sn":         c.AppCertSN,
		"alipay_root_cert_sn": c.AlipayRootCertSN,
	}

	for k, v := range systemParams {
		params[k] = v
	}
	if bizContent != nil {
		b, _ := json.Marshal(bizContent)
		params["biz_content"] = string(b)
	}

	sign, err := c.Sign(params)
	if err != nil {
		return fmt.Errorf("sign failed: %w", err)
	}
	params["sign"] = sign

	data := url.Values{}
	for k, v := range params {
		data.Set(k, v)
	}

	slog.Info("Alipay API request", "method", method, "url", gatewayURL)
	req, err := http.NewRequest(http.MethodPost, gatewayURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxAlipayResponseLen))
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &root); err != nil {
		return fmt.Errorf("unmarshal root failed: %w", err)
	}
	if errResp, ok := root["error_response"]; ok {
		return fmt.Errorf("alipay error: %s", string(errResp))
	}

	nodeBytes, ok := root[responseNode]
	if !ok {
		return fmt.Errorf("response node %s not found", responseNode)
	}

	if signNode, ok := root["sign"]; ok {
		var signStr string
		if err := json.Unmarshal(signNode, &signStr); err == nil && signStr != "" {
			if err := c.Verify(string(nodeBytes), signStr); err != nil {
				return fmt.Errorf("verify failed: %w", err)
			}
		}
	}

	if err := json.Unmarshal(nodeBytes, out); err != nil {
		return fmt.Errorf("unmarshal business node failed: %w", err)
	}
	return nil
}

type OauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	OpenID       string `json:"open_id"`
	UnionID      string `json:"union_id"`
	ExpiresIn    int    `json:"expires_in"`
	ReExpiresIn  int    `json:"re_expires_in"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
}

func (c *Client) GetOauthToken(authCode string) (*OauthTokenResponse, error) {
	systemParams := map[string]string{
		"grant_type": "authorization_code",
		"code":       authCode,
	}

	var out OauthTokenResponse
	err := c.DoRequest("alipay.system.oauth.token", systemParams, nil, "alipay_system_oauth_token_response", &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type UserInfoResponse struct {
	Code     string `json:"code"`
	Msg      string `json:"msg"`
	UserID   string `json:"user_id"`
	Avatar   string `json:"avatar"`
	Province string `json:"province"`
	City     string `json:"city"`
	NickName string `json:"nick_name"`
	Gender   string `json:"gender"`
}

func (c *Client) GetUserInfo(accessToken string) (*UserInfoResponse, error) {
	systemParams := map[string]string{
		"auth_token": accessToken,
	}

	var out UserInfoResponse
	err := c.DoRequest("alipay.user.info.share", systemParams, nil, "alipay_user_info_share_response", &out)
	if err != nil {
		return nil, err
	}
	if out.Code != "" && out.Code != "10000" {
		return nil, fmt.Errorf("alipay API returned code %s msg %s", out.Code, out.Msg)
	}
	return &out, nil
}

func canonicalContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k != "sign" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, params[k]))
	}
	return strings.Join(pairs, "&")
}
