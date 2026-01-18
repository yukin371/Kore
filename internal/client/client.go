// Package client 提供 Kore 2.0 的 gRPC 客户端实现
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	rpc "github.com/yukin/kore/api/proto"
)

// KoreClient Kore 客户端
type KoreClient struct {
	conn   *grpc.ClientConn
	client rpc.KoreClient

	// 配置
	serverAddr string
	timeout    time.Duration

	// 状态管理
	connected bool
	mu        sync.RWMutex

	// 重连配置
	enableAutoReconnect bool
	reconnectDelay      time.Duration
}

// NewKoreClient 创建新的客户端
func NewKoreClient(serverAddr string, opts ...ClientOption) (*KoreClient, error) {
	client := &KoreClient{
		serverAddr:        serverAddr,
		timeout:           5 * time.Second,
		reconnectDelay:    1 * time.Second,
		enableAutoReconnect: false,
	}

	// 应用选项
	for _, opt := range opts {
		opt(client)
	}

	// 连接到服务器
	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

// ClientOption 客户端配置选项
type ClientOption func(*KoreClient)

// WithTimeout 设置默认超时
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *KoreClient) {
		c.timeout = timeout
	}
}

// WithAutoReconnect 启用自动重连
func WithAutoReconnect(enable bool, delay time.Duration) ClientOption {
	return func(c *KoreClient) {
		c.enableAutoReconnect = enable
		if delay > 0 {
			c.reconnectDelay = delay
		}
	}
}

// connect 建立连接（内部方法）
func (c *KoreClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// 创建连接
	conn, err := grpc.DialContext(ctx, c.serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*100), // 100 MB
			grpc.MaxCallSendMsgSize(1024*1024*100),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", c.serverAddr, err)
	}

	c.conn = conn
	c.client = rpc.NewKoreClient(conn)
	c.connected = true

	return nil
}

// Close 关闭连接
func (c *KoreClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	c.connected = false
	return nil
}

