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

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// imageCache stores the mapping from original URL to cached filename.
// This is used by both the downloader and the transformer.
type imageCache struct {
	mapping  map[string]string
	cacheDir string
}

// urlRewriter is a goldmark transformer that updates image URLs.
type urlRewriter struct {
	cache *imageCache
}

// Transform modifies the AST to update image destinations.
func (t *urlRewriter) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if n.Kind() == ast.KindImage {
			img := n.(*ast.Image)
			originalURL := string(img.Destination)

			if isRemoteURL(originalURL) {
				if cachedFile, ok := t.cache.mapping[originalURL]; ok {
					// Replace the destination with the new, local path.
					// Ensure the path is relative to where the new markdown file will be.
					newPath := filepath.Join(t.cache.cacheDir, cachedFile)
					img.Destination = []byte(newPath)
					log.Printf("Updated image path for %s to %s", originalURL, newPath)
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

// downloadAndCacheImages parses a markdown document, downloads all images,
// and saves them to a cache directory named by their MD5 hash.
func downloadAndCacheImages(tid string, mdDoc []byte, cacheDir string) ([]byte, error) {
	// Create the cache directory if it doesn't exist.
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &imageCache{
		mapping:  make(map[string]string),
		cacheDir: cacheDir,
	}

	// Create a single Goldmark instance with all configurations.
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.PrioritizedValue{
				Value:    &urlRewriter{cache: cache},
				Priority: 200,
			}),
		),
	)

	// Step 1: Parse the document and download images.
	// We do this manually to populate the cache before the transformer runs.
	doc := md.Parser().Parse(text.NewReader(mdDoc))
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

	// Step 2: Render the updated document. The urlRewriter will be triggered here.
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, mdDoc, doc); err != nil {
		return nil, fmt.Errorf("failed to render updated markdown: %w", err)
	}

	return buf.Bytes(), nil
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
