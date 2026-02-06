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
	"runtime"
	"strings"
	"sync"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// ImageHandler handles image downloading, caching and processing
type ImageHandler struct {
	cacheDir   string
	mapping    map[string]string
	httpClient *http.Client
}

// NewImageHandler creates a new image handler
func NewImageHandler(cacheDir string) *ImageHandler {
	return &ImageHandler{
		cacheDir: cacheDir,
		mapping:  make(map[string]string),
		httpClient: &http.Client{
			Timeout: 0, // No timeout for downloads
		},
	}
}

// DownloadTask represents an image download task
type DownloadTask struct {
	URL  string
	Post *Post
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

// DownloadAndCacheImages parses a markdown document, downloads all images,
// and saves them to a cache directory named by their MD5 hash.
func (ih *ImageHandler) DownloadAndCacheImages(tid string, mdDoc []byte, post *Post) ([]byte, error) {
	// Build map of existing images for quick lookup
	existingImages := make(map[string]*Image)
	for i := range post.Images {
		existingImages[post.Images[i].URL] = &post.Images[i]
	}

	// Create a Goldmark instance for parsing
	md := goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID()))

	// Step 1: Parse the document
	doc := md.Parser().Parse(text.NewReader(mdDoc))

	// Step 2: Walk the AST to collect image URLs for concurrent download
	// Pre-allocate slices with estimated capacity to reduce allocations
	estimatedImageCount := bytes.Count(mdDoc, []byte("![")) // Rough estimate based on markdown image syntax
	if estimatedImageCount == 0 {
		estimatedImageCount = 10 // Default capacity
	}

	imageNodes := make([]*ast.Image, 0, estimatedImageCount)
	imageURLs := make([]string, 0, estimatedImageCount)

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if n.Kind() == ast.KindImage {
			img := n.(*ast.Image)
			imageURL := string(img.Destination)

			if ih.isRemoteURL(imageURL) {
				if _, ok := ih.mapping[imageURL]; ok {
					return ast.WalkContinue, nil
				}

				// Check if image already cached in metadata
				if existing, ok := existingImages[imageURL]; ok && existing.Downloaded {
					ih.mapping[imageURL] = existing.Local
					slog.Info("Reusing cached image", "url", imageURL, "path", existing.Local)
					return ast.WalkContinue, nil
				}

				imageURLs = append(imageURLs, imageURL)
				imageNodes = append(imageNodes, img)
			}
		}
		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during AST walk: %w", err)
	}

	// No images to download
	if len(imageURLs) == 0 {
		return ih.convertASTToMarkdown(md, mdDoc, doc)
	}

	// Step 3: Download images concurrently
	ih.downloadImagesConcurrently(tid, imageURLs, post)

	// Step 4: Walk the AST again to replace URLs with cached paths
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if n.Kind() == ast.KindImage {
			img := n.(*ast.Image)
			originalURL := string(img.Destination)

			if ih.isRemoteURL(originalURL) {
				if cachedFile, ok := ih.mapping[originalURL]; ok {
					// Replace the destination with the new, local path.
					// Path is relative to the markdown file location (tid/post.md -> tid/images/filename)
					newPath := filepath.Join(ih.cacheDir, cachedFile)
					img.Destination = []byte(newPath)
					slog.Info("Updated image path", "original_url", originalURL, "new_path", newPath)
				}
			}
		}
		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during URL replacement: %w", err)
	}

	// Step 5: Convert the AST back to markdown
	return ih.convertASTToMarkdown(md, mdDoc, doc)
}

// downloadImagesConcurrently downloads multiple images using a worker pool
func (ih *ImageHandler) downloadImagesConcurrently(tid string, imageURLs []string, post *Post) map[string]string {
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
		for _, url := range imageURLs {
			tasks <- DownloadTask{URL: url, Post: post}
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

		ih.processDownloadedImage(tid, result.URL, result.ImageData, post)
	}

	return ih.mapping
}

// processDownloadedImage processes a downloaded image and updates the mapping
func (ih *ImageHandler) processDownloadedImage(tid, url string, imageData []byte, post *Post) {
	hash := md5.Sum(imageData)
	filename := fmt.Sprintf("%x%s", hash, filepath.Ext(url))
	filePath := filepath.Join(tid, ih.cacheDir, filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		slog.Info("Image file already exists, skipping write", "path", filePath)
	} else {
		if err := os.WriteFile(filePath, imageData, 0644); err != nil {
			slog.Error("Failed to save image to cache", "path", filePath, "error", err)
			return
		}
	}

	slog.Info("Cached image successfully", "original_url", url, "cached_path", filePath)
	ih.mapping[url] = filename

	// Add to post images metadata
	image := Image{
		URL:        url,
		Local:      filename,
		Alt:        "", // Will be populated during AST walk
		Downloaded: true,
		FileSize:   int64(len(imageData)),
	}
	post.Images = append(post.Images, image)
}

// convertASTToMarkdown converts AST back to markdown format
func (ih *ImageHandler) convertASTToMarkdown(md goldmark.Markdown, mdDoc []byte, doc ast.Node) ([]byte, error) {
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, mdDoc, doc); err != nil {
		return nil, fmt.Errorf("failed to render markdown: %w", err)
	}

	// The renderer produces HTML, but we want markdown
	// We'll use the html-to-markdown converter that's already used in generator.go
	markdown, err := htmltomarkdown.ConvertString(buf.String())
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML back to markdown: %w", err)
	}

	return []byte(markdown), nil
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
