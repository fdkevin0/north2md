package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DefaultDataExtractor 默认数据提取器实现
type DataExtractor struct {
	selectors *HTMLSelectors
}

// NewDataExtractor 创建新的数据提取器
func NewDataExtractor(selectors *HTMLSelectors) *DataExtractor {
	return &DataExtractor{
		selectors: selectors,
	}
}

// ExtractPost 提取完整的帖子数据
func (e *DataExtractor) ExtractPost(parser *HTMLParser) (*Post, error) {
	post := &Post{
		CreatedAt: time.Now(),
	}

	// 提取标题
	titleElement := parser.FindElement(e.selectors.Title)
	if titleElement.Length() > 0 {
		post.Title = strings.TrimSpace(titleElement.Text())
	}

	// 提取版块信息
	forumElement := parser.FindElement(e.selectors.Forum)
	if forumElement.Length() > 0 {
		post.Forum = e.extractForumName(forumElement)
	}

	// 设置URL
	baseURL := parser.GetBaseURL()
	if baseURL != "" {
		post.URL = baseURL
	}

	// 提取主楼内容
	mainPost, err := e.ExtractMainPost(parser)
	if err != nil {
		return nil, fmt.Errorf("提取主楼失败: %v", err)
	}
	post.MainPost = *mainPost
	post.CreatedAt = mainPost.PostTime

	// 提取回复
	replies, err := e.ExtractReplies(parser)
	if err != nil {
		return nil, fmt.Errorf("提取回复失败: %v", err)
	}
	post.Replies = replies

	return post, nil
}

// ExtractPostFromMultiplePages 从多个页面提取完整的帖子数据
func (e *DataExtractor) ExtractPostFromMultiplePages(parsers []*HTMLParser) (*Post, error) {
	if len(parsers) == 0 {
		return nil, fmt.Errorf("没有提供页面解析器")
	}

	// 使用第一页的数据初始化帖子
	post, err := e.ExtractPost(parsers[0])
	if err != nil {
		return nil, fmt.Errorf("提取第一页数据失败: %v", err)
	}

	// 从后续页面提取回复并追加到帖子中
	for i := 1; i < len(parsers); i++ {
		replies, err := e.ExtractReplies(parsers[i])
		if err != nil {
			fmt.Printf("提取第%d页回复失败: %v\n", i+1, err)
			continue
		}

		// 追加回复
		post.Replies = append(post.Replies, replies...)
	}

	// 更新总楼层数
	post.TotalFloors = 1 + len(post.Replies) // 主楼 + 回复数

	return post, nil
}

// ExtractMainPost 提取主楼内容
func (e *DataExtractor) ExtractMainPost(parser *HTMLParser) (*PostEntry, error) {
	// 查找第一个帖子表格
	postTables := parser.FindElements(e.selectors.PostTable)
	if postTables.Length() == 0 {
		return nil, fmt.Errorf("未找到帖子内容")
	}

	firstTable := postTables.First()
	return e.extractPostEntry(firstTable, "GF", parser.GetBaseURL())
}

// ExtractReplies 提取所有回复
func (e *DataExtractor) ExtractReplies(parser *HTMLParser) ([]PostEntry, error) {
	var replies []PostEntry

	// 查找所有帖子表格，跳过第一个（主楼）
	postTables := parser.FindElements(e.selectors.PostTable)

	for i := 1; i < postTables.Length(); i++ {
		table := postTables.Eq(i)
		floorNumber := e.generateFloorNumber(i)

		entry, err := e.extractPostEntry(table, floorNumber, parser.GetBaseURL())
		if err != nil {
			fmt.Printf("提取第%d楼失败: %v\n", i, err)
			continue
		}

		replies = append(replies, *entry)
	}

	return replies, nil
}

