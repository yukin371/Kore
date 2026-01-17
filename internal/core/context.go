package core

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/saracen/walker"
)

// File represents a file with its content
type File struct {
	Path    string
	Content string
}

// ProjectContext represents the collected project context
type ProjectContext struct {
	FileTree     string   // Complete directory tree
	FocusedFiles []File   // Focused files with full content
	TotalTokens  int      // Estimated token count
}

// ContextManager manages project context with layered strategy
type ContextManager struct {
	projectRoot  string
	ignoreMatcher *IgnoreMatcher
	focusedPaths  map[string]bool
	focusLRU      *LRU
	maxTokens     int
	maxTreeDepth  int
	maxFilesPerDir int
	mu            sync.RWMutex
}

// IgnoreMatcher 处理 .gitignore 样式的模式匹配
type IgnoreMatcher struct {
	patterns    []string
	projectRoot string
}

// NewIgnoreMatcher 创建新的忽略模式匹配器
func NewIgnoreMatcher(projectRoot string) *IgnoreMatcher {
	gitignorePath := filepath.Join(projectRoot, ".gitignore")

	// 默认忽略规则
	defaultPatterns := []string{
		".git",
		"node_modules",
		".DS_Store",
		"*.tmp",
		"*.log",
		"vendor",
		"bin",
		"obj",
		".idea",
		".vscode",
		"*.exe",
		"*.dll",
		"*.so",
		"*.dylib",
		"*.test",
		"__pycache__",
		"*.pyc",
		".pytest_cache",
		".mypy_cache",
		"dist",
		"build",
		"target",
		"*.o",
		"a.out",
	}

	// 收集所有模式
	allPatterns := make([]string, 0, len(defaultPatterns))
	allPatterns = append(allPatterns, defaultPatterns...)

	// 如果存在 .gitignore 文件，加载它
	if _, err := os.Stat(gitignorePath); err == nil {
		patterns, err := os.ReadFile(gitignorePath)
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(patterns)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				// 跳过空行和注释
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				allPatterns = append(allPatterns, line)
			}
		}
	}

	return &IgnoreMatcher{
		patterns:    allPatterns,
		projectRoot: projectRoot,
	}
}

// ShouldIgnore 返回 true 如果路径应该被忽略
func (im *IgnoreMatcher) ShouldIgnore(path string) bool {
	// 转换为相对路径
	relPath := path
	if filepath.IsAbs(path) {
		var err error
		relPath, err = filepath.Rel(im.projectRoot, path)
		if err != nil {
			return false
		}
	}

	// 检查是否匹配任意模式
	for _, pattern := range im.patterns {
		// 精确匹配目录名
		if relPath == pattern || filepath.Base(relPath) == pattern {
			return true
		}

		// 通配符匹配
		if strings.HasPrefix(pattern, "*") {
			ext := "." + pattern[1:]
			if filepath.Ext(relPath) == ext {
				return true
			}
		}

		// 路径包含匹配
		if strings.Contains(relPath, pattern) {
			return true
		}
	}

	return false
}

// LRU is a simple Least Recently Used cache
type LRU struct {
	items map[string]int
	order []string
	maxSize int
}

// NewLRU creates a new LRU cache
func NewLRU(maxSize int) *LRU {
	return &LRU{
		items: make(map[string]int),
		order: make([]string, 0),
		maxSize: maxSize,
	}
}

// Add adds an item to the LRU
func (l *LRU) Add(key string) {
	if _, exists := l.items[key]; exists {
		// Move to end (most recently used)
		l.order = append(l.order, key)
		l.items[key] = len(l.order) - 1
		return
	}

	l.order = append(l.order, key)
	l.items[key] = len(l.order) - 1

	// Evict if over capacity
	if len(l.order) > l.maxSize {
		oldest := l.order[0]
		delete(l.items, oldest)
		l.order = l.order[1:]
	}
}

// GetOldest removes and returns the oldest item
func (l *LRU) GetOldest() string {
	if len(l.order) == 0 {
		return ""
	}
	oldest := l.order[0]
	delete(l.items, oldest)
	l.order = l.order[1:]
	return oldest
}

// Touch marks an item as recently used
func (l *LRU) Touch(key string) {
	if _, exists := l.items[key]; exists {
		l.Add(key)
	}
}

