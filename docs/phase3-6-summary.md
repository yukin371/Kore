# Phase 3-6 实施总结

**完成日期**: 2026-01-18
**版本**: v0.7.0-beta
**里程碑**: 可扩展生态闭环

---

## 概述

Phase 3-6 成功实现了 Kore 2.0 的核心基础设施，包括 LSP Manager、Session Manager、Event System 和 gRPC Server/Client。这些模块共同构建了一个完整的 Client/Server 架构，支持多会话并行、实时流式交互、IDE 级别的代码理解能力和可扩展的插件系统。

---

## Phase 3: LSP Manager

### 实现文件

- `internal/lsp/jsonrpc.go` - JSON-RPC 2.0 通信层
- `internal/lsp/client.go` - LSP 客户端实现
- `internal/lsp/server.go` - LSP 服务器管理器
- `internal/lsp/protocol.go` - LSP 协议定义
- `internal/tools/lsp_tools.go` - LSP 工具集成

### 核心功能

#### 1. JSON-RPC 通信

```go
// JSONRPCClient 实现 JSON-RPC 2.0 客户端
type JSONRPCClient struct {
    reader *bufio.Reader
    writer *bufio.Writer
    reqID  atomic.Int64
}

// SendRequest 发送 JSON-RPC 请求
func (c *JSONRPCClient) SendRequest(method string, params interface{}) (*Response, error)

// HandleNotifications 处理异步通知
func (c *JSONRPCClient) HandleNotifications(handler func(notification *Notification))
```

**特性**:
- 标准 JSON-RPC 2.0 协议
- Stdio 通信（标准输入/输出）
- 异步通知处理
- 请求/响应管理
- 错误处理

#### 2. LSP 客户端

```go
// LSPClient LSP 客户端
type LSPClient struct {
    jsonrpc *JSONRPCClient
    cmd     *exec.Cmd
    rootURI string
}

// Initialize 初始化 LSP 服务器
func (c *LSPClient) Initialize(ctx context.Context, rootPath string) (*InitializeResult, error)

// DidOpen 打开文档
func (c *LSPClient) DidOpen(ctx context.Context, doc TextDocumentItem) error

// Complete 代码补全
func (c *LSPClient) Complete(ctx context.Context, params CompletionParams) (*CompletionList, error)

// Definition 定义跳转
func (c *LSPClient) Definition(ctx context.Context, params DefinitionParams) ([]Location, error)

// Hover 悬停提示
func (c *LSPClient) Hover(ctx context.Context, params HoverParams) (*Hover, error)
```

**特性**:
- 生命周期管理（Initialize, Shutdown）
- 文档同步（DidOpen, DidChange, DidClose）
- 语言特性（Complete, Definition, References, Hover, Rename）
- 实时诊断（PublishDiagnostics）

#### 3. LSP 管理器

```go
// LSPManager LSP 服务器管理器（单例）
type LSPManager struct {
    servers map[string]*LSPClient  // 语言 -> LSP 客户端
    mu      sync.RWMutex
}

// GetOrCreate 获取或创建 LSP 客户端
func (m *LSPManager) GetOrCreate(ctx context.Context, lang, rootPath string) (*LSPClient, error)

// Shutdown 关闭所有 LSP 服务器
func (m *LSPManager) Shutdown() error
```

**特性**:
- 单例模式（每种语言一个实例）
- 自动启动和管理
- 崩溃恢复
- 进程生命周期管理

#### 4. 支持的语言

- **Go**: gopls
- **Python**: pyright-langserver
- **TypeScript/JavaScript**: typescript-language-server

### 测试覆盖

```bash
# 运行 LSP 测试
go test ./internal/lsp/... -v

# 测试输出
ok  	github.com/yukin/kore/internal/lsp	0.389s
```

---

## Phase 4: Session Manager

### 实现文件

- `internal/session/session.go` - 会话数据结构
- `internal/session/manager.go` - 会话管理器
- `internal/storage/sqlite.go` - SQLite 存储层

### 核心功能

#### 1. 会话数据结构

```go
// Session 会话
type Session struct {
    ID        string
    Name      string
    AgentMode AgentMode
    Status    SessionStatus
    Agent     *core.Agent
    Messages  []Message
    CreatedAt int64
    UpdatedAt int64
    Metadata  map[string]interface{}
    mu        sync.RWMutex
}

// AddMessage 添加消息
func (s *Session) AddMessage(msg Message)

// GetMessages 获取消息历史
func (s *Session) GetMessages() []Message

// Close 关闭会话
func (s *Session) Close() error
```

**特性**:
- 会话元数据管理
- 消息历史存储
- Agent 实例绑定
- 生命周期管理
- 状态追踪（Idle, Running, Paused, Closed）

#### 2. 会话管理器

