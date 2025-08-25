package main

import (
	"encoding/json"
	"time"
)

// Post 表示一个完整的论坛帖子
type Post struct {
	Title       string      `json:"title"`        // 帖子标题
	URL         string      `json:"url"`          // 帖子链接
	Forum       string      `json:"forum"`        // 版块名称
	MainPost    PostEntry   `json:"main_post"`    // 主楼内容
	Replies     []PostEntry `json:"replies"`      // 回复列表
	TotalFloors int         `json:"total_floors"` // 总楼层数
	CreatedAt   time.Time   `json:"created_at"`   // 创建时间
}

// PostEntry 表示单个楼层的内容
type PostEntry struct {
	Floor       string       `json:"floor"`        // 楼层标识(GF, B1F, B2F...)
	Author      Author       `json:"author"`       // 作者信息
	Content     string       `json:"content"`      // 帖子内容(纯文本)
	HTMLContent string       `json:"html_content"` // 原始HTML内容
	Images      []Image      `json:"images"`       // 图片列表
	Attachments []Attachment `json:"attachments"`  // 附件列表
	PostTime    time.Time    `json:"post_time"`    // 发帖时间
	PostID      string       `json:"post_id"`      // 帖子ID
}

// Author 表示作者信息
type Author struct {
	Username     string `json:"username"`      // 用户名
	UID          string `json:"uid"`           // 用户ID
	Avatar       string `json:"avatar"`        // 头像链接
	PostCount    int    `json:"post_count"`    // 发帖数
	RegisterDate string `json:"register_date"` // 注册时间
	LastLogin    string `json:"last_login"`    // 最后登录
	Signature    string `json:"signature"`     // 个性签名
}

// Image 表示图片信息
type Image struct {
	URL          string `json:"url"`           // 原始图片URL
	LocalPath    string `json:"local_path"`    // 本地缓存路径
	Alt          string `json:"alt"`           // 图片描述
	IsAttachment bool   `json:"is_attachment"` // 是否为附件
	FileSize     int64  `json:"file_size"`     // 文件大小
	Downloaded   bool   `json:"downloaded"`    // 是否已下载
}

// Attachment 表示附件信息
type Attachment struct {
	URL        string `json:"url"`         // 原始URL
	LocalPath  string `json:"local_path"` // 本地缓存路径
	FileName   string `json:"file_name"`  // 文件名
	FileSize   int64  `json:"file_size"`  // 文件大小
	MimeType   string `json:"mime_type"`  // 文件类型
	Downloaded bool   `json:"downloaded"` // 是否已下载
}

// CookieEntry 表示Cookie信息
type CookieEntry struct {
	Name     string    `json:"name"`      // Cookie名称
	Value    string    `json:"value"`     // Cookie值
	Domain   string    `json:"domain"`    // 域名
	Path     string    `json:"path"`      // 路径
	Expires  time.Time `json:"expires"`   // 过期时间
	MaxAge   int       `json:"max_age"`   // 最大存在时间(秒)
	Secure   bool      `json:"secure"`    // 是否只在HTTPS下传输
	HttpOnly bool      `json:"http_only"` // 是否仅HTTP可访问
	SameSite string    `json:"same_site"` // SameSite属性
	
	// 新增字段
	Source     string    `json:"source"`      // Cookie来源 (curl, browser, manual)
	ImportedAt time.Time `json:"imported_at"` // 导入时间
	RawValue   string    `json:"raw_value"`   // 原始Cookie值 (用于调试)
}

// CookieJar Cookie管理器
type CookieJar struct {
	Cookies     []CookieEntry `json:"cookies"`     // Cookie列表
	FilePath    string        `json:"file_path"`   // 存储文件路径
	LastUpdated time.Time     `json:"last_updated"` // 最后更新时间
}

// ToJSON 将结构体转换为JSON字符串
func (p *Post) ToJSON() (string, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	return string(data), err
}

// ToJSON 将CookieJar转换为JSON字符串
func (cj *CookieJar) ToJSON() (string, error) {
	data, err := json.MarshalIndent(cj, "", "  ")
	return string(data), err
}

// FromJSON 从JSON字符串解析Post
func (p *Post) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), p)
}

// FromJSON 从JSON字符串解析CookieJar
func (cj *CookieJar) FromJSON(data string) error {
	return json.Unmarshal([]byte(data), cj)
}