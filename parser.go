package south2md

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Pre-compiled regex patterns for better performance
var (
	uidPattern          = regexp.MustCompile(`UID:\s*(\d+)`)
	postCountPattern    = regexp.MustCompile(`发帖:\s*(\d+)`)
	registerDatePattern = regexp.MustCompile(`注册时间:\s*([0-9\-]+)`)
	lastLoginPattern    = regexp.MustCompile(`最后登录:\s*([0-9\-]+)`)
	uidURLPattern       = regexp.MustCompile(`uid[=-](\d+)`)
	digitsPattern       = regexp.MustCompile(`(\d+)`)
)

// PostParser HTML解析和数据提取器
type PostParser struct {
	doc       *goquery.Document
	baseURL   string
	selectors *HTMLSelectors
}

// NewPostParser 创建新的帖子解析器
func NewPostParser(selectors *HTMLSelectors) *PostParser {
	return &PostParser{
		selectors: selectors,
	}
}

// LoadFromString 从字符串加载HTML
func (p *PostParser) LoadFromString(html string) error {
	return p.LoadFromReader(strings.NewReader(html))
}

// LoadFromFile 从文件加载HTML
func (p *PostParser) LoadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	return p.LoadFromReader(file)
}

// LoadFromReader 从读取器加载HTML
func (p *PostParser) LoadFromReader(reader io.Reader) error {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return NewParseError("解析HTML字符串失败", err)
	}

	p.doc = doc
	return nil
}

// FindElement 查找单个元素
func (p *PostParser) FindElement(selector string) *goquery.Selection {
	if p.doc == nil {
		return nil
	}
	return p.doc.Find(selector).First()
}

// FindElements 查找多个元素
func (p *PostParser) FindElements(selector string) *goquery.Selection {
	if p.doc == nil {
		return nil
	}
	return p.doc.Find(selector)
}

