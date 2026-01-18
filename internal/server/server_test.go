package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpc "github.com/yukin/kore/api/proto"
)

// TestCreateSession 测试创建会话
func TestCreateSession(t *testing.T) {
	// 创建服务器
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(NewMockSessionManager()),
		WithEventBus(NewMockEventBus()),
	)

	// 启动服务器
	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// 创建测试请求
	req := &rpc.CreateSessionRequest{
		Name:      "test-session",
		AgentType: "general",
		Config: map[string]string{
			"test": "value",
		},
	}

	// 调用 RPC
	ctx := context.Background()
	resp, err := server.CreateSession(ctx, req)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test-session", resp.Name)
	assert.Equal(t, "general", resp.AgentType)
	assert.Equal(t, "running", resp.Status)
}

// TestGetSession 测试获取会话
func TestGetSession(t *testing.T) {
	mockMgr := NewMockSessionManager()

	// 预先创建一个会话
	ctx := context.Background()
	session, err := mockMgr.CreateSession(ctx, "test-session", "general", nil)
	require.NoError(t, err)

	// 创建服务器
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(mockMgr),
		WithEventBus(NewMockEventBus()),
	)

	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// 测试获取存在的会话
	req := &rpc.GetSessionRequest{
		SessionId: session.Id,
	}

	resp, err := server.GetSession(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, session.Id, resp.Id)

	// 测试获取不存在的会话
	req2 := &rpc.GetSessionRequest{
		SessionId: "non-existent",
	}

	resp2, err := server.GetSession(ctx, req2)
	assert.Error(t, err)
	assert.Nil(t, resp2)
}

// TestListSessions 测试列出会话
func TestListSessions(t *testing.T) {
	mockMgr := NewMockSessionManager()

	// 创建几个会话
	ctx := context.Background()
	_, _ = mockMgr.CreateSession(ctx, "session-1", "general", nil)
	_, _ = mockMgr.CreateSession(ctx, "session-2", "build", nil)

	// 创建服务器
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(mockMgr),
		WithEventBus(NewMockEventBus()),
	)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// 测试列出会话
	req := &rpc.ListSessionsRequest{
		Limit:  10,
		Offset: 0,
	}

	resp, err := server.ListSessions(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(2), resp.Total)
	assert.Len(t, resp.Sessions, 2)
}

// TestCloseSession 测试关闭会话
func TestCloseSession(t *testing.T) {
	mockMgr := NewMockSessionManager()

	// 创建会话
	ctx := context.Background()
	session, err := mockMgr.CreateSession(ctx, "test-session", "general", nil)
	require.NoError(t, err)

	// 创建服务器
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(mockMgr),
		WithEventBus(NewMockEventBus()),
	)

	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// 测试关闭会话
	req := &rpc.CloseSessionRequest{
		SessionId: session.Id,
	}

	resp, err := server.CloseSession(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)

	// 验证会话已关闭
	_, err = mockMgr.GetSession(ctx, session.Id)
	assert.Error(t, err)
}

// TestSendMessage 测试消息流
func TestSendMessage(t *testing.T) {
	mockMgr := NewMockSessionManager()

	// 创建会话
	ctx := context.Background()
	session, err := mockMgr.CreateSession(ctx, "test-session", "general", nil)
	require.NoError(t, err)

	// 创建模拟流
	mockStream := &MockSendMessageStream{
		sessionID: session.Id,
		messages:  make([]string, 0),
	}

	// 创建服务器
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(mockMgr),
		WithEventBus(NewMockEventBus()),
	)

	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// 测试发送消息
	err = server.SendMessage(mockStream)
	assert.NoError(t, err)
	assert.Len(t, mockStream.messages, 1)
}

// MockSendMessageStream 模拟消息流
type MockSendMessageStream struct {
	sessionID string
	messages  []string
	sentCount int
	recvCount int
}

func (m *MockSendMessageStream) Send(resp *rpc.MessageResponse) error {
	m.messages = append(m.messages, resp.Content)
	m.sentCount++
	return nil
}

