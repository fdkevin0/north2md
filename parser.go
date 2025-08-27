package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTMLParser HTML解析器
type HTMLParser struct {
	doc     *goquery.Document
	baseURL string
}

// NewHTMLParser 创建新的HTML解析器
func NewHTMLParser() *HTMLParser {
	return &HTMLParser{}
}

// LoadFromString 从字符串加载HTML
func (p *HTMLParser) LoadFromString(html string) error {
	return p.LoadFromReader(strings.NewReader(html))
}

// LoadFromFile 从文件加载HTML
func (p *HTMLParser) LoadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	return p.LoadFromReader(file)
}

func (p *HTMLParser) LoadFromReader(reader io.Reader) error {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return fmt.Errorf("解析HTML字符串失败: %v", err)
	}

	p.doc = doc
	return nil
}

// FindElement 查找单个元素
func (p *HTMLParser) FindElement(selector string) *goquery.Selection {
	if p.doc == nil {
		return nil
	}
	return p.doc.Find(selector).First()
}

// FindElements 查找多个元素
func (p *HTMLParser) FindElements(selector string) *goquery.Selection {
	if p.doc == nil {
		return nil
	}
	return p.doc.Find(selector)
}

// GetBaseURL 获取基础URL
func (p *HTMLParser) GetBaseURL() string {
	if p.baseURL != "" {
		return p.baseURL
	}

	// 尝试从HTML文档中获取base标签
	baseElement := p.FindElement("base")
	if baseElement != nil && baseElement.Length() > 0 {
		if href, exists := baseElement.Attr("href"); exists {
			if strings.HasPrefix(href, "//") {
				return "https:" + href
			}
			return href
		}
	}

	return ""
}

// SetBaseURL 设置基础URL
func (p *HTMLParser) SetBaseURL(baseURL string) {
	p.baseURL = baseURL
}

// ResolveURL 解析相对URL为绝对URL
func (p *HTMLParser) ResolveURL(relativeURL string) string {
	if relativeURL == "" {
		return ""
	}

	// 如果已经是绝对URL，直接返回
	if strings.HasPrefix(relativeURL, "http://") || strings.HasPrefix(relativeURL, "https://") {
		return relativeURL
	}

	baseURL := p.GetBaseURL()
	if baseURL == "" {
		return relativeURL
	}

	// 处理协议相对URL
	if strings.HasPrefix(relativeURL, "//") {
		if strings.HasPrefix(baseURL, "https://") {
			return "https:" + relativeURL
		}
		return "http:" + relativeURL
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return relativeURL
	}

	relative, err := url.Parse(relativeURL)
	if err != nil {
		return relativeURL
	}

	resolved := base.ResolveReference(relative)
	return resolved.String()
}