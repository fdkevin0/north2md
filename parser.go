package main

import (
	"fmt"
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

// GoqueryElement goquery元素包装
type GoqueryElement struct {
	selection *goquery.Selection
}

// GoqueryElements goquery元素集合包装
type GoqueryElements struct {
	selection *goquery.Selection
}

// NewHTMLParser 创建新的HTML解析器
func NewHTMLParser() *HTMLParser {
	return &HTMLParser{}
}

// LoadFromFile 从文件加载HTML
func (p *HTMLParser) LoadFromFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	doc, err := goquery.NewDocumentFromReader(file)
	if err != nil {
		return fmt.Errorf("解析HTML文档失败: %v", err)
	}

	p.doc = doc
	return nil
}

// LoadFromString 从字符串加载HTML
func (p *HTMLParser) LoadFromString(html string) error {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return fmt.Errorf("解析HTML字符串失败: %v", err)
	}

	p.doc = doc
	return nil
}

// FindElement 查找单个元素
func (p *HTMLParser) FindElement(selector string) Element {
	if p.doc == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}

	selection := p.doc.Find(selector).First()
	return &GoqueryElement{selection: selection}
}

// FindElements 查找多个元素
func (p *HTMLParser) FindElements(selector string) Elements {
	if p.doc == nil {
		return &GoqueryElements{selection: &goquery.Selection{}}
	}

	selection := p.doc.Find(selector)
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

// GetDocument 获取goquery文档对象
func (p *HTMLParser) GetDocument() *goquery.Document {
	return p.doc
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

// GoqueryElement 方法实现

// Text 获取元素文本内容
func (e *GoqueryElement) Text() string {
	if e.selection == nil {
		return ""
	}
	return strings.TrimSpace(e.selection.Text())
}

// HTML 获取元素HTML内容
func (e *GoqueryElement) HTML() string {
	if e.selection == nil {
		return ""
	}
	html, _ := e.selection.Html()
	return html
}

// Attr 获取元素属性
func (e *GoqueryElement) Attr(name string) (string, bool) {
	if e.selection == nil {
		return "", false
	}
	return e.selection.Attr(name)
}

// Find 在元素内查找子元素
func (e *GoqueryElement) Find(selector string) Elements {
	if e.selection == nil {
		return &GoqueryElements{selection: &goquery.Selection{}}
	}
	return &GoqueryElements{selection: e.selection.Find(selector)}
}

// First 获取第一个元素
func (e *GoqueryElement) First() Element {
	if e.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: e.selection.First()}
}

// Last 获取最后一个元素
func (e *GoqueryElement) Last() Element {
	if e.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: e.selection.Last()}
}

// Eq 获取指定索引的元素
func (e *GoqueryElement) Eq(index int) Element {
	if e.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: e.selection.Eq(index)}
}

// Length 获取元素数量
func (e *GoqueryElement) Length() int {
	if e.selection == nil {
		return 0
	}
	return e.selection.Length()
}

// Each 遍历元素
func (e *GoqueryElement) Each(fn func(int, Element)) {
	if e.selection == nil {
		return
	}
	e.selection.Each(func(i int, s *goquery.Selection) {
		fn(i, &GoqueryElement{selection: s})
	})
}

// HasClass 检查是否有指定CSS类
func (e *GoqueryElement) HasClass(class string) bool {
	if e.selection == nil {
		return false
	}
	return e.selection.HasClass(class)
}

// Parent 获取父元素
func (e *GoqueryElement) Parent() Element {
	if e.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: e.selection.Parent()}
}

// Children 获取子元素
func (e *GoqueryElement) Children() Elements {
	if e.selection == nil {
		return &GoqueryElements{selection: &goquery.Selection{}}
	}
	return &GoqueryElements{selection: e.selection.Children()}
}

// Next 获取下一个兄弟元素
func (e *GoqueryElement) Next() Element {
	if e.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: e.selection.Next()}
}

// Prev 获取上一个兄弟元素
func (e *GoqueryElement) Prev() Element {
	if e.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: e.selection.Prev()}
}

// GoqueryElements 方法实现

// Length 获取元素集合大小
func (es *GoqueryElements) Length() int {
	if es.selection == nil {
		return 0
	}
	return es.selection.Length()
}

// Eq 获取指定索引的元素
func (es *GoqueryElements) Eq(index int) Element {
	if es.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: es.selection.Eq(index)}
}

// First 获取第一个元素
func (es *GoqueryElements) First() Element {
	if es.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: es.selection.First()}
}

// Last 获取最后一个元素
func (es *GoqueryElements) Last() Element {
	if es.selection == nil {
		return &GoqueryElement{selection: &goquery.Selection{}}
	}
	return &GoqueryElement{selection: es.selection.Last()}
}

// Each 遍历元素集合
func (es *GoqueryElements) Each(fn func(int, Element)) {
	if es.selection == nil {
		return
	}
	es.selection.Each(func(i int, s *goquery.Selection) {
		fn(i, &GoqueryElement{selection: s})
	})
}

// Text 获取所有元素的文本内容
func (es *GoqueryElements) Text() string {
	if es.selection == nil {
		return ""
	}
	return strings.TrimSpace(es.selection.Text())
}

// HTML 获取第一个元素的HTML内容
func (es *GoqueryElements) HTML() string {
	if es.selection == nil {
		return ""
	}
	html, _ := es.selection.Html()
	return html
}
