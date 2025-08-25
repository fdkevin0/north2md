package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	// 全局配置
	config *Config

	// 命令行参数
	flagTID          string
	flagInputFile    string
	flagOutputFile   string
	flagCacheDir     string
	flagBaseURL      string
	flagCookieFile   string
	flagNoCache      bool
	flagNoCookie     bool
	flagTimeout      int
	flagMaxConcurrent int
	flagVerbose      bool
	flagHeaders      []string
	
	// Cookie 相关参数
	flagCurlCommand  string
	flagCurlFile     string
	flagTestURL      string
	flagOverwrite    bool
	flagTestMode     bool
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "north2md",
	Short: "HTML数据提取器 - 从南+ South Plus论坛提取帖子内容并转换为Markdown",
	Long: `HTML数据提取器是一个用Go语言开发的工具，专门用于从"南+ South Plus"论坛抓取帖子内容并转换为Markdown格式。

支持功能：
- 通过帖子ID直接抓取在线帖子内容
- 解析本地HTML文件
- 下载并缓存帖子中的所有附件(图片、文件)
- 生成格式化的Markdown文档
- Cookie管理和用户身份认证
- 并发下载优化`,
	Example: `  # 通过TID抓取在线帖子
  north2md --tid=2636739 --output=post.md

  # 使用Cookie文件登录
  north2md --tid=2636739 --cookie-file=./cookies.json --output=post.md

  # 解析本地HTML文件
  north2md --input=post.html --output=post.md

  # 指定缓存目录
  north2md --tid=2636739 --cache-dir=./cache --output=post.md

  # 禁用附件下载
  north2md --tid=2636739 --no-cache --output=post.md`,
	RunE: runExtractor,
}

// extractCmd 提取命令
var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "提取帖子内容",
	Long:  `从指定的帖子ID或HTML文件中提取内容并转换为Markdown格式`,
	RunE:  runExtractor,
}

// downloadCmd 下载命令
var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "下载帖子附件",
	Long:  `仅下载帖子中的附件到本地缓存，不生成Markdown文件`,
	RunE:  runDownloader,
}

// configCmd 配置命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "管理配置文件",
	Long:  `创建、查看或修改配置文件`,
}

// configInitCmd 初始化配置命令
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化默认配置文件",
	Long:  `创建默认的配置文件到当前目录`,
	RunE:  runConfigInit,
}

// cookieCmd cookie管理命令
var cookieCmd = &cobra.Command{
	Use:   "cookie",
	Short: "Cookie管理工具",
	Long:  `管理和操作Cookie数据`,
}

// cookieImportCmd cookie导入命令
var cookieImportCmd = &cobra.Command{
	Use:   "import",
	Short: "从 curl 命令导入 cookie",
	Long:  `从 curl 命令或包含 curl 命令的文件中解析并导入 cookie`,
	Example: `  # 从 curl 命令导入 cookie
  north2md cookie import --curl="curl 'https://example.com' -b 'session=abc123'"

  # 从文件导入 curl 命令
  north2md cookie import --file=./curl.txt

  # 覆盖已存在的 cookie
  north2md cookie import --file=./curl.txt --overwrite

  # 从 curl 命令导入并立即测试
  north2md cookie import --curl="curl '...' -b '...'" --test --test-url="https://north-plus.net/read.php?tid-2625015.html"`,
	RunE:  runCookieImport,
}

// cookieListCmd cookie列表命令
var cookieListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有 cookie",
	Long:  `显示当前存储的所有 cookie 信息`,
	RunE:  runCookieList,
}

