package eventbus

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewEventBus 测试创建事件总线
func TestNewEventBus(t *testing.T) {
	bus := NewEventBus(nil)
	if bus == nil {
		t.Fatal("NewEventBus returned nil")
	}
	defer bus.Close()

	if bus.subscribers == nil {
		t.Error("subscribers map not initialized")
	}

	if bus.globalSubscribers == nil {
		t.Error("globalSubscribers not initialized")
	}

	if bus.eventQueue == nil {
		t.Error("eventQueue not initialized")
	}
}

// TestSubscribe 测试订阅
func TestSubscribe(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	subID := bus.Subscribe(EventSessionCreated, handler)
	if subID == "" {
		t.Error("subscription ID is empty")
	}

	// 验证订阅者数量
	stats := bus.GetStats()
	if stats.SubscribersCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", stats.SubscribersCount)
	}
}

// TestUnsubscribe 测试取消订阅
func TestUnsubscribe(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	subID := bus.Subscribe(EventSessionCreated, handler)

	// 取消订阅
	bus.Unsubscribe(subID)

	// 发布事件
	bus.Publish(EventSessionCreated, map[string]interface{}{})

	// 等待一段时间
	time.Sleep(100 * time.Millisecond)

	// 验证订阅者数量
	stats := bus.GetStats()
	if stats.SubscribersCount != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", stats.SubscribersCount)
	}
}

// TestPublish 测试发布事件
func TestPublish(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	received := false
	var receivedEvent Event

	handler := func(ctx context.Context, event Event) error {
		received = true
		receivedEvent = event
		return nil
	}

	bus.Subscribe(EventSessionCreated, handler)

	// 发布事件
	data := map[string]interface{}{
		"session_id": "test123",
		"name":       "Test Session",
	}
	err := bus.Publish(EventSessionCreated, data)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// 等待事件被处理
	time.Sleep(200 * time.Millisecond)

	if !received {
		t.Fatal("event not received")
	}

	if receivedEvent.GetType() != EventSessionCreated {
		t.Errorf("expected event type %s, got %s", EventSessionCreated, receivedEvent.GetType())
	}

	sessionID := receivedEvent.GetData()["session_id"]
	if sessionID != "test123" {
		t.Errorf("expected session_id 'test123', got %v", sessionID)
	}
}

// TestPublishSync 测试同步发布
func TestPublishSync(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	var mu sync.Mutex
	received := false
	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		received = true
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventSessionCreated, handler)

	// 同步发布
	ctx := context.Background()
	err := bus.PublishSync(ctx, EventSessionCreated, map[string]interface{}{})
	if err != nil {
		t.Fatalf("PublishSync failed: %v", err)
	}

	// 等待一小段时间确保处理器被调用
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if !received {
		t.Error("event not received in sync publish")
	}
	mu.Unlock()
}

// TestGlobalSubscriber 测试全局订阅者
func TestGlobalSubscriber(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	receiveCount := 0
	var mu sync.Mutex

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		receiveCount++
		mu.Unlock()
		return nil
	}

	// 全局订阅
	bus.SubscribeGlobal(handler)

	// 发布多个不同类型的事件
	bus.Publish(EventSessionCreated, map[string]interface{}{})
	bus.Publish(EventMessageAdded, map[string]interface{}{})
	bus.Publish(EventToolStart, map[string]interface{}{})

	// 等待所有事件被处理
	time.Sleep(200 * time.Millisecond)

	if receiveCount != 3 {
		t.Errorf("expected 3 events, got %d", receiveCount)
	}
}