// NewContextManager 创建新的上下文管理器
func NewContextManager(projectRoot string, maxTokens int) *ContextManager {
	return &ContextManager{
		projectRoot:   projectRoot,
		ignoreMatcher: NewIgnoreMatcher(projectRoot),
		focusedPaths:  make(map[string]bool),
		focusLRU:      NewLRU(20), // 最多跟踪 20 个焦点文件
		maxTokens:     maxTokens,
		maxTreeDepth:  5,
		maxFilesPerDir: 50,
	}
}

// BuildContext constructs the project context
func (c *ContextManager) BuildContext(ctx context.Context) (*ProjectContext, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fileTree, err := c.buildFileTree()
	if err != nil {
		return nil, fmt.Errorf("failed to build file tree: %w", err)
	}

	focusedFiles := c.getFocusedFiles()
	totalTokens := c.estimateTokens(fileTree, focusedFiles)

	return &ProjectContext{
		FileTree:     fileTree,
		FocusedFiles: focusedFiles,
		TotalTokens:  totalTokens,
	}, nil
}

// buildFileTree 生成带层次结构的目录树
func (c *ContextManager) buildFileTree() (string, error) {
	type DirInfo struct {
		Path     string
		RelPath  string
		Files    []string
		Subdirs  map[string]*DirInfo
		Parent   *DirInfo
	}

	root := &DirInfo{
		Path:    c.projectRoot,
		RelPath: ".",
		Subdirs: make(map[string]*DirInfo),
	}

	// 第一遍：构建目录结构
	err := walker.Walk(c.projectRoot, func(path string, fi os.FileInfo) error {
		if fi.IsDir() {
			if c.ignoreMatcher.ShouldIgnore(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if c.ignoreMatcher.ShouldIgnore(path) {
			return nil
		}

		relPath, err := filepath.Rel(c.projectRoot, path)
		if err != nil {
			return nil
		}

		dirPath := filepath.Dir(relPath)
		if dirPath == "." {
			root.Files = append(root.Files, relPath)
		} else {
			// 简化处理：直接添加到列表
			root.Files = append(root.Files, relPath)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	// 限制文件数量
	if len(root.Files) > 1000 {
		root.Files = root.Files[:1000]
	}

	// 生成树形字符串
	var builder strings.Builder
	builder.WriteString("📁 项目结构:\n")

	// 统计文件类型
	fileTypes := make(map[string]int)
	for _, file := range root.Files {
		ext := strings.TrimPrefix(filepath.Ext(file), ".")
		if ext == "" {
			ext = "无扩展名"
		}
		fileTypes[ext]++
	}

	// 按扩展名排序显示
	builder.WriteString("\n文件统计:\n")
	for ext, count := range fileTypes {
		builder.WriteString(fmt.Sprintf("  .%s: %d\n", ext, count))
	}
	builder.WriteString(fmt.Sprintf("\n总计: %d 个文件\n\n", len(root.Files)))

	// 文件列表（分组显示）
	lastDir := ""
	for _, file := range root.Files {
		dir := filepath.Dir(file)
		if dir != lastDir {
			if dir == "." {
				builder.WriteString("根目录:\n")
			} else {
				builder.WriteString(fmt.Sprintf("%s/:\n", dir))
			}
			lastDir = dir
		}
		builder.WriteString(fmt.Sprintf("  - %s\n", filepath.Base(file)))
	}

	if len(root.Files) >= 1000 {
		builder.WriteString("\n... (更多文件未显示)\n")
	}

	return builder.String(), nil
}

// getFocusedFiles retrieves content of all focused files
func (c *ContextManager) getFocusedFiles() []File {
	files := make([]File, 0, len(c.focusedPaths))

	for path := range c.focusedPaths {
		c.focusLRU.Touch(path)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		relPath, _ := filepath.Rel(c.projectRoot, path)
		files = append(files, File{
			Path:    relPath,
			Content: string(content),
		})
	}

	return files
}

// estimateTokens 改进的 token 估算（考虑中文和代码）
func (c *ContextManager) estimateTokens(fileTree string, files []File) int {
	total := len(fileTree) / 3 // 改进：中文约 1 token ≈ 3 字符

	for _, file := range files {
		content := file.Content
		// 检测中文字符比例
		chineseChars := 0
		for _, r := range content {
			if r >= 0x4e00 && r <= 0x9fff { // 中日韩统一表意文字符
				chineseChars++
			}
		}

		// 中文字符按 1/3 计算，其他按 1/4 计算
		chineseTokens := chineseChars / 3
		otherTokens := (len(content) - chineseChars) / 4
		total += int(chineseTokens + otherTokens)
	}

	return total
}

// FilePriority 文件优先级评分
type FilePriority struct {
	Path         string
	Priority     int  // 优先级分数 (0-100)
	Reason       string // 原因说明
}

// calculateFilePriority 计算文件重要性分数
func (c *ContextManager) calculateFilePriority(relPath string) FilePriority {
	priority := 0
	reasons := []string{}

	base := filepath.Base(relPath)
	ext := strings.ToLower(filepath.Ext(relPath))

	// 高优先级文件
	priorityFiles := map[string]int{
		"README.md":     90,
		"README":        90,
		"README.txt":    85,
		"CHANGELOG.md":  80,
		"CONTRIBUTING.md": 75,
		"LICENSE":       70,
		".gitignore":    60,
		"dockerfile":    65,
		"docker-compose.yml": 65,
	}

	if score, exists := priorityFiles[base]; exists {
		priority += score
		reasons = append(reasons, fmt.Sprintf("关键文档 (%s)", base))
	}

	// 重要目录
	importantDirs := []string{"cmd", "internal", "pkg", "api"}
	for _, dir := range importantDirs {
		if strings.HasPrefix(relPath, dir) {
			priority += 30
			reasons = append(reasons, fmt.Sprintf("重要目录 (%s)", dir))
			break
		}
	}

	// 文件扩展名优先级
	extPriority := map[string]int{
		".go":    50,  // Go 源代码
		".md":    40,  // Markdown 文档
		".yaml":  30,  // 配置文件
		".yml":   30,
		".json":  25,
		".txt":   20,
		".mod":   45,  // Go 模块定义
		".sum":   20,
	}

	if score, exists := extPriority[ext]; exists {
		priority += score
		reasons = append(reasons, fmt.Sprintf("源代码/配置 (%s)", ext))
	}

	// 测试文件优先级较低
	testPatterns := []string{"_test.go", "_test.md", "test/", "tests/"}
	for _, pattern := range testPatterns {
		if strings.Contains(strings.ToLower(relPath), pattern) {
			priority -= 20
			reasons = append(reasons, "测试文件")
			break
		}
	}

	// 确保优先级在合理范围内
	if priority < 0 {
		priority = 0
	}
	if priority > 100 {
		priority = 100
	}

	return FilePriority{
		Path:     relPath,
		Priority: priority,
		Reason:   strings.Join(reasons, ", "),
	}
}

// AutoSelectFiles 自动选择重要文件（用于自动填充）
func (c *ContextManager) AutoSelectFiles(maxFiles int) []FilePriority {
	c.mu.Lock()
	defer c.mu.Unlock()

	var priorities []FilePriority

	// 遍历项目文件，计算优先级
	walker.Walk(c.projectRoot, func(path string, fi os.FileInfo) error {
		if fi.IsDir() {
			if c.ignoreMatcher.ShouldIgnore(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if c.ignoreMatcher.ShouldIgnore(path) {
			return nil
		}

		relPath, err := filepath.Rel(c.projectRoot, path)
		if err != nil {
			return nil
		}

		priority := c.calculateFilePriority(relPath)
		priorities = append(priorities, priority)

		return nil
	})

	// 按优先级排序
	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i].Priority > priorities[j].Priority
	})

	// 返回前 N 个
	if len(priorities) > maxFiles {
		priorities = priorities[:maxFiles]
	}

	return priorities
}

// AddFocus adds a file to the focused paths
func (c *ContextManager) AddFocus(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	absPath, err := filepath.Abs(filepath.Join(c.projectRoot, path))
	if err != nil {
		return err
	}

	// Verify path is within project root
	if !strings.HasPrefix(absPath, c.projectRoot+string(os.PathSeparator)) {
		return fmt.Errorf("path outside project root: %s", path)
	}

	c.focusedPaths[absPath] = true
	c.focusLRU.Add(absPath)

	// Check token budget and evict if necessary
	currentTokens := c.estimateTokens("", c.getFocusedFiles())
	for currentTokens > c.maxTokens && len(c.focusedPaths) > 0 {
		oldest := c.focusLRU.GetOldest()
		delete(c.focusedPaths, oldest)
		currentTokens = c.estimateTokens("", c.getFocusedFiles())
	}

	return nil
}

// ReadFile reads a file and adds it to focus
func (c *ContextManager) ReadFile(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(c.projectRoot, path))
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	// Add to focus after successful read
	c.AddFocus(path)

	return string(content), nil
}

// GetProjectRoot returns the project root path
func (c *ContextManager) GetProjectRoot() string {
	return c.projectRoot
}