// cookieTestCmd cookie测试命令
var cookieTestCmd = &cobra.Command{
	Use:   "test",
	Short: "测试 cookie 访问性",
	Long:  `测试 cookie 是否能够绕过登录墙并访问指定页面`,
	Example: `  # 测试 cookie 访问性
  north2md cookie test --url="https://north-plus.net/read.php?tid-2625015.html"

  # 使用指定的 cookie 文件测试
  north2md cookie test --url="https://north-plus.net/read.php?tid-2625015.html" --cookie-file=./cookies.json

  # 详细模式显示测试结果
  north2md cookie test --url="https://north-plus.net/read.php?tid-2625015.html" --verbose`,
	RunE:  runCookieTest,
}

// cookieValidateCmd cookie验证命令
var cookieValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证 cookie 有效性",
	Long:  `验证当前 cookie 的有效性和权限级别`,
	RunE:  runCookieValidate,
}

func init() {
	// 初始化默认配置
	config = NewDefaultConfig()

	// 根命令参数
	rootCmd.PersistentFlags().StringVar(&flagTID, "tid", "", "帖子ID (用于在线抓取)")
	rootCmd.PersistentFlags().StringVar(&flagInputFile, "input", "", "本地HTML文件路径")
	rootCmd.PersistentFlags().StringVar(&flagOutputFile, "output", "post.md", "输出Markdown文件路径")
	rootCmd.PersistentFlags().StringVar(&flagCacheDir, "cache-dir", "./cache", "附件缓存目录")
	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", "https://north-plus.net/", "论坛基础URL")
	rootCmd.PersistentFlags().StringVar(&flagCookieFile, "cookie-file", "./cookies.json", "Cookie文件路径")
	rootCmd.PersistentFlags().BoolVar(&flagNoCache, "no-cache", false, "禁用附件缓存")
	rootCmd.PersistentFlags().BoolVar(&flagNoCookie, "no-cookie", false, "禁用Cookie功能")
	rootCmd.PersistentFlags().IntVar(&flagTimeout, "timeout", 30, "HTTP请求超时(秒)")
	rootCmd.PersistentFlags().IntVar(&flagMaxConcurrent, "max-concurrent", 5, "最大并发下载数")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "详细日志输出")
	rootCmd.PersistentFlags().StringArrayVar(&flagHeaders, "header", []string{}, "自定义HTTP请求头 (格式: Key:Value)")

	// 添加子命令
	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(cookieCmd)
	configCmd.AddCommand(configInitCmd)
	
	// 添加 cookie 子命令
	cookieCmd.AddCommand(cookieImportCmd)
	cookieCmd.AddCommand(cookieListCmd)
	cookieCmd.AddCommand(cookieTestCmd)
	cookieCmd.AddCommand(cookieValidateCmd)
	
	// cookie import 命令参数
	cookieImportCmd.Flags().StringVar(&flagCurlCommand, "curl", "", "curl 命令字符串")
	cookieImportCmd.Flags().StringVar(&flagCurlFile, "file", "", "包含 curl 命令的文件路径")
	cookieImportCmd.Flags().BoolVar(&flagOverwrite, "overwrite", false, "是否覆盖已存在的 cookie")
	cookieImportCmd.Flags().BoolVar(&flagTestMode, "test", false, "导入后立即测试 cookie 有效性")
	cookieImportCmd.Flags().StringVar(&flagTestURL, "test-url", "", "测试 URL（仅在 --test 模式下有效）")
	cookieImportCmd.MarkFlagsMutuallyExclusive("curl", "file")
	
	// cookie test 命令参数
	cookieTestCmd.Flags().StringVar(&flagTestURL, "url", "", "测试 URL")
	cookieTestCmd.MarkFlagRequired("url")

	// 标记必需参数
	rootCmd.MarkFlagsMutuallyExclusive("tid", "input")
}

// Execute 执行命令行程序
func Execute() error {
	return rootCmd.Execute()
}