// GetBaseURL 获取基础URL
func (p *PostParser) GetBaseURL() string {
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

// ExtractPost 提取完整的帖子数据
func (p *PostParser) ExtractPost() (*Post, error) {
	post := &Post{
		CreatedAt: time.Now(),
	}

	// 提取标题
	titleElement := p.FindElement(p.selectors.Title)
	if titleElement != nil && titleElement.Length() > 0 {
		post.Title = strings.TrimSpace(titleElement.Text())
	}

	// 提取版块信息
	forumElement := p.FindElement(p.selectors.Forum)
	if forumElement != nil && forumElement.Length() > 0 {
		post.Forum = p.extractForumName(forumElement)
	}

	// 设置URL
	baseURL := p.GetBaseURL()
	if baseURL != "" {
		post.URL = baseURL
	}

	// 提取TID
	post.TID = p.extractTID()

	// 提取主楼内容
	mainPost, err := p.ExtractMainPost()
	if err != nil {
		return nil, fmt.Errorf("提取主楼失败: %v", err)
	}
	post.MainPost = *mainPost
	post.CreatedAt = mainPost.PostTime

	// 提取回复
	replies, err := p.ExtractReplies()
	if err != nil {
		return nil, fmt.Errorf("提取回复失败: %v", err)
	}
	post.Replies = replies

	// 更新总楼层数
	post.TotalFloors = 1 + len(post.Replies)

	return post, nil
}

// ExtractPostFromMultiplePages 从多个页面提取完整的帖子数据
func (p *PostParser) ExtractPostFromMultiplePages(parsers []*PostParser) (*Post, error) {
	if len(parsers) == 0 {
		return nil, fmt.Errorf("没有提供页面解析器")
	}

	// 使用第一页的数据初始化帖子
	post, err := parsers[0].ExtractPost()
	if err != nil {
		return nil, fmt.Errorf("提取第一页数据失败: %v", err)
	}

	// 从后续页面提取回复并追加到帖子中
	for i := 1; i < len(parsers); i++ {
		replies, err := parsers[i].ExtractReplies()
		if err != nil {
			slog.Error("Failed to extract replies from page", "page", i+1, "error", err)
			continue
		}

		// 追加回复
		post.Replies = append(post.Replies, replies...)
	}

	// 更新总楼层数
	post.TotalFloors = 1 + len(post.Replies)

	return post, nil
}

// ExtractMainPost 提取主楼内容
func (p *PostParser) ExtractMainPost() (*PostEntry, error) {
	// 查找主楼表格
	postTable := p.FindElement(p.selectors.PostTable)
	if postTable == nil || postTable.Length() == 0 {
		return nil, NewValidationError(fmt.Sprintf("未找到帖子表格 (选择器: %s)", p.selectors.PostTable))
	}

	// 查找主楼内容
	postContent := postTable.Find(p.selectors.PostContent)
	if postContent == nil || postContent.Length() == 0 {
		return nil, NewValidationError(fmt.Sprintf("未找到帖子内容 (选择器: %s)", p.selectors.PostContent))
	}

	return p.extractPostEntry(postTable, "GF")
}

// ExtractReplies 提取所有回复
func (p *PostParser) ExtractReplies() ([]PostEntry, error) {
	// 查找所有帖子表格，跳过第一个（主楼）
	postTables := p.FindElements(p.selectors.PostTable)
	if postTables == nil || postTables.Length() == 0 {
		return nil, NewValidationError(fmt.Sprintf("未找到帖子表格 (选择器: %s)", p.selectors.PostTable))
	}

	tableCount := postTables.Length()
	if tableCount <= 1 {
		return []PostEntry{}, nil
	}

	// Pre-allocate slice for better memory efficiency
	replies := make([]PostEntry, 0, tableCount-1)

	// Cache DOM selections to avoid repeated Eq(i) calls
	tables := make([]*goquery.Selection, tableCount)
	for i := 0; i < tableCount; i++ {
		tables[i] = postTables.Eq(i)
	}

	for i := 1; i < tableCount; i++ {
		floorNumber := p.generateFloorNumber(i)
		entry, err := p.extractPostEntry(tables[i], floorNumber)
		if err != nil {
			slog.Error("Failed to extract floor", "floor", i, "error", err)
			continue
		}

		replies = append(replies, *entry)
	}

	return replies, nil
}

// extractPostEntry 提取单个帖子条目
func (p *PostParser) extractPostEntry(table *goquery.Selection, floor string) (*PostEntry, error) {
	entry := &PostEntry{
		Floor: floor,
	}

	// 提取作者信息
	author, err := p.ExtractAuthor(table)
	if err != nil {
		author = &Author{} // 使用空的作者信息
	}
	entry.Author = *author

	// 提取发帖时间
	timeElement := table.Find(p.selectors.PostTime)
	if timeElement.Length() > 0 {
		entry.PostTime = p.parsePostTime(timeElement.First().Text())
	}

	// 提取帖子内容
	contentElement := table.Find(p.selectors.PostContent)
	if contentElement.Length() > 0 {
		if html, err := contentElement.Html(); err == nil {
			entry.HTMLContent = p.cleanHTMLContent(html)
		}
	}

	// 提取帖子ID
	entry.PostID = p.extractPostID(table)

	return entry, nil
}

// ExtractAuthor 提取作者信息
func (p *PostParser) ExtractAuthor(element *goquery.Selection) (*Author, error) {
	author := &Author{}

	// 提取用户名
	usernameElement := element.Find("a[href*=\"u.php\"] strong")
	if usernameElement.Length() > 0 {
		author.Username = strings.TrimSpace(usernameElement.Text())
	} else {
		usernameElement := element.Find("strong")
		if usernameElement.Length() > 0 {
			author.Username = strings.TrimSpace(usernameElement.Text())
		}
	}

	// 提取UID
	uidElement := element.Find("a[href*=\"u.php\"]")
	if uidElement.Length() > 0 {
		if href, exists := uidElement.First().Attr("href"); exists {
			author.UID = p.extractUIDFromURL(href)
		}
	}

	// 提取头像
	avatarElement := element.Find("img[loading=\"lazy\"]")
	if avatarElement.Length() > 0 {
		if src, exists := avatarElement.First().Attr("src"); exists {
			author.Avatar = src
		}
	}

	// 提取其他统计信息
	userInfoElements := element.Find(".user-info")
	if userInfoElements.Length() > 0 {
		for i := 0; i < userInfoElements.Length(); i++ {
			infoElement := userInfoElements.Eq(i)
			infoText := infoElement.Text()

			// 提取UID
			if author.UID == "" {
				uidMatches := uidPattern.FindStringSubmatch(infoText)
				if len(uidMatches) > 1 {
					author.UID = uidMatches[1]
				}
			}

			// 提取发帖数
			if author.PostCount == 0 {
				postCountMatches := postCountPattern.FindStringSubmatch(infoText)
				if len(postCountMatches) > 1 {
					if count, err := strconv.Atoi(postCountMatches[1]); err == nil {
						author.PostCount = count
					}
				}
			}

			// 提取注册时间
			if author.RegisterDate == "" {
				registerDateMatches := registerDatePattern.FindStringSubmatch(infoText)
				if len(registerDateMatches) > 1 {
					author.RegisterDate = registerDateMatches[1]
				}
			}

			// 提取最后登录时间
			if author.LastLogin == "" {
				lastLoginMatches := lastLoginPattern.FindStringSubmatch(infoText)
				if len(lastLoginMatches) > 1 {
					author.LastLogin = lastLoginMatches[1]
				}
			}
		}
	}

	// 提取个性签名 - 从 div.bianji 元素提取
	if author.Signature == "" {
		signatureElement := element.Find(".bianji")
		if signatureElement.Length() > 0 {
			signatureText := strings.TrimSpace(signatureElement.Text())
			// 移除括号
			signatureText = strings.TrimPrefix(signatureText, "（")
			signatureText = strings.TrimSuffix(signatureText, "）")
			author.Signature = signatureText
		}
	}

	return author, nil
}

// 辅助方法

// extractForumName 提取版块名称
func (p *PostParser) extractForumName(element *goquery.Selection) string {
	text := element.Text()

	// 通常版块名称在导航的最后一个链接中
	parts := strings.Split(text, "»")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}

	return strings.TrimSpace(text)
}

