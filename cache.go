package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log"
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
)

// imageCache stores the mapping from original URL to cached filename.
// This is used by both the downloader and the transformer.
type imageCache struct {
	mapping  map[string]string
	cacheDir string
}

// downloadAndCacheImages parses a markdown document, downloads all images,
// and saves them to a cache directory named by their MD5 hash.
func downloadAndCacheImages(tid string, mdDoc []byte, cacheDir string) ([]byte, error) {
	cache := &imageCache{
		mapping:  make(map[string]string),
		cacheDir: cacheDir,
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

			if isRemoteURL(imageURL) {
				log.Printf("Downloading image from: %s", imageURL)

				if _, ok := cache.mapping[imageURL]; ok {
					return ast.WalkContinue, nil
				}

				imageData, err := downloadImage(imageURL)
				if err != nil {
					log.Printf("error downloading image %s: %v", imageURL, err)
					return ast.WalkContinue, nil
				}

				hash := md5.Sum(imageData)
				filename := fmt.Sprintf("%x%s", hash, filepath.Ext(imageURL))
				filePath := filepath.Join(tid, cache.cacheDir, filename)

				if err := os.WriteFile(filePath, imageData, 0644); err != nil {
					log.Printf("error saving image to cache %s: %v", filePath, err)
					return ast.WalkContinue, nil
				}
				log.Printf("Cached image %s as %s", imageURL, filePath)
				cache.mapping[imageURL] = filename
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

			if isRemoteURL(originalURL) {
				if cachedFile, ok := cache.mapping[originalURL]; ok {
					// Replace the destination with the new, local path.
					// Path is relative to the markdown file location (tid/post.md -> tid/images/filename)
					newPath := filepath.Join(cache.cacheDir, cachedFile)
					img.Destination = []byte(newPath)
					log.Printf("Updated image path for %s to %s", originalURL, newPath)
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
func downloadImage(imageURL string) ([]byte, error) {
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
func isRemoteURL(imageURL string) bool {
	u, err := url.Parse(imageURL)
	if err != nil || !u.IsAbs() || !strings.HasPrefix(u.Scheme, "http") {
		return false
	}
	return true
}
