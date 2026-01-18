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
	return func(next EventHandler) EventHandler {
		return func(ctx context.Context, event Event) error {
			// 简单实现：使用 time.After 等待间隔
			if interval > 0 {
				time.Sleep(interval)
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
