// Package core 提供 AGENTS.md 自动注入功能
//
// 向上遍历目录树，收集所有 AGENTS.md 文件并注入上下文
// 灵感来自: https://github.com/code-yeongyu/oh-my-opencode
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AGENTSMDLoader AGENTS.md 文件加载器
type AGENTSMDLoader struct {
	cache      map[string]*CachedAgentsMD
	cacheMutex sync.RWMutex
	enabled    bool
	maxDepth   int // 最大向上遍历深度
	projectRoot string
}

// CachedAgentsMD 缓存的 AGENTS.md 内容
type CachedAgentsMD struct {
	Path      string
	Content   string
	LoadedAt  time.Time
	Priority  int // 优先级，越小越优先
}

// NewAGENTSMDLoader 创建新的 AGENTS.md 加载器
func NewAGENTSMDLoader(projectRoot string) *AGENTSMDLoader {
	return &AGENTSMDLoader{
		cache:       make(map[string]*CachedAgentsMD),
		enabled:     true,
		maxDepth:    10, // 默认最多向上 10 层
		projectRoot: projectRoot,
	}
}

// Enable 启用 AGENTS.md 加载
func (loader *AGENTSMDLoader) Enable() {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	loader.enabled = true
}

// Disable 禁用 AGENTS.md 加载
func (loader *AGENTSMDLoader) Disable() {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	loader.enabled = false
}

// SetMaxDepth 设置最大向上遍历深度
func (loader *AGENTSMDLoader) SetMaxDepth(depth int) {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	loader.maxDepth = depth
}

// LoadFromDirectory 从指定目录加载所有 AGENTS.md 文件
func (loader *AGENTSMDLoader) LoadFromDirectory(startPath string) ([]*CachedAgentsMD, error) {
	if !loader.enabled {
		return []*CachedAgentsMD{}, nil
	}

	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()

	var agentsMDs []*CachedAgentsMD

	// 向上遍历目录树
	currentDir := startPath
	priority := 0

	for i := 0; i <= loader.maxDepth; i++ {
		// 检查 AGENTS.md 文件是否存在
		agentsMDPath := filepath.Join(currentDir, "AGENTS.md")

		// 检查缓存
		if cached, ok := loader.cache[agentsMDPath]; ok {
			// 检查缓存是否过期（1小时）
			if time.Since(cached.LoadedAt) < time.Hour {
				// 更新优先级
				cached.Priority = priority
				agentsMDs = append(agentsMDs, cached)
				priority++
				continue
			}
		}

		// 读取文件
		if content, err := loader.readAgentsMD(agentsMDPath); err == nil {
			cached := &CachedAgentsMD{
				Path:     agentsMDPath,
				Content:  content,
				LoadedAt: time.Now(),
				Priority: priority,
			}

			// 添加到缓存
			loader.cache[agentsMDPath] = cached
			agentsMDs = append(agentsMDs, cached)
			priority++
		}

		// 移动到父目录
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// 已到达根目录
			break
		}
		currentDir = parentDir
	}

	return agentsMDs, nil
}

// readAgentsMD 读取 AGENTS.md 文件
func (loader *AGENTSMDLoader) readAgentsMD(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// GenerateContext 生成包含所有 AGENTS.md 的上下文
func (loader *AGENTSMDLoader) GenerateContext(startPath string) (string, error) {
	agentsMDs, err := loader.LoadFromDirectory(startPath)
	if err != nil {
		return "", err
	}

	if len(agentsMDs) == 0 {
		return "", nil
	}

	var context strings.Builder

	context.WriteString("\n## 📋 AGENTS.md 上下文\n\n")
	context.WriteString(fmt.Sprintf("已加载 %d 个 AGENTS.md 文件（按优先级排序）：\n\n", len(agentsMDs)))

	// 按优先级排序
	for i, agentsMD := range agentsMDs {
		relPath, err := filepath.Rel(startPath, agentsMD.Path)
		if err != nil {
			relPath = agentsMD.Path
		}

		context.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, relPath))
		context.WriteString("```\n")
		context.WriteString(agentsMD.Content)
		context.WriteString("\n```\n\n")
	}

	return context.String(), nil
}

// ClearCache 清空缓存
func (loader *AGENTSMDLoader) ClearCache() {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()

	loader.cache = make(map[string]*CachedAgentsMD)
}

// GetCachedCount 获取缓存中的文件数量
func (loader *AGENTSMDLoader) GetCachedCount() int {
	loader.cacheMutex.RLock()
	defer loader.cacheMutex.RUnlock()

	return len(loader.cache)
}

// IsEnabled 检查是否启用
func (loader *AGENTSMDLoader) IsEnabled() bool {
	loader.cacheMutex.RLock()
	defer loader.cacheMutex.RUnlock()

	return loader.enabled
}

// GetStats 获取统计信息
func (loader *AGENTSMDLoader) GetStats() *AGENTSMDStats {
	loader.cacheMutex.RLock()
	defer loader.cacheMutex.RUnlock()

	stats := &AGENTSMDStats{
		Enabled:     loader.enabled,
		CachedCount: len(loader.cache),
		MaxDepth:    loader.maxDepth,
	}

	// 计算总大小
	for _, cached := range loader.cache {
		stats.TotalSize += len(cached.Content)
	}

	return stats
}

// AGENTSMDStats AGENTS.md 统计信息
type AGENTSMDStats struct {
	Enabled     bool
	CachedCount int
	TotalSize   int
	MaxDepth    int
}

// FindAllAgentsMD 查找指定目录及其子目录中的所有 AGENTS.md 文件
func (loader *AGENTSMDLoader) FindAllAgentsMD(rootPath string) ([]string, error) {
	var agentsMDPaths []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过隐藏目录和 node_modules 等
		if info.IsDir() {
			baseName := filepath.Base(path)
			if strings.HasPrefix(baseName, ".") ||
				baseName == "node_modules" ||
				baseName == "vendor" ||
				baseName == "target" ||
				baseName == "bin" ||
				baseName == "obj" {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否是 AGENTS.md 文件
		if filepath.Base(path) == "AGENTS.md" {
			relPath, err := filepath.Rel(rootPath, path)
			if err != nil {
				relPath = path
			}
			agentsMDPaths = append(agentsMDPaths, relPath)
		}

		return nil
	})

	return agentsMDPaths, err
}

// RefreshCache 刷新缓存（重新加载所有文件）
func (loader *AGENTSMDLoader) RefreshCache(startPath string) error {
	loader.ClearCache()

	_, err := loader.LoadFromDirectory(startPath)
	return err
}

// Validate 验证 AGENTS.md 文件格式
func (loader *AGENTSMDLoader) Validate(content string) []string {
	var warnings []string

	// 检查文件是否为空
	if strings.TrimSpace(content) == "" {
		warnings = append(warnings, "文件内容为空")
	}

	// 检查是否包含标题
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && !strings.HasPrefix(lines[0], "#") {
		warnings = append(warnings, "建议在文件开头添加标题（# 标题）")
	}

	// 检查文件大小
	if len(content) > 10000 { // 10KB
		warnings = append(warnings, "文件较大，建议精简内容以提高性能")
	}

	return warnings
}
