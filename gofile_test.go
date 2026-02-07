package south2md

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestEnsureAccountTokenParsesGzipResponse(t *testing.T) {
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("unexpected method: %s", req.Method)
				}
				if req.URL.String() != "https://api.gofile.io/accounts" {
					t.Fatalf("unexpected url: %s", req.URL.String())
				}

				body := mustGzipJSON(t, map[string]any{
					"status": "ok",
					"data": map[string]any{
						"token": "tok-test-123",
					},
				})
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(body)),
				}
				resp.Header.Set("Content-Encoding", "gzip")
				return resp, nil
			}),
		},
	}

	token, err := handler.ensureAccountToken()
	if err != nil {
		t.Fatalf("ensureAccountToken failed: %v", err)
	}
	if token != "tok-test-123" {
		t.Fatalf("unexpected token: %s", token)
	}
}

func TestFetchContentParsesGzipBodyWithoutEncodingHeader(t *testing.T) {
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected method: %s", req.Method)
				}
				if !strings.HasPrefix(req.URL.String(), "https://api.gofile.io/contents/abc123") {
					t.Fatalf("unexpected url: %s", req.URL.String())
				}

				body := mustGzipJSON(t, map[string]any{
					"status": "ok",
					"data": map[string]any{
						"id":       "abc123",
						"type":     "folder",
						"name":     "root",
						"children": map[string]any{},
					},
				})
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		},
	}

	content, err := handler.fetchContent("abc123", "token-1", "")
	if err != nil {
		t.Fatalf("fetchContent failed: %v", err)
	}
	if content.ID != "abc123" {
		t.Fatalf("unexpected content id: %s", content.ID)
	}
	if content.Type != "folder" {
		t.Fatalf("unexpected content type: %s", content.Type)
	}
}

func mustGzipJSON(t *testing.T, payload any) []byte {
	t.Helper()

	var raw bytes.Buffer
	zw := gzip.NewWriter(&raw)
	if err := json.NewEncoder(zw).Encode(payload); err != nil {
		t.Fatalf("encode json: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return raw.Bytes()
}

func TestDownloadFileFallbackToFullWhenRangeIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("Range"); got != "bytes=3-" {
					t.Fatalf("unexpected range header: %q", got)
				}
				payload := []byte("abcdef")
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(payload)),
				}
				resp.Header.Set("Content-Length", "6")
				return resp, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "video.mp4",
		Link:     "https://example.com/download/video.mp4",
	}
	partPath := filepath.Join(tmpDir, "video.mp4.part")
	if err := os.WriteFile(partPath, []byte("abc"), 0644); err != nil {
		t.Fatalf("write part file: %v", err)
	}

	if err := handler.downloadFile(file); err != nil {
		t.Fatalf("downloadFile failed: %v", err)
	}

	finalPath := filepath.Join(tmpDir, "video.mp4")
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("unexpected final file content: %q", string(got))
	}
}

func TestDownloadFileExceededRetryReturnsLastError(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &GofileHandler{
		maxRetries: 2,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("server error")),
				}, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "video.mp4",
		Link:     "https://example.com/download/video.mp4",
	}

	err := handler.downloadFile(file)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeded retry limit") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("missing root cause in error: %v", err)
	}
}

func TestDownloadFileSucceedsWithoutContentLength(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("abcdef")),
				}, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "no-length.bin",
		Link:     "https://example.com/download/no-length.bin",
	}

	if err := handler.downloadFile(file); err != nil {
		t.Fatalf("downloadFile failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, "no-length.bin"))
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("unexpected file content: %q", string(got))
	}
}

func TestDownloadFileResumeWithContentLengthFallback(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("Range"); got != "bytes=3-" {
					t.Fatalf("unexpected range header: %q", got)
				}
				resp := &http.Response{
					StatusCode: http.StatusPartialContent,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("def")),
				}
				resp.Header.Set("Content-Length", "3")
				return resp, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "resume.bin",
		Link:     "https://example.com/download/resume.bin",
	}
	partPath := filepath.Join(tmpDir, "resume.bin.part")
	if err := os.WriteFile(partPath, []byte("abc"), 0644); err != nil {
		t.Fatalf("write part file: %v", err)
	}

	if err := handler.downloadFile(file); err != nil {
		t.Fatalf("downloadFile failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, "resume.bin"))
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("unexpected file content: %q", string(got))
	}
}

func TestDownloadFileSendsAuthHeaders(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &GofileHandler{
		token:      "tok-xyz",
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.Header.Get("Authorization"); got != "Bearer tok-xyz" {
					t.Fatalf("missing authorization header: %q", got)
				}
				if got := req.Header.Get("Cookie"); got != "accountToken=tok-xyz" {
					t.Fatalf("missing cookie header: %q", got)
				}
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("abcdef")),
				}
				resp.Header.Set("Content-Length", "6")
				return resp, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "auth.bin",
		Link:     "https://example.com/download/auth.bin",
	}

	if err := handler.downloadFile(file); err != nil {
		t.Fatalf("downloadFile failed: %v", err)
	}
}

func TestDownloadFileRejectsHTMLResponse(t *testing.T) {
	tmpDir := t.TempDir()
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body := "<html><body>forbidden</body></html>"
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				resp.Header.Set("Content-Length", "35")
				return resp, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "video.mp4",
		Link:     "https://example.com/download/video.mp4",
	}

	err := handler.downloadFile(file)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected HTML response body") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadFileSkipsWhenExistingDigestMatches(t *testing.T) {
	tmpDir := t.TempDir()
	requestCount := 0
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("abcdef")),
				}
				resp.Header.Set("Content-Length", "6")
				return resp, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "skip.bin",
		Link:     "https://example.com/download/skip.bin",
		Size:     6,
	}

	if err := handler.downloadFile(file); err != nil {
		t.Fatalf("first downloadFile failed: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("unexpected request count after first download: %d", requestCount)
	}

	if err := handler.downloadFile(file); err != nil {
		t.Fatalf("second downloadFile failed: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected second pull to skip network request, count=%d", requestCount)
	}
}

func TestDownloadFileRedownloadsWhenDigestMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	requestCount := 0
	handler := &GofileHandler{
		maxRetries: 1,
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("abcdef")),
				}
				resp.Header.Set("Content-Length", "6")
				return resp, nil
			}),
		},
	}

	file := gofileRemoteFile{
		Path:     tmpDir,
		Filename: "repair.bin",
		Link:     "https://example.com/download/repair.bin",
		Size:     6,
	}
	finalPath := filepath.Join(tmpDir, "repair.bin")
	if err := os.WriteFile(finalPath, []byte("abcdeg"), 0644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	if err := writeGofileDigest(gofileDigestPath(finalPath), gofileFileDigest{
		Size: 6,
		MD5:  "bad",
	}); err != nil {
		t.Fatalf("write stale digest: %v", err)
	}

	if err := handler.downloadFile(file); err != nil {
		t.Fatalf("downloadFile failed: %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("expected one redownload request, count=%d", requestCount)
	}

	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read repaired file: %v", err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("unexpected repaired file content: %q", string(got))
	}
}
