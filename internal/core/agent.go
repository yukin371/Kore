package core

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// UIInterface defines the abstract interface for user interaction
// The Agent doesn't care whether it's CLI, TUI, or GUI
type UIInterface interface {
	// SendStream sends streaming content to the user
	SendStream(content string)

	// RequestConfirm asks user for confirmation on an action
	// Returns true if user approved, false if rejected
	RequestConfirm(action string, args string) bool

	// RequestConfirmWithDiff asks user for confirmation with diff preview
	RequestConfirmWithDiff(path string, diffText string) bool

	// ShowStatus updates the status display
	ShowStatus(status string)
}

// ToolExecutor defines the interface for executing tools
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (string, error)
}

// Agent represents the core AI agent
type Agent struct {
	UI          UIInterface
	ContextMgr  *ContextManager
	LLMProvider LLMProvider
	Tools       ToolExecutor
	History     *ConversationHistory
	Config      *Config
	// 【新增】工具调用历史和文件缓存
	toolHistory *ToolCallHistory
	fileCache   *FileCache
}

// Config holds agent configuration
type Config struct {
	LLM struct {
		Model       string
		Temperature float32
		MaxTokens   int
	}
	ParallelTools bool // 是否启用并行工具执行（默认 false）
}

// NewAgent creates a new agent instance
func NewAgent(ui UIInterface, llm LLMProvider, tools ToolExecutor, projectRoot string) *Agent {
	return &Agent{
		UI:          ui,
		ContextMgr:  NewContextManager(projectRoot, 8000), // 8K token budget
		LLMProvider: llm,
		Tools:       tools,
		History:     NewConversationHistory(),
		Config:      &Config{},
		// 【新增】初始化工具调用历史和文件缓存
		toolHistory: NewToolCallHistory(),
		fileCache:   NewFileCache(),
	}
}

// Run executes the agent main loop with ReAct pattern
func (a *Agent) Run(ctx context.Context, userMessage string) error {
	// Build and inject system prompt with context (must be first)
	projectCtx, err := a.ContextMgr.BuildContext(ctx)
	if err != nil {
		a.UI.SendStream(fmt.Sprintf("Warning: Could not build project context: %v\n", err))
	} else {
		systemPrompt := a.buildSystemPrompt(projectCtx)
		a.History.AddSystemMessage(systemPrompt)
	}

	// Add user message after system prompt
	a.History.AddUserMessage(userMessage)

	// ReAct loop: keep going until no more tool calls
	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Call LLM
		req := a.History.BuildRequest(a.Config.LLM.MaxTokens, a.Config.LLM.Temperature)
		stream, err := a.LLMProvider.ChatStream(ctx, req)
		if err != nil {
			return fmt.Errorf("LLM stream error: %w", err)
		}

		// Process stream
		currentToolCalls := make([]*ToolCall, 0)
		var contentBuilder strings.Builder

		for event := range stream {
			switch event.Type {
			case EventContent:
				a.UI.SendStream(event.Content)
				contentBuilder.WriteString(event.Content)

			case EventToolCall:
				// Accumulate tool call data
				updateToolCalls(&currentToolCalls, event)

			case EventError:
				a.UI.SendStream(fmt.Sprintf("\n[Error: %s]", event.Content))

			case EventDone:
				// Stream finished, but not necessarily the task
			}
		}

		// Save assistant's complete response
		fullContent := contentBuilder.String()

		// 智谱 API 要求：当有工具调用时，content 不能为空或只有空白
		// 如果没有文本内容，使用占位符
		if len(currentToolCalls) > 0 && strings.TrimSpace(fullContent) == "" {
			fullContent = " " // 单个空格，避免空内容
		}

		a.History.AddAssistantMessage(fullContent, toolCallsToSlice(currentToolCalls))

		// Execute tools if any
		if len(currentToolCalls) > 0 {
			// 并行执行工具（如果配置支持）
			if a.Config.ParallelTools {
				a.executeToolsParallel(ctx, currentToolCalls)
			} else {
				a.executeToolsSequential(ctx, currentToolCalls)
			}

			// Continue to next LLM call with tool results
			continue
		}

		// No tool calls, task complete
		break
	}

	return nil
}

