package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
)

func testAuthenticator() *Authenticator {
	return NewAuthenticator(config.AuthConfig{
		Enabled:       true,
		SessionSecret: "test-secret",
		Users: []config.UserConfig{
			{
				UID:          "admin",
				AlipayUserID: "2088123456789012",
				BaiduOpenID:  "baidu-openid",
				Groups:       []string{"home", "admin"},
			},
		},
	})
}

func TestIssueSessionProviderMapping(t *testing.T) {
	a := testAuthenticator()

	user, err := a.IssueSession("alipay", "2088123456789012", "Alice")
	if err != nil {
		t.Fatalf("IssueSession(alipay) failed: %v", err)
	}
	if user.UserID != "admin" || user.DisplayName != "Alice" || len(user.Groups) != 2 {
		t.Fatalf("unexpected user: %+v", user)
	}

	user, err = a.IssueSession("baidu", "baidu-openid", "")
	if err != nil {
		t.Fatalf("IssueSession(baidu) failed: %v", err)
	}
	if user.UserID != "admin" || user.DisplayName != "admin" {
		t.Fatalf("unexpected baidu user: %+v", user)
	}
}

func TestIssueSessionRejectsUnknownProviderIdentity(t *testing.T) {
	_, err := testAuthenticator().IssueSession("alipay", "unknown", "Mallory")
	if err == nil {
		t.Fatal("expected unknown provider identity to be rejected")
	}
}

func TestSetSessionIssuesSessionAndCSRFCookies(t *testing.T) {
	a := testAuthenticator()
	rr := httptest.NewRecorder()
	a.SetSession(rr, &User{UserID: "admin", DisplayName: "Alice", Groups: []string{"home"}})

	cookies := cookiesByName(rr.Result().Cookies())
	if cookies[SessionCookieName] == nil {
		t.Fatalf("missing %s cookie", SessionCookieName)
	}
	if cookies[CSRFCookieName] == nil {
		t.Fatalf("missing %s cookie", CSRFCookieName)
	}
	if cookies[SessionCookieName].Expires.IsZero() || cookies[CSRFCookieName].Expires.IsZero() {
		t.Fatal("session and csrf cookies must both have explicit expiry")
	}
	if !cookies[SessionCookieName].Expires.Equal(cookies[CSRFCookieName].Expires) {
		t.Fatalf("session and csrf expiry mismatch: %v != %v", cookies[SessionCookieName].Expires, cookies[CSRFCookieName].Expires)
	}
}

func TestClearSessionClearsSessionAndCSRF(t *testing.T) {
	a := testAuthenticator()
	rr := httptest.NewRecorder()
	a.ClearSession(rr)

	cookies := cookiesByName(rr.Result().Cookies())
	for _, name := range []string{SessionCookieName, CSRFCookieName} {
		cookie := cookies[name]
		if cookie == nil {
			t.Fatalf("missing cleared %s cookie", name)
		}
		if cookie.MaxAge != -1 {
			t.Fatalf("%s cookie MaxAge = %d, want -1", name, cookie.MaxAge)
		}
	}
}

func TestCSRFMiddlewareRejectsMissingToken(t *testing.T) {
	a := testAuthenticator()
	handler := a.CSRFMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	handler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func cookiesByName(cookies []*http.Cookie) map[string]*http.Cookie {
	out := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		out[cookie.Name] = cookie
	}
	return out
}
