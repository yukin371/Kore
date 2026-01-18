package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yukin371/Kore/internal/infrastructure/fs"
)

// ListFilesTool 实现文件列表功能
type ListFilesTool struct {
	projectRoot string
	fs          *SecurityInterceptor
	maxDepth    int
	maxFiles    int
}

// NewListFilesTool 创建文件列表工具
func NewListFilesTool(projectRoot string, fs *SecurityInterceptor) *ListFilesTool {
	return &ListFilesTool{
		projectRoot: projectRoot,
		fs:          fs,
		maxDepth:    10,    // 最大深度
		maxFiles:    1000,  // 最大文件数
	}
}

// Name 返回工具名称
func (t *ListFilesTool) Name() string {
	return "list_files"
}

// Description 返回工具描述
func (t *ListFilesTool) Description() string {
	return "列出项目目录结构和文件。支持递归遍历、路径过滤、文件类型过滤等。"
}

// Schema 返回工具的参数 JSON Schema
func (t *ListFilesTool) Schema() string {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "起始路径（相对于项目根目录），默认为当前目录",
			},
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "文件名模式过滤（例如：*.go, *.md），可选",
			},
			"recursive": map[string]interface{}{
				"type":        "boolean",
				"description": "是否递归遍历子目录，默认为 true",
			},
			"max_depth": map[string]interface{}{
				"type":        "integer",
				"description": "最大递归深度，默认为 10",
			},
			"show_hidden": map[string]interface{}{
				"type":        "boolean",
				"description": "是否显示隐藏文件（以.开头），默认为 false",
			},
		},
	}

	jsonBytes, _ := json.Marshal(schema)
	return string(jsonBytes)
}

// Execute 执行文件列表
func (t *ListFilesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// 解析参数
	var params struct {
		Path       string `json:"path,omitempty"`
		Pattern    string `json:"pattern,omitempty"`
		Recursive  bool   `json:"recursive,omitempty"`
		MaxDepth   int    `json:"max_depth,omitempty"`
		ShowHidden bool   `json:"show_hidden,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 设置默认值
	if params.Path == "" {
		params.Path = "."
	}
	if !params.Recursive {
		params.MaxDepth = 1
	}
	if params.MaxDepth == 0 {
		params.MaxDepth = t.maxDepth
	}

	// 验证路径
	safePath, err := t.fs.ValidatePath(params.Path)
	if err != nil {
		return "", fmt.Errorf("路径验证失败: %w", err)
	}

	// 列出文件
	files, err := t.listFiles(safePath, params.Pattern, params.MaxDepth, params.ShowHidden)
	if err != nil {
		return "", fmt.Errorf("列出文件失败: %w", err)
	}

	// 格式化结果
	return t.formatResults(files, params.Path), nil
}

// ToolFileInfo 工具使用的文件信息类型
type ToolFileInfo struct {
	Path     string // 相对路径
	FullPath string // 完整路径
	IsDir    bool   // 是否为目录
	Size     int64  // 文件大小（字节）
}

// listFiles 列出文件
func (t *ListFilesTool) listFiles(path, pattern string, maxDepth int, showHidden bool) ([]ToolFileInfo, error) {
	// 使用 fs.FastWalk 遍历文件
	config := fs.WalkConfig{
		Root:       path,
		MaxDepth:   maxDepth,
		MaxFiles:   t.maxFiles,
		IgnoreFunc: func(relPath string) bool {
			baseName := filepath.Base(relPath)

			// 跳过隐藏文件
			if !showHidden {
				if strings.HasPrefix(baseName, ".") && baseName != "." && baseName != ".." {
					return true
				}
			}

			// 应用文件名模式过滤
			if pattern != "" {
				matched, err := filepath.Match(pattern, baseName)
				if err == nil && !matched {
					return true
				}
			}

			return false
		},
	}

	result, err := fs.FastWalk(config)
	if err != nil {
		return nil, err
	}

	// 转换为 ToolFileInfo 格式
	files := make([]ToolFileInfo, 0, len(result.Files))
	for _, f := range result.Files {
		files = append(files, ToolFileInfo{
			Path:     f.RelPath,
			FullPath: f.Path,
			IsDir:    f.IsDir,
			Size:     f.Size,
		})
	}

	return files, nil
}

// formatResults 格式化结果
func (t *ListFilesTool) formatResults(files []ToolFileInfo, basePath string) string {
	if len(files) == 0 {
		return "未找到文件"
	}

	// 按类型和名称排序
	sort.Slice(files, func(i, j int) bool {
		// 目录优先
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		// 按路径排序
		return files[i].Path < files[j].Path
	})

	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("📁 %s\n\n", basePath))
	builder.WriteString(fmt.Sprintf("共 %d 个项目\n\n", len(files)))

	// 按目录分组
	currentDir := ""
	for _, file := range files {
		// 获取文件所在目录
		fileDir := filepath.Dir(file.Path)
		if fileDir == "." {
			fileDir = ""
		}

		// 如果目录变化，显示新目录标题
		if fileDir != currentDir {
			currentDir = fileDir
			if currentDir != "" {
				builder.WriteString(fmt.Sprintf("\n📂 %s/\n", currentDir))
			}
		}

		// 格式化文件信息
		if file.IsDir {
			builder.WriteString(fmt.Sprintf("  📁 %s/\n", filepath.Base(file.Path)))
		} else {
			size := t.formatSize(file.Size)
			builder.WriteString(fmt.Sprintf("  📄 %s (%s)\n", filepath.Base(file.Path), size))
		}
	}

	return builder.String()
}

// formatSize 格式化文件大小
func (t *ListFilesTool) formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
