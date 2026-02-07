package south2md

import (
	"strings"
	"testing"
	"time"
)

func TestBuildCookieRequestHeader(t *testing.T) {
	cookies := []*CookieEntry{
		{
			Name:     "eb9e6_winduser",
			Value:    "abc%3D%3D",
			Domain:   "south-plus.net",
			Path:     "/",
			Expires:  time.Unix(1801598486, 0),
			Secure:   true,
			HttpOnly: true,
		},
		{
			Name:   "cf_clearance",
			Value:  "token-value",
			Domain: ".south-plus.net",
			Path:   "/",
		},
	}

	header := buildCookieRequestHeader(cookies)
	if header == "" {
		t.Fatalf("expected non-empty cookie header")
	}

	if !strings.Contains(header, "eb9e6_winduser=abc%3D%3D") {
		t.Fatalf("missing login cookie: %q", header)
	}
	if !strings.Contains(header, "cf_clearance=token-value") {
		t.Fatalf("missing cf cookie: %q", header)
	}

	if strings.Contains(header, "Path=") || strings.Contains(header, "Domain=") || strings.Contains(header, "HttpOnly") {
		t.Fatalf("request cookie header contains invalid attributes: %q", header)
	}
}

func TestCookieNames(t *testing.T) {
	cookies := []*CookieEntry{
		{Name: "a", Value: "1"},
		nil,
		{Name: "", Value: "x"},
		{Name: "b", Value: "2"},
	}

	names := cookieNames(cookies)
	if len(names) != 2 {
		t.Fatalf("unexpected cookie name count: %d", len(names))
	}
	if names[0] != "a" || names[1] != "b" {
		t.Fatalf("unexpected cookie names: %#v", names)
	}
}
