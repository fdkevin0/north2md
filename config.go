package south2md

import (
	"time"
)

// Config 应用配置
type Config struct {
	// 输入配置
	TID     string `toml:"tid" mapstructure:"tid"`           // 帖子ID(用于在线抓取)
	BaseURL string `toml:"base_url" mapstructure:"base_url"` // 论坛基础URL

	// 输出配置
	OutputFile string `toml:"output_file" mapstructure:"output_file"` // 输出Markdown文件路径
	CacheDir   string `toml:"cache_dir" mapstructure:"cache_dir"`     // 附件缓存目录

	// CSS选择器配置
	SelectorTitle       string `toml:"title" mapstructure:"title"`               // 标题选择器
	SelectorForum       string `toml:"forum" mapstructure:"forum"`               // 版块选择器
	SelectorPostTable   string `toml:"post_table" mapstructure:"post_table"`     // 帖子表格选择器
	SelectorAuthorName  string `toml:"author_name" mapstructure:"author_name"`   // 作者名称选择器
	SelectorPostTime    string `toml:"post_time" mapstructure:"post_time"`       // 发帖时间选择器
	SelectorPostContent string `toml:"post_content" mapstructure:"post_content"` // 帖子内容选择器
	SelectorFloor       string `toml:"floor" mapstructure:"floor"`               // 楼层选择器
	SelectorAuthorInfo  string `toml:"author_info" mapstructure:"author_info"`   // 作者信息区域选择器
	SelectorAvatar      string `toml:"avatar" mapstructure:"avatar"`             // 头像选择器
	SelectorImages      string `toml:"images" mapstructure:"images"`             // 图片选择器

	// HTTP请求配置
	HTTPTimeout          time.Duration     `toml:"timeout" mapstructure:"timeout"`                     // 请求超时时间
	HTTPUserAgent        string            `toml:"user_agent" mapstructure:"user_agent"`               // User-Agent
	HTTPMaxRetries       int               `toml:"max_retries" mapstructure:"max_retries"`             // 最大重试次数
	HTTPRetryDelay       time.Duration     `toml:"retry_delay" mapstructure:"retry_delay"`             // 重试间隔
	HTTPMaxConcurrent    int               `toml:"max_concurrent" mapstructure:"max_concurrent"`       // 最大并发数
	HTTPStrictPagination bool              `toml:"strict_pagination" mapstructure:"strict_pagination"` // 分页抓取失败是否严格报错
	HTTPCookieFile       string            `toml:"cookie_file" mapstructure:"cookie_file"`             // Cookie文件路径
	HTTPEnableCookie     bool              `toml:"enable_cookie" mapstructure:"enable_cookie"`         // 是否启用Cookie
	HTTPCustomHeaders    map[string]string `toml:"custom_headers" mapstructure:"custom_headers"`       // 自定义请求头

	// Markdown生成配置
	MarkdownIncludeAuthorInfo bool   `toml:"include_author_info" mapstructure:"include_author_info"` // 是否包含作者详细信息
	MarkdownIncludeImages     bool   `toml:"include_images" mapstructure:"include_images"`           // 是否包含图片
	MarkdownImageStyle        string `toml:"image_style" mapstructure:"image_style"`                 // 图片显示方式(inline/reference)
	MarkdownTableOfContents   bool   `toml:"table_of_contents" mapstructure:"table_of_contents"`     // 是否生成目录
	MarkdownIncludeTOC        bool   `toml:"include_toc" mapstructure:"include_toc"`                 // 是否包含目录
	MarkdownFloorNumbering    bool   `toml:"floor_numbering" mapstructure:"floor_numbering"`         // 是否显示楼层编号

	// 缓存配置
	CacheEnableCache  bool  `toml:"enable_cache" mapstructure:"enable_cache"`   // 是否启用缓存
	CacheCacheImages  bool  `toml:"cache_images" mapstructure:"cache_images"`   // 是否缓存图片
	CacheCacheFiles   bool  `toml:"cache_files" mapstructure:"cache_files"`     // 是否缓存其他附件
	CacheMaxFileSize  int64 `toml:"max_file_size" mapstructure:"max_file_size"` // 最大文件大小(字节)
	CacheSkipExisting bool  `toml:"skip_existing" mapstructure:"skip_existing"` // 是否跳过已存在文件

	// Gofile config
	GofileEnable       bool   `toml:"gofile_enable" mapstructure:"gofile_enable"`               // Enable gofile downloads
	GofileTool         string `toml:"gofile_tool" mapstructure:"gofile_tool"`                   // gofile-downloader script path
	GofileDir          string `toml:"gofile_dir" mapstructure:"gofile_dir"`                     // gofile download directory
	GofileToken        string `toml:"gofile_token" mapstructure:"gofile_token"`                 // gofile account token
	GofileVenvDir      string `toml:"gofile_venv_dir" mapstructure:"gofile_venv_dir"`           // gofile virtualenv directory
	GofileSkipExisting bool   `toml:"gofile_skip_existing" mapstructure:"gofile_skip_existing"` // Skip already downloaded content
}

