package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fdkevin0/south2md"
	"github.com/spf13/cobra"
)

var (
	// 全局配置
	config *south2md.Config

	// 命令行参数
	flagTID        string
	flagInputFile  string
	flagOutputFile string
	flagOffline    bool
	flagCacheDir   string
	flagBaseURL    string
	// 简化：移除部分不常用的参数
	flagCookieFile         string
	flagNoCache            bool
	flagTimeout            int
	flagMaxConcurrent      int
	flagDebug              bool
	flagGofileEnable       bool
	flagGofileTool         string
	flagGofileDir          string
	flagGofileToken        string
	flagGofileVenvDir      string
	flagGofileSkipExisting bool

	// Cookie相关参数
	flagCookieImportFile string
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "south2md [TID]",
	Short: "HTML数据提取器 - 从南+ South Plus论坛提取帖子内容并转换为Markdown",
	Long: `HTML数据提取器是一个用Go语言开发的工具，专门用于从"南+ South Plus"论坛抓取帖子内容并转换为Markdown格式。
支持功能：
- 通过帖子ID直接抓取在线帖子内容
- 解析本地HTML文件
- 下载并缓存帖子中的所有附件(图片、文件)
- 生成格式化的Markdown文档`,
	Example: `  # 通过TID抓取在线帖子
  south2md 2636739
  south2md --tid=2636739

  # 使用Cookie文件登录
  south2md 2636739 --cookie-file=./cookies.txt

  # 解析本地HTML文件
  south2md --input=post.html

  # 导出已存储帖子到指定目录
  south2md 2636739 --offline --output=./exports`,
	RunE: runExtractor,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		south2md.InitLogger(flagDebug)
	},
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
	Short: "Import a Netscape cookie file",
	Long:  `Import a Netscape cookie file and cache it to the user data dir`,
	Example: `  # Import a Netscape cookie file
  south2md cookie import --file=./cookies.txt`,
	RunE: runCookieImport,
}

func init() {
	// 初始化默认配置
	config = south2md.NewDefaultConfig()

	// 根命令参数
	rootCmd.PersistentFlags().StringVar(&flagTID, "tid", "", "帖子ID (用于在线抓取)")
	rootCmd.PersistentFlags().StringVar(&flagInputFile, "input", "", "输入HTML文件路径")
	rootCmd.PersistentFlags().StringVar(&flagOutputFile, "output", "", "导出目录路径（可选）")
	rootCmd.PersistentFlags().BoolVar(&flagOffline, "offline", false, "离线模式：只从本地库导出，不抓取线上数据")
	rootCmd.PersistentFlags().StringVar(&flagCacheDir, "cache-dir", config.CacheDir, "附件缓存目录")
	rootCmd.PersistentFlags().StringVar(&flagBaseURL, "base-url", "https://south-plus.net/", "论坛基础URL")
	rootCmd.PersistentFlags().StringVar(&flagCookieFile, "cookie-file", config.HTTPCookieFile, "Cookie file path (Netscape format)")
	rootCmd.PersistentFlags().BoolVar(&flagNoCache, "no-cache", false, "禁用附件缓存")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "启用调试日志")
	rootCmd.PersistentFlags().IntVar(&flagTimeout, "timeout", 30, "HTTP请求超时(秒)")
	rootCmd.PersistentFlags().IntVar(&flagMaxConcurrent, "max-concurrent", 5, "最大并发下载数")
	rootCmd.PersistentFlags().BoolVar(&flagGofileEnable, "gofile-enable", config.GofileEnable, "启用gofile下载")
	rootCmd.PersistentFlags().StringVar(&flagGofileTool, "gofile-tool", config.GofileTool, "gofile-downloader脚本路径")
	rootCmd.PersistentFlags().StringVar(&flagGofileDir, "gofile-dir", config.GofileDir, "gofile下载目录")
	rootCmd.PersistentFlags().StringVar(&flagGofileToken, "gofile-token", config.GofileToken, "gofile账号token")
	rootCmd.PersistentFlags().StringVar(&flagGofileVenvDir, "gofile-venv-dir", config.GofileVenvDir, "gofile虚拟环境目录")
	rootCmd.PersistentFlags().BoolVar(&flagGofileSkipExisting, "gofile-skip-existing", config.GofileSkipExisting, "跳过已存在的gofile内容")

	// 添加子命令
	rootCmd.AddCommand(cookieCmd)
	cookieCmd.AddCommand(cookieImportCmd)

	// cookie import 命令参数
	cookieImportCmd.Flags().StringVar(&flagCookieImportFile, "file", "", "Cookie file path (Netscape format)")

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
		config.HTTPCookieFile = flagCookieFile
	}

	if flagNoCache {
		config.CacheEnableCache = false
	}

	if flagTimeout > 0 {
		config.HTTPTimeout = time.Duration(flagTimeout) * time.Second
	}

	if flagMaxConcurrent > 0 {
		config.HTTPMaxConcurrent = flagMaxConcurrent
	}

	config.GofileEnable = flagGofileEnable
	if flagGofileTool != "" {
		config.GofileTool = flagGofileTool
	}
	if flagGofileDir != "" {
		config.GofileDir = flagGofileDir
	}
	if flagGofileToken != "" {
		config.GofileToken = flagGofileToken
	}
	if flagGofileVenvDir != "" {
		config.GofileVenvDir = flagGofileVenvDir
	}
	config.GofileSkipExisting = flagGofileSkipExisting

	return nil
}

