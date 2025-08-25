package main

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// FormatHTML recursively prints formatted HTML with indentation
func FormatHTML(n *html.Node, depth int, indent string, prefix string) {
	if n == nil {
		return
	}

	spaces := strings.Repeat(indent, depth)

	switch n.Type {
	case html.DocumentNode:
		FormatHTML(n.FirstChild, depth, indent, prefix)

	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text != "" {
			fmt.Printf("%s%s%s\n", spaces, prefix, text)
		}
		FormatHTML(n.NextSibling, depth, indent, prefix)

	case html.ElementNode:
		// Open tag
		fmt.Printf("%s%s<%s", spaces, prefix, n.Data)
		for _, attr := range n.Attr {
			fmt.Printf(" %s=\"%s\"", attr.Key, attr.Val)
		}
		fmt.Println(">")

		// Children
		FormatHTML(n.FirstChild, depth+1, indent, prefix)

		// Close tag
		fmt.Printf("%s</%s>\n", spaces, n.Data)

		// Sibling
		FormatHTML(n.NextSibling, depth, indent, prefix)

	case html.CommentNode:
		fmt.Printf("%s<!-- %s -->\n", spaces, n.Data)
		FormatHTML(n.NextSibling, depth, indent, prefix)

	default:
		FormatHTML(n.NextSibling, depth, indent, prefix)
	}
}
