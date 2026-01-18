// Package agent 提供 TODO 继续执行器
//
// 强制智能体完成所有未完成的 TODO 事项
// 灵感来自: https://github.com/code-yeongyu/oh-my-opencode
package agent

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yukin371/Kore/internal/core"
)

// TodoContinuator TODO 继续执行器
type TodoContinuator struct {
	enabled bool
	todos   []*Todo
	mu      sync.RWMutex
}

// NewTodoContinuator 创建新的 TODO 继续执行器
func NewTodoContinuator() *TodoContinuator {
	return &TodoContinuator{
		enabled: true,
		todos:   make([]*Todo, 0),
	}
}

// Enable 启用 Todo 继续执行
func (tc *TodoContinuator) Enable() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.enabled = true
}

// Disable 禁用 Todo 继续执行
func (tc *TodoContinuator) Disable() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.enabled = false
}

// ExtractTodos 从对话历史中提取 TODO 事项
func (tc *TodoContinuator) ExtractTodos(history *core.ConversationHistory) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// 清空现有 TODO
	tc.todos = tc.todos[:0]

	// 从最近的 10 条消息中提取 TODO
	maxMessages := 10
	startIdx := len(history.GetMessages()) - maxMessages
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(history.GetMessages()); i++ {
		msg := history.GetMessages()[i]
		tc.todos = append(tc.todos, tc.extractTodosFromMessage(msg)...)
	}

	// 去重 TODO
	tc.deduplicateTodos()
}

// extractTodosFromMessage 从单条消息中提取 TODO
func (tc *TodoContinuator) extractTodosFromMessage(msg core.Message) []*Todo {
	var todos []*Todo

	// 匹配 TODO 格式：- [ ] 未完成的任务，- [x] 已完成的任务
	re := regexp.MustCompile(`-\s*\[([ x])\]\s*(.+)`)
	// 匹配 TODO: 开头的 TODO 任务
	reTodo := regexp.MustCompile(`(?i)^TODO:\s*(.+)`)
	reFixme := regexp.MustCompile(`(?i)^FIXME:\s*(.+)`)
	reAsk := regexp.MustCompile(`(?i)^(ASK|询问)\b`)

	// 检查是否包含 TODO/FIXME/ASK 标记
	hasTodo := reTodo.MatchString(msg.Content)
	hasFixme := reFixme.MatchString(msg.Content)
	hasAsk := reAsk.MatchString(msg.Content)

	if !hasTodo && !hasFixme && !hasAsk && !re.MatchString(msg.Content) {
		return []*Todo{} // 没有 TODO 相关内容
	}

	// 提取 [ ] 和 [x] 格式的 TODO
	matches := re.FindAllStringSubmatch(msg.Content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		// match[1] = 状态，match[2] = 描述
		status := match[1] != "x"

		// 标准化为 TODO 结构
		todo := &Todo{
			Description: strings.TrimSpace(match[2]),
			Done:        !status, // [ ] 未完成，[x] 已完成
			Source:      msg.Role,
			CreatedAt:   time.Now(),
		}

		todos = append(todos, todo)
	}

	// 提取 TODO: 开头的 TODO
	if hasTodo {
		matches := reTodo.FindAllStringSubmatch(msg.Content, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			todo := &Todo{
				Description: strings.TrimSpace(match[1]),
				Done:        false,
				Source:      msg.Role,
				CreatedAt:   time.Now(),
			}
			todos = append(todos, todo)
		}
	}

	// 提取 FIXME: 开头的 TODO
	if hasFixme {
		matches := reFixme.FindAllStringSubmatch(msg.Content, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			todo := &Todo{
				Description: strings.TrimSpace(match[1]),
				Done:        false,
				Source:      msg.Role,
				CreatedAt:   time.Now(),
			}
			todos = append(todos, todo)
		}
	}

	return todos
}

// deduplicateTodos 去重 TODO
func (tc *TodoContinuator) deduplicateTodos() {
	seen := make(map[string]bool)
	var uniqueTodos []*Todo

	for _, todo := range tc.todos {
		key := todo.Description
		if !seen[key] {
			seen[key] = true
			uniqueTodos = append(uniqueTodos, todo)
		}
	}

	tc.todos = uniqueTodos
}

// GetIncompleteTodos 获取所有未完成的 TODO
func (tc *TodoContinuator) GetIncompleteTodos() []*Todo {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	var incomplete []*Todo
	for _, todo := range tc.todos {
		if !todo.Done && !isTaskComplete(todo.Description) {
			incomplete = append(incomplete, todo)
		}
	}

	return incomplete
}