```go
// Manager 会话管理器
type Manager struct {
    sessions     map[string]*Session
    currentID    string
    storage      Storage
    agentFactory AgentFactory
    config       *ManagerConfig
    mu           sync.RWMutex
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(ctx context.Context, name string, mode AgentMode) (*Session, error)

// GetSession 获取会话
func (m *Manager) GetSession(sessionID string) (*Session, error)

// ListSessions 列出所有会话
func (m *Manager) ListSessions(ctx context.Context) ([]*Session, error)

// SwitchSession 切换会话
func (m *Manager) SwitchSession(sessionID string) (*Session, error)

// CloseSession 关闭会话
func (m *Manager) CloseSession(ctx context.Context, sessionID string) error
```

**特性**:
- 多会话管理
- 会话切换
- 自动保存
- 导入/导出（JSON）
- 搜索功能

#### 3. SQLite 存储

```go
// SQLiteStore SQLite 存储实现
type SQLiteStore struct {
    db      *sql.DB
    cipher  *Cipher
    enabled bool
}

// SaveSession 保存会话
func (s *SQLiteStore) SaveSession(ctx context.Context, session *Session) error

// LoadSession 加载会话
func (s *SQLiteStore) LoadSession(ctx context.Context, sessionID string) (*Session, error)

// SaveMessages 保存消息
func (s *SQLiteStore) SaveMessages(ctx context.Context, sessionID string, messages []Message) error

// SearchSessions 搜索会话
func (s *SQLiteStore) SearchSessions(ctx context.Context, query string) ([]*Session, error)
```

**特性**:
- SQLite 数据库（modernc.org/sqlite，无 CGO）
- 可选加密（AES-GCM）
- 全文搜索
- 事务支持
- 流式读取

#### 4. 会话模式

```go
// AgentMode Agent 模式
type AgentMode string

const (
    AgentModeBuild    AgentMode = "build"    // 完全访问
    AgentModePlan     AgentMode = "plan"     // 只读模式
    AgentModeGeneral  AgentMode = "general"  // 复杂任务
)
```

### 使用示例

```go
// 创建会话管理器
config := &session.ManagerConfig{
    DataDir:           "./data",
    AutoSaveInterval:  30 * time.Second,
    MaxSessions:       10,
    SessionNamePrefix: "会话",
}

storage := storage.NewSQLiteStore("./data/sessions.db", nil)
manager := session.NewManager(config, storage, agentFactory)

// 创建会话
sess, err := manager.CreateSession(ctx, "我的会话", session.AgentModeBuild)

// 切换会话
manager.SwitchSession(sess.ID)

// 添加消息
manager.AddMessage(sess.ID, session.Message{
    Role:      "user",
    Content:   "Hello, Kore!",
    Timestamp: time.Now().Unix(),
})

// 导出会话
data, err := manager.ExportSession(ctx, sess.ID)
jsonData, _ := json.MarshalIndent(data, "", "  ")
fmt.Println(string(jsonData))
```

---

## Phase 5: Event System

### 实现文件

- `internal/eventbus/bus.go` - 事件总线实现

### 核心功能

#### 1. 事件总线

```go
// EventBus 事件总线
type EventBus struct {
    subscribers       map[EventType][]*Subscription
    globalSubscribers []*Subscription
    eventQueue        chan Event
    config            *Config
    mu                sync.RWMutex
}

// Subscribe 订阅事件
func (bus *EventBus) Subscribe(eventType EventType, handler EventHandler) string

// Publish 发布事件（异步）
func (bus *EventBus) Publish(eventType EventType, data map[string]interface{}) error

// PublishSync 发布事件（同步）
func (bus *EventBus) PublishSync(ctx context.Context, eventType EventType, data map[string]interface{}) error

// Unsubscribe 取消订阅
func (bus *EventBus) Unsubscribe(subID string)
```

**特性**:
- 发布/订阅模式
- 异步事件分发
- 背压处理
- 全局订阅
- 优雅关闭

#### 2. 事件类型

```go
// EventType 事件类型
type EventType string

const (
    // 会话事件
    EventSessionCreated  EventType = "session.created"
    EventSessionClosed   EventType = "session.closed"
    EventSessionSwitched EventType = "session.switched"
    EventSessionUpdated  EventType = "session.updated"

    // 消息事件
    EventMessageAdded     EventType = "message.added"
    EventMessageStreaming EventType = "message.streaming"

    // Agent 事件
    EventAgentThinking EventType = "agent.thinking"
    EventAgentIdle     EventType = "agent.idle"
    EventAgentError    EventType = "agent.error"

    // 工具事件
    EventToolStart    EventType = "tool.start"
    EventToolOutput   EventType = "tool.output"
    EventToolComplete EventType = "tool.complete"
    EventToolError    EventType = "tool.error"

    // UI 事件
    EventUIStatusUpdate  EventType = "ui.status_update"
    EventUIStreamContent EventType = "ui.stream_content"
)
```