// runExtractor 运行提取器
func runExtractor(cmd *cobra.Command, args []string) error {
	// 初始化配置
	if err := initConfig(); err != nil {
		return fmt.Errorf("初始化配置失败: %v", err)
	}

	// 如果提供了位置参数，确保TID被正确设置
	if len(args) > 0 && config.TID == "" {
		config.TID = args[0]
	}

	if flagOffline && flagInputFile != "" {
		return fmt.Errorf("--offline 模式下不支持 --input")
	}

	if flagOffline && config.TID == "" {
		return fmt.Errorf("--offline 模式必须指定帖子ID")
	}

	storeDir := filepath.Join(south2md.DefaultDataDir("south2md"), "posts")
	store := south2md.NewPostStore(storeDir)
	if err := store.EnsureRoot(); err != nil {
		return fmt.Errorf("初始化本地数据目录失败: %v", err)
	}

	if flagOffline {
		if flagOutputFile == "" {
			return fmt.Errorf("--offline 模式需要指定 --output 导出目录")
		}
		exportGenerator := newMarkdownGenerator()
		exportGenerator.SetDownloadEnabled(false)
		post, err := store.LoadPostFromStore(config.TID)
		if err != nil {
			return fmt.Errorf("离线加载帖子失败: %v", err)
		}
		exportDir := resolveExportDir(flagOutputFile)
		exportedDir, err := store.ExportPost(config.TID, exportDir)
		if err != nil {
			return fmt.Errorf("离线导出失败: %v", err)
		}
		if err := exportGenerator.ExportPost(post, exportDir); err != nil {
			return fmt.Errorf("离线导出Markdown失败: %v", err)
		}
		fmt.Printf("✓ 离线导出完成: %s\n", exportedDir)
		return nil
	}

	// 创建HTTP客户端
	httpOptions := &south2md.HTTPOptions{
		Timeout:       config.HTTPTimeout,
		UserAgent:     config.HTTPUserAgent,
		MaxRetries:    config.HTTPMaxRetries,
		RetryDelay:    config.HTTPRetryDelay,
		MaxConcurrent: config.HTTPMaxConcurrent,
		CookieFile:    config.HTTPCookieFile,
		EnableCookie:  config.HTTPEnableCookie,
		CustomHeaders: config.HTTPCustomHeaders,
	}
	client := south2md.NewHTTPClient(httpOptions)

	// 创建Fetcher
	httpClient := south2md.NewFetcher(client, httpOptions, config.BaseURL)

	// 创建帖子解析器
	postParser := south2md.NewPostParser(&south2md.HTMLSelectors{
		Title:       config.SelectorTitle,
		Forum:       config.SelectorForum,
		PostTable:   config.SelectorPostTable,
		AuthorName:  config.SelectorAuthorName,
		PostTime:    config.SelectorPostTime,
		PostContent: config.SelectorPostContent,
		Floor:       config.SelectorFloor,
		AuthorInfo:  config.SelectorAuthorInfo,
		Avatar:      config.SelectorAvatar,
		Images:      config.SelectorImages,
	})

	markdownGenerator := newMarkdownGenerator()

	// 获取帖子内容
	var post *south2md.Post
	var err error

	if config.TID != "" {
		// 在线抓取模式
		post, err = httpClient.FetchPostWithPagination(config.TID, postParser, &south2md.HTMLSelectors{
			Title:       config.SelectorTitle,
			Forum:       config.SelectorForum,
			PostTable:   config.SelectorPostTable,
			AuthorName:  config.SelectorAuthorName,
			PostTime:    config.SelectorPostTime,
			PostContent: config.SelectorPostContent,
			Floor:       config.SelectorFloor,
			AuthorInfo:  config.SelectorAuthorInfo,
			Avatar:      config.SelectorAvatar,
			Images:      config.SelectorImages,
		})
		if err != nil {
			return fmt.Errorf("抓取帖子失败: %v", err)
		}
	} else if flagInputFile != "" {
		// 从本地文件加载
		if err := postParser.LoadFromFile(flagInputFile); err != nil {
			return fmt.Errorf("加载HTML文件失败: %v", err)
		}

		// 提取帖子数据
		post, err = postParser.ExtractPost()
		if err != nil {
			return fmt.Errorf("提取帖子数据失败: %v", err)
		}
	} else {
		return fmt.Errorf("必须指定帖子ID或 --input 参数")
	}

	if post.TID == "" {
		post.TID = config.TID
	}
	if post.TID == "" {
		return fmt.Errorf("无法确定帖子ID，请提供 --tid")
	}

	// 始终先入库到 XDG data 目录
	fmt.Println("正在保存帖子到本地库...")
	if err := markdownGenerator.StorePost(post, store.RootDir()); err != nil {
		return fmt.Errorf("保存帖子到本地库失败: %v", err)
	}
	fmt.Printf("✓ 帖子已存储到 %s/%s/\n", store.RootDir(), post.TID)

	// 可选导出
	if flagOutputFile != "" {
		exportDir := resolveExportDir(flagOutputFile)
		exportedDir, err := store.ExportPost(post.TID, exportDir)
		if err != nil {
			return fmt.Errorf("导出帖子失败: %v", err)
		}
		if err := markdownGenerator.ExportPost(post, exportDir); err != nil {
			return fmt.Errorf("导出Markdown失败: %v", err)
		}
		fmt.Printf("✓ 帖子已导出到 %s\n", exportedDir)
	}

	return nil
}