// runExtractor 运行提取器
func runExtractor(cmd *cobra.Command, args []string) error {
	// 验证参数
	if err := validateFlags(); err != nil {
		return err
	}

	// 应用命令行参数到配置
	applyFlags()

	// 打印配置信息
	if flagVerbose {
		printConfig()
	}

	// 创建HTML解析器
	parser := NewHTMLParser()

	// 加载HTML内容
	if err := loadHTML(parser); err != nil {
		return fmt.Errorf("加载HTML失败: %v", err)
	}

	// 创建数据提取器
	extractor := NewDataExtractor(&config.Selectors)

	// 提取帖子数据
	fmt.Println("正在提取帖子数据...")
	post, err := extractor.ExtractPost(parser)
	if err != nil {
		return fmt.Errorf("提取帖子数据失败: %v", err)
	}

	if flagVerbose {
		fmt.Printf("提取完成: 标题=\"%s\", 总楼层=%d\n", post.Title, post.TotalFloors)
	}

	// 下载附件 (如果启用)
	if !flagNoCache && config.CacheOpts.EnableCache {
		if err := downloadAttachments(post); err != nil {
			fmt.Printf("警告: 下载附件时发生错误: %v\n", err)
		}
	}

	// 生成Markdown
	fmt.Println("正在生成Markdown文档...")
	generator := NewMarkdownGenerator(&config.MarkdownOpts)
	markdown, err := generator.GenerateMarkdown(post)
	if err != nil {
		return fmt.Errorf("生成Markdown失败: %v", err)
	}

	// 保存到文件
	if err := os.WriteFile(config.OutputFile, []byte(markdown), 0644); err != nil {
		return fmt.Errorf("保存Markdown文件失败: %v", err)
	}

	fmt.Printf("✓ Markdown文档已保存到: %s\n", config.OutputFile)

	// 打印统计信息
	if flagVerbose {
		printStats(post)
	}

	return nil
}

// runDownloader 运行下载器
func runDownloader(cmd *cobra.Command, args []string) error {
	// 验证参数
	if err := validateFlags(); err != nil {
		return err
	}

	// 应用命令行参数到配置
	applyFlags()

	// 强制启用缓存
	config.CacheOpts.EnableCache = true

	// 创建HTML解析器
	parser := NewHTMLParser()

	// 加载HTML内容
	if err := loadHTML(parser); err != nil {
		return fmt.Errorf("加载HTML失败: %v", err)
	}

	// 创建数据提取器
	extractor := NewDataExtractor(&config.Selectors)

	// 提取帖子数据
	fmt.Println("正在提取帖子数据...")
	post, err := extractor.ExtractPost(parser)
	if err != nil {
		return fmt.Errorf("提取帖子数据失败: %v", err)
	}

	// 仅下载附件
	if err := downloadAttachments(post); err != nil {
		return fmt.Errorf("下载附件失败: %v", err)
	}

	fmt.Println("✓ 附件下载完成")
	return nil
}

// runConfigInit 初始化配置文件
func runConfigInit(cmd *cobra.Command, args []string) error {
	configFile := "config.json"
	
	// 检查配置文件是否已存在
	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("配置文件 %s 已存在，是否覆盖? (y/N): ", configFile)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	// 创建默认配置
	defaultConfig := NewDefaultConfig()
	
	// 保存配置文件
	configJSON, err := defaultConfig.ToJSON()
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(configFile, []byte(configJSON), 0644); err != nil {
		return fmt.Errorf("保存配置文件失败: %v", err)
	}

	fmt.Printf("✓ 默认配置文件已保存到: %s\n", configFile)
	return nil
}

// validateFlags 验证命令行参数
func validateFlags() error {
	// 必须指定TID或输入文件
	if flagTID == "" && flagInputFile == "" {
		return fmt.Errorf("必须指定 --tid 或 --input 参数")
	}

	// TID和输入文件不能同时指定
	if flagTID != "" && flagInputFile != "" {
		return fmt.Errorf("--tid 和 --input 参数不能同时指定")
	}

	// 验证输入文件是否存在
	if flagInputFile != "" {
		if _, err := os.Stat(flagInputFile); os.IsNotExist(err) {
			return fmt.Errorf("输入文件不存在: %s", flagInputFile)
		}
	}

	return nil
}

