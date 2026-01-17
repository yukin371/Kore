// Package agent 提供 General Agent 实现
//
// General Agent（通用模式）- 复杂任务编排
// - 权限: 完全访问权限 + 调用其他 Agent
// - 用途: 调用其他 Agent、协调任务
// - 场景: 用户说"重构这个模块并测试"
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/yukin/kore/internal/core"
	"github.com/yukin/kore/internal/types"
)

// GeneralAgent 通用模式 Agent
//
// General Agent 拥有完全访问权限，并且可以：
// - 调用 Build Agent 处理构建任务
// - 调用 Plan Agent 处理分析任务
// - 协调多个子任务
// - 编排复杂的工作流
//
// 适用于需要多个阶段协作的复杂任务
type GeneralAgent struct {
	*BaseAgent
	buildAgent *BuildAgent
	planAgent  *PlanAgent
}

// NewGeneralAgent 创建新的 General Agent
//
// 参数:
//   - coreAgent: 核心 Agent 实例
//   - projectRoot: 项目根目录
//
// 返回:
//   - *GeneralAgent: General Agent 实例
//   - error: 错误信息
func NewGeneralAgent(coreAgent *core.Agent, projectRoot string) (*GeneralAgent, error) {
	config := DefaultAgentModeConfig(types.ModeAnalyze)

	baseAgent, err := NewBaseAgent(types.ModeAnalyze, config, coreAgent)
	if err != nil {
		return nil, fmt.Errorf("创建 General Agent 失败: %w", err)
	}

	// 创建子 Agent
	buildAgent, err := NewBuildAgent(coreAgent, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 Build Agent 失败: %w", err)
	}

	planAgent, err := NewPlanAgent(coreAgent, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 Plan Agent 失败: %w", err)
	}

	return &GeneralAgent{
		BaseAgent:  baseAgent,
		buildAgent: buildAgent,
		planAgent:  planAgent,
	}, nil
}

// NewGeneralAgentWithConfig 使用自定义配置创建 General Agent
//
// 参数:
//   - coreAgent: 核心 Agent 实例
//   - config: 自定义配置
//   - projectRoot: 项目根目录
//
// 返回:
//   - *GeneralAgent: General Agent 实例
//   - error: 错误信息
func NewGeneralAgentWithConfig(coreAgent *core.Agent, config *AgentModeConfig, projectRoot string) (*GeneralAgent, error) {
	if config.Mode != types.ModeAnalyze {
		return nil, fmt.Errorf("配置模式不匹配: 期望 %s, 实际 %s", types.ModeAnalyze, config.Mode)
	}

	baseAgent, err := NewBaseAgent(types.ModeAnalyze, config, coreAgent)
	if err != nil {
		return nil, fmt.Errorf("创建 General Agent 失败: %w", err)
	}

	// 创建子 Agent
	buildAgent, err := NewBuildAgent(coreAgent, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 Build Agent 失败: %w", err)
	}

	planAgent, err := NewPlanAgent(coreAgent, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("创建 Plan Agent 失败: %w", err)
	}

	return &GeneralAgent{
		BaseAgent:  baseAgent,
		buildAgent: buildAgent,
		planAgent:  planAgent,
	}, nil
}

// Run 执行 General Agent
//
// General Agent 的执行流程：
// 1. 分析任务，判断需要哪些子任务
// 2. 可能调用 Plan Agent 进行分析
// 3. 可能调用 Build Agent 执行构建
// 4. 协调各个子任务的执行
//
// 参数:
//   - ctx: 上下文
//   - prompt: 用户提示
//
// 返回:
//   - error: 错误信息
func (a *GeneralAgent) Run(ctx context.Context, prompt string) error {
	// 验证 Agent 有效性
	if !a.IsValid() {
		return fmt.Errorf("General Agent 无效")
	}

	// 添加模式特定的系统提示
	systemPrompt := a.buildSystemPrompt()
	a.coreAgent.History.AddSystemMessage(systemPrompt)

	// 分析任务类型
	taskType := a.analyzeTask(prompt)

	switch taskType {
	case TaskTypeAnalysis:
		// 纯分析任务，委托给 Plan Agent
		a.coreAgent.UI.SendStream("\n🔄 检测到分析任务，委托给 Plan Agent...\n")
		return a.planAgent.Run(ctx, prompt)

	case TaskTypeBuild:
		// 纯构建任务，委托给 Build Agent
		a.coreAgent.UI.SendStream("\n🔧 检测到构建任务，委托给 Build Agent...\n")
		return a.buildAgent.Run(ctx, prompt)

	case TaskTypeMixed:
		// 混合任务，需要协调执行
		a.coreAgent.UI.SendStream("\n🎯 检测到复杂任务，启动协调模式...\n")
		return a.runCoordinatedTask(ctx, prompt)

	default:
		// 默认执行核心 Agent
		return a.BaseAgent.Run(ctx, prompt)
	}
}

// TaskType 任务类型
type TaskType int

const (
	// TaskTypeUnknown 未知任务类型
	TaskTypeUnknown TaskType = iota
	// TaskTypeAnalysis 分析任务（只读）
	TaskTypeAnalysis
	// TaskTypeBuild 构建任务（写入）
	TaskTypeBuild
	// TaskTypeMixed 混合任务（需要协调）
	TaskTypeMixed
)

