package core

import (
	"sync"
)

// ConversationHistory manages the conversation history for an agent session
type ConversationHistory struct {
	messages []Message
	mu       sync.RWMutex
}

// NewConversationHistory creates a new conversation history
func NewConversationHistory() *ConversationHistory {
	return &ConversationHistory{
		messages: make([]Message, 0),
	}
}

// AddUserMessage adds a user message to the history
func (h *ConversationHistory) AddUserMessage(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, Message{
		Role:    "user",
		Content: content,
	})
}

// AddAssistantMessage adds an assistant message to the history
func (h *ConversationHistory) AddAssistantMessage(content string, toolCalls []ToolCall) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, Message{
		Role:       "assistant",
		Content:    content,
		ToolCalls:  toolCalls,
	})
}

// AddToolOutput adds a tool result message to the history
func (h *ConversationHistory) AddToolOutput(toolCallID, output string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, Message{
		Role:       "tool",
		Content:    output,
		ToolCallID: toolCallID,
	})
}

// AddSystemMessage adds a system message to the history
func (h *ConversationHistory) AddSystemMessage(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, Message{
		Role:    "system",
		Content: content,
	})
}

// GetMessages returns a copy of all messages
func (h *ConversationHistory) GetMessages() []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	messages := make([]Message, len(h.messages))
	copy(messages, h.messages)
	return messages
}

// BuildRequest constructs a ChatRequest from the current history
func (h *ConversationHistory) BuildRequest(maxTokens int, temperature float32) ChatRequest {
	h.mu.RLock()
	defer h.mu.RUnlock()

	messages := make([]Message, len(h.messages))
	copy(messages, h.messages)

	return ChatRequest{
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
}

// Clear removes all messages from the history
func (h *ConversationHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = make([]Message, 0)
}

// Count returns the number of messages in the history
func (h *ConversationHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.messages)
}
