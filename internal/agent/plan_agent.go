// Package agent 提供 Plan Agent 实现
//
// Plan Agent（规划模式）- 只读访问权限
// - 权限: 只读访问权限
// - 用途: 分析代码、查看文件、提供建议
// - 场景: 用户说"帮我理解这个架构"
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/yukin/kore/internal/core"
	"github.com/yukin/kore/internal/types"
)

// PlanAgent 规划模式 Agent
//
// Plan Agent 只拥有只读权限，可以：
// - 读取文件
// - 搜索文件
// - 列出文件
//
// 不允许：
// - 写入文件
// - 执行命令
//
// 适用于代码分析、架构理解等只读任务
type PlanAgent struct {
	*BaseAgent
}

// NewPlanAgent 创建新的 Plan Agent
//
// 参数:
//   - coreAgent: 核心 Agent 实例
//   - projectRoot: 项目根目录
//
// 返回:
//   - *PlanAgent: Plan Agent 实例
//   - error: 错误信息
func NewPlanAgent(coreAgent *core.Agent, projectRoot string) (*PlanAgent, error) {
	config := DefaultAgentModeConfig(types.ModeSearch)

	baseAgent, err := NewBaseAgent(types.ModeSearch, config, coreAgent)
	if err != nil {
		return nil, fmt.Errorf("创建 Plan Agent 失败: %w", err)
	}

	return &PlanAgent{
		BaseAgent: baseAgent,
	}, nil
}

// NewPlanAgentWithConfig 使用自定义配置创建 Plan Agent
//
// 参数:
//   - coreAgent: 核心 Agent 实例
//   - config: 自定义配置
//
// 返回:
//   - *PlanAgent: Plan Agent 实例
//   - error: 错误信息
func NewPlanAgentWithConfig(coreAgent *core.Agent, config *AgentModeConfig) (*PlanAgent, error) {
	if config.Mode != types.ModeSearch {
		return nil, fmt.Errorf("配置模式不匹配: 期望 %s, 实际 %s", types.ModeSearch, config.Mode)
	}

	baseAgent, err := NewBaseAgent(types.ModeSearch, config, coreAgent)
	if err != nil {
		return nil, fmt.Errorf("创建 Plan Agent 失败: %w", err)
	}

	return &PlanAgent{
		BaseAgent: baseAgent,
	}, nil
}

// Run 执行 Plan Agent
//
// Plan Agent 的执行流程：
// 1. 检查工具权限（只允许只读工具）
// 2. 执行核心 Agent 逻辑
// 3. 拦截写入和执行操作
//
// 参数:
//   - ctx: 上下文
//   - prompt: 用户提示
//
// 返回:
//   - error: 错误信息
func (a *PlanAgent) Run(ctx context.Context, prompt string) error {
	// 验证 Agent 有效性
	if !a.IsValid() {
		return fmt.Errorf("Plan Agent 无效")
	}

	// 添加模式特定的系统提示
	systemPrompt := a.buildSystemPrompt()
	a.coreAgent.History.AddSystemMessage(systemPrompt)

	// 包装工具执行器，添加权限检查
	wrappedTools := &PlanToolWrapper{
		ToolExecutor: a.coreAgent.Tools,
		planAgent:    a,
	}

	// 临时替换工具执行器
	originalTools := a.coreAgent.Tools
	a.coreAgent.Tools = wrappedTools
	defer func() {
		a.coreAgent.Tools = originalTools
	}()

	// 执行核心 Agent
	if err := a.BaseAgent.Run(ctx, prompt); err != nil {
		return fmt.Errorf("Plan Agent 执行失败: %w", err)
	}

	return nil
}

