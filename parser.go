package south2md

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/cascadia"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// Pre-compiled regex patterns for better performance
var (
	uidPattern          = regexp.MustCompile(`UID:\s*(\d+)`)
	postCountPattern    = regexp.MustCompile(`发帖:\s*(\d+)`)
	registerDatePattern = regexp.MustCompile(`注册时间:\s*([0-9\-]+)`)
	lastLoginPattern    = regexp.MustCompile(`最后登录:\s*([0-9\-]+)`)
	uidURLPattern       = regexp.MustCompile(`uid[=-](\d+)`)
	digitsPattern       = regexp.MustCompile(`(\d+)`)

	selectorCache sync.Map
)

type DOMSelection struct {
	nodes []*html.Node
}

func (s *DOMSelection) Length() int {
	if s == nil {
		return 0
	}
	return len(s.nodes)
}

func (s *DOMSelection) First() *DOMSelection {
	if s == nil || len(s.nodes) == 0 {
		return &DOMSelection{}
	}
	return &DOMSelection{nodes: s.nodes[:1]}
}

func (s *DOMSelection) Eq(i int) *DOMSelection {
	if s == nil || i < 0 || i >= len(s.nodes) {
		return &DOMSelection{}
	}
	return &DOMSelection{nodes: s.nodes[i : i+1]}
}

func (s *DOMSelection) Find(selector string) *DOMSelection {
	if s == nil || len(s.nodes) == 0 {
		return &DOMSelection{}
	}

	compiled, err := compileSelector(selector)
	if err != nil {
		return &DOMSelection{}
	}

	matches := make([]*html.Node, 0)
	for _, node := range s.nodes {
		matches = append(matches, cascadia.QueryAll(node, compiled)...)
	}

	return &DOMSelection{nodes: matches}
}

func (s *DOMSelection) Text() string {
	if s == nil || len(s.nodes) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, node := range s.nodes {
		builder.WriteString(htmlquery.InnerText(node))
	}
	return builder.String()
}

func (s *DOMSelection) Attr(attrName string) (string, bool) {
	if s == nil || len(s.nodes) == 0 {
		return "", false
	}

	for _, attr := range s.nodes[0].Attr {
		if attr.Key == attrName {
			return attr.Val, true
		}
	}
	return "", false
}

func (s *DOMSelection) Html() (string, error) {
	if s == nil || len(s.nodes) == 0 {
		return "", nil
	}

	var builder strings.Builder
	for child := s.nodes[0].FirstChild; child != nil; child = child.NextSibling {
		var buffer bytes.Buffer
		if err := html.Render(&buffer, child); err != nil {
			return "", err
		}
		builder.Write(buffer.Bytes())
	}

	return builder.String(), nil
}

func compileSelector(selector string) (cascadia.Selector, error) {
	if cached, ok := selectorCache.Load(selector); ok {
		return cached.(cascadia.Selector), nil
	}

	compiled, err := cascadia.Compile(selector)
	if err != nil {
		return nil, err
	}
	selectorCache.Store(selector, compiled)
	return compiled, nil
}

// PostParser HTML parser and extractor.
type PostParser struct {
	doc       *html.Node
	baseURL   string
	selectors *HTMLSelectors
}

// NewPostParser creates a new post parser.
func NewPostParser(selectors *HTMLSelectors) *PostParser {
	return &PostParser{
		selectors: selectors,
	}
}

// LoadFromString loads HTML from string.
func (p *PostParser) LoadFromString(htmlContent string) error {
	return p.LoadFromReader(strings.NewReader(htmlContent))
}

// LoadFromFile loads HTML from file.
func (p *PostParser) LoadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	return p.LoadFromReader(file)
}

// LoadFromReader loads HTML from reader.
func (p *PostParser) LoadFromReader(reader io.Reader) error {
	doc, err := html.Parse(reader)
	if err != nil {
		return NewParseError("解析HTML字符串失败", err)
	}

	p.doc = doc
	return nil
}

// FindElement finds a single element.
func (p *PostParser) FindElement(selector string) *DOMSelection {
	elements := p.FindElements(selector)
	if elements == nil {
		return nil
	}
	return elements.First()
}

// FindElements finds multiple elements.
func (p *PostParser) FindElements(selector string) *DOMSelection {
	if p.doc == nil {
		return nil
	}

	compiled, err := compileSelector(selector)
	if err != nil {
		return &DOMSelection{}
	}

	return &DOMSelection{
		nodes: cascadia.QueryAll(p.doc, compiled),
	}
}

