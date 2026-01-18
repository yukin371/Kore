// Package agent 提供上下文窗口监控功能
//
// 监控 token 使用率，在达到阈值时提醒或自动压缩
// 灵感来自: https://github.com/code-yeongyu/oh-my-opencode
package agent

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yukin371/Kore/internal/core"
)

// ContextMonitor 上下文窗口监控器
type ContextMonitor struct {
	warningThreshold float64  // 0.7 = 70%
	compressThreshold float64  // 0.85 = 85%
}

// MonitorAction 监控动作
type MonitorAction int

const (
	ActionNone   MonitorAction = iota // 无操作
	ActionWarn                        // 警告用户
	ActionCompress                     // 自动压缩
)

// TokenEstimator Token 估算器
type TokenEstimator struct {
	// 粗略估算：1 token ≈ 4 字符（英文）或 1-2 字符（中文）
}

// EstimateTokens 估算文本的 token 数量
func (te *TokenEstimator) EstimateTokens(text string) int {
	// 粗略估算：英文约 4 字符/token，中文约 1.5 字符/token
	// 取平均值：3 字符/token
	runeCount := utf8.RuneCountInString(text)
	return int(float64(runeCount) / 3.0)
}

// Check 检查上下文使用率并返回建议的动作
func (cm *ContextMonitor) Check(history *core.ConversationHistory, modelMaxTokens int) MonitorAction {
	usage := cm.calculateUsage(history, modelMaxTokens)

	if usage >= cm.compressThreshold {
		return ActionCompress
	}

	if usage >= cm.warningThreshold {
		return ActionWarn
	}

	return ActionNone
}

// calculateUsage 计算当前使用率
func (cm *ContextMonitor) calculateUsage(history *core.ConversationHistory, maxTokens int) float64 {
	estimator := &TokenEstimator{}
	totalTokens := 0

	// 估算所有消息的 token 数量
	for _, msg := range history.GetMessages() {
		totalTokens += estimator.EstimateTokens(msg.Content)
		totalTokens += estimator.EstimateTokens(msg.Role) // role 字段
	}

	return float64(totalTokens) / float64(maxTokens)
}

// GetUsageReport 获取使用率报告
func (cm *ContextMonitor) GetUsageReport(history *core.ConversationHistory, modelMaxTokens int) *UsageReport {
	usage := cm.calculateUsage(history, modelMaxTokens)
	estimator := &TokenEstimator{}
	totalTokens := 0

	for _, msg := range history.GetMessages() {
		totalTokens += estimator.EstimateTokens(msg.Content)
	}

	return &UsageReport{
		UsagePercent:    usage,
		EstimatedTokens:  totalTokens,
		MaxTokens:       modelMaxTokens,
		RemainingTokens:  modelMaxTokens - totalTokens,
		RecommendedAction: cm.getRecommendedAction(usage),
	}
}

// UsageReport 使用率报告
type UsageReport struct {
	UsagePercent    float64
	EstimatedTokens int
	MaxTokens       int
	RemainingTokens int
	RecommendedAction string
}

func (cm *ContextMonitor) getRecommendedAction(usage float64) string {
	if usage >= cm.compressThreshold {
		return "建议立即压缩会话历史"
	}
	if usage >= cm.warningThreshold {
		return "警告：上下文即将用尽"
	}
	return "上下文充足"
}

// FormatPrompt 提示信息格式化
func (cm *ContextMonitor) FormatPrompt(report *UsageReport) string {
	var status string
	var icon string

	switch {
	case report.UsagePercent >= 0.85:
		status = "严重"
		icon = "🔴"
	case report.UsagePercent >= 0.7:
		status = "警告"
		icon = "🟡"
	default:
		status = "正常"
		icon = "🟢"
	}

	return fmt.Sprintf("%s 上下文状态: %s\n当前使用: %.1f%% (%d/%d tokens)\n剩余空间: %d tokens\n建议: %s",
		icon, status, report.UsagePercent*100,
		report.EstimatedTokens, report.MaxTokens, report.RemainingTokens,
		report.RecommendedAction,
	)
}

// ShouldCompress 判断是否应该压缩
func (cm *ContextMonitor) ShouldCompress(history *core.ConversationHistory, modelMaxTokens int) bool {
	return cm.Check(history, modelMaxTokens) == ActionCompress
}

// CreateCompressionPrompt 创建压缩提示
func (cm *ContextMonitor) CreateCompressionPrompt(history *core.ConversationHistory) string {
	// 总结最近的对话，保留关键信息
	var summary strings.Builder

	summary.WriteString("# 对话总结\n\n")

	// 保留最近的用户请求
	lastUserIdx := -1
	for i := len(history.GetMessages()) - 1; i >= 0; i-- {
		if history.GetMessages()[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx >= 0 {
		summary.WriteString(fmt.Sprintf("## 最近的任务\n%s\n\n",
			history.GetMessages()[lastUserIdx].Content))
	}

	// 列出所有未完成的 TODO
	summary.WriteString("## 待办事项\n")
	todos := cm.extractTodos(history)
	if len(todos) > 0 {
		for _, todo := range todos {
			if !todo.Done {
				summary.WriteString(fmt.Sprintf("- [ ] %s\n", todo.Description))
			}
		}
	} else {
		summary.WriteString("(无待办事项)\n")
	}

	summary.WriteString("\n## 重要上下文\n")
	summary.WriteString("保留的关键上下文将在下一条消息中自动注入。\n")

	return summary.String()
}

// extractTodos 提取 TODO 事项（简化实现）
func (cm *ContextMonitor) extractTodos(history *core.ConversationHistory) []Todo {
	var todos []Todo

	for _, msg := range history.GetMessages() {
		// 简单的 TODO 检测
		lines := strings.Split(msg.Content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [x]") {
				todos = append(todos, Todo{
					Description: strings.TrimPrefix(line, "- [ ]"),
					Done:       strings.HasPrefix(line, "- [x]"),
				})
			}
		}
	}

	return todos
}
