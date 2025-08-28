package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Fetcher HTTP抓取器
type Fetcher struct {
	client        *http.Client
	config        *HTTPOptions
	cookieManager *CookieManager
	baseURL       string
}

// configureProxy 从环境变量配置代理
func configureProxy() *http.Transport {
	// 优先检查 HTTPS_PROXY，然后是 HTTP_PROXY
	proxyURL := os.Getenv("HTTPS_PROXY")
	if proxyURL == "" {
		proxyURL = os.Getenv("HTTP_PROXY")
	}

	if proxyURL == "" {
		return nil // 没有配置代理
	}

	// 解析代理 URL
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		slog.Warn("Invalid proxy URL detected", "proxy", proxyURL, "error", err)
		return nil
	}

	// 获取 NO_PROXY 列表
	noProxy := os.Getenv("NO_PROXY")

	// 创建带代理的 Transport
	transport := &http.Transport{
		Proxy: http.ProxyURL(parsedURL),
	}

	// 如果有 NO_PROXY，设置代理忽略规则
	if noProxy != "" {
		slog.Warn("Using proxy with bypass rules", "proxy", proxyURL, "no_proxy", noProxy)
	} else {
		slog.Warn("Using proxy server", "proxy", proxyURL)
	}

	return transport
}

// NewHTTPFetcher 创建新的HTTP抓取器
func NewHTTPFetcher(config *HTTPOptions, baseURL string) *Fetcher {
	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// 配置代理
	transport := configureProxy()
	if transport != nil {
		client.Transport = transport
	}

	fetcher := &Fetcher{
		client:        client,
		config:        config,
		cookieManager: NewCookieManager(),
		baseURL:       baseURL,
	}

	// 加载Cookie
	if config.EnableCookie && config.CookieFile != "" {
		fetcher.LoadCookies(config.CookieFile)
	}

	return fetcher
}

// FetchPost 抓取指定TID的帖子内容
func (f *Fetcher) FetchPost(tid string) (string, error) {
	if tid == "" {
		return "", fmt.Errorf("TID不能为空")
	}

	// 构建完整的URL
	postURL := f.buildPostURL(tid, 1) // 第一页

	return f.FetchURL(postURL)
}

// buildPostURL 构建帖子URL
func (f *Fetcher) buildPostURL(tid string, page int) string {
	// 确保baseURL以/结尾
	baseURL := strings.TrimRight(f.baseURL, "/")

	// 如果是第一页，使用原始URL格式
	if page <= 1 {
		return fmt.Sprintf("%s/read.php?tid-%s.html", baseURL, tid)
	}

	// 对于其他页，添加页码参数
	return fmt.Sprintf("%s/read.php?tid-%s-page-%d.html", baseURL, tid, page)
}

// FetchPostWithPage 抓取指定TID和页码的帖子内容
func (f *Fetcher) FetchPostWithPage(tid string, page int) (string, error) {
	if tid == "" {
		return "", fmt.Errorf("TID不能为空")
	}

	slog.Info("Fetching post", "tid", tid, "page", page)

	// 构建完整的URL，包含页码参数
	postURL := f.buildPostURL(tid, page)

	return f.FetchURL(postURL)
}

// FetchURL 抓取指定URL的内容
func (f *Fetcher) FetchURL(targetURL string) (string, error) {
	resp, err := f.FetchWithRetry(targetURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应内容失败: %v", err)
	}

	// 更新Cookie
	if f.config.EnableCookie {
		f.cookieManager.UpdateFromResponse(resp)
		// 保存Cookie到文件
		if f.config.CookieFile != "" {
			f.SaveCookies(f.config.CookieFile)
		}
	}

	return string(body), nil
}

// FetchWithRetry 带重试机制的HTTP请求
func (f *Fetcher) FetchWithRetry(targetURL string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= f.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 等待重试间隔
			time.Sleep(f.config.RetryDelay)
			slog.Info("Retrying request", "attempt", attempt, "url", targetURL)
		}

		resp, err := f.doRequest(targetURL)
		if err != nil {
			lastErr = err
			// 网络错误，继续重试
			continue
		}

		// 检查HTTP状态码
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		// 4xx错误不重试
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP错误 %d: %s", resp.StatusCode, resp.Status)
		}

		// 5xx错误继续重试
		resp.Body.Close()
		lastErr = fmt.Errorf("服务器错误 %d: %s", resp.StatusCode, resp.Status)

		// 5xx错误时增加重试间隔
		if resp.StatusCode >= 500 {
			time.Sleep(f.config.RetryDelay)
		}
	}

	return nil, fmt.Errorf("请求失败，已重试 %d 次: %v", f.config.MaxRetries, lastErr)
}

