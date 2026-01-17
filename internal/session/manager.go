package session

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yukin/kore/internal/core"
)

// Storage 定义会话存储接口
type Storage interface {
	// SaveSession 保存会话元数据
	SaveSession(ctx context.Context, session *Session) error

	// LoadSession 加载会话元数据
	LoadSession(ctx context.Context, sessionID string) (*Session, error)

	// ListSessions 列出所有会话
	ListSessions(ctx context.Context) ([]*Session, error)

	// DeleteSession 删除会话
	DeleteSession(ctx context.Context, sessionID string) error

	// SaveMessages 保存会话消息
	SaveMessages(ctx context.Context, sessionID string, messages []Message) error

	// LoadMessages 加载会话消息
	LoadMessages(ctx context.Context, sessionID string) ([]Message, error)

	// SearchSessions 搜索会话
	SearchSessions(ctx context.Context, query string) ([]*Session, error)
}

// Manager 会话管理器
type Manager struct {
	// 会话存储（sessionID -> Session）
	sessions map[string]*Session

	// 当前活跃会话
	currentSessionID string

	// 持久化存储
	storage Storage

	// Agent 工厂函数（用于创建新 Agent 实例）
	agentFactory func(*Session) (*core.Agent, error)

	// 互斥锁
	mu sync.RWMutex

	// 配置
	config *ManagerConfig
}

// ManagerConfig 会话管理器配置
type ManagerConfig struct {
	// 数据目录（用于持久化）
	DataDir string

	// 自动保存间隔
	AutoSaveInterval time.Duration

	// 最大会话数（0 = 无限制）
	MaxSessions int

	// 会话名称前缀
	SessionNamePrefix string
}

// NewManager 创建会话管理器
func NewManager(config *ManagerConfig, storage Storage, agentFactory func(*Session) (*core.Agent, error)) (*Manager, error) {
	if config == nil {
		config = &ManagerConfig{
			DataDir:           "./data",
			AutoSaveInterval:  30 * time.Second,
			MaxSessions:       0,
			SessionNamePrefix: "会话",
		}
	}

	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}

	if agentFactory == nil {
		return nil, fmt.Errorf("agentFactory cannot be nil")
	}

	mgr := &Manager{
		sessions:     make(map[string]*Session),
		storage:      storage,
		agentFactory: agentFactory,
		config:       config,
	}

	// 启动自动保存协程
	go mgr.autoSaveLoop()

	return mgr, nil
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(ctx context.Context, name string, agentMode AgentMode) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查会话数限制
	if m.config.MaxSessions > 0 && len(m.sessions) >= m.config.MaxSessions {
		return nil, fmt.Errorf("maximum session limit reached (%d)", m.config.MaxSessions)
	}

	// 生成会话 ID
	sessionID := uuid.New().String()

	// 创建临时会话对象（用于 Agent 工厂）
	tempSession := &Session{
		ID:        sessionID,
		Name:      name,
		AgentMode: agentMode,
	}

	// 使用工厂创建 Agent 实例
	agent, err := m.agentFactory(tempSession)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// 创建会话
	session := NewSession(sessionID, name, agentMode, agent)

	// 添加到内存
	m.sessions[sessionID] = session

	// 持久化到数据库
	if err := m.storage.SaveSession(ctx, session); err != nil {
		// 存储失败，从内存中移除
		delete(m.sessions, sessionID)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// GetSession 获取会话
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ListSessions 列出所有会话
func (m *Manager) ListSessions(ctx context.Context) ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 从内存中获取
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// CloseSession 关闭会话
func (m *Manager) CloseSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 关闭会话
	if err := session.Close(); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	// 保存消息历史
	if err := m.storage.SaveMessages(ctx, sessionID, session.GetMessages()); err != nil {
		return fmt.Errorf("failed to save messages: %w", err)
	}

	// 从内存中移除
	delete(m.sessions, sessionID)

	// 如果关闭的是当前会话，清空当前会话 ID
	if m.currentSessionID == sessionID {
		m.currentSessionID = ""
	}

	return nil
}

// SwitchSession 切换到指定会话
func (m *Manager) SwitchSession(sessionID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 检查会话是否活跃
	if !session.IsActive() {
		return nil, fmt.Errorf("session is not active: %s", sessionID)
	}

	// 切换当前会话
	m.currentSessionID = sessionID

	return session, nil
}

