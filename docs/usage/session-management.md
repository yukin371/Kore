# Session Management 使用指南

**版本**: v0.7.0-beta
**最后更新**: 2026-01-18

---

## 目录

1. [概述](#概述)
2. [会话基础](#会话基础)
3. [会话管理](#会话管理)
4. [持久化存储](#持久化存储)
5. [高级功能](#高级功能)
6. [使用示例](#使用示例)
7. [最佳实践](#最佳实践)

---

## 概述

Session Manager 是 Kore 2.0 的核心组件，负责管理多个独立的对话会话。每个会话拥有：
- 独立的 Agent 实例
- 独立的消息历史
- 独立的上下文管理
- 独立的配置和状态

### 核心特性

- **多会话并行**: 同时运行多个独立的对话
- **会话隔离**: 会话间完全隔离，互不干扰
- **持久化存储**: 自动保存会话和消息到 SQLite
- **可选加密**: 支持 AES-GCM 加密存储
- **导入/导出**: JSON 格式的会话导入导出
- **全文搜索**: 基于 SQLite FTS5 的会话搜索

---

## 会话基础

### 会话结构

```go
type Session struct {
    ID        string                 // 会话唯一标识
    Name      string                 // 会话名称
    AgentMode AgentMode             // Agent 模式（build/plan/general）
    Status    SessionStatus          // 状态（Idle/Running/Paused/Closed）
    Agent     *core.Agent           // Agent 实例
    Messages  []Message             // 消息历史
    CreatedAt int64                 // 创建时间
    UpdatedAt int64                 // 更新时间
    Metadata  map[string]interface{} // 元数据
}
```

### Agent 模式

Kore 支持三种 Agent 模式：

```go
const (
    AgentModeBuild    AgentMode = "build"    // 完全访问权限
    AgentModePlan     AgentMode = "plan"     // 只读模式
    AgentModeGeneral  AgentMode = "general"  // 复杂任务处理
)
```

#### Build Agent
- **权限**: 完全访问
- **功能**: 文件读写、命令执行、LSP 调用
- **用途**: 实际的代码修改和构建任务

#### Plan Agent
- **权限**: 只读
- **功能**: 文件读取、代码分析
- **用途**: 代码审查、架构规划

#### General Agent
- **权限**: 受限
- **功能**: 平衡的读写权限
- **用途**: 复杂的多步骤任务

### 会话状态

```go
const (
    SessionStatusIdle    SessionStatus = "idle"     // 空闲
    SessionStatusRunning SessionStatus = "running"  // 运行中
    SessionStatusPaused  SessionStatus = "paused"   // 暂停
    SessionStatusClosed  SessionStatus = "closed"   // 已关闭
)
```

---

## 会话管理

### 创建会话管理器

```go
import (
    "context"
    "github.com/yukin/kore/internal/session"
    "github.com/yukin/kore/internal/storage"
)

// 配置
config := &session.ManagerConfig{
    DataDir:           "./data",
    AutoSaveInterval:  30 * time.Second,
    MaxSessions:       10,
    SessionNamePrefix: "会话",
}

// 存储层
storage := storage.NewSQLiteStore("./data/sessions.db", nil)

// Agent 工厂
agentFactory := func(sess *session.Session) (*core.Agent, error) {
    // 创建 Agent 实例
    return core.NewAgent(sess.AgentMode, sess.ID)
}

// 创建管理器
manager, err := session.NewManager(config, storage, agentFactory)
if err != nil {
    log.Fatal(err)
}
```

### 创建会话

```go
ctx := context.Background()

// 创建会话
sess, err := manager.CreateSession(ctx, "我的项目", session.AgentModeBuild)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("会话创建成功: %s (ID: %s)\n", sess.Name, sess.ID)
```

### 获取会话

```go
// 通过 ID 获取
sess, err := manager.GetSession(sessionID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("会话: %s, 状态: %s\n", sess.Name, sess.Status)
```

### 列出所有会话

```go
sessions, err := manager.ListSessions(ctx)
if err != nil {
    log.Fatal(err)
}

for _, sess := range sessions {
    fmt.Printf("- %s (%s) [%s]\n", sess.Name, sess.ID, sess.Status)
}
```

### 切换会话

```go
// 切换到指定会话
sess, err := manager.SwitchSession(sessionID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("切换到会话: %s\n", sess.Name)
```

### 关闭会话

```go
// 关闭会话
err := manager.CloseSession(ctx, sessionID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("会话 %s 已关闭\n", sessionID)
```

### 重命名会话

```go
// 重命名会话
err := manager.RenameSession(ctx, sessionID, "新名称")
if err != nil {
    log.Fatal(err)
}
```

---

## 持久化存储

### SQLite 存储层

```go
import "github.com/yukin/kore/internal/storage"

// 创建存储（无加密）
store := storage.NewSQLiteStore("./data/sessions.db", nil)

// 创建加密存储
cipher := storage.NewCipher([]byte("32-byte-key-1234567890123456"))
store := storage.NewSQLiteStore("./data/sessions.db", cipher)
```

### 数据库 Schema

```sql
-- 会话表
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    agent_mode TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    metadata JSON
);

-- 消息表
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    metadata JSON,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- 索引
CREATE INDEX idx_messages_session_id ON messages(session_id);
CREATE INDEX idx_messages_timestamp ON messages(timestamp);
```

### 自动保存

会话管理器会自动保存所有更改：

```go
// 配置自动保存间隔
config := &session.ManagerConfig{
    AutoSaveInterval: 30 * time.Second, // 每 30 秒保存一次
}
```

### 手动保存

```go
// 管理器会在以下情况自动保存：
// 1. 创建会话后
// 2. 关闭会话前
// 3. 定时自动保存（AutoSaveInterval）

// 如果需要立即保存所有会话：
ctx := context.Background()
manager.Shutdown(ctx) // 会触发保存
```

---

## 高级功能

### 消息管理

```go
// 添加消息
manager.AddMessage(sessionID, session.Message{
    ID:        "msg-123",
    SessionID: sessionID,
    Role:      "user",
    Content:   "Hello, Kore!",
    Timestamp: time.Now().Unix(),
})

// 获取消息
messages, err := manager.GetMessages(sessionID)
for _, msg := range messages {
    fmt.Printf("[%s] %s: %s\n", msg.Role, msg.Timestamp, msg.Content)
}
```

### 导出会话

```go
// 导出为 JSON
data, err := manager.ExportSession(ctx, sessionID)
if err != nil {
    log.Fatal(err)
}

// 保存到文件
jsonData, _ := json.MarshalIndent(data, "", "  ")
os.WriteFile("session-export.json", jsonData, 0644)
```

### 导入会话

```go
// 从 JSON 导入
jsonData, _ := os.ReadFile("session-export.json")
var data map[string]interface{}
json.Unmarshal(jsonData, &data)

// 导入会话
sess, err := manager.ImportSession(ctx, data)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("会话导入成功: %s\n", sess.Name)
```

### 搜索会话

```go
// 搜索会话（基于名称和消息内容）
sessions, err := manager.SearchSessions(ctx, "database")
if err != nil {
    log.Fatal(err)
}

for _, sess := range sessions {
    fmt.Printf("- %s: %s\n", sess.ID, sess.Name)
}
```

### 会话元数据

```go
// 设置元数据
sess.Metadata["project"] = "my-project"
sess.Metadata["language"] = "go"
sess.Metadata["framework"] = "gin"

// 保存会话
storage.SaveSession(ctx, sess)
```

---

## 使用示例

### 示例 1: 基本会话管理

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/yukin/kore/internal/session"
    "github.com/yukin/kore/internal/storage"
)

func main() {
    // 创建存储
    store := storage.NewSQLiteStore("./data/sessions.db", nil)

    // 创建管理器
    config := &session.ManagerConfig{
        DataDir:           "./data",
        AutoSaveInterval:  30 * time.Second,
        MaxSessions:       10,
    }

    manager, err := session.NewManager(config, store, agentFactory)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // 创建会话
    sess1, _ := manager.CreateSession(ctx, "项目 A", session.AgentModeBuild)
    sess2, _ := manager.CreateSession(ctx, "项目 B", session.AgentModePlan)

    // 列出会话
    sessions, _ := manager.ListSessions(ctx)
    for _, s := range sessions {
        fmt.Printf("- %s (%s)\n", s.Name, s.ID)
    }

    // 切换会话
    manager.SwitchSession(sess1.ID)

    // 添加消息
    manager.AddMessage(sess1.ID, session.Message{
        Role:      "user",
        Content:   "帮我实现用户认证",
        Timestamp: time.Now().Unix(),
    })

    // 关闭会话
    manager.CloseSession(ctx, sess1.ID)
}
```

### 示例 2: 多会话并行

```go
package main

import (
    "context"
    "sync"

    "github.com/yukin/kore/internal/session"
)

func main() {
    manager := createSessionManager()
    ctx := context.Background()

    // 创建多个会话
    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()

            sess, _ := manager.CreateSession(ctx, fmt.Sprintf("任务 %d", idx), session.AgentModeBuild)

            // 执行任务
            processSession(sess)

            // 完成后关闭
            manager.CloseSession(ctx, sess.ID)
        }(i)
    }

    wg.Wait()
}

func processSession(sess *session.Session) {
    // 处理会话任务
    manager.AddMessage(sess.ID, session.Message{
        Role:      "user",
        Content:   "执行任务",
        Timestamp: time.Now().Unix(),
    })
}
```

### 示例 3: 会话导入导出

```go
package main

import (
    "context"
    "encoding/json"
    "os"
)

func main() {
    manager := createSessionManager()
    ctx := context.Background()

    // 导出会话
    sess, _ := manager.CreateSession(ctx, "我的会话", session.AgentModeBuild)
    data, _ := manager.ExportSession(ctx, sess.ID)

    // 保存到文件
    jsonData, _ := json.MarshalIndent(data, "", "  ")
    os.WriteFile("session-backup.json", jsonData, 0644)

    // 从文件导入
    jsonData, _ = os.ReadFile("session-backup.json")
    var importedData map[string]interface{}
    json.Unmarshal(jsonData, &importedData)

    // 导入会话
    importedSess, _ := manager.ImportSession(ctx, importedData)
    fmt.Printf("导入的会话: %s\n", importedSess.Name)
}
```

---

## 最佳实践

### 1. 会话命名

```go
// 好的命名
sess, _ := manager.CreateSession(ctx, "用户认证模块", session.AgentModeBuild)
sess, _ := manager.CreateSession(ctx, "API 文档审查", session.AgentModePlan)

// 避免模糊的命名
sess, _ := manager.CreateSession(ctx, "任务 1", session.AgentModeBuild) // 不推荐
```

### 2. 选择合适的 Agent 模式

```go
// 实际代码修改
sess, _ := manager.CreateSession(ctx, "实现用户注册", session.AgentModeBuild)

// 代码审查
sess, _ := manager.CreateSession(ctx, "审查 PR #123", session.AgentModePlan)

// 复杂的多步骤任务
sess, _ := manager.CreateSession(ctx, "重构支付系统", session.AgentModeGeneral)
```

### 3. 及时关闭不需要的会话

```go
// 使用 defer 确保会话关闭
sess, err := manager.CreateSession(ctx, "临时会话", session.AgentModeBuild)
if err != nil {
    return err
}
defer manager.CloseSession(ctx, sess.ID)

// 使用会话
// ...
```

### 4. 使用元数据组织会话

```go
// 设置项目元数据
sess.Metadata["project"] = "kore"
sess.Metadata["component"] = "auth"
sess.Metadata["priority"] = "high"
sess.Metadata["assignee"] = "john"

// 基于元数据筛选会话
sessions, _ := manager.ListSessions(ctx)
for _, s := range sessions {
    if s.Metadata["project"] == "kore" {
        // 处理 Kore 项目的会话
    }
}
```

### 5. 错误处理

```go
// 总是检查错误
sess, err := manager.CreateSession(ctx, "我的会话", session.AgentModeBuild)
if err != nil {
    // 处理错误
    if err.Error() == "maximum session limit reached" {
        // 关闭旧会话
        closeOldSessions(manager)
        // 重试
        sess, err = manager.CreateSession(ctx, "我的会话", session.AgentModeBuild)
    }
    if err != nil {
        log.Fatal(err)
    }
}
```

### 6. 资源清理

```go
// 程序退出时，确保管理器正确关闭
func main() {
    manager := createSessionManager()
    defer func() {
        ctx := context.Background()
        manager.Shutdown(ctx)
    }()

    // 使用管理器
    // ...
}
```

---

## API 参考

### Session Manager 接口

```go
type Manager struct{}

func NewManager(config *ManagerConfig, storage Storage, agentFactory AgentFactory) (*Manager, error)
func (m *Manager) CreateSession(ctx context.Context, name string, mode AgentMode) (*Session, error)
func (m *Manager) GetSession(sessionID string) (*Session, error)
func (m *Manager) ListSessions(ctx context.Context) ([]*Session, error)
func (m *Manager) CloseSession(ctx context.Context, sessionID string) error
func (m *Manager) SwitchSession(sessionID string) (*Session, error)
func (m *Manager) RenameSession(ctx context.Context, sessionID string, newName string) error
func (m *Manager) AddMessage(sessionID string, msg Message) error
func (m *Manager) GetMessages(sessionID string) ([]Message, error)
func (m *Manager) ExportSession(ctx context.Context, sessionID string) (map[string]interface{}, error)
func (m *Manager) ImportSession(ctx context.Context, data map[string]interface{}) (*Session, error)
func (m *Manager) SearchSessions(ctx context.Context, query string) ([]*Session, error)
func (m *Manager) Shutdown(ctx context.Context) error
```

### Session 接口

```go
type Session struct{}

func NewSession(id, name string, mode AgentMode, agent *core.Agent) *Session
func (s *Session) AddMessage(msg Message)
func (s *Session) GetMessages() []Message
func (s *Session) IsActive() bool
func (s *Session) Close() error
```

---

## 相关文档

- [LSP Manager 使用指南](./lsp-manager.md)
- [Event Bus 使用指南](./event-bus.md)
- [gRPC API 使用指南](./grpc-api.md)
- [Phase 3-6 实施总结](../phase3-6-summary.md)

---

**文档版本**: 1.0
**最后更新**: 2026-01-18
**维护者**: Kore Team
