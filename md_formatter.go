package main

import (
	"fmt"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
)

// MarkdownFormatter handles markdown formatting operations
type MarkdownFormatter struct {
	options *MarkdownOptions
}

// NewMarkdownFormatter creates a new markdown formatter
func NewMarkdownFormatter(options *MarkdownOptions) *MarkdownFormatter {
	return &MarkdownFormatter{
		options: options,
	}
}

// FormatTitle formats the document title
func (mf *MarkdownFormatter) FormatTitle(title string) string {
	return fmt.Sprintf("## %s\n\n", mf.escapeMarkdown(title))
}

// FormatAttribution formats the attribution information
func (mf *MarkdownFormatter) FormatAttribution() string {
	return "Made by north2md (c) fdkevin [GitHub Repo](https://github.com/fdkevin0/north2md)\n\n"
}

// FormatPopularReplies formats the popular replies section
func (mf *MarkdownFormatter) FormatPopularReplies(post *Post, imageHandler *ImageHandler) string {
	if len(post.Replies) == 0 {
		return ""
	}

	var md strings.Builder
	md.WriteString("##### 热门回复\n\n")

	for i, reply := range post.Replies {
		if i >= 10 { // 只显示前10个热门回复
			break
		}

		// 生成楼层链接和文本
		floorText := fmt.Sprintf("%s楼", reply.Floor)

		// 提取回复内容的前50个字符作为预览
		preview, err := imageHandler.GetPlainTextFromHTML(reply.HTMLContent)
		if err != nil {
			// If there's an error, just use empty string
			preview = ""
		}
		// 移除换行符，创建单行预览
		preview = ReplaceNewlines(preview, " ")
		preview = TruncateText(preview, 50)

		fmt.Fprintf(&md, "- [%s](#pid%s): %s\n", floorText, reply.PostID, EscapeMarkdown(preview))
	}
	md.WriteString("\n")

	return md.String()
}

// FormatPostEntry formats a single post entry with complex header
func (mf *MarkdownFormatter) FormatPostEntry(tid string, entry PostEntry, index int, floor string, post *Post, imageHandler *ImageHandler) (string, error) {
	var md strings.Builder

	// 复杂标题格式
	floorDisplay := floor
	if floor == "0" {
		floorDisplay = "0"
	}

	// 构建复杂的span标题
	header := fmt.Sprintf("##### <span id=\"pid%s\">%s.[%d] \\<pid:%s\\> %s by UID:%s(%s)</span>",
		entry.PostID,
		floorDisplay,
		index,
		entry.PostID,
		entry.PostTime.Format("2006-01-02 15:04:05"),
		entry.Author.UID,
		entry.Author.Username)

	md.WriteString(header)
	md.WriteString("\n\n")

	if entry.HTMLContent != "" {
		markdown, err := htmltomarkdown.ConvertString(entry.HTMLContent,
			converter.WithDomain("https://north-plus.net/"),
		)
		if err != nil {
			return "", fmt.Errorf("failed to convert HTML to markdown: %w", err)
		}

		md2, err := imageHandler.DownloadAndCacheImages(tid, []byte(markdown), post)
		if err != nil {
			return "", fmt.Errorf("failed to download and cache images: %w", err)
		}

		md.WriteString(string(md2))
		md.WriteString("\n\n")
	}

	return md.String(), nil
}

// FormatFooter formats the document footer
func (mf *MarkdownFormatter) FormatFooter() string {
	var md strings.Builder
	md.WriteString("---\n\n")
	md.WriteString("*本文档由 ngapost2md 自动生成*\n\n")
	fmt.Fprintf(&md, "*生成时间: %s*\n", time.Now().Format("2006-01-02 15:04:05"))
	return md.String()
}

// escapeMarkdown 转义Markdown特殊字符 (废弃的本地实现，使用共享的EscapeMarkdown)
// 保留这个方法以避免破坏现有代码，但内部调用共享实现
func (mf *MarkdownFormatter) escapeMarkdown(text string) string {
	return EscapeMarkdown(text)
}