// isTaskComplete 判断任务是否完成
func isTaskComplete(description string) bool {
	// 检查是否包含完成关键词
	completionKeywords := []string{
		"完成", "已完成", "done", "已实现", "implemented",
		"resolved", "已修复", "fixed", "solved",
	}

	descLower := strings.ToLower(description)
	for _, keyword := range completionKeywords {
		if strings.Contains(descLower, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

// Enforce 检查并强制完成所有 TODO
func (tc *TodoContinuator) Enforce(history *core.ConversationHistory) error {
	if !tc.enabled {
		return nil
	}

	// 提取 TODO
	tc.ExtractTodos(history)

	// 获取未完成的 TODO
	incomplete := tc.GetIncompleteTodos()

	if len(incomplete) == 0 {
		// 没有未完成的 TODO
		return nil
	}

	// 生成错误消息
	todoDesc := make([]string, 0, len(incomplete))
	for _, todo := range incomplete {
		todoDesc = append(todoDesc, fmt.Sprintf("- [ ] %s", todo.Description))
	}

	return fmt.Errorf("还有 %d 个 TODO 事项未完成:\n\n%s\n\n请先完成这些 TODO。",
		len(incomplete),
		strings.Join(todoDesc, "\n"))
}

// UpdateTodoStatus 更新 TODO 状态
func (tc *TodoContinuator) UpdateTodoStatus(todo *Todo) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	for _, t := range tc.todos {
		if t.Description == todo.Description {
			if todo.Done != t.Done {
				// TODO 状态改变
				t.Done = todo.Done
				return
			}
		}
	}

	// 如果不存在，添加新的 TODO
	tc.todos = append(tc.todos, todo)
}

// GetTodoList 获取所有 TODO
func (tc *TodoContinuator) GetTodoList() []*Todo {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	result := make([]*Todo, len(tc.todos))
	copy(result, tc.todos)
	return result
}

// Clear 清空 TODO 列表
func (tc *TodoContinuator) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.todos = make([]*Todo, 0)
}

// GetTodoStats 获取 TODO 统计
func (tc *TodoContinuator) GetTodoStats() *TodoStats {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	stats := &TodoStats{
		Total:     len(tc.todos),
		Completed: 0,
		Remaining: 0,
		BySource:  make(map[string]int),
	}

	for _, todo := range tc.todos {
		if todo.Done {
			stats.Completed++
		} else {
			stats.Remaining++
		}
		stats.BySource[todo.Source]++
	}

	return stats
}

// TodoStats TODO 统计信息
type TodoStats struct {
	Total     int            `json:"total"`
	Completed int            `json:"completed"`
	Remaining int            `json:"remaining"`
	BySource  map[string]int `json:"by_source"`
}

// IsTodoPresent 检查是否有 TODO 事项
func (tc *TodoContinuator) IsTodoPresent(history *core.ConversationHistory) bool {
	for _, msg := range history.GetMessages() {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, "todo") ||
			strings.Contains(content, "[ ]") ||
			strings.Contains(content, "fixme") {
			return true
		}
	}

	return false
}

// FilterBySource 按来源过滤 TODO
func (tc *TodoContinuator) FilterBySource(source string) []*Todo {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	var filtered []*Todo
	for _, todo := range tc.todos {
		if todo.Source == source {
			filtered = append(filtered, todo)
		}
	}

	return filtered
}

// SortByPriority 按优先级排序 TODO
func (tc *TodoContinuator) SortByPriority() []*Todo {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	sorted := make([]*Todo, len(tc.todos))
	copy(sorted, tc.todos)

	// 按以下优先级排序：
	// 1. 未完成的 TODO 优先
	// 2. 用户 TODO 优先于工具 TODO
	// 3. 近期 TODO 优先于远期 TODO

	sort.Slice(sorted, func(i, j int) bool {
		// 未完成的优先
		if sorted[i].Done != sorted[j].Done {
			return !sorted[i].Done && sorted[j].Done
		}

		// 用户 TODO 优先
		if sorted[i].Source == "user" && sorted[j].Source != "user" {
			return true
		}

		// 近期 TODO 优先（通过在列表中的位置判断）
		return i < j
	})

	return sorted
}

// GeneratePrompt 生成 TODO 继续执行提示
func (tc *TodoContinuator) GeneratePrompt(history *core.ConversationHistory) string {
	incomplete := tc.GetIncompleteTodos()

	if len(incomplete) == 0 {
		return "所有 TODO 已完成！"
	}

	var prompt strings.Builder

	prompt.WriteString("\n## 待办事项检查\n\n")
	prompt.WriteString("以下 TODO 事项尚未完成：\n\n")

	for _, todo := range incomplete {
		mark := "[ ]"
		if todo.Done {
			mark = "[x]"
		}
		source := ""
		if todo.Source != "" {
			source = fmt.Sprintf(" (%s)", todo.Source)
		}
		prompt.WriteString(fmt.Sprintf("%s %s%s\n", mark, todo.Description, source))
	}

	prompt.WriteString("\n请按优先级完成这些 TODO 事项。")
	prompt.WriteString("完成后请明确说明 \"已完成\"。\n")

	return prompt.String()
}

