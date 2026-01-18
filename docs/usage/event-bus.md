# Event Bus 使用指南

**版本**: v0.7.0-beta
**最后更新**: 2026-01-18

---

## 目录

1. [概述](#概述)
2. [核心概念](#核心概念)
3. [事件类型](#事件类型)
4. [发布订阅](#发布订阅)
5. [背压处理](#背压处理)
6. [使用示例](#使用示例)
7. [最佳实践](#最佳实践)

---

## 概述

Event Bus 是 Kore 2.0 的实时事件分发系统，采用发布/订阅模式，支持背压处理和异步事件分发。它是实现实时流式交互的核心组件。

### 核心特性

- **发布/订阅模式**: 解耦事件生产者和消费者
- **背压处理**: 队列、超时、缓冲区配置
- **异步分发**: 不阻塞事件发布者
- **全局订阅**: 支持监听所有事件
- **类型安全**: 强类型的事件定义
- **生命周期管理**: 优雅关闭和资源清理

---

## 核心概念

### 事件结构

```go
type Event struct {
    Type      EventType              `json:"type"`       // 事件类型
    Timestamp int64                  `json:"timestamp"`  // 时间戳
    Data      map[string]interface{} `json:"data"`       // 事件数据
}
```

### 事件处理器

```go
type EventHandler func(ctx context.Context, event Event) error
```

### 订阅

```go
type Subscription struct {
    ID        string        // 订阅 ID
    EventType EventType     // 事件类型（空字符串 = 全局订阅）
    Handler   EventHandler  // 处理器函数
    Buffer    int           // 缓冲区大小
}
```

### 配置

```go
type Config struct {
    QueueSize      int           // 事件队列大小（0 = 无缓冲）
    DefaultBuffer  int           // 默认缓冲区大小
    EventTimeout   time.Duration // 事件处理超时
}
```

---

## 事件类型

### 会话事件

```go
const (
    EventSessionCreated  EventType = "session.created"   // 会话创建
    EventSessionClosed   EventType = "session.closed"    // 会话关闭
    EventSessionSwitched EventType = "session.switched"  // 会话切换
    EventSessionUpdated  EventType = "session.updated"   // 会话更新
)
```

### 消息事件

```go
const (
    EventMessageAdded     EventType = "message.added"      // 消息添加
    EventMessageStreaming EventType = "message.streaming"  // 消息流式传输
)
```

### Agent 事件

```go
const (
    EventAgentThinking EventType = "agent.thinking"  // Agent 思考中
    EventAgentIdle     EventType = "agent.idle"      // Agent 空闲
    EventAgentError    EventType = "agent.error"     // Agent 错误
)
```

### 工具事件

```go
const (
    EventToolStart    EventType = "tool.start"     // 工具开始执行
    EventToolOutput   EventType = "tool.output"    // 工具输出
    EventToolComplete EventType = "tool.complete"  // 工具完成
    EventToolError    EventType = "tool.error"     // 工具错误
)
```

### UI 事件

```go
const (
    EventUIStatusUpdate  EventType = "ui.status_update"   // UI 状态更新
    EventUIStreamContent EventType = "ui.stream_content"  // UI 流式内容
)
```

---

## 发布订阅

### 创建事件总线

```go
import "github.com/yukin/kore/internal/eventbus"

// 使用默认配置
bus := eventbus.NewEventBus(nil)

// 自定义配置
config := &eventbus.Config{
    QueueSize:      1000,           // 队列大小
    DefaultBuffer:  100,            // 默认缓冲区
    EventTimeout:   5 * time.Second, // 超时
}
bus := eventbus.NewEventBus(config)
```

### 订阅事件

```go
// 订阅特定事件类型
subID := bus.Subscribe(eventbus.EventSessionCreated, func(ctx context.Context, evt eventbus.Event) error {
    sessionID := evt.Data["session_id"].(string)
    name := evt.Data["name"].(string)
    fmt.Printf("会话创建: %s (%s)\n", name, sessionID)
    return nil
})

// 全局订阅（监听所有事件）
globalSubID := bus.SubscribeGlobal(func(ctx context.Context, evt eventbus.Event) error {
    fmt.Printf("[%s] %v\n", evt.Type, evt.Data)
    return nil
})
```

### 取消订阅

```go
// 取消特定订阅
bus.Unsubscribe(subID)

// 取消全局订阅
bus.Unsubscribe(globalSubID)
```

### 发布事件

```go
// 异步发布（非阻塞）
err := bus.Publish(eventbus.EventSessionCreated, map[string]interface{}{
    "session_id": "sess-123",
    "name":       "我的会话",
})

// 同步发布（阻塞等待处理完成）
err := bus.PublishSync(ctx, eventbus.EventSessionCreated, map[string]interface{}{
    "session_id": "sess-123",
    "name":       "我的会话",
})
```

### 使用便捷函数

```go
// 发布会话创建事件
bus.PublishSessionCreated("sess-123", "我的会话")

// 发布会话关闭事件
bus.PublishSessionClosed("sess-123")

// 发布会话切换事件
bus.PublishSessionSwitched("sess-old", "sess-new")

// 发布消息添加事件
bus.PublishMessageAdded("sess-123", "user", "Hello, Kore!")

// 发布消息流式事件
bus.PublishMessageStreaming("sess-123", "流式内容...")

// 发布 Agent 思考事件
bus.PublishAgentThinking("sess-123")

// 发布 Agent 空闲事件
bus.PublishAgentIdle("sess-123")

// 发布工具开始事件
bus.PublishToolStart("sess-123", "read_file", map[string]interface{}{
    "path": "/path/to/file",
})

// 发布工具输出事件
bus.PublishToolOutput("sess-123", "read_file", "文件内容...")

// 发布工具完成事件
bus.PublishToolComplete("sess-123", "read_file")

// 发布 UI 状态更新事件
bus.PublishUIStatusUpdate("running", "正在处理...")

// 发布 UI 流式内容事件
bus.PublishUIStreamContent("流式内容...")
```

---

## 背压处理

### 队列配置

```go
// 配置队列大小
config := &eventbus.Config{
    QueueSize: 1000, // 最多缓存 1000 个事件
}
bus := eventbus.NewEventBus(config)
```

### 背压检测

```go
// 发布事件时检测背压
err := bus.Publish(eventbus.EventSessionCreated, data)
if err != nil {
    if err.Error() == "event queue is full (backpressure)" {
        // 处理背压
        fmt.Println("警告: 事件队列已满，丢弃事件")
    }
}
```

### 超时处理

```go
// 配置事件处理超时
config := &eventbus.Config{
    EventTimeout: 5 * time.Second, // 5 秒超时
}
bus := eventbus.NewEventBus(config)

// 事件处理器内部处理超时
bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
    // 检查上下文是否超时
    select {
    case <-ctx.Done():
        return ctx.Err() // 超时
    default:
        // 处理事件
        return nil
    }
})
```

### 缓冲区配置

```go
// 为订阅配置缓冲区大小
subID := bus.Subscribe(eventbus.EventToolOutput, func(ctx context.Context, evt eventbus.Event) error {
    // 处理事件
    return nil
})

// 注意: 当前版本使用默认缓冲区，未来版本支持自定义缓冲区
```

---

## 使用示例

### 示例 1: 基本发布订阅

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/yukin/kore/internal/eventbus"
)

func main() {
    // 创建事件总线
    bus := eventbus.NewEventBus(nil)
    defer bus.Close()

    // 订阅会话创建事件
    subID := bus.Subscribe(eventbus.EventSessionCreated, func(ctx context.Context, evt eventbus.Event) error {
        sessionID := evt.Data["session_id"].(string)
        name := evt.Data["name"].(string)
        fmt.Printf("会话创建: %s (%s)\n", name, sessionID)
        return nil
    })

    // 发布事件
    bus.PublishSessionCreated("sess-123", "我的会话")

    // 等待事件处理
    time.Sleep(100 * time.Millisecond)

    // 取消订阅
    bus.Unsubscribe(subID)
}
```

### 示例 2: 全局事件监听

```go
package main

import (
    "context"
    "fmt"

    "github.com/yukin/kore/internal/eventbus"
)

func main() {
    bus := eventbus.NewEventBus(nil)
    defer bus.Close()

    // 全局订阅（监听所有事件）
    subID := bus.SubscribeGlobal(func(ctx context.Context, evt eventbus.Event) error {
        fmt.Printf("[%s] %v\n", evt.Type, evt.Data)
        return nil
    })

    // 发布多个事件
    bus.PublishSessionCreated("sess-1", "会话 1")
    bus.PublishMessageAdded("sess-1", "user", "Hello")
    bus.PublishAgentThinking("sess-1")

    bus.Unsubscribe(subID)
}
```

### 示例 3: 事件链处理

```go
package main

import (
    "context"
    "fmt"
    "sync"

    "github.com/yukin/kore/internal/eventbus"
)

func main() {
    bus := eventbus.NewEventBus(nil)
    defer bus.Close()

    var wg sync.WaitGroup

    // 监听工具开始事件
    bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
        wg.Add(1)
        defer wg.Done()

        toolName := evt.Data["tool"].(string)
        fmt.Printf("工具开始: %s\n", toolName)

        // 模拟工具执行
        time.Sleep(100 * time.Millisecond)

        // 发布工具完成事件
        bus.PublishToolComplete(evt.Data["session_id"].(string), toolName)
        return nil
    })

    // 监听工具完成事件
    bus.Subscribe(eventbus.EventToolComplete, func(ctx context.Context, evt eventbus.Event) error {
        wg.Add(1)
        defer wg.Done()

        toolName := evt.Data["tool"].(string)
        fmt.Printf("工具完成: %s\n", toolName)
        return nil
    })

    // 触发工具执行
    bus.PublishToolStart("sess-123", "read_file", map[string]interface{}{
        "path": "/path/to/file",
    })

    wg.Wait()
}
```

### 示例 4: 错误处理和超时

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/yukin/kore/internal/eventbus"
)

func main() {
    config := &eventbus.Config{
        EventTimeout: 2 * time.Second,
    }
    bus := eventbus.NewEventBus(config)
    defer bus.Close()

    // 订阅事件（模拟长时间处理）
    bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
        fmt.Println("开始处理...")

        // 模拟长时间处理
        select {
        case <-time.After(5 * time.Second):
            fmt.Println("处理完成")
            return nil
        case <-ctx.Done():
            fmt.Println("处理超时!")
            return ctx.Err()
        }
    })

    // 发布事件
    bus.PublishToolStart("sess-123", "slow_tool", nil)

    // 等待超时
    time.Sleep(3 * time.Second)
}
```

### 示例 5: 与 Session Manager 集成

```go
package main

import (
    "context"
    "fmt"

    "github.com/yukin/kore/internal/eventbus"
    "github.com/yukin/kore/internal/session"
)

func main() {
    // 创建事件总线
    bus := eventbus.NewEventBus(nil)
    defer bus.Close()

    // 创建会话管理器
    manager := createSessionManager(bus)

    // 监听会话事件
    bus.Subscribe(eventbus.EventSessionCreated, func(ctx context.Context, evt eventbus.Event) error {
        sessionID := evt.Data["session_id"].(string)
        name := evt.Data["name"].(string)
        fmt.Printf("新会话创建: %s (%s)\n", name, sessionID)
        return nil
    })

    bus.Subscribe(eventbus.EventMessageAdded, func(ctx context.Context, evt eventbus.Event) error {
        sessionID := evt.Data["session_id"].(string)
        role := evt.Data["role"].(string)
        content := evt.Data["content"].(string)
        fmt.Printf("[%s] %s: %s\n", sessionID, role, content)
        return nil
    })

    // 创建会话（会触发 SessionCreated 事件）
    ctx := context.Background()
    sess, _ := manager.CreateSession(ctx, "我的会话", session.AgentModeBuild)

    // 添加消息（会触发 MessageAdded 事件）
    manager.AddMessage(sess.ID, session.Message{
        Role:      "user",
        Content:   "Hello, Kore!",
        Timestamp: time.Now().Unix(),
    })
}
```

---

## 最佳实践

### 1. 总是检查错误

```go
// 好的做法
err := bus.Publish(eventbus.EventSessionCreated, data)
if err != nil {
    log.Printf("发布事件失败: %v", err)
    // 处理错误
}

// 避免忽略错误
bus.Publish(eventbus.EventSessionCreated, data) // 不推荐
```

### 2. 及时取消订阅

```go
// 使用 defer 确保取消订阅
subID := bus.Subscribe(eventbus.EventToolStart, handler)
defer bus.Unsubscribe(subID)

// 使用订阅
// ...
```

### 3. 处理器应该是幂等的

```go
// 好的做法：幂等处理器
bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
    toolName := evt.Data["tool"].(string)
    // 检查是否已处理
    if isProcessed(toolName) {
        return nil
    }
    // 处理事件
    return processTool(toolName)
})

// 避免：非幂等处理器
bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
    // 直接处理，可能导致重复执行
    return processTool(evt.Data["tool"].(string))
})
```

### 4. 避免在处理器中阻塞

```go
// 好的做法：异步处理
bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
    go func() {
        // 长时间运行的任务
        longRunningTask()
    }()
    return nil
})

// 避免：同步阻塞
bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
    // 阻塞操作，会延迟其他事件处理
    return longRunningTask()
})
```

### 5. 使用上下文超时

```go
// 配置超时
config := &eventbus.Config{
    EventTimeout: 5 * time.Second,
}
bus := eventbus.NewEventBus(config)

// 处理器内部检查上下文
bus.Subscribe(eventbus.EventToolStart, func(ctx context.Context, evt eventbus.Event) error {
    // 定期检查上下文
    for i := 0; i < 100; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // 处理任务
            time.Sleep(100 * time.Millisecond)
        }
    }
    return nil
})
```

### 6. 优雅关闭

```go
// 使用 defer 确保关闭
bus := eventbus.NewEventBus(nil)
defer bus.Close()

// 使用事件总线
// ...
```

### 7. 事件数据应该是不可变的

```go
// 好的做法：复制数据
bus.Subscribe(eventbus.EventSessionCreated, func(ctx context.Context, evt eventbus.Event) error {
    sessionID := evt.Data["session_id"].(string)
    // 使用数据，不修改
    fmt.Printf("会话: %s\n", sessionID)
    return nil
})

// 避免：修改事件数据
bus.Subscribe(eventbus.EventSessionCreated, func(ctx context.Context, evt eventbus.Event) error {
    // 修改事件数据（不推荐）
    evt.Data["session_id"] = "new-id"
    return nil
})
```

---

## API 参考

### EventBus 接口

```go
type EventBus struct{}

func NewEventBus(config *Config) *EventBus
func (bus *EventBus) Subscribe(eventType EventType, handler EventHandler) string
func (bus *EventBus) SubscribeGlobal(handler EventHandler) string
func (bus *EventBus) Unsubscribe(subID string)
func (bus *EventBus) Publish(eventType EventType, data map[string]interface{}) error
func (bus *EventBus) PublishSync(ctx context.Context, eventType EventType, data map[string]interface{}) error
func (bus *EventBus) Close() error

// 便捷函数
func (bus *EventBus) PublishSessionCreated(sessionID, name string) error
func (bus *EventBus) PublishSessionClosed(sessionID string) error
func (bus *EventBus) PublishSessionSwitched(fromID, toID string) error
func (bus *EventBus) PublishMessageAdded(sessionID, role, content string) error
func (bus *EventBus) PublishMessageStreaming(sessionID, content string) error
func (bus *EventBus) PublishAgentThinking(sessionID string) error
func (bus *EventBus) PublishAgentIdle(sessionID string) error
func (bus *EventBus) PublishToolStart(sessionID, toolName string, args map[string]interface{}) error
func (bus *EventBus) PublishToolOutput(sessionID, toolName, output string) error
func (bus *EventBus) PublishToolComplete(sessionID, toolName string) error
func (bus *EventBus) PublishUIStatusUpdate(status, message string) error
func (bus *EventBus) PublishUIStreamContent(content string) error
```

---

## 相关文档

- [Session Management 使用指南](./session-management.md)
- [LSP Manager 使用指南](./lsp-manager.md)
- [gRPC API 使用指南](./grpc-api.md)
- [Phase 3-6 实施总结](../phase3-6-summary.md)

---

**文档版本**: 1.0
**最后更新**: 2026-01-18
**维护者**: Kore Team
