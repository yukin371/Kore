package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yukin371/Kore/internal/core"
)

// SessionStatus 表示会话状态
type SessionStatus int

const (
	SessionActive   SessionStatus = iota // 活跃
	SessionIdle                          // 空闲
	SessionClosed                        // 已关闭
)

// AgentMode 表示 Agent 类型
type AgentMode string

const (
	ModeBuild   AgentMode = "build"   // 构建模式（完全访问）
	ModePlan    AgentMode = "plan"    // 规划模式（只读）
	ModeGeneral AgentMode = "general" // 通用模式（复杂任务）
)

// Message 表示会话中的一条消息
type Message struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Role      string                 `json:"role"` // "user", "assistant", "system"
	Content   string                 `json:"content"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SessionStats 会话统计信息
type SessionStats struct {
	MessageCount   int   `json:"message_count"`   // 消息总数
	UserMsgCount   int   `json:"user_msg_count"`   // 用户消息数
	AssistantMsgCount int `json:"assistant_msg_count"` // 助手消息数
	ToolCallCount  int   `json:"tool_call_count"`  // 工具调用次数
	TokenUsed      int64 `json:"token_used"`       // 使用的 token 数（估算）
	LastActiveAt   int64 `json:"last_active_at"`   // 最后活跃时间
}

// Session 表示一个会话
type Session struct {
	// 基础信息
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	AgentMode AgentMode              `json:"agent_mode"`
	Status    SessionStatus          `json:"status"`
	CreatedAt int64                  `json:"created_at"`
	UpdatedAt int64                  `json:"updated_at"`

	// 扩展元数据
	Description string                 `json:"description,omitempty"` // 会话描述
	Tags        []string               `json:"tags,omitempty"`        // 会话标签
	Statistics  SessionStats           `json:"statistics"`            // 会话统计

	// Agent 实例（每个会话独立的 Agent）
	Agent *core.Agent `json:"-"`

	// 消息历史（内存缓存）
	Messages []Message `json:"-"`

	// 工具执行记录（独立于 Agent）
	ToolExecutions []ToolExecution `json:"-"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 互斥锁（保护并发访问）
	mu sync.RWMutex

	// 取消上下文
	cancel context.CancelFunc
}

