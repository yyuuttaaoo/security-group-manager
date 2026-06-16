package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/yyuuttaaoo/security-group-manager/pkg/alipay"
	"github.com/yyuuttaaoo/security-group-manager/pkg/auth"
	"github.com/yyuuttaaoo/security-group-manager/pkg/baidu"
	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
)

func TestOAuthStateCookieLifecycle(t *testing.T) {
	authenticator = auth.NewAuthenticator(config.AuthConfig{SessionSecret: "secret"})

	rr := httptest.NewRecorder()
	setOAuthStateCookie(rr, "state-token")
	cookies := cookiesByName(rr.Result().Cookies())
	if cookies[oauthStateCookieName] == nil {
		t.Fatalf("missing %s cookie", oauthStateCookieName)
	}
	if cookies[oauthStateCookieName].Value != "state-token" {
		t.Fatalf("state cookie value = %q", cookies[oauthStateCookieName].Value)
	}

	req := httptest.NewRequest(http.MethodGet, "/callback?state=state-token", nil)
	req.AddCookie(cookies[oauthStateCookieName])
	if !validateOAuthState(req, "state-token") {
		t.Fatal("expected OAuth state to validate")
	}
	if validateOAuthState(req, "tampered") {
		t.Fatal("expected tampered OAuth state to fail")
	}

	rr = httptest.NewRecorder()
	clearOAuthStateCookie(rr)
	cookies = cookiesByName(rr.Result().Cookies())
	if cookies[oauthStateCookieName] == nil || cookies[oauthStateCookieName].MaxAge != -1 {
		t.Fatalf("oauth state cookie was not cleared: %+v", cookies[oauthStateCookieName])
	}
}

func TestHandleLogoutClearsAuthSecurityAndOAuthState(t *testing.T) {
	authenticator = auth.NewAuthenticator(config.AuthConfig{SessionSecret: "secret"})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	handleLogout(rr, req)

	cookies := cookiesByName(rr.Result().Cookies())
	for _, name := range []string{auth.SessionCookieName, auth.CSRFCookieName, oauthStateCookieName} {
		cookie := cookies[name]
		if cookie == nil {
			t.Fatalf("missing cleared %s cookie", name)
		}
		if cookie.MaxAge != -1 {
			t.Fatalf("%s MaxAge = %d, want -1", name, cookie.MaxAge)
		}
	}
}

func TestAlipayLoginRedirectSetsMatchingState(t *testing.T) {
	authenticator = auth.NewAuthenticator(config.AuthConfig{SessionSecret: "secret"})
	alipayClient = &alipay.Client{AppID: "alipay-app"}
	alipayRedirectURI = "https://example.com/api/oauth/alipay/callback"
	defer func() {
		alipayClient = nil
		alipayRedirectURI = ""
	}()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/alipay/login", nil)
	handleAlipayLogin(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	stateCookie := cookiesByName(rr.Result().Cookies())[oauthStateCookieName]
	if stateCookie == nil || stateCookie.Value == "" {
		t.Fatal("missing oauth state cookie")
	}

	redirectURL, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("redirect URL parse failed: %v", err)
	}
	if got := redirectURL.Query().Get("state"); got != stateCookie.Value {
		t.Fatalf("redirect state = %q, want cookie state %q", got, stateCookie.Value)
	}
}

func TestBaiduLoginRedirectSetsMatchingState(t *testing.T) {
	authenticator = auth.NewAuthenticator(config.AuthConfig{SessionSecret: "secret"})
	baiduClient = baidu.NewClient("device", "baidu-app-key", "secret", "https://example.com/api/oauth/baidu/callback")
	defer func() { baiduClient = nil }()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/baidu/login", nil)
	handleBaiduLogin(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	stateCookie := cookiesByName(rr.Result().Cookies())[oauthStateCookieName]
	if stateCookie == nil || stateCookie.Value == "" {
		t.Fatal("missing oauth state cookie")
	}

	redirectURL, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("redirect URL parse failed: %v", err)
	}
	if got := redirectURL.Query().Get("state"); got != stateCookie.Value {
		t.Fatalf("redirect state = %q, want cookie state %q", got, stateCookie.Value)
	}
}

func TestAlipayCallbackRejectsTamperedState(t *testing.T) {
	authenticator = auth.NewAuthenticator(config.AuthConfig{SessionSecret: "secret"})
	alipayClient = &alipay.Client{AppID: "alipay-app"}
	defer func() { alipayClient = nil }()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/oauth/alipay/callback", nil)
	req.Form = url.Values{
		"auth_code": []string{"code"},
		"state":     []string{"tampered"},
	}
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "expected"})

	handleAlipayCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if stateCookie := cookiesByName(rr.Result().Cookies())[oauthStateCookieName]; stateCookie == nil || stateCookie.MaxAge != -1 {
		t.Fatalf("oauth state was not cleared on bad callback: %+v", stateCookie)
	}
}

func cookiesByName(cookies []*http.Cookie) map[string]*http.Cookie {
	out := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		out[cookie.Name] = cookie
	}
	return out
}
