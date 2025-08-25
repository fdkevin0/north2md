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
	configCmd.AddCommand(configInitCmd)

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