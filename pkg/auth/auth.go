package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
)

const (
	csrfCookieName = "sgm_csrf"
	csrfHeaderName = "X-CSRF-Token"
)

type User struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

type Authenticator struct {
	Config config.AuthConfig
}

func NewAuthenticator(cfg config.AuthConfig) *Authenticator {
	return &Authenticator{Config: cfg}
}

// Simple cookie-based session
const sessionCookieName = "sgm_session"

func (a *Authenticator) Login(username, password string) (*User, error) {
	for _, u := range a.Config.Users {
		if u.Username == username {
			// Decode base64 password
			decodedPwd, err := base64.StdEncoding.DecodeString(u.Password)
			if err != nil {
				return nil, fmt.Errorf("config error: invalid base64 password")
			}
			if string(decodedPwd) == password {
				return &User{
					Username: u.Username,
					Groups:   u.Groups,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("invalid credentials")
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

	// Constant time comparison to prevent timing attacks
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
	token, err := GenerateRandomString(32)
	if err != nil {
		return "", err
	}

	// CSRF cookie should NOT be HttpOnly so JS can read it
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false, // Important!
		Secure:   a.Config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	return token, nil
}

func (a *Authenticator) CSRFMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Config.Enabled {
			next(w, r)
			return
		}

		// Only check for state-changing methods
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			cookie, err := r.Cookie(csrfCookieName)
			if err != nil {
				http.Error(w, "Missing CSRF cookie", http.StatusForbidden)
				return
			}

			headerToken := r.Header.Get(csrfHeaderName)
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
	val := a.sign(user.Username)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.Config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	// Set CSRF cookie on login
	a.SetCSRFCookie(w)
}

func (a *Authenticator) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (a *Authenticator) GetUserFromRequest(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}

	username, err := a.verify(cookie.Value)
	if err != nil {
		return nil, err
	}

	// Validate username exists in config
	for _, u := range a.Config.Users {
		if u.Username == username {
			return &User{
				Username: u.Username,
				Groups:   u.Groups,
			}, nil
		}
	}
	return nil, fmt.Errorf("invalid session")
}

// Middleware
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
