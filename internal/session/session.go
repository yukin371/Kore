package session

import (
	"context"
	"sync"
	"time"

	"github.com/yukin/kore/internal/core"
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

// Session 表示一个会话
type Session struct {
	// 基础信息
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	AgentMode AgentMode              `json:"agent_mode"`
	Status    SessionStatus          `json:"status"`
	CreatedAt int64                  `json:"created_at"`
	UpdatedAt int64                  `json:"updated_at"`

	// Agent 实例（每个会话独立的 Agent）
	Agent *core.Agent `json:"-"`

	// 消息历史（内存缓存）
	Messages []Message `json:"-"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// 互斥锁（保护并发访问）
	mu sync.RWMutex

	// 取消上下文
	cancel context.CancelFunc
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
		Agent:     agent,
		Messages:  make([]Message, 0, 100),
		Metadata:  make(map[string]interface{}),
		cancel:    cancel,
	}
}

// AddMessage 添加消息到会话
func (s *Session) AddMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now().Unix()
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