// extractPostEntry 提取单个帖子条目
func (e *DataExtractor) extractPostEntry(table Element, floor, baseURL string) (*PostEntry, error) {
	entry := &PostEntry{
		Floor: floor,
	}

	// 提取作者信息
	author, err := e.ExtractAuthor(table)
	if err != nil {
		fmt.Printf("提取作者信息失败: %v\n", err)
		author = &Author{} // 使用空的作者信息
	}
	entry.Author = *author

	// 提取发帖时间
	timeElement := table.Find(e.selectors.PostTime)
	if timeElement.Length() > 0 {
		entry.PostTime = e.parsePostTime(timeElement.Text())
	}

	// 提取帖子内容
	contentElement := table.Find(e.selectors.PostContent)
	if contentElement.Length() > 0 {
		entry.HTMLContent = contentElement.HTML()
		entry.Content = e.cleanTextContent(contentElement.Text())
	}

	// 提取帖子ID
	entry.PostID = e.extractPostID(table)

	// 提取图片和附件
	if contentElement.Length() > 0 {
		// 提取图片
		images, err := e.ExtractImages(contentElement.First(), baseURL)
		if err != nil {
			fmt.Printf("提取图片失败: %v\n", err)
		} else {
			entry.Images = images
		}

		// 提取附件
		attachments, err := e.ExtractAttachments(table, baseURL)
		if err != nil {
			fmt.Printf("提取附件失败: %v\n", err)
		} else {
			entry.Attachments = attachments
		}
	} else {
		// 如果没有内容元素，仍然尝试提取附件
		attachments, err := e.ExtractAttachments(table, baseURL)
		if err != nil {
			fmt.Printf("提取附件失败: %v\n", err)
		} else {
			entry.Attachments = attachments
		}
	}

	return entry, nil
}

// ExtractAuthor 提取作者信息
func (e *DataExtractor) ExtractAuthor(element Element) (*Author, error) {
	author := &Author{}

	// 提取用户名 - 查找strong标签
	usernameElement := element.Find("strong")
	if usernameElement.Length() > 0 {
		author.Username = strings.TrimSpace(usernameElement.Text())
	}

	// 提取UID - 从链接href中获取
	uidElement := element.Find("a[href*=\"u.php?uid\"]")
	if uidElement.Length() > 0 {
		if href, exists := uidElement.First().Attr("href"); exists {
			author.UID = e.extractUIDFromURL(href)
		}
	}

	// 提取头像
	avatarElement := element.Find("img[src*=\"avatar\"], img[src*=\"face\"]")
	if avatarElement.Length() > 0 {
		if src, exists := avatarElement.First().Attr("src"); exists {
			author.Avatar = src
		}
	}

	// 提取其他统计信息 - 从右侧信息区域
	infoElement := element.Find(".tiptop .tar, .authorinfo")
	if infoElement.Length() > 0 {
		infoText := infoElement.Text()
		author.PostCount = e.extractPostCount(infoText)
		author.RegisterDate = e.extractRegisterDate(infoText)
	}

	return author, nil
}

// ExtractImages 提取图片信息
func (e *DataExtractor) ExtractImages(element Element, baseURL string) ([]Image, error) {
	var images []Image

	// 查找所有图片
	imgElements := element.Find("img")

	imgElements.Each(func(i int, img Element) {
		src, exists := img.Attr("src")
		if !exists || src == "" {
			return
		}

		// 跳过头像、表情等小图片
		if e.isSkippableImage(src) {
			return
		}

		image := Image{
			URL: e.resolveURL(src, baseURL),
		}

		// 提取alt属性
		if alt, exists := img.Attr("alt"); exists {
			image.Alt = alt
		}

		// 检查是否为附件图片
		image.IsAttachment = e.isAttachmentImage(img)

		images = append(images, image)
	})

	return images, nil
}

// ExtractAttachments 提取附件信息
func (e *DataExtractor) ExtractAttachments(element Element, baseURL string) ([]Attachment, error) {
	var attachments []Attachment

	// 查找附件链接
	attachElements := element.Find("a[href*=\"attachment\"], a[href*=\"download\"]")

	attachElements.Each(func(i int, link Element) {
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}

		attachment := Attachment{
			URL:      e.resolveURL(href, baseURL),
			FileName: e.extractFileNameFromURL(href),
		}

		// 从链接文本中提取文件名
		linkText := strings.TrimSpace(link.Text())
		if linkText != "" && !strings.Contains(linkText, "http") {
			attachment.FileName = linkText
		}

		// 尝试提取文件大小信息
		parent := link.Parent()
		if parent.Length() > 0 {
			parentText := parent.Text()
			attachment.FileSize = e.extractFileSize(parentText)
		}

		attachments = append(attachments, attachment)
	})

	return attachments, nil
}

// 辅助方法

