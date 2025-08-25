package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// CurlCommand 表示解析后的 curl 命令
type CurlCommand struct {
	URL     string            `json:"url"`      // 目标URL
	Headers map[string]string `json:"headers"`  // HTTP请求头
	Cookies string            `json:"cookies"`  // Cookie字符串
	Method  string            `json:"method"`   // HTTP方法
	Data    string            `json:"data"`     // POST数据
}

// CurlImportOptions curl 导入配置
type CurlImportOptions struct {
	OverwriteExisting bool     `json:"overwrite_existing"` // 是否覆盖已存在的cookie
	AutoInferDomain   bool     `json:"auto_infer_domain"`  // 是否自动推断域名
	AutoInferPath     bool     `json:"auto_infer_path"`    // 是否自动推断路径
	DefaultExpiry     int      `json:"default_expiry"`     // 默认过期时间(小时)
	FilterPatterns    []string `json:"filter_patterns"`   // 过滤模式
}

// CurlParser curl命令解析器接口
type CurlParser interface {
	ParseCommand(curlCmd string) (*CurlCommand, error)
	ParseFromFile(filePath string) ([]*CurlCommand, error)
	ExtractCookies(curlCmd *CurlCommand) ([]*CookieEntry, error)
	ValidateCommand(curlCmd string) error
}

// DefaultCurlParser 默认curl解析器实现
type DefaultCurlParser struct {
	options *CurlImportOptions
}

// CookieManager Cookie管理接口
type CookieManager interface {
	LoadFromFile(filepath string) error
	SaveToFile(filepath string) error
	AddCookie(cookie *CookieEntry)
	GetCookiesForURL(urlStr string) []*CookieEntry
	UpdateFromResponse(resp *http.Response)
	CleanExpired()
	SetCookieFromString(cookieStr, domain, path string) error
	GetCookieString(domain string) string
	ClearCookies()
}

// DefaultCookieManager 默认Cookie管理器实现
type DefaultCookieManager struct {
	jar *CookieJar
}

// NewCookieManager 创建新的Cookie管理器
func NewCookieManager() *DefaultCookieManager {
	return &DefaultCookieManager{
		jar: &CookieJar{
			Cookies:     make([]CookieEntry, 0),
			LastUpdated: time.Now(),
		},
	}
}

// LoadFromFile 从文件加载Cookie
func (cm *DefaultCookieManager) LoadFromFile(filepath string) error {
	if filepath == "" {
		return nil
	}

	cm.jar.FilePath = filepath

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

	err = cm.jar.FromJSON(string(data))
	if err != nil {
		// JSON解析失败，备份旧文件后重建
		backupPath := filepath + ".backup." + time.Now().Format("20060102_150405")
		os.Rename(filepath, backupPath)
		return cm.SaveToFile(filepath)
	}

	// 清理过期Cookie
	cm.CleanExpired()

	return nil
}

// SaveToFile 保存Cookie到文件
func (cm *DefaultCookieManager) SaveToFile(filepath string) error {
	if filepath == "" {
		return nil
	}

	cm.jar.FilePath = filepath
	cm.jar.LastUpdated = time.Now()

	// 清理过期Cookie
	cm.CleanExpired()

	jsonData, err := cm.jar.ToJSON()
	if err != nil {
		return fmt.Errorf("序列化Cookie失败: %v", err)
	}

	err = os.WriteFile(filepath, []byte(jsonData), 0600)
	if err != nil {
		return fmt.Errorf("写入Cookie文件失败: %v", err)
	}

	return nil
}

