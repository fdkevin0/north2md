package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// MarkdownGenerator Markdown生成器
type MarkdownGenerator struct {
	options *MarkdownOptions
}

// NewMarkdownGenerator 创建新的Markdown生成器
func NewMarkdownGenerator(options *MarkdownOptions) *MarkdownGenerator {
	return &MarkdownGenerator{
		options: options,
	}
}

// GenerateMarkdown 生成完整的Markdown文档
func (g *MarkdownGenerator) GenerateMarkdown(post *Post) (string, error) {
	var md strings.Builder

	// 文档标题
	md.WriteString(fmt.Sprintf("## %s\n\n", g.escapeMarkdown(post.Title)))

	// 归属信息
	md.WriteString("Made by north2md (c) fdkevin [GitHub Repo](https://github.com/fdkevin0/north2md)\n\n")

	// 热门回复
	if len(post.Replies) > 0 {
		g.writePopularReplies(&md, post)
	}

	md.WriteString("----\n\n")

	// 主楼内容
	g.writeMainPost(&md, post)

	// 回复内容
	if len(post.Replies) > 0 {
		for i, reply := range post.Replies {
			g.writeReplyPost(&md, reply, i+1)
		}
	}

	// 文档尾部信息
	g.writeFooter(&md, post)

	return md.String(), nil
}

// SavePost 保存帖子到指定目录结构
func (g *MarkdownGenerator) SavePost(post *Post, baseDir string) error {
	// 创建以TID命名的目录
	tidDir := filepath.Join(baseDir, post.TID)
	if err := os.MkdirAll(tidDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 创建images和attachments子目录
	imagesDir := filepath.Join(tidDir, "images")
	attachmentsDir := filepath.Join(tidDir, "attachments")

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("创建images目录失败: %v", err)
	}

	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		return fmt.Errorf("创建attachments目录失败: %v", err)
	}

	// 生成Markdown内容
	markdown, err := g.GenerateMarkdown(post)
	if err != nil {
		return fmt.Errorf("生成Markdown失败: %v", err)
	}

	// 保存post.md文件
	postFile := filepath.Join(tidDir, "post.md")
	if err := os.WriteFile(postFile, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("保存post.md失败: %v", err)
	}

	// 保存元数据
	metadata, err := toml.Marshal(post)
	if err != nil {
		return fmt.Errorf("生成元数据失败: %v", err)
	}

	metadataFile := filepath.Join(tidDir, "metadata.toml")
	if err := os.WriteFile(metadataFile, metadata, 0644); err != nil {
		return fmt.Errorf("保存metadata.toml失败: %v", err)
	}

	return nil
}

// writePopularReplies 写入热门回复部分
func (g *MarkdownGenerator) writePopularReplies(md *strings.Builder, post *Post) {
	md.WriteString("##### 热门回复\n\n")

	for i, reply := range post.Replies {
		if i >= 10 { // 只显示前10个热门回复
			break
		}

		// 生成楼层链接和文本
		floorText := fmt.Sprintf("%s楼", reply.Floor)

		// 提取回复内容的前50个字符作为预览
		preview := strings.TrimSpace(reply.Content)
		// 移除换行符，创建单行预览
		preview = strings.ReplaceAll(preview, "\n", " ")
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}

		fmt.Fprintf(md, "- [%s](#pid%s): %s\n", floorText, reply.PostID, g.escapeMarkdown(preview))
	}
	md.WriteString("\n")
}

// writeMainPost 写入主楼内容
func (g *MarkdownGenerator) writeMainPost(md *strings.Builder, post *Post) {
	// 主楼使用特殊的格式化方式
	g.writePostWithComplexHeader(md, post.MainPost, 0, "0")
	md.WriteString("\n")
}

// writeReplyPost 写入回复楼层内容
func (g *MarkdownGenerator) writeReplyPost(md *strings.Builder, reply PostEntry, index int) {
	g.writePostWithComplexHeader(md, reply, index, reply.Floor)
	md.WriteString("\n")
}

// writePostWithComplexHeader 使用复杂标题格式写入帖子
func (g *MarkdownGenerator) writePostWithComplexHeader(md *strings.Builder, entry PostEntry, index int, floor string) {
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

	// 写入内容
	if entry.Content != "" {
		content := g.formatContent(entry.Content)
		md.WriteString(content)
		md.WriteString("\n\n")
	}

	// 图片
	if g.options.IncludeImages && len(entry.Images) > 0 {
		images := g.FormatImages(entry.Images)
		if images != "" {
			md.WriteString("**图片**:\n\n")
			md.WriteString(images)
			md.WriteString("\n")
		}
	}

	// 附件
	if len(entry.Attachments) > 0 {
		attachments := g.FormatAttachments(entry.Attachments)
		if attachments != "" {
			md.WriteString("**附件**:\n\n")
			md.WriteString(attachments)
			md.WriteString("\n")
		}
	}
}