// HTTPOptions HTTP请求配置 (向后兼容)
type HTTPOptions struct {
	Timeout          time.Duration     `toml:"timeout"`
	UserAgent        string            `toml:"user_agent"`
	MaxRetries       int               `toml:"max_retries"`
	RetryDelay       time.Duration     `toml:"retry_delay"`
	MaxConcurrent    int               `toml:"max_concurrent"`
	StrictPagination bool              `toml:"strict_pagination"`
	CookieFile       string            `toml:"cookie_file"`
	EnableCookie     bool              `toml:"enable_cookie"`
	CustomHeaders    map[string]string `toml:"custom_headers"`
}

// HTMLSelectors CSS选择器配置 (向后兼容)
type HTMLSelectors struct {
	Title       string `toml:"title"`
	Forum       string `toml:"forum"`
	PostTable   string `toml:"post_table"`
	AuthorName  string `toml:"author_name"`
	PostTime    string `toml:"post_time"`
	PostContent string `toml:"post_content"`
	Floor       string `toml:"floor"`
	AuthorInfo  string `toml:"author_info"`
	Avatar      string `toml:"avatar"`
	Images      string `toml:"images"`
}

// MarkdownOptions Markdown生成选项 (向后兼容)
type MarkdownOptions struct {
	IncludeAuthorInfo bool   `toml:"include_author_info"`
	IncludeImages     bool   `toml:"include_images"`
	ImageStyle        string `toml:"image_style"`
	TableOfContents   bool   `toml:"table_of_contents"`
	IncludeTOC        bool   `toml:"include_toc"`
	FloorNumbering    bool   `toml:"floor_numbering"`
}

// Default configuration values (centralized for maintainability)
var defaultConfig = &Config{
	BaseURL:    "https://south-plus.net/",
	OutputFile: "post.md",
	CacheDir:   DefaultCacheDir("south2md"),

	// CSS选择器配置
	SelectorTitle:       "h1#subject_tpc",
	SelectorForum:       "#breadcrumbs .crumbs-item.gray3:nth-child(3)",
	SelectorPostTable:   "table.js-post",
	SelectorAuthorName:  "strong",
	SelectorPostTime:    ".tiptop .gray",
	SelectorPostContent: "div[id^='read_']",
	SelectorFloor:       ".tiptop .fl a",
	SelectorAuthorInfo:  ".tiptop .tar",
	SelectorAvatar:      "img[src*=\"avatar\"]",
	SelectorImages:      "img",

	// HTTP配置
	HTTPTimeout:          30 * time.Second,
	HTTPUserAgent:        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36",
	HTTPMaxRetries:       3,
	HTTPRetryDelay:       2 * time.Second,
	HTTPMaxConcurrent:    5,
	HTTPStrictPagination: true,
	HTTPCookieFile:       DefaultCookieFile("south2md"),
	HTTPEnableCookie:     true,
	HTTPCustomHeaders:    make(map[string]string),

	// Markdown配置
	MarkdownIncludeAuthorInfo: true,
	MarkdownIncludeImages:     true,
	MarkdownImageStyle:        "inline",
	MarkdownTableOfContents:   true,
	MarkdownIncludeTOC:        true,
	MarkdownFloorNumbering:    true,

	// 缓存配置
	CacheEnableCache:  true,
	CacheCacheImages:  true,
	CacheCacheFiles:   true,
	CacheMaxFileSize:  10 * 1024 * 1024, // 10MB
	CacheSkipExisting: true,

	// Gofile配置
	GofileEnable:       true,
	GofileTool:         "",
	GofileDir:          "gofile",
	GofileToken:        "",
	GofileVenvDir:      "",
	GofileSkipExisting: true,
}

// NewDefaultConfig 创建默认配置
func NewDefaultConfig() *Config {
	config := *defaultConfig // Copy defaults
	if config.GofileTool == "" {
		config.GofileTool = DefaultGofileToolPath("south2md")
	}
	if config.GofileVenvDir == "" {
		config.GofileVenvDir = DefaultGofileVenvDir("south2md")
	}
	return &config
}
