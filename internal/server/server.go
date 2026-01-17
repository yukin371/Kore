// Package server 提供 Kore 2.0 的 gRPC 服务器实现
package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rpc "github.com/yukin/kore/api/proto"
)

// KoreServer 实现 gRPC 服务
type KoreServer struct {
	rpc.UnimplementedKoreServer

	// 依赖注入
	sessionManager SessionManager
	eventBus       EventBus

	// 服务器配置
	listenAddr string
	grpcServer *grpc.Server
	mu         sync.RWMutex

	// 生命周期管理
	started   bool
	shutdown  chan struct{}
	wg        sync.WaitGroup
}

// NewKoreServer 创建新的服务器实例
func NewKoreServer(listenAddr string, opts ...ServerOption) *KoreServer {
	server := &KoreServer{
		listenAddr: listenAddr,
		shutdown:   make(chan struct{}),
	}

	// 应用选项
	for _, opt := range opts {
		opt(server)
	}

	return server
}

// ServerOption 服务器配置选项
type ServerOption func(*KoreServer)

// WithSessionManager 设置会话管理器
func WithSessionManager(sm SessionManager) ServerOption {
	return func(s *KoreServer) {
		s.sessionManager = sm
	}
}

// WithEventBus 设置事件总线
func WithEventBus(eb EventBus) ServerOption {
	return func(s *KoreServer) {
		s.eventBus = eb
	}
}

// Start 启动 gRPC 服务器
func (s *KoreServer) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	// 创建监听器
	lis, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.listenAddr, err)
	}

	// 创建 gRPC 服务器
	s.grpcServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(1024*1024*100), // 100 MB
		grpc.MaxSendMsgSize(1024*1024*100),
	)

	// 注册服务
	rpc.RegisterKoreServer(s.grpcServer, s)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.grpcServer.Serve(lis); err != nil {
			select {
			case <-s.shutdown:
				// 正常关闭
				return
			default:
				// 错误
				fmt.Printf("server error: %v\n", err)
			}
		}
	}()

	return nil
}

// Stop 优雅关闭服务器
func (s *KoreServer) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return fmt.Errorf("server not started")
	}
	s.mu.Unlock()

	// 通知关闭
	close(s.shutdown)

	// 停止接受新连接
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// 等待所有 goroutine 完成
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	// 等待最多 10 秒
	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for server to shutdown")
	}
}

// Addr 返回服务器监听地址
func (s *KoreServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listenAddr
}

// ============================================================================
// 会话管理 RPC 实现（Phase 4 完成）
// ============================================================================

// CreateSession 创建新会话
func (s *KoreServer) CreateSession(ctx context.Context, req *rpc.CreateSessionRequest) (*rpc.Session, error) {
	if s.sessionManager == nil {
		return nil, status.Error(codes.Unimplemented, "session manager not configured")
	}

	session, err := s.sessionManager.CreateSession(ctx, req.Name, req.AgentType, req.Config)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return session, nil
}

