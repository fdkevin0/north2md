package south2md

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var gofileURLPattern = regexp.MustCompile(`https?://(?:www\.)?gofile\.io/d/([A-Za-z0-9]+)`)

// GofileHandler manages gofile downloads via external python tool.
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
}

// NewGofileHandler creates a new handler from config.
func NewGofileHandler(config *Config) *GofileHandler {
	if config == nil {
		return nil
	}
	return &GofileHandler{
		toolPath:      config.GofileTool,
		venvDir:       config.GofileVenvDir,
		downloadDir:   config.GofileDir,
		rootDir:       ".",
		token:         config.GofileToken,
		maxConcurrent: config.HTTPMaxConcurrent,
		maxRetries:    config.HTTPMaxRetries,
		timeoutSec:    int(config.HTTPTimeout.Seconds()),
		userAgent:     config.HTTPUserAgent,
		skipExisting:  config.GofileSkipExisting,
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
	if gh == nil || gh.toolPath == "" {
		return markdown, nil
	}

	urls := ExtractGofileLinks(string(markdown))
	if len(urls) == 0 {
		return markdown, nil
	}

	if err := gh.ensureEnvironment(); err != nil {
		return markdown, err
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
		url := m[0]
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		urls = append(urls, url)
	}
	sort.Strings(urls)
	return urls
}

func annotateGofileLinks(markdown string, mapping map[string]string) string {
	return gofileURLPattern.ReplaceAllStringFunc(markdown, func(url string) string {
		local, ok := mapping[url]
		if !ok || local == "" {
			return url
		}
		return fmt.Sprintf("%s (local: %s)", url, local)
	})
}

func (gh *GofileHandler) ensureEnvironment() error {
	if _, err := os.Stat(gh.toolPath); err != nil {
		return fmt.Errorf("gofile tool not found: %w", err)
	}
	python, err := gh.resolvePython()
	if err != nil {
		return err
	}

	if err := gh.ensureVenv(python); err != nil {
		return err
	}

	return gh.ensureRequirements()
}

func (gh *GofileHandler) resolvePython() (string, error) {
	if gh.venvDir != "" {
		venvPython := filepath.Join(gh.venvDir, "bin", "python")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython, nil
		}
	}

	python, err := exec.LookPath("python3")
	if err == nil {
		return python, nil
	}

	python, err = exec.LookPath("python")
	if err == nil {
		return python, nil
	}

	return "", fmt.Errorf("python not found in PATH")
}

func (gh *GofileHandler) ensureVenv(systemPython string) error {
	if gh.venvDir == "" {
		return fmt.Errorf("gofile venv dir is empty")
	}
	venvPython := filepath.Join(gh.venvDir, "bin", "python")
	if _, err := os.Stat(venvPython); err == nil {
		return nil
	}

	if err := os.MkdirAll(gh.venvDir, 0755); err != nil {
		return fmt.Errorf("failed to create venv dir: %w", err)
	}

	cmd := exec.Command(systemPython, "-m", "venv", gh.venvDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create venv: %w", err)
	}

	return nil
}

func (gh *GofileHandler) ensureRequirements() error {
	toolDir := filepath.Dir(gh.toolPath)
	reqFile := filepath.Join(toolDir, "requirements.txt")
	if _, err := os.Stat(reqFile); err != nil {
		return fmt.Errorf("requirements.txt not found: %w", err)
	}

	reqHash, err := fileSHA256(reqFile)
	if err != nil {
		return err
	}

	marker := filepath.Join(gh.venvDir, ".requirements.sha256")
	if existing, err := os.ReadFile(marker); err == nil {
		if strings.TrimSpace(string(existing)) == reqHash {
			return nil
		}
	}

	venvPython := filepath.Join(gh.venvDir, "bin", "python")
	cmd := exec.Command(venvPython, "-m", "pip", "install", "-r", reqFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install requirements: %w", err)
	}

	if err := os.WriteFile(marker, []byte(reqHash), 0644); err != nil {
		return fmt.Errorf("failed to write requirements marker: %w", err)
	}

	return nil
}

func (gh *GofileHandler) downloadBatch(baseDir string, urls []string) error {
	if gh.skipExisting && gh.allContentDirsPresent(baseDir, urls) {
		return nil
	}

	tmpFile, err := os.CreateTemp(baseDir, "gofile-urls-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create url list: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(strings.Join(urls, "\n")); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write url list: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close url list: %w", err)
	}

	venvPython := filepath.Join(gh.venvDir, "bin", "python")
	cmd := exec.Command(venvPython, gh.toolPath, tmpFile.Name())
	cmd.Dir = filepath.Dir(gh.toolPath)
	cmd.Env = append(os.Environ(),
		"GF_DOWNLOAD_DIR="+baseDir,
		"GF_MAX_CONCURRENT_DOWNLOADS="+strconv.Itoa(gh.maxConcurrent),
		"GF_MAX_RETRIES="+strconv.Itoa(gh.maxRetries),
		"GF_TIMEOUT="+strconv.Itoa(gh.timeoutSec),
	)
	if gh.userAgent != "" {
		cmd.Env = append(cmd.Env, "GF_USERAGENT="+gh.userAgent)
	}
	if gh.token != "" {
		cmd.Env = append(cmd.Env, "GF_TOKEN="+gh.token)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gofile downloader failed: %w", err)
	}

	return nil
}

func (gh *GofileHandler) allContentDirsPresent(baseDir string, urls []string) bool {
	for _, url := range urls {
		contentID := extractGofileContentID(url)
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

	for _, url := range urls {
		contentID := extractGofileContentID(url)
		if contentID == "" {
			continue
		}

		contentDir := filepath.Join(baseDir, contentID)
		localFiles := listFilesRecursive(contentDir)
		relativeDir := filepath.ToSlash(filepath.Join(gh.downloadDir, contentID))
		record := GofileFile{
			URL:        url,
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
			mapping[url] = record.LocalDir
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

func extractGofileContentID(url string) string {
	m := gofileURLPattern.FindStringSubmatch(url)
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

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hash: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
