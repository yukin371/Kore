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

	// LLM 事件
	EventLLMTokenStart    EventType = "llm.token_start"
	EventLLMTokenDelta    EventType = "llm.token_delta"
	EventLLMTokenComplete EventType = "llm.token_complete"
	EventLLMRequestStart  EventType = "llm.request_start"
	EventLLMRequestComplete EventType = "llm.request_complete"
	EventLLMError         EventType = "llm.error"

	// UI 事件
	EventUIStatusUpdate   EventType = "ui.status_update"
	EventUIStreamContent  EventType = "ui.stream_content"
)

// EventPriority 事件优先级
type EventPriority int

const (
	// PriorityLow 低优先级
	PriorityLow EventPriority = iota
	// PriorityNormal 正常优先级
	PriorityNormal
	// PriorityHigh 高优先级
	PriorityHigh
	// PriorityCritical 紧急优先级
	PriorityCritical
)

// Event 事件接口
type Event interface {
	// GetType 获取事件类型
	GetType() EventType

	// GetTimestamp 获取时间戳
	GetTimestamp() int64

	// GetData 获取事件数据
	GetData() map[string]interface{}

	// GetPriority 获取优先级
	GetPriority() EventPriority

	// GetMetadata 获取元数据
	GetMetadata() map[string]interface{}
}

