package eventbus

import (
	"context"
	"fmt"
)

// ========== 会话事件 ==========

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

// PublishSessionUpdated 发布会话更新事件
func (bus *EventBus) PublishSessionUpdated(sessionID string, changes map[string]interface{}) error {
	data := map[string]interface{}{
		"session_id": sessionID,
	}
	for k, v := range changes {
		data[k] = v
	}
	return bus.Publish(EventSessionUpdated, data)
}

// ========== 消息事件 ==========

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

// ========== Agent 状态事件 ==========

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

// PublishAgentError 发布 Agent 错误事件
func (bus *EventBus) PublishAgentError(sessionID string, errMsg string) error {
	return bus.Publish(EventAgentError, map[string]interface{}{
		"session_id": sessionID,
		"error":      errMsg,
	})
}

// ========== 工具执行事件 ==========

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

// PublishToolError 发布工具错误事件
func (bus *EventBus) PublishToolError(sessionID, toolName, errMsg string) error {
	return bus.Publish(EventToolError, map[string]interface{}{
		"session_id": sessionID,
		"tool":       toolName,
		"error":      errMsg,
	})
}

// ========== UI 事件 ==========

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

// ========== 便捷函数 ==========

// PublishWithPriority 发布带优先级的事件
func (bus *EventBus) PublishWithPriority(eventType EventType, data map[string]interface{}, priority EventPriority) error {
	event := NewEventWithPriority(eventType, data, priority)
	return bus.PublishEvent(event)
}

// PublishWithMetadata 发布带元数据的事件
func (bus *EventBus) PublishWithMetadata(eventType EventType, data map[string]interface{}, metadata map[string]interface{}) error {
	event := NewEventWithMetadata(eventType, data, metadata)
	return bus.PublishEvent(event)
}

// PublishSessionEvent 发布会话相关事件的通用函数
func (bus *EventBus) PublishSessionEvent(eventType EventType, sessionID string, data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["session_id"] = sessionID
	return bus.Publish(eventType, data)
}

// PublishToolEvent 发布工具相关事件的通用函数
func (bus *EventBus) PublishToolEvent(eventType EventType, sessionID, toolName string, data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["session_id"] = sessionID
	data["tool"] = toolName
	return bus.Publish(eventType, data)
}

// PublishError 发布错误事件的通用函数
func (bus *EventBus) PublishError(eventType EventType, err error, data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["error"] = err.Error()
	return bus.Publish(eventType, data)
}

// ========== 批量发布 ==========

// PublishBatch 批量发布事件
func (bus *EventBus) PublishBatch(events []Event) error {
	for _, event := range events {
		if err := bus.PublishEvent(event); err != nil {
			return fmt.Errorf("failed to publish event %s: %w", event.GetType(), err)
		}
	}
	return nil
}

// PublishBatchSync 同步批量发布事件
func (bus *EventBus) PublishBatchSync(ctx context.Context, events []Event) error {
	for _, event := range events {
		if err := bus.PublishEventSync(ctx, event); err != nil {
			return fmt.Errorf("failed to publish event %s: %w", event.GetType(), err)
		}
	}
	return nil
}
