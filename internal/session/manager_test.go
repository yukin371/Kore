package session

import (
	"context"
	"testing"
	"time"

	"github.com/yukin/kore/internal/core"
	"github.com/yukin/kore/internal/storage"
)

// MockStorage 用于测试的模拟存储
type MockStorage struct {
	sessions  map[string]*Session
	messages  map[string][]Message
	callCount int
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		sessions: make(map[string]*Session),
		messages: make(map[string][]Message),
	}
}

func (m *MockStorage) SaveSession(ctx context.Context, session *Session) error {
	m.sessions[session.ID] = session
	m.callCount++
	return nil
}

func (m *MockStorage) LoadSession(ctx context.Context, sessionID string) (*Session, error) {
	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, storage.ErrSessionNotFound
	}
	return sess, nil
}

func (m *MockStorage) ListSessions(ctx context.Context) ([]*Session, error) {
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (m *MockStorage) DeleteSession(ctx context.Context, sessionID string) error {
	if _, ok := m.sessions[sessionID]; !ok {
		return storage.ErrSessionNotFound
	}
	delete(m.sessions, sessionID)
	delete(m.messages, sessionID)
	return nil
}

func (m *MockStorage) SaveMessages(ctx context.Context, sessionID string, messages []Message) error {
	m.messages[sessionID] = messages
	return nil
}

func (m *MockStorage) LoadMessages(ctx context.Context, sessionID string) ([]Message, error) {
	messages, ok := m.messages[sessionID]
	if !ok {
		return []Message{}, nil
	}
	return messages, nil
}

func (m *MockStorage) SearchSessions(ctx context.Context, query string) ([]*Session, error) {
	return m.ListSessions(ctx)
}

// MockAgentFactory 模拟 Agent 工厂
func MockAgentFactory(sess *Session) (*core.Agent, error) {
	// 返回一个空的 Agent 实例
	return core.NewAgent(nil, nil, nil, ""), nil
}

func TestNewManager(t *testing.T) {
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       10,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)

	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr == nil {
		t.Fatal("Manager is nil")
	}

	if mgr.config.MaxSessions != 10 {
		t.Errorf("Expected MaxSessions 10, got %d", mgr.config.MaxSessions)
	}
}

func TestCreateSession(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话1", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if sess == nil {
		t.Fatal("Session is nil")
	}

	if sess.Name != "测试会话1" {
		t.Errorf("Expected name '测试会话1', got '%s'", sess.Name)
	}

	if sess.AgentMode != ModeBuild {
		t.Errorf("Expected mode %s, got %s", ModeBuild, sess.AgentMode)
	}

	if sess.Status != SessionActive {
		t.Errorf("Expected status %d, got %d", SessionActive, sess.Status)
	}
}

func TestCreateSessionMaxLimit(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       2, // 最多 2 个会话
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建第一个会话
	_, err = mgr.CreateSession(ctx, "会话1", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session 1: %v", err)
	}

	// 创建第二个会话
	_, err = mgr.CreateSession(ctx, "会话2", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session 2: %v", err)
	}

	// 创建第三个会话（应该失败）
	_, err = mgr.CreateSession(ctx, "会话3", ModeBuild)
	if err == nil {
		t.Error("Expected error when creating session beyond limit, got nil")
	}
}

func TestGetSession(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 获取会话
	fetched, err := mgr.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if fetched.ID != sess.ID {
		t.Errorf("Expected ID %s, got %s", sess.ID, fetched.ID)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 获取不存在的会话
	_, err = mgr.GetSession("non-existent-id")
	if err == nil {
		t.Error("Expected error when getting non-existent session, got nil")
	}
}

func TestDeleteSession(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 删除会话
	err = mgr.DeleteSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// 验证会话已删除
	_, err = mgr.GetSession(sess.ID)
	if err == nil {
		t.Error("Expected error when getting deleted session, got nil")
	}
}

func TestSwitchSession(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建两个会话
	sess1, err := mgr.CreateSession(ctx, "会话1", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session 1: %v", err)
	}

	sess2, err := mgr.CreateSession(ctx, "会话2", ModePlan)
	if err != nil {
		t.Fatalf("Failed to create session 2: %v", err)
	}

	// 切换到会话1
	current, err := mgr.SwitchSession(sess1.ID)
	if err != nil {
		t.Fatalf("Failed to switch to session 1: %v", err)
	}

	if current.ID != sess1.ID {
		t.Errorf("Expected current session ID %s, got %s", sess1.ID, current.ID)
	}

	// 切换到会话2
	current, err = mgr.SwitchSession(sess2.ID)
	if err != nil {
		t.Fatalf("Failed to switch to session 2: %v", err)
	}

	if current.ID != sess2.ID {
		t.Errorf("Expected current session ID %s, got %s", sess2.ID, current.ID)
	}
}

func TestAddMessage(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 添加消息
	msg := Message{
		ID:        "msg1",
		SessionID: sess.ID,
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now().Unix(),
	}

	err = mgr.AddMessage(sess.ID, msg)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	// 验证消息
	messages := sess.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Content != "Hello" {
		t.Errorf("Expected message content 'Hello', got '%s'", messages[0].Content)
	}

	// 验证统计信息
	stats := sess.GetStatistics()
	if stats.MessageCount != 1 {
		t.Errorf("Expected message count 1, got %d", stats.MessageCount)
	}

	if stats.UserMsgCount != 1 {
		t.Errorf("Expected user message count 1, got %d", stats.UserMsgCount)
	}
}

func TestSessionTags(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 添加标签
	err = mgr.AddSessionTag(ctx, sess.ID, "golang")
	if err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	err = mgr.AddSessionTag(ctx, sess.ID, "testing")
	if err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	// 验证标签
	tags := sess.GetTags()
	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}

	// 移除标签
	err = mgr.RemoveSessionTag(ctx, sess.ID, "golang")
	if err != nil {
		t.Fatalf("Failed to remove tag: %v", err)
	}

	// 验证标签已移除
	tags = sess.GetTags()
	if len(tags) != 1 {
		t.Errorf("Expected 1 tag after removal, got %d", len(tags))
	}

	if tags[0] != "testing" {
		t.Errorf("Expected tag 'testing', got '%s'", tags[0])
	}
}