func (m *MockSendMessageStream) Recv() (*rpc.MessageRequest, error) {
	m.recvCount++
	if m.recvCount > 1 {
		return nil, assert.AnError // EOF
	}

	return &rpc.MessageRequest{
		SessionId: m.sessionID,
		Content:   "Hello, Kore!",
		Role:      "user",
		Metadata:  make(map[string]string),
	}, nil
}

func (m *MockSendMessageStream) Context() context.Context {
	return context.Background()
}

// TestSubscribeEvents 测试事件订阅
func TestSubscribeEvents(t *testing.T) {
	mockBus := NewMockEventBus()

	// 创建服务器
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(NewMockSessionManager()),
		WithEventBus(mockBus),
	)

	err := server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// 创建模拟流
	mockStream := &MockEventStream{
		events: make([]*rpc.Event, 0),
	}

	// 测试订阅事件
	req := &rpc.SubscribeRequest{
		SessionId:  "test-session",
		EventTypes: []string{"test.event"},
	}

	err = server.SubscribeEvents(req, mockStream)
	assert.NoError(t, err)
	assert.Len(t, mockStream.events, 1)
}

// MockEventStream 模拟事件流
type MockEventStream struct {
	events []*rpc.Event
	ctx    context.Context
	cancel context.CancelFunc
}

func (m *MockEventStream) Send(event *rpc.Event) error {
	m.events = append(m.events, event)
	// 接收一个事件后取消
	if len(m.events) >= 1 {
		m.cancel()
	}
	return nil
}

func (m *MockEventStream) Context() context.Context {
	if m.ctx == nil {
		m.ctx, m.cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	}
	return m.ctx
}

// TestExecuteCommand 测试命令执行
func TestExecuteCommand(t *testing.T) {
	mockMgr := NewMockSessionManager()

	// 创建会话
	ctx := context.Background()
	session, err := mockMgr.CreateSession(ctx, "test-session", "general", nil)
	require.NoError(t, err)

	// 创建服务器
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(mockMgr),
		WithEventBus(NewMockEventBus()),
	)

	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// 创建模拟流
	mockStream := &MockCommandStream{
		outputs: make([]*rpc.CommandOutput, 0),
	}

	// 测试执行命令
	req := &rpc.CommandRequest{
		SessionId:  session.Id,
		Command:    "echo",
		Args:       []string{"hello"},
		WorkingDir: "/tmp",
		Env:        map[string]string{"TEST": "value"},
		Background: false,
	}

	err = server.ExecuteCommand(req, mockStream)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(mockStream.outputs), 2) // 至少有输出和退出消息
}

// MockCommandStream 模拟命令流
type MockCommandStream struct {
	outputs []*rpc.CommandOutput
	ctx     context.Context
}

func (m *MockCommandStream) Send(output *rpc.CommandOutput) error {
	m.outputs = append(m.outputs, output)
	return nil
}

func (m *MockCommandStream) Context() context.Context {
	if m.ctx == nil {
		m.ctx = context.Background()
	}
	return m.ctx
}

// TestServerLifecycle 测试服务器生命周期
func TestServerLifecycle(t *testing.T) {
	// 创建服务器
	server := NewKoreServer("127.0.0.1:0")

	// 测试启动
	err := server.Start()
	assert.NoError(t, err)
	assert.True(t, server.IsRunning())

	// 测试重复启动
	err = server.Start()
	assert.Error(t, err)

	// 测试停止
	err = server.Stop()
	assert.NoError(t, err)
	assert.False(t, server.IsRunning())

	// 测试重复停止
	err = server.Stop()
	assert.Error(t, err)
}

// TestAutoDetectPort 测试自动端口检测
func TestAutoDetectPort(t *testing.T) {
	port, err := AutoDetectPort()
	assert.NoError(t, err)
	assert.NotEmpty(t, port)
	assert.Contains(t, port, "127.0.0.1:")
}

// BenchmarkCreateSession 性能测试
func BenchmarkCreateSession(b *testing.B) {
	mockMgr := NewMockSessionManager()
	server := NewKoreServer("127.0.0.1:0",
		WithSessionManager(mockMgr),
		WithEventBus(NewMockEventBus()),
	)

	ctx := context.Background()
	req := &rpc.CreateSessionRequest{
		Name:      "bench-session",
		AgentType: "general",
		Config:    nil,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = server.CreateSession(ctx, req)
	}
}
