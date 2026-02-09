package south2md

import (
	"strings"
	"testing"
)

func TestDownloadAndCacheImagesReplacesOnlyImageURLs(t *testing.T) {
	h := NewImageHandler("images")
	h.SetDownloadEnabled(false)

	post := &Post{
		Images: []Image{
			{
				URL:        "https://cdn.example.com/a.jpg",
				Local:      "a-local.jpg",
				Downloaded: true,
			},
		},
	}

	markdown := strings.Join([]string{
		"![img](https://cdn.example.com/a.jpg)",
		"[link](https://cdn.example.com/a.jpg)",
	}, "\n")

	got, err := h.DownloadAndCacheImages("100", []byte(markdown), post)
	if err != nil {
		t.Fatalf("DownloadAndCacheImages returned error: %v", err)
	}

	gotText := string(got)
	if !strings.Contains(gotText, "![img](images/a-local.jpg)") {
		t.Fatalf("expected image URL replacement, got: %q", gotText)
	}
	if !strings.Contains(gotText, "[link](https://cdn.example.com/a.jpg)") {
		t.Fatalf("expected non-image link unchanged, got: %q", gotText)
	}
}

func TestDownloadAndCacheImagesDoesNotLeakMappingAcrossCalls(t *testing.T) {
	h := NewImageHandler("images")
	h.SetDownloadEnabled(false)

	post1 := &Post{
		Images: []Image{{
			URL:        "https://img.example.com/one.jpg",
			Local:      "one-local.jpg",
			Downloaded: true,
		}},
	}
	post2 := &Post{
		Images: []Image{{
			URL:        "https://img.example.com/two.jpg",
			Local:      "two-local.jpg",
			Downloaded: true,
		}},
	}

	first, err := h.DownloadAndCacheImages("101", []byte("![one](https://img.example.com/one.jpg)"), post1)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if !strings.Contains(string(first), "images/one-local.jpg") {
		t.Fatalf("expected first replacement, got: %q", string(first))
	}

	second, err := h.DownloadAndCacheImages("102", []byte("![two](https://img.example.com/two.jpg)"), post2)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	secondText := string(second)
	if !strings.Contains(secondText, "images/two-local.jpg") {
		t.Fatalf("expected second replacement, got: %q", secondText)
	}
	if strings.Contains(secondText, "one-local.jpg") {
		t.Fatalf("unexpected cross-call mapping leak: %q", secondText)
	}
}