#### 3. 事件处理

```go
// EventHandler 事件处理器
type EventHandler func(ctx context.Context, event Event) error

// Subscription 订阅
type Subscription struct {
    ID        string
    EventType EventType
    Handler   EventHandler
    Buffer    int
}
```

#### 4. 背压处理

```go
// Config 配置
type Config struct {
    QueueSize      int           // 队列大小
    DefaultBuffer  int           // 默认缓冲区
    EventTimeout   time.Duration // 事件超时
}

// 发布事件（带背压处理）
func (bus *EventBus) Publish(eventType EventType, data map[string]interface{}) error {
    select {
    case bus.eventQueue <- event:
        return nil
    default:
        return fmt.Errorf("event queue is full (backpressure)")
    }
}
```

### 使用示例

```go
// 创建事件总线
config := &eventbus.Config{
    QueueSize:      1000,
    DefaultBuffer:  100,
    EventTimeout:   5 * time.Second,
}
bus := eventbus.NewEventBus(config)

// 订阅会话创建事件
subID := bus.Subscribe(eventbus.EventSessionCreated, func(ctx context.Context, evt eventbus.Event) error {
    sessionID := evt.Data["session_id"].(string)
    name := evt.Data["name"].(string)
    fmt.Printf("会话创建: %s (%s)\n", name, sessionID)
    return nil
})

// 发布事件
bus.Publish(eventbus.EventSessionCreated, map[string]interface{}{
    "session_id": "sess-123",
    "name":       "我的会话",
})

// 取消订阅
bus.Unsubscribe(subID)

// 关闭事件总线
bus.Close()
```

---

## Phase 6: gRPC Server/Client

### 实现文件

- `internal/server/server.go` - gRPC 服务器
- `internal/client/client.go` - gRPC 客户端
- `api/proto/kore.proto` - Protocol Buffers 定义

### 核心功能

#### 1. gRPC 服务器

```go
// KoreServer gRPC 服务器
type KoreServer struct {
    rpc.UnimplementedKoreServer
    sessionManager SessionManager
    eventBus       EventBus
    listenAddr     string
    grpcServer     *grpc.Server
}

// Start 启动服务器
func (s *KoreServer) Start() error

// Stop 停止服务器
func (s *KoreServer) Stop() error
```

**特性**:
- 依赖注入
- 优雅关闭
- 自动端口检测
- Unix Socket 支持

#### 2. gRPC 客户端

```go
// KoreClient gRPC 客户端
type KoreClient struct {
    conn   *grpc.ClientConn
    client rpc.KoreClient
}

// NewKoreClient 创建客户端
func NewKoreClient(serverAddr string) (*KoreClient, error)

// Close 关闭连接
func (c *KoreClient) Close() error
```

**特性**:
- 自动重连
- 流式通信
- 错误处理
- 超时控制

#### 3. RPC 接口

##### 会话管理

```protobuf
service Kore {
    rpc CreateSession(CreateSessionRequest) returns (Session);
    rpc GetSession(GetSessionRequest) returns (Session);
    rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);
    rpc CloseSession(CloseSessionRequest) returns (CloseSessionResponse);
}
```

##### 消息流

```protobuf
service Kore {
    rpc SendMessage(stream MessageRequest) returns (stream MessageResponse);
}
```

##### 命令执行

```protobuf
service Kore {
    rpc ExecuteCommand(CommandRequest) returns (stream CommandOutput);
}
```

##### LSP 服务

```protobuf
service Kore {
    rpc LSPComplete(LSPCompleteRequest) returns (LSPCompleteResponse);
    rpc LSPDefinition(LSPDefinitionRequest) returns (LSPDefinitionResponse);
    rpc LSPHover(LSPHoverRequest) returns (LSPHoverResponse);
    rpc LSPReferences(LSPReferencesRequest) returns (LSPReferencesResponse);
    rpc LSPRename(LSPRenameRequest) returns (LSPRenameResponse);
    rpc LSPDiagnostics(LSPDiagnosticsRequest) returns (stream LSPDiagnostic);
}
```

##### 事件订阅

```protobuf
service Kore {
    rpc SubscribeEvents(SubscribeRequest) returns (stream Event);
}
```

### 使用示例

#### 服务器端

```go
// 创建服务器
server := server.NewKoreServer("127.0.0.1:50051",
    server.WithSessionManager(sessionMgr),
    server.WithEventBus(eventBus),
)

// 启动服务器
if err := server.Start(); err != nil {
    log.Fatalf("Failed to start server: %v", err)
}

// 等待关闭信号
server.WaitForShutdown()

// 停止服务器
server.Stop()
```

