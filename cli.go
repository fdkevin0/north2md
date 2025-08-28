package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	// 全局配置
	config *Config

	// 命令行参数
	flagTID        string
	flagInputFile  string
	flagOutputFile string
	flagCacheDir   string
	flagBaseURL    string
	// 简化：移除部分不常用的参数
	flagCookieFile    string
	flagNoCache       bool
	flagTimeout       int
	flagMaxConcurrent int
	flagDebug         bool

	// Cookie相关参数
	flagCurlCommand string
	flagCurlFile    string
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "north2md [TID]",
	Short: "HTML数据提取器 - 从南+ South Plus论坛提取帖子内容并转换为Markdown",
	Long: `HTML数据提取器是一个用Go语言开发的工具，专门用于从"南+ South Plus"论坛抓取帖子内容并转换为Markdown格式。
支持功能：
- 通过帖子ID直接抓取在线帖子内容
- 解析本地HTML文件
- 下载并缓存帖子中的所有附件(图片、文件)
- 生成格式化的Markdown文档`,
	Example: `  # 通过TID抓取在线帖子
  north2md 2636739 --output=post.md
  north2md --tid=2636739 --output=post.md

  # 使用Cookie文件登录
  north2md 2636739 --cookie-file=./cookies.toml --output=post.md

  # 解析本地HTML文件
  north2md --input=post.html --output=post.md

  # 指定缓存目录
  north2md 2636739 --cache-dir=./cache --output=post.md`,
	RunE: runExtractor,
	Args: cobra.MaximumNArgs(1), // 允许最多一个位置参数
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
  north2md cookie import --file=./curl.txt`,
	RunE: runCookieImport,
}

func init() {
	// 初始化默认配置
	config = NewDefaultConfig()

	// 根命令参数
	rootCmd.PersistentFlags().StringVar(&flagTID, "tid", "", "帖子ID (用于在线抓取)")
	rootCmd.PersistentFlags().StringVar(&flagInputFile, "input", "", "输入HTML文件路径")
	rootCmd.PersistentFlags().StringVar(&flagOutputFile, "output", "post.md", "输出Markdown文件路径")
	rootCmd.PersistentFlags().StringVar(&flagCacheDir, "cache-dir", "./cache", "附件缓存目录")
	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", "https://north-plus.net/", "论坛基础URL")
	rootCmd.PersistentFlags().StringVar(&flagCookieFile, "cookie-file", "./cookies.toml", "Cookie文件路径")
	rootCmd.PersistentFlags().BoolVar(&flagNoCache, "no-cache", false, "禁用附件缓存")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "启用调试日志")
	rootCmd.PersistentFlags().IntVar(&flagTimeout, "timeout", 30, "HTTP请求超时(秒)")
	rootCmd.PersistentFlags().IntVar(&flagMaxConcurrent, "max-concurrent", 5, "最大并发下载数")

	// 添加子命令
	rootCmd.AddCommand(cookieCmd)
	cookieCmd.AddCommand(cookieImportCmd)

	// cookie import 命令参数 (简化)
	cookieImportCmd.Flags().StringVar(&flagCurlCommand, "curl", "", "curl 命令字符串")
	cookieImportCmd.Flags().StringVar(&flagCurlFile, "file", "", "包含 curl 命令的文件路径")
	cookieImportCmd.MarkFlagsMutuallyExclusive("curl", "file")

	// 标记必需参数
	rootCmd.MarkFlagsMutuallyExclusive("tid", "input")
}

// Execute 执行命令行程序
func Execute() error {
	return rootCmd.Execute()
}

// initConfig 初始化配置
func initConfig() error {
	// 如果提供了位置参数，将其作为TID
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		// 检查第一个参数是否是命令
		firstArg := os.Args[1]
		isCommand := firstArg == "extract" || firstArg == "download" || firstArg == "cookie"

		if !isCommand && config.TID == "" {
			config.TID = firstArg
		}
	}

	// 更新配置参数
	if flagTID != "" {
		config.TID = flagTID
	}

	if flagOutputFile != "" {
		config.OutputFile = flagOutputFile
	}

	if flagCacheDir != "" {
		config.CacheDir = flagCacheDir
	}

	if flagBaseURL != "" {
		config.BaseURL = flagBaseURL
	}

	if flagCookieFile != "" {
		config.HTTPOpts.CookieFile = flagCookieFile
	}

	if flagNoCache {
		config.CacheOpts.EnableCache = false
	}

	if flagTimeout > 0 {
		config.HTTPOpts.Timeout = time.Duration(flagTimeout) * time.Second
	}

	if flagMaxConcurrent > 0 {
		config.HTTPOpts.MaxConcurrent = flagMaxConcurrent
	}

	return nil
}

// runExtractor 运行提取器
func runExtractor(cmd *cobra.Command, args []string) error {
	// 初始化配置
	if err := initConfig(); err != nil {
		return fmt.Errorf("初始化配置失败: %v", err)
	}

	// 初始化日志系统
	initLogger(flagDebug)

	// 如果提供了位置参数，确保TID被正确设置
	if len(args) > 0 && config.TID == "" {
		config.TID = args[0]
	}

	// 创建HTTP客户端
	httpClient := NewHTTPFetcher(&config.HTTPOpts, config.BaseURL)

	// 创建HTML解析器
	htmlParser := NewHTMLParser()

	// 创建Markdown生成器
	markdownGenerator := NewMarkdownGenerator(&config.MarkdownOpts)

	// 获取帖子内容
	var post *Post
	var err error

	if config.TID != "" {
		// 在线抓取模式
		fetcher := NewOnlinePostFetcher(httpClient, htmlParser, &config.Selectors)
		post, err = fetcher.FetchPost(config.TID)
		if err != nil {
			return fmt.Errorf("抓取帖子失败: %v", err)
		}
	} else if flagInputFile != "" {
		// 从本地文件加载
		if err := htmlParser.LoadFromFile(flagInputFile); err != nil {
			return fmt.Errorf("加载HTML文件失败: %v", err)
		}

		// 创建数据提取器
		extractor := NewDataExtractor(&config.Selectors)

		// 提取帖子数据
		post, err = extractor.ExtractPost(htmlParser)
		if err != nil {
			return fmt.Errorf("提取帖子数据失败: %v", err)
		}
	} else {
		return fmt.Errorf("必须指定帖子ID或 --input 参数")
	}

	// 创建输出目录
	outputDir := filepath.Dir(config.OutputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 获取基础目录
	baseDir := outputDir
	if config.OutputFile != "post.md" {
		baseDir = filepath.Dir(config.OutputFile)
	}

	// 保存帖子
	fmt.Println("正在保存帖子...")
	if err := markdownGenerator.SavePost(post, baseDir); err != nil {
		return fmt.Errorf("保存帖子失败: %v", err)
	}

	fmt.Printf("✓ 帖子已保存到 %s/%s/\n", baseDir, post.TID)
	return nil
}

// runCookieImport 运行 cookie 导入命令
func runCookieImport(cmd *cobra.Command, args []string) error {
	// 简化版本的Cookie导入逻辑
	fmt.Println("Cookie导入功能已简化")
	return nil
}
