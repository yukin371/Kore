package core

import "context"

// EventType represents the type of stream event
type EventType int

const (
	EventContent  EventType = iota // Ordinary text content
	EventToolCall                   // Tool call invocation
	EventError                      // Error occurred
	EventDone                       // Stream completed
)

// StreamEvent represents a single event in the LLM response stream
type StreamEvent struct {
	Type     EventType
	Content  string         // Text content for EventContent and EventError
	ToolCall *ToolCallDelta // Tool call data for EventToolCall
}

// ToolCallDelta represents incremental tool call data during streaming
type ToolCallDelta struct {
	ID        string // Tool call identifier
	Name      string // Tool/function name
	Arguments string // JSON fragment, accumulated during streaming
}

// ToolCall represents a complete tool call after streaming assembly
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // Complete JSON arguments string
}

// ChatRequest represents a request to the LLM
type ChatRequest struct {
	Messages    []Message
	MaxTokens   int
	Temperature float32
}

// Message represents a single message in the conversation
type Message struct {
	Role    string // "system", "user", "assistant", "tool"
	Content string
	ToolCalls []ToolCall // For assistant messages
	ToolCallID string // For tool messages
}

// LLMProvider defines the interface for LLM providers (OpenAI, Ollama, etc.)
type LLMProvider interface {
	// ChatStream initiates a streaming chat request and returns a channel of StreamEvent
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)

	// SetModel changes the active model
	SetModel(model string)

	// GetModel returns the current model name
	GetModel() string
}
