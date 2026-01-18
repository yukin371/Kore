# gRPC API 使用指南

**版本**: v0.7.0-beta
**最后更新**: 2026-01-18

---

## 目录

1. [概述](#概述)
2. [架构设计](#架构设计)
3. [服务定义](#服务定义)
4. [服务器使用](#服务器使用)
5. [客户端使用](#客户端使用)
6. [流式通信](#流式通信)
7. [错误处理](#错误处理)
8. [使用示例](#使用示例)
9. [最佳实践](#最佳实践)

---

## 概述

Kore 2.0 采用 Client/Server 架构，通过 gRPC 进行通信。服务器提供会话管理、消息流、命令执行、LSP 服务和事件订阅等 RPC 接口，客户端可以通过这些接口与服务器交互。

### 核心特性

- **双向流式通信**: 支持实时的双向消息流
- **多客户端支持**: 同时支持 CLI、TUI、GUI 等多种客户端
- **远程控制**: 支持通过网络远程控制服务器
- **类型安全**: 使用 Protocol Buffers 定义接口
- **高性能**: 基于 HTTP/2 和 gRPC
- **优雅关闭**: 支持优雅的服务器关闭

### 通信模式

```
┌─────────────┐                    ┌─────────────┐
│   Client    │                    │   Server    │
├─────────────┤                    ├─────────────┤
│ CLI Client  │                    │ Session Mgr │
│ TUI Client  │◄─── gRPC ──────────▶│ Event Bus   │
│ GUI Client  │                    │ LSP Manager │
│ Web Client  │                    │ Agent Core  │
└─────────────┘                    └─────────────┘
```

---

## 架构设计

### Protocol Buffers 定义

文件位置: `api/proto/kore.proto`

```protobuf
syntax = "proto3";

package kore;

option go_package = "github.com/yukin/kore/api/proto";

// Kore 服务
service Kore {
    // 会话管理
    rpc CreateSession(CreateSessionRequest) returns (Session);
    rpc GetSession(GetSessionRequest) returns (Session);
    rpc ListSessions(ListSessionsRequest) returns (ListSessionsResponse);
    rpc CloseSession(CloseSessionRequest) returns (CloseSessionResponse);

    // 消息流（双向流）
    rpc SendMessage(stream MessageRequest) returns (stream MessageResponse);

    // 命令执行（流式输出）
    rpc ExecuteCommand(CommandRequest) returns (stream CommandOutput);

    // LSP 请求
    rpc LSPComplete(LSPCompleteRequest) returns (LSPCompleteResponse);
    rpc LSPDefinition(LSPDefinitionRequest) returns (LSPDefinitionResponse);
    rpc LSPHover(LSPHoverRequest) returns (LSPHoverResponse);
    rpc LSPReferences(LSPReferencesRequest) returns (LSPReferencesResponse);
    rpc LSPRename(LSPRenameRequest) returns (LSPRenameResponse);
    rpc LSPDiagnostics(LSPDiagnosticsRequest) returns (stream LSPDiagnostic);

    // 虚拟文档
    rpc CreateVirtualDocument(CreateVirtualDocRequest) returns (CreateVirtualDocResponse);
    rpc UpdateVirtualDocument(UpdateVirtualDocRequest) returns (UpdateVirtualDocResponse);
    rpc CloseVirtualDocument(CloseVirtualDocRequest) returns (CloseVirtualDocResponse);

    // 事件订阅
    rpc SubscribeEvents(SubscribeRequest) returns (stream Event);
}
```

### 消息类型

```protobuf
// 会话
message Session {
    string id = 1;
    string name = 2;
    string agent_type = 3;
    string status = 4;
    int64 created_at = 5;
    int64 last_active_at = 6;
    map<string, string> metadata = 7;
}

// 消息
message MessageRequest {
    string session_id = 1;
    string content = 2;
    string role = 3;
}

message MessageResponse {
    string content = 1;
    string role = 2;
    int64 timestamp = 3;
    bool done = 4;
}

// 命令
message CommandRequest {
    string session_id = 1;
    string command = 2;
    repeated string args = 3;
    string working_dir = 4;
    map<string, string> env = 5;
}

message CommandOutput {
    enum OutputType {
        STDOUT = 0;
        STDERR = 1;
        EXIT = 2;
    }
    OutputType type = 1;
    bytes data = 2;
    int32 exit_code = 3;
}

// 事件
message Event {
    string type = 1;
    string session_id = 2;
    map<string, string> data = 3;
    int64 timestamp = 4;
}
```

---

## 服务定义

### 会话管理 RPC

#### CreateSession

创建新的会话。

**请求**:
```protobuf
message CreateSessionRequest {
    string name = 1;
    string agent_type = 2;  // "build", "plan", "general"
    map<string, string> config = 3;
}
```

**响应**:
```protobuf
message Session {
    string id = 1;
    string name = 2;
    string agent_type = 3;
    string status = 4;
    int64 created_at = 5;
    int64 last_active_at = 6;
    map<string, string> metadata = 7;
}
```

#### GetSession

获取会话信息。

**请求**:
```protobuf
message GetSessionRequest {
    string session_id = 1;
}
```

**响应**: `Session`

#### ListSessions

列出所有会话。

**请求**:
```protobuf
message ListSessionsRequest {
    int32 limit = 1;
    int32 offset = 2;
}
```

**响应**:
```protobuf
message ListSessionsResponse {
    repeated Session sessions = 1;
    int32 total = 2;
}
```

#### CloseSession

关闭会话。

**请求**:
```protobuf
message CloseSessionRequest {
    string session_id = 1;
}
```

**响应**:
```protobuf
message CloseSessionResponse {
    bool success = 1;
}
```

### 消息流 RPC

#### SendMessage

双向流式消息传输。

**请求流**: `MessageRequest`
```protobuf
message MessageRequest {
    string session_id = 1;
    string content = 2;
    string role = 3;
}
```

**响应流**: `MessageResponse`
```protobuf
message MessageResponse {
    string content = 1;
    string role = 2;
    int64 timestamp = 3;
    bool done = 4;
}
```

### 命令执行 RPC

#### ExecuteCommand

执行命令并流式返回输出。

**请求**:
```protobuf
message CommandRequest {
    string session_id = 1;
    string command = 2;
    repeated string args = 3;
    string working_dir = 4;
    map<string, string> env = 5;
}
```

**响应流**: `CommandOutput`
```protobuf
message CommandOutput {
    enum OutputType {
        STDOUT = 0;
        STDERR = 1;
        EXIT = 2;
    }
    OutputType type = 1;
    bytes data = 2;
    int32 exit_code = 3;
}
```

### LSP 服务 RPC

#### LSPComplete

代码补全。

**请求**:
```protobuf
message LSPCompleteRequest {
    string session_id = 1;
    string file_uri = 2;
    int32 line = 3;
    int32 character = 4;
}
```

**响应**:
```protobuf
message LSPCompleteResponse {
    repeated CompletionItem items = 1;
}

message CompletionItem {
    string label = 1;
    string kind = 2;
    string detail = 3;
    string documentation = 4;
    string insert_text = 5;
}
```

#### LSPDefinition

定义跳转。

**请求**:
```protobuf
message LSPDefinitionRequest {
    string session_id = 1;
    string file_uri = 2;
    int32 line = 3;
    int32 character = 4;
}
```

**响应**:
```protobuf
message LSPDefinitionResponse {
    repeated Location locations = 1;
}

message Location {
    string uri = 1;
    Range range = 2;
}

message Range {
    Position start = 1;
    Position end = 2;
}

message Position {
    int32 line = 1;
    int32 character = 2;
}
```

#### LSPHover

悬停提示。

**请求**:
```protobuf
message LSPHoverRequest {
    string session_id = 1;
    string file_uri = 2;
    int32 line = 3;
    int32 character = 4;
}
```

**响应**:
```protobuf
message LSPHoverResponse {
    HoverContent contents = 1;
}

message HoverContent {
    string kind = 1;
    string value = 2;
}
```

#### LSPReferences

引用查找。

**请求**:
```protobuf
message LSPReferencesRequest {
    string session_id = 1;
    string file_uri = 2;
    int32 line = 3;
    int32 character = 4;
}
```

**响应**:
```protobuf
message LSPReferencesResponse {
    repeated Location locations = 1;
}
```

#### LSPRename

重命名符号。

**请求**:
```protobuf
message LSPRenameRequest {
    string session_id = 1;
    string file_uri = 2;
    int32 line = 3;
    int32 character = 4;
    string new_name = 5;
}
```

**响应**:
```protobuf
message LSPRenameResponse {
    WorkspaceEdit edit = 1;
}

message WorkspaceEdit {
    map<string, TextEdit> changes = 1;
}

message TextEdit {
    Range range = 1;
    string new_text = 2;
}
```

#### LSPDiagnostics

诊断信息流。

**请求**:
```protobuf
message LSPDiagnosticsRequest {
    string session_id = 1;
    string file_uri = 2;
}
```

**响应流**: `LSPDiagnostic`
```protobuf
message LSPDiagnostic {
    Range range = 1;
    string severity = 2;
    string code = 3;
    string source = 4;
    string message = 5;
}
```

### 虚拟文档 RPC

#### CreateVirtualDocument

创建虚拟文档。

**请求**:
```protobuf
message CreateVirtualDocRequest {
    string session_id = 1;
    string path = 2;
    string content = 3;
    string language = 4;
}
```

**响应**:
```protobuf
message CreateVirtualDocResponse {
    string document_id = 1;
}
```

#### UpdateVirtualDocument

更新虚拟文档。

**请求**:
```protobuf
message UpdateVirtualDocRequest {
    string session_id = 1;
    string document_id = 2;
    string content = 3;
}
```

**响应**:
```protobuf
message UpdateVirtualDocResponse {
    bool success = 1;
}
```

#### CloseVirtualDocument

关闭虚拟文档。

**请求**:
```protobuf
message CloseVirtualDocRequest {
    string session_id = 1;
    string document_id = 2;
}
```

**响应**:
```protobuf
message CloseVirtualDocResponse {
    bool success = 1;
}
```

### 事件订阅 RPC

#### SubscribeEvents

订阅事件流。

**请求**:
```protobuf
message SubscribeRequest {
    repeated string event_types = 1;
}
```

**响应流**: `Event`
```protobuf
message Event {
    string type = 1;
    string session_id = 2;
    map<string, string> data = 3;
    int64 timestamp = 4;
}
```

---

## 服务器使用

### 创建服务器

```go
package main

import (
    "context"
    "log"

    "github.com/yukin/kore/internal/server"
    "github.com/yukin/kore/internal/session"
    "github.com/yukin/kore/internal/eventbus"
)

func main() {
    // 创建依赖
    sessionMgr := createSessionManager()
    eventBus := eventbus.NewEventBus(nil)

    // 创建服务器
    srv := server.NewKoreServer("127.0.0.1:50051",
        server.WithSessionManager(sessionMgr),
        server.WithEventBus(eventBus),
    )

    // 启动服务器
    if err := srv.Start(); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }

    log.Printf("Server started on %s", srv.Addr())

    // 等待关闭信号
    srv.WaitForShutdown()

    // 停止服务器
    if err := srv.Stop(); err != nil {
        log.Fatalf("Failed to stop server: %v", err)
    }
}
```

### 自动端口检测

```go
// 自动检测可用端口
addr, err := server.AutoDetectPort()
if err != nil {
    log.Fatal(err)
}

srv := server.NewKoreServer(addr)
```

### Unix Socket（Linux/macOS）

```go
// 创建临时 Unix Socket
path, err := server.CreateTempUnixSocket()
if err != nil {
    log.Fatal(err)
}

srv := server.NewKoreServer(path)
```

### 优雅关闭

```go
// 使用信号处理
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    fmt.Println("\n正在关闭服务器...")
    srv.Stop()
    os.Exit(0)
}()

srv.WaitForShutdown()
```

---

## 客户端使用

### 创建客户端

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/yukin/kore/internal/client"
)

func main() {
    // 连接服务器
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cli, err := client.NewKoreClient(ctx, "127.0.0.1:50051")
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer cli.Close()

    log.Println("Connected to server")

    // 使用客户端
    // ...
}
```

### 会话管理

```go
ctx := context.Background()

// 创建会话
sess, err := cli.CreateSession(ctx, &rpc.CreateSessionRequest{
    Name:      "我的会话",
    AgentType: "build",
    Config:    map[string]string{},
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("会话创建: %s (%s)\n", sess.Name, sess.Id)

// 获取会话
sess, err = cli.GetSession(ctx, &rpc.GetSessionRequest{
    SessionId: sess.Id,
})

// 列出会话
list, err := cli.ListSessions(ctx, &rpc.ListSessionsRequest{
    Limit:  10,
    Offset: 0,
})
for _, s := range list.Sessions {
    fmt.Printf("- %s (%s)\n", s.Name, s.Id)
}

// 关闭会话
resp, err := cli.CloseSession(ctx, &rpc.CloseSessionRequest{
    SessionId: sess.Id,
})
if resp.Success {
    fmt.Println("会话已关闭")
}
```

### 消息流（双向流）

```go
ctx := context.Background()

// 创建消息流
stream, err := cli.SendMessage(ctx)
if err != nil {
    log.Fatal(err)
}

// 发送消息
err = stream.Send(&rpc.MessageRequest{
    SessionId: "sess-123",
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
        log.Fatal(err)
    }

    fmt.Printf("[%s] %s\n", resp.Role, resp.Content)

    if resp.Done {
        break
    }
}

// 关闭流
stream.CloseSend()
```

### 命令执行（流式输出）

```go
ctx := context.Background()

// 执行命令
stream, err := cli.ExecuteCommand(ctx, &rpc.CommandRequest{
    SessionId:  "sess-123",
    Command:    "go",
    Args:       []string{"test", "./..."},
    WorkingDir: "/path/to/project",
})
if err != nil {
    log.Fatal(err)
}

// 接收输出
for {
    output, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }

    switch output.Type {
    case rpc.CommandOutput_STDOUT:
        fmt.Printf("STDOUT: %s", output.Data)
    case rpc.CommandOutput_STDERR:
        fmt.Printf("STDERR: %s", output.Data)
    case rpc.CommandOutput_EXIT:
        fmt.Printf("Exit code: %d\n", output.ExitCode)
    }
}
```

### LSP 服务

```go
ctx := context.Background()

// 代码补全
resp, err := cli.LSPComplete(ctx, &rpc.LSPCompleteRequest{
    SessionId: "sess-123",
    FileUri:   "file:///path/to/file.go",
    Line:      10,
    Character: 5,
})
if err != nil {
    log.Fatal(err)
}

for _, item := range resp.Items {
    fmt.Printf("- %s: %s\n", item.Label, item.Detail)
}

// 定义跳转
resp, err := cli.LSPDefinition(ctx, &rpc.LSPDefinitionRequest{
    SessionId: "sess-123",
    FileUri:   "file:///path/to/file.go",
    Line:      10,
    Character: 5,
})

for _, loc := range resp.Locations {
    fmt.Printf("%s:%d:%d\n", loc.Uri, loc.Range.Start.Line, loc.Range.Start.Character)
}
```

### 事件订阅

```go
ctx := context.Background()

// 订阅事件
stream, err := cli.SubscribeEvents(ctx, &rpc.SubscribeRequest{
    EventTypes: []string{
        "session.created",
        "message.added",
        "tool.start",
    },
})
if err != nil {
    log.Fatal(err)
}

// 接收事件
for {
    event, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("[%s] %v\n", event.Type, event.Data)
}
```

---

## 流式通信

### 双向流（SendMessage）

```go
// 客户端
stream, _ := cli.SendMessage(ctx)

// 发送多条消息
for i := 0; i < 5; i++ {
    stream.Send(&rpc.MessageRequest{
        SessionId: "sess-123",
        Content:   fmt.Sprintf("Message %d", i),
        Role:      "user",
    })
}

// 接收响应
for {
    resp, _ := stream.Recv()
    if resp.Done {
        break
    }
    fmt.Println(resp.Content)
}
```

### 服务器流（ExecuteCommand, LSPDiagnostics, SubscribeEvents）

```go
// 客户端接收流式响应
stream, _ := cli.ExecuteCommand(ctx, &rpc.CommandRequest{
    SessionId: "sess-123",
    Command:   "go",
    Args:      []string{"build"},
})

for {
    output, _ := stream.Recv()
    if output == nil {
        break
    }
    fmt.Print(string(output.Data))
}
```

---

## 错误处理

### gRPC 状态码

```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// 检查错误码
sess, err := cli.GetSession(ctx, &rpc.GetSessionRequest{
    SessionId: "invalid-id",
})

if err != nil {
    st, ok := status.FromError(err)
    if ok {
        switch st.Code() {
        case codes.NotFound:
            fmt.Println("会话不存在")
        case codes.InvalidArgument:
            fmt.Println("无效的参数")
        case codes.Internal:
            fmt.Println("服务器内部错误")
        default:
            fmt.Printf("错误: %s\n", st.Message())
        }
    }
}
```

### 超时处理

```go
// 设置超时
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

sess, err := cli.CreateSession(ctx, &rpc.CreateSessionRequest{
    Name:      "我的会话",
    AgentType: "build",
})

if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        fmt.Println("请求超时")
    }
}
```

### 重试机制

```go
// 带重试的请求
func CreateSessionWithRetry(cli *client.KoreClient, req *rpc.CreateSessionRequest) (*rpc.Session, error) {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        sess, err := cli.CreateSession(ctx, req)
        cancel()

        if err == nil {
            return sess, nil
        }

        // 检查是否可重试
        if status.Code(err) == codes.Unavailable {
            time.Sleep(time.Second * time.Duration(i+1))
            continue
        }

        return nil, err
    }
    return nil, fmt.Errorf("max retries exceeded")
}
```

---

## 使用示例

### 示例 1: 简单的 CLI 客户端

```go
package main

import (
    "bufio"
    "context"
    "fmt"
    "log"
    "os"

    "github.com/yukin/kore/internal/client"
    rpc "github.com/yukin/kore/api/proto"
)

func main() {
    // 连接服务器
    cli, err := client.NewKoreClient(context.Background(), "127.0.0.1:50051")
    if err != nil {
        log.Fatal(err)
    }
    defer cli.Close()

    // 创建会话
    sess, err := cli.CreateSession(context.Background(), &rpc.CreateSessionRequest{
        Name:      "CLI 会话",
        AgentType: "build",
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("会话: %s\n", sess.Name)

    // 读取用户输入
    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Print("> ")
        if !scanner.Scan() {
            break
        }

        input := scanner.Text()
        if input == "exit" {
            break
        }

        // 发送消息
        stream, err := cli.SendMessage(context.Background())
        if err != nil {
            log.Fatal(err)
        }

        stream.Send(&rpc.MessageRequest{
            SessionId: sess.Id,
            Content:   input,
            Role:      "user",
        })

        // 接收响应
        for {
            resp, err := stream.Recv()
            if err != nil {
                break
            }
            fmt.Printf("%s", resp.Content)
            if resp.Done {
                break
            }
        }
        fmt.Println()
    }
}
```

### 示例 2: 事件监听器

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yukin/kore/internal/client"
)

func main() {
    cli, _ := client.NewKoreClient(context.Background(), "127.0.0.1:50051")
    defer cli.Close()

    // 订阅所有事件
    stream, err := cli.SubscribeEvents(context.Background(), &rpc.SubscribeRequest{})
    if err != nil {
        log.Fatal(err)
    }

    // 监听事件
    for {
        event, err := stream.Recv()
        if err != nil {
            log.Fatal(err)
        }

        fmt.Printf("[%s] %s: %v\n", event.Type, event.SessionId, event.Data)
    }
}
```

---

## 最佳实践

### 1. 连接管理

```go
// 使用 defer 确保关闭连接
cli, err := client.NewKoreClient(ctx, addr)
if err != nil {
    return err
}
defer cli.Close()

// 使用客户端
// ...
```

### 2. 上下文管理

```go
// 总是使用带超时的上下文
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

resp, err := cli.GetSession(ctx, &rpc.GetSessionRequest{
    SessionId: sessionID,
})
```

### 3. 流式关闭

```go
// 确保关闭流
stream, err := cli.SendMessage(ctx)
if err != nil {
    return err
}
defer stream.CloseSend()

// 使用流
// ...
```

### 4. 错误处理

```go
// 检查所有错误
sess, err := cli.CreateSession(ctx, req)
if err != nil {
    // 处理错误
    return fmt.Errorf("create session failed: %w", err)
}
```

---

## API 参考

### Server 接口

```go
type KoreServer struct{}

func NewKoreServer(listenAddr string, opts ...ServerOption) *KoreServer
func (s *KoreServer) Start() error
func (s *KoreServer) Stop() error
func (s *KoreServer) Addr() string
func (s *KoreServer) IsRunning() bool
func (s *KoreServer) WaitForShutdown()

func AutoDetectPort() (string, error)
func CreateTempUnixSocket() (string, error)
```

### Client 接口

```go
type KoreClient struct{}

func NewKoreClient(ctx context.Context, serverAddr string) (*KoreClient, error)
func (c *KoreClient) Close() error

func (c *KoreClient) CreateSession(ctx context.Context, req *CreateSessionRequest) (*Session, error)
func (c *KoreClient) GetSession(ctx context.Context, req *GetSessionRequest) (*Session, error)
func (c *KoreClient) ListSessions(ctx context.Context, req *ListSessionsRequest) (*ListSessionsResponse, error)
func (c *KoreClient) CloseSession(ctx context.Context, req *CloseSessionRequest) (*CloseSessionResponse, error)
func (c *KoreClient) SendMessage(ctx context.Context) (Kore_SendMessageClient, error)
func (c *KoreClient) ExecuteCommand(ctx context.Context, req *CommandRequest) (Kore_ExecuteCommandClient, error)
func (c *KoreClient) LSPComplete(ctx context.Context, req *LSPCompleteRequest) (*LSPCompleteResponse, error)
func (c *KoreClient) LSPDefinition(ctx context.Context, req *LSPDefinitionRequest) (*LSPDefinitionResponse, error)
func (c *KoreClient) LSPHover(ctx context.Context, req *LSPHoverRequest) (*LSPHoverResponse, error)
func (c *KoreClient) LSPReferences(ctx context.Context, req *LSPReferencesRequest) (*LSPReferencesResponse, error)
func (c *KoreClient) LSPRename(ctx context.Context, req *LSPRenameRequest) (*LSPRenameResponse, error)
func (c *KoreClient) LSPDiagnostics(ctx context.Context, req *LSPDiagnosticsRequest) (Kore_LSPDiagnosticsClient, error)
func (c *KoreClient) SubscribeEvents(ctx context.Context, req *SubscribeRequest) (Kore_SubscribeEventsClient, error)
```

---

## 相关文档

- [Session Management 使用指南](./session-management.md)
- [Event Bus 使用指南](./event-bus.md)
- [LSP Manager 使用指南](./lsp-manager.md)
- [Phase 3-6 实施总结](../phase3-6-summary.md)

---

**文档版本**: 1.0
**最后更新**: 2026-01-18
**维护者**: Kore Team