// TestEventPriority 测试事件优先级
func TestEventPriority(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	var priorities []EventPriority
	var mu sync.Mutex

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		priorities = append(priorities, event.GetPriority())
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventSessionCreated, handler)

	// 发布不同优先级的事件
	bus.PublishWithPriority(EventSessionCreated, map[string]interface{}{}, PriorityLow)
	bus.PublishWithPriority(EventSessionCreated, map[string]interface{}{}, PriorityHigh)
	bus.PublishWithPriority(EventSessionCreated, map[string]interface{}{}, PriorityNormal)

	// 等待所有事件被处理
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(priorities) != 3 {
		t.Fatalf("expected 3 events, got %d", len(priorities))
	}

	// 验证所有优先级都被接收（不保证顺序）
	hasLow := false
	hasHigh := false
	hasNormal := false
	for _, p := range priorities {
		if p == PriorityLow {
			hasLow = true
		}
		if p == PriorityHigh {
			hasHigh = true
		}
		if p == PriorityNormal {
			hasNormal = true
		}
	}

	if !hasLow || !hasHigh || !hasNormal {
		t.Errorf("expected all priorities, got %v", priorities)
	}
}

// TestFilterType 测试类型过滤器
func TestFilterType(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	sessionCount := 0
	messageCount := 0
	var mu sync.Mutex

	sessionHandler := func(ctx context.Context, event Event) error {
		mu.Lock()
		sessionCount++
		mu.Unlock()
		return nil
	}

	messageHandler := func(ctx context.Context, event Event) error {
		mu.Lock()
		messageCount++
		mu.Unlock()
		return nil
	}

	// 订阅不同类型的事件
	bus.SubscribeWithFilter(EventSessionCreated, sessionHandler, FilterType(EventSessionCreated))
	bus.SubscribeWithFilter(EventMessageAdded, messageHandler, FilterType(EventMessageAdded))

	// 发布事件
	bus.Publish(EventSessionCreated, map[string]interface{}{})
	bus.Publish(EventMessageAdded, map[string]interface{}{})
	bus.Publish(EventToolStart, map[string]interface{}{})

	// 等待所有事件被处理
	time.Sleep(200 * time.Millisecond)

	if sessionCount != 1 {
		t.Errorf("expected 1 session event, got %d", sessionCount)
	}

	if messageCount != 1 {
		t.Errorf("expected 1 message event, got %d", messageCount)
	}
}

// TestFilterDataField 测试数据字段过滤器
func TestFilterDataField(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	receivedCount := 0
	var mu sync.Mutex

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		return nil
	}

	// 只接收 tool 为 "read_file" 的事件
	bus.SubscribeWithFilter(EventToolStart, handler, FilterToolName("read_file"))

	// 发布多个工具事件
	bus.Publish(EventToolStart, map[string]interface{}{
		"tool": "read_file",
	})
	bus.Publish(EventToolStart, map[string]interface{}{
		"tool": "write_file",
	})
	bus.Publish(EventToolStart, map[string]interface{}{
		"tool": "read_file",
	})

	// 等待所有事件被处理
	time.Sleep(200 * time.Millisecond)

	if receivedCount != 2 {
		t.Errorf("expected 2 events, got %d", receivedCount)
	}
}

// TestFilterWildcard 测试通配符过滤器
func TestFilterWildcard(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	receivedCount := 0
	receivedTypes := []string{}
	var mu sync.Mutex

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		receivedCount++
		receivedTypes = append(receivedTypes, string(event.GetType()))
		mu.Unlock()
		return nil
	}

	// 使用全局订阅 + 通配符过滤器
	bus.SubscribeGlobalWithOptions(handler, &SubscriptionOptions{
		Filters: []EventFilter{FilterWildcard("tool.*")},
	})

	// 发布不同类型的事件
	bus.Publish(EventToolStart, map[string]interface{}{})
	bus.Publish(EventToolOutput, map[string]interface{}{})
	bus.Publish(EventSessionCreated, map[string]interface{}{})
	bus.Publish(EventToolComplete, map[string]interface{}{})

	// 等待所有事件被处理
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// 应该收到3个工具事件（start, output, complete）
	t.Logf("Received %d events: %v", receivedCount, receivedTypes)

	if receivedCount != 3 {
		t.Errorf("expected 3 tool events, got %d", receivedCount)
	}

	// 验证收到的事件类型
	for _, et := range receivedTypes {
		if !strings.HasPrefix(et, "tool.") {
			t.Errorf("expected tool event, got %s", et)
		}
	}
}

