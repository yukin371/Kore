package eventbus

import (
	"strings"
)

// FilterType 按类型过滤
func FilterType(eventType EventType) EventFilter {
	return func(event Event) bool {
		return event.GetType() == eventType
	}
}

// FilterTypes 按多个类型过滤（OR）
func FilterTypes(eventTypes ...EventType) EventFilter {
	return func(event Event) bool {
		for _, et := range eventTypes {
			if event.GetType() == et {
				return true
			}
		}
		return false
	}
}

// FilterPriority 按优先级过滤
func FilterPriority(priority EventPriority) EventFilter {
	return func(event Event) bool {
		return event.GetPriority() == priority
	}
}

// FilterPriorityMin 按最小优先级过滤（>=）
func FilterPriorityMin(minPriority EventPriority) EventFilter {
	return func(event Event) bool {
		return event.GetPriority() >= minPriority
	}
}

// FilterPriorityMax 按最大优先级过滤（<=）
func FilterPriorityMax(maxPriority EventPriority) EventFilter {
	return func(event Event) bool {
		return event.GetPriority() <= maxPriority
	}
}

// FilterDataField 按数据字段过滤
func FilterDataField(key string, value interface{}) EventFilter {
	return func(event Event) bool {
		data := event.GetData()
		if data == nil {
			return false
		}

		v, ok := data[key]
		if !ok {
			return false
		}

		return v == value
	}
}

// FilterDataFieldExists 检查数据字段是否存在
func FilterDataFieldExists(key string) EventFilter {
	return func(event Event) bool {
		data := event.GetData()
		if data == nil {
			return false
		}

		_, ok := data[key]
		return ok
	}
}

// FilterMetadataField 按元数据字段过滤
func FilterMetadataField(key string, value interface{}) EventFilter {
	return func(event Event) bool {
		metadata := event.GetMetadata()
		if metadata == nil {
			return false
		}

		v, ok := metadata[key]
		if !ok {
			return false
		}

		return v == value
	}
}

// FilterSessionID 按会话ID过滤（便捷函数）
func FilterSessionID(sessionID string) EventFilter {
	return FilterDataField("session_id", sessionID)
}

// FilterToolName 按工具名称过滤（便捷函数）
func FilterToolName(toolName string) EventFilter {
	return FilterDataField("tool", toolName)
}

// FilterContentType 按内容类型过滤（便捷函数）
func FilterContentType(contentType string) EventFilter {
	return FilterDataField("content_type", contentType)
}

// FilterWildcard 通配符过滤（支持 * 匹配）
//
// 示例:
// - "tool.*" 匹配所有工具事件
// - "*.start" 匹配所有开始事件
func FilterWildcard(pattern string) EventFilter {
	return func(event Event) bool {
		eventType := string(event.GetType())

		// 完全匹配
		if pattern == eventType {
			return true
		}

		// 通配符匹配
		if strings.Contains(pattern, "*") {
			// 转换为正则表达式
			regexPattern := strings.ReplaceAll(pattern, ".", "\\.")
			regexPattern = strings.ReplaceAll(regexPattern, "*", ".*")

			// 简单的前缀/后缀匹配
			if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
				// *pattern* - 包含
				search := strings.TrimPrefix(pattern, "*")
				search = strings.TrimSuffix(search, "*")
				return strings.Contains(eventType, search)
			}

			if strings.HasPrefix(pattern, "*") {
				// *pattern - 后缀匹配
				suffix := strings.TrimPrefix(pattern, "*")
				return strings.HasSuffix(eventType, suffix)
			}

			if strings.HasSuffix(pattern, "*") {
				// pattern* - 前缀匹配
				prefix := strings.TrimSuffix(pattern, "*")
				return strings.HasPrefix(eventType, prefix)
			}
		}

		return false
	}
}

// FilterAnd 逻辑与过滤器
func FilterAnd(filters ...EventFilter) EventFilter {
	return func(event Event) bool {
		for _, filter := range filters {
			if !filter(event) {
				return false
			}
		}
		return true
	}
}

// FilterOr 逻辑或过滤器
func FilterOr(filters ...EventFilter) EventFilter {
	return func(event Event) bool {
		for _, filter := range filters {
			if filter(event) {
				return true
			}
		}
		return false
	}
}

// FilterNot 逻辑非过滤器
func FilterNot(filter EventFilter) EventFilter {
	return func(event Event) bool {
		return !filter(event)
	}
}

// FilterCustom 自定义过滤器
func FilterCustom(fn func(Event) bool) EventFilter {
	return fn
}

// FilterDataFieldString 按字符串字段过滤（支持部分匹配）
func FilterDataFieldString(key, substring string, contains bool) EventFilter {
	return func(event Event) bool {
		data := event.GetData()
		if data == nil {
			return false
		}

		v, ok := data[key]
		if !ok {
			return false
		}

		str, ok := v.(string)
		if !ok {
			return false
		}

		if contains {
			return strings.Contains(str, substring)
		}
		return str == substring
	}
}

// FilterByCategory 按事件类别过滤
//
// 事件类别：
// - "session": 会话事件
// - "message": 消息事件
// - "agent": Agent事件
// - "tool": 工具事件
// - "ui": UI事件
func FilterByCategory(category string) EventFilter {
	return func(event Event) bool {
		eventType := string(event.GetType())

		switch category {
		case "session":
			return strings.HasPrefix(eventType, "session.")
		case "message":
			return strings.HasPrefix(eventType, "message.")
		case "agent":
			return strings.HasPrefix(eventType, "agent.")
		case "tool":
			return strings.HasPrefix(eventType, "tool.")
		case "ui":
			return strings.HasPrefix(eventType, "ui.")
		default:
			return false
		}
	}
}
