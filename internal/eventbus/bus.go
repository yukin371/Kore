package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType 事件类型
type EventType string

const (
	// 会话事件
	EventSessionCreated   EventType = "session.created"
	EventSessionClosed    EventType = "session.closed"
	EventSessionSwitched  EventType = "session.switched"
	EventSessionUpdated   EventType = "session.updated"

	// 消息事件
	EventMessageAdded     EventType = "message.added"
	EventMessageStreaming EventType = "message.streaming"

	// Agent 状态事件
	EventAgentThinking    EventType = "agent.thinking"
	EventAgentIdle        EventType = "agent.idle"
	EventAgentError       EventType = "agent.error"

	// 工具执行事件
	EventToolStart        EventType = "tool.start"
	EventToolOutput       EventType = "tool.output"
	EventToolComplete     EventType = "tool.complete"
	EventToolError        EventType = "tool.error"

	// UI 事件
	EventUIStatusUpdate   EventType = "ui.status_update"
	EventUIStreamContent  EventType = "ui.stream_content"
)

// Event 事件
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// EventHandler 事件处理器函数
type EventHandler func(ctx context.Context, event Event) error

// Subscription 事件订阅
type Subscription struct {
	ID        string
	EventType EventType
	Handler   EventHandler
	Buffer    int // 缓冲区大小
}

// EventBus 事件总线
type EventBus struct {
	// 订阅者（按事件类型分组）
	subscribers map[EventType][]*Subscription

	// 全局订阅者（监听所有事件）
	globalSubscribers []*Subscription

	// 事件队列（用于背压处理）
	eventQueue chan Event

	// 互斥锁
	mu sync.RWMutex

	// 配置
	config *Config

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 等待组
	wg sync.WaitGroup
}

// Config 事件总线配置
type Config struct {
	// 队列大小（0 = 无缓冲）
	QueueSize int

	// 默认缓冲区大小
	DefaultBuffer int

	// 事件超时（防止阻塞）
	EventTimeout time.Duration
}

