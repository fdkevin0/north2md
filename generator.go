package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// MarkdownGenerator Markdown生成器
type MarkdownGenerator struct {
	formatter    *MarkdownFormatter
	imageHandler *ImageHandler
}

// NewMarkdownGenerator 创建新的Markdown生成器
func NewMarkdownGenerator(options *MarkdownOptions) *MarkdownGenerator {
	return &MarkdownGenerator{
		formatter:    NewMarkdownFormatter(options),
		imageHandler: NewImageHandler("images"),
	}
}

// GenerateMarkdown 生成完整的Markdown文档
func (g *MarkdownGenerator) GenerateMarkdown(post *Post) (string, error) {
	var md strings.Builder

	// 文档标题
	md.WriteString(g.formatter.FormatTitle(post.Title))

	// 归属信息
	md.WriteString(g.formatter.FormatAttribution())

	// 热门回复
	md.WriteString(g.formatter.FormatPopularReplies(post, g.imageHandler))

	md.WriteString("----\n\n")

	// 主楼内容
	mainPostContent, err := g.formatter.FormatPostEntry(post.TID, post.MainPost, 0, "0", post, g.imageHandler)
	if err != nil {
		return "", fmt.Errorf("failed to format main post: %w", err)
	}
	md.WriteString(mainPostContent)
	md.WriteString("\n")

	// 回复内容
	if len(post.Replies) > 0 {
		for i, reply := range post.Replies {
			replyContent, err := g.formatter.FormatPostEntry(post.TID, reply, i+1, reply.Floor, post, g.imageHandler)
			if err != nil {
				return "", fmt.Errorf("failed to format reply %d: %w", i, err)
			}
			md.WriteString(replyContent)
			md.WriteString("\n")
		}
	}

	// 文档尾部信息
	md.WriteString(g.formatter.FormatFooter())

	return md.String(), nil
}

// SavePost 保存帖子到指定目录结构
func (g *MarkdownGenerator) SavePost(post *Post, baseDir string) error {
	// 创建以TID命名的目录
	tidDir := filepath.Join(baseDir, post.TID)
	if err := os.MkdirAll(tidDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	imagesDir := filepath.Join(tidDir, "images")

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("创建images目录失败: %v", err)
	}

	// 检查是否存在现有metadata，如果存在则加载图片缓存信息
	metadataFile := filepath.Join(tidDir, "metadata.toml")
	if _, err := os.Stat(metadataFile); err == nil {
		data, err := os.ReadFile(metadataFile)
		if err == nil {
			var existingPost Post
			err = toml.Unmarshal(data, &existingPost)
			if err == nil {
				post.Images = existingPost.Images
				slog.Info("Loaded existing image cache from metadata", "count", len(post.Images))
			} else {
				slog.Warn("Failed to unmarshal existing metadata", "error", err)
			}
		} else {
			slog.Warn("Failed to read existing metadata", "error", err)
		}
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

	if err := os.WriteFile(metadataFile, metadata, 0644); err != nil {
		return fmt.Errorf("保存metadata.toml失败: %v", err)
	}

	return nil
}

