package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// MarkdownGenerator Markdown生成器接口
type MarkdownGenerator interface {
	GenerateMarkdown(post *Post) (string, error)
	FormatPost(entry *PostEntry) string
	FormatAuthor(author *Author) string
	FormatImages(images []Image) string
	FormatAttachments(attachments []Attachment) string
	GenerateTableOfContents(post *Post) string
}

// DefaultMarkdownGenerator 默认Markdown生成器实现
type DefaultMarkdownGenerator struct {
	options *MarkdownOptions
}

// NewMarkdownGenerator 创建新的Markdown生成器
func NewMarkdownGenerator(options *MarkdownOptions) *DefaultMarkdownGenerator {
	return &DefaultMarkdownGenerator{
		options: options,
	}
}

// GenerateMarkdown 生成完整的Markdown文档
func (g *DefaultMarkdownGenerator) GenerateMarkdown(post *Post) (string, error) {
	var md strings.Builder

	// 文档标题
	md.WriteString(fmt.Sprintf("# %s\n\n", g.escapeMarkdown(post.Title)))

	// 帖子基本信息
	g.writePostInfo(&md, post)

	// 目录 (如果启用)
	if g.options.TableOfContents || g.options.IncludeTOC {
		toc := g.GenerateTableOfContents(post)
		if toc != "" {
			md.WriteString("## 目录\n\n")
			md.WriteString(toc)
			md.WriteString("\n")
		}
	}

	md.WriteString("---\n\n")

	// 主楼内容
	md.WriteString("## 主楼 (GF)\n\n")
	md.WriteString(g.FormatPost(&post.MainPost))
	md.WriteString("\n")

	// 回复内容
	if len(post.Replies) > 0 {
		md.WriteString("## 回复\n\n")
		for i, reply := range post.Replies {
			if g.options.FloorNumbering {
				md.WriteString(fmt.Sprintf("### %s楼\n\n", reply.Floor))
			} else {
				md.WriteString(fmt.Sprintf("### 回复 %d\n\n", i+1))
			}
			md.WriteString(g.FormatPost(&reply))
			md.WriteString("\n")
		}
	}

	// 文档尾部信息
	g.writeFooter(&md, post)

	return md.String(), nil
}

