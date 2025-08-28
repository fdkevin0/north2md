package main

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
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

// ImageHandler handles image downloading, caching and processing
type ImageHandler struct {
	cacheDir string
	mapping  map[string]string
}

// NewImageHandler creates a new image handler
func NewImageHandler(cacheDir string) *ImageHandler {
	return &ImageHandler{
		cacheDir: cacheDir,
		mapping:  make(map[string]string),
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

	// Step 2: Walk the AST to find and download images
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

				slog.Info("Downloading image", "url", imageURL)

				imageData, err := ih.downloadImage(imageURL)
				if err != nil {
					slog.Error("Failed to download image", "url", imageURL, "error", err)
					return ast.WalkContinue, nil
				}

				hash := md5.Sum(imageData)
				filename := fmt.Sprintf("%x%s", hash, filepath.Ext(imageURL))
				filePath := filepath.Join(tid, ih.cacheDir, filename)

				// Check if file already exists
				if _, err := os.Stat(filePath); err == nil {
					slog.Info("Image file already exists, skipping write", "path", filePath)
				} else {
					if err := os.WriteFile(filePath, imageData, 0644); err != nil {
						slog.Error("Failed to save image to cache", "path", filePath, "error", err)
						return ast.WalkContinue, nil
					}
				}

				slog.Info("Cached image successfully", "original_url", imageURL, "cached_path", filePath)
				ih.mapping[imageURL] = filename

				// Add to post images metadata
				alt := string(img.Title)
				image := Image{
					URL:        imageURL,
					Local:      filename,
					Alt:        alt,
					Downloaded: true,
					FileSize:   int64(len(imageData)),
				}
				post.Images = append(post.Images, image)
			}
		}
		return ast.WalkContinue, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error during AST walk: %w", err)
	}

	// Step 3: Walk the AST again to replace URLs with cached paths
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

	// Step 4: Convert the AST back to markdown
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
	resp, err := http.Get(imageURL)
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

// ExtractText recursively traverses the HTML nodes and extracts all plain text.
func (ih *ImageHandler) extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		// Replace multiple spaces with a single space and trim.
		text := strings.TrimSpace(n.Data)
		if text != "" {
			return text
		}
	}

	// Skip script and style tags to avoid including code.
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
		return ""
	}

	var builder strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childText := ih.extractText(c)
		if childText != "" {
			// Add a space between text from different elements.
			if builder.Len() > 0 && builder.String()[builder.Len()-1] != ' ' {
				builder.WriteString(" ")
			}
			builder.WriteString(childText)
		}
	}
	return builder.String()
}

// GetPlainTextFromHTML parses an HTML string and returns the pure text content.
func (ih *ImageHandler) GetPlainTextFromHTML(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}
	text := ih.extractText(doc)

	// Normalize spacing after extraction.
	text = strings.Join(strings.Fields(text), " ")

	text = strings.Trim(text, "\n")
	text = strings.Trim(text, " ")
	text = strings.Trim(text, "\n")
	return text, nil
}
