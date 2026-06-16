package baidu

import (
	"net/url"
	"testing"
)

func TestGetAuthURLIncludesStateAndDisplay(t *testing.T) {
	client := NewClient("device-id", "app-key", "secret", "https://example.com/callback")

	authURL := client.GetAuthURL("state-token", false)
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	q := parsed.Query()

	assertQuery(t, q, "client_id", "app-key")
	assertQuery(t, q, "redirect_uri", "https://example.com/callback")
	assertQuery(t, q, "state", "state-token")
	assertQuery(t, q, "device_id", "device-id")
	assertQuery(t, q, "display", "page")
	assertQuery(t, q, "qrcode", "1")
}

func TestGetAuthURLMobileDisplay(t *testing.T) {
	client := NewClient("", "app-key", "secret", "https://example.com/callback")
	parsed, err := url.Parse(client.GetAuthURL("state-token", true))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	assertQuery(t, parsed.Query(), "display", "mobile")
}

func assertQuery(t *testing.T, q url.Values, key string, want string) {
	t.Helper()
	if got := q.Get(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
