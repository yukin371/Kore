package eventbus

import (
	"context"
	"fmt"
	"time"
)

// LoggingMiddleware 日志中间件
func LoggingMiddleware(logger func(string)) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			startTime := time.Now()

			// 记录事件开始
			if logger != nil {
				logger(fmt.Sprintf("[EventBus] Event started: %s", event.GetType()))
			}

			// 调用下一个处理器
			err := next(ctx, event)

			// 记录事件结束
			duration := time.Since(startTime)
			if logger != nil {
				if err != nil {
					logger(fmt.Sprintf("[EventBus] Event failed: %s (duration: %v, error: %v)",
						event.GetType(), duration, err))
				} else {
					logger(fmt.Sprintf("[EventBus] Event completed: %s (duration: %v)",
						event.GetType(), duration))
				}
			}

			return err
		}
	}
}

// RecoveryMiddleware 恢复中间件（捕获panic）
func RecoveryMiddleware(logger func(string)) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic recovered: %v", r)
					if logger != nil {
						logger(fmt.Sprintf("[EventBus] Panic recovered in event %s: %v",
							event.GetType(), r))
					}
				}
			}()

			return next(ctx, event)
		}
	}
}

// TimeoutMiddleware 超时中间件
func TimeoutMiddleware(timeout time.Duration) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// 创建通道来接收结果
			resultChan := make(chan error, 1)

			go func() {
				resultChan <- next(ctx, event)
			}()

			select {
			case err := <-resultChan:
				return err
			case <-ctx.Done():
				return fmt.Errorf("event handler timeout: %s", event.GetType())
			}
		}
	}
}

// ThrottleMiddleware 限流中间件
func ThrottleMiddleware(interval time.Duration) MiddlewareFunc {
	var throttleChan chan struct{}

	if interval > 0 {
		throttleChan = make(chan struct{}, 1)
		go func() {
			for range time.NewTicker(interval).C {
				select {
				case throttleChan <- struct{}{}:
				default:
				}
			}
		}()
	}

	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			if throttleChan != nil {
				<-throttleChan
			}

			return next(ctx, event)
		}
	}
}

// ValidationMiddleware 验证中间件
func ValidationMiddleware(validator func(Event) error) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			// 验证事件
			if validator != nil {
				if err := validator(event); err != nil {
					return fmt.Errorf("event validation failed: %w", err)
				}
			}

			return next(ctx, event)
		}
	}
}

// TransformMiddleware 转换中间件
func TransformMiddleware(transformer func(Event) Event) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			// 转换事件
			if transformer != nil {
				event = transformer(event)
			}

			return next(ctx, event)
		}
	}
}

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware(metrics *EventMetrics) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			startTime := time.Now()

			// 增加处理计数
			metrics.IncrementHandled(event.GetType())

			// 调用下一个处理器
			err := next(ctx, event)

			// 记录处理时间
			duration := time.Since(startTime)
			metrics.RecordDuration(event.GetType(), duration)

			// 记录错误
			if err != nil {
				metrics.IncrementErrors(event.GetType())
			}

			return err
		}
	}
}

// RetryMiddleware 重试中间件
func RetryMiddleware(maxRetries int, delay time.Duration) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					// 等待后重试
					select {
					case <-time.After(delay * time.Duration(attempt)):
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				if err := next(ctx, event); err == nil {
					return nil
				} else {
					lastErr = err
				}
			}

			return fmt.Errorf("event handler failed after %d retries: %w", maxRetries, lastErr)
		}
	}
}

// FilterMiddleware 过滤中间件
func FilterMiddleware(filter func(Event) bool) MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			// 检查是否应该处理此事件
			if filter != nil && !filter(event) {
				return nil // 跳过处理
			}

			return next(ctx, event)
		}
	}
}

// CircuitBreakerMiddleware 熔断器中间件
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	maxFailures     int
	resetTimeout    time.Duration
	currentState    CircuitState
	failures        int
	lastFailureTime time.Time
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		currentState: StateClosed,
	}
}

func (cb *CircuitBreaker) Middleware() MiddlewareFunc {
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			// 检查熔断器状态
			if cb.currentState == StateOpen {
				// 检查是否可以尝试恢复
				if time.Since(cb.lastFailureTime) > cb.resetTimeout {
					cb.currentState = StateHalfOpen
				} else {
					return fmt.Errorf("circuit breaker is open for event: %s", event.GetType())
				}
			}

			// 调用处理器
			err := next(ctx, event)

			// 更新熔断器状态
			if err != nil {
				cb.failures++
				cb.lastFailureTime = time.Now()

				if cb.failures >= cb.maxFailures {
					cb.currentState = StateOpen
				}

				return err
			}

			// 成功时重置
			if cb.currentState == StateHalfOpen {
				cb.currentState = StateClosed
			}
			cb.failures = 0

			return nil
		}
	}
}

// EventMetrics 事件指标
type EventMetrics struct {
	HandledCount  map[EventType]int64
	ErrorCount    map[EventType]int64
	Durations     map[EventType][]time.Duration
	mu            interface{}
}

// NewEventMetrics 创建事件指标收集器
func NewEventMetrics() *EventMetrics {
	return &EventMetrics{
		HandledCount: make(map[EventType]int64),
		ErrorCount:   make(map[EventType]int64),
		Durations:    make(map[EventType][]time.Duration),
	}
}

// IncrementHandled 增加处理计数
func (m *EventMetrics) IncrementHandled(eventType EventType) {
	m.HandledCount[eventType]++
}

// IncrementErrors 增加错误计数
func (m *EventMetrics) IncrementErrors(eventType EventType) {
	m.ErrorCount[eventType]++
}

// RecordDuration 记录处理时间
func (m *EventMetrics) RecordDuration(eventType EventType, duration time.Duration) {
	m.Durations[eventType] = append(m.Durations[eventType], duration)

	// 只保留最近100条记录
	if len(m.Durations[eventType]) > 100 {
		m.Durations[eventType] = m.Durations[eventType][1:]
	}
}

// GetAverageDuration 获取平均处理时间
func (m *EventMetrics) GetAverageDuration(eventType EventType) time.Duration {
	durations, ok := m.Durations[eventType]
	if !ok || len(durations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}

	return sum / time.Duration(len(durations))
}

// GetHandledCount 获取处理次数
func (m *EventMetrics) GetHandledCount(eventType EventType) int64 {
	return m.HandledCount[eventType]
}

// GetErrorCount 获取错误次数
func (m *EventMetrics) GetErrorCount(eventType EventType) int64 {
	return m.ErrorCount[eventType]
}

// GetErrorRate 获取错误率
func (m *EventMetrics) GetErrorRate(eventType EventType) float64 {
	handled := m.HandledCount[eventType]
	errors := m.ErrorCount[eventType]

	if handled == 0 {
		return 0
	}

	return float64(errors) / float64(handled)
}