// analyzeTask 分析任务类型
func (a *GeneralAgent) analyzeTask(prompt string) TaskType {
	promptLower := strings.ToLower(prompt)

	// 分析关键词
	analysisKeywords := []string{
		"分析", "理解", "解释", "说明", "查看", "阅读",
		"analyze", "understand", "explain", "review", "read",
	}

	// 构建关键词
	buildKeywords := []string{
		"创建", "修改", "写入", "实现", "重构", "修复",
		"create", "modify", "write", "implement", "refactor", "fix",
	}

	// 混合任务关键词
	mixedKeywords := []string{
		"并", "然后", "之后", "接着", "同时",
		"and then", "after", "also", "followed by",
	}

	hasAnalysis := false
	hasBuild := false
	hasMixed := false

	// 检查分析关键词
	for _, keyword := range analysisKeywords {
		if strings.Contains(promptLower, keyword) {
			hasAnalysis = true
			break
		}
	}

	// 检查构建关键词
	for _, keyword := range buildKeywords {
		if strings.Contains(promptLower, keyword) {
			hasBuild = true
			break
		}
	}

	// 检查混合关键词
	for _, keyword := range mixedKeywords {
		if strings.Contains(promptLower, keyword) {
			hasMixed = true
			break
		}
	}

	// 判断任务类型
	if hasMixed && (hasAnalysis || hasBuild) {
		return TaskTypeMixed
	}

	if hasAnalysis && !hasBuild {
		return TaskTypeAnalysis
	}

	if hasBuild && !hasAnalysis {
		return TaskTypeBuild
	}

	// 默认为混合任务
	return TaskTypeMixed
}

// runCoordinatedTask 执行协调任务
func (a *GeneralAgent) runCoordinatedTask(ctx context.Context, prompt string) error {
	// 第一步：使用 Plan Agent 分析
	a.coreAgent.UI.SendStream("\n📋 第一阶段：分析任务...\n")
	analysisPrompt := fmt.Sprintf("请分析以下任务，提供详细的执行计划：\n\n%s\n\n请提供：\n1. 任务理解\n2. 需要修改的文件\n3. 执行步骤\n4. 验证方法", prompt)

	if err := a.planAgent.Run(ctx, analysisPrompt); err != nil {
		return fmt.Errorf("分析阶段失败: %w", err)
	}

	// 第二步：使用 Build Agent 执行
	a.coreAgent.UI.SendStream("\n🔧 第二阶段：执行任务...\n")
	if err := a.buildAgent.Run(ctx, prompt); err != nil {
		return fmt.Errorf("执行阶段失败: %w", err)
	}

	a.coreAgent.UI.SendStream("\n✅ 任务完成！\n")
	return nil
}

// buildSystemPrompt 构建 General Agent 专用的系统提示
func (a *GeneralAgent) buildSystemPrompt() string {
	return `## General Agent 模式

你现在是 **General Agent**（通用模式），拥有完全访问权限并可以协调其他 Agent。

### 你的能力
- ✅ 读取文件
- ✅ 写入文件
- ✅ 执行命令
- ✅ 调用 Plan Agent 进行分析
- ✅ 调用 Build Agent 执行构建
- ✅ 协调复杂的多阶段任务

### 你的任务
1. 理解用户的复杂需求
2. 分解任务为多个阶段
3. 为每个阶段选择合适的 Agent
4. 协调各个阶段的执行
5. 确保整个任务顺利完成

### 工作原则
- 首先理解整个任务的范围
- 将复杂任务分解为可管理的子任务
- 为每个子任务选择最合适的工具和 Agent
- 监控每个子任务的执行状态
- 在子任务之间传递必要的上下文
- 最终汇总所有结果

### 协调策略
- **分析任务**: 委托给 Plan Agent
- **构建任务**: 委托给 Build Agent
- **混合任务**: 先分析后执行

### 示例场景
- "重构这个模块并更新测试"
- "先分析性能瓶颈，然后优化代码"
- "理解这个架构，然后添加新功能"
- "审查代码并修复发现的问题"

请开始协调任务。`
}

// CanWrite 检查是否允许写入
func (a *GeneralAgent) CanWrite() bool {
	return true
}

// CanExecuteCommand 检查是否允许执行命令
func (a *GeneralAgent) CanExecuteCommand() bool {
	return true
}

// CanInvokeAgents 检查是否允许调用其他 Agent
func (a *GeneralAgent) CanInvokeAgents() bool {
	return true
}

// GetCapabilities 获取 General Agent 的能力描述
func (a *GeneralAgent) GetCapabilities() string {
	capabilities := []string{
		"🎯 **General Agent** - 通用模式",
		"",
		"**权限:**",
		"  ✅ 完全访问权限",
		"  ✅ Agent 编排能力",
		"",
		"**允许的操作:**",
		"  📖 读取文件",
		"  ✏️  写入文件",
		"  ⚡ 执行命令",
		"  🔍 搜索文件",
		"  📋 列出文件",
		"  🤖 调用 Plan Agent",
		"  🔧 调用 Build Agent",
		"",
		"**适用场景:**",
		"  • 复杂任务编排",
		"  • 多阶段工作流",
		"  • 分析+执行",
		"  • 协调多个子任务",
	}

	return strings.Join(capabilities, "\n")
}

// ValidateToolCall 验证工具调用是否被允许
//
// General Agent 允许所有工具调用
func (a *GeneralAgent) ValidateToolCall(toolName string) error {
	// General Agent 允许所有工具
	return nil
}

// GetSummary 获取 Agent 摘要信息
func (a *GeneralAgent) GetSummary() map[string]interface{} {
	return map[string]interface{}{
		"mode":           a.mode.String(),
		"description":    a.mode.String(),
		"can_write":      a.CanWrite(),
		"can_execute":    a.CanExecuteCommand(),
		"can_invoke":     a.CanInvokeAgents(),
		"max_iterations": a.config.MaxIterations,
		"allowed_tools":  a.config.AllowedTools,
		"denied_tools":   a.config.DeniedTools,
		"sub_agents": []string{
			a.buildAgent.GetMode().String(),
			a.planAgent.GetMode().String(),
		},
	}
}
