package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	rpc "github.com/yukin371/Kore/api/proto"
	"github.com/yukin371/Kore/internal/core"
	"github.com/yukin371/Kore/internal/eventbus"
	"github.com/yukin371/Kore/internal/session"
)

// Mock implementations for testing

// MockSessionManager 模拟会话管理器（用于测试）
type MockSessionManager struct {
	sessions map[string]*rpc.Session
	mu       sync.RWMutex
}

// NewMockSessionManager 创建模拟会话管理器
func NewMockSessionManager() *MockSessionManager {
	return &MockSessionManager{
		sessions: make(map[string]*rpc.Session),
	}
}

func (m *MockSessionManager) CreateSession(ctx context.Context, name, agentType string, config map[string]string) (*rpc.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &rpc.Session{
		Id:           fmt.Sprintf("session-%d", time.Now().UnixNano()),
		Name:         name,
		AgentType:    agentType,
		Status:       "running",
		CreatedAt:    time.Now().Unix(),
		LastActiveAt: time.Now().Unix(),
		Metadata:     config,
	}
	m.sessions[session.Id] = session
	return session, nil
}

func (m *MockSessionManager) GetSession(ctx context.Context, sessionID string) (*rpc.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

func (m *MockSessionManager) ListSessions(ctx context.Context, limit, offset int32) ([]*rpc.Session, int32, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*rpc.Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions, int32(len(sessions)), nil
}

func (m *MockSessionManager) CloseSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[sessionID]; !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(m.sessions, sessionID)
	return nil
}

// MockEventBus 模拟事件总线（用于测试）
type MockEventBus struct {
	events chan *rpc.Event
	mu     sync.RWMutex
}

// NewMockEventBus 创建模拟事件总线
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events: make(chan *rpc.Event, 100),
	}
}

func (m *MockEventBus) Subscribe(ctx context.Context, sessionID string, eventTypes []string) (<-chan *rpc.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	eventChan := make(chan *rpc.Event, 10)

	// 发送一些模拟事件
	go func() {
		eventChan <- &rpc.Event{
			Type:      "test.event",
			SessionId: sessionID,
			Timestamp: time.Now().Unix(),
			Data:      []byte("test"),
		}
	}()

	return eventChan, nil
}

func (m *MockEventBus) Publish(event *rpc.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case m.events <- event:
		return nil
	default:
		return fmt.Errorf("event queue is full")
	}
}

// SessionManagerAdapter 会话管理器适配器
type SessionManagerAdapter struct {
	manager *session.Manager
	eventBus *eventbus.EventBus
}

// NewSessionManagerAdapter 创建会话管理器适配器
func NewSessionManagerAdapter(manager *session.Manager, eventBus *eventbus.EventBus) *SessionManagerAdapter {
	return &SessionManagerAdapter{
		manager:   manager,
		eventBus:  eventBus,
	}
}

