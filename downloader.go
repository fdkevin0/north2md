package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AttachmentDownloader 附件下载器接口
type AttachmentDownloader interface {
	DownloadImage(img *Image, cacheDir string) error
	DownloadAttachment(att *Attachment, cacheDir string) error
	DownloadAll(post *Post, cacheDir string) error
	GetLocalPath(url string, cacheDir string) string
	CheckCache(url string, cacheDir string) (string, bool)
	UpdateLocalPaths(post *Post, cacheDir string)
}

// DefaultAttachmentDownloader 默认附件下载器实现
type DefaultAttachmentDownloader struct {
	httpFetcher HTTPFetcher
	config      *CacheOptions
	semaphore   chan struct{} // 限制并发数
}

// DownloadMetadata 下载元数据
type DownloadMetadata struct {
	TID       string                  `json:"tid"`
	Downloads map[string]DownloadInfo `json:"downloads"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// DownloadInfo 下载信息
type DownloadInfo struct {
	OriginalURL string    `json:"original_url"`
	LocalPath   string    `json:"local_path"`
	FileSize    int64     `json:"file_size"`
	Downloaded  bool      `json:"downloaded"`
	DownloadAt  time.Time `json:"download_at"`
	MD5Hash     string    `json:"md5_hash"`
}

// NewAttachmentDownloader 创建新的附件下载器
func NewAttachmentDownloader(httpFetcher HTTPFetcher, config *CacheOptions) *DefaultAttachmentDownloader {
	// 创建信号量限制并发数
	maxConcurrent := 5
	if config != nil {
		// 可以从config中获取最大并发数，这里暂时使用固定值
	}

	return &DefaultAttachmentDownloader{
		httpFetcher: httpFetcher,
		config:      config,
		semaphore:   make(chan struct{}, maxConcurrent),
	}
}

// DownloadAll 下载帖子中的所有附件
func (d *DefaultAttachmentDownloader) DownloadAll(post *Post, cacheDir string) error {
	if !d.config.EnableCache {
		return nil
	}

	// 确保缓存目录存在
	if err := d.ensureCacheDir(cacheDir); err != nil {
		return fmt.Errorf("创建缓存目录失败: %v", err)
	}

	// 加载现有的下载元数据
	metadata := d.loadMetadata(cacheDir)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// 下载主楼的附件
	d.downloadPostEntryAttachments(&post.MainPost, cacheDir, metadata, &wg, &mu, &errors)

	// 下载回复中的附件
	for i := range post.Replies {
		d.downloadPostEntryAttachments(&post.Replies[i], cacheDir, metadata, &wg, &mu, &errors)
	}

	// 等待所有下载完成
	wg.Wait()

	// 保存元数据
	d.saveMetadata(metadata, cacheDir)

	// 更新本地路径
	d.UpdateLocalPaths(post, cacheDir)

	// 返回错误信息
	if len(errors) > 0 {
		return fmt.Errorf("下载过程中发生 %d 个错误，第一个错误: %v", len(errors), errors[0])
	}

	return nil
}

// downloadPostEntryAttachments 下载单个楼层的附件
func (d *DefaultAttachmentDownloader) downloadPostEntryAttachments(
	entry *PostEntry,
	cacheDir string,
	metadata *DownloadMetadata,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	errors *[]error,
) {
	// 下载图片
	if d.config.CacheImages {
		for i := range entry.Images {
			wg.Add(1)
			go func(img *Image) {
				defer wg.Done()

				d.semaphore <- struct{}{}        // 获取信号量
				defer func() { <-d.semaphore }() // 释放信号量

				if err := d.DownloadImage(img, cacheDir); err != nil {
					mu.Lock()
					*errors = append(*errors, fmt.Errorf("下载图片失败 %s: %v", img.URL, err))
					mu.Unlock()
				} else {
					// 更新元数据
					mu.Lock()
					d.updateMetadata(metadata, img.URL, img.LocalPath, img.FileSize, true)
					mu.Unlock()
				}
			}(&entry.Images[i])
		}
	}

	// 下载其他附件
	if d.config.CacheFiles {
		for i := range entry.Attachments {
			wg.Add(1)
			go func(att *Attachment) {
				defer wg.Done()

				d.semaphore <- struct{}{}        // 获取信号量
				defer func() { <-d.semaphore }() // 释放信号量

				if err := d.DownloadAttachment(att, cacheDir); err != nil {
					mu.Lock()
					*errors = append(*errors, fmt.Errorf("下载附件失败 %s: %v", att.URL, err))
					mu.Unlock()
				} else {
					// 更新元数据
					mu.Lock()
					d.updateMetadata(metadata, att.URL, att.LocalPath, att.FileSize, true)
					mu.Unlock()
				}
			}(&entry.Attachments[i])
		}
	}
}

// DownloadAllToPostDir 下载帖子中的所有附件到指定的帖子目录
func (d *DefaultAttachmentDownloader) DownloadAllToPostDir(post *Post, baseDir string) error {
	if !d.config.EnableCache {
		return nil
	}

	// 创建以TID命名的目录
	tidDir := filepath.Join(baseDir, post.TID)
	imagesDir := filepath.Join(tidDir, "images")
	attachmentsDir := filepath.Join(tidDir, "attachments")

	// 确保目录存在
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("创建images目录失败: %v", err)
	}
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		return fmt.Errorf("创建attachments目录失败: %v", err)
	}

	// 加载现有的下载元数据
	metadata := d.loadMetadata(tidDir)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	// 下载主楼的附件
	d.downloadPostEntryAttachmentsToDir(&post.MainPost, tidDir, imagesDir, attachmentsDir, metadata, &wg, &mu, &errors)

	// 下载回复中的附件
	for i := range post.Replies {
		d.downloadPostEntryAttachmentsToDir(&post.Replies[i], tidDir, imagesDir, attachmentsDir, metadata, &wg, &mu, &errors)
	}

	// 等待所有下载完成
	wg.Wait()

	// 保存元数据到帖子目录
	d.saveMetadata(metadata, tidDir)

	// 更新本地路径
	d.UpdateLocalPathsForPostDir(post, tidDir)

	// 返回错误信息
	if len(errors) > 0 {
		return fmt.Errorf("下载过程中发生 %d 个错误，第一个错误: %v", len(errors), errors[0])
	}

	return nil
}

// downloadPostEntryAttachmentsToDir 下载单个楼层的附件到指定目录
func (d *DefaultAttachmentDownloader) downloadPostEntryAttachmentsToDir(
	entry *PostEntry,
	tidDir, imagesDir, attachmentsDir string,
	metadata *DownloadMetadata,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	errors *[]error,
) {
	// 下载图片
	if d.config.CacheImages {
		for i := range entry.Images {
			wg.Add(1)
			go func(img *Image) {
				defer wg.Done()

				d.semaphore <- struct{}{}        // 获取信号量
				defer func() { <-d.semaphore }() // 释放信号量

				// 生成本地路径到帖子目录的images子目录
				localPath := d.GetLocalPath(img.URL, imagesDir)

				// 检查缓存
				if _, exists := d.CheckCacheInDir(img.URL, tidDir); exists && d.config.SkipExisting {
					img.LocalPath = localPath
					img.Downloaded = true
					return
				}

				// 确保目录存在
				if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
					mu.Lock()
					*errors = append(*errors, fmt.Errorf("创建图片目录失败: %v", err))
					mu.Unlock()
					return
				}

				// 下载文件
				fileSize, err := d.downloadFile(img.URL, localPath)
				if err != nil {
					mu.Lock()
					*errors = append(*errors, fmt.Errorf("下载图片失败 %s: %v", img.URL, err))
					mu.Unlock()
					return
				}

				// 更新图片信息
				img.LocalPath = localPath
				img.FileSize = fileSize
				img.Downloaded = true

				fmt.Printf("下载图片成功: %s -> %s\n", img.URL, localPath)

				// 更新元数据
				mu.Lock()
				d.updateMetadata(metadata, img.URL, img.LocalPath, img.FileSize, true)
				mu.Unlock()
			}(&entry.Images[i])
		}
	}

	// 下载其他附件
	if d.config.CacheFiles {
		for i := range entry.Attachments {
			wg.Add(1)
			go func(att *Attachment) {
				defer wg.Done()

				d.semaphore <- struct{}{}        // 获取信号量
				defer func() { <-d.semaphore }() // 释放信号量

				// 生成本地路径到帖子目录的attachments子目录
				localPath := d.GetLocalPath(att.URL, attachmentsDir)

				// 检查缓存
				if _, exists := d.CheckCacheInDir(att.URL, tidDir); exists && d.config.SkipExisting {
					att.LocalPath = localPath
					att.Downloaded = true
					return
				}

				// 确保目录存在
				if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
					mu.Lock()
					*errors = append(*errors, fmt.Errorf("创建附件目录失败: %v", err))
					mu.Unlock()
					return
				}

				// 下载文件
				fileSize, err := d.downloadFile(att.URL, localPath)
				if err != nil {
					mu.Lock()
					*errors = append(*errors, fmt.Errorf("下载附件失败 %s: %v", att.URL, err))
					mu.Unlock()
					return
				}

				// 更新附件信息
				att.LocalPath = localPath
				att.FileSize = fileSize
				att.Downloaded = true

				fmt.Printf("下载附件成功: %s -> %s\n", att.URL, localPath)

				// 更新元数据
				mu.Lock()
				d.updateMetadata(metadata, att.URL, att.LocalPath, att.FileSize, true)
				mu.Unlock()
			}(&entry.Attachments[i])
		}
	}
}

// DownloadImage 下载图片
func (d *DefaultAttachmentDownloader) DownloadImage(img *Image, cacheDir string) error {
	if img.URL == "" {
		return fmt.Errorf("图片URL为空")
	}

	// 检查缓存
	if localPath, exists := d.CheckCache(img.URL, cacheDir); exists && d.config.SkipExisting {
		img.LocalPath = localPath
		img.Downloaded = true
		return nil
	}

	// 生成本地路径
	localPath := d.GetLocalPath(img.URL, filepath.Join(cacheDir, "images"))

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建图片目录失败: %v", err)
	}

	// 下载文件
	fileSize, err := d.downloadFile(img.URL, localPath)
	if err != nil {
		return err
	}

	// 更新图片信息
	img.LocalPath = localPath
	img.FileSize = fileSize
	img.Downloaded = true

	fmt.Printf("下载图片成功: %s -> %s\n", img.URL, localPath)
	return nil
}

// DownloadAttachment 下载附件
func (d *DefaultAttachmentDownloader) DownloadAttachment(att *Attachment, cacheDir string) error {
	if att.URL == "" {
		return fmt.Errorf("附件URL为空")
	}

	// 检查缓存
	if localPath, exists := d.CheckCache(att.URL, cacheDir); exists && d.config.SkipExisting {
		att.LocalPath = localPath
		att.Downloaded = true
		return nil
	}

	// 生成本地路径
	localPath := d.GetLocalPath(att.URL, filepath.Join(cacheDir, "attachments"))

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建附件目录失败: %v", err)
	}

	// 下载文件
	fileSize, err := d.downloadFile(att.URL, localPath)
	if err != nil {
		return err
	}

	// 更新附件信息
	att.LocalPath = localPath
	att.FileSize = fileSize
	att.Downloaded = true

	fmt.Printf("下载附件成功: %s -> %s\n", att.URL, localPath)
	return nil
}

// downloadFile 下载文件到本地
func (d *DefaultAttachmentDownloader) downloadFile(url, localPath string) (int64, error) {
	// 检查文件大小限制
	if d.config.MaxFileSize > 0 {
		// 先获取文件大小
		size, err := d.getFileSize(url)
		if err == nil && size > d.config.MaxFileSize {
			return 0, fmt.Errorf("文件太大: %d bytes (限制: %d bytes)", size, d.config.MaxFileSize)
		}
	}

	// 创建临时文件
	tmpPath := localPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer tmpFile.Close()

	// 下载文件
	resp, err := d.httpFetcher.FetchWithRetry(url)
	if err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	// 复制内容
	written, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("写入文件失败: %v", err)
	}

	// 关闭临时文件
	tmpFile.Close()

	// 移动到最终位置
	if err := os.Rename(tmpPath, localPath); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("移动文件失败: %v", err)
	}

	return written, nil
}

// getFileSize 获取远程文件大小
func (d *DefaultAttachmentDownloader) getFileSize(url string) (int64, error) {
	resp, err := http.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.ContentLength, nil
}

// GetLocalPath 生成本地路径
func (d *DefaultAttachmentDownloader) GetLocalPath(url, cacheDir string) string {
	// 使用URL的MD5哈希作为文件名，避免路径冲突
	hash := fmt.Sprintf("%x", md5.Sum([]byte(url)))

	// 尝试从URL中提取文件扩展名
	ext := d.extractFileExtension(url)

	// 构建文件名
	filename := hash + ext

	return filepath.Join(cacheDir, filename)
}

// extractFileExtension 从URL中提取文件扩展名
func (d *DefaultAttachmentDownloader) extractFileExtension(url string) string {
	// 移除查询参数
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// 提取扩展名
	if idx := strings.LastIndex(url, "."); idx != -1 {
		ext := url[idx:]
		// 验证扩展名长度
		if len(ext) <= 5 {
			return ext
		}
	}

	// 根据URL模式推测扩展名
	lowerURL := strings.ToLower(url)
	if strings.Contains(lowerURL, "image") || strings.Contains(lowerURL, "img") {
		return ".jpg"
	}

	return ""
}

// CheckCache 检查文件是否已在缓存中
func (d *DefaultAttachmentDownloader) CheckCache(url, cacheDir string) (string, bool) {
	localPath := d.GetLocalPath(url, cacheDir)

	// 检查文件是否存在
	if _, err := os.Stat(localPath); err == nil {
		return localPath, true
	}

	return "", false
}

// CheckCacheInDir 检查文件是否已在指定目录的缓存中
func (d *DefaultAttachmentDownloader) CheckCacheInDir(url, tidDir string) (string, bool) {
	// 构建可能的文件路径
	imagesDir := filepath.Join(tidDir, "images")
	attachmentsDir := filepath.Join(tidDir, "attachments")

	// 生成文件名
	hash := fmt.Sprintf("%x", md5.Sum([]byte(url)))
	ext := d.extractFileExtension(url)
	filename := hash + ext

	// 检查images目录
	imagePath := filepath.Join(imagesDir, filename)
	if _, err := os.Stat(imagePath); err == nil {
		return imagePath, true
	}

	// 检查attachments目录
	attachmentPath := filepath.Join(attachmentsDir, filename)
	if _, err := os.Stat(attachmentPath); err == nil {
		return attachmentPath, true
	}

	return "", false
}

// UpdateLocalPaths 更新帖子中所有附件的本地路径
func (d *DefaultAttachmentDownloader) UpdateLocalPaths(post *Post, cacheDir string) {
	// 更新主楼
	d.updatePostEntryPaths(&post.MainPost, cacheDir)

	// 更新回复
	for i := range post.Replies {
		d.updatePostEntryPaths(&post.Replies[i], cacheDir)
	}
}

// UpdateLocalPathsForPostDir 更新帖子中所有附件的本地路径（针对帖子目录）
func (d *DefaultAttachmentDownloader) UpdateLocalPathsForPostDir(post *Post, tidDir string) {
	// 更新主楼
	d.updatePostEntryPathsForPostDir(&post.MainPost, tidDir)

	// 更新回复
	for i := range post.Replies {
		d.updatePostEntryPathsForPostDir(&post.Replies[i], tidDir)
	}
}

// updatePostEntryPaths 更新单个楼层的附件路径
func (d *DefaultAttachmentDownloader) updatePostEntryPaths(entry *PostEntry, cacheDir string) {
	// 更新图片路径
	for i := range entry.Images {
		if entry.Images[i].LocalPath == "" {
			if localPath, exists := d.CheckCache(entry.Images[i].URL, filepath.Join(cacheDir, "images")); exists {
				entry.Images[i].LocalPath = localPath
				entry.Images[i].Downloaded = true
			}
		}
	}

	// 更新附件路径
	for i := range entry.Attachments {
		if entry.Attachments[i].LocalPath == "" {
			if localPath, exists := d.CheckCache(entry.Attachments[i].URL, filepath.Join(cacheDir, "attachments")); exists {
				entry.Attachments[i].LocalPath = localPath
				entry.Attachments[i].Downloaded = true
			}
		}
	}
}

// updatePostEntryPathsForPostDir 更新单个楼层的附件路径（针对帖子目录）
func (d *DefaultAttachmentDownloader) updatePostEntryPathsForPostDir(entry *PostEntry, tidDir string) {
	// 更新图片路径
	for i := range entry.Images {
		if entry.Images[i].LocalPath == "" {
			if localPath, exists := d.CheckCacheInDir(entry.Images[i].URL, tidDir); exists {
				entry.Images[i].LocalPath = localPath
				entry.Images[i].Downloaded = true
			}
		}
	}

	// 更新附件路径
	for i := range entry.Attachments {
		if entry.Attachments[i].LocalPath == "" {
			if localPath, exists := d.CheckCacheInDir(entry.Attachments[i].URL, tidDir); exists {
				entry.Attachments[i].LocalPath = localPath
				entry.Attachments[i].Downloaded = true
			}
		}
	}
}

// ensureCacheDir 确保缓存目录存在
func (d *DefaultAttachmentDownloader) ensureCacheDir(cacheDir string) error {
	dirs := []string{
		cacheDir,
		filepath.Join(cacheDir, "images"),
		filepath.Join(cacheDir, "attachments"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// loadMetadata 加载下载元数据
func (d *DefaultAttachmentDownloader) loadMetadata(cacheDir string) *DownloadMetadata {
	metadataPath := filepath.Join(cacheDir, "metadata.json")

	metadata := &DownloadMetadata{
		Downloads: make(map[string]DownloadInfo),
		UpdatedAt: time.Now(),
	}

	if data, err := os.ReadFile(metadataPath); err == nil {
		json.Unmarshal(data, metadata)
	}

	return metadata
}

// saveMetadata 保存下载元数据
func (d *DefaultAttachmentDownloader) saveMetadata(metadata *DownloadMetadata, cacheDir string) error {
	metadataPath := filepath.Join(cacheDir, "metadata.json")
	metadata.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// updateMetadata 更新元数据信息
func (d *DefaultAttachmentDownloader) updateMetadata(metadata *DownloadMetadata, url, localPath string, fileSize int64, downloaded bool) {
	info := DownloadInfo{
		OriginalURL: url,
		LocalPath:   localPath,
		FileSize:    fileSize,
		Downloaded:  downloaded,
		DownloadAt:  time.Now(),
	}

	// 计算MD5哈希
	if downloaded && localPath != "" {
		if hash, err := d.calculateFileMD5(localPath); err == nil {
			info.MD5Hash = hash
		}
	}

	metadata.Downloads[url] = info
}

// calculateFileMD5 计算文件MD5哈希
func (d *DefaultAttachmentDownloader) calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// GetDownloadStats 获取下载统计信息
func (d *DefaultAttachmentDownloader) GetDownloadStats(cacheDir string) (int, int, int64) {
	metadata := d.loadMetadata(cacheDir)

	total := len(metadata.Downloads)
	downloaded := 0
	var totalSize int64

	for _, info := range metadata.Downloads {
		if info.Downloaded {
			downloaded++
			totalSize += info.FileSize
		}
	}

	return total, downloaded, totalSize
}

// CopyFilesToPostDir 将下载的文件复制到以TID命名的目录中
func (d *DefaultAttachmentDownloader) CopyFilesToPostDir(post *Post, baseDir string) error {
	// 创建以TID命名的目录
	tidDir := filepath.Join(baseDir, post.TID)
	imagesDir := filepath.Join(tidDir, "images")
	attachmentsDir := filepath.Join(tidDir, "attachments")

	// 确保目录存在
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("创建images目录失败: %v", err)
	}
	if err := os.MkdirAll(attachmentsDir, 0755); err != nil {
		return fmt.Errorf("创建attachments目录失败: %v", err)
	}

	// 复制主楼中的文件
	if err := d.copyPostEntryFiles(&post.MainPost, imagesDir, attachmentsDir); err != nil {
		return err
	}

	// 复制回复中的文件
	for i := range post.Replies {
		if err := d.copyPostEntryFiles(&post.Replies[i], imagesDir, attachmentsDir); err != nil {
			return err
		}
	}

	return nil
}

// copyPostEntryFiles 复制单个楼层的文件
func (d *DefaultAttachmentDownloader) copyPostEntryFiles(entry *PostEntry, imagesDir, attachmentsDir string) error {
	// 复制图片
	for _, img := range entry.Images {
		if img.LocalPath != "" && img.Downloaded {
			targetPath := filepath.Join(imagesDir, filepath.Base(img.LocalPath))
			if err := copyFile(img.LocalPath, targetPath); err != nil {
				return fmt.Errorf("复制图片失败 %s: %v", img.LocalPath, err)
			}
		}
	}

	// 复制附件
	for _, att := range entry.Attachments {
		if att.LocalPath != "" && att.Downloaded {
			targetPath := filepath.Join(attachmentsDir, filepath.Base(att.LocalPath))
			if err := copyFile(att.LocalPath, targetPath); err != nil {
				return fmt.Errorf("复制附件失败 %s: %v", att.LocalPath, err)
			}
		}
	}

	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	// 检查源文件是否存在
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("源文件不存在: %s", src)
	}

	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 复制内容
	_, err = io.Copy(dstFile, srcFile)
	return err
}