// buildSystemPrompt 构建 Plan Agent 专用的系统提示
func (a *PlanAgent) buildSystemPrompt() string {
	return `## Plan Agent 模式

你现在是 **Plan Agent**（规划模式），只有只读访问权限。

### 你的能力
- ✅ 读取文件
- ✅ 搜索文件
- ✅ 列出文件
- ❌ 写入文件（禁止）
- ❌ 执行命令（禁止）

### 你的任务
1. 理解用户的问题
2. 阅读和分析相关代码
3. 提供详细的解释和建议
4. 如果需要修改代码，提供具体的修改方案

### 工作原则
- 专注于分析和理解
- 提供清晰的结构化说明
- 使用图表或示例帮助理解
- 如果需要修改，提供详细的步骤
- 不要尝试写入文件或执行命令

### 示例场景
- "帮我理解这个模块的架构"
- "解释这个函数的工作原理"
- "分析这个项目的依赖关系"
- "这个代码有什么问题？如何改进？"

请开始分析任务。`
}

// CanWrite 检查是否允许写入
func (a *PlanAgent) CanWrite() bool {
	return false
}

// CanExecuteCommand 检查是否允许执行命令
func (a *PlanAgent) CanExecuteCommand() bool {
	return false
}

// CanInvokeAgents 检查是否允许调用其他 Agent
func (a *PlanAgent) CanInvokeAgents() bool {
	return false
}

// GetCapabilities 获取 Plan Agent 的能力描述
func (a *PlanAgent) GetCapabilities() string {
	capabilities := []string{
		"📋 **Plan Agent** - 规划模式",
		"",
		"**权限:**",
		"  🔒 只读访问权限",
		"",
		"**允许的操作:**",
		"  📖 读取文件",
		"  🔍 搜索文件",
		"  📋 列出文件",
		"",
		"**禁止的操作:**",
		"  ❌ 写入文件",
		"  ❌ 执行命令",
		"",
		"**适用场景:**",
		"  • 代码分析",
		"  • 架构理解",
		"  • 代码审查",
		"  • 提供建议",
		"  • 文档解释",
	}

	return strings.Join(capabilities, "\n")
}

// ValidateToolCall 验证工具调用是否被允许
//
// Plan Agent 只允许只读工具
func (a *PlanAgent) ValidateToolCall(toolName string) error {
	// 检查配置中的白名单和黑名单
	if !a.config.IsToolAllowed(toolName) {
		return fmt.Errorf("Plan Agent 不允许使用工具: %s", toolName)
	}

	// 额外的安全检查
	dangerousTools := []string{"write_file", "run_command"}
	for _, dangerous := range dangerousTools {
		if toolName == dangerous {
			return fmt.Errorf("Plan Agent 模式下禁止使用 %s 工具", toolName)
		}
	}

	return nil
}

// GetSummary 获取 Agent 摘要信息
func (a *PlanAgent) GetSummary() map[string]interface{} {
	return map[string]interface{}{
		"mode":           a.mode.String(),
		"description":    a.mode.String(),
		"can_write":      a.CanWrite(),
		"can_execute":    a.CanExecuteCommand(),
		"can_invoke":     a.CanInvokeAgents(),
		"max_iterations": a.config.MaxIterations,
		"allowed_tools":  a.config.AllowedTools,
		"denied_tools":   a.config.DeniedTools,
	}
}

// PlanToolWrapper 工具执行器包装器，用于限制 Plan Agent 的权限
type PlanToolWrapper struct {
	ToolExecutor core.ToolExecutor
	planAgent    *PlanAgent
}

// Execute 执行工具调用（带权限检查）
func (w *PlanToolWrapper) Execute(ctx context.Context, call core.ToolCall) (string, error) {
	// 验证工具调用权限
	if err := w.planAgent.ValidateToolCall(call.Name); err != nil {
		// 返回错误信息而不是实际执行
		return fmt.Sprintf(`{"error": "权限错误: %s", "suggestion": "如需执行此操作，请切换到 Build Agent 模式"}`, err.Error()), nil
	}

	// 执行工具调用
	return w.ToolExecutor.Execute(ctx, call)
}