// AddCookie 添加Cookie
func (cm *DefaultCookieManager) AddCookie(cookie *CookieEntry) {
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
func (cm *DefaultCookieManager) GetCookiesForURL(urlStr string) []*CookieEntry {
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
func (cm *DefaultCookieManager) isCookieApplicable(cookie *CookieEntry, u *url.URL) bool {
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
func (cm *DefaultCookieManager) domainMatches(cookieDomain, host string) bool {
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
func (cm *DefaultCookieManager) pathMatches(cookiePath, urlPath string) bool {
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
func (cm *DefaultCookieManager) UpdateFromResponse(resp *http.Response) {
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
func (cm *DefaultCookieManager) CleanExpired() {
	now := time.Now()
	var validCookies []CookieEntry

	for _, cookie := range cm.jar.Cookies {
		// 检查是否过期
		if !cookie.Expires.IsZero() && cookie.Expires.Before(now) {
			continue // 跳过过期Cookie
		}

		// 检查MaxAge
		if cookie.MaxAge > 0 {
			// 这里需要Cookie的创建时间，但我们没有存储，所以暂时保留
			// 在实际实现中，可能需要添加CreatedAt字段
		}

		validCookies = append(validCookies, cookie)
	}

	cm.jar.Cookies = validCookies
}

// SetCookieFromString 从字符串设置Cookie
func (cm *DefaultCookieManager) SetCookieFromString(cookieStr, domain, path string) error {
	// 解析Cookie字符串，格式："name=value; name2=value2"
	pairs := strings.Split(cookieStr, ";")
	
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if name == "" {
			continue
		}

		cookie := &CookieEntry{
			Name:   name,
			Value:  value,
			Domain: domain,
			Path:   path,
		}

		cm.AddCookie(cookie)
	}

	return nil
}

// GetCookieString 获取指定域名的Cookie字符串
func (cm *DefaultCookieManager) GetCookieString(domain string) string {
	var cookies []string

	for _, cookie := range cm.jar.Cookies {
		if cm.domainMatches(cookie.Domain, domain) {
			cookies = append(cookies, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
		}
	}

	return strings.Join(cookies, "; ")
}

// GetAllCookies 获取所有Cookie
func (cm *DefaultCookieManager) GetAllCookies() []CookieEntry {
	return cm.jar.Cookies
}

// ClearCookies 清除所有Cookie
func (cm *DefaultCookieManager) ClearCookies() {
	cm.jar.Cookies = make([]CookieEntry, 0)
}

// GetCookieCount 获取Cookie数量
func (cm *DefaultCookieManager) GetCookieCount() int {
	return len(cm.jar.Cookies)
}

// NewCurlParser 创建新的 curl 解析器
func NewCurlParser(options *CurlImportOptions) *DefaultCurlParser {
	if options == nil {
		options = &CurlImportOptions{
			OverwriteExisting: false,
			AutoInferDomain:   true,
			AutoInferPath:     true,
			DefaultExpiry:     24 * 7, // 7天
			FilterPatterns:    []string{},
		}
	}
	return &DefaultCurlParser{options: options}
}

// ParseCommand 解析 curl 命令
func (p *DefaultCurlParser) ParseCommand(curlCmd string) (*CurlCommand, error) {
	if curlCmd == "" {
		return nil, fmt.Errorf("空的 curl 命令")
	}

	// 清理换行符和反斜杠
	curlCmd = strings.ReplaceAll(curlCmd, "\\", " ")
	curlCmd = strings.ReplaceAll(curlCmd, "\n", " ")
	curlCmd = regexp.MustCompile(`\s+`).ReplaceAllString(curlCmd, " ")
	curlCmd = strings.TrimSpace(curlCmd)

	// 检查是否以 curl 开头
	if !strings.HasPrefix(curlCmd, "curl ") {
		return nil, fmt.Errorf("无效的 curl 命令，必须以 'curl ' 开头")
	}

	cmd := &CurlCommand{
		Headers: make(map[string]string),
		Method:  "GET",
	}

	// 1. 提取 URL
	if err := p.extractURL(curlCmd, cmd); err != nil {
		return nil, fmt.Errorf("提取 URL 失败: %v", err)
	}

	// 2. 提取 Headers
	if err := p.extractHeaders(curlCmd, cmd); err != nil {
		return nil, fmt.Errorf("提取 Headers 失败: %v", err)
	}

	// 3. 提取 Cookies
	if err := p.extractCookies(curlCmd, cmd); err != nil {
		return nil, fmt.Errorf("提取 Cookies 失败: %v", err)
	}

	// 4. 提取 HTTP 方法
	p.extractMethod(curlCmd, cmd)

	// 5. 提取 POST 数据
	p.extractData(curlCmd, cmd)

	return cmd, nil
}

// extractURL 提取 URL
func (p *DefaultCurlParser) extractURL(curlCmd string, cmd *CurlCommand) error {
	// 匹配 URL，支持单引号、双引号和无引号
	urlPatterns := []string{
		`curl\s+'([^']+)'`, // 单引号
		`curl\s+"([^"]+)"`, // 双引号
		`curl\s+([^\s-]+)`, // 无引号
	}

	for _, pattern := range urlPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(curlCmd)
		if len(matches) > 1 {
			cmd.URL = strings.TrimSpace(matches[1])
			// 验证 URL 格式
			if _, err := url.Parse(cmd.URL); err != nil {
				return fmt.Errorf("无效的 URL 格式: %s", cmd.URL)
			}
			return nil
		}
	}

	return fmt.Errorf("未找到 URL")
}

// extractHeaders 提取 HTTP 头
func (p *DefaultCurlParser) extractHeaders(curlCmd string, cmd *CurlCommand) error {
	// 匹配 -H 参数
	headerPattern := `-H\s+['"]([^'"]+)['"]`
	re := regexp.MustCompile(headerPattern)
	matches := re.FindAllStringSubmatch(curlCmd, -1)

	for _, match := range matches {
		if len(match) > 1 {
			headerStr := match[1]
			// 解析头部格式 "Key: Value"
			parts := strings.SplitN(headerStr, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				cmd.Headers[key] = value
			}
		}
	}

	return nil
}

// extractCookies 提取 Cookies
func (p *DefaultCurlParser) extractCookies(curlCmd string, cmd *CurlCommand) error {
	// 匹配 -b 参数
	cookiePattern := `-b\s+['"]([^'"]+)['"]`
	re := regexp.MustCompile(cookiePattern)
	matches := re.FindStringSubmatch(curlCmd)

	if len(matches) > 1 {
		cmd.Cookies = matches[1]
	}

	return nil
}

// extractMethod 提取 HTTP 方法
func (p *DefaultCurlParser) extractMethod(curlCmd string, cmd *CurlCommand) {
	// 匹配 -X 参数
	methodPattern := `-X\s+([A-Z]+)`
	re := regexp.MustCompile(methodPattern)
	matches := re.FindStringSubmatch(curlCmd)

	if len(matches) > 1 {
		cmd.Method = matches[1]
	} else if strings.Contains(curlCmd, "-d ") || strings.Contains(curlCmd, "--data") {
		// 如果有 -d 参数，默认为 POST
		cmd.Method = "POST"
	}
}

// extractData 提取 POST 数据
func (p *DefaultCurlParser) extractData(curlCmd string, cmd *CurlCommand) {
	// 匹配 -d 参数
	dataPattern := `-d\s+['"]([^'"]+)['"]`
	re := regexp.MustCompile(dataPattern)
	matches := re.FindStringSubmatch(curlCmd)

	if len(matches) > 1 {
		cmd.Data = matches[1]
	}
}

// ParseFromFile 从文件解析 curl 命令
func (p *DefaultCurlParser) ParseFromFile(filePath string) ([]*CurlCommand, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	
	var commands []*CurlCommand
	var currentCmd strings.Builder
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// 如果是新的 curl 命令
		if strings.HasPrefix(line, "curl ") {
			// 处理上一个命令
			if currentCmd.Len() > 0 {
				cmd, err := p.ParseCommand(currentCmd.String())
				if err == nil {
					commands = append(commands, cmd)
				}
				currentCmd.Reset()
			}
			currentCmd.WriteString(line)
		} else {
			// 继续上一个命令
			if currentCmd.Len() > 0 {
				currentCmd.WriteString(" ")
				currentCmd.WriteString(line)
			}
		}
	}
	
	// 处理最后一个命令
	if currentCmd.Len() > 0 {
		cmd, err := p.ParseCommand(currentCmd.String())
		if err == nil {
			commands = append(commands, cmd)
		}
	}
	
	return commands, nil
}

// ExtractCookies 从 CurlCommand 提取 Cookie 列表
func (p *DefaultCurlParser) ExtractCookies(curlCmd *CurlCommand) ([]*CookieEntry, error) {
	if curlCmd == nil || curlCmd.Cookies == "" {
		return []*CookieEntry{}, nil
	}

	// 解析 URL 获取域名和路径
	u, err := url.Parse(curlCmd.URL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %v", err)
	}

	domain := u.Host
	path := u.Path
	if path == "" {
		path = "/"
	}

	// 解析 Cookie 字符串
	var cookies []*CookieEntry
	cookiePairs := strings.Split(curlCmd.Cookies, ";")

	for _, pair := range cookiePairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if name == "" {
			continue
		}

		// 创建 CookieEntry
		cookie := &CookieEntry{
			Name:       name,
			Value:      value,
			Domain:     domain,
			Path:       path,
			Source:     "curl",
			ImportedAt: time.Now(),
			RawValue:   pair,
		}

		// 设置默认过期时间
		if p.options.DefaultExpiry > 0 {
			cookie.Expires = time.Now().Add(time.Duration(p.options.DefaultExpiry) * time.Hour)
		}

		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

// ValidateCommand 验证 curl 命令
func (p *DefaultCurlParser) ValidateCommand(curlCmd string) error {
	_, err := p.ParseCommand(curlCmd)
	return err
}