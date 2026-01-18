// Package agent 提供 Build Agent 实现
//
// Build Agent（构建模式）- 完全访问权限
// - 权限: 完全访问权限
// - 用途: 创建文件、修改代码、执行命令
// - 场景: 用户说"帮我创建一个新功能"
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/yukin371/Kore/internal/core"
	"github.com/yukin371/Kore/internal/types"
)

// BuildAgent 构建模式 Agent
//
// Build Agent 拥有完全访问权限，可以：
// - 读取文件
// - 写入文件
// - 执行命令
// - 搜索文件
// - 列出文件
//
// 适用于需要实际修改代码或执行命令的任务
type BuildAgent struct {
	*BaseAgent
}

// NewBuildAgent 创建新的 Build Agent
//
// 参数:
//   - coreAgent: 核心 Agent 实例
//   - projectRoot: 项目根目录
//
// 返回:
//   - *BuildAgent: Build Agent 实例
//   - error: 错误信息
func NewBuildAgent(coreAgent *core.Agent, projectRoot string) (*BuildAgent, error) {
	config := DefaultAgentModeConfig(types.ModeUltraWork)

	baseAgent, err := NewBaseAgent(types.ModeUltraWork, config, coreAgent)
	if err != nil {
		return nil, fmt.Errorf("创建 Build Agent 失败: %w", err)
	}

	return &BuildAgent{
		BaseAgent: baseAgent,
	}, nil
}

// NewBuildAgentWithConfig 使用自定义配置创建 Build Agent
//
// 参数:
//   - coreAgent: 核心 Agent 实例
//   - config: 自定义配置
//
// 返回:
//   - *BuildAgent: Build Agent 实例
//   - error: 错误信息
func NewBuildAgentWithConfig(coreAgent *core.Agent, config *AgentModeConfig) (*BuildAgent, error) {
	if config.Mode != types.ModeUltraWork {
		return nil, fmt.Errorf("配置模式不匹配: 期望 %s, 实际 %s", types.ModeUltraWork, config.Mode)
	}

	baseAgent, err := NewBaseAgent(types.ModeUltraWork, config, coreAgent)
	if err != nil {
		return nil, fmt.Errorf("创建 Build Agent 失败: %w", err)
	}

	return &BuildAgent{
		BaseAgent: baseAgent,
	}, nil
}

// Run 执行 Build Agent
//
// Build Agent 的执行流程：
// 1. 检查工具权限（允许所有工具）
// 2. 执行核心 Agent 逻辑
// 3. 处理工具调用
//
// 参数:
//   - ctx: 上下文
//   - prompt: 用户提示
//
// 返回:
//   - error: 错误信息
func (a *BuildAgent) Run(ctx context.Context, prompt string) error {
	// 验证 Agent 有效性
	if !a.IsValid() {
		return fmt.Errorf("Build Agent 无效")
	}

	// 添加模式特定的系统提示
	systemPrompt := a.buildSystemPrompt()
	a.coreAgent.History.AddSystemMessage(systemPrompt)

	// 执行核心 Agent
	if err := a.BaseAgent.Run(ctx, prompt); err != nil {
		return fmt.Errorf("Build Agent 执行失败: %w", err)
	}

	return nil
}

// buildSystemPrompt 构建 Build Agent 专用的系统提示
func (a *BuildAgent) buildSystemPrompt() string {
	return `## Build Agent 模式

你现在是 **Build Agent**（构建模式），拥有完全访问权限。

### 你的能力
- ✅ 读取文件
- ✅ 写入文件
- ✅ 执行命令
- ✅ 搜索文件
- ✅ 列出文件

### 你的任务
1. 理解用户的需求
2. 分析现有代码
3. 创建或修改文件
4. 执行必要的命令
5. 验证修改是否正确

### 工作原则
- 仔细分析现有代码结构
- 一次性完成所有相关修改
- 使用工具验证修改（如运行测试）
- 如果不确定，先询问用户
- 完成后总结修改内容

### 示例场景
- "帮我创建一个新的用户认证模块"
- "重构这个函数，提高性能"
- "修复这个 bug 并添加测试"
- "更新文档以反映最新的 API 变更"

请开始执行任务。`
}

// CanWrite 检查是否允许写入
func (a *BuildAgent) CanWrite() bool {
	return true
}

// CanExecuteCommand 检查是否允许执行命令
func (a *BuildAgent) CanExecuteCommand() bool {
	return true
}

// CanInvokeAgents 检查是否允许调用其他 Agent
func (a *BuildAgent) CanInvokeAgents() bool {
	return false
}

// GetCapabilities 获取 Build Agent 的能力描述
func (a *BuildAgent) GetCapabilities() string {
	capabilities := []string{
		"🔧 **Build Agent** - 构建模式",
		"",
		"**权限:**",
		"  ✅ 完全访问权限",
		"",
		"**允许的操作:**",
		"  📖 读取文件",
		"  ✏️  写入文件",
		"  ⚡ 执行命令",
		"  🔍 搜索文件",
		"  📋 列出文件",
		"",
		"**适用场景:**",
		"  • 创建新功能",
		"  • 修改代码",
		"  • 重构模块",
		"  • 修复 Bug",
		"  • 运行测试",
	}

	return strings.Join(capabilities, "\n")
}

// ValidateToolCall 验证工具调用是否被允许
//
// Build Agent 允许所有工具调用
func (a *BuildAgent) ValidateToolCall(toolName string) error {
	// Build Agent 允许所有工具
	return nil
}

// GetSummary 获取 Agent 摘要信息
func (a *BuildAgent) GetSummary() map[string]interface{} {
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