// applyFlags 应用命令行参数到配置
func applyFlags() {
	config.TID = flagTID
	config.InputFile = flagInputFile
	config.OutputFile = flagOutputFile
	config.CacheDir = flagCacheDir
	config.BaseURL = flagBaseURL

	// HTTP配置
	config.HTTPOpts.Timeout = time.Duration(flagTimeout) * time.Second
	config.HTTPOpts.MaxConcurrent = flagMaxConcurrent
	config.HTTPOpts.CookieFile = flagCookieFile
	config.HTTPOpts.EnableCookie = !flagNoCookie

	// 缓存配置
	config.CacheOpts.EnableCache = !flagNoCache

	// 解析自定义请求头
	if len(flagHeaders) > 0 {
		if config.HTTPOpts.CustomHeaders == nil {
			config.HTTPOpts.CustomHeaders = make(map[string]string)
		}
		
		for _, header := range flagHeaders {
			parts := strings.SplitN(header, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				config.HTTPOpts.CustomHeaders[key] = value
			}
		}
	}
}

// loadHTML 加载HTML内容
func loadHTML(parser HTMLParser) error {
	if config.InputFile != "" {
		// 从本地文件加载
		fmt.Printf("正在加载本地HTML文件: %s\n", config.InputFile)
		return parser.LoadFromFile(config.InputFile)
	} else {
		// 从在线抓取
		fmt.Printf("正在抓取在线帖子: TID=%s\n", config.TID)
		
		// 创建HTTP抓取器
		fetcher := NewHTTPFetcher(&config.HTTPOpts, config.BaseURL)
		
		// 抓取HTML内容
		html, err := fetcher.FetchPost(config.TID)
		if err != nil {
			return err
		}

		// 设置基础URL
		postURL := fmt.Sprintf("%s/read.php?tid-%s.html", 
			strings.TrimRight(config.BaseURL, "/"), config.TID)
		parser.SetBaseURL(postURL)

		return parser.LoadFromString(html)
	}
}

// downloadAttachments 下载附件
func downloadAttachments(post *Post) error {
	if !config.CacheOpts.EnableCache {
		return nil
	}

	fmt.Println("正在下载附件...")
	
	// 创建HTTP抓取器
	fetcher := NewHTTPFetcher(&config.HTTPOpts, config.BaseURL)
	
	// 创建附件下载器
	downloader := NewAttachmentDownloader(fetcher, &config.CacheOpts)
	
	// 下载所有附件
	if err := downloader.DownloadAll(post, config.CacheDir); err != nil {
		return err
	}

	// 打印下载统计
	total, downloaded, totalSize := downloader.GetDownloadStats(config.CacheDir)
	fmt.Printf("✓ 附件下载完成: %d/%d 个文件, 总大小: %s\n", 
		downloaded, total, formatFileSize(totalSize))

	return nil
}

// printConfig 打印配置信息
func printConfig() {
	fmt.Println("=== 配置信息 ===")
	fmt.Printf("TID: %s\n", config.TID)
	fmt.Printf("输入文件: %s\n", config.InputFile)
	fmt.Printf("输出文件: %s\n", config.OutputFile)
	fmt.Printf("缓存目录: %s\n", config.CacheDir)
	fmt.Printf("基础URL: %s\n", config.BaseURL)
	fmt.Printf("启用缓存: %t\n", config.CacheOpts.EnableCache)
	fmt.Printf("启用Cookie: %t\n", config.HTTPOpts.EnableCookie)
	fmt.Printf("请求超时: %v\n", config.HTTPOpts.Timeout)
	fmt.Printf("最大并发: %d\n", config.HTTPOpts.MaxConcurrent)
	fmt.Println("================")
}