// BaseEvent 基础事件实现
type BaseEvent struct {
	Type      EventType              `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Priority  EventPriority          `json:"priority"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// GetType 实现Event接口
func (e *BaseEvent) GetType() EventType {
	return e.Type
}

// GetTimestamp 实现Event接口
func (e *BaseEvent) GetTimestamp() int64 {
	return e.Timestamp
}

// GetData 实现Event接口
func (e *BaseEvent) GetData() map[string]interface{} {
	return e.Data
}

// GetPriority 实现Event接口
func (e *BaseEvent) GetPriority() EventPriority {
	return e.Priority
}

// GetMetadata 实现Event接口
func (e *BaseEvent) GetMetadata() map[string]interface{} {
	return e.Metadata
}

// NewEvent 创建新事件
func NewEvent(eventType EventType, data map[string]interface{}) Event {
	return &BaseEvent{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
		Priority:  PriorityNormal,
		Metadata:  make(map[string]interface{}),
	}
}

// NewEventWithPriority 创建带优先级的事件
func NewEventWithPriority(eventType EventType, data map[string]interface{}, priority EventPriority) Event {
	return &BaseEvent{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
		Priority:  priority,
		Metadata:  make(map[string]interface{}),
	}
}

// NewEventWithMetadata 创建带元数据的事件
func NewEventWithMetadata(eventType EventType, data map[string]interface{}, metadata map[string]interface{}) Event {
	return &BaseEvent{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
		Priority:  PriorityNormal,
		Metadata:  metadata,
	}
}

// EventHandler 事件处理器函数
type EventHandler func(ctx context.Context, event Event) error

// EventFilter 事件过滤器函数
type EventFilter func(event Event) bool

// Subscription 事件订阅
type Subscription struct {
	ID        string
	EventType EventType
	Handler   EventHandler
	Filters   []EventFilter // 过滤器链
	Priority  int           // 订阅优先级（越大越优先）
	Buffer    int           // 缓冲区大小
	Once      bool          // 是否只触发一次
}

// EventBus 事件总线
type EventBus struct {
	// 订阅者（按事件类型分组）
	subscribers map[EventType][]*Subscription

	// 全局订阅者（监听所有事件）
	globalSubscribers []*Subscription

	// 事件队列（支持优先级）
	eventQueue chan *priorityEvent

	// 互斥锁
	mu sync.RWMutex

	// 配置
	config *Config

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 等待组
	wg sync.WaitGroup

	// 统计信息
	stats *Stats

	// 中间件
	middlewares []MiddlewareFunc
}

// priorityEvent 带优先级的事件
type priorityEvent struct {
	event    Event
	priority EventPriority
}

// Stats 事件总线统计信息
type Stats struct {
	EventsPublished   int64
	EventsProcessed   int64
	EventsFailed      int64
	SubscribersCount  int
	LastError         string
	LastErrorTime     int64
	mu                sync.RWMutex
}

// Config 事件总线配置
type Config struct {
	// 队列大小（0 = 无缓冲）
	QueueSize int

	// 默认缓冲区大小
	DefaultBuffer int

	// 事件超时（防止阻塞）
	EventTimeout time.Duration

	// 是否启用统计
	EnableStats bool

	// 最大重试次数
	MaxRetries int

	// 重试延迟
	RetryDelay time.Duration
}

// MiddlewareFunc 中间件函数
type MiddlewareFunc func(next EventHandler) EventHandler

// NewEventBus 创建事件总线
func NewEventBus(config *Config) *EventBus {
	if config == nil {
		config = &Config{
			QueueSize:      1000,
			DefaultBuffer:  100,
			EventTimeout:   5 * time.Second,
			EnableStats:    true,
			MaxRetries:     3,
			RetryDelay:     100 * time.Millisecond,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	bus := &EventBus{
		subscribers:       make(map[EventType][]*Subscription),
		globalSubscribers: make([]*Subscription, 0),
		eventQueue:        make(chan *priorityEvent, config.QueueSize),
		config:            config,
		ctx:               ctx,
		cancel:            cancel,
		stats:             &Stats{},
		middlewares:       make([]MiddlewareFunc, 0),
	}

	// 启动事件分发循环
	bus.wg.Add(1)
	go bus.dispatchLoop()

	return bus
}

// Subscribe 订阅事件
func (bus *EventBus) Subscribe(eventType EventType, handler EventHandler) string {
	return bus.SubscribeWithOptions(eventType, handler, nil)
}

// SubscribeWithOptions 使用选项订阅事件
func (bus *EventBus) SubscribeWithOptions(
	eventType EventType,
	handler EventHandler,
	options *SubscriptionOptions,
) string {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	// 生成订阅 ID
	subID := generateID()

	if options == nil {
		options = &SubscriptionOptions{
			Priority: 0,
			Buffer:   bus.config.DefaultBuffer,
			Once:     false,
		}
	}

	sub := &Subscription{
		ID:        subID,
		EventType: eventType,
		Handler:   handler,
		Filters:   options.Filters,
		Priority:  options.Priority,
		Buffer:    options.Buffer,
		Once:      options.Once,
	}

	// 添加到订阅者列表并按优先级排序
	bus.subscribers[eventType] = append(bus.subscribers[eventType], sub)
	bus.sortSubscribers(bus.subscribers[eventType])

	// 更新统计
	if bus.config.EnableStats {
		bus.stats.SubscribersCount++
	}

	return subID
}

// SubscribeGlobal 全局订阅（监听所有事件）
func (bus *EventBus) SubscribeGlobal(handler EventHandler) string {
	return bus.SubscribeGlobalWithOptions(handler, nil)
}

// SubscribeGlobalWithOptions 使用选项全局订阅
func (bus *EventBus) SubscribeGlobalWithOptions(
	handler EventHandler,
	options *SubscriptionOptions,
) string {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	// 生成订阅 ID
	subID := generateID()

	if options == nil {
		options = &SubscriptionOptions{
			Priority: 0,
			Buffer:   bus.config.DefaultBuffer,
			Once:     false,
		}
	}

	sub := &Subscription{
		ID:        subID,
		EventType: "", // 空字符串表示全局订阅
		Handler:   handler,
		Filters:   options.Filters,
		Priority:  options.Priority,
		Buffer:    options.Buffer,
		Once:      options.Once,
	}

	bus.globalSubscribers = append(bus.globalSubscribers, sub)
	bus.sortSubscribers(bus.globalSubscribers)

	// 更新统计
	if bus.config.EnableStats {
		bus.stats.SubscribersCount++
	}

	return subID
}

// SubscribeWithFilter 带过滤器的订阅
func (bus *EventBus) SubscribeWithFilter(
	eventType EventType,
	handler EventHandler,
	filter EventFilter,
) string {
	return bus.SubscribeWithOptions(eventType, handler, &SubscriptionOptions{
		Filters: []EventFilter{filter},
	})
}

// SubscriptionOptions 订阅选项
type SubscriptionOptions struct {
	Priority int            // 订阅优先级
	Buffer   int            // 缓冲区大小
	Once     bool           // 是否只触发一次
	Filters  []EventFilter  // 过滤器链
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

				// 更新统计
				if bus.config.EnableStats {
					bus.stats.SubscribersCount--
				}
				return
			}
		}
	}

	// 从全局订阅者中移除
	for i, sub := range bus.globalSubscribers {
		if sub.ID == subID {
			bus.globalSubscribers = append(bus.globalSubscribers[:i], bus.globalSubscribers[i+1:]...)

			// 更新统计
			if bus.config.EnableStats {
				bus.stats.SubscribersCount--
			}
			return
		}
	}
}

// Publish 发布事件（异步）
func (bus *EventBus) Publish(eventType EventType, data map[string]interface{}) error {
	event := NewEvent(eventType, data)
	return bus.PublishEvent(event)
}

