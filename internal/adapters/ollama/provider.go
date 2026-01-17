// Package ollama 提供 Ollama 本地模型的 LLM Provider 实现
package ollama

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yukin/kore/internal/core"
)

// 支持原生工具调用的 Ollama 模型列表
var toolCapableModels = []string{
	"llama3.1",
	"llama3.2",
	"llama3.3",
	"llama4",
	"qwen2.5",
	"qwen2.5-coder",
	"deepseek-coder",
	"mistral-nemo",
}

// Provider 实现 Ollama 的 LLMProvider 接口
type Provider struct {
	baseURL            string
	model              string
	supportsTools      bool // 是否支持原生工具调用
	client             *http.Client
	useXMLFormat       bool // 是否使用 XML 格式作为回退
}

// NewProvider 创建一个新的 Ollama Provider
func NewProvider(baseURL, model string) *Provider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// 检测模型是否支持原生工具调用
	supportsTools := checkToolSupport(model)

	return &Provider{
		baseURL:       baseURL,
		model:         model,
		supportsTools: supportsTools,
		useXMLFormat:  !supportsTools, // 如果不支持原生工具，使用 XML 格式
		client:        &http.Client{Timeout: 120 * time.Second},
	}
}

// checkToolSupport 检查模型是否支持原生工具调用
func checkToolSupport(model string) bool {
	model = strings.ToLower(model)
	for _, capable := range toolCapableModels {
		if strings.Contains(model, capable) {
			return true
		}
	}
	return false
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
	// 如果使用 XML 格式，需要在 system prompt 中注入工具说明
	if p.useXMLFormat {
		req = p.injectToolFormat(req)
	}

	// 构建请求体
	requestBody, err := p.buildChatRequest(req)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	// 创建 HTTP 请求
	url := p.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")

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
	go p.processStream(ctx, resp.Body, eventChan, p.useXMLFormat)

	return eventChan, nil
}

// injectToolFormat 在不支持原生工具的模型中注入 XML 工具格式
func (p *Provider) injectToolFormat(req core.ChatRequest) core.ChatRequest {
	toolFormat := `

可用的工具:
<tool name="read_file">读取文件内容</tool>
<tool name="write_file">写入文件内容</tool>
<tool name="run_command">执行 shell 命令</tool>

使用 XML 标签调用工具，例如:
<tool name="read_file">main.go</tool>

当需要执行工具时，使用上述 XML 格式。`

	// 在第一条 system message 后追加工具格式
	for i, msg := range req.Messages {
		if msg.Role == "system" {
			req.Messages[i].Content = msg.Content + toolFormat
			break
		}
	}

	return req
}

// buildChatRequest 构建 Ollama API 请求体
func (p *Provider) buildChatRequest(req core.ChatRequest) (string, error) {
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type ChatRequest struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Stream      bool      `json:"stream"`
		Options     struct {
			Temperature float32 `json:"temperature,omitempty"`
			NumPredict  int     `json:"num_predict,omitempty"`
		} `json:"options,omitempty"`
	}

	messages := make([]Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	chatReq := ChatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
	}
	chatReq.Options.Temperature = req.Temperature
	chatReq.Options.NumPredict = req.MaxTokens

	data, err := json.Marshal(chatReq)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// processStream 处理 Ollama 的流式响应（JSONL 格式）
func (p *Provider) processStream(ctx context.Context, body io.ReadCloser, eventChan chan<- core.StreamEvent, useXMLFormat bool) {
	defer close(eventChan)
	defer body.Close()

	reader := bufio.NewReader(body)
	var contentBuffer strings.Builder // 用于累积内容，解析 XML 工具调用

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

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Ollama 返回 JSONL 格式
		var chunk struct {
			Message     struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
			Error string `json:"error,omitempty"`
		}

		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue // 跳过无法解析的行
		}

		// 检查错误
		if chunk.Error != "" {
			eventChan <- core.StreamEvent{Type: core.EventError, Content: chunk.Error}
			return
		}

		// 处理文本内容
		if chunk.Message.Content != "" {
			// 如果使用 XML 格式，累积内容以解析工具调用
			if useXMLFormat {
				contentBuffer.WriteString(chunk.Message.Content)

				// 尝试解析完整的工具调用
				if tools := p.parseXMLToolCalls(contentBuffer.String()); len(tools) > 0 {
					for _, tool := range tools {
						eventChan <- core.StreamEvent{
							Type: core.EventToolCall,
							ToolCall: &core.ToolCallDelta{
								ID:        fmt.Sprintf("call_%d", time.Now().UnixNano()),
								Name:      tool.Name,
								Arguments: tool.ArgsJSON,
							},
						}
					}
					// 清空缓冲区
					contentBuffer.Reset()
				}
			} else {
				// 直接发送内容
				eventChan <- core.StreamEvent{
					Type:    core.EventContent,
					Content: chunk.Message.Content,
				}
			}
		}

		// 检查是否完成
		if chunk.Done {
			// 如果使用 XML 格式，发送剩余内容
			if useXMLFormat && contentBuffer.Len() > 0 {
				remaining := contentBuffer.String()
				// 移除工具调用标签，只保留文本
				cleaned := p.removeToolTags(remaining)
				if cleaned != "" {
					eventChan <- core.StreamEvent{
						Type:    core.EventContent,
						Content: cleaned,
					}
				}
			}
			eventChan <- core.StreamEvent{Type: core.EventDone}
			return
		}
	}
}

// XMLToolCall 表示 XML 格式的工具调用
type XMLToolCall struct {
	Name    string
	ArgsJSON string
}

// parseXMLToolCalls 解析 XML 格式的工具调用
func (p *Provider) parseXMLToolCalls(content string) []XMLToolCall {
	// 匹配 <tool name="tool_name">args</tool> 格式
	re := regexp.MustCompile(`<tool\s+name=["']([^"']+)["']>\s*(.+?)\s*</tool>`)
	matches := re.FindAllStringSubmatch(content, -1)

	var tools []XMLToolCall
	for _, match := range matches {
		if len(match) >= 3 {
			// 构建参数 JSON
			argsJSON := fmt.Sprintf(`{"input": %q}`, strings.TrimSpace(match[2]))
			tools = append(tools, XMLToolCall{
				Name:    match[1],
				ArgsJSON: argsJSON,
			})
		}
	}

	return tools
}

// removeToolTags 移除工具调用标签，保留文本内容
func (p *Provider) removeToolTags(content string) string {
	// 移除 <tool name="...">...</tool> 标签
	re := regexp.MustCompile(`<tool\s+name=["'][^"']+["']>\s*.+?\s*</tool>`)
	cleaned := re.ReplaceAllString(content, "")

	// 清理多余空行
	lines := strings.Split(cleaned, "\n")
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
