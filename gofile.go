package south2md

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var gofileURLPattern = regexp.MustCompile(`https?://(?:www\.)?gofile\.io/d/([A-Za-z0-9]+)`)

// GofileHandler manages gofile downloads via Go HTTP client.
type GofileHandler struct {
	toolPath      string
	venvDir       string
	downloadDir   string
	rootDir       string
	token         string
	maxConcurrent int
	maxRetries    int
	timeoutSec    int
	userAgent     string
	skipExisting  bool
	httpClient    *http.Client
}

type gofileAPIResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

type gofileAccountData struct {
	Token string `json:"token"`
}

type gofileContentData struct {
	ID             string                       `json:"id"`
	Type           string                       `json:"type"`
	Name           string                       `json:"name"`
	Link           string                       `json:"link"`
	Password       string                       `json:"password"`
	PasswordStatus string                       `json:"passwordStatus"`
	Children       map[string]gofileContentData `json:"children"`
}

type gofileRemoteFile struct {
	Path     string
	Filename string
	Link     string
}

// NewGofileHandler creates a new handler from config.
func NewGofileHandler(config *Config) *GofileHandler {
	if config == nil {
		return nil
	}
	timeout := time.Duration(int(config.HTTPTimeout.Seconds())) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &GofileHandler{
		toolPath:      config.GofileTool,
		venvDir:       config.GofileVenvDir,
		downloadDir:   config.GofileDir,
		rootDir:       ".",
		token:         config.GofileToken,
		maxConcurrent: config.HTTPMaxConcurrent,
		maxRetries:    max(1, config.HTTPMaxRetries),
		timeoutSec:    int(config.HTTPTimeout.Seconds()),
		userAgent:     config.HTTPUserAgent,
		skipExisting:  config.GofileSkipExisting,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetRootDir sets the write root for gofile downloads.
func (gh *GofileHandler) SetRootDir(rootDir string) {
	if gh == nil {
		return
	}
	if rootDir == "" {
		gh.rootDir = "."
		return
	}
	gh.rootDir = rootDir
}

// DownloadAndAnnotateGofileLinks downloads gofile links and annotates markdown with local paths.
func (gh *GofileHandler) DownloadAndAnnotateGofileLinks(tid string, markdown []byte, post *Post) ([]byte, error) {
	if gh == nil {
		return markdown, nil
	}

	urls := ExtractGofileLinks(string(markdown))
	if len(urls) == 0 {
		return markdown, nil
	}

	baseDir := filepath.Join(gh.rootDir, tid, gh.downloadDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return markdown, fmt.Errorf("failed to create gofile directory: %w", err)
	}

	if err := gh.downloadBatch(baseDir, urls); err != nil {
		slog.Warn("Gofile download failed", "error", err)
	}

	mapping := gh.collectLocalFiles(baseDir, urls, post)
	if len(mapping) == 0 {
		return markdown, nil
	}

	annotated := annotateGofileLinks(string(markdown), mapping)
	return []byte(annotated), nil
}

// ExtractGofileLinks finds gofile share links in markdown.
func ExtractGofileLinks(markdown string) []string {
	matches := gofileURLPattern.FindAllStringSubmatch(markdown, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) == 0 {
			continue
		}
		rawURL := m[0]
		if _, ok := seen[rawURL]; ok {
			continue
		}
		seen[rawURL] = struct{}{}
		urls = append(urls, rawURL)
	}
	sort.Strings(urls)
	return urls
}

func annotateGofileLinks(markdown string, mapping map[string]string) string {
	return gofileURLPattern.ReplaceAllStringFunc(markdown, func(rawURL string) string {
		local, ok := mapping[rawURL]
		if !ok || local == "" {
			return rawURL
		}
		return fmt.Sprintf("%s (local: %s)", rawURL, local)
	})
}

func (gh *GofileHandler) downloadBatch(baseDir string, urls []string) error {
	if gh.skipExisting && gh.allContentDirsPresent(baseDir, urls) {
		return nil
	}

	token, err := gh.ensureAccountToken()
	if err != nil {
		return err
	}

	var errs []error
	for _, rawURL := range urls {
		contentID := extractGofileContentID(rawURL)
		if contentID == "" {
			errs = append(errs, fmt.Errorf("invalid gofile url: %s", rawURL))
			continue
		}

		contentDir := filepath.Join(baseDir, contentID)
		if err := os.MkdirAll(contentDir, 0755); err != nil {
			errs = append(errs, fmt.Errorf("failed to create content dir for %s: %w", rawURL, err))
			continue
		}

		files, err := gh.buildContentTree(contentDir, contentID, token, "", map[string]int{})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to fetch content tree for %s: %w", rawURL, err))
			continue
		}

		for _, file := range files {
			if err := gh.downloadFile(file); err != nil {
				errs = append(errs, fmt.Errorf("download failed for %s: %w", file.Link, err))
			}
		}
	}

	return errors.Join(errs...)
}

