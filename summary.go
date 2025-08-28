package main

import (
	"strings"

	"golang.org/x/net/html"
)

// extractText recursively traverses the HTML nodes and extracts all plain text.
func extractText(n *html.Node) string {
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
		childText := extractText(c)
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
func GetPlainTextFromHTML(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}
	text := extractText(doc)

	// Normalize spacing after extraction.
	text = strings.Join(strings.Fields(text), " ")

	text = strings.Trim(text, "\n")
	text = strings.Trim(text, " ")
	text = strings.Trim(text, "\n")
	return text, nil
}
