package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// LoginStatus 登录状态
type LoginStatus int

const (
	LoginStatusGuest   LoginStatus = iota // 游客（未登录）
	LoginStatusMember                     // 已登录
	LoginStatusUnknown                    // 未知状态
)

// String 登录状态字符串表示
func (s LoginStatus) String() string {
	switch s {
	case LoginStatusGuest:
		return "未登录"
	case LoginStatusMember:
		return "已登录"
	default:
		return "未知"
	}
}

// ValidationResult Cookie验证结果
type ValidationResult struct {
	IsValid      bool          `json:"is_valid"`      // Cookie是否有效
	LoginStatus  LoginStatus   `json:"login_status"`  // 登录状态
	TestURL      string        `json:"test_url"`      // 测试URL
	TestedAt     time.Time     `json:"tested_at"`     // 测试时间
	ResponseTime time.Duration `json:"response_time"` // 响应时间
	ErrorMessage string        `json:"error_message"` // 错误信息
	StatusCode   int           `json:"status_code"`   // HTTP状态码
	ContentLength int64        `json:"content_length"` // 内容长度
	RedirectURL  string        `json:"redirect_url"`  // 重定向URL
	HasLoginWall bool          `json:"has_login_wall"` // 是否有登录墙
}

// ValidationOptions 验证配置
type ValidationOptions struct {
	TestTimeout    time.Duration `json:"test_timeout"`    // 测试超时时间
	RetryCount     int           `json:"retry_count"`     // 重试次数
	TestUserAgent  string        `json:"test_user_agent"` // 测试用户代理
	EnableRedirect bool          `json:"enable_redirect"` // 是否跟随重定向
	MaxRedirects   int           `json:"max_redirects"`   // 最大重定向次数
}

// CookieValidator Cookie验证器接口
type CookieValidator interface {
	ValidateCookies(url string, cookies []*CookieEntry) (*ValidationResult, error)
	TestPageAccess(url string) (*ValidationResult, error)
	DetectLoginWall(htmlContent string) bool
	CheckLoginStatus(htmlContent string) LoginStatus
}

// DefaultCookieValidator 默认Cookie验证器实现
type DefaultCookieValidator struct {
	httpClient *http.Client
	config     *ValidationOptions
	cookies    []*CookieEntry
}

// NewCookieValidator 创建新的Cookie验证器
func NewCookieValidator(config *ValidationOptions) *DefaultCookieValidator {
	if config == nil {
		config = &ValidationOptions{
			TestTimeout:    30 * time.Second,
			RetryCount:     3,
			TestUserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
			EnableRedirect: true,
			MaxRedirects:   5,
		}
	}
	
	// 创建 HTTP 客户端
	httpClient := &http.Client{
		Timeout: config.TestTimeout,
	}
	
	// 配置重定向策略
	if !config.EnableRedirect {
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if config.MaxRedirects > 0 {
		httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= config.MaxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		}
	}
	
	return &DefaultCookieValidator{
		httpClient: httpClient,
		config:     config,
		cookies:    make([]*CookieEntry, 0),
	}
}

// ValidateCookies 验证Cookie有效性
func (v *DefaultCookieValidator) ValidateCookies(testURL string, cookies []*CookieEntry) (*ValidationResult, error) {
	startTime := time.Now()
	
	// 设置 cookies
	v.cookies = cookies
	
	result := &ValidationResult{
		TestURL:  testURL,
		TestedAt: startTime,
	}

	// 测试页面访问
	accessResult, err := v.TestPageAccess(testURL)
	if err != nil {
		result.ErrorMessage = err.Error()
		result.ResponseTime = time.Since(startTime)
		return result, err
	}

	// 复制访问结果
	*result = *accessResult
	result.ResponseTime = time.Since(startTime)

	// 判断Cookie是否有效：没有登录墙且状态为已登录
	if !result.HasLoginWall && result.LoginStatus == LoginStatusMember && result.StatusCode == 200 {
		result.IsValid = true
	} else {
		result.IsValid = false
		// 如果有登录墙，直接返回错误
		if result.HasLoginWall {
			return result, fmt.Errorf("检测到登录墙，需要登录才能访问")
		}
	}

	return result, nil
}

// TestPageAccess 测试页面访问
func (v *DefaultCookieValidator) TestPageAccess(testURL string) (*ValidationResult, error) {
	// 创建 HTTP 请求
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置 User-Agent
	if v.config.TestUserAgent != "" {
		req.Header.Set("User-Agent", v.config.TestUserAgent)
	}

	// 添加 Cookies
	for _, cookie := range v.cookies {
		req.AddCookie(&http.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Expires:  cookie.Expires,
			MaxAge:   cookie.MaxAge,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
		})
	}

	// 发送请求
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应内容失败: %v", err)
	}

	htmlContent := string(body)

	result := &ValidationResult{
		TestURL:       testURL,
		TestedAt:      time.Now(),
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
	}

	// 检查重定向
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if location := resp.Header.Get("Location"); location != "" {
			result.RedirectURL = location
		}
	}

	// 检测登录墙
	result.HasLoginWall = v.DetectLoginWall(htmlContent)
	
	// 检查登录状态
	result.LoginStatus = v.CheckLoginStatus(htmlContent)

	return result, nil
}

// DetectLoginWall 检测登录墙
func (v *DefaultCookieValidator) DetectLoginWall(htmlContent string) bool {
	// 检测标题中的登录提示 - 根据实际登录墙页面内容
	if strings.Contains(htmlContent, "只有注册会员才能进入") {
		return true
	}
	
	// 检测常见的登录墙提示
	loginWallPatterns := []string{
		`需要登录`,
		`请先登录`,
		`登录后查看`,
		`权限不足`,
		`访问被拒绝`,
		`您没有权限`,
		`请登录后访问`,
		`登录后才能查看`,
		`注册会员才能`,
		`会员专享`,
		`需要登录才能`,
		`本版块为正规版块`,
	}

	for _, pattern := range loginWallPatterns {
		if matched, _ := regexp.MatchString(pattern, htmlContent); matched {
			return true
		}
	}

	// 检测登录表单
	loginFormPattern := `<form[^>]*login[^>]*>`
	if matched, _ := regexp.MatchString(loginFormPattern, htmlContent); matched {
		return true
	}

	return false
}

// CheckLoginStatus 检查登录状态
func (v *DefaultCookieValidator) CheckLoginStatus(htmlContent string) LoginStatus {
	// 如果有登录墙，说明未登录
	if v.DetectLoginWall(htmlContent) {
		return LoginStatusGuest
	}

	// 检测已登录的标识
	loggedInPatterns := []string{
		`发表回复`,
		`快速回复`,
		`发表主题`,
		`个人资料`,
		`用户中心`,
		`退出登录`,
		`我的收藏`,
		`私信`,
		`签到`,
		`用户名`,
	}

	for _, pattern := range loggedInPatterns {
		if matched, _ := regexp.MatchString(pattern, htmlContent); matched {
			return LoginStatusMember
		}
	}

	// 检测帖子内容（如果能看到正常的帖子内容，说明已登录）
	contentPatterns := []string{
		`<div[^>]*id[^>]*read_`,  // 帖子内容div
		`class="f14"[^>]*read_`, // 帖子内容样式
		`楼主`,
		`层主`,
		`发表于`,
	}

	for _, pattern := range contentPatterns {
		if matched, _ := regexp.MatchString(pattern, htmlContent); matched {
			return LoginStatusMember
		}
	}

	return LoginStatusUnknown
}