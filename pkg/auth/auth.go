package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
)

const (
	CSRFCookieName    = "sgm_csrf"
	CSRFHeaderName    = "X-CSRF-Token"
	SessionCookieName = "sgm_session"
	SessionDuration   = 24 * time.Hour
	sessionCookiePath = "/"
	csrfCookiePath    = "/"
)

type User struct {
	UserID      string   `json:"user_id"`
	DisplayName string   `json:"display_name"`
	Groups      []string `json:"groups"`
}

type Authenticator struct {
	Config config.AuthConfig
}

func NewAuthenticator(cfg config.AuthConfig) *Authenticator {
	return &Authenticator{Config: cfg}
}

func (a *Authenticator) IssueSession(provider string, externalID string, displayName string) (*User, error) {
	for _, u := range a.Config.Users {
		uid := userUID(u)
		if uid == "" {
			continue
		}

		match := false
		switch provider {
		case "alipay":
			match = u.AlipayUserID != "" && u.AlipayUserID == externalID
		case "baidu":
			match = u.BaiduOpenID != "" && u.BaiduOpenID == externalID
		case "dev":
			match = uid == externalID
		}

		if match {
			if displayName == "" {
				displayName = uid
			}
			return &User{
				UserID:      uid,
				DisplayName: displayName,
				Groups:      u.Groups,
			}, nil
		}
	}
	return nil, fmt.Errorf("user not found or unauthorized")
}

func (a *Authenticator) sign(data string) string {
	h := hmac.New(sha256.New, []byte(a.Config.SessionSecret))
	h.Write([]byte(data))
	signature := base64.URLEncoding.EncodeToString(h.Sum(nil))
	return data + "." + signature
}

func (a *Authenticator) verify(signedData string) (string, error) {
	parts := strings.Split(signedData, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid signature format")
	}
	data := parts[0]
	signature := parts[1]

	expectedSignature := a.sign(data)
	expectedParts := strings.Split(expectedSignature, ".")
	if len(expectedParts) != 2 {
		return "", fmt.Errorf("internal error signing data")
	}

	if !hmac.Equal([]byte(signature), []byte(expectedParts[1])) {
		return "", fmt.Errorf("invalid signature")
	}
	return data, nil
}

func GenerateRandomString(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (a *Authenticator) SetCSRFCookie(w http.ResponseWriter) (string, error) {
	return a.setCSRFCookie(w, time.Now().Add(SessionDuration))
}

func (a *Authenticator) setCSRFCookie(w http.ResponseWriter, expires time.Time) (string, error) {
	token, err := GenerateRandomString(32)
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     csrfCookiePath,
		HttpOnly: false,
		Secure:   a.Config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	})
	return token, nil
}

func (a *Authenticator) CSRFMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Config.Enabled {
			next(w, r)
			return
		}

		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			cookie, err := r.Cookie(CSRFCookieName)
			if err != nil {
				http.Error(w, "Missing CSRF cookie", http.StatusForbidden)
				return
			}

			headerToken := r.Header.Get(CSRFHeaderName)
			if headerToken == "" {
				http.Error(w, "Missing CSRF header", http.StatusForbidden)
				return
			}

			if cookie.Value != headerToken {
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		next(w, r)
	}
}

func (a *Authenticator) SetSession(w http.ResponseWriter, user *User) {
	expires := time.Now().Add(SessionDuration)
	b, _ := json.Marshal(user)
	val := a.sign(base64.URLEncoding.EncodeToString(b))

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    val,
		Path:     sessionCookiePath,
		HttpOnly: true,
		Secure:   a.Config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	})

	a.setCSRFCookie(w, expires)
}

func (a *Authenticator) ClearSession(w http.ResponseWriter) {
	clearCookie(w, SessionCookieName, sessionCookiePath, true, a.Config.CookieSecure)
	clearCookie(w, CSRFCookieName, csrfCookiePath, false, a.Config.CookieSecure)
}

func clearCookie(w http.ResponseWriter, name string, path string, httpOnly bool, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (a *Authenticator) GetUserFromRequest(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, err
	}

	signedVal, err := a.verify(cookie.Value)
	if err != nil {
		return nil, err
	}

	b, err := base64.URLEncoding.DecodeString(signedVal)
	if err != nil {
		return a.getLegacySessionUser(signedVal)
	}

	var sessionUser User
	if err := json.Unmarshal(b, &sessionUser); err != nil {
		return nil, fmt.Errorf("invalid session format")
	}

	for _, u := range a.Config.Users {
		uid := userUID(u)
		if uid == sessionUser.UserID {
			displayName := sessionUser.DisplayName
			if displayName == "" {
				displayName = uid
			}
			return &User{
				UserID:      uid,
				DisplayName: displayName,
				Groups:      u.Groups,
			}, nil
		}
	}
	return nil, fmt.Errorf("invalid session")
}

func (a *Authenticator) getLegacySessionUser(username string) (*User, error) {
	for _, u := range a.Config.Users {
		if u.LegacyUsername == username {
			uid := userUID(u)
			if uid == "" {
				uid = username
			}
			return &User{
				UserID:      uid,
				DisplayName: uid,
				Groups:      u.Groups,
			}, nil
		}
	}
	return nil, fmt.Errorf("invalid session encoding")
}

type contextKey string

const UserContextKey contextKey = "user"

func (a *Authenticator) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Config.Enabled {
			next(w, r)
			return
		}

		user, err := a.GetUserFromRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (a *Authenticator) HasGroupAccess(user *User, group string) bool {
	for _, g := range user.Groups {
		if g == group {
			return true
		}
	}
	return false
}

func userUID(u config.UserConfig) string {
	if u.UID != "" {
		return u.UID
	}
	return u.LegacyUsername
}