// SavePost 保存帖子到指定目录结构
func (g *DefaultMarkdownGenerator) SavePost(post *Post, baseDir string) error {
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

// writePostInfo 写入帖子基本信息
func (g *DefaultMarkdownGenerator) writePostInfo(md *strings.Builder, post *Post) {
	md.WriteString("**基本信息**\n\n")

	if post.Forum != "" {
		fmt.Fprintf(md, "- **版块**: %s\n", g.escapeMarkdown(post.Forum))
	}

	if post.URL != "" {
		fmt.Fprintf(md, "- **原帖链接**: <%s>\n", post.URL)
	}

	if !post.CreatedAt.IsZero() {
		fmt.Fprintf(md, "- **创建时间**: %s\n", post.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Fprintf(md, "- **总楼层数**: %d\n", post.TotalFloors)
	md.WriteString("\n")
}

// FormatPost 格式化单个楼层内容
func (g *DefaultMarkdownGenerator) FormatPost(entry *PostEntry) string {
	var md strings.Builder

	// 作者信息
	if g.options.IncludeAuthorInfo {
		authorInfo := g.FormatAuthor(&entry.Author)
		if authorInfo != "" {
			md.WriteString(authorInfo)
			md.WriteString("\n")
		}
	}

	// 发帖时间
	if !entry.PostTime.IsZero() {
		md.WriteString(fmt.Sprintf("**发帖时间**: %s\n\n", entry.PostTime.Format("2006-01-02 15:04:05")))
	}

	// 帖子内容
	if entry.Content != "" {
		md.WriteString("**内容**:\n\n")
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

	return md.String()
}

// FormatAuthor 格式化作者信息
func (g *DefaultMarkdownGenerator) FormatAuthor(author *Author) string {
	if author.Username == "" {
		return ""
	}

	var md strings.Builder
	md.WriteString("**作者信息**:\n\n")

	// 用户名
	md.WriteString(fmt.Sprintf("- **用户名**: %s", g.escapeMarkdown(author.Username)))

	// UID
	if author.UID != "" {
		md.WriteString(fmt.Sprintf(" (UID: %s)", author.UID))
	}
	md.WriteString("\n")

	// 头像
	if author.Avatar != "" {
		md.WriteString(fmt.Sprintf("- **头像**: ![avatar](%s)\n", author.Avatar))
	}

	// 发帖数
	if author.PostCount > 0 {
		md.WriteString(fmt.Sprintf("- **发帖数**: %d\n", author.PostCount))
	}

	// 注册时间
	if author.RegisterDate != "" {
		md.WriteString(fmt.Sprintf("- **注册时间**: %s\n", author.RegisterDate))
	}

	// 最后登录
	if author.LastLogin != "" {
		md.WriteString(fmt.Sprintf("- **最后登录**: %s\n", author.LastLogin))
	}

	// 个性签名
	if author.Signature != "" {
		md.WriteString(fmt.Sprintf("- **个性签名**: %s\n", g.escapeMarkdown(author.Signature)))
	}

	return md.String()
}

// FormatImages 格式化图片列表
func (g *DefaultMarkdownGenerator) FormatImages(images []Image) string {
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
func (g *DefaultMarkdownGenerator) FormatAttachments(attachments []Attachment) string {
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

// GenerateTableOfContents 生成目录
func (g *DefaultMarkdownGenerator) GenerateTableOfContents(post *Post) string {
	var md strings.Builder

	// 主楼
	md.WriteString("- [主楼 (GF)](#主楼-gf)\n")

	// 回复
	if len(post.Replies) > 0 {
		md.WriteString("- [回复](#回复)\n")
		for i, reply := range post.Replies {
			if g.options.FloorNumbering {
				floorLink := strings.ToLower(strings.ReplaceAll(reply.Floor, "F", "f"))
				md.WriteString(fmt.Sprintf("  - [%s楼](#%s楼)\n", reply.Floor, floorLink))
			} else {
				md.WriteString(fmt.Sprintf("  - [回复 %d](#回复-%d)\n", i+1, i+1))
			}
		}
	}

	return md.String()
}

// writeFooter 写入文档尾部信息
func (g *DefaultMarkdownGenerator) writeFooter(md *strings.Builder, post *Post) {
	md.WriteString("---\n\n")
	md.WriteString("*本文档由 HTML数据提取器 自动生成*\n\n")
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
func (g *DefaultMarkdownGenerator) formatContent(content string) string {
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
func (g *DefaultMarkdownGenerator) escapeMarkdown(text string) string {
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
func (g *DefaultMarkdownGenerator) convertToRelativePath(absolutePath string) string {
	// 简单地使用文件名，或者根据需要实现更复杂的相对路径逻辑
	return filepath.Base(absolutePath)
}

// formatFileSize 格式化文件大小
func (g *DefaultMarkdownGenerator) formatFileSize(size int64) string {
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

// SetImageStyle 设置图片样式
func (g *DefaultMarkdownGenerator) SetImageStyle(style string) {
	g.options.ImageStyle = style
}

// SetIncludeAuthorInfo 设置是否包含作者信息
func (g *DefaultMarkdownGenerator) SetIncludeAuthorInfo(include bool) {
	g.options.IncludeAuthorInfo = include
}

// SetIncludeImages 设置是否包含图片
func (g *DefaultMarkdownGenerator) SetIncludeImages(include bool) {
	g.options.IncludeImages = include
}

// SetTableOfContents 设置是否生成目录
func (g *DefaultMarkdownGenerator) SetTableOfContents(include bool) {
	g.options.TableOfContents = include
	g.options.IncludeTOC = include
}