// IsConnected 检查是否已连接
func (c *KoreClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// ServerAddr 返回服务器地址
func (c *KoreClient) ServerAddr() string {
	return c.serverAddr
}

// ============================================================================
// 会话管理
// ============================================================================

// CreateSession 创建新会话
func (c *KoreClient) CreateSession(ctx context.Context, name, agentType string, config map[string]string) (*rpc.Session, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	req := &rpc.CreateSessionRequest{
		Name:      name,
		AgentType: agentType,
		Config:    config,
	}

	session, err := c.client.CreateSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GetSession 获取会话信息
func (c *KoreClient) GetSession(ctx context.Context, sessionID string) (*rpc.Session, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	req := &rpc.GetSessionRequest{
		SessionId: sessionID,
	}

	session, err := c.client.GetSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

// ListSessions 列出所有会话
func (c *KoreClient) ListSessions(ctx context.Context, limit, offset int32) ([]*rpc.Session, int32, error) {
	if !c.IsConnected() {
		return nil, 0, fmt.Errorf("not connected to server")
	}

	req := &rpc.ListSessionsRequest{
		Limit:  limit,
		Offset: offset,
	}

	resp, err := c.client.ListSessions(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sessions: %w", err)
	}

	return resp.Sessions, resp.Total, nil
}

// CloseSession 关闭会话
func (c *KoreClient) CloseSession(ctx context.Context, sessionID string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to server")
	}

	req := &rpc.CloseSessionRequest{
		SessionId: sessionID,
	}

	_, err := c.client.CloseSession(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	return nil
}

// ============================================================================
// 消息流（双向流）
// ============================================================================

// SendMessageClient 消息流客户端
type SendMessageClient struct {
	stream rpc.Kore_SendMessageClient
}

// Send 发送消息
func (mc *SendMessageClient) Send(ctx context.Context, content, role string, metadata map[string]string) error {
	req := &rpc.MessageRequest{
		SessionId: "", // 从上下文获取
		Content:    content,
		Role:       role,
		Metadata:   metadata,
	}

	return mc.stream.Send(req)
}

// Recv 接收消息
func (mc *SendMessageClient) Recv() (*rpc.MessageResponse, error) {
	return mc.stream.Recv()
}

// Close 关闭流
func (mc *SendMessageClient) Close() error {
	return mc.stream.CloseSend()
}

// SendMessage 创建消息流
func (c *KoreClient) SendMessage(ctx context.Context) (*SendMessageClient, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	stream, err := c.client.SendMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create message stream: %w", err)
	}

	return &SendMessageClient{stream: stream}, nil
}

// ============================================================================
// 命令执行
// ============================================================================

// ExecuteCommandClient 命令执行客户端
type ExecuteCommandClient struct {
	stream rpc.Kore_ExecuteCommandClient
}

// Recv 接收命令输出
func (ec *ExecuteCommandClient) Recv() (*rpc.CommandOutput, error) {
	return ec.stream.Recv()
}

// ExecuteCommand 执行命令
func (c *KoreClient) ExecuteCommand(ctx context.Context, req *rpc.CommandRequest) (*ExecuteCommandClient, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	stream, err := c.client.ExecuteCommand(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %w", err)
	}

	return &ExecuteCommandClient{stream: stream}, nil
}

// ============================================================================
// LSP 功能
// ============================================================================

// LSPComplete 代码补全
func (c *KoreClient) LSPComplete(ctx context.Context, req *rpc.LSPCompleteRequest) (*rpc.LSPCompleteResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	resp, err := c.client.LSPComplete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LSP complete failed: %w", err)
	}

	return resp, nil
}

// LSPDefinition 定义跳转
func (c *KoreClient) LSPDefinition(ctx context.Context, req *rpc.LSPDefinitionRequest) (*rpc.LSPDefinitionResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	resp, err := c.client.LSPDefinition(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LSP definition failed: %w", err)
	}

	return resp, nil
}

// LSPHover 悬停提示
func (c *KoreClient) LSPHover(ctx context.Context, req *rpc.LSPHoverRequest) (*rpc.LSPHoverResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	resp, err := c.client.LSPHover(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LSP hover failed: %w", err)
	}

	return resp, nil
}

// ============================================================================
// 事件订阅
// ============================================================================

// EventSubscriber 事件订阅器
type EventSubscriber struct {
	stream rpc.Kore_SubscribeEventsClient
}

// Recv 接收事件
func (es *EventSubscriber) Recv() (*rpc.Event, error) {
	return es.stream.Recv()
}

// Close 关闭订阅
func (es *EventSubscriber) Close() error {
	// gRPC streaming 会自动关闭，这里只是一个占位符
	return nil
}

// SubscribeEvents 订阅事件流
func (c *KoreClient) SubscribeEvents(ctx context.Context, req *rpc.SubscribeRequest) (*EventSubscriber, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to server")
	}

	stream, err := c.client.SubscribeEvents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	return &EventSubscriber{stream: stream}, nil
}

// ============================================================================
// 重连逻辑（自动重连）
// ============================================================================

// reconnect 尝试重新连接
func (c *KoreClient) reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 关闭旧连接
	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false

	// 尝试重新连接
	for i := 0; i < 3; i++ {
		time.Sleep(c.reconnectDelay)

		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()

		conn, err := grpc.DialContext(ctx, c.serverAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err == nil {
			c.conn = conn
			c.client = rpc.NewKoreClient(conn)
			c.connected = true
			return nil
		}
	}

	return fmt.Errorf("failed to reconnect after 3 attempts")
}

// Ping 检测连接是否活跃
func (c *KoreClient) Ping(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// 使用一个简单的 RPC 调用来检测连接
	_, err := c.client.ListSessions(ctx, &rpc.ListSessionsRequest{Limit: 1})
	if err != nil {
		// 检查是否是网络错误
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.Unavailable {
				return fmt.Errorf("connection lost")
			}
		}
	}

	return err
}
