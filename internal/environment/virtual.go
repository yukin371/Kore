package environment

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// VirtualFileSystem 虚拟文件系统
// 用于在内存中管理文档，支持预览后确认再写入
type VirtualFileSystem struct {
	mu        sync.RWMutex
	documents map[string]*VirtualDocument
}

// VirtualDocument 虚拟文档
type VirtualDocument struct {
	Path        string      // 文档路径
	Content     []byte      // 文档内容
	CreatedAt   time.Time   // 创建时间
	UpdatedAt   time.Time   // 更新时间
	IsCommitted bool        // 是否已提交到磁盘
	Metadata    map[string]interface{} // 元数据
}

// NewVirtualFileSystem 创建虚拟文件系统
func NewVirtualFileSystem() *VirtualFileSystem {
	return &VirtualFileSystem{
		documents: make(map[string]*VirtualDocument),
	}
}

// Create 创建虚拟文档
func (vfs *VirtualFileSystem) Create(path string, content []byte) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	// 检查是否已存在
	if _, exists := vfs.documents[path]; exists {
		return fmt.Errorf("虚拟文档已存在: %s", path)
	}

	// 创建新文档
	doc := &VirtualDocument{
		Path:        path,
		Content:     content,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsCommitted: false,
		Metadata:    make(map[string]interface{}),
	}

	vfs.documents[path] = doc

	return nil
}

// Read 读取虚拟文档
func (vfs *VirtualFileSystem) Read(path string) ([]byte, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	doc, exists := vfs.documents[path]
	if !exists {
		return nil, fmt.Errorf("虚拟文档不存在: %s", path)
	}

	return doc.Content, nil
}

// Update 更新虚拟文档
func (vfs *VirtualFileSystem) Update(path string, content []byte) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	doc, exists := vfs.documents[path]
	if !exists {
		return fmt.Errorf("虚拟文档不存在: %s", path)
	}

	doc.Content = content
	doc.UpdatedAt = time.Now()

	return nil
}

// Delete 删除虚拟文档
func (vfs *VirtualFileSystem) Delete(path string) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if _, exists := vfs.documents[path]; !exists {
		return fmt.Errorf("虚拟文档不存在: %s", path)
	}

	delete(vfs.documents, path)

	return nil
}

// List 列出所有虚拟文档
func (vfs *VirtualFileSystem) List() ([]string, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	paths := make([]string, 0, len(vfs.documents))
	for path := range vfs.documents {
		paths = append(paths, path)
	}

	return paths, nil
}

// Exists 检查文档是否存在
func (vfs *VirtualFileSystem) Exists(path string) bool {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	_, exists := vfs.documents[path]
	return exists
}

// GetMetadata 获取文档元数据
func (vfs *VirtualFileSystem) GetMetadata(path string) (map[string]interface{}, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	doc, exists := vfs.documents[path]
	if !exists {
		return nil, fmt.Errorf("虚拟文档不存在: %s", path)
	}

	return doc.Metadata, nil
}

// SetMetadata 设置文档元数据
func (vfs *VirtualFileSystem) SetMetadata(path string, metadata map[string]interface{}) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	doc, exists := vfs.documents[path]
	if !exists {
		return fmt.Errorf("虚拟文档不存在: %s", path)
	}

	doc.Metadata = metadata

	return nil
}

// GetDocument 获取完整文档信息
func (vfs *VirtualFileSystem) GetDocument(path string) (*VirtualDocument, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	doc, exists := vfs.documents[path]
	if !exists {
		return nil, fmt.Errorf("虚拟文档不存在: %s", path)
	}

	// 返回副本，避免外部修改
	docCopy := &VirtualDocument{
		Path:        doc.Path,
		Content:     make([]byte, len(doc.Content)),
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
		IsCommitted: doc.IsCommitted,
		Metadata:    make(map[string]interface{}),
	}

	copy(docCopy.Content, doc.Content)

	for k, v := range doc.Metadata {
		docCopy.Metadata[k] = v
	}

	return docCopy, nil
}

// Diff 对比虚拟文档和磁盘文件的差异
func (vfs *VirtualFileSystem) Diff(path string, diskContent []byte) (*DiffResult, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	doc, exists := vfs.documents[path]
	if !exists {
		return nil, fmt.Errorf("虚拟文档不存在: %s", path)
	}

	// 简单对比：是否有差异
	hasDiff := len(doc.Content) != len(diskContent)
	if !hasDiff {
		for i := range doc.Content {
			if doc.Content[i] != diskContent[i] {
				hasDiff = true
				break
			}
		}
	}

	result := &DiffResult{
		Path1:   path + " (virtual)",
		Path2:   path + " (disk)",
		HasDiff: hasDiff,
	}

	// 这里可以集成更复杂的 diff 算法
	// 目前只返回是否有差异

	return result, nil
}

// Commit 提交虚拟文档到磁盘（需要外部实现）
// 这个方法只标记文档为已提交，实际的写入操作由 LocalEnvironment 完成
func (vfs *VirtualFileSystem) Commit(path string) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	doc, exists := vfs.documents[path]
	if !exists {
		return fmt.Errorf("虚拟文档不存在: %s", path)
	}

	doc.IsCommitted = true
	doc.UpdatedAt = time.Now()

	return nil
}

// Rollback 回滚虚拟文档（删除未提交的更改）
func (vfs *VirtualFileSystem) Rollback(path string) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if _, exists := vfs.documents[path]; !exists {
		return fmt.Errorf("虚拟文档不存在: %s", path)
	}

	// 删除未提交的文档
	delete(vfs.documents, path)

	return nil
}

// Clear 清空所有虚拟文档
func (vfs *VirtualFileSystem) Clear() error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	vfs.documents = make(map[string]*VirtualDocument)

	return nil
}

// GetStats 获取虚拟文件系统统计信息
func (vfs *VirtualFileSystem) GetStats() map[string]interface{} {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_documents"] = len(vfs.documents)
	stats["committed"] = 0
	stats["uncommitted"] = 0
	stats["total_size"] = 0

	for _, doc := range vfs.documents {
		if doc.IsCommitted {
			stats["committed"] = stats["committed"].(int) + 1
		} else {
			stats["uncommitted"] = stats["uncommitted"].(int) + 1
		}
		stats["total_size"] = stats["total_size"].(int) + len(doc.Content)
	}

	return stats
}

// GroupByDirectory 按目录分组文档
func (vfs *VirtualFileSystem) GroupByDirectory() map[string][]string {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	groups := make(map[string][]string)

	for path := range vfs.documents {
		dir := filepath.Dir(path)
		groups[dir] = append(groups[dir], path)
	}

	return groups
}