// GetBaseURL returns the base URL from document.
func (p *PostParser) GetBaseURL() string {
	if p.baseURL != "" {
		return p.baseURL
	}

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

// ExtractPost extracts full post data.
func (p *PostParser) ExtractPost() (*Post, error) {
	post := &Post{
		CreatedAt: time.Now(),
	}

	titleElement := p.FindElement(p.selectors.Title)
	if titleElement != nil && titleElement.Length() > 0 {
		post.Title = strings.TrimSpace(titleElement.Text())
	}

	forumElement := p.FindElement(p.selectors.Forum)
	if forumElement != nil && forumElement.Length() > 0 {
		post.Forum = p.extractForumName(forumElement)
	}

	baseURL := p.GetBaseURL()
	if baseURL != "" {
		post.URL = baseURL
	}

	post.TID = p.extractTID()

	mainPost, err := p.ExtractMainPost()
	if err != nil {
		return nil, fmt.Errorf("提取主楼失败: %v", err)
	}
	post.MainPost = *mainPost
	post.CreatedAt = mainPost.PostTime

	replies, err := p.ExtractReplies()
	if err != nil {
		return nil, fmt.Errorf("提取回复失败: %v", err)
	}
	post.Replies = replies
	post.TotalFloors = 1 + len(post.Replies)

	return post, nil
}

// ExtractPostFromMultiplePages extracts full post data from multiple page parsers.
func (p *PostParser) ExtractPostFromMultiplePages(parsers []*PostParser) (*Post, error) {
	if len(parsers) == 0 {
		return nil, fmt.Errorf("没有提供页面解析器")
	}

	post, err := parsers[0].ExtractPost()
	if err != nil {
		return nil, fmt.Errorf("提取第一页数据失败: %v", err)
	}

	for i := 1; i < len(parsers); i++ {
		replies, err := parsers[i].ExtractReplies()
		if err != nil {
			slog.Error("Failed to extract replies from page", "page", i+1, "error", err)
			continue
		}

		post.Replies = append(post.Replies, replies...)
	}

	post.TotalFloors = 1 + len(post.Replies)
	return post, nil
}

// ExtractMainPost extracts the main post.
func (p *PostParser) ExtractMainPost() (*PostEntry, error) {
	postTable := p.FindElement(p.selectors.PostTable)
	if postTable == nil || postTable.Length() == 0 {
		return nil, NewValidationError(fmt.Sprintf("未找到帖子表格 (选择器: %s)", p.selectors.PostTable))
	}

	postContent := postTable.Find(p.selectors.PostContent)
	if postContent == nil || postContent.Length() == 0 {
		return nil, NewValidationError(fmt.Sprintf("未找到帖子内容 (选择器: %s)", p.selectors.PostContent))
	}

	return p.extractPostEntry(postTable, "GF")
}

// ExtractReplies extracts all replies.
func (p *PostParser) ExtractReplies() ([]PostEntry, error) {
	postTables := p.FindElements(p.selectors.PostTable)
	if postTables == nil || postTables.Length() == 0 {
		return nil, NewValidationError(fmt.Sprintf("未找到帖子表格 (选择器: %s)", p.selectors.PostTable))
	}

	tableCount := postTables.Length()
	if tableCount <= 1 {
		return []PostEntry{}, nil
	}

	replies := make([]PostEntry, 0, tableCount-1)
	for i := 1; i < tableCount; i++ {
		floorNumber := p.generateFloorNumber(i)
		entry, err := p.extractPostEntry(postTables.Eq(i), floorNumber)
		if err != nil {
			slog.Error("Failed to extract floor", "floor", i, "error", err)
			continue
		}
		replies = append(replies, *entry)
	}

	return replies, nil
}

// extractPostEntry extracts a single post entry.
func (p *PostParser) extractPostEntry(table *DOMSelection, floor string) (*PostEntry, error) {
	entry := &PostEntry{
		Floor: floor,
	}

	author, err := p.ExtractAuthor(table)
	if err != nil {
		author = &Author{}
	}
	entry.Author = *author

	timeElement := table.Find(p.selectors.PostTime)
	if timeElement.Length() > 0 {
		entry.PostTime = p.parsePostTime(timeElement.First().Text())
	}

	contentElement := table.Find(p.selectors.PostContent)
	if contentElement.Length() > 0 {
		if htmlContent, err := contentElement.Html(); err == nil {
			entry.HTMLContent = p.cleanHTMLContent(htmlContent)
		}
	}

	entry.PostID = p.extractPostID(table)
	return entry, nil
}

// ExtractAuthor extracts author information.
func (p *PostParser) ExtractAuthor(element *DOMSelection) (*Author, error) {
	author := &Author{}

	usernameElement := element.Find("a[href*=\"u.php\"] strong")
	if usernameElement.Length() > 0 {
		author.Username = strings.TrimSpace(usernameElement.Text())
	} else {
		usernameElement = element.Find("strong")
		if usernameElement.Length() > 0 {
			author.Username = strings.TrimSpace(usernameElement.Text())
		}
	}

	uidElement := element.Find("a[href*=\"u.php\"]")
	if uidElement.Length() > 0 {
		if href, exists := uidElement.First().Attr("href"); exists {
			author.UID = p.extractUIDFromURL(href)
		}
	}

	avatarElement := element.Find("img[loading=\"lazy\"]")
	if avatarElement.Length() > 0 {
		if src, exists := avatarElement.First().Attr("src"); exists {
			author.Avatar = src
		}
	}

	userInfoElements := element.Find(".user-info")
	for i := 0; i < userInfoElements.Length(); i++ {
		infoText := userInfoElements.Eq(i).Text()

		if author.UID == "" {
			uidMatches := uidPattern.FindStringSubmatch(infoText)
			if len(uidMatches) > 1 {
				author.UID = uidMatches[1]
			}
		}

		if author.PostCount == 0 {
			postCountMatches := postCountPattern.FindStringSubmatch(infoText)
			if len(postCountMatches) > 1 {
				if count, err := strconv.Atoi(postCountMatches[1]); err == nil {
					author.PostCount = count
				}
			}
		}

		if author.RegisterDate == "" {
			registerDateMatches := registerDatePattern.FindStringSubmatch(infoText)
			if len(registerDateMatches) > 1 {
				author.RegisterDate = registerDateMatches[1]
			}
		}

		if author.LastLogin == "" {
			lastLoginMatches := lastLoginPattern.FindStringSubmatch(infoText)
			if len(lastLoginMatches) > 1 {
				author.LastLogin = lastLoginMatches[1]
			}
		}
	}

	if author.Signature == "" {
		signatureElement := element.Find(".bianji")
		if signatureElement.Length() > 0 {
			signatureText := strings.TrimSpace(signatureElement.Text())
			signatureText = strings.TrimPrefix(signatureText, "（")
			signatureText = strings.TrimSuffix(signatureText, "）")
			author.Signature = signatureText
		}
	}

	return author, nil
}

func (p *PostParser) extractForumName(element *DOMSelection) string {
	text := element.Text()
	parts := strings.Split(text, "»")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return strings.TrimSpace(text)
}

func (p *PostParser) generateFloorNumber(index int) string {
	if index == 0 {
		return "GF"
	}
	return fmt.Sprintf("B%dF", index)
}

func (p *PostParser) parsePostTime(timeText string) time.Time {
	timeText = strings.TrimSpace(timeText)

	formats := []string{
		"2006-1-2 15:04:05",
		"2006-01-02 15:04:05",
		"2006-1-2 15:04",
		"2006-01-02 15:04",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeText); err == nil {
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t
		}
	}

	return time.Now()
}

