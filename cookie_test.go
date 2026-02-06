package south2md

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestCookieFileLoad(t *testing.T) {
	content := strings.Join([]string{
		netscapeCookieHeader,
		".example.com\tTRUE\t/\tTRUE\t2147483647\tsessionid\tabc",
		"#HttpOnly_example.com\tFALSE\t/\tFALSE\t0\ttoken\txyz",
		"",
	}, "\n")

	tmp, err := os.CreateTemp("", "cookie-load-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	cm := NewCookieManager()
	if err := cm.LoadFromFile(tmp.Name()); err != nil {
		t.Fatalf("load cookie file: %v", err)
	}

	if len(cm.jar.Cookies) != 2 {
		t.Fatalf("unexpected cookie count: %d", len(cm.jar.Cookies))
	}

	session := findCookie(cm.jar.Cookies, "sessionid")
	if session == nil {
		t.Fatalf("missing sessionid cookie")
	}
	if session.Domain != ".example.com" {
		t.Fatalf("unexpected domain: %s", session.Domain)
	}
	if !session.Secure {
		t.Fatalf("expected secure cookie")
	}
	if session.Expires.IsZero() {
		t.Fatalf("expected expiration to be set")
	}

	token := findCookie(cm.jar.Cookies, "token")
	if token == nil {
		t.Fatalf("missing token cookie")
	}
	if !token.HttpOnly {
		t.Fatalf("expected httponly cookie")
	}
	if !token.Expires.IsZero() {
		t.Fatalf("expected session cookie expiry")
	}
}

func TestCookieFileSaveAndReload(t *testing.T) {
	tmp, err := os.CreateTemp("", "cookie-save-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	cm := NewCookieManager()
	cm.AddCookie(&CookieEntry{
		Name:     "a",
		Value:    "1",
		Domain:   ".example.com",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		Expires:  time.Unix(1700000000, 0),
	})
	cm.AddCookie(&CookieEntry{
		Name:   "b",
		Value:  "2",
		Domain: "example.com",
		Path:   "/path",
	})

	if err := cm.SaveToFile(tmpPath); err != nil {
		t.Fatalf("save cookie file: %v", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("read cookie file: %v", err)
	}
	if !strings.Contains(string(data), netscapeCookieHeader) {
		t.Fatalf("missing cookie header")
	}

	reload := NewCookieManager()
	if err := reload.LoadFromFile(tmpPath); err != nil {
		t.Fatalf("reload cookie file: %v", err)
	}
	if len(reload.jar.Cookies) != 2 {
		t.Fatalf("unexpected cookie count after reload: %d", len(reload.jar.Cookies))
	}
}

func findCookie(cookies []CookieEntry, name string) *CookieEntry {
	for i := range cookies {
		if cookies[i].Name == name {
			return &cookies[i]
		}
	}
	return nil
}