// ToolExecution 工具执行记录
type ToolExecution struct {
	ID        string                 `json:"id"`
	Tool      string                 `json:"tool"`
	Arguments string                 `json:"arguments"`
	Result    string                 `json:"result"`
	Success   bool                   `json:"success"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewSession 创建新会话
func NewSession(id string, name string, agentMode AgentMode, agent *core.Agent) *Session {
	now := time.Now().Unix()
	_, cancel := context.WithCancel(context.Background())

	return &Session{
		ID:        id,
		Name:      name,
		AgentMode: agentMode,
		Status:    SessionActive,
		CreatedAt: now,
		UpdatedAt: now,
		Description: "",
		Tags:     make([]string, 0),
		Statistics: SessionStats{
			LastActiveAt: now,
		},
		Agent:          agent,
		Messages:       make([]Message, 0, 100),
		ToolExecutions: make([]ToolExecution, 0, 100),
		Metadata:       make(map[string]interface{}),
		cancel:         cancel,
	}
}

// AddMessage 添加消息到会话
func (s *Session) AddMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 自动生成消息ID（如果未设置）
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now().Unix()

	// 更新统计信息
	s.Statistics.MessageCount++
	s.Statistics.LastActiveAt = s.UpdatedAt

	switch msg.Role {
	case "user":
		s.Statistics.UserMsgCount++
	case "assistant":
		s.Statistics.AssistantMsgCount++
	}

	// 估算 token 使用（简单估算：中文字符数 + 英文单词数 * 1.3）
	s.Statistics.TokenUsed += estimateTokens(msg.Content)
}

// GetMessages 获取会话的所有消息
func (s *Session) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本
	messages := make([]Message, len(s.Messages))
	copy(messages, s.Messages)
	return messages
}

// GetLastNMessages 获取最近的 N 条消息
func (s *Session) GetLastNMessages(n int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.Messages)
	if total == 0 {
		return []Message{}
	}

	start := total - n
	if start < 0 {
		start = 0
	}

	// 返回副本
	messages := make([]Message, total-start)
	copy(messages, s.Messages[start:])
	return messages
}

// UpdateStatus 更新会话状态
func (s *Session) UpdateStatus(status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = status
	s.UpdatedAt = time.Now().Unix()
}

// SetMetadata 设置元数据
func (s *Session) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Metadata == nil {
		s.Metadata = make(map[string]interface{})
	}
	s.Metadata[key] = value
	s.UpdatedAt = time.Now().Unix()
}

// GetMetadata 获取元数据
func (s *Session) GetMetadata(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Metadata == nil {
		return nil, false
	}
	val, ok := s.Metadata[key]
	return val, ok
}

// Close 关闭会话
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 取消 Agent 的上下文
	if s.cancel != nil {
		s.cancel()
	}

	// 更新状态
	s.Status = SessionClosed
	s.UpdatedAt = time.Now().Unix()

	return nil
}

// IsActive 检查会话是否活跃
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Status == SessionActive
}

// GetContext 获取会话的上下文（用于 Agent 执行）
func (s *Session) GetContext() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cancel != nil {
		return context.Background()
	}
	return context.Background()
}

// GetDataForStorage 获取会话数据用于存储（线程安全）
func (s *Session) GetDataForStorage() (id, name string, agentMode AgentMode, status SessionStatus, createdAt, updatedAt int64, metadata map[string]interface{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ID, s.Name, s.AgentMode, s.Status, s.CreatedAt, s.UpdatedAt, s.Metadata
}

// GetAgent 获取会话的 Agent 实例
func (s *Session) GetAgent() *core.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Agent
}

// SetDescription 设置会话描述
func (s *Session) SetDescription(description string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Description = description
	s.UpdatedAt = time.Now().Unix()
}

// GetDescription 获取会话描述
func (s *Session) GetDescription() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Description
}

// AddTag 添加标签
func (s *Session) AddTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.Tags {
		if t == tag {
			return // 标签已存在
		}
	}

	s.Tags = append(s.Tags, tag)
	s.UpdatedAt = time.Now().Unix()
}

// RemoveTag 移除标签
func (s *Session) RemoveTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.Tags {
		if t == tag {
			s.Tags = append(s.Tags[:i], s.Tags[i+1:]...)
			s.UpdatedAt = time.Now().Unix()
			return
		}
	}
}

// GetTags 获取所有标签
func (s *Session) GetTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make([]string, len(s.Tags))
	copy(tags, s.Tags)
	return tags
}

// GetStatistics 获取统计信息
func (s *Session) GetStatistics() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.Statistics
}

// RecordToolExecution 记录工具执行
func (s *Session) RecordToolExecution(execution ToolExecution) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ToolExecutions = append(s.ToolExecutions, execution)
	s.Statistics.ToolCallCount++
	s.Statistics.LastActiveAt = time.Now().Unix()
	s.UpdatedAt = s.Statistics.LastActiveAt
}

// GetToolExecutions 获取工具执行记录
func (s *Session) GetToolExecutions() []ToolExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	executions := make([]ToolExecution, len(s.ToolExecutions))
	copy(executions, s.ToolExecutions)
	return executions
}

// SetStatus 设置会话状态
func (s *Session) SetStatus(status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = status
	s.UpdatedAt = time.Now().Unix()
}

// estimateTokens 估算文本的 token 数量
func estimateTokens(text string) int64 {
	if len(text) == 0 {
		return 0
	}

	// 简单估算：中文字符 = 1 token，英文单词 = 1.3 tokens
	// 这是一个粗略估计，实际 token 数量取决于分词器
	total := 0
	chineseChars := 0
	englishWords := 0

	inWord := false
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			// 中文字符
			chineseChars++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '\'' {
			// 英文字符
			if !inWord {
				englishWords++
				inWord = true
			}
		} else {
			// 其他字符
			inWord = false
		}
	}

	// 计算 token 数
	total = chineseChars + int(float64(englishWords)*1.3)

	return int64(total)
}

// generateMessageID 生成唯一的消息ID
func generateMessageID() string {
	return uuid.New().String()
}