func TestSessionStatistics(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 添加消息
	msg1 := Message{
		ID:        "msg1",
		SessionID: sess.ID,
		Role:      "user",
		Content:   "This is a test message",
		Timestamp: time.Now().Unix(),
	}

	msg2 := Message{
		ID:        "msg2",
		SessionID: sess.ID,
		Role:      "assistant",
		Content:   "This is a response",
		Timestamp: time.Now().Unix(),
	}

	mgr.AddMessage(sess.ID, msg1)
	mgr.AddMessage(sess.ID, msg2)

	// 获取统计信息
	stats, err := mgr.GetSessionStatistics(sess.ID)
	if err != nil {
		t.Fatalf("Failed to get statistics: %v", err)
	}

	if stats.MessageCount != 2 {
		t.Errorf("Expected message count 2, got %d", stats.MessageCount)
	}

	if stats.UserMsgCount != 1 {
		t.Errorf("Expected user message count 1, got %d", stats.UserMsgCount)
	}

	if stats.AssistantMsgCount != 1 {
		t.Errorf("Expected assistant message count 1, got %d", stats.AssistantMsgCount)
	}

	if stats.TokenUsed == 0 {
		t.Error("Expected non-zero token usage")
	}
}

func TestRenameSession(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "旧名称", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 重命名
	err = mgr.RenameSession(ctx, sess.ID, "新名称")
	if err != nil {
		t.Fatalf("Failed to rename session: %v", err)
	}

	// 验证
	if sess.Name != "新名称" {
		t.Errorf("Expected name '新名称', got '%s'", sess.Name)
	}
}

func TestCloseSession(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 设置为当前会话
	_, err = mgr.SwitchSession(sess.ID)
	if err != nil {
		t.Fatalf("Failed to switch session: %v", err)
	}

	// 关闭会话
	err = mgr.CloseSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// 验证会话已关闭
	if sess.Status != SessionClosed {
		t.Errorf("Expected status %d, got %d", SessionClosed, sess.Status)
	}

	// 验证当前会话已清空
	_, err = mgr.GetCurrentSession()
	if err == nil {
		t.Error("Expected error when getting current session after close, got nil")
	}
}

func TestToolExecution(t *testing.T) {
	ctx := context.Background()
	config := &ManagerConfig{
		DataDir:           "./test_data",
		AutoSaveInterval:  30 * time.Second,
		MaxSessions:       0,
		SessionNamePrefix: "测试会话",
	}

	storage := NewMockStorage()
	mgr, err := NewManager(config, storage, MockAgentFactory)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 创建会话
	sess, err := mgr.CreateSession(ctx, "测试会话", ModeBuild)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// 记录工具执行
	execution := ToolExecution{
		ID:        "exec1",
		Tool:      "read_file",
		Arguments: `{"path": "test.go"}`,
		Result:    "success",
		Success:   true,
		Timestamp: time.Now().Unix(),
	}

	err = mgr.RecordToolExecution(sess.ID, execution)
	if err != nil {
		t.Fatalf("Failed to record tool execution: %v", err)
	}

	// 获取工具执行记录
	executions, err := mgr.GetToolExecutions(sess.ID)
	if err != nil {
		t.Fatalf("Failed to get tool executions: %v", err)
	}

	if len(executions) != 1 {
		t.Errorf("Expected 1 execution, got %d", len(executions))
	}

	// 验证统计信息
	stats := sess.GetStatistics()
	if stats.ToolCallCount != 1 {
		t.Errorf("Expected tool call count 1, got %d", stats.ToolCallCount)
	}
}
