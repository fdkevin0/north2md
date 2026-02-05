package north2md

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/samber/lo"
)

// CurlCommand 表示解析后的 curl 命令
type CurlCommand struct {
	URL     string            `toml:"url"`     // 目标URL
	Headers map[string]string `toml:"headers"` // HTTP请求头
	Cookies string            `toml:"cookies"` // Cookie字符串
	Method  string            `toml:"method"`  // HTTP方法
	Data    string            `toml:"data"`    // POST数据
}

// CurlImportOptions curl 导入配置
type CurlImportOptions struct {
	OverwriteExisting bool     `toml:"overwrite_existing"` // 是否覆盖已存在的cookie
	AutoInferDomain   bool     `toml:"auto_infer_domain"`  // 是否自动推断域名
	AutoInferPath     bool     `toml:"auto_infer_path"`    // 是否自动推断路径
	DefaultExpiry     int      `toml:"default_expiry"`     // 默认过期时间(小时)
	FilterPatterns    []string `toml:"filter_patterns"`    // 过滤模式
}

// CookieManager Cookie管理器
type CookieManager struct {
	jar *CookieJar
}

// NewCookieManager 创建新的Cookie管理器
func NewCookieManager() *CookieManager {
	return &CookieManager{
		jar: &CookieJar{
			Cookies:     make([]CookieEntry, 0),
			LastUpdated: time.Now(),
		},
	}
}

// LoadFromFile 从文件加载Cookie
func (cm *CookieManager) LoadFromFile(filepath string) error {
	if filepath == "" {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		// 文件不存在，创建空的Cookie文件
		return cm.SaveToFile(filepath)
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("读取Cookie文件失败: %v", err)
	}

	if len(data) == 0 {
		// 空文件，使用默认值
		return nil
	}

	err = toml.Unmarshal(data, cm.jar)
	if err != nil {
		// TOML解析失败，备份旧文件后重建
		backupPath := filepath + ".backup." + time.Now().Format("20060102_150405")
		os.Rename(filepath, backupPath)
		return cm.SaveToFile(filepath)
	}

	// 清理过期Cookie
	cm.CleanExpired()

	if !lo.ContainsBy(cm.jar.Cookies, func(item CookieEntry) bool {
		return item.Name == "eb9e6_winduser"
	}) {
		slog.Warn("User not logged in, clearing cookies")
		cm.ClearCookies()
	}

	return nil
}

// SaveToFile 保存Cookie到文件
func (cm *CookieManager) SaveToFile(filepath string) error {
	if filepath == "" {
		return nil
	}

	cm.jar.LastUpdated = time.Now()

	// 清理过期Cookie
	cm.CleanExpired()

	tomlData, err := toml.Marshal(cm.jar)
	if err != nil {
		return fmt.Errorf("序列化Cookie失败: %v", err)
	}

	err = os.WriteFile(filepath, tomlData, 0600)
	if err != nil {
		return fmt.Errorf("写入Cookie文件失败: %v", err)
	}

	return nil
}

// AddCookie 添加Cookie
func (cm *CookieManager) AddCookie(cookie *CookieEntry) {
	if cookie == nil {
		return
	}

	// 查找是否已存在相同的Cookie（相同name、domain、path）
	for i, existingCookie := range cm.jar.Cookies {
		if existingCookie.Name == cookie.Name &&
			existingCookie.Domain == cookie.Domain &&
			existingCookie.Path == cookie.Path {
			// 更新现有Cookie
			cm.jar.Cookies[i] = *cookie
			return
		}
	}

	// 添加新Cookie
	cm.jar.Cookies = append(cm.jar.Cookies, *cookie)
}

// GetCookiesForURL 获取指定URL适用的Cookie
func (cm *CookieManager) GetCookiesForURL(urlStr string) []*CookieEntry {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}

	var result []*CookieEntry
	for i, cookie := range cm.jar.Cookies {
		if cm.isCookieApplicable(&cookie, u) {
			result = append(result, &cm.jar.Cookies[i])
		}
	}

	return result
}

// isCookieApplicable 检查Cookie是否适用于指定URL
func (cm *CookieManager) isCookieApplicable(cookie *CookieEntry, u *url.URL) bool {
	// 检查过期时间
	if !cookie.Expires.IsZero() && cookie.Expires.Before(time.Now()) {
		return false
	}

	// 检查域名匹配
	if !cm.domainMatches(cookie.Domain, u.Host) {
		return false
	}

	// 检查路径匹配
	if !cm.pathMatches(cookie.Path, u.Path) {
		return false
	}

	// 检查HTTPS限制
	if cookie.Secure && u.Scheme != "https" {
		return false
	}

	return true
}

// domainMatches 检查域名是否匹配
func (cm *CookieManager) domainMatches(cookieDomain, host string) bool {
	if cookieDomain == "" {
		return true
	}

	// 完全匹配
	if cookieDomain == host {
		return true
	}

	// 域名匹配（支持子域名）
	if strings.HasPrefix(cookieDomain, ".") {
		return strings.HasSuffix(host, cookieDomain) || host == cookieDomain[1:]
	}

	return false
}

// pathMatches 检查路径是否匹配
func (cm *CookieManager) pathMatches(cookiePath, urlPath string) bool {
	if cookiePath == "" || cookiePath == "/" {
		return true
	}

	// 确保路径以/开头
	if !strings.HasPrefix(cookiePath, "/") {
		cookiePath = "/" + cookiePath
	}

	return strings.HasPrefix(urlPath, cookiePath)
}

// UpdateFromResponse 从HTTP响应更新Cookie
func (cm *CookieManager) UpdateFromResponse(resp *http.Response) {
	if resp == nil {
		return
	}

	cookies := resp.Cookies()
	for _, httpCookie := range cookies {
		cookie := &CookieEntry{
			Name:     httpCookie.Name,
			Value:    httpCookie.Value,
			Domain:   httpCookie.Domain,
			Path:     httpCookie.Path,
			Expires:  httpCookie.Expires,
			MaxAge:   httpCookie.MaxAge,
			Secure:   httpCookie.Secure,
			HttpOnly: httpCookie.HttpOnly,
		}

		// 处理SameSite属性
		switch httpCookie.SameSite {
		case http.SameSiteDefaultMode:
			cookie.SameSite = "Default"
		case http.SameSiteLaxMode:
			cookie.SameSite = "Lax"
		case http.SameSiteStrictMode:
			cookie.SameSite = "Strict"
		case http.SameSiteNoneMode:
			cookie.SameSite = "None"
		}

		// 如果没有设置域名，使用响应的Host
		if cookie.Domain == "" {
			if resp.Request != nil && resp.Request.URL != nil {
				cookie.Domain = resp.Request.URL.Host
			}
		}

		// 如果没有设置路径，使用默认路径
		if cookie.Path == "" {
			cookie.Path = "/"
		}

		cm.AddCookie(cookie)
	}
}

// CleanExpired 清理过期Cookie
func (cm *CookieManager) CleanExpired() {
	now := time.Now()

	// Pre-allocate slice with current capacity to reduce allocations
	validCookies := make([]CookieEntry, 0, len(cm.jar.Cookies))

	for _, cookie := range cm.jar.Cookies {
		// 检查是否过期
		if !cookie.Expires.IsZero() && cookie.Expires.Before(now) {
			continue // 跳过过期Cookie
		}

		validCookies = append(validCookies, cookie)
	}

	cm.jar.Cookies = validCookies
}

// ClearCookies 清除所有Cookie
func (cm *CookieManager) ClearCookies() {
	cm.jar.Cookies = make([]CookieEntry, 0)
}
