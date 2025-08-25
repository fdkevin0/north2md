package main

import (
	"time"
)

// Config 应用配置
type Config struct {
	// 输入配置
	TID     string `toml:"tid"`      // 帖子ID(用于在线抓取)
	BaseURL string `toml:"base_url"` // 论坛基础URL

	// 输出配置
	OutputFile string `toml:"output_file"` // 输出Markdown文件路径
	CacheDir   string `toml:"cache_dir"`   // 附件缓存目录

	// 功能配置
	Selectors    HTMLSelectors   `toml:"selectors"`     // CSS选择器配置
	MarkdownOpts MarkdownOptions `toml:"markdown_opts"` // Markdown生成选项
	HTTPOpts     HTTPOptions     `toml:"http_opts"`     // HTTP请求配置
	CacheOpts    CacheOptions    `toml:"cache_opts"`    // 缓存配置
}

// HTTPOptions HTTP请求配置
type HTTPOptions struct {
	Timeout       time.Duration     `toml:"timeout"`        // 请求超时时间
	UserAgent     string            `toml:"user_agent"`     // User-Agent
	MaxRetries    int               `toml:"max_retries"`    // 最大重试次数
	RetryDelay    time.Duration     `toml:"retry_delay"`    // 重试间隔
	MaxConcurrent int               `toml:"max_concurrent"` // 最大并发数
	CookieFile    string            `toml:"cookie_file"`    // Cookie文件路径
	EnableCookie  bool              `toml:"enable_cookie"`  // 是否启用Cookie
	CustomHeaders map[string]string `toml:"custom_headers"` // 自定义请求头
}

// CacheOptions 缓存配置
type CacheOptions struct {
	EnableCache  bool  `toml:"enable_cache"`  // 是否启用缓存
	CacheImages  bool  `toml:"cache_images"`  // 是否缓存图片
	CacheFiles   bool  `toml:"cache_files"`   // 是否缓存其他附件
	MaxFileSize  int64 `toml:"max_file_size"` // 最大文件大小(字节)
	SkipExisting bool  `toml:"skip_existing"` // 是否跳过已存在文件
}

// HTMLSelectors CSS选择器配置
type HTMLSelectors struct {
	Title       string `toml:"title"`        // 标题选择器
	Forum       string `toml:"forum"`        // 版块选择器
	PostTable   string `toml:"post_table"`   // 帖子表格选择器
	AuthorName  string `toml:"author_name"`  // 作者名称选择器
	PostTime    string `toml:"post_time"`    // 发帖时间选择器
	PostContent string `toml:"post_content"` // 帖子内容选择器
	Floor       string `toml:"floor"`        // 楼层选择器
	AuthorInfo  string `toml:"author_info"`  // 作者信息区域选择器
	Avatar      string `toml:"avatar"`       // 头像选择器
	Images      string `toml:"images"`       // 图片选择器
	Attachments string `toml:"attachments"`  // 附件选择器
}

// MarkdownOptions Markdown生成选项
type MarkdownOptions struct {
	IncludeAuthorInfo bool   `toml:"include_author_info"` // 是否包含作者详细信息
	IncludeImages     bool   `toml:"include_images"`      // 是否包含图片
	ImageStyle        string `toml:"image_style"`         // 图片显示方式(inline/reference)
	TableOfContents   bool   `toml:"table_of_contents"`   // 是否生成目录
	IncludeTOC        bool   `toml:"include_toc"`         // 是否包含目录
	FloorNumbering    bool   `toml:"floor_numbering"`     // 是否显示楼层编号
}

// NewDefaultConfig 创建默认配置
func NewDefaultConfig() *Config {
	return &Config{
		BaseURL:    "https://north-plus.net/",
		OutputFile: "post.md",
		CacheDir:   "./cache",
		Selectors: HTMLSelectors{
			Title:       "h1#subject_tpc",
			Forum:       ".nav a",
			PostTable:   "table", // 使用更通用的选择器
			AuthorName:  "strong",
			PostTime:    ".tiptop .gray",
			PostContent: "*", // 使用通配符选择器，匹配任何元素
			Floor:       ".tiptop .fl a",
			AuthorInfo:  ".tiptop .tar",
			Avatar:      "img[src*=\"avatar\"]",
			Images:      "img",
			Attachments: "a[href*=\"attachment\"]",
		},
		MarkdownOpts: MarkdownOptions{
			IncludeAuthorInfo: true,
			IncludeImages:     true,
			ImageStyle:        "inline",
			TableOfContents:   true,
			IncludeTOC:        true,
			FloorNumbering:    true,
		},
		HTTPOpts: HTTPOptions{
			Timeout:       30 * time.Second,
			UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			MaxRetries:    3,
			RetryDelay:    2 * time.Second,
			MaxConcurrent: 5,
			CookieFile:    "./cookies.toml",
			EnableCookie:  true,
			CustomHeaders: make(map[string]string),
		},
		CacheOpts: CacheOptions{
			EnableCache:  true,
			CacheImages:  true,
			CacheFiles:   true,
			MaxFileSize:  10 * 1024 * 1024, // 10MB
			SkipExisting: true,
		},
	}
}
