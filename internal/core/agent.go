package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

		// Execute tool
		a.UI.ShowStatus(fmt.Sprintf("Running %s...", call.Name))
		result, err := a.Tools.Execute(ctx, *call)

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

			// Execute tool
			a.UI.ShowStatus(fmt.Sprintf("Running %s...", toolCall.Name))
			result, err := a.Tools.Execute(ctx, *toolCall)

			// 【新增】发送工具执行结果状态
			a.notifyToolExecutionEnd(err)

			// Format output as JSON
			var output string
			if err != nil {
				errorResult := map[string]interface{}{
					"error": err.Error(),
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
	return fmt.Sprintf(`Project Context:
%s

Focused Files (%d files, ~%d tokens):
%s

Current working directory: %s`,
		ctx.FileTree,
		len(ctx.FocusedFiles),
		ctx.TotalTokens,
		formatFocusedFiles(ctx.FocusedFiles),
		a.ContextMgr.GetProjectRoot(),
	)
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