// printStats 打印统计信息
func printStats(post *Post) {
	totalImages := len(post.MainPost.Images)
	totalAttachments := len(post.MainPost.Attachments)
	
	for _, reply := range post.Replies {
		totalImages += len(reply.Images)
		totalAttachments += len(reply.Attachments)
	}

	fmt.Println("=== 统计信息 ===")
	fmt.Printf("帖子标题: %s\n", post.Title)
	fmt.Printf("版块: %s\n", post.Forum)
	fmt.Printf("总楼层: %d\n", post.TotalFloors)
	fmt.Printf("图片数量: %d\n", totalImages)
	fmt.Printf("附件数量: %d\n", totalAttachments)
	fmt.Println("================")
}

// formatFileSize 格式化文件大小
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// ToJSON 将配置转换为JSON字符串
func (c *Config) ToJSON() (string, error) {
	// 这里需要实现JSON序列化，暂时返回空
	return "{}", nil
}

// runCookieImport 运行 cookie 导入命令
func runCookieImport(cmd *cobra.Command, args []string) error {
	// 验证参数
	if flagCurlCommand == "" && flagCurlFile == "" {
		return fmt.Errorf("必须指定 --curl 或 --file 参数")
	}

	// 创建 curl 解析器
	options := &CurlImportOptions{
		OverwriteExisting: flagOverwrite,
		AutoInferDomain:   true,
		AutoInferPath:     true,
		DefaultExpiry:     24 * 7, // 7天
	}
	parser := NewCurlParser(options)

	// 解析 curl 命令
	var commands []*CurlCommand
	var err error

	if flagCurlCommand != "" {
		// 从命令行解析
		curlCmd, err := parser.ParseCommand(flagCurlCommand)
		if err != nil {
			return fmt.Errorf("解析 curl 命令失败: %v", err)
		}
		commands = []*CurlCommand{curlCmd}
	} else {
		// 从文件解析
		commands, err = parser.ParseFromFile(flagCurlFile)
		if err != nil {
			return fmt.Errorf("从文件解析 curl 命令失败: %v", err)
		}
	}

	if len(commands) == 0 {
		return fmt.Errorf("未找到有效的 curl 命令")
	}

	// 创建 Cookie 管理器
	cookieManager := NewCookieManager()
	if err := cookieManager.LoadFromFile(flagCookieFile); err != nil {
		fmt.Printf("警告: 加载 Cookie 文件失败: %v\n", err)
	}

	// 提取和导入 cookies
	totalCookies := 0
	for i, curlCmd := range commands {
		fmt.Printf("正在处理第 %d 个 curl 命令: %s\n", i+1, curlCmd.URL)
		
		cookies, err := parser.ExtractCookies(curlCmd)
		if err != nil {
			fmt.Printf("警告: 提取 cookies 失败: %v\n", err)
			continue
		}

		for _, cookie := range cookies {
			cookieManager.AddCookie(cookie)
			totalCookies++
			if flagVerbose {
				fmt.Printf("  + 添加 Cookie: %s=%s (域名: %s)\n", 
					cookie.Name, cookie.Value[:min(20, len(cookie.Value))], cookie.Domain)
			}
		}
	}

	// 保存 cookies
	if err := cookieManager.SaveToFile(flagCookieFile); err != nil {
		return fmt.Errorf("保存 Cookie 文件失败: %v", err)
	}

	fmt.Printf("✓ 成功导入 %d 个 cookies 到 %s\n", totalCookies, flagCookieFile)

	// 如果启用测试模式
	if flagTestMode {
		if flagTestURL == "" {
			// 使用第一个 curl 命令的 URL 作为测试 URL
			flagTestURL = commands[0].URL
		}
		return runCookieTestInternal(flagTestURL, cookieManager)
	}

	return nil
}

