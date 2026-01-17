// Package agent 提供 Ralph Loop 自引用开发循环
//
// 持续执行直到任务完成，不会中途放弃
// 灵感来自: https://github.com/code-yeongyu/oh-my-opencode
package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yukin/kore/internal/core"
)

// RalphLoopConfig Ralph Loop 配置
type RalphLoopConfig struct {
	Enabled       bool
	MaxIterations int
	DoneToken     string  // 默认 "DONE"
}

// DefaultRalphLoopConfig 默认配置
func DefaultRalphLoopConfig() *RalphLoopConfig {
	return &RalphLoopConfig{
		Enabled:       false, // 默认关闭，通过命令或关键词激活
		MaxIterations: 100,
		DoneToken:     "DONE",
	}
}

// RalphLoopController Ralph Loop 控制器
type RalphLoopController struct {
	config        *RalphLoopConfig
	agent         *Agent
	contextMgr    *core.ContextManager
	llmProvider   core.LLMProvider
	toolExecutor  core.ToolExecutor
	ui            core.UIInterface
	history       *core.ConversationHistory

	currentLoop  int
	startTime     time.Time
	lastAction    time.Time
	mu            sync.RWMutex
}

// NewRalphLoopController 创建 Ralph Loop 控制器
func NewRalphLoopController(
	agent *Agent,
	contextMgr *core.ContextManager,
	llmProvider core.LLMProvider,
	toolExecutor core.ToolExecutor,
	ui core.UIInterface,
	config *RalphLoopConfig,
) *RalphLoopController {
	return &RalphLoopController{
		config:       config,
		agent:       agent,
		contextMgr:   contextMgr,
		llmProvider:  llmProvider,
		toolExecutor: toolExecutor,
		ui:           ui,
		history:      core.NewConversationHistory(),
		config:       config,
		startTime:    time.Now(),
	}
}

