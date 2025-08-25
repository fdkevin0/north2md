package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPFetcher HTTP抓取器接口
type HTTPFetcher interface {
	FetchPost(tid string) (string, error)
	FetchWithRetry(url string) (*http.Response, error)
	SetHeaders(headers map[string]string)
	SetTimeout(timeout time.Duration)
	LoadCookies(cookieFile string) error
	SaveCookies(cookieFile string) error
	SetCookie(cookie *CookieEntry)
	GetCookies(domain string) []*CookieEntry
	ClearCookies()
	FetchURL(url string) (string, error)
}

// DefaultHTTPFetcher 默认HTTP抓取器实现
type DefaultHTTPFetcher struct {
	client        *http.Client
	config        *HTTPOptions
	cookieManager CookieManager
	baseURL       string
}

// NewHTTPFetcher 创建新的HTTP抓取器
func NewHTTPFetcher(config *HTTPOptions, baseURL string) *DefaultHTTPFetcher {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	fetcher := &DefaultHTTPFetcher{
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
func (f *DefaultHTTPFetcher) FetchPost(tid string) (string, error) {
	if tid == "" {
		return "", fmt.Errorf("TID不能为空")
	}

	// 构建完整的URL
	postURL := f.buildPostURL(tid)
	
	return f.FetchURL(postURL)
}

// buildPostURL 构建帖子URL
func (f *DefaultHTTPFetcher) buildPostURL(tid string) string {
	// 确保baseURL以/结尾
	baseURL := strings.TrimRight(f.baseURL, "/")
	return fmt.Sprintf("%s/read.php?tid-%s.html", baseURL, tid)
}

// FetchURL 抓取指定URL的内容
func (f *DefaultHTTPFetcher) FetchURL(targetURL string) (string, error) {
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
func (f *DefaultHTTPFetcher) FetchWithRetry(targetURL string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= f.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 等待重试间隔
			time.Sleep(f.config.RetryDelay)
			fmt.Printf("重试第 %d 次请求: %s\n", attempt, targetURL)
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
func (f *DefaultHTTPFetcher) doRequest(targetURL string) (*http.Response, error) {
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

// SetHeaders 设置自定义请求头
func (f *DefaultHTTPFetcher) SetHeaders(headers map[string]string) {
	if f.config.CustomHeaders == nil {
		f.config.CustomHeaders = make(map[string]string)
	}

	for key, value := range headers {
		f.config.CustomHeaders[key] = value
	}
}

// SetTimeout 设置请求超时时间
func (f *DefaultHTTPFetcher) SetTimeout(timeout time.Duration) {
	f.config.Timeout = timeout
	f.client.Timeout = timeout
}

// LoadCookies 从文件加载Cookie
func (f *DefaultHTTPFetcher) LoadCookies(cookieFile string) error {
	if !f.config.EnableCookie {
		return nil
	}

	return f.cookieManager.LoadFromFile(cookieFile)
}

// SaveCookies 保存Cookie到文件
func (f *DefaultHTTPFetcher) SaveCookies(cookieFile string) error {
	if !f.config.EnableCookie {
		return nil
	}

	return f.cookieManager.SaveToFile(cookieFile)
}

// SetCookie 设置Cookie
func (f *DefaultHTTPFetcher) SetCookie(cookie *CookieEntry) {
	if f.config.EnableCookie {
		f.cookieManager.AddCookie(cookie)
	}
}

// GetCookies 获取指定域名的Cookie
func (f *DefaultHTTPFetcher) GetCookies(domain string) []*CookieEntry {
	if !f.config.EnableCookie {
		return nil
	}

	// 构建一个示例URL用于Cookie匹配
	testURL := fmt.Sprintf("https://%s/", domain)
	return f.cookieManager.GetCookiesForURL(testURL)
}

// ClearCookies 清除所有Cookie
func (f *DefaultHTTPFetcher) ClearCookies() {
	if f.config.EnableCookie {
		f.cookieManager.ClearCookies()
	}
}

// SetCookieFromString 从字符串设置Cookie
func (f *DefaultHTTPFetcher) SetCookieFromString(cookieStr, domain string) error {
	if !f.config.EnableCookie {
		return nil
	}

	return f.cookieManager.SetCookieFromString(cookieStr, domain, "/")
}

// GetCookieString 获取Cookie字符串
func (f *DefaultHTTPFetcher) GetCookieString(domain string) string {
	if !f.config.EnableCookie {
		return ""
	}

	return f.cookieManager.GetCookieString(domain)
}

// ValidateURL 验证URL是否有效
func (f *DefaultHTTPFetcher) ValidateURL(urlStr string) error {
	_, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("无效的URL: %v", err)
	}
	return nil
}

// GetBaseURL 获取基础URL
func (f *DefaultHTTPFetcher) GetBaseURL() string {
	return f.baseURL
}

// SetBaseURL 设置基础URL
func (f *DefaultHTTPFetcher) SetBaseURL(baseURL string) {
	f.baseURL = strings.TrimRight(baseURL, "/")
}