func (gh *GofileHandler) ensureAccountToken() (string, error) {
	if strings.TrimSpace(gh.token) != "" {
		return gh.token, nil
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.gofile.io/accounts", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create account request: %w", err)
	}
	gh.applyBaseHeaders(req, "")

	resp, err := gh.doRequestWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("account request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("account request failed with status %d", resp.StatusCode)
	}

	var envelope gofileAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return "", fmt.Errorf("failed to parse account response: %w", err)
	}
	if envelope.Status != "ok" {
		return "", fmt.Errorf("account response status is %q", envelope.Status)
	}

	var data gofileAccountData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return "", fmt.Errorf("failed to parse account data: %w", err)
	}
	if strings.TrimSpace(data.Token) == "" {
		return "", fmt.Errorf("account token is empty")
	}

	gh.token = data.Token
	return gh.token, nil
}

func (gh *GofileHandler) buildContentTree(
	parentDir string,
	contentID string,
	token string,
	password string,
	pathingCount map[string]int,
) ([]gofileRemoteFile, error) {
	content, err := gh.fetchContent(contentID, token, password)
	if err != nil {
		return nil, err
	}

	if content.Password != "" && content.PasswordStatus != "" && content.PasswordStatus != "passwordOk" {
		return nil, fmt.Errorf("password protected content: %s", contentID)
	}

	if content.Type != "folder" {
		filePath := resolveNamingCollision(pathingCount, parentDir, content.Name, false)
		return []gofileRemoteFile{{
			Path:     filepath.Dir(filePath),
			Filename: filepath.Base(filePath),
			Link:     content.Link,
		}}, nil
	}

	absolutePath := resolveNamingCollision(pathingCount, parentDir, content.Name, true)
	if filepath.Base(parentDir) == contentID {
		absolutePath = parentDir
	}
	if err := os.MkdirAll(absolutePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create folder %s: %w", absolutePath, err)
	}

	var result []gofileRemoteFile
	keys := make([]string, 0, len(content.Children))
	for key := range content.Children {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		child := content.Children[key]
		if child.Type == "folder" {
			childFiles, err := gh.buildContentTree(absolutePath, child.ID, token, password, pathingCount)
			if err != nil {
				return nil, err
			}
			result = append(result, childFiles...)
			continue
		}

		filePath := resolveNamingCollision(pathingCount, absolutePath, child.Name, false)
		result = append(result, gofileRemoteFile{
			Path:     filepath.Dir(filePath),
			Filename: filepath.Base(filePath),
			Link:     child.Link,
		})
	}

	return result, nil
}

func (gh *GofileHandler) fetchContent(contentID, token, password string) (gofileContentData, error) {
	parsed, err := url.Parse(fmt.Sprintf("https://api.gofile.io/contents/%s", contentID))
	if err != nil {
		return gofileContentData{}, fmt.Errorf("failed to build content url: %w", err)
	}
	q := parsed.Query()
	q.Set("cache", "true")
	q.Set("sortField", "createTime")
	q.Set("sortDirection", "1")
	if password != "" {
		q.Set("password", hashPassword(password))
	}
	parsed.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, parsed.String(), nil)
	if err != nil {
		return gofileContentData{}, fmt.Errorf("failed to create content request: %w", err)
	}
	gh.applyBaseHeaders(req, token)
	req.Header.Set("X-Website-Token", "4fd6sg89d7s6")

	resp, err := gh.doRequestWithRetry(req)
	if err != nil {
		return gofileContentData{}, fmt.Errorf("content request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return gofileContentData{}, fmt.Errorf("content request failed with status %d", resp.StatusCode)
	}

	var envelope gofileAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return gofileContentData{}, fmt.Errorf("failed to parse content response: %w", err)
	}
	if envelope.Status != "ok" {
		return gofileContentData{}, fmt.Errorf("content response status is %q", envelope.Status)
	}

	var data gofileContentData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return gofileContentData{}, fmt.Errorf("failed to parse content data: %w", err)
	}
	return data, nil
}

func (gh *GofileHandler) downloadFile(file gofileRemoteFile) error {
	if file.Path == "" || file.Filename == "" || file.Link == "" {
		return fmt.Errorf("invalid file metadata")
	}

	if err := os.MkdirAll(file.Path, 0755); err != nil {
		return fmt.Errorf("failed to create file path: %w", err)
	}

	finalPath := filepath.Join(file.Path, file.Filename)
	if info, err := os.Stat(finalPath); err == nil && info.Size() > 0 {
		return nil
	}

	tmpPath := finalPath + ".part"
	var partSize int64
	if info, err := os.Stat(tmpPath); err == nil {
		partSize = info.Size()
	}

	for i := 0; i < max(1, gh.maxRetries); i++ {
		if err := gh.downloadFileAttempt(file.Link, tmpPath, finalPath, partSize); err == nil {
			return nil
		}
		if info, statErr := os.Stat(tmpPath); statErr == nil {
			partSize = info.Size()
		}
	}

	return fmt.Errorf("exceeded retry limit")
}