// Run 运行 Ralph Loop
func (rlc *RalphLoopController) Run(ctx context.Context, prompt string) error {
	rlc.mu.Lock()
	rlc.currentLoop = 0
	rlc.startTime = time.Now()
	rlc.mu.Unlock()

	rlc.ui.SendStream("\n🔄 Ralph Loop 模式启动！将持续执行直到任务完成。\n\n")
	rlc.ui.ShowStatus("Ralph Loop: 初始化中...")

	// 添加初始用户消息
	rlc.history.AddUserMessage(prompt)

	// 检查最后一条助手消息
	if len(rlc.history.messages) == 0 {

	for rlc.currentLoop < rlc.config.MaxIterations {
		// 检查上下文使用率
		if err := rlc.checkAndHandleContext(ctx); err != nil {
			return fmt.Errorf("上下文检查失败: %w", err)
		}

		// 执行一次 Agent 迭代
		rlc.ui.ShowStatus(fmt.Sprintf("Ralph Loop: 迭代 %d/%d", rlc.currentLoop+1, rlc.config.MaxIterations))

		// 构建请求
		rlc.ui.SendStream(fmt.Sprintf("\n--- 迭代 %d ---\n", rlc.currentLoop+1))

		// 运行 Agent（这会自动处理工具调用等）
		err := rlc.agent.Run(ctx, prompt)
		if err != nil {
			rlc.ui.SendStream(fmt.Sprintf("\n❌ 错误: %v\n", err))
			// 即使出错也继续尝试，除非是致命错误
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// 继续下一轮，可能 AI 可以自我修正
		}

		rlc.currentLoop++

		// 检查是否完成
		if rlc.isTaskComplete() {
			rlc.ui.SendStream("\n✅ 任务完成！Ralph Loop 结束。\n")
			rlc.ui.ShowStatus("Ralph Loop: 完成")
			return nil
		}

		// 检查是否应该停止
		if rlc.shouldStop(ctx) {
			rlc.ui.SendStream(fmt.Sprintf("\n⏸️ Ralph Loop 在 %d 次迭代后停止\n", rlc.currentLoop))
			return fmt.Errorf("达到最大循环次数或用户取消")
		}

		// 更新提示，引用历史
		prompt = rlc.generateNextPrompt(ctx)
	}

	return fmt.Errorf("达到最大循环次数 %d", rlc.config.MaxIterations)
}

// isTaskComplete 检查任务是否完成
func (rlc *RalphLoopController) isTaskComplete() bool {
	// 检查最后一条助手消息
	if len(rlc.history.messages) == 0 {
		return false
	}

	lastMsg := rlc.history.messages[len(rlc.history.messages)-1]

	// 检查是否包含 DONE 标记
	if lastMsg.Role == "assistant" {
		content := strings.ToUpper(lastMsg.Content)
		return strings.Contains(content, strings.ToUpper(rlc.config.DoneToken))
	}

	// 检查工具输出中是否有完成标记
	for _, msg := range rlc.history.messages {
		if msg.Role == "tool" {
			if strings.Contains(strings.ToUpper(msg.Content), strings.ToUpper(rlc.config.DoneToken)) {
				return true
			}
		}
	}

	return false
}

// shouldStop 检查是否应该停止
func (rlc *RalphLoopController) shouldStop(ctx context.Context) bool {
	// 检查用户是否取消
	if ctx.Err() != nil {
		return true
	}

	// 检查最后一次操作是否太久（超过 5 分钟没有活动）
	if time.Since(rlc.lastAction) > 5*time.Minute {
		rlc.ui.SendStream("\n⚠️ 检测到长时间无活动，询问是否继续...\n")
		// TODO: 实现用户确认逻辑
		return true
	}

	return false
}

// generateNextPrompt 生成下一轮提示
func (rlc *RalphLoopController) generateNextPrompt(ctx context.Context) string {
	var prompt strings.Builder

	prompt.WriteString("\n## 历史回顾\n\n")

	// 添加最近的对话（简化版，节省 token）
	maxHistory := 5  // 只保留最近 5 条消息
	startIdx := len(rlc.history.messages) - maxHistory
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(rlc.history.messages); i++ {
		msg := rlc.history.messages[i]
		prompt.WriteString(fmt.Sprintf("**%s**: %s\n\n", msg.Role, msg.Content))
	}

	prompt.WriteString("\n## 继续任务\n\n")
	prompt.WriteString("请继续完成上述任务。如果已经完成，请在回复中明确包含 \"DONE\"。\n")
	prompt.WriteString("如果遇到问题，请尝试不同的方法或寻求帮助。\n")

	return prompt.String()
}

// checkAndHandleContext 检查并处理上下文问题
func (rlc *RalphLoopController) checkAndHandleContext(ctx context.Context) error {
	// 获取模型最大 token 数（这里简化处理，实际应该从 LLM provider 获取）
	modelMaxTokens := 200000 // 假设 Claude Opus 4.5 的 200k token

	// 创建上下文监控器
	monitor := &ContextMonitor{
		warningThreshold: 0.7,
		compressThreshold: 0.85,
	}

	action := monitor.Check(rlc.history, modelMaxTokens)

	switch action {
	case ActionWarn:
		report := monitor.GetUsageReport(rlc.history, modelMaxTokens)
		rlc.ui.SendStream(fmt.Sprintf("\n⚠️ %s\n", monitor.FormatPrompt(report)))
		rlc.ui.ShowStatus(fmt.Sprintf("上下文使用: %.1f%%", report.UsagePercent*100))
		// 等待用户确认是否继续
		time.Sleep(2 * time.Second)

	case ActionCompress:
		rlc.ui.SendStream("\n🗜️ 上下文即将用尽，自动压缩会话...\n")
		compressionPrompt := monitor.CreateCompressionPrompt(rlc.history)

		// 创建新会话，使用压缩后的上下文
	// TODO: 实现会话创建和切换
	rlc.ui.SendStream(fmt.Sprintf("\n压缩后的上下文:\n%s\n", compressionPrompt))

		// 清空旧历史，保留压缩后的摘要
	rlc.history = core.NewConversationHistory()
		rlc.history.AddUserMessage(compressionPrompt)
		rlc.ui.ShowStatus("会话已压缩")
		time.Sleep(1 * time.Second)
	}

	return nil
}

// GetStatistics 获取统计信息
func (rlc *RalphLoopController) GetStatistics() *RalphLoopStatistics {
	rlc.mu.RLock()
	defer rlc.RUnlock()

	duration := time.Since(rlc.startTime)

	return &RalphLoopStatistics{
		CurrentLoop: rlc.currentLoop,
	TotalActions: len(rlc.history.messages),
		Duration:     duration,
	IsRunning:    rlc.currentLoop < rlc.config.MaxIterations,
	}
}

// RalphLoopStatistics Ralph Loop 统计信息
type RalphLoopStatistics struct {
	CurrentLoop int
	TotalActions int
	Duration     time.Duration
	IsRunning    bool
}

// IsActive 检查 Ralph Loop 是否活跃
func (rlc *RalphLoopController) IsActive() bool {
	rlc.mu.RLock()
	defer rlc.RUnlock()
	return rlc.currentLoop > 0 && rlc.currentLoop < rlc.config.MaxIterations
}

// Cancel 取消 Ralph Loop
func (rlc *RalphLoopController) Cancel() {
	rlc.mu.Lock()
	defer rlc.mu.Unlock()
	rlc.currentLoop = rlc.config.MaxIterations // 强制停止
	rlc.ui.SendStream("\n⏹ Ralph Loop 已取消\n")
}

// EnableRalphLoop 在现有 Agent 中启用 Ralph Loop 模式
func EnableRalphLoop(agent *Agent, config *RalphLoopConfig) *RalphLoopController {
	return &RalphLoopController{
		agent:        agent,
		contextMgr:   agent.ContextMgr,
		llmProvider:  agent.LLMProvider,
		toolExecutor:  agent.Tools,
		ui:            agent.UI,
		history:       agent.History,
		config:        config,
		startTime:    time.Now(),
	}
}