// NewEventBus 创建事件总线
func NewEventBus(config *Config) *EventBus {
	if config == nil {
		config = &Config{
			QueueSize:      1000,
			DefaultBuffer:  100,
			EventTimeout:   5 * time.Second,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	bus := &EventBus{
		subscribers:       make(map[EventType][]*Subscription),
		globalSubscribers: make([]*Subscription, 0),
		eventQueue:        make(chan Event, config.QueueSize),
		config:            config,
		ctx:               ctx,
		cancel:            cancel,
	}

	// 启动事件分发循环
	bus.wg.Add(1)
	go bus.dispatchLoop()

	return bus
}

// Subscribe 订阅事件
func (bus *EventBus) Subscribe(eventType EventType, handler EventHandler) string {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	// 生成订阅 ID
	subID := generateID()

	sub := &Subscription{
		ID:        subID,
		EventType: eventType,
		Handler:   handler,
		Buffer:    bus.config.DefaultBuffer,
	}

	// 添加到订阅者列表
	bus.subscribers[eventType] = append(bus.subscribers[eventType], sub)

	return subID
}

// SubscribeGlobal 全局订阅（监听所有事件）
func (bus *EventBus) SubscribeGlobal(handler EventHandler) string {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	// 生成订阅 ID
	subID := generateID()

	sub := &Subscription{
		ID:        subID,
		EventType: "", // 空字符串表示全局订阅
		Handler:   handler,
		Buffer:    bus.config.DefaultBuffer,
	}

	bus.globalSubscribers = append(bus.globalSubscribers, sub)

	return subID
}

// Unsubscribe 取消订阅
func (bus *EventBus) Unsubscribe(subID string) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	// 从普通订阅者中移除
	for eventType, subs := range bus.subscribers {
		for i, sub := range subs {
			if sub.ID == subID {
				// 删除订阅
				bus.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}

	// 从全局订阅者中移除
	for i, sub := range bus.globalSubscribers {
		if sub.ID == subID {
			bus.globalSubscribers = append(bus.globalSubscribers[:i], bus.globalSubscribers[i+1:]...)
			return
		}
	}
}

// Publish 发布事件（异步）
func (bus *EventBus) Publish(eventType EventType, data map[string]interface{}) error {
	event := Event{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	select {
	case bus.eventQueue <- event:
		return nil
	default:
		// 队列已满（背压处理）
		return fmt.Errorf("event queue is full (backpressure)")
	}
}

// PublishSync 发布事件（同步等待）
func (bus *EventBus) PublishSync(ctx context.Context, eventType EventType, data map[string]interface{}) error {
	event := Event{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	return bus.dispatchEvent(ctx, event)
}

// dispatchLoop 事件分发循环
func (bus *EventBus) dispatchLoop() {
	defer bus.wg.Done()

	for {
		select {
		case <-bus.ctx.Done():
			// 上下文已取消，退出循环
			return

		case event := <-bus.eventQueue:
			// 分发事件
			ctx, cancel := context.WithTimeout(context.Background(), bus.config.EventTimeout)
			bus.dispatchEvent(ctx, event)
			cancel()
		}
	}
}

// dispatchEvent 分发事件到订阅者
func (bus *EventBus) dispatchEvent(ctx context.Context, event Event) error {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	// 1. 通知全局订阅者
	for _, sub := range bus.globalSubscribers {
		if sub.Handler != nil {
			// 异步调用（防止阻塞）
			go bus.handleEvent(ctx, sub, event)
		}
	}

	// 2. 通知特定类型的订阅者
	subs, ok := bus.subscribers[event.Type]
	if !ok {
		return nil // 没有订阅者
	}

	for _, sub := range subs {
		if sub.Handler != nil {
			// 异步调用（防止阻塞）
			go bus.handleEvent(ctx, sub, event)
		}
	}

	return nil
}

// handleEvent 处理单个事件
func (bus *EventBus) handleEvent(ctx context.Context, sub *Subscription, event Event) {
	defer func() {
		// 捕获 panic
		if r := recover(); r != nil {
			// TODO: 日志记录
			fmt.Printf("Event handler panic: %v\n", r)
		}
	}()

	// 调用处理器
	if err := sub.Handler(ctx, event); err != nil {
		// TODO: 日志记录
		fmt.Printf("Event handler error: %v\n", err)
	}
}

// Close 关闭事件总线
func (bus *EventBus) Close() error {
	// 取消上下文
	bus.cancel()

	// 等待分发循环结束
	bus.wg.Wait()

	// 关闭队列
	close(bus.eventQueue)

	return nil
}

// 辅助函数
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ========== 便捷函数 ==========

// PublishSessionCreated 发布会话创建事件
func (bus *EventBus) PublishSessionCreated(sessionID, name string) error {
	return bus.Publish(EventSessionCreated, map[string]interface{}{
		"session_id": sessionID,
		"name":       name,
	})
}

// PublishSessionClosed 发布会话关闭事件
func (bus *EventBus) PublishSessionClosed(sessionID string) error {
	return bus.Publish(EventSessionClosed, map[string]interface{}{
		"session_id": sessionID,
	})
}

// PublishSessionSwitched 发布会话切换事件
func (bus *EventBus) PublishSessionSwitched(fromID, toID string) error {
	return bus.Publish(EventSessionSwitched, map[string]interface{}{
		"from_session_id": fromID,
		"to_session_id":   toID,
	})
}

// PublishMessageAdded 发布消息添加事件
func (bus *EventBus) PublishMessageAdded(sessionID, role, content string) error {
	return bus.Publish(EventMessageAdded, map[string]interface{}{
		"session_id": sessionID,
		"role":       role,
		"content":    content,
	})
}

// PublishMessageStreaming 发布消息流式事件
func (bus *EventBus) PublishMessageStreaming(sessionID, content string) error {
	return bus.Publish(EventMessageStreaming, map[string]interface{}{
		"session_id": sessionID,
		"content":    content,
	})
}

// PublishAgentThinking 发布 Agent 思考事件
func (bus *EventBus) PublishAgentThinking(sessionID string) error {
	return bus.Publish(EventAgentThinking, map[string]interface{}{
		"session_id": sessionID,
	})
}

// PublishAgentIdle 发布 Agent 空闲事件
func (bus *EventBus) PublishAgentIdle(sessionID string) error {
	return bus.Publish(EventAgentIdle, map[string]interface{}{
		"session_id": sessionID,
	})
}

// PublishToolStart 发布工具开始事件
func (bus *EventBus) PublishToolStart(sessionID, toolName string, args map[string]interface{}) error {
	return bus.Publish(EventToolStart, map[string]interface{}{
		"session_id": sessionID,
		"tool":       toolName,
		"args":       args,
	})
}

// PublishToolOutput 发布工具输出事件
func (bus *EventBus) PublishToolOutput(sessionID, toolName, output string) error {
	return bus.Publish(EventToolOutput, map[string]interface{}{
		"session_id": sessionID,
		"tool":       toolName,
		"output":     output,
	})
}

// PublishToolComplete 发布工具完成事件
func (bus *EventBus) PublishToolComplete(sessionID, toolName string) error {
	return bus.Publish(EventToolComplete, map[string]interface{}{
		"session_id": sessionID,
		"tool":       toolName,
	})
}

// PublishUIStatusUpdate 发布 UI 状态更新事件
func (bus *EventBus) PublishUIStatusUpdate(status, message string) error {
	return bus.Publish(EventUIStatusUpdate, map[string]interface{}{
		"status":  status,
		"message": message,
	})
}

// PublishUIStreamContent 发布 UI 流式内容事件
func (bus *EventBus) PublishUIStreamContent(content string) error {
	return bus.Publish(EventUIStreamContent, map[string]interface{}{
		"content": content,
	})
}
