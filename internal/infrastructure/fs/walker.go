// Package fs 提供文件系统遍历工具
package fs

import (
	"os"
	"path/filepath"
	"strings"
)

// FileInfo 包含文件信息
type FileInfo struct {
	Path    string
	RelPath string
	IsDir   bool
	Size    int64
}

// WalkConfig 遍历配置
type WalkConfig struct {
	Root        string
	MaxDepth    int
	MaxFiles    int
	IgnoreFunc  func(path string) bool
}

// WalkResult 遍历结果
type WalkResult struct {
	Files    []FileInfo
	Total    int
	Truncated bool
}

// FastWalk 快速遍历目录树
func FastWalk(config WalkConfig) (*WalkResult, error) {
	result := &WalkResult{
		Files: make([]FileInfo, 0),
	}

	err := filepath.Walk(config.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		// 计算相对路径
		relPath, err := filepath.Rel(config.Root, path)
		if err != nil {
			return nil
		}

		// 检查深度
		depth := strings.Count(relPath, string(os.PathSeparator))
		if info.IsDir() && depth > config.MaxDepth {
			return filepath.SkipDir
		}

		// 检查忽略规则
		if config.IgnoreFunc != nil && config.IgnoreFunc(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过根目录
		if path == config.Root {
			return nil
		}

		// 检查文件数量限制
		if !info.IsDir() && len(result.Files) >= config.MaxFiles {
			result.Truncated = true
			return filepath.SkipDir
		}

		// 添加文件信息
		result.Files = append(result.Files, FileInfo{
			Path:    path,
			RelPath: relPath,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
		})
		result.Total++

		return nil
	})

	return result, err
}

// BuildFileTree 构建文件树字符串表示
func BuildFileTree(root string, maxDepth int, ignoreFunc func(path string) bool) (string, error) {
	config := WalkConfig{
		Root:       root,
		MaxDepth:   maxDepth,
		MaxFiles:   1000,
		IgnoreFunc: ignoreFunc,
	}

	result, err := FastWalk(config)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, file := range result.Files {
		if !file.IsDir {
			builder.WriteString(file.RelPath + "\n")
		}
	}

	if result.Truncated {
		builder.WriteString("... (更多文件被截断)\n")
	}

	return builder.String(), nil
}
