package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yyuuttaaoo/security-group-manager/pkg/alipay"
	"github.com/yyuuttaaoo/security-group-manager/pkg/auth"
	"github.com/yyuuttaaoo/security-group-manager/pkg/baidu"
	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
	"github.com/yyuuttaaoo/security-group-manager/pkg/logger"
	"github.com/yyuuttaaoo/security-group-manager/pkg/manager"
	"github.com/yyuuttaaoo/security-group-manager/pkg/utils"
)

const (
	oauthStateCookieName = "oauth_state"
	oauthStateDuration   = 5 * time.Minute
)

var authenticator *auth.Authenticator
var alipayClient *alipay.Client
var baiduClient *baidu.Client
var alipayRedirectURI string

type UpdateRequest struct {
	IP    string `json:"ip"`
	Group string `json:"group"`
}

type UpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Logs    string `json:"logs"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type UserInfoResponse struct {
	Username string   `json:"username"`
	UserID   string   `json:"user_id"`
	Groups   []string `json:"groups"`
}

func setOAuthStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   authenticator.Config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(oauthStateDuration),
		MaxAge:   int(oauthStateDuration.Seconds()),
	})
}

func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   authenticator.Config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func validateOAuthState(r *http.Request, state string) bool {
	cookie, err := r.Cookie(oauthStateCookieName)
	return err == nil && cookie.Value != "" && cookie.Value == state
}

func handleAlipayLogin(w http.ResponseWriter, r *http.Request) {
	if alipayClient == nil {
		http.Error(w, "Alipay OAuth not configured", http.StatusInternalServerError)
		return
	}

	state, err := auth.GenerateRandomString(32)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	setOAuthStateCookie(w, state)

	authURL := fmt.Sprintf("https://openauth.alipay.com/oauth2/publicAppAuthorize.htm?app_id=%s&scope=auth_user&redirect_uri=%s&state=%s",
		alipayClient.AppID,
		url.QueryEscape(alipayRedirectURI),
		url.QueryEscape(state))

	if isMobileUserAgent(r.UserAgent()) {
		finalURL := fmt.Sprintf("alipays://platformapi/startapp?appId=20000067&url=%s", url.QueryEscape(authURL))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<html>
<head><meta name="viewport" content="width=device-width, initial-scale=1"><title>Jumping to Alipay</title></head>
<body style="font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; background-color: #0f172a; color: #e2e8f0; flex-direction: column;">
	<p>Opening Alipay App...</p>
	<a href="%s" style="margin-top: 20px; padding: 10px 20px; background-color: #3b82f6; color: white; text-decoration: none; border-radius: 5px;">Click here if app does not open</a>
	<a href="%s" style="margin-top: 20px; color: #94a3b8; text-decoration: none;">Continue in browser instead</a>
	<script>
		window.location.href = "%s";
		setTimeout(function() { window.location.href = "%s"; }, 3000);
	</script>
</body>
</html>
`, finalURL, authURL, finalURL, authURL)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func handleAlipayCallback(w http.ResponseWriter, r *http.Request) {
	if alipayClient == nil {
		http.Error(w, "Alipay OAuth not configured", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		authCode := r.URL.Query().Get("auth_code")
		state := r.URL.Query().Get("state")
		if authCode == "" {
			clearOAuthStateCookie(w)
			http.Error(w, "Missing auth_code", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<html>
<head><meta name="viewport" content="width=device-width, initial-scale=1"><title>Authenticating...</title></head>
<body style="font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; background-color: #0f172a; color: #e2e8f0;">
	<p>Authenticating...</p>
	<form id="authForm" method="POST" action="/api/oauth/alipay/callback">
		<input type="hidden" name="auth_code" value="%s">
		<input type="hidden" name="state" value="%s">
	</form>
	<script>document.getElementById('authForm').submit();</script>
</body>
</html>`, template.HTMLEscapeString(authCode), template.HTMLEscapeString(state))
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authCode := r.FormValue("auth_code")
	state := r.FormValue("state")
	if authCode == "" {
		clearOAuthStateCookie(w)
		http.Error(w, "Missing auth_code", http.StatusBadRequest)
		return
	}
	if !validateOAuthState(r, state) {
		clearOAuthStateCookie(w)
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	tokenResp, err := alipayClient.GetOauthToken(authCode)
	if err != nil {
		clearOAuthStateCookie(w)
		slog.Error("Alipay token exchange failed", "error", err)
		writeOAuthRetryPage(w, http.StatusInternalServerError, "Authorization Failed or Expired", "Your Alipay authorization code is invalid or has already been consumed. Please try logging in again.", "/api/oauth/alipay/login")
		return
	}

	externalID := tokenResp.UserID
	displayName := ""
	if tokenResp.AccessToken != "" {
		userInfo, err := alipayClient.GetUserInfo(tokenResp.AccessToken)
		if err == nil && userInfo != nil {
			if userInfo.UserID != "" {
				externalID = userInfo.UserID
			}
			displayName = userInfo.NickName
		} else {
			slog.Warn("Failed to fetch Alipay user info", "error", err)
		}
	}
	if externalID == "" {
		externalID = tokenResp.OpenID
	}
	if externalID == "" {
		clearOAuthStateCookie(w)
		http.Error(w, "Failed to extract Alipay user id", http.StatusInternalServerError)
		return
	}

	user, err := authenticator.IssueSession("alipay", externalID, displayName)
	if err != nil {
		clearOAuthStateCookie(w)
		slog.Warn("Unauthorized Alipay login attempt", "user_id", externalID, "error", err)
		writeUnauthorizedPage(w, "Alipay User ID", externalID)
		return
	}

	authenticator.SetSession(w, user)
	clearOAuthStateCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func handleBaiduLogin(w http.ResponseWriter, r *http.Request) {
	if baiduClient == nil {
		http.Error(w, "Baidu OAuth not configured", http.StatusInternalServerError)
		return
	}

	state, err := auth.GenerateRandomString(32)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	setOAuthStateCookie(w, state)

	http.Redirect(w, r, baiduClient.GetAuthURL(state, isMobileUserAgent(r.UserAgent())), http.StatusFound)
}

func handleBaiduCallback(w http.ResponseWriter, r *http.Request) {
	if baiduClient == nil {
		http.Error(w, "Baidu OAuth not configured", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		clearOAuthStateCookie(w)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	authCode := r.FormValue("code")
	state := r.FormValue("state")
	if authCode == "" {
		clearOAuthStateCookie(w)
		http.Error(w, "Missing auth code", http.StatusBadRequest)
		return
	}
	if !validateOAuthState(r, state) {
		clearOAuthStateCookie(w)
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	tokenResp, err := baiduClient.GetOauthToken(authCode)
	if err != nil {
		clearOAuthStateCookie(w)
		slog.Error("Baidu token exchange failed", "error", err)
		writeOAuthRetryPage(w, http.StatusInternalServerError, "Authorization Failed or Expired", "Your Baidu authorization code is invalid or has already been consumed. Please try logging in again.", "/api/oauth/baidu/login")
		return
	}

	userInfo, err := baiduClient.GetUserInfo(tokenResp.AccessToken)
	if err != nil || userInfo == nil || userInfo.OpenID == "" {
		clearOAuthStateCookie(w)
		slog.Error("Failed to fetch Baidu user info", "error", err)
		http.Error(w, "Failed to extract openid from Baidu", http.StatusInternalServerError)
		return
	}

	user, err := authenticator.IssueSession("baidu", userInfo.OpenID, userInfo.Username)
	if err != nil {
		clearOAuthStateCookie(w)
		slog.Warn("Unauthorized Baidu login attempt", "baidu_openid", userInfo.OpenID, "error", err)
		writeUnauthorizedPage(w, "Baidu OpenID", userInfo.OpenID)
		return
	}

	authenticator.SetSession(w, user)
	clearOAuthStateCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	authenticator.ClearSession(w)
	clearOAuthStateCookie(w)
	json.NewEncoder(w).Encode(LoginResponse{Success: true, Message: "Logged out"})
}

func handleUserInfo(w http.ResponseWriter, r *http.Request) {
	user, err := authenticator.GetUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(UserInfoResponse{
		Username: user.DisplayName,
		UserID:   user.UserID,
		Groups:   user.Groups,
	})
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.Context().Value(auth.UserContextKey).(*auth.User)
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.IP == "" {
		http.Error(w, "IP is required", http.StatusBadRequest)
		return
	}
	if err := utils.ValidateIPOrCIDR(req.IP); err != nil {
		http.Error(w, fmt.Sprintf("Invalid IP: %v", err), http.StatusBadRequest)
		return
	}

	groupName := req.Group
	if groupName == "" {
		groupName = "default"
	}
	if !authenticator.HasGroupAccess(user, groupName) {
		http.Error(w, fmt.Sprintf("You do not have permission to modify group '%s'", groupName), http.StatusForbidden)
		return
	}

	slog.Info("Request started", "method", r.Method, "path", r.URL.Path, "user_id", user.UserID)

	var logBuffer bytes.Buffer
	regions := []string{"cn-hongkong", "ap-northeast-1", "us-west-1"}
	mw := io.MultiWriter(logger.LogWriter, &logBuffer)
	reqLogger := slog.New(slog.NewTextHandler(mw, nil))

	var errOccurred bool
	for _, region := range regions {
		reqLogger.Info("Processing Region", "region", region, "group", groupName)
		if err := manager.ProcessRegion(region, req.IP, groupName, reqLogger); err != nil {
			reqLogger.Error("Error processing region", "region", region, "error", err)
			errOccurred = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UpdateResponse{
		Success: !errOccurred,
		Message: "Update process completed",
		Logs:    logBuffer.String(),
	})
}

type GeoInfo struct {
	IP          string `json:"ip"`
	City        string `json:"city"`
	Region      string `json:"region"`
	CountryName string `json:"country_name"`
	Org         string `json:"org"`
}

func handleGetIP(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	respData := GeoInfo{IP: ip}

	parsedIP := net.ParseIP(ip)
	if parsedIP != nil && !isPrivateOrLoopback(parsedIP) {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://ipapi.co/%s/json/", ip), nil)
		if err == nil {
			req.Header.Set("User-Agent", "SecurityGroupManager/1.0")
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var geo GeoInfo
					if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&geo); err == nil {
						respData.City = geo.City
						respData.Region = geo.Region
						respData.CountryName = geo.CountryName
						respData.Org = geo.Org
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(respData)
}

func handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `
<html>
<body style="font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; background-color: #0f172a; color: #e2e8f0; flex-direction: column;">
	<h2>Local Dev Login</h2>
	<form method="POST" action="/api/dev/login" style="display: flex; flex-direction: column; gap: 10px;">
		<input type="text" name="user_id" placeholder="Enter configured UID" style="padding: 10px; border-radius: 5px; border: 1px solid #475569; background: #1e293b; color: white; width: 300px;">
		<input type="text" name="display_name" placeholder="Display Name (optional)" style="padding: 10px; border-radius: 5px; border: 1px solid #475569; background: #1e293b; color: white; width: 300px;">
		<button type="submit" style="padding: 10px; background-color: #3b82f6; color: white; border: none; border-radius: 5px; cursor: pointer;">Login</button>
	</form>
</body>
</html>`)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.FormValue("user_id")
	displayName := r.FormValue("display_name")
	user, err := authenticator.IssueSession("dev", userID, displayName)
	if err != nil {
		http.Error(w, fmt.Sprintf("UID %s is not authorized in config users list", userID), http.StatusForbidden)
		return
	}

	authenticator.SetSession(w, user)
	http.Redirect(w, r, "/", http.StatusFound)
}

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	logger.Setup(cfg.Log)
	slog.Info("Starting server", "auth_enabled", cfg.Auth.Enabled, "address", cfg.Server.Address, "port", cfg.Server.Port)

	authenticator = auth.NewAuthenticator(cfg.Auth)
	alipayRedirectURI = cfg.Alipay.RedirectURI

	if cfg.Alipay.AppID != "" {
		client, err := alipay.NewClient(
			cfg.Alipay.AppID,
			cfg.Alipay.PrivateKeyPath,
			cfg.Alipay.AppCertPath,
			cfg.Alipay.AlipayPublicCertPath,
			cfg.Alipay.AlipayRootCertPath,
		)
		if err != nil {
			slog.Error("Failed to init Alipay client", "error", err)
			os.Exit(1)
		}
		alipayClient = client
	}

	if cfg.Baidu.AppKey != "" {
		baiduClient = baidu.NewClient(
			cfg.Baidu.AppID,
			cfg.Baidu.AppKey,
			cfg.Baidu.SecretKey,
			cfg.Baidu.RedirectURI,
		)
	}

	fs := http.FileServer(http.Dir("web"))
	http.Handle("/", fs)

	http.HandleFunc("/api/oauth/alipay/login", handleAlipayLogin)
	http.HandleFunc("/api/oauth/alipay/callback", handleAlipayCallback)
	http.HandleFunc("/api/oauth/baidu/login", handleBaiduLogin)
	http.HandleFunc("/api/oauth/baidu/callback", handleBaiduCallback)
	http.HandleFunc("/api/oauth/login", handleAlipayLogin)
	http.HandleFunc("/api/oauth/callback", handleAlipayCallback)
	http.HandleFunc("/api/logout", authenticator.CSRFMiddleware(handleLogout))
	http.HandleFunc("/api/user/info", handleUserInfo)

	if cfg.Server.DevMode {
		http.HandleFunc("/api/dev/login", handleDevLogin)
		slog.Warn("DEV MODE is enabled. /api/dev/login is accessible.")
	}

	http.HandleFunc("/api/update", authenticator.CSRFMiddleware(authenticator.Middleware(handleUpdate)))
	http.HandleFunc("/api/ip", handleGetIP)

	port := os.Getenv("PORT")
	if port == "" {
		if cfg.Server.Port != "" {
			port = cfg.Server.Port
		} else {
			port = "8080"
		}
	}

	addr := cfg.Server.Address + ":" + port
	slog.Info("Server listening", "address", addr, "tls", cfg.Server.TLS)

	if cfg.Server.TLS {
		if err := http.ListenAndServeTLS(addr, cfg.Server.CertFile, cfg.Server.KeyFile, nil); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	} else {
		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}
}

func isMobileUserAgent(userAgent string) bool {
	return strings.Contains(userAgent, "Mobile") || strings.Contains(userAgent, "Android") || strings.Contains(userAgent, "iPhone")
}

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	for _, header := range []string{"CF-Connecting-IP", "True-Client-IP", "X-Real-IP", "X-Forwarded-For"} {
		if value := r.Header.Get(header); value != "" {
			ip = value
			break
		}
	}

	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	} else if strings.Contains(ip, ",") {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	return strings.TrimSpace(ip)
}

func isPrivateOrLoopback(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate()
}

func writeOAuthRetryPage(w http.ResponseWriter, status int, title string, message string, loginPath string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `
<html>
<body style="font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; background-color: #0f172a; color: #e2e8f0; flex-direction: column; text-align: center;">
	<h2>%s</h2>
	<p>%s</p>
	<a href="%s" style="margin-top: 20px; padding: 10px 20px; background-color: #3b82f6; color: white; text-decoration: none; border-radius: 5px;">Try Again</a>
</body>
</html>`, template.HTMLEscapeString(title), template.HTMLEscapeString(message), template.HTMLEscapeString(loginPath))
}

func writeUnauthorizedPage(w http.ResponseWriter, label string, externalID string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, `
<html>
<body style="font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; background-color: #0f172a; color: #e2e8f0; flex-direction: column;">
	<h2>Unauthorized Access</h2>
	<p>%s <strong style="color: #ef4444;">%s</strong> is not authorized to access this system.</p>
	<p style="color: #94a3b8; font-size: 0.875rem; margin-top: 0.5rem;">Please add this identifier to your server config.yaml to grant access.</p>
	<a href="/" style="margin-top: 20px; padding: 10px 20px; background-color: #3b82f6; color: white; text-decoration: none; border-radius: 5px;">Back to Login</a>
</body>
</html>`, template.HTMLEscapeString(label), template.HTMLEscapeString(externalID))
}
