// Package tools 提供 LSP（语言服务器协议）集成工具
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yukin/kore/internal/lsp"
	"github.com/yukin/kore/pkg/logger"
)

// LSPManager 管理 LSP 服务器连接
type LSPManager struct {
	manager *lsp.Manager
	log     *logger.Logger
	roots   map[string]string // 文件路径 -> 项目根目录
	mu      sync.RWMutex
}

// NewLSPManager 创建新的 LSP 管理器
func NewLSPManager(projectRoot string, log *logger.Logger) *LSPManager {
	return &LSPManager{
		manager: lsp.NewManager(&lsp.ManagerConfig{
			RootPath: projectRoot,
		}, log),
		log:   log,
		roots: make(map[string]string),
	}
}

// Start 启动 LSP 管理器
func (m *LSPManager) Start(ctx context.Context) error {
	return m.manager.Start(ctx)
}

// Stop 停止 LSP 管理器
func (m *LSPManager) Stop(ctx context.Context) error {
	return m.manager.Stop(ctx)
}

// GetClient 获取或创建 LSP 客户端
func (m *LSPManager) GetClient(ctx context.Context, filePath string) (*lsp.Client, error) {
	// 检测语言
	languageID := lsp.GetLanguageID(filePath)
	if languageID == "plaintext" {
		return nil, fmt.Errorf("不支持的文件类型: %s", filepath.Base(filePath))
	}

	// 获取或创建客户端
	client, err := m.manager.GetOrCreateClient(ctx, languageID)
	if err != nil {
		return nil, fmt.Errorf("获取 LSP 客户端失败: %w", err)
	}

	return client, nil
}

// LSPCompletionTool 代码补全工具
type LSPCompletionTool struct {
	lspManager *LSPManager
	projectRoot string
}

// NewLSPCompletionTool 创建代码补全工具
func NewLSPCompletionTool(lspManager *LSPManager, projectRoot string) *LSPCompletionTool {
	return &LSPCompletionTool{
		lspManager:  lspManager,
		projectRoot: projectRoot,
	}
}

func (t *LSPCompletionTool) Name() string {
	return "lsp_completion"
}

func (t *LSPCompletionTool) Description() string {
	return "获取代码补全建议（基于语言服务器）"
}

func (t *LSPCompletionTool) Schema() string {
	return `{
		"name": "lsp_completion",
		"description": "获取代码补全建议（基于语言服务器）",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径（相对于项目根目录）"
				},
				"line": {
					"type": "integer",
					"description": "行号（0-indexed）"
				},
				"character": {
					"type": "integer",
					"description": "字符位置（0-indexed）"
				}
			},
			"required": ["path", "line", "character"]
		}
	}`
}