// PublishEvent 发布事件对象（异步）
func (bus *EventBus) PublishEvent(event Event) error {
	// 更新统计
	if bus.config.EnableStats {
		bus.stats.mu.Lock()
		bus.stats.EventsPublished++
		bus.stats.mu.Unlock()
	}

	priorityEvent := &priorityEvent{
		event:    event,
		priority: event.GetPriority(),
	}

	select {
	case bus.eventQueue <- priorityEvent:
		return nil
	default:
		// 队列已满（背压处理）
		return fmt.Errorf("event queue is full (backpressure)")
	}
}

// PublishSync 发布事件（同步等待）
func (bus *EventBus) PublishSync(ctx context.Context, eventType EventType, data map[string]interface{}) error {
	event := NewEvent(eventType, data)
	return bus.PublishEventSync(ctx, event)
}

// PublishEventSync 发布事件对象（同步等待）
func (bus *EventBus) PublishEventSync(ctx context.Context, event Event) error {
	// 更新统计
	if bus.config.EnableStats {
		bus.stats.mu.Lock()
		bus.stats.EventsPublished++
		bus.stats.mu.Unlock()
	}

	return bus.dispatchEvent(ctx, event)
}

// Use 添加中间件
func (bus *EventBus) Use(middleware MiddlewareFunc) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.middlewares = append(bus.middlewares, middleware)
}

// dispatchLoop 事件分发循环
func (bus *EventBus) dispatchLoop() {
	defer bus.wg.Done()

	for {
		select {
		case <-bus.ctx.Done():
			// 上下文已取消，退出循环
			return

		case priorityEvent := <-bus.eventQueue:
			// 分发事件
			ctx, cancel := context.WithTimeout(context.Background(), bus.config.EventTimeout)
			bus.dispatchEvent(ctx, priorityEvent.event)
			cancel()
		}
	}
}

// dispatchEvent 分发事件到订阅者
func (bus *EventBus) dispatchEvent(ctx context.Context, event Event) error {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	// 更新统计
	if bus.config.EnableStats {
		bus.stats.mu.Lock()
		bus.stats.EventsProcessed++
		bus.stats.mu.Unlock()
	}

	// 1. 通知全局订阅者
	for _, sub := range bus.globalSubscribers {
		if sub.Handler != nil && bus.checkFilters(sub, event) {
			// 异步调用（防止阻塞）
			go bus.handleEventWithRetry(ctx, sub, event)

			// 如果是一次性订阅，取消订阅
			if sub.Once {
				go bus.Unsubscribe(sub.ID)
			}
		}
	}

	// 2. 通知特定类型的订阅者
	subs, ok := bus.subscribers[event.GetType()]
	if !ok {
		return nil // 没有订阅者
	}

	for _, sub := range subs {
		if sub.Handler != nil && bus.checkFilters(sub, event) {
			// 异步调用（防止阻塞）
			go bus.handleEventWithRetry(ctx, sub, event)

			// 如果是一次性订阅，取消订阅
			if sub.Once {
				go bus.Unsubscribe(sub.ID)
			}
		}
	}

	return nil
}

// handleEventWithRetry 带重试的事件处理
func (bus *EventBus) handleEventWithRetry(ctx context.Context, sub *Subscription, event Event) {
	var lastErr error

	for attempt := 0; attempt <= bus.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 重试延迟
			select {
			case <-time.After(bus.config.RetryDelay * time.Duration(attempt)):
			case <-ctx.Done():
				return
			}
		}

		// 调用处理器
		if err := bus.handleEvent(ctx, sub, event); err != nil {
			lastErr = err

			// 检查是否应该重试
			if attempt < bus.config.MaxRetries {
				continue
			}

			// 记录错误
			if bus.config.EnableStats {
				bus.stats.mu.Lock()
				bus.stats.EventsFailed++
				bus.stats.LastError = err.Error()
				bus.stats.LastErrorTime = time.Now().Unix()
				bus.stats.mu.Unlock()
			}
		} else {
			// 成功，退出重试循环
			return
		}
	}

	// 所有重试都失败
	if lastErr != nil {
		fmt.Printf("Event handler failed after %d attempts: %v\n", bus.config.MaxRetries, lastErr)
	}
}

// handleEvent 处理单个事件
func (bus *EventBus) handleEvent(ctx context.Context, sub *Subscription, event Event) error {
	defer func() {
		// 捕获 panic
		if r := recover(); r != nil {
			// TODO: 日志记录
			fmt.Printf("Event handler panic: %v\n", r)
		}
	}()

	// 应用中间件链
	handler := sub.Handler
	for i := len(bus.middlewares) - 1; i >= 0; i-- {
		handler = bus.middlewares[i](handler)
	}

	// 调用处理器
	return handler(ctx, event)
}