// doRequest 执行单个HTTP请求
func (f *Fetcher) doRequest(targetURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置User-Agent
	if f.config.UserAgent != "" {
		req.Header.Set("User-Agent", f.config.UserAgent)
	}

	// 设置自定义请求头
	for key, value := range f.config.CustomHeaders {
		req.Header.Set(key, value)
	}

	// 添加Cookie
	if f.config.EnableCookie {
		cookies := f.cookieManager.GetCookiesForURL(targetURL)
		for _, cookie := range cookies {
			httpCookie := &http.Cookie{
				Name:     cookie.Name,
				Value:    cookie.Value,
				Domain:   cookie.Domain,
				Path:     cookie.Path,
				Expires:  cookie.Expires,
				MaxAge:   cookie.MaxAge,
				Secure:   cookie.Secure,
				HttpOnly: cookie.HttpOnly,
			}

			// 处理SameSite属性
			switch cookie.SameSite {
			case "Lax":
				httpCookie.SameSite = http.SameSiteLaxMode
			case "Strict":
				httpCookie.SameSite = http.SameSiteStrictMode
			case "None":
				httpCookie.SameSite = http.SameSiteNoneMode
			default:
				httpCookie.SameSite = http.SameSiteDefaultMode
			}

			req.AddCookie(httpCookie)
		}
	}

	// 执行请求
	return f.client.Do(req)
}

// LoadCookies 从文件加载Cookie
func (f *Fetcher) LoadCookies(cookieFile string) error {
	if !f.config.EnableCookie {
		return nil
	}

	return f.cookieManager.LoadFromFile(cookieFile)
}

// SaveCookies 保存Cookie到文件
func (f *Fetcher) SaveCookies(cookieFile string) error {
	if !f.config.EnableCookie {
		return nil
	}

	return f.cookieManager.SaveToFile(cookieFile)
}

// FetchPostWithPagination 获取指定TID的帖子（自动处理分页）
func (f *Fetcher) FetchPostWithPagination(tid string, postParser *PostParser, selectors *HTMLSelectors) (*Post, error) {
	// 首先获取第一页以确定总页数
	firstPageHTML, err := f.FetchPost(tid)
	if err != nil {
		return nil, fmt.Errorf("获取帖子第一页失败: %v", err)
	}

	// 解析第一页
	if err := postParser.LoadFromString(firstPageHTML); err != nil {
		return nil, fmt.Errorf("解析第一页HTML失败: %v", err)
	}

	// 尝试从第一页获取总页数
	totalPages := f.extractTotalPages(postParser)
	if totalPages <= 0 {
		// 如果无法提取总页数，默认为1页
		totalPages = 1
	}

	// 收集所有页面的解析器
	var parsers []*PostParser

	// 添加第一页解析器
	parsers = append(parsers, postParser)

	// 获取剩余页面
	for page := 2; page <= totalPages; page++ {
		pageHTML, err := f.FetchPostWithPage(tid, page)
		if err != nil {
			slog.Error("Failed to fetch post page", "page", page, "error", err)
			continue
		}

		// 创建新的解析器实例
		pageParser := NewPostParser(selectors)
		if err := pageParser.LoadFromString(pageHTML); err != nil {
			slog.Error("Failed to parse HTML for page", "page", page, "error", err)
			continue
		}

		parsers = append(parsers, pageParser)
	}

	// 从所有页面提取数据
	// Use the first parser to extract data from all parsers
	post, err := parsers[0].ExtractPostFromMultiplePages(parsers)
	if err != nil {
		return nil, fmt.Errorf("从多页提取帖子数据失败: %v", err)
	}

	// 设置TID
	post.TID = tid

	return post, nil
}

// extractTotalPages 从页面中提取总页数
func (f *Fetcher) extractTotalPages(parser *PostParser) int {
	// 查找包含页数信息的元素
	// 根据示例HTML，页数信息在 "Pages: 1/8" 格式中
	pagesElement := parser.FindElement(".pagesone")
	if pagesElement.Length() > 0 {
		text := pagesElement.Text()
		// 使用正则表达式提取总页数
		re := regexp.MustCompile(`Pages:\s*\d+/(\d+)`)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			if totalPages, err := strconv.Atoi(matches[1]); err == nil {
				return totalPages
			}
		}
	}

	// 如果上面的方法失败，尝试查找页面链接中的最大页码
	pageLinks := parser.FindElements("a[href*='page-']")
	maxPage := 0
	if pageLinks != nil {
		pageLinks.Each(func(i int, element *goquery.Selection) {
			href, exists := element.Attr("href")
			if !exists {
				return
			}

			// 使用正则表达式提取页码
			re := regexp.MustCompile(`page-(\d+)`)
			matches := re.FindStringSubmatch(href)
			if len(matches) > 1 {
				if page, err := strconv.Atoi(matches[1]); err == nil && page > maxPage {
					maxPage = page
				}
			}
		})
	}

	return maxPage
}
