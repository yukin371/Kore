// Package tools 提供工具执行框架和安全拦截器
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/yukin371/Kore/internal/core"
)

// Tool 定义工具接口
type Tool interface {
	// Name 返回工具名称
	Name() string

	// Description 返回工具描述
	Description() string

	// Schema 返回工具的 JSON Schema（用于 LLM 理解参数格式）
	Schema() string

	// Execute 执行工具
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolBox 工具箱，管理所有已注册的工具
type ToolBox struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewToolBox 创建新的工具箱
func NewToolBox() *ToolBox {
	return &ToolBox{
		tools: make(map[string]Tool),
	}
}

// Register 注册一个工具
func (tb *ToolBox) Register(tool Tool) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tools[tool.Name()] = tool
}

// Get 获取工具
func (tb *ToolBox) Get(name string) (Tool, bool) {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	tool, ok := tb.tools[name]
	return tool, ok
}

// List 列出所有工具
func (tb *ToolBox) List() []Tool {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	tools := make([]Tool, 0, len(tb.tools))
	for _, tool := range tb.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetToolSchemas 返回所有工具的 JSON Schema（用于发送给 LLM）
func (tb *ToolBox) GetToolSchemas() string {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	var schemas []string
	for _, tool := range tb.tools {
		schemas = append(schemas, tool.Schema())
	}
	return strings.Join(schemas, "\n")
}

// ToolExecutor 工具执行器，整合安全拦截和工具调用
type ToolExecutor struct {
	toolbox  *ToolBox
	security *SecurityInterceptor
	projectRoot string
}

// NewToolExecutor 创建新的工具执行器
func NewToolExecutor(projectRoot string) *ToolExecutor {
	te := &ToolExecutor{
		toolbox:  NewToolBox(),
		security: NewSecurityInterceptor(projectRoot),
		projectRoot: projectRoot,
	}

	// 注册默认工具
	te.RegisterDefaultTools()

	return te
}

// RegisterDefaultTools 注册默认工具集
func (te *ToolExecutor) RegisterDefaultTools() {
	te.RegisterTool(&ReadFileTool{security: te.security, projectRoot: te.projectRoot})
	te.RegisterTool(&WriteFileTool{security: te.security, projectRoot: te.projectRoot})
	te.RegisterTool(&RunCommandTool{security: te.security})
	te.RegisterTool(NewSearchFilesTool(te.projectRoot, te.security))
	te.RegisterTool(NewListFilesTool(te.projectRoot, te.security))
}

// RegisterTool 注册自定义工具
func (te *ToolExecutor) RegisterTool(tool Tool) {
	te.toolbox.Register(tool)
}

// Execute 执行工具调用
func (te *ToolExecutor) Execute(ctx context.Context, call core.ToolCall) (string, error) {
	// 获取工具
	tool, ok := te.toolbox.Get(call.Name)
	if !ok {
		return "", fmt.Errorf("未知工具: %s", call.Name)
	}

	// 解析参数
	var args json.RawMessage
	args = json.RawMessage(call.Arguments)

	// 执行工具
	result, err := tool.Execute(ctx, args)
	if err != nil {
		return "", err
	}

	return result, nil
}

// GetToolSchemas 获取所有工具的 Schema
func (te *ToolExecutor) GetToolSchemas() string {
	return te.toolbox.GetToolSchemas()
}

// ==================== 基础工具实现 ====================

// ReadFileTool 读取文件工具
type ReadFileTool struct {
	security    *SecurityInterceptor
	projectRoot string
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "读取文件内容，支持指定行范围"
}

func (t *ReadFileTool) Schema() string {
	return `{
		"name": "read_file",
		"description": "读取文件内容，支持指定行范围",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径（相对于项目根目录）"
				},
				"line_start": {
					"type": "integer",
					"description": "起始行号（可选，0-indexed）"
				},
				"line_end": {
					"type": "integer",
					"description": "结束行号（可选，0-indexed）"
				}
			},
			"required": ["path"]
		}
	}`
}

func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		LineStart int    `json:"line_start,omitempty"`
		LineEnd   int    `json:"line_end,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 验证路径安全性
	safePath, err := t.security.ValidatePath(params.Path)
	if err != nil {
		return "", err
	}

	// 读取文件
	content, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	// 如果指定了行范围，进行切片
	lines := strings.Split(string(content), "\n")
	start := params.LineStart
	end := params.LineEnd

	if start < 0 {
		start = 0
	}
	if end > len(lines) || end == 0 {
		end = len(lines)
	}
	if start > end {
		start = end
	}

	if start > 0 || end < len(lines) {
		return strings.Join(lines[start:end], "\n"), nil
	}

	return string(content), nil
}

// WriteFileTool 写入文件工具
type WriteFileTool struct {
	security    *SecurityInterceptor
	projectRoot string
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "写入文件内容（完全覆盖）"
}

func (t *WriteFileTool) Schema() string {
	return `{
		"name": "write_file",
		"description": "写入文件内容（完全覆盖现有内容）",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "文件路径（相对于项目根目录）"
				},
				"content": {
					"type": "string",
					"description": "要写入的完整内容"
				}
			},
			"required": ["path", "content"]
		}
	}`
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 验证路径安全性
	safePath, err := t.security.ValidatePath(params.Path)
	if err != nil {
		return "", err
	}

	// 写入文件
	if err := os.WriteFile(safePath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return "文件写入成功", nil
}

// RunCommandTool 执行命令工具
type RunCommandTool struct {
	security *SecurityInterceptor
}

func (t *RunCommandTool) Name() string {
	return "run_command"
}

func (t *RunCommandTool) Description() string {
	return "执行 shell 命令"
}

func (t *RunCommandTool) Schema() string {
	return `{
		"name": "run_command",
		"description": "执行 shell 命令",
		"parameters": {
			"type": "object",
			"properties": {
				"cmd": {
					"type": "string",
					"description": "要执行的命令"
				}
			},
			"required": ["cmd"]
		}
	}`
}

func (t *RunCommandTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Cmd string `json:"cmd"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 验证命令安全性
	if err := t.security.ValidateCommand(params.Cmd); err != nil {
		return "", err
	}

	// 跨平台命令执行
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", params.Cmd)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", params.Cmd)
	}

	// 执行命令
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("命令执行失败: %v\n输出: %s", err, string(output)), nil
	}

	// 截断过长输出（最大 10KB）
	const maxOutput = 10000
	if len(output) > maxOutput {
		output = output[:maxOutput]
		output = append(output, []byte("\n... (输出已截断)")...)
	}

	return string(output), nil
}