// extractForumName 提取版块名称
func (e *DataExtractor) extractForumName(element Element) string {
	// 从导航链接中提取版块名称
	text := element.Text()

	// 通常版块名称在导航的最后一个链接中
	parts := strings.Split(text, "»")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-1])
	}

	return strings.TrimSpace(text)
}

// generateFloorNumber 生成楼层编号
func (e *DataExtractor) generateFloorNumber(index int) string {
	if index == 0 {
		return "GF"
	}
	return fmt.Sprintf("B%dF", index)
}

// parsePostTime 解析发帖时间
func (e *DataExtractor) parsePostTime(timeText string) time.Time {
	timeText = strings.TrimSpace(timeText)

	// 尝试多种时间格式
	formats := []string{
		"2006-1-2 15:04:05",
		"2006-01-02 15:04:05",
		"2006/1/2 15:04:05",
		"2006/01/02 15:04:05",
		"1-2 15:04",
		"01-02 15:04",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeText); err == nil {
			// 如果解析成功但没有年份，使用当前年份
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
func (e *DataExtractor) extractPostID(element Element) string {
	// 尝试从read_xxx id中提取
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
func (e *DataExtractor) extractUIDFromURL(url string) string {
	re := regexp.MustCompile(`uid[=-](\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractPostCount 从文本中提取发帖数
func (e *DataExtractor) extractPostCount(text string) int {
	re := regexp.MustCompile(`帖子[：:]?\s*(\d+)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if count, err := strconv.Atoi(matches[1]); err == nil {
			return count
		}
	}
	return 0
}

// extractRegisterDate 从文本中提取注册时间
func (e *DataExtractor) extractRegisterDate(text string) string {
	re := regexp.MustCompile(`注册[：:]?\s*(\d{4}-\d{1,2}-\d{1,2})`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// isSkippableImage 检查是否应跳过的图片
func (e *DataExtractor) isSkippableImage(src string) bool {
	skipPatterns := []string{
		"avatar", "face", "icon", "smile", "emotion",
		"star", "level", "rank", "medal",
	}

	lowerSrc := strings.ToLower(src)
	for _, pattern := range skipPatterns {
		if strings.Contains(lowerSrc, pattern) {
			return true
		}
	}

	return false
}

// isAttachmentImage 检查是否为附件图片
func (e *DataExtractor) isAttachmentImage(img Element) bool {
	// 检查父元素是否包含附件相关信息
	parent := img.Parent()
	for i := 0; i < 3 && parent.Length() > 0; i++ {
		parentHTML := strings.ToLower(parent.HTML())
		if strings.Contains(parentHTML, "attachment") ||
			strings.Contains(parentHTML, "attach") {
			return true
		}
		parent = parent.Parent()
	}

	// 检查src是否包含attachment
	if src, exists := img.Attr("src"); exists {
		return strings.Contains(strings.ToLower(src), "attachment")
	}

	return false
}

// extractFileNameFromURL 从URL中提取文件名
func (e *DataExtractor) extractFileNameFromURL(url string) string {
	// 从URL路径中提取文件名
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]

		// 去除查询参数
		if idx := strings.Index(filename, "?"); idx != -1 {
			filename = filename[:idx]
		}

		return filename
	}
	return ""
}

// extractFileSize 从文本中提取文件大小
func (e *DataExtractor) extractFileSize(text string) int64 {
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(KB|MB|GB|B)`)
	matches := re.FindStringSubmatch(strings.ToUpper(text))
	if len(matches) < 3 {
		return 0
	}

	size, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := matches[2]
	switch unit {
	case "KB":
		return int64(size * 1024)
	case "MB":
		return int64(size * 1024 * 1024)
	case "GB":
		return int64(size * 1024 * 1024 * 1024)
	default:
		return int64(size)
	}
}

// cleanTextContent 清理文本内容
func (e *DataExtractor) cleanTextContent(text string) string {
	// 移除多余的空白字符
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// 移除前后空白
	text = strings.TrimSpace(text)

	return text
}

// resolveURL 解析URL
func (e *DataExtractor) resolveURL(relativeURL, baseURL string) string {
	// 使用parser中的URL解析逻辑
	// 创建一个临时parser来处理URL解析
	parser := NewHTMLParser()
	parser.SetBaseURL(baseURL)
	return parser.ResolveURL(relativeURL)
}