func newMarkdownGenerator() *south2md.MarkdownGenerator {
	var gofileHandler *south2md.GofileHandler
	if config.GofileEnable {
		gofileHandler = south2md.NewGofileHandler(config)
	}
	return south2md.NewMarkdownGenerator(&south2md.MarkdownOptions{
		IncludeAuthorInfo: config.MarkdownIncludeAuthorInfo,
		IncludeImages:     config.MarkdownIncludeImages,
		ImageStyle:        config.MarkdownImageStyle,
		TableOfContents:   config.MarkdownTableOfContents,
		IncludeTOC:        config.MarkdownIncludeTOC,
		FloorNumbering:    config.MarkdownFloorNumbering,
	}, gofileHandler)
}

func resolveExportDir(output string) string {
	if output == "" {
		return ""
	}
	if strings.EqualFold(filepath.Ext(output), ".md") {
		return filepath.Dir(output)
	}
	return output
}

// runCookieImport 运行 cookie 导入命令
func runCookieImport(cmd *cobra.Command, args []string) error {
	if flagCookieImportFile == "" {
		return fmt.Errorf("missing required flag: --file")
	}

	destPath := south2md.DefaultCookieFile("south2md")
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create cookie cache directory: %v", err)
	}

	cm := south2md.NewCookieManager()
	if err := cm.LoadFromFile(flagCookieImportFile); err != nil {
		return fmt.Errorf("failed to load cookie file: %v", err)
	}
	if err := cm.SaveToFile(destPath); err != nil {
		return fmt.Errorf("failed to save cookie file: %v", err)
	}

	fmt.Printf("Cookie file cached at %s\n", destPath)
	return nil
}