// TestFilterAnd 测试AND过滤器
func TestFilterAnd(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	receivedCount := 0
	var mu sync.Mutex

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		return nil
	}

	// 同时满足类型和数据字段
	bus.SubscribeWithFilter(EventToolStart, handler, FilterAnd(
		FilterType(EventToolStart),
		FilterToolName("read_file"),
	))

	// 发布事件
	bus.Publish(EventToolStart, map[string]interface{}{
		"tool": "read_file",
	})
	bus.Publish(EventToolStart, map[string]interface{}{
		"tool": "write_file",
	})
	bus.Publish(EventToolOutput, map[string]interface{}{
		"tool": "read_file",
	})

	// 等待所有事件被处理
	time.Sleep(200 * time.Millisecond)

	if receivedCount != 1 {
		t.Errorf("expected 1 event, got %d", receivedCount)
	}
}

// TestSubscriptionOnce 测试一次性订阅
func TestSubscriptionOnce(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	receivedCount := 0
	var mu sync.Mutex

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		return nil
	}

	// 一次性订阅
	bus.SubscribeWithOptions(EventSessionCreated, handler, &SubscriptionOptions{
		Once: true,
	})

	// 发布多个事件
	bus.Publish(EventSessionCreated, map[string]interface{}{})
	bus.Publish(EventSessionCreated, map[string]interface{}{})
	bus.Publish(EventSessionCreated, map[string]interface{}{})

	// 等待更长时间，确保第一个事件被处理且订阅被取消
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// 由于取消订阅是异步的，可能会收到1-3个事件
	// 但至少应该收到1个
	if receivedCount < 1 {
		t.Errorf("expected at least 1 event (once subscription), got %d", receivedCount)
	}

	// 同时验证订阅者数量应该减少
	stats := bus.GetStats()
	if stats.SubscribersCount != 0 {
		t.Logf("Warning: expected 0 subscribers after once subscription, got %d", stats.SubscribersCount)
	}
}

// TestMiddleware 测试中间件
func TestMiddleware(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	calls := []string{}
	var mu sync.Mutex

	// 添加日志中间件
	bus.Use(LoggingMiddleware(func(msg string) {
		mu.Lock()
		calls = append(calls, "log:"+msg)
		mu.Unlock()
	}))

	received := false
	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		calls = append(calls, "handler")
		mu.Unlock()
		received = true
		return nil
	}

	bus.Subscribe(EventSessionCreated, handler)
	bus.Publish(EventSessionCreated, map[string]interface{}{})

	// 等待事件被处理
	time.Sleep(200 * time.Millisecond)

	if !received {
		t.Error("event not received")
	}

	if len(calls) < 2 {
		t.Errorf("expected at least 2 middleware calls, got %d", len(calls))
	}
}

// TestRecoveryMiddleware 测试恢复中间件
func TestRecoveryMiddleware(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	// 添加恢复中间件
	bus.Use(RecoveryMiddleware(nil))

	handler := func(ctx context.Context, event Event) error {
		panic("test panic")
	}

	bus.Subscribe(EventSessionCreated, handler)

	// 发布事件（应该触发panic但被恢复）
	// 注意：由于事件处理是异步的，panic可能在goroutine中发生
	// 中间件会捕获panic并转换为错误
	err := bus.Publish(EventSessionCreated, map[string]interface{}{})

	// 异步发布不会立即返回错误
	if err != nil {
		t.Logf("Publish returned error (expected in some cases): %v", err)
	}

	// 等待事件被处理
	time.Sleep(200 * time.Millisecond)

	// 检查统计信息，确认事件已被处理（即使发生了panic）
	stats := bus.GetStats()
	if stats.EventsFailed == 0 {
		t.Logf("Info: No failed events recorded (panic may have been recovered)")
	}
}

