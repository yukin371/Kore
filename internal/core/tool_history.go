package core

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
	ID        string    // 调用 ID
	Tool      string    // 工具名称
	Arguments string    // 参数（JSON）
	Result    string    // 结果
	Timestamp time.Time // 时间戳
	Success   bool      // 是否成功
}

// ToolCallHistory 工具调用历史管理器
type ToolCallHistory struct {
	calls []ToolCallRecord
	mu    sync.RWMutex
}

// NewToolCallHistory 创建工具调用历史
func NewToolCallHistory() *ToolCallHistory {
	return &ToolCallHistory{
		calls: make([]ToolCallRecord, 0, 50),
	}
}

// Record 记录一次工具调用
func (h *ToolCallHistory) Record(call ToolCallRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.calls = append(h.calls, call)

	// 限制历史长度（保留最近 50 条）
	if len(h.calls) > 50 {
		h.calls = h.calls[len(h.calls)-50:]
	}
}

// GetSummary 获取工具调用摘要（提供给 AI）
func (h *ToolCallHistory) GetSummary() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.calls) == 0 {
		return "## 工具调用历史\n\n暂无工具调用记录。"
	}

	var summary strings.Builder
	summary.WriteString("## 最近工具调用\n\n")

	// 只显示最近 10 次调用
	count := len(h.calls)
	start := 0
	if count > 10 {
		start = count - 10
	}

	for i := start; i < count; i++ {
		call := h.calls[i]

		status := "✓"
		if !call.Success {
			status = "✗"
		}

		summary.WriteString(fmt.Sprintf("- [%s] **%s**(%s)\n",
			status, call.Tool, call.Arguments))

		// 如果失败，显示错误信息
		if !call.Success && call.Result != "" {
			// 截断长错误信息
			errMsg := call.Result
			if len(errMsg) > 100 {
				errMsg = errMsg[:97] + "..."
			}
			summary.WriteString(fmt.Sprintf("  错误: %s\n", errMsg))
		}
	}

	return summary.String()
}

// GetLastCallOfType 获取特定工具的最后一次调用
func (h *ToolCallHistory) GetLastCallOfType(toolName string) (ToolCallRecord, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := len(h.calls) - 1; i >= 0; i-- {
		if h.calls[i].Tool == toolName {
			return h.calls[i], true
		}
	}

	return ToolCallRecord{}, false
}

// Clear 清空历史
func (h *ToolCallHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.calls = make([]ToolCallRecord, 0, 50)
}

// GetAll 获取所有调用记录
func (h *ToolCallHistory) GetAll() []ToolCallRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]ToolCallRecord, len(h.calls))
	copy(result, h.calls)
	return result
}