// ShouldForceTodo 判断是否应该强制执行 TODO
func (tc *TodoContinuator) ShouldForceTodo(history *core.ConversationHistory) bool {
	if !tc.enabled {
		return false
	}

	// 检查最近的对话中是否有 TODO 事项
	maxMessages := 5
	startIdx := len(history.GetMessages()) - maxMessages
	if startIdx < 0 {
		startIdx = 0
	}

	// 检查是否有未完成的 TODO
	for i := startIdx; i < len(history.GetMessages()); i++ {
		msg := history.GetMessages()[i]
		content := strings.ToLower(msg.Content)

		// 快速检查是否包含 TODO 相关的词
		if strings.Contains(content, "todo") ||
			strings.Contains(content, "[ ]") ||
			strings.Contains(content, "fixme") {
			// 发现 TODO，检查是否完成
			todos := tc.extractTodosFromMessage(msg)
			for _, todo := range todos {
				if !todo.Done && !isTaskComplete(todo.Description) {
					return true
				}
			}
		}
	}

	return false
}

// GetReminders 生成提醒消息
func (tc *TodoContinuator) GetReminders() []string {
	incomplete := tc.GetIncompleteTodos()

	if len(incomplete) == 0 {
		return []string{"所有 TODO 已完成！"}
	}

	var reminders []string

	// 按来源分组
	// 用户 TODO 优先
	// 工具 TODO 次之
	// 系统 TODO 最后

	// 用户 TODO
	userTodos := tc.FilterBySource("user")
	if len(userTodos) > 0 {
		reminders = append(reminders, fmt.Sprintf("待完成的用户 TODO 事项 (%d 个):", len(userTodos)))
		for _, todo := range userTodos {
			if !todo.Done {
				reminders = append(reminders, fmt.Sprintf("  - [ ] %s", todo.Description))
			}
		}
	}

	// 工具 TODO
	toolTodos := tc.FilterBySource("tool")
	if len(toolTodos) > 0 {
		reminders = append(reminders, "\n工具生成的 TODO 事项:")
		for _, todo := range toolTodos[:3] { // 最多显示 3 个
			if !todo.Done {
				reminders = append(reminders, fmt.Sprintf("  - [ ] %s", todo.Description))
			}
		}
	}

	// 系统 TODO
	systemTodos := tc.FilterBySource("system")
	if len(systemTodos) > 0 {
		reminders = append(reminders, "\n系统生成的 TODO 事项（建议人工检查）:")
		for _, todo := range systemTodos[:3] {
			if !todo.Done {
				reminders = append(reminders, fmt.Sprintf("  - [ ] %s", todo.Description))
			}
		}
	}

	return reminders
}

// SetEnabled 设置是否启用
func (tc *TodoContinuator) SetEnabled(enabled bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.enabled = enabled
}

// IsEnabled 检查是否启用
func (tc *TodoContinuator) IsEnabled() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.enabled
}

// GetStatus 获取当前状态
func (tc *TodoContinuator) GetStatus() *TodoStatus {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	stats := tc.GetTodoStats()

	return &TodoStatus{
		Enabled:   tc.enabled,
		Total:     stats.Total,
		Completed: stats.Completed,
		Remaining: stats.Remaining,
		BySource:  stats.BySource,
	}
}

// TodoStatus TODO 状态
type TodoStatus struct {
	Enabled   bool
	Total     int
	Completed int
	Remaining int
	BySource  map[string]int
}

// Todo TODO 事项
type Todo struct {
	Description string
	Done        bool
	Source      string // "user", "assistant", "tool", "system"
	CreatedAt   time.Time
}

// NewTodo 创建新 TODO
func (tc *TodoContinuator) NewTodo(description string, source string) *Todo {
	return &Todo{
		Description: description,
		Done:        false,
		Source:      source,
		CreatedAt:   time.Now(),
	}
}

// GetCompletedCount 获取已完成 TODO 数量
func (tc *TodoContinuator) GetCompletedCount() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	count := 0
	for _, todo := range tc.todos {
		if todo.Done {
			count++
		}
	}

	return count
}

// GetRemainingCount 获取未完成 TODO 数量
func (tc *TodoContinuator) GetRemainingCount() int {
	return len(tc.todos) - tc.GetCompletedCount()
}
