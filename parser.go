package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Element 表示DOM元素的包装接口
type Element interface {
	Text() string
	HTML() string
	Attr(string) (string, bool)
	Find(string) Elements
	First() Element
	Last() Element
	Eq(int) Element
	Length() int
	Each(func(int, Element))
	HasClass(string) bool
	Parent() Element
	Children() Elements
	Next() Element
	Prev() Element
}

// Elements 表示DOM元素集合
type Elements interface {
	Length() int
	Eq(int) Element
	First() Element
	Last() Element
	Each(func(int, Element))
	Text() string
	HTML() string
}

// DefaultHTMLParser 默认HTML解析器实现
type HTMLParser struct {
	doc     *goquery.Document
	baseURL string
}

// ensureSelection 确保selection不为nil，消除特殊情况
func (p *HTMLParser) ensureSelection() *goquery.Selection {
	if p.doc == nil {
		return &goquery.Selection{}
	}
	return p.doc.Selection
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
func (p *HTMLParser) FindElement(selector string) Element {
	selection := p.ensureSelection().Find(selector).First()
	return &GoqueryElement{selection: selection}
}

// FindElements 查找多个元素
func (p *HTMLParser) FindElements(selector string) Elements {
	selection := p.ensureSelection().Find(selector)
	return &GoqueryElements{selection: selection}
}

// GetBaseURL 获取基础URL
func (p *HTMLParser) GetBaseURL() string {
	if p.baseURL != "" {
		return p.baseURL
	}

	// 尝试从HTML文档中获取base标签
	baseElement := p.FindElement("base")
	if baseElement.Length() > 0 {
		if href, exists := baseElement.Attr("href"); exists {
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

// GoqueryElement goquery元素包装
type GoqueryElement struct {
	selection *goquery.Selection
}

// GoqueryElements goquery元素集合包装
type GoqueryElements struct {
	selection *goquery.Selection
}

// ensureSelection 确保selection不为nil，消除特殊情况
func (e *GoqueryElement) ensureSelection() *goquery.Selection {
	if e.selection == nil {
		return &goquery.Selection{}
	}
	return e.selection
}

// Text 获取元素文本内容
func (e *GoqueryElement) Text() string {
	return strings.TrimSpace(e.ensureSelection().Text())
}

// HTML 获取元素HTML内容
func (e *GoqueryElement) HTML() string {
	html, _ := e.ensureSelection().Html()
	return html
}

// Attr 获取元素属性
func (e *GoqueryElement) Attr(name string) (string, bool) {
	return e.ensureSelection().Attr(name)
}

// Find 在元素内查找子元素
func (e *GoqueryElement) Find(selector string) Elements {
	return &GoqueryElements{selection: e.ensureSelection().Find(selector)}
}

// First 获取第一个元素
func (e *GoqueryElement) First() Element {
	return &GoqueryElement{selection: e.ensureSelection().First()}
}

// Last 获取最后一个元素
func (e *GoqueryElement) Last() Element {
	return &GoqueryElement{selection: e.ensureSelection().Last()}
}

// Eq 获取指定索引的元素
func (e *GoqueryElement) Eq(index int) Element {
	return &GoqueryElement{selection: e.ensureSelection().Eq(index)}
}

// Length 获取元素数量
func (e *GoqueryElement) Length() int {
	return e.ensureSelection().Length()
}

// Each 遍历元素
func (e *GoqueryElement) Each(fn func(int, Element)) {
	e.ensureSelection().Each(func(i int, s *goquery.Selection) {
		fn(i, &GoqueryElement{selection: s})
	})
}

// HasClass 检查是否有指定CSS类
func (e *GoqueryElement) HasClass(class string) bool {
	return e.ensureSelection().HasClass(class)
}

// Parent 获取父元素
func (e *GoqueryElement) Parent() Element {
	return &GoqueryElement{selection: e.ensureSelection().Parent()}
}

// Children 获取子元素
func (e *GoqueryElement) Children() Elements {
	return &GoqueryElements{selection: e.ensureSelection().Children()}
}

// Next 获取下一个兄弟元素
func (e *GoqueryElement) Next() Element {
	return &GoqueryElement{selection: e.ensureSelection().Next()}
}

// Prev 获取上一个兄弟元素
func (e *GoqueryElement) Prev() Element {
	return &GoqueryElement{selection: e.ensureSelection().Prev()}
}

// GoqueryElements 方法实现

// ensureSelection 确保selection不为nil，消除特殊情况
func (es *GoqueryElements) ensureSelection() *goquery.Selection {
	if es.selection == nil {
		return &goquery.Selection{}
	}
	return es.selection
}

// Length 获取元素集合大小
func (es *GoqueryElements) Length() int {
	return es.ensureSelection().Length()
}

// Eq 获取指定索引的元素
func (es *GoqueryElements) Eq(index int) Element {
	return &GoqueryElement{selection: es.ensureSelection().Eq(index)}
}

// First 获取第一个元素
func (es *GoqueryElements) First() Element {
	return &GoqueryElement{selection: es.ensureSelection().First()}
}

// Last 获取最后一个元素
func (es *GoqueryElements) Last() Element {
	return &GoqueryElement{selection: es.ensureSelection().Last()}
}

// Each 遍历元素集合
func (es *GoqueryElements) Each(fn func(int, Element)) {
	es.ensureSelection().Each(func(i int, s *goquery.Selection) {
		fn(i, &GoqueryElement{selection: s})
	})
}

// Text 获取所有元素的文本内容
func (es *GoqueryElements) Text() string {
	return strings.TrimSpace(es.ensureSelection().Text())
}

// HTML 获取第一个元素的HTML内容
func (es *GoqueryElements) HTML() string {
	html, _ := es.ensureSelection().Html()
	return html
}