// GetCurrentSession 获取当前活跃会话
func (m *Manager) GetCurrentSession() (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentSessionID == "" {
		return nil, fmt.Errorf("no active session")
	}

	session, ok := m.sessions[m.currentSessionID]
	if !ok {
		return nil, fmt.Errorf("current session not found: %s", m.currentSessionID)
	}

	return session, nil
}

// RenameSession 重命名会话
func (m *Manager) RenameSession(ctx context.Context, sessionID string, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 更新名称
	session.mu.Lock()
	session.Name = newName
	session.UpdatedAt = time.Now().Unix()
	session.mu.Unlock()

	// 持久化
	if err := m.storage.SaveSession(ctx, session); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// AddMessage 添加消息到会话
func (m *Manager) AddMessage(sessionID string, msg Message) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 添加到会话
	session.AddMessage(msg)

	return nil
}

// GetMessages 获取会话消息
func (m *Manager) GetMessages(sessionID string) ([]Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session.GetMessages(), nil
}

// ExportSession 导出会话（JSON 格式）
func (m *Manager) ExportSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// 获取完整消息历史
	messages := session.GetMessages()

	// 构建导出数据
	export := map[string]interface{}{
		"id":         session.ID,
		"name":       session.Name,
		"agent_mode": session.AgentMode,
		"status":     session.Status,
		"created_at": session.CreatedAt,
		"updated_at": session.UpdatedAt,
		"metadata":   session.Metadata,
		"messages":   messages,
	}

	return export, nil
}

// ImportSession 导入会话（JSON 格式）
func (m *Manager) ImportSession(ctx context.Context, data map[string]interface{}) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 解析数据
	sessionID, ok := data["id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid session data: missing id")
	}

	name, ok := data["name"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid session data: missing name")
	}

	agentModeStr, ok := data["agent_mode"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid session data: missing agent_mode")
	}
	agentMode := AgentMode(agentModeStr)

	// 检查会话是否已存在
	if _, exists := m.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session already exists: %s", sessionID)
	}

	// 创建临时会话
	tempSession := &Session{
		ID:        sessionID,
		Name:      name,
		AgentMode: agentMode,
	}

	// 创建 Agent
	agent, err := m.agentFactory(tempSession)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// 创建会话
	session := NewSession(sessionID, name, agentMode, agent)

	// 恢复消息
	messagesData, ok := data["messages"].([]interface{})
	if ok {
		for _, msgData := range messagesData {
			msgMap, ok := msgData.(map[string]interface{})
			if !ok {
				continue
			}

			msg := Message{
				ID:        sessionID + "-" + fmt.Sprintf("%d", len(session.Messages)),
				SessionID: sessionID,
				Role:      getString(msgMap, "role"),
				Content:   getString(msgMap, "content"),
				Timestamp: getInt64(msgMap, "timestamp"),
			}
			session.Messages = append(session.Messages, msg)
		}
	}

	// 添加到内存
	m.sessions[sessionID] = session

	// 持久化
	if err := m.storage.SaveSession(ctx, session); err != nil {
		delete(m.sessions, sessionID)
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// SearchSessions 搜索会话
func (m *Manager) SearchSessions(ctx context.Context, query string) ([]*Session, error) {
	// 委托给存储层
	return m.storage.SearchSessions(ctx, query)
}

// autoSaveLoop 自动保存循环
func (m *Manager) autoSaveLoop() {
	ticker := time.NewTicker(m.config.AutoSaveInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		m.saveAllSessions(ctx)
	}
}

// saveAllSessions 保存所有会话
func (m *Manager) saveAllSessions(ctx context.Context) {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()

	// 并发保存
	for _, session := range sessions {
		if err := m.storage.SaveSession(ctx, session); err != nil {
			// 日志记录（TODO: 添加日志系统）
			continue
		}
		if err := m.storage.SaveMessages(ctx, session.ID, session.GetMessages()); err != nil {
			// 日志记录
			continue
		}
	}
}

// Shutdown 关闭管理器
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保存所有会话
	for _, session := range m.sessions {
		if err := session.Close(); err != nil {
			// 日志记录
			continue
		}
		if err := m.storage.SaveSession(ctx, session); err != nil {
			// 日志记录
			continue
		}
		if err := m.storage.SaveMessages(ctx, session.ID, session.GetMessages()); err != nil {
			// 日志记录
			continue
		}
	}

	return nil
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt64(m map[string]interface{}, key string) int64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case int:
			return int64(v)
		}
	}
	return 0
}