func (gh *GofileHandler) downloadFileAttempt(link, tmpPath, finalPath string, partSize int64) error {
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	if gh.userAgent != "" {
		req.Header.Set("User-Agent", gh.userAgent)
	}
	if partSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", partSize))
	}

	resp, err := gh.doRequestWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !isValidDownloadStatus(resp.StatusCode, partSize) {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	totalSize, err := extractFileSize(resp.Header, partSize)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to stat temp file: %w", err)
	}
	if info.Size() != totalSize {
		return fmt.Errorf("download incomplete: %d != %d", info.Size(), totalSize)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("failed to finalize file: %w", err)
	}
	return nil
}

func (gh *GofileHandler) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	attempts := max(1, gh.maxRetries)
	var lastErr error

	for i := 0; i < attempts; i++ {
		cloned := req.Clone(req.Context())
		resp, err := gh.httpClient.Do(cloned)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isRetryableNetError(err) {
			break
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown request error")
	}
	return nil, lastErr
}

func (gh *GofileHandler) applyBaseHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Accept-Encoding", "gzip")
	if gh.userAgent != "" {
		req.Header.Set("User-Agent", gh.userAgent)
	} else {
		req.Header.Set("User-Agent", "Mozilla/5.0")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Cookie", "accountToken="+token)
	}
}

func (gh *GofileHandler) allContentDirsPresent(baseDir string, urls []string) bool {
	for _, rawURL := range urls {
		contentID := extractGofileContentID(rawURL)
		if contentID == "" {
			return false
		}
		contentDir := filepath.Join(baseDir, contentID)
		if !dirHasFiles(contentDir) {
			return false
		}
	}
	return true
}

func (gh *GofileHandler) collectLocalFiles(baseDir string, urls []string, post *Post) map[string]string {
	if post == nil {
		return nil
	}

	mapping := make(map[string]string, len(urls))

	for _, rawURL := range urls {
		contentID := extractGofileContentID(rawURL)
		if contentID == "" {
			continue
		}

		contentDir := filepath.Join(baseDir, contentID)
		localFiles := listFilesRecursive(contentDir)
		relativeDir := filepath.ToSlash(filepath.Join(gh.downloadDir, contentID))
		record := GofileFile{
			URL:        rawURL,
			ContentID:  contentID,
			LocalDir:   relativeDir,
			LocalFiles: make([]string, 0, len(localFiles)),
			Downloaded: len(localFiles) > 0,
		}

		for _, file := range localFiles {
			rel, err := filepath.Rel(baseDir, file)
			if err != nil {
				continue
			}
			record.LocalFiles = append(record.LocalFiles, filepath.ToSlash(filepath.Join(gh.downloadDir, rel)))
		}

		if len(localFiles) == 0 {
			record.Error = "download_failed"
		}

		post.GofileFiles = upsertGofileRecord(post.GofileFiles, record)
		if record.Downloaded && record.LocalDir != "" {
			mapping[rawURL] = record.LocalDir
		}
	}

	return mapping
}

func upsertGofileRecord(records []GofileFile, record GofileFile) []GofileFile {
	for i := range records {
		if records[i].URL == record.URL {
			records[i] = record
			return records
		}
	}
	return append(records, record)
}

func extractGofileContentID(rawURL string) string {
	m := gofileURLPattern.FindStringSubmatch(rawURL)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func dirHasFiles(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if dirHasFiles(filepath.Join(dir, entry.Name())) {
				return true
			}
			continue
		}
		return true
	}
	return false
}

func listFilesRecursive(root string) []string {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}
	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files
}

func resolveNamingCollision(pathingCount map[string]int, parentDir, childName string, isDir bool) string {
	targetPath := filepath.Join(parentDir, childName)
	count, exists := pathingCount[targetPath]
	if !exists {
		pathingCount[targetPath] = 0
		return targetPath
	}

	count++
	pathingCount[targetPath] = count
	if isDir {
		return fmt.Sprintf("%s(%d)", targetPath, count)
	}

	ext := filepath.Ext(targetPath)
	base := strings.TrimSuffix(targetPath, ext)
	return fmt.Sprintf("%s(%d)%s", base, count, ext)
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func isRetryableNetError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func isValidDownloadStatus(statusCode int, partSize int64) bool {
	switch statusCode {
	case http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusInternalServerError:
		return false
	}
	if partSize == 0 {
		return statusCode == http.StatusOK || statusCode == http.StatusPartialContent
	}
	return statusCode == http.StatusPartialContent
}

func extractFileSize(header http.Header, partSize int64) (int64, error) {
	if partSize == 0 {
		contentLength := header.Get("Content-Length")
		if contentLength == "" {
			return 0, fmt.Errorf("missing Content-Length")
		}
		var size int64
		if _, err := fmt.Sscanf(contentLength, "%d", &size); err != nil {
			return 0, fmt.Errorf("invalid Content-Length: %w", err)
		}
		return size, nil
	}

	contentRange := header.Get("Content-Range")
	if contentRange == "" {
		return 0, fmt.Errorf("missing Content-Range")
	}

	parts := strings.Split(contentRange, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid Content-Range: %s", contentRange)
	}
	var size int64
	if _, err := fmt.Sscanf(parts[1], "%d", &size); err != nil {
		return 0, fmt.Errorf("invalid Content-Range total size: %w", err)
	}
	return size, nil
}