// runCookieList 运行 cookie 列表命令
func runCookieList(cmd *cobra.Command, args []string) error {
	cookieManager := NewCookieManager()
	if err := cookieManager.LoadFromFile(flagCookieFile); err != nil {
		return fmt.Errorf("加载 Cookie 文件失败: %v", err)
	}

	cookies := cookieManager.GetAllCookies()
	if len(cookies) == 0 {
		fmt.Println("没有找到任何 cookies")
		return nil
	}

	fmt.Printf("共找到 %d 个 cookies:\n\n", len(cookies))

	for i, cookie := range cookies {
		fmt.Printf("%d. %s\n", i+1, cookie.Name)
		fmt.Printf("   值: %s\n", truncateString(cookie.Value, 50))
		fmt.Printf("   域名: %s\n", cookie.Domain)
		fmt.Printf("   路径: %s\n", cookie.Path)
		if !cookie.Expires.IsZero() {
			fmt.Printf("   过期时间: %s\n", cookie.Expires.Format("2006-01-02 15:04:05"))
		}
		if cookie.Source != "" {
			fmt.Printf("   来源: %s\n", cookie.Source)
		}
		if !cookie.ImportedAt.IsZero() {
			fmt.Printf("   导入时间: %s\n", cookie.ImportedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}

	return nil
}

// runCookieTest 运行 cookie 测试命令
func runCookieTest(cmd *cobra.Command, args []string) error {
	cookieManager := NewCookieManager()
	if err := cookieManager.LoadFromFile(flagCookieFile); err != nil {
		return fmt.Errorf("加载 Cookie 文件失败: %v", err)
	}

	return runCookieTestInternal(flagTestURL, cookieManager)
}

// runCookieTestInternal 内部 cookie 测试函数
func runCookieTestInternal(testURL string, cookieManager *DefaultCookieManager) error {
	// 创建 Cookie 验证器
	validator := NewCookieValidator(nil)

	// 获取适用的 cookies
	cookies := cookieManager.GetCookiesForURL(testURL)

	fmt.Printf("正在测试 URL: %s\n", testURL)
	fmt.Printf("使用 %d 个 cookies\n\n", len(cookies))

	// 执行验证
	result, err := validator.ValidateCookies(testURL, cookies)
	if err != nil {
		return fmt.Errorf("验证 cookies 失败: %v", err)
	}

	// 显示结果
	fmt.Println("=== 测试结果 ===")
	fmt.Printf("Cookie 有效性: %s\n", getBoolDisplay(result.IsValid))
	fmt.Printf("HTTP 状态码: %d\n", result.StatusCode)
	fmt.Printf("登录状态: %s\n", result.LoginStatus.String())
	fmt.Printf("响应时间: %v\n", result.ResponseTime)
	fmt.Printf("内容长度: %d 字节\n", result.ContentLength)

	if result.RedirectURL != "" {
		fmt.Printf("重定向 URL: %s\n", result.RedirectURL)
	}

	if result.HasLoginWall {
		fmt.Printf("登录墙: 是\n")
	} else {
		fmt.Printf("登录墙: 否\n")
	}

	if result.ErrorMessage != "" {
		fmt.Printf("错误信息: %s\n", result.ErrorMessage)
	}

	fmt.Println("===============")

	// 显示建议
	if result.IsValid {
		fmt.Println("✓ Cookies 状态良好，可以正常使用")
	} else {
		if result.HasLoginWall {
			fmt.Println("⚠ 检测到登录墙，需要重新获取 cookies")
		} else {
			fmt.Println("⚠ Cookies 可能已过期或无效，请检查")
		}
	}

	return nil
}

// runCookieValidate 运行 cookie 验证命令
func runCookieValidate(cmd *cobra.Command, args []string) error {
	// TODO: 实现 cookie 验证逻辑
	fmt.Println("✓ Cookie 验证功能尚未实现")
	return nil
}

// 工具函数

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func getBoolDisplay(b bool) string {
	if b {
		return "✓ 是"
	}
	return "✗ 否"
}