// FormatImages 格式化图片列表
func (g *MarkdownGenerator) FormatImages(images []Image) string {
	if len(images) == 0 {
		return ""
	}

	var md strings.Builder

	for i, img := range images {
		if g.options.ImageStyle == "reference" {
			// 引用式图片
			md.WriteString(fmt.Sprintf("![image%d][img%d]\n\n", i+1, i+1))
		} else {
			// 内联式图片
			imgPath := img.URL

			// 如果有本地路径，优先使用本地路径
			if img.LocalPath != "" {
				// 转换为相对路径
				imgPath = g.convertToRelativePath(img.LocalPath)
			}

			alt := fmt.Sprintf("image%d", i+1)
			if img.Alt != "" {
				alt = g.escapeMarkdown(img.Alt)
			}

			md.WriteString(fmt.Sprintf("![%s](%s)", alt, imgPath))

			// 添加图片信息
			if img.FileSize > 0 {
				md.WriteString(fmt.Sprintf(" *(%s)*", g.formatFileSize(img.FileSize)))
			}

			if img.IsAttachment {
				md.WriteString(" *(附件)*")
			}

			md.WriteString("\n\n")
		}
	}

	// 如果使用引用式，添加引用定义
	if g.options.ImageStyle == "reference" {
		md.WriteString("\n")
		for i, img := range images {
			imgPath := img.URL
			if img.LocalPath != "" {
				imgPath = g.convertToRelativePath(img.LocalPath)
			}
			md.WriteString(fmt.Sprintf("[img%d]: %s\n", i+1, imgPath))
		}
	}

	return md.String()
}

// FormatAttachments 格式化附件列表
func (g *MarkdownGenerator) FormatAttachments(attachments []Attachment) string {
	if len(attachments) == 0 {
		return ""
	}

	var md strings.Builder

	for _, att := range attachments {
		attachPath := att.URL

		// 如果有本地路径，优先使用本地路径
		if att.LocalPath != "" {
			attachPath = g.convertToRelativePath(att.LocalPath)
		}

		fileName := att.FileName
		if fileName == "" {
			fileName = "attachment"
		}

		md.WriteString(fmt.Sprintf("- [%s](%s)", g.escapeMarkdown(fileName), attachPath))

		// 添加文件信息
		var info []string

		if att.FileSize > 0 {
			info = append(info, g.formatFileSize(att.FileSize))
		}

		if att.MimeType != "" {
			info = append(info, att.MimeType)
		}

		if len(info) > 0 {
			md.WriteString(fmt.Sprintf(" *(%s)*", strings.Join(info, ", ")))
		}

		if att.Downloaded {
			md.WriteString(" ✓")
		}

		md.WriteString("\n")
	}

	return md.String()
}

// writeFooter 写入文档尾部信息
func (g *MarkdownGenerator) writeFooter(md *strings.Builder, post *Post) {
	md.WriteString("---\n\n")
	md.WriteString("*本文档由 ngapost2md 自动生成*\n\n")
	fmt.Fprintf(md, "*生成时间: %s*\n", time.Now().Format("2006-01-02 15:04:05"))

	// 统计信息
	totalImages := len(post.MainPost.Images)
	totalAttachments := len(post.MainPost.Attachments)

	for _, reply := range post.Replies {
		totalImages += len(reply.Images)
		totalAttachments += len(reply.Attachments)
	}

	if totalImages > 0 || totalAttachments > 0 {
		md.WriteString("\n**统计信息**:\n\n")
		if totalImages > 0 {
			fmt.Fprintf(md, "- 图片数量: %d\n", totalImages)
		}
		if totalAttachments > 0 {
			fmt.Fprintf(md, "- 附件数量: %d\n", totalAttachments)
		}
	}
}

// 辅助方法

// formatContent 格式化帖子内容
func (g *MarkdownGenerator) formatContent(content string) string {
	// 移除多余的空白行
	content = strings.TrimSpace(content)

	// 将内容按段落分割并重新组织
	paragraphs := strings.Split(content, "\n")
	var formattedParagraphs []string

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			formattedParagraphs = append(formattedParagraphs, paragraph)
		}
	}

	return strings.Join(formattedParagraphs, "\n\n")
}

// escapeMarkdown 转义Markdown特殊字符
func (g *MarkdownGenerator) escapeMarkdown(text string) string {
	// 转义Markdown特殊字符
	replacements := map[string]string{
		"\\": "\\\\",
		"`":  "\\`",
		"*":  "\\*",
		"_":  "\\_",
		"{":  "\\{",
		"}":  "\\}",
		"[":  "\\[",
		"]":  "\\]",
		"(":  "\\(",
		")":  "\\)",
		"#":  "\\#",
		"+":  "\\+",
		"-":  "\\-",
		".":  "\\.",
		"!":  "\\!",
		"|":  "\\|",
	}

	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	return text
}

// convertToRelativePath 将绝对路径转换为相对路径
func (g *MarkdownGenerator) convertToRelativePath(absolutePath string) string {
	// 简单地使用文件名，或者根据需要实现更复杂的相对路径逻辑
	return fmt.Sprintf("images/%s", filepath.Base(absolutePath))
}

// formatFileSize 格式化文件大小
func (g *MarkdownGenerator) formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}