// executeToolsSequential 顺序执行工具
func (a *Agent) executeToolsSequential(ctx context.Context, toolCalls []*ToolCall) {
	for _, call := range toolCalls {
		// Request user confirmation
		if !a.UI.RequestConfirm(call.Name, call.Arguments) {
			// 工具结果必须是JSON格式
			errorResult := map[string]interface{}{
				"error": "User rejected the operation",
			}
			errorJSON, _ := json.Marshal(errorResult)
			a.History.AddToolOutput(call.ID, string(errorJSON))
			a.UI.SendStream(fmt.Sprintf("\n[Skipped %s]\n", call.Name))
			continue
		}

		// 【新增】发送工具执行开始状态
		a.notifyToolExecutionStart(call.Name, call.Arguments)

		// 【新增】检查智能文件缓存（仅对 read_file）
		if call.Name == "read_file" {
			var args map[string]interface{}
			json.Unmarshal([]byte(call.Arguments), &args)
			if path, ok := args["path"].(string); ok {
				content, cached, _ := a.fileCache.CheckRead(path)
				if cached {
					// 文件未修改，使用缓存
					result := map[string]interface{}{
						"content": content,
						"cached": true,
						"message": "文件内容未改变，使用缓存",
					}
					resultJSON, _ := json.Marshal(result)
					a.History.AddToolOutput(call.ID, string(resultJSON))

					// 记录工具调用（使用缓存）
					a.toolHistory.Record(ToolCallRecord{
						ID:        call.ID,
						Tool:      call.Name,
						Arguments: call.Arguments,
						Result:    "(使用缓存)",
						Success:   true,
						Timestamp: time.Now(),
					})

					a.notifyToolExecutionEnd(nil)
					continue
				}
			}
		}

		// Execute tool
		a.UI.ShowStatus(fmt.Sprintf("Running %s...", call.Name))
		result, err := a.Tools.Execute(ctx, *call)

		// 【新增】记录工具调用历史
		a.toolHistory.Record(ToolCallRecord{
			ID:        call.ID,
			Tool:      call.Name,
			Arguments: call.Arguments,
			Result:    result,
			Success:   err == nil,
			Timestamp: time.Now(),
		})

		// 【新增】如果是写入操作，更新缓存而非删除
		if call.Name == "write_file" && err == nil {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(call.Arguments), &args); err == nil {
				if path, ok := args["path"].(string); ok {
					if content, ok := args["content"].(string); ok {
						// 更新缓存，这样下次 read_file 可以直接使用
						a.fileCache.UpdateAfterWrite(path, content)
					}
				}
			}
		}

		// 【新增】发送工具执行结果状态
		a.notifyToolExecutionEnd(err)

		// Add result to history - 必须是JSON格式
		var output string
		if err != nil {
			errorResult := map[string]interface{}{
				"error": err.Error(),
			}
			errorJSON, _ := json.Marshal(errorResult)
			output = string(errorJSON)
		} else {
			// 如果结果已经是JSON格式，直接使用
			// 否则包装为JSON
			if strings.TrimSpace(result) != "" && (strings.HasPrefix(result, "{") || strings.HasPrefix(result, "[")) {
				output = result
			} else {
				successResult := map[string]interface{}{
					"result": result,
				}
				successJSON, _ := json.Marshal(successResult)
				output = string(successJSON)
			}
		}
		a.History.AddToolOutput(call.ID, output)
	}
}

// executeToolsParallel 并行执行工具
func (a *Agent) executeToolsParallel(ctx context.Context, toolCalls []*ToolCall) {
	type ToolResult struct {
		ID     string
		Output string
	}

	// 创建结果通道
	resultChan := make(chan ToolResult, len(toolCalls))

	// 启动 goroutine 并行执行工具
	var wg sync.WaitGroup
	for _, call := range toolCalls {
		wg.Add(1)
		go func(toolCall *ToolCall) {
			defer wg.Done()

			// Request user confirmation
			if !a.UI.RequestConfirm(toolCall.Name, toolCall.Arguments) {
				errorResult := map[string]interface{}{
					"error": "User rejected the operation",
				}
				errorJSON, _ := json.Marshal(errorResult)
				resultChan <- ToolResult{
					ID:     toolCall.ID,
					Output: string(errorJSON),
				}
				a.UI.SendStream(fmt.Sprintf("\n[Skipped %s]\n", toolCall.Name))
				return
			}

			// 【新增】发送工具执行开始状态
			a.notifyToolExecutionStart(toolCall.Name, toolCall.Arguments)

			// 【新增】检查智能文件缓存（仅对 read_file）
			var result string
			var execErr error
			if toolCall.Name == "read_file" {
				var args map[string]interface{}
				json.Unmarshal([]byte(toolCall.Arguments), &args)
				if path, ok := args["path"].(string); ok {
					content, cached, _ := a.fileCache.CheckRead(path)
					if cached {
						// 文件未修改，使用缓存
						cachedResult := map[string]interface{}{
							"content": content,
							"cached": true,
							"message": "文件内容未改变，使用缓存",
						}
						resultJSON, _ := json.Marshal(cachedResult)
						result = string(resultJSON)
						execErr = nil
					} else {
						// 执行实际工具
						result, execErr = a.Tools.Execute(ctx, *toolCall)
					}
				} else {
					result, execErr = a.Tools.Execute(ctx, *toolCall)
				}
			} else {
				// Execute tool
				a.UI.ShowStatus(fmt.Sprintf("Running %s...", toolCall.Name))
				result, execErr = a.Tools.Execute(ctx, *toolCall)
			}

			// 【新增】发送工具执行结果状态
			a.notifyToolExecutionEnd(execErr)

			// 【新增】记录工具调用历史
			a.toolHistory.Record(ToolCallRecord{
				ID:        toolCall.ID,
				Tool:      toolCall.Name,
				Arguments: toolCall.Arguments,
				Result:    result,
				Success:   execErr == nil,
				Timestamp: time.Now(),
			})

			// 【新增】如果是写入操作，更新缓存而非删除
			if toolCall.Name == "write_file" && execErr == nil {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err == nil {
					if path, ok := args["path"].(string); ok {
						if content, ok := args["content"].(string); ok {
							// 更新缓存，这样下次 read_file 可以直接使用
							a.fileCache.UpdateAfterWrite(path, content)
						}
					}
				}
			}

			// Format output as JSON
			var output string
			if execErr != nil {
				errorResult := map[string]interface{}{
					"error": execErr.Error(),
				}
				errorJSON, _ := json.Marshal(errorResult)
				output = string(errorJSON)
			} else {
				if strings.TrimSpace(result) != "" && (strings.HasPrefix(result, "{") || strings.HasPrefix(result, "[")) {
					output = result
				} else {
					successResult := map[string]interface{}{
						"result": result,
					}
					successJSON, _ := json.Marshal(successResult)
					output = string(successJSON)
				}
			}

			resultChan <- ToolResult{
				ID:     toolCall.ID,
				Output: output,
			}
		}(call)
	}

	// 等待所有工具完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果并添加到历史
	for result := range resultChan {
		a.History.AddToolOutput(result.ID, result.Output)
	}
}

// buildSystemPrompt constructs the system prompt with project context
func (a *Agent) buildSystemPrompt(ctx *ProjectContext) string {
	var parts []string

	// 1. 基础系统提示词（从 system.txt 加载）
	systemPrompt := loadSystemPrompt()
	parts = append(parts, systemPrompt)

	// 2. 工具使用指南（从 tools.txt 加载）
	toolGuide := loadToolGuide()
	parts = append(parts, toolGuide)

	// 3. 【新增】工具调用历史摘要
	toolHistorySummary := a.toolHistory.GetSummary()
	parts = append(parts, toolHistorySummary)

	// 4. 项目上下文
	projectContext := fmt.Sprintf(`

## 项目上下文 (Project Context)

项目根目录: %s

项目目录树:
%s

关注的文件 (%d 个文件, ~%d tokens):
%s

当前工作目录: %s`,
		a.ContextMgr.GetProjectRoot(),
		ctx.FileTree,
		len(ctx.FocusedFiles),
		ctx.TotalTokens,
		formatFocusedFiles(ctx.FocusedFiles),
		a.ContextMgr.GetProjectRoot(),
	)
	parts = append(parts, projectContext)

	// 5. 当前日期时间
	parts = append(parts, fmt.Sprintf("\n当前时间: %s", time.Now().Format("2006-01-02 15:04")))

	return strings.Join(parts, "\n\n")
}

// loadSystemPrompt 加载系统提示词文件
func loadSystemPrompt() string {
	// 尝试从嵌入的文件读取
	// 如果使用 embed，需要在文件顶部添加 //go:embed 指令
	// 这里我们先尝试从文件系统读取
	content, err := os.ReadFile("api/prompts/system.txt")
	if err != nil {
		// 如果读取失败，返回默认提示词
		return `你是 Kore，一个用 Go 构建的专业编程助手和自动化工作流代理。

你的任务是帮助开发者：
- 理解代码库并解释代码逻辑
- 精确地修改代码
- 运行命令和测试
- 自动化重复的开发任务

你可以使用工具来读取文件、写入文件和执行命令。`
	}

	return string(content)
}

// loadToolGuide 加载工具使用指南
func loadToolGuide() string {
	content, err := os.ReadFile("api/prompts/tools.txt")
	if err != nil {
		// 如果读取失败，返回基本指南
		return `## 工具使用指南

使用工具来完成实际任务。你可以并行调用多个工具以提高效率。

重要：
- 不要重复读取已经读取过的文件
- 工具调用结果会保存到对话历史中
- 一次性说明所有修改，而不是多次小修改`
	}

	return string(content)
}

// formatFocusedFiles formats focused files for display
func formatFocusedFiles(files []File) string {
	if len(files) == 0 {
		return "No files focused"
	}

	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	return strings.Join(paths, ", ")
}

// updateToolCalls updates the tool calls accumulator during streaming
func updateToolCalls(calls *[]*ToolCall, event StreamEvent) {
	if event.ToolCall == nil {
		return
	}

	// Find existing tool call with this ID
	var existing *ToolCall
	for _, call := range *calls {
		if call.ID == event.ToolCall.ID {
			existing = call
			break
		}
	}

	if existing != nil {
		// Append arguments to existing call
		existing.Arguments += event.ToolCall.Arguments
	} else {
		// Create new tool call
		*calls = append(*calls, &ToolCall{
			ID:        event.ToolCall.ID,
			Name:      event.ToolCall.Name,
			Arguments: event.ToolCall.Arguments,
		})
	}
}

// toolCallsToSlice converts []*ToolCall to []ToolCall
func toolCallsToSlice(calls []*ToolCall) []ToolCall {
	result := make([]ToolCall, len(calls))
	for i, call := range calls {
		result[i] = *call
	}
	return result
}

// ========== 工具执行状态通知 ==========

// notifyToolExecutionStart 通知工具执行开始
func (a *Agent) notifyToolExecutionStart(toolName string, arguments string) {
	// 使用类型断言检查 UI 是否支持扩展接口
	if tuiUI, ok := a.UI.(interface {
		StartToolExecution(toolName string, payload map[string]string)
	}); ok {
		// 提取 payload（如果有）
		payload := a.extractToolPayload(toolName, arguments)
		tuiUI.StartToolExecution(toolName, payload)
	}
}

// notifyToolExecutionEnd 通知工具执行结束
func (a *Agent) notifyToolExecutionEnd(err error) {
	// 使用类型断言检查 UI 是否支持扩展接口
	if tuiUI, ok := a.UI.(interface {
		EndToolExecution(success bool, errMsg string)
	}); ok {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		tuiUI.EndToolExecution(err == nil, errMsg)
	}
}

// extractToolPayload 从工具参数中提取元数据
func (a *Agent) extractToolPayload(toolName string, arguments string) map[string]string {
	payload := make(map[string]string)

	// 根据工具类型提取相关信息
	switch toolName {
	case "read_file", "list_files":
		// 从 arguments 中提取文件路径
		if file := a.extractJSONField(arguments, "path"); file != "" {
			payload["file"] = file
		}
	case "search_files":
		// 提取搜索模式
		if pattern := a.extractJSONField(arguments, "pattern"); pattern != "" {
			payload["pattern"] = pattern
		}
	case "run_command":
		// 提取命令
		if cmd := a.extractJSONField(arguments, "cmd"); cmd != "" {
			payload["command"] = cmd
		}
	}

	return payload
}

// extractJSONField 从 JSON 字符串中提取字段值
func (a *Agent) extractJSONField(jsonStr, field string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ""
	}

	if val, ok := data[field]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	return ""
}