// checkFilters 检查事件是否通过所有过滤器
func (bus *EventBus) checkFilters(sub *Subscription, event Event) bool {
	// 如果没有过滤器，通过
	if len(sub.Filters) == 0 {
		return true
	}

	// 检查所有过滤器
	for _, filter := range sub.Filters {
		if !filter(event) {
			return false
		}
	}

	return true
}

// sortSubscribers 按优先级排序订阅者（降序）
func (bus *EventBus) sortSubscribers(subs []*Subscription) {
	// 简单的冒泡排序（订阅者数量通常不大）
	n := len(subs)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if subs[j].Priority < subs[j+1].Priority {
				subs[j], subs[j+1] = subs[j+1], subs[j]
			}
		}
	}
}

// GetStats 获取统计信息
func (bus *EventBus) GetStats() *Stats {
	if !bus.config.EnableStats {
		return nil
	}

	bus.stats.mu.RLock()
	defer bus.stats.mu.RUnlock()

	// 返回副本
	return &Stats{
		EventsPublished:  bus.stats.EventsPublished,
		EventsProcessed:  bus.stats.EventsProcessed,
		EventsFailed:     bus.stats.EventsFailed,
		SubscribersCount: bus.stats.SubscribersCount,
		LastError:        bus.stats.LastError,
		LastErrorTime:    bus.stats.LastErrorTime,
	}
}

// ResetStats 重置统计信息
func (bus *EventBus) ResetStats() {
	if !bus.config.EnableStats {
		return
	}

	bus.stats.mu.Lock()
	defer bus.stats.mu.Unlock()

	bus.stats.EventsPublished = 0
	bus.stats.EventsProcessed = 0
	bus.stats.EventsFailed = 0
	bus.stats.LastError = ""
	bus.stats.LastErrorTime = 0
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

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// NewLLMTokenStartEvent 创建 LLM token 开始事件
func NewLLMTokenStartEvent(requestID, model string, maxTokens int) Event {
	return &BaseEvent{
		Type:      EventLLMTokenStart,
		Timestamp: time.Now().Unix(),
		Priority:  PriorityNormal,
		Data: map[string]interface{}{
			"request_id":  requestID,
			"model":       model,
			"max_tokens":  maxTokens,
		},
	}
}

// NewLLMTokenDeltaEvent 创建 LLM token 增量事件
func NewLLMTokenDeltaEvent(requestID string, token string, tokenCount int) Event {
	return &BaseEvent{
		Type:      EventLLMTokenDelta,
		Timestamp: time.Now().Unix(),
		Priority:  PriorityLow, // 高频事件，低优先级
		Data: map[string]interface{}{
			"request_id":   requestID,
			"token":        token,
			"token_count":  tokenCount,
		},
	}
}

// NewLLMTokenCompleteEvent 创建 LLM token 完成事件
func NewLLMTokenCompleteEvent(requestID string, totalTokens int, duration int64) Event {
	return &BaseEvent{
		Type:      EventLLMTokenComplete,
		Timestamp: time.Now().Unix(),
		Priority:  PriorityNormal,
		Data: map[string]interface{}{
			"request_id":    requestID,
			"total_tokens":  totalTokens,
			"duration_ms":   duration,
			"tokens_per_second": float64(totalTokens) / (float64(duration) / 1000),
		},
	}
}

// NewLLMRequestStartEvent 创建 LLM 请求开始事件
func NewLLMRequestStartEvent(requestID, model, provider string) Event {
	return &BaseEvent{
		Type:      EventLLMRequestStart,
		Timestamp: time.Now().Unix(),
		Priority:  PriorityNormal,
		Data: map[string]interface{}{
			"request_id": requestID,
			"model":      model,
			"provider":   provider,
		},
	}
}

// NewLLMRequestCompleteEvent 创建 LLM 请求完成事件
func NewLLMRequestCompleteEvent(requestID string, totalTokens int, duration int64, success bool) Event {
	return &BaseEvent{
		Type:      EventLLMRequestComplete,
		Timestamp: time.Now().Unix(),
		Priority:  PriorityNormal,
		Data: map[string]interface{}{
			"request_id":    requestID,
			"total_tokens":  totalTokens,
			"duration_ms":   duration,
			"success":       success,
		},
	}
}

// NewLLMErrorEvent 创建 LLM 错误事件
func NewLLMErrorEvent(requestID string, err error) Event {
	return &BaseEvent{
		Type:      EventLLMError,
		Timestamp: time.Now().Unix(),
		Priority:  PriorityHigh, // 错误事件高优先级
		Data: map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		},
	}
}