// CreateSession 创建会话（实现 gRPC 接口）
func (a *SessionManagerAdapter) CreateSession(ctx context.Context, name string, agentType string, config map[string]string) (*rpc.Session, error) {
	// 转换 agent 类型
	mode := session.AgentMode(agentType)

	// 创建会话
	sess, err := a.manager.CreateSession(ctx, name, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// 发布事件
	if a.eventBus != nil {
		a.eventBus.PublishSessionCreated(sess.ID, sess.Name)
	}

	// 转换为 gRPC 格式
	return a.toRPCSession(sess), nil
}

// GetSession 获取会话
func (a *SessionManagerAdapter) GetSession(ctx context.Context, sessionID string) (*rpc.Session, error) {
	sess, err := a.manager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return a.toRPCSession(sess), nil
}

// ListSessions 列出会话
func (a *SessionManagerAdapter) ListSessions(ctx context.Context, limit, offset int32) ([]*rpc.Session, int32, error) {
	sessions, err := a.manager.ListSessions(ctx)
	if err != nil {
		return nil, 0, err
	}

	total := int32(len(sessions))

	// 应用分页
	start := offset
	end := offset + limit
	if start >= int32(len(sessions)) {
		return []*rpc.Session{}, total, nil
	}
	if end > int32(len(sessions)) {
		end = int32(len(sessions))
	}

	pagedSessions := sessions[start:end]

	// 转换为 gRPC 格式
	result := make([]*rpc.Session, len(pagedSessions))
	for i, sess := range pagedSessions {
		result[i] = a.toRPCSession(sess)
	}

	return result, total, nil
}

// CloseSession 关闭会话
func (a *SessionManagerAdapter) CloseSession(ctx context.Context, sessionID string) error {
	sess, err := a.manager.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 关闭会话
	if err := a.manager.CloseSession(ctx, sessionID); err != nil {
		return err
	}

	// 发布事件
	if a.eventBus != nil {
		a.eventBus.PublishSessionClosed(sessionID)
		_ = sess // 使用 sess 避免未使用变量警告
	}

	return nil
}

// GetSessionInternal 获取内部会话对象（用于其他 RPC）
func (a *SessionManagerAdapter) GetSessionInternal(sessionID string) (*session.Session, error) {
	return a.manager.GetSession(sessionID)
}

// toRPCSession 转换为 gRPC Session 格式
func (a *SessionManagerAdapter) toRPCSession(sess *session.Session) *rpc.Session {
	// 使用 getter 方法来获取数据，避免直接访问未导出的字段
	id, name, agentMode, status, createdAt, updatedAt, metadata := sess.GetDataForStorage()

	// 转换 metadata
	metadataStr := make(map[string]string)
	for k, v := range metadata {
		if str, ok := v.(string); ok {
			metadataStr[k] = str
		}
	}

	return &rpc.Session{
		Id:           id,
		Name:         name,
		AgentType:    string(agentMode),
		Status:       a.statusToString(status),
		CreatedAt:    createdAt,
		LastActiveAt: updatedAt,
		Metadata:     metadataStr,
	}
}

// statusToString 转换状态为字符串
func (a *SessionManagerAdapter) statusToString(status session.SessionStatus) string {
	switch status {
	case session.SessionActive:
		return "running"
	case session.SessionIdle:
		return "idle"
	case session.SessionClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// EventBusAdapter 事件总线适配器
type EventBusAdapter struct {
	bus *eventbus.EventBus
}

// NewEventBusAdapter 创建事件总线适配器
func NewEventBusAdapter(bus *eventbus.EventBus) *EventBusAdapter {
	return &EventBusAdapter{
		bus: bus,
	}
}

// Subscribe 订阅事件（实现 gRPC 接口）
func (a *EventBusAdapter) Subscribe(ctx context.Context, sessionID string, eventTypes []string) (<-chan *rpc.Event, error) {
	// 创建输出通道
	out := make(chan *rpc.Event, 100)

	// 订阅事件
	subIDs := make([]string, 0)

	// 如果没有指定事件类型，订阅所有事件
	if len(eventTypes) == 0 || (len(eventTypes) == 1 && eventTypes[0] == "") {
		subID := a.bus.SubscribeGlobal(func(ctx context.Context, event eventbus.Event) error {
			rpcEvent := a.toRPCEvent(event)
			select {
			case out <- rpcEvent:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		subIDs = append(subIDs, subID)
	} else {
		// 订阅指定类型的事件
		for _, eventType := range eventTypes {
			et := eventbus.EventType(eventType)
			subID := a.bus.Subscribe(et, func(ctx context.Context, event eventbus.Event) error {
				// 过滤会话 ID
				if sessionID != "" && sessionID != "*" {
					if sid, ok := event.GetData()["session_id"].(string); ok && sid != sessionID {
						return nil
					}
				}

				rpcEvent := a.toRPCEvent(event)
				select {
				case out <- rpcEvent:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			subIDs = append(subIDs, subID)
		}
	}

	// 监听上下文取消，自动取消订阅
	go func() {
		<-ctx.Done()
		for _, subID := range subIDs {
			a.bus.Unsubscribe(subID)
		}
		close(out)
	}()

	return out, nil
}

// Publish 发布事件（实现 gRPC 接口）
func (a *EventBusAdapter) Publish(event *rpc.Event) error {
	// 转换为内部事件格式
	data := make(map[string]interface{})

	// 解析数据（JSON bytes -> map）
	// 注意：这里简化处理，实际应该解析 JSON
	// TODO: 实现 JSON 解析

	return a.bus.Publish(eventbus.EventType(event.Type), data)
}

// toRPCEvent 转换为 gRPC Event 格式
func (a *EventBusAdapter) toRPCEvent(event eventbus.Event) *rpc.Event {
	// 序列化数据
	// TODO: 实现 JSON 序列化

	return &rpc.Event{
		Type:      string(event.GetType()),
		SessionId: getStringFromMap(event.GetData(), "session_id"),
		Data:      []byte{}, // TODO: JSON 序列化
		Timestamp: event.GetTimestamp(),
	}
}

// getStringFromMap 从 map 中获取字符串
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// AgentAdapter Agent 适配器（用于工具执行）
type AgentAdapter struct{}

// NewAgentAdapter 创建 Agent 适配器
func NewAgentAdapter() *AgentAdapter {
	return &AgentAdapter{}
}

// ExecuteTool 执行工具（实现 gRPC 接口）
func (a *AgentAdapter) ExecuteTool(ctx context.Context, agent *core.Agent, toolName string, args map[string]interface{}) (map[string]interface{}, error) {
	// 通过 Agent 执行工具
	// 这需要与 core.Agent 集成
	// TODO: 实现工具执行逻辑

	return nil, fmt.Errorf("tool execution not yet implemented")
}

// ProcessMessage 处理消息（用于 SendMessage RPC）
func (a *AgentAdapter) ProcessMessage(ctx context.Context, agent *core.Agent, content string, callback func(string)) error {
	// 通过 Agent 处理消息
	// 这需要与 core.Agent 集成
	// TODO: 实现消息处理逻辑

	return fmt.Errorf("message processing not yet implemented")
}

// CommandAdapter 命令执行适配器
type CommandAdapter struct{}

// NewCommandAdapter 创建命令执行适配器
func NewCommandAdapter() *CommandAdapter {
	return &CommandAdapter{}
}

// ExecuteCommand 执行命令
func (a *CommandAdapter) ExecuteCommand(ctx context.Context, sessionID string, command string, args []string, workingDir string, env map[string]string, background bool) (int32, <-chan *rpc.CommandOutput, error) {
	// TODO: 实现命令执行逻辑
	// 这应该创建一个命令进程，并流式返回输出

	outputChan := make(chan *rpc.CommandOutput, 100)

	go func() {
		defer close(outputChan)
		// 模拟命令执行
		time.Sleep(100 * time.Millisecond)
		outputChan <- &rpc.CommandOutput{
			Type: rpc.CommandOutput_STDOUT,
			Data: []byte(fmt.Sprintf("Executing: %s %v\n", command, args)),
		}
		outputChan <- &rpc.CommandOutput{
			Type:     rpc.CommandOutput_EXIT,
			ExitCode: 0,
		}
	}()

	return 0, outputChan, nil
}
