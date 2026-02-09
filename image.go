package south2md

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var imageLinkPattern = regexp.MustCompile(`!\[[^\]]*\]\(\s*(<)?([^)\s>]+)(>)?([^)]*)\)`)

// ImageHandler handles image downloading, caching and processing
type ImageHandler struct {
	cacheDir   string
	rootDir    string
	download   bool
	httpClient *http.Client
}

// NewImageHandler creates a new image handler
func NewImageHandler(cacheDir string) *ImageHandler {
	return &ImageHandler{
		cacheDir: cacheDir,
		rootDir:  ".",
		download: true,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for downloads
		},
	}
}

// SetRootDir sets the write root for cached image files.
func (ih *ImageHandler) SetRootDir(rootDir string) {
	if ih == nil {
		return
	}
	if rootDir == "" {
		ih.rootDir = "."
		return
	}
	ih.rootDir = rootDir
}

// SetDownloadEnabled controls whether missing remote images are downloaded.
func (ih *ImageHandler) SetDownloadEnabled(enabled bool) {
	if ih == nil {
		return
	}
	ih.download = enabled
}

// DownloadTask represents an image download task
type DownloadTask struct {
	URL string
}

// DownloadResult represents the result of an image download
type DownloadResult struct {
	URL       string
	ImageData []byte
	Error     error
}

func (ih *ImageHandler) downloadWorker(tasks <-chan DownloadTask, results chan<- DownloadResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range tasks {
		imageData, err := ih.downloadImage(task.URL)
		results <- DownloadResult{
			URL:       task.URL,
			ImageData: imageData,
			Error:     err,
		}
	}
}

// DownloadAndCacheImages replaces remote markdown image URLs with cached paths.
func (ih *ImageHandler) DownloadAndCacheImages(tid string, mdDoc []byte, post *Post) ([]byte, error) {
	mapping := make(map[string]string)
	existingImages := make(map[string]string)
	if post != nil {
		for i := range post.Images {
			if !post.Images[i].Downloaded || post.Images[i].URL == "" || post.Images[i].Local == "" {
				continue
			}
			existingImages[post.Images[i].URL] = post.Images[i].Local
		}
	}

	imageURLs := ih.extractRemoteImageURLs(mdDoc)
	if len(imageURLs) == 0 {
		return mdDoc, nil
	}

	pending := make([]string, 0, len(imageURLs))
	for _, imageURL := range imageURLs {
		if local, ok := existingImages[imageURL]; ok {
			mapping[imageURL] = local
			slog.Info("Reusing cached image", "url", imageURL, "path", local)
			continue
		}
		pending = append(pending, imageURL)
	}

	if ih.download && len(pending) > 0 {
		ih.downloadImagesConcurrently(tid, pending, post, mapping)
	}

	return ih.replaceImageURLs(mdDoc, mapping), nil
}

// downloadImagesConcurrently downloads multiple images using a worker pool
func (ih *ImageHandler) downloadImagesConcurrently(tid string, imageURLs []string, post *Post, mapping map[string]string) {
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Cap at 8 workers to avoid overwhelming the server
	}

	tasks := make(chan DownloadTask, len(imageURLs))
	results := make(chan DownloadResult, len(imageURLs))
	var wg sync.WaitGroup

	// Start worker pool
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go ih.downloadWorker(tasks, results, &wg)
	}

	// Send tasks to workers
	go func() {
		for _, rawURL := range imageURLs {
			tasks <- DownloadTask{URL: rawURL}
		}
		close(tasks)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	for result := range results {
		if result.Error != nil {
			slog.Error("Failed to download image", "url", result.URL, "error", result.Error)
			continue
		}

		ih.processDownloadedImage(tid, result.URL, result.ImageData, post, mapping)
	}
}

// processDownloadedImage processes a downloaded image and updates the mapping
func (ih *ImageHandler) processDownloadedImage(tid, rawURL string, imageData []byte, post *Post, mapping map[string]string) {
	hash := md5.Sum(imageData)
	filename := fmt.Sprintf("%x%s", hash, filepath.Ext(rawURL))
	filePath := filepath.Join(ih.rootDir, tid, ih.cacheDir, filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		slog.Info("Image file already exists, skipping write", "path", filePath)
	} else {
		if err := os.WriteFile(filePath, imageData, 0644); err != nil {
			slog.Error("Failed to save image to cache", "path", filePath, "error", err)
			return
		}
	}

	slog.Info("Cached image successfully", "original_url", rawURL, "cached_path", filePath)
	mapping[rawURL] = filename

	if post != nil {
		image := Image{
			URL:        rawURL,
			Local:      filename,
			Alt:        "",
			Downloaded: true,
			FileSize:   int64(len(imageData)),
		}
		post.Images = append(post.Images, image)
	}
}

func (ih *ImageHandler) extractRemoteImageURLs(mdDoc []byte) []string {
	matches := imageLinkPattern.FindAllSubmatchIndex(mdDoc, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 6 || match[4] < 0 || match[5] < 0 {
			continue
		}
		imageURL := string(mdDoc[match[4]:match[5]])
		if !ih.isRemoteURL(imageURL) {
			continue
		}
		if _, ok := seen[imageURL]; ok {
			continue
		}
		seen[imageURL] = struct{}{}
		urls = append(urls, imageURL)
	}

	return urls
}

func (ih *ImageHandler) replaceImageURLs(mdDoc []byte, mapping map[string]string) []byte {
	if len(mapping) == 0 {
		return mdDoc
	}

	matches := imageLinkPattern.FindAllSubmatchIndex(mdDoc, -1)
	if len(matches) == 0 {
		return mdDoc
	}

	var out bytes.Buffer
	out.Grow(len(mdDoc) + len(matches)*8)

	last := 0
	for _, match := range matches {
		if len(match) < 6 || match[4] < 0 || match[5] < 0 {
			continue
		}

		start := match[0]
		end := match[1]
		urlStart := match[4]
		urlEnd := match[5]

		if start < last {
			continue
		}
		out.Write(mdDoc[last:start])

		originalURL := string(mdDoc[urlStart:urlEnd])
		cachedFile, ok := mapping[originalURL]
		if !ok {
			out.Write(mdDoc[start:end])
			last = end
			continue
		}

		newPath := strings.ReplaceAll(filepath.Join(ih.cacheDir, cachedFile), "\\", "/")
		out.Write(mdDoc[start:urlStart])
		out.WriteString(newPath)
		out.Write(mdDoc[urlEnd:end])
		last = end

		slog.Info("Updated image path", "original_url", originalURL, "new_path", newPath)
	}

	out.Write(mdDoc[last:])
	return out.Bytes()
}

// downloadImage fetches image data from a URL.
func (ih *ImageHandler) downloadImage(imageURL string) ([]byte, error) {
	resp, err := ih.httpClient.Get(imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %s", resp.Status)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return imageData, nil
}

// isRemoteURL checks if a URL is an absolute remote URL.
func (ih *ImageHandler) isRemoteURL(imageURL string) bool {
	u, err := url.Parse(imageURL)
	if err != nil || !u.IsAbs() || !strings.HasPrefix(u.Scheme, "http") {
		return false
	}
	return true
}