func (t *LSPCompletionTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 获取 LSP 客户端
	client, err := t.lspManager.GetClient(ctx, params.Path)
	if err != nil {
		return "", err
	}

	// 构建文件 URI
	uri := lsp.PathToURI(filepath.Join(t.projectRoot, params.Path))

	// 请求补全
	completion, err := client.Completion(ctx, uri, lsp.Position{
		Line:      params.Line,
		Character: params.Character,
	})
	if err != nil {
		return "", fmt.Errorf("补全请求失败: %w", err)
	}

	// 格式化结果
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个补全项:\n", len(completion.Items)))

	for i, item := range completion.Items {
		if i >= 20 { // 限制显示数量
			sb.WriteString(fmt.Sprintf("... 还有 %d 项\n", len(completion.Items)-20))
			break
		}

		sb.WriteString(fmt.Sprintf("%d. %s", i+1, item.Label))
		if item.Detail != "" {
			sb.WriteString(fmt.Sprintf(" - %s", item.Detail))
		}
		if item.Kind != 0 {
			sb.WriteString(fmt.Sprintf(" [%d]", item.Kind))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// LSPDefinitionTool 跳转到定义工具
type LSPDefinitionTool struct {
	lspManager *LSPManager
	projectRoot string
}

// NewLSPDefinitionTool 创建跳转到定义工具
func NewLSPDefinitionTool(lspManager *LSPManager, projectRoot string) *LSPDefinitionTool {
	return &LSPDefinitionTool{
		lspManager:  lspManager,
		projectRoot: projectRoot,
	}
}

func (t *LSPDefinitionTool) Name() string {
	return "lsp_definition"
}

func (t *LSPDefinitionTool) Description() string {
	return "跳转到符号定义（基于语言服务器）"
}

func (t *LSPDefinitionTool) Schema() string {
	return `{
		"name": "lsp_definition",
		"description": "跳转到符号定义（基于语言服务器）",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径（相对于项目根目录）"
				},
				"line": {
					"type": "integer",
					"description": "行号（0-indexed）"
				},
				"character": {
					"type": "integer",
					"description": "字符位置（0-indexed）"
				}
			},
			"required": ["path", "line", "character"]
		}
	}`
}

func (t *LSPDefinitionTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 获取 LSP 客户端
	client, err := t.lspManager.GetClient(ctx, params.Path)
	if err != nil {
		return "", err
	}

	// 构建文件 URI
	uri := lsp.PathToURI(filepath.Join(t.projectRoot, params.Path))

	// 请求定义
	locations, err := client.Definition(ctx, uri, lsp.Position{
		Line:      params.Line,
		Character: params.Character,
	})
	if err != nil {
		return "", fmt.Errorf("定义请求失败: %w", err)
	}

	// 格式化结果
	if len(locations) == 0 {
		return "未找到定义", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个定义:\n", len(locations)))

	for i, loc := range locations {
		sb.WriteString(fmt.Sprintf("%d. %s:%d:%d\n",
			i+1,
			loc.URI,
			loc.Range.Start.Line+1, // 转换为 1-indexed
			loc.Range.Start.Character+1,
		))
	}

	return sb.String(), nil
}

// LSPReferencesTool 查找引用工具
type LSPReferencesTool struct {
	lspManager *LSPManager
	projectRoot string
}

// NewLSPReferencesTool 创建查找引用工具
func NewLSPReferencesTool(lspManager *LSPManager, projectRoot string) *LSPReferencesTool {
	return &LSPReferencesTool{
		lspManager:  lspManager,
		projectRoot: projectRoot,
	}
}

func (t *LSPReferencesTool) Name() string {
	return "lsp_references"
}

func (t *LSPReferencesTool) Description() string {
	return "查找符号引用（基于语言服务器）"
}

func (t *LSPReferencesTool) Schema() string {
	return `{
		"name": "lsp_references",
		"description": "查找符号引用（基于语言服务器）",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径（相对于项目根目录）"
				},
				"line": {
					"type": "integer",
					"description": "行号（0-indexed）"
				},
				"character": {
					"type": "integer",
					"description": "字符位置（0-indexed）"
				}
			},
			"required": ["path", "line", "character"]
		}
	}`
}

func (t *LSPReferencesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 获取 LSP 客户端
	client, err := t.lspManager.GetClient(ctx, params.Path)
	if err != nil {
		return "", err
	}

	// 构建文件 URI
	uri := lsp.PathToURI(filepath.Join(t.projectRoot, params.Path))

	// 请求引用
	locations, err := client.References(ctx, uri, lsp.Position{
		Line:      params.Line,
		Character: params.Character,
	})
	if err != nil {
		return "", fmt.Errorf("引用请求失败: %w", err)
	}

	// 格式化结果
	if len(locations) == 0 {
		return "未找到引用", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个引用:\n", len(locations)))

	for i, loc := range locations {
		sb.WriteString(fmt.Sprintf("%d. %s:%d:%d\n",
			i+1,
			loc.URI,
			loc.Range.Start.Line+1, // 转换为 1-indexed
			loc.Range.Start.Character+1,
		))
	}

	return sb.String(), nil
}

// LSPHoverTool 悬停提示工具
type LSPHoverTool struct {
	lspManager *LSPManager
	projectRoot string
}

// NewLSPHoverTool 创建悬停提示工具
func NewLSPHoverTool(lspManager *LSPManager, projectRoot string) *LSPHoverTool {
	return &LSPHoverTool{
		lspManager:  lspManager,
		projectRoot: projectRoot,
	}
}

func (t *LSPHoverTool) Name() string {
	return "lsp_hover"
}

func (t *LSPHoverTool) Description() string {
	return "获取符号悬停信息（基于语言服务器）"
}

func (t *LSPHoverTool) Schema() string {
	return `{
		"name": "lsp_hover",
		"description": "获取符号悬停信息（基于语言服务器）",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径（相对于项目根目录）"
				},
				"line": {
					"type": "integer",
					"description": "行号（0-indexed）"
				},
				"character": {
					"type": "integer",
					"description": "字符位置（0-indexed）"
				}
			},
			"required": ["path", "line", "character"]
		}
	}`
}

func (t *LSPHoverTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		Line      int    `json:"line"`
		Character int    `json:"character"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 获取 LSP 客户端
	client, err := t.lspManager.GetClient(ctx, params.Path)
	if err != nil {
		return "", err
	}

	// 构建文件 URI
	uri := lsp.PathToURI(filepath.Join(t.projectRoot, params.Path))

	// 请求悬停信息
	hover, err := client.Hover(ctx, uri, lsp.Position{
		Line:      params.Line,
		Character: params.Character,
	})
	if err != nil {
		return "", fmt.Errorf("悬停请求失败: %w", err)
	}

	// 检查是否有内容
	if hover.Contents == nil {
		return "无悬停信息", nil
	}

	// 格式化结果
	return formatHoverContent(hover.Contents)
}

// formatHoverContent 格式化悬停内容
func formatHoverContent(content interface{}) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case map[string]interface{}:
		// 可能是 MarkupContent
		if kind, ok := v["kind"].(string); ok {
			if value, ok := v["value"].(string); ok {
				return fmt.Sprintf("[%s]\n%s", kind, value), nil
			}
		}
		// 尝试直接序列化
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case []interface{}:
		// 可能是字符串数组
		var parts []string
		for _, item := range v {
			if str, ok := item.(string); ok {
				parts = append(parts, str)
			}
		}
		return strings.Join(parts, "\n"), nil
	default:
		// 尝试序列化
		data, err := json.MarshalIndent(content, "", "  ")
		if err != nil {
			return "", fmt.Errorf("无法格式化悬停内容: %T", content)
		}
		return string(data), nil
	}
}