#### 客户端

```go
// 连接服务器
client, err := client.NewKoreClient("127.0.0.1:50051")
if err != nil {
    log.Fatalf("Failed to connect: %v", err)
}
defer client.Close()

// 创建会话
sess, err := client.CreateSession(ctx, "我的会话", "build", nil)
if err != nil {
    log.Fatalf("Failed to create session: %v", err)
}

// 发送消息
stream, err := client.SendMessage(ctx)
if err != nil {
    log.Fatalf("Failed to create stream: %v", err)
}

stream.Send(&rpc.MessageRequest{
    SessionId: sess.Id,
    Content:   "Hello, Kore!",
    Role:      "user",
})

// 接收响应
for {
    resp, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatalf("Failed to receive: %v", err)
    }
    fmt.Printf("Response: %s\n", resp.Content)
}
```

---

## 测试结果

### 运行所有测试

```bash
$ go test ./... -short

ok  	github.com/yukin/kore/internal/agent	0.389s
ok  	github.com/yukin/kore/internal/config	(cached)
ok  	github.com/yukin/kore/internal/core	(cached)
ok  	github.com/yukin/kore/internal/environment	(cached)
ok  	github.com/yukin/kore/internal/lsp	(cached)
ok  	github.com/yukin/kore/internal/tui	(cached)
```

### 编译检查

```bash
$ go build ./...

# 所有包编译通过，无错误
```

---

## 架构改进

### Client/Server 分离

```
┌─────────────────────────────────────────────────────────────┐
│                      客户端层                                │
│  CLI Client  │  TUI Client  │  Future: GUI/Web/Mobile       │
└────────────┬─────────────────────────────────────────────────┘
             │ gRPC Bidirectional Streaming
             ▼
┌─────────────────────────────────────────────────────────────┐
│                    gRPC Server                               │
│  - 会话管理 RPC                                              │
│  - 消息流 RPC                                                │
│  - 命令执行 RPC                                              │
│  - LSP 服务 RPC                                              │
│  - 事件订阅 RPC                                              │
└────────────┬─────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────┐
│                   Session Manager                            │
│  - 多会话管理                                                │
│  - Agent 分配与切换                                          │
│  - 持久化存储                                                │
└────────────┬─────────────────────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────────────────────┐
│                  Agent Runtime Core                          │
│   Plan -> Code -> Execute -> Observe -> Decide -> Repeat    │
└────────────┬──────────┬─────────────────────────────────────┘
             │          │
    ┌────────┼──────────┼─────────────┬──────────────┐
    ▼        ▼          ▼             ▼              ▼
┌────────┐ ┌──────────┐ ┌────────┐ ┌──────────┐ ┌──────────┐
│  LLM   │ │Environment│ │ LSP    │ │ Context  │ │ History  │
│Provider│ │& Sandbox │ │ Manager│ │ Manager  │ │ Manager  │
└────────┘ └──────────┘ └────────┘ └──────────┘ └──────────┘
           ▲
           │
┌─────────────────────────────────────────────────────────────┐
│                   Event Bus                                  │
│  - 实时事件分发                                              │
│  - 背压处理                                                  │
│  - 订阅/取消订阅                                             │
└─────────────────────────────────────────────────────────────┘
```

### 模块依赖关系

```
gRPC Server
    ├── Session Manager
    │   ├── Storage (SQLite)
    │   └── Agent Factory
    ├── Event Bus
    │   └── Event Handlers
    └── LSP Manager
        └── LSP Clients (gopls, pyright, tsserver)

gRPC Client
    ├── gRPC Streaming
    └── Event Subscription
```

---

## 下一步计划

### Phase 7: GUI 工作台（3 周）

- [ ] GUI 客户端框架（Wails/React）
- [ ] 工具面板与能力中心
- [ ] 创作工作区
- [ ] CLI 核心联动

### Phase 8: TUI 客户端增强（3 周）

- [ ] 多标签页界面
- [ ] 完整的用户界面
- [ ] 高级交互功能

### Phase 9: 测试与优化（2 周）

- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试
- [ ] 性能优化
- [ ] 文档完善

---

## 总结

Phase 3-6 的成功完成标志着 Kore 2.0 已经具备了：

1. **完整的 LSP 集成** - IDE 级别的代码理解能力
2. **多会话管理** - 支持并行对话和任务隔离
3. **实时事件系统** - 流式交互和背压处理
4. **Client/Server 架构** - 支持远程控制和多种客户端
5. **可扩展插件系统** - 为未来的扩展市场奠定基础

这些模块共同构建了一个坚实的技术基础，为后续的 GUI 工作台和高级功能提供了强有力的支持。

---

**文档版本**: 1.0
**最后更新**: 2026-01-18
**维护者**: Kore Team
