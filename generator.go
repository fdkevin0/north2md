package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
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
			g.writeReplyPost(post.TID, &md, reply, i+1)
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
		preview := strings.TrimSpace(reply.HTMLContent)
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
	g.writePostWithComplexHeader(post.TID, md, post.MainPost, 0, "0")
	md.WriteString("\n")
}

// writeReplyPost 写入回复楼层内容
func (g *MarkdownGenerator) writeReplyPost(tid string, md *strings.Builder, reply PostEntry, index int) {
	g.writePostWithComplexHeader(tid, md, reply, index, reply.Floor)
	md.WriteString("\n")
}

// writePostWithComplexHeader 使用复杂标题格式写入帖子
func (g *MarkdownGenerator) writePostWithComplexHeader(tid string, md *strings.Builder, entry PostEntry, index int, floor string) {
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
			log.Fatalln(err)
		}

		md2, err := downloadAndCacheImages(tid, []byte(markdown), "images")
		if err != nil {
			log.Fatalln(err)
		}

		md.WriteString(string(md2))
		md.WriteString("\n\n")
	}
}

// writeFooter 写入文档尾部信息
func (g *MarkdownGenerator) writeFooter(md *strings.Builder, post *Post) {
	md.WriteString("---\n\n")
	md.WriteString("*本文档由 ngapost2md 自动生成*\n\n")
	fmt.Fprintf(md, "*生成时间: %s*\n", time.Now().Format("2006-01-02 15:04:05"))
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
