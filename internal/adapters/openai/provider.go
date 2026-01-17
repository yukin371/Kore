// Package openai 提供 OpenAI API 的 LLM Provider 实现
// 也兼容智谱 API、DeepSeek 等兼容 OpenAI 格式的 API
package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yukin/kore/internal/core"
)

// Provider 实现 OpenAI 的 LLMProvider 接口
type Provider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewProvider 创建一个新的 OpenAI Provider
func NewProvider(apiKey, model string) *Provider {
	baseURL := "https://api.openai.com/v1"
	if apiKey == "" {
		baseURL = "http://localhost:11434/v1" // Ollama 兼容端点
	}

	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// SetBaseURL 设置自定义 BaseURL（用于代理或本地部署）
func (p *Provider) SetBaseURL(url string) {
	p.baseURL = url
}

// SetModel 设置使用的模型
func (p *Provider) SetModel(model string) {
	p.model = model
}

// GetModel 返回当前模型名称
func (p *Provider) GetModel() string {
	return p.model
}

// ChatStream 发起流式聊天请求
func (p *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	// 构建请求体
	requestBody, err := p.buildChatRequest(req)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	// 创建 HTTP 请求
	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	// 发送请求
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	// 创建事件通道
	eventChan := make(chan core.StreamEvent, 16)

	// 启动 goroutine 处理流式响应
	go p.processStream(ctx, resp.Body, eventChan)

	return eventChan, nil
}

// buildChatRequest 构建 OpenAI API 请求体
func (p *Provider) buildChatRequest(req core.ChatRequest) (string, error) {
	// 定义消息中的工具调用格式（兼容智谱 API）
	type MessageToolCall struct {
		ID     string `json:"id"`
		Type   string `json:"type"` // 智谱 API 需要 type 字段
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}

	type Message struct {
		Role       string            `json:"role"`
		Content    string            `json:"content"`
		ToolCalls  []MessageToolCall `json:"tool_calls,omitempty"`
		ToolCallID string            `json:"tool_call_id,omitempty"`
	}

	// 定义工具参数类型
	type ToolParameter struct {
		Type        string `json:"type"`
		Description string `json:"description,omitempty"`
		Enum        []string `json:"enum,omitempty"`
	}

	type ToolParameters struct {
		Type       string                    `json:"type"`
		Properties map[string]ToolParameter `json:"properties"`
		Required   []string                  `json:"required"`
	}

	type ToolFunction struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  ToolParameters `json:"parameters"`
	}

	type Tool struct {
		Type     string       `json:"type"`
		Function ToolFunction `json:"function"`
	}

	// 构建工具列表
	tools := []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_file",
				Description: "读取文件内容，支持指定行范围",
				Parameters: ToolParameters{
					Type: "object",
					Properties: map[string]ToolParameter{
						"path": {
							Type:        "string",
							Description: "文件路径（相对于项目根目录）",
						},
						"line_start": {
							Type:        "integer",
							Description: "起始行号（可选，0-indexed）",
						},
						"line_end": {
							Type:        "integer",
							Description: "结束行号（可选，0-indexed）",
						},
					},
					Required: []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "write_file",
				Description: "写入文件内容（完全覆盖现有内容）",
				Parameters: ToolParameters{
					Type: "object",
					Properties: map[string]ToolParameter{
						"path": {
							Type:        "string",
							Description: "文件路径（相对于项目根目录）",
						},
						"content": {
							Type:        "string",
							Description: "要写入的完整内容",
						},
					},
					Required: []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "run_command",
				Description: "执行 shell 命令",
				Parameters: ToolParameters{
					Type: "object",
					Properties: map[string]ToolParameter{
						"cmd": {
							Type:        "string",
							Description: "要执行的命令",
						},
					},
					Required: []string{"cmd"},
				},
			},
		},
	}

	type ChatCompletionRequest struct {
		Model       string   `json:"model"`
		Messages    []Message `json:"messages"`
		MaxTokens   int      `json:"max_tokens,omitempty"`
		Temperature float32  `json:"temperature,omitempty"`
		Stream      bool     `json:"stream"`
		Tools       []Tool   `json:"tools,omitempty"`
	}

	messages := make([]Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		// 转换工具调用格式（从 core.ToolCall 到智谱格式）
		if len(msg.ToolCalls) > 0 {
			messages[i].ToolCalls = make([]MessageToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				messages[i].ToolCalls[j] = MessageToolCall{
					ID:   tc.ID,
					Type: "function", // 智谱 API 需要 type 字段
				}
				messages[i].ToolCalls[j].Function.Name = tc.Name
				messages[i].ToolCalls[j].Function.Arguments = tc.Arguments
			}
		}
	}

	chatReq := ChatCompletionRequest{
		Model:       p.model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
		Tools:       tools, // 始终包含 tools 定义
	}

	data, err := json.Marshal(chatReq)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// processStream 处理 SSE 流式响应
func (p *Provider) processStream(ctx context.Context, body io.ReadCloser, eventChan chan<- core.StreamEvent) {
	defer close(eventChan)
	defer body.Close()

	reader := bufio.NewReader(body)

	for {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			eventChan <- core.StreamEvent{Type: core.EventError, Content: "请求已取消"}
			return
		default:
		}

		// 读取一行
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				eventChan <- core.StreamEvent{Type: core.EventError, Content: fmt.Sprintf("读取错误: %v", err)}
			}
			break
		}

		// SSE 格式以 "data: " 开头
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// 提取 JSON 数据
		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		// [DONE] 表示流结束
		if data == "[DONE]" {
			eventChan <- core.StreamEvent{Type: core.EventDone}
			return
		}

		// 解析 JSON
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int `json:"index"`
						ID       string `json:"id,omitempty"`
						Name     string `json:"name,omitempty"`
						Arguments string `json:"arguments,omitempty"`
						// 智谱 API 使用 function 嵌套
						Function *struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 跳过无法解析的行
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// 处理文本内容
		if choice.Delta.Content != "" {
			eventChan <- core.StreamEvent{
				Type:    core.EventContent,
				Content: choice.Delta.Content,
			}
		}

		// 处理工具调用（兼容 OpenAI 和智谱格式）
		for _, tc := range choice.Delta.ToolCalls {
			name := tc.Name
			arguments := tc.Arguments

			// 智谱 API 使用 function 嵌套格式
			if tc.Function != nil {
				name = tc.Function.Name
				arguments = tc.Function.Arguments
			}

			eventChan <- core.StreamEvent{
				Type: core.EventToolCall,
				ToolCall: &core.ToolCallDelta{
					ID:        tc.ID,
					Name:      name,
					Arguments: arguments,
				},
			}
		}

		// 检查是否完成
		if choice.FinishReason != nil {
			eventChan <- core.StreamEvent{Type: core.EventDone}
			return
		}
	}
}