// generateFloorNumber 生成楼层编号
func (p *PostParser) generateFloorNumber(index int) string {
	if index == 0 {
		return "GF"
	}
	return fmt.Sprintf("B%dF", index)
}

// parsePostTime 解析发帖时间
func (p *PostParser) parsePostTime(timeText string) time.Time {
	timeText = strings.TrimSpace(timeText)

	// 尝试多种时间格式
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

	// 如果都失败了，返回当前时间
	return time.Now()
}

// extractPostID 提取帖子ID
func (p *PostParser) extractPostID(element *goquery.Selection) string {
	// 尝试从table cell的id中提取 (e.g., id="td_tpc")
	tableCell := element.Find("th[id^=\"td_\"]")
	if tableCell.Length() > 0 {
		if id, exists := tableCell.First().Attr("id"); exists {
			return strings.TrimPrefix(id, "td_")
		}
	}

	// 尝试从read_xxx id中提取 (fallback)
	contentElement := element.Find("[id^=\"read_\"]")
	if contentElement.Length() > 0 {
		if id, exists := contentElement.First().Attr("id"); exists {
			return strings.TrimPrefix(id, "read_")
		}
	}

	// 尝试从其他可能的位置提取
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

// extractUIDFromURL 从URL中提取UID
func (p *PostParser) extractUIDFromURL(urlStr string) string {
	matches := uidURLPattern.FindStringSubmatch(urlStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// cleanHTMLContent 清理HTML内容
func (p *PostParser) cleanHTMLContent(str string) string {
	// 使用共享的清理函数确保一致性
	return CleanHTMLContent(str)
}

// extractTID 提取帖子ID
func (p *PostParser) extractTID() string {
	// 尝试从标题中提取TID
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

	// 尝试从URL中提取TID
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

	// 尝试从页面中的链接中提取TID
	tidElements := p.FindElements("a[href*='tid-']")
	if tidElements == nil || tidElements.Length() == 0 {
		return ""
	}

	// Cache DOM selections to avoid repeated Eq(i) calls
	elements := make([]*goquery.Selection, tidElements.Length())
	for i := 0; i < tidElements.Length(); i++ {
		elements[i] = tidElements.Eq(i)
	}

	for _, element := range elements {
		if href, exists := element.Attr("href"); exists {
			if strings.Contains(href, "tid-") {
				parts := strings.Split(href, "tid-")
				if len(parts) > 1 {
					tidPart := parts[1]
					matches := digitsPattern.FindStringSubmatch(tidPart)
					if len(matches) > 0 {
						return matches[1]
					}
				}
			}
		}
	}

	return ""
}