// GetSession 获取会话信息
func (s *KoreServer) GetSession(ctx context.Context, req *rpc.GetSessionRequest) (*rpc.Session, error) {
	if s.sessionManager == nil {
		return nil, status.Error(codes.Unimplemented, "session manager not configured")
	}

	session, err := s.sessionManager.GetSession(ctx, req.SessionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return session, nil
}

// ListSessions 列出所有会话
func (s *KoreServer) ListSessions(ctx context.Context, req *rpc.ListSessionsRequest) (*rpc.ListSessionsResponse, error) {
	if s.sessionManager == nil {
		return nil, status.Error(codes.Unimplemented, "session manager not configured")
	}

	sessions, total, err := s.sessionManager.ListSessions(ctx, req.Limit, req.Offset)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &rpc.ListSessionsResponse{
		Sessions: sessions,
		Total:    total,
	}, nil
}

// CloseSession 关闭会话
func (s *KoreServer) CloseSession(ctx context.Context, req *rpc.CloseSessionRequest) (*rpc.CloseSessionResponse, error) {
	if s.sessionManager == nil {
		return nil, status.Error(codes.Unimplemented, "session manager not configured")
	}

	err := s.sessionManager.CloseSession(ctx, req.SessionId)
	if err != nil {
		return &rpc.CloseSessionResponse{Success: false}, status.Error(codes.Internal, err.Error())
	}

	return &rpc.CloseSessionResponse{Success: true}, nil
}

// ============================================================================
// 消息流 RPC 实现（Phase 6 完成）
// ============================================================================

// SendMessage 双向流式消息（占位符）
func (s *KoreServer) SendMessage(stream rpc.Kore_SendMessageServer) error {
	return status.Error(codes.Unimplemented, "not yet implemented")
}

// ============================================================================
// 命令执行 RPC 实现（Phase 2 完成）
// ============================================================================

// ExecuteCommand 执行命令并流式返回输出（占位符）
func (s *KoreServer) ExecuteCommand(req *rpc.CommandRequest, stream rpc.Kore_ExecuteCommandServer) error {
	return status.Error(codes.Unimplemented, "not yet implemented")
}

// ============================================================================
// LSP RPC 实现（Phase 3 完成）
// ============================================================================

// LSPComplete 代码补全（占位符）
func (s *KoreServer) LSPComplete(ctx context.Context, req *rpc.LSPCompleteRequest) (*rpc.LSPCompleteResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// LSPDefinition 定义跳转（占位符）
func (s *KoreServer) LSPDefinition(ctx context.Context, req *rpc.LSPDefinitionRequest) (*rpc.LSPDefinitionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// LSPHover 悬停提示（占位符）
func (s *KoreServer) LSPHover(ctx context.Context, req *rpc.LSPHoverRequest) (*rpc.LSPHoverResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// LSPReferences 引用查找（占位符）
func (s *KoreServer) LSPReferences(ctx context.Context, req *rpc.LSPReferencesRequest) (*rpc.LSPReferencesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// LSPRename 重命名（占位符）
func (s *KoreServer) LSPRename(ctx context.Context, req *rpc.LSPRenameRequest) (*rpc.LSPRenameResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// LSPDiagnostics 诊断信息流（占位符）
func (s *KoreServer) LSPDiagnostics(req *rpc.LSPDiagnosticsRequest, stream rpc.Kore_LSPDiagnosticsServer) error {
	return status.Error(codes.Unimplemented, "not yet implemented")
}

// ============================================================================
// 虚拟文档 RPC 实现（Phase 3 完成）
// ============================================================================

// CreateVirtualDocument 创建虚拟文档（占位符）
func (s *KoreServer) CreateVirtualDocument(ctx context.Context, req *rpc.CreateVirtualDocRequest) (*rpc.CreateVirtualDocResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// UpdateVirtualDocument 更新虚拟文档（占位符）
func (s *KoreServer) UpdateVirtualDocument(ctx context.Context, req *rpc.UpdateVirtualDocRequest) (*rpc.UpdateVirtualDocResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// CloseVirtualDocument 关闭虚拟文档（占位符）
func (s *KoreServer) CloseVirtualDocument(ctx context.Context, req *rpc.CloseVirtualDocRequest) (*rpc.CloseVirtualDocResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not yet implemented")
}

// ============================================================================
// 事件订阅 RPC 实现（Phase 5 完成）
// ============================================================================

// SubscribeEvents 订阅事件流（占位符）
func (s *KoreServer) SubscribeEvents(req *rpc.SubscribeRequest, stream rpc.Kore_SubscribeEventsServer) error {
	return status.Error(codes.Unimplemented, "not yet implemented")
}

// ============================================================================
// 接口定义（用于依赖注入）
// ============================================================================

// SessionManager 会话管理器接口
type SessionManager interface {
	CreateSession(ctx context.Context, name, agentType string, config map[string]string) (*rpc.Session, error)
	GetSession(ctx context.Context, sessionID string) (*rpc.Session, error)
	ListSessions(ctx context.Context, limit, offset int32) ([]*rpc.Session, int32, error)
	CloseSession(ctx context.Context, sessionID string) error
}

// EventBus 事件总线接口
type EventBus interface {
	Subscribe(ctx context.Context, sessionID string, eventTypes []string) (<-chan *rpc.Event, error)
	Publish(event *rpc.Event) error
}

// IsRunning 检查服务器是否正在运行
func (s *KoreServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// WaitForShutdown 阻塞直到服务器关闭
func (s *KoreServer) WaitForShutdown() {
	s.wg.Wait()
}

// 自动检测可用端口
func AutoDetectPort() (string, error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := lis.Addr().(*net.TCPAddr)
	lis.Close()
	return fmt.Sprintf("127.0.0.1:%d", addr.Port), nil
}

// CreateTempUnixSocket 创建临时 Unix Socket（仅在 Linux/macOS）
func CreateTempUnixSocket() (string, error) {
	if os.Getenv("GOOS") == "windows" {
		return "", fmt.Errorf("unix socket not supported on windows")
	}

	dir := os.TempDir()
	path := dir + "/kore.sock"
	os.Remove(path) // 清理旧文件

	return path, nil
}