func (p *PostParser) extractPostID(element *DOMSelection) string {
	tableCell := element.Find("th[id^=\"td_\"]")
	if tableCell.Length() > 0 {
		if id, exists := tableCell.First().Attr("id"); exists {
			return strings.TrimPrefix(id, "td_")
		}
	}

	contentElement := element.Find("[id^=\"read_\"]")
	if contentElement.Length() > 0 {
		if id, exists := contentElement.First().Attr("id"); exists {
			return strings.TrimPrefix(id, "read_")
		}
	}

	linkElement := element.Find("a[href*=\"#\"]")
	if linkElement.Length() > 0 {
		if href, exists := linkElement.First().Attr("href"); exists {
			if idx := strings.LastIndex(href, "#"); idx != -1 {
				return href[idx+1:]
			}
		}
	}

	return ""
}

func (p *PostParser) extractUIDFromURL(urlStr string) string {
	matches := uidURLPattern.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (p *PostParser) cleanHTMLContent(str string) string {
	return CleanHTMLContent(str)
}

func (p *PostParser) extractTID() string {
	titleElement := p.FindElement("title")
	if titleElement != nil && titleElement.Length() > 0 {
		titleText := titleElement.Text()
		if strings.Contains(titleText, "read.php?tid-") {
			parts := strings.Split(titleText, "read.php?tid-")
			if len(parts) > 1 {
				tidPart := parts[1]
				matches := digitsPattern.FindStringSubmatch(tidPart)
				if len(matches) > 0 {
					return matches[1]
				}
			}
		}
	}

	baseURL := p.GetBaseURL()
	if baseURL != "" && strings.Contains(baseURL, "tid-") {
		parts := strings.Split(baseURL, "tid-")
		if len(parts) > 1 {
			tidPart := parts[1]
			matches := digitsPattern.FindStringSubmatch(tidPart)
			if len(matches) > 0 {
				return matches[1]
			}
		}
	}

	tidElements := p.FindElements("a[href*='tid-']")
	if tidElements == nil || tidElements.Length() == 0 {
		return ""
	}

	for i := 0; i < tidElements.Length(); i++ {
		href, exists := tidElements.Eq(i).Attr("href")
		if !exists || !strings.Contains(href, "tid-") {
			continue
		}

		parts := strings.Split(href, "tid-")
		if len(parts) <= 1 {
			continue
		}

		tidPart := parts[1]
		matches := digitsPattern.FindStringSubmatch(tidPart)
		if len(matches) > 0 {
			return matches[1]
		}
	}

	return ""
}
