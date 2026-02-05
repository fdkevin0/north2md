package north2md

import (
	"time"
)

// Post 表示一个完整的论坛帖子
type Post struct {
	TID         string      `toml:"tid"`          // 帖子ID
	Title       string      `toml:"title"`        // 帖子标题
	URL         string      `toml:"url"`          // 帖子链接
	Forum       string      `toml:"forum"`        // 版块名称
	MainPost    PostEntry   `toml:"main_post"`    // 主楼内容
	Replies     []PostEntry `toml:"replies"`      // 回复列表
	TotalFloors int         `toml:"total_floors"` // 总楼层数
	Images      []Image     `toml:"images"`       // 图片信息列表
	CreatedAt   time.Time   `toml:"created_at"`   // 创建时间
}

// PostEntry 表示单个楼层的内容
type PostEntry struct {
	Floor       string    `toml:"floor"`        // 楼层标识(GF, B1F, B2F...)
	Author      Author    `toml:"author"`       // 作者信息
	HTMLContent string    `toml:"html_content"` // 原始HTML内容
	PostTime    time.Time `toml:"post_time"`    // 发帖时间
	PostID      string    `toml:"post_id"`      // 帖子ID
}

// Author 表示作者信息
type Author struct {
	Username     string `toml:"username"`      // 用户名
	UID          string `toml:"uid"`           // 用户ID
	Avatar       string `toml:"avatar"`        // 头像链接
	PostCount    int    `toml:"post_count"`    // 发帖数
	RegisterDate string `toml:"register_date"` // 注册时间
	LastLogin    string `toml:"last_login"`    // 最后登录
	Signature    string `toml:"signature"`     // 个性签名
}

// Image 表示图片信息
type Image struct {
	URL        string `toml:"url"`        // 原始图片URL
	Local      string `toml:"local"`      // 本地缓存路径
	Alt        string `toml:"alt"`        // 图片描述
	FileSize   int64  `toml:"file_size"`  // 文件大小
	Downloaded bool   `toml:"downloaded"` // 是否已下载
}

// CookieEntry 表示Cookie信息
type CookieEntry struct {
	Name     string    `toml:"name"`      // Cookie名称
	Value    string    `toml:"value"`     // Cookie值
	Domain   string    `toml:"domain"`    // 域名
	Path     string    `toml:"path"`      // 路径
	Expires  time.Time `toml:"expires"`   // 过期时间
	MaxAge   int       `toml:"max_age"`   // 最大存在时间(秒)
	Secure   bool      `toml:"secure"`    // 是否只在HTTPS下传输
	HttpOnly bool      `toml:"http_only"` // 是否仅HTTP可访问
	SameSite string    `toml:"same_site"` // SameSite属性
}

// CookieJar Cookie管理器
type CookieJar struct {
	Cookies     []CookieEntry `toml:"cookies"`      // Cookie列表
	LastUpdated time.Time     `toml:"last_updated"` // 最后更新时间
}