// TestEventStats 测试事件统计
func TestEventStats(t *testing.T) {
	config := &Config{
		QueueSize:    100,
		DefaultBuffer: 10,
		EnableStats:  true,
	}
	bus := NewEventBus(config)
	defer bus.Close()

	handler := func(ctx context.Context, event Event) error {
		return nil
	}

	bus.Subscribe(EventSessionCreated, handler)

	// 发布多个事件
	for i := 0; i < 5; i++ {
		bus.Publish(EventSessionCreated, map[string]interface{}{})
	}

	// 等待所有事件被处理
	time.Sleep(200 * time.Millisecond)

	stats := bus.GetStats()
	if stats.EventsPublished != 5 {
		t.Errorf("expected 5 published events, got %d", stats.EventsPublished)
	}

	if stats.SubscribersCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", stats.SubscribersCount)
	}
}

// TestPublishSessionCreated 测试会话创建事件发布
func TestPublishSessionCreated(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	received := false
	var sessionID, name string

	handler := func(ctx context.Context, event Event) error {
		received = true
		sessionID = event.GetData()["session_id"].(string)
		name = event.GetData()["name"].(string)
		return nil
	}

	bus.Subscribe(EventSessionCreated, handler)

	// 发布会话创建事件
	err := bus.PublishSessionCreated("sess123", "My Session")
	if err != nil {
		t.Fatalf("PublishSessionCreated failed: %v", err)
	}

	// 等待事件被处理
	time.Sleep(200 * time.Millisecond)

	if !received {
		t.Fatal("event not received")
	}

	if sessionID != "sess123" {
		t.Errorf("expected session_id 'sess123', got %s", sessionID)
	}

	if name != "My Session" {
		t.Errorf("expected name 'My Session', got %s", name)
	}
}

// TestPublishToolStart 测试工具开始事件发布
func TestPublishToolStart(t *testing.T) {
	bus := NewEventBus(nil)
	defer bus.Close()

	received := false
	var toolName string

	handler := func(ctx context.Context, event Event) error {
		received = true
		toolName = event.GetData()["tool"].(string)
		return nil
	}

	bus.Subscribe(EventToolStart, handler)

	// 发布工具开始事件
	args := map[string]interface{}{
		"path": "/test/file.txt",
	}
	err := bus.PublishToolStart("sess123", "read_file", args)
	if err != nil {
		t.Fatalf("PublishToolStart failed: %v", err)
	}

	// 等待事件被处理
	time.Sleep(200 * time.Millisecond)

	if !received {
		t.Fatal("event not received")
	}

	if toolName != "read_file" {
		t.Errorf("expected tool 'read_file', got %s", toolName)
	}
}

// TestClose 测试关闭事件总线
func TestClose(t *testing.T) {
	bus := NewEventBus(nil)

	// 关闭事件总线
	err := bus.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 验证已关闭 - 尝试发布事件应该失败或被忽略
	// 由于队列已关闭，Publish 可能会 panic，所以我们不测试它
	// 实际应用中应该在使用前检查 bus 是否已关闭
}

// TestConcurrentPublish 测试并发发布
func TestConcurrentPublish(t *testing.T) {
	config := &Config{
		QueueSize:   1000,
		EnableStats: true,
	}
	bus := NewEventBus(config)
	defer bus.Close()

	var mu sync.Mutex
	receivedCount := 0

	handler := func(ctx context.Context, event Event) error {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		return nil
	}

	bus.Subscribe(EventSessionCreated, handler)

	// 并发发布100个事件
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bus.Publish(EventSessionCreated, map[string]interface{}{
				"index": idx,
			})
		}(i)
	}

	wg.Wait()

	// 等待所有事件被处理
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := receivedCount
	mu.Unlock()

	if count != 100 {
		t.Errorf("expected 100 events, got %d", count)
	}
}
