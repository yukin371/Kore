package core

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"sync"
	"time"
)

// FileCache 智能文件缓存（Content-Aware）
// 基于文件修改时间和内容哈希来避免重复读取
type FileCache struct {
	hashes   map[string]string    // path -> MD5 hash
	modTimes map[string]time.Time // path -> last modified time
	contents map[string]string    // path -> cached content
	mu       sync.RWMutex
}

// NewFileCache 创建文件缓存
func NewFileCache() *FileCache {
	return &FileCache{
		hashes:   make(map[string]string),
		modTimes: make(map[string]time.Time),
		contents: make(map[string]string),
	}
}

// CheckRead 检查文件是否需要读取
// 返回: (content, cached, changed)
//   - content: 文件内容（从缓存或实际读取）
//   - cached: 是否来自缓存
//   - changed: 文件是否已被外部修改
func (c *FileCache) CheckRead(path string) (string, bool, bool) {
	info, err := os.Stat(path)
	if err != nil {
		// 文件不存在或无法访问
		return "", false, false
	}

	c.mu.RLock()
	lastMod, ok := c.modTimes[path]
	c.mu.RUnlock()

	// 如果缓存中没有，需要读取
	if !ok {
		return c.readAndCache(path)
	}

	// 如果修改时间变了，需要重新读取
	if !info.ModTime().Equal(lastMod) {
		return c.readAndCache(path)
	}

	// 文件未修改，返回缓存
	c.mu.RLock()
	content := c.contents[path]
	c.mu.RUnlock()

	return content, true, false // cached, not changed
}

// readAndCache 读取文件并更新缓存
func (c *FileCache) readAndCache(path string) (string, bool, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false, false
	}

	contentStr := string(content)

	// 计算 MD5 hash
	hash := md5.Sum(content)
	hashStr := hex.EncodeToString(hash[:])

	// 获取文件信息
	info, _ := os.Stat(path)

	// 更新缓存
	c.mu.Lock()
	c.hashes[path] = hashStr
	c.modTimes[path] = info.ModTime()
	c.contents[path] = contentStr
	c.mu.Unlock()

	return contentStr, false, true // fresh read, not cached, changed
}

// Invalidate 使缓存失效（用于文件写入后）
func (c *FileCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.hashes, path)
	delete(c.modTimes, path)
	delete(c.contents, path)
}

// GetHash 获取文件的 MD5 hash
func (c *FileCache) GetHash(path string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hash, ok := c.hashes[path]
	return hash, ok
}

// UpdateAfterWrite 在写入操作后更新缓存
// 这样下次读取同一文件时可以直接使用缓存，避免重复读取
func (c *FileCache) UpdateAfterWrite(path string, content string) {
	// 计算 MD5 hash
	hash := md5.Sum([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	// 获取当前文件信息（用于更新 modTime）
	info, err := os.Stat(path)
	if err != nil {
		// 文件不存在或无法访问，使用当前时间
		c.mu.Lock()
		c.hashes[path] = hashStr
		c.modTimes[path] = time.Now()
		c.contents[path] = content
		c.mu.Unlock()
		return
	}

	// 更新缓存
	c.mu.Lock()
	c.hashes[path] = hashStr
	c.modTimes[path] = info.ModTime()
	c.contents[path] = content
	c.mu.Unlock()
}

// Clear 清空所有缓存
func (c *FileCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hashes = make(map[string]string)
	c.modTimes = make(map[string]time.Time)
	c.contents = make(map[string]string)
}
