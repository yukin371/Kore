// Package agent 提供多 Agent 系统实现
//
// 支持三种不同的 Agent 模式（已废弃，请使用 keyword_detector.go 中的模式）：
// - Build Agent: 构建模式，完全访问权限
// - Plan Agent: 规划模式，只读访问权限
// - General Agent: 通用模式，复杂任务编排
//
// 新的模式系统（keyword_detector.go）支持：
// - Normal: 正常模式
// - UltraWork: 最大性能模式，并行智能体编排
// - Search: 搜索模式，最大化搜索力度
// - Analyze: 分析模式，深度分析
package agent

import (
	"context"
	"fmt"

	"github.com/yukin/kore/internal/core"
	"github.com/yukin/kore/internal/types"
)

// AgentModeConfig Agent 模式配置
type AgentModeConfig struct {
	// Mode Agent 模式
	Mode types.AgentMode

	// AllowedTools 允许使用的工具列表（空表示允许所有工具）
	AllowedTools []string

	// DeniedTools 禁止使用的工具列表（优先级高于 AllowedTools）
	DeniedTools []string

	// MaxIterations 最大迭代次数（用于 General Agent）
	MaxIterations int
}

// DefaultAgentModeConfig 返回默认的模式配置
// 注意：此函数保留用于向后兼容，建议使用 keyword_detector 中的模式配置
func DefaultAgentModeConfig(mode types.AgentMode) *AgentModeConfig {
	switch mode {
	case types.ModeUltraWork:
		return &AgentModeConfig{
			Mode:          types.ModeUltraWork,
			AllowedTools:  []string{}, // 允许所有工具
			DeniedTools:   []string{},
			MaxIterations: 100,
		}

	case types.ModeSearch:
		return &AgentModeConfig{
			Mode:          types.ModeSearch,
			AllowedTools:  []string{"read_file", "list_files", "search_files"},
			DeniedTools:   []string{"write_file", "run_command"},
			MaxIterations: 30,
		}

	case types.ModeAnalyze:
		return &AgentModeConfig{
			Mode:          types.ModeAnalyze,
			AllowedTools:  []string{}, // 允许所有工具
			DeniedTools:   []string{},
			MaxIterations: 50,
		}

	default: // ModeNormal
		return &AgentModeConfig{
			Mode:          types.ModeNormal,
			AllowedTools:  []string{},
			DeniedTools:   []string{},
			MaxIterations: 50,
		}
	}
}

// Validate 验证配置
func (c *AgentModeConfig) Validate() error {
	validModes := map[types.AgentMode]bool{
		types.ModeNormal:    true,
		types.ModeUltraWork: true,
		types.ModeSearch:    true,
		types.ModeAnalyze:   true,
	}

	if !validModes[c.Mode] {
		return fmt.Errorf("无效的 Agent 模式: %s", c.Mode)
	}

	if c.MaxIterations <= 0 {
		return fmt.Errorf("最大迭代次数必须大于 0")
	}

	// 验证 Search 模式的配置
	if c.Mode == types.ModeSearch {
		for _, tool := range c.AllowedTools {
			if tool == "write_file" || tool == "run_command" {
				return fmt.Errorf("Search 模式不允许使用写入或执行工具: %s", tool)
			}
		}
	}

	return nil
}

// IsToolAllowed 检查工具是否允许使用
func (c *AgentModeConfig) IsToolAllowed(toolName string) bool {
	// 检查黑名单
	for _, denied := range c.DeniedTools {
		if denied == toolName {
			return false
		}
	}

	// 如果白名单为空，允许所有工具
	if len(c.AllowedTools) == 0 {
		return true
	}

	// 检查白名单
	for _, allowed := range c.AllowedTools {
		if allowed == toolName {
			return true
		}
	}

	return false
}

// AgentFactory Agent 工厂接口
type AgentFactory interface {
	// CreateAgent 创建指定模式的 Agent
	CreateAgent(mode types.AgentMode, config *AgentModeConfig) (Agent, error)
}

// Agent Agent 接口
type Agent interface {
	// GetMode 获取 Agent 模式
	GetMode() types.AgentMode

	// GetConfig 获取 Agent 配置
	GetConfig() *AgentModeConfig

	// Run 执行 Agent
	Run(ctx context.Context, prompt string) error

	// IsValid 检查 Agent 是否有效（权限检查）
	IsValid() bool
}

// BaseAgent 基础 Agent 实现
type BaseAgent struct {
	mode      types.AgentMode
	config    *AgentModeConfig
	coreAgent *core.Agent
}

// NewBaseAgent 创建基础 Agent
func NewBaseAgent(mode types.AgentMode, config *AgentModeConfig, coreAgent *core.Agent) (*BaseAgent, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &BaseAgent{
		mode:      mode,
		config:    config,
		coreAgent: coreAgent,
	}, nil
}

// GetMode 获取 Agent 模式
func (a *BaseAgent) GetMode() types.AgentMode {
	return a.mode
}

// GetConfig 获取 Agent 配置
func (a *BaseAgent) GetConfig() *AgentModeConfig {
	return a.config
}

// IsValid 检查 Agent 是否有效
func (a *BaseAgent) IsValid() bool {
	if a.coreAgent == nil {
		return false
	}

	if a.config == nil {
		return false
	}

	return true
}

// Run 执行 Agent（基础实现，由具体 Agent 覆盖）
func (a *BaseAgent) Run(ctx context.Context, prompt string) error {
	return a.coreAgent.Run(ctx, prompt)
}

// CreateAgent 根据 Agent 模式创建对应的 Agent 实例
//
// 参数:
//   - mode: Agent 模式 (ModeBuild, ModePlan, ModeGeneral)
//   - coreAgent: 核心 Agent 实例
//   - projectRoot: 项目根目录
//
// 返回:
//   - Agent: Agent 实例
//   - error: 错误信息
//
// 示例:
//   agent, err := CreateAgent(types.ModeBuild, coreAgent, "/path/to/project")
//   if err != nil {
//       log.Fatal(err)
//   }
func CreateAgent(mode types.AgentMode, coreAgent *core.Agent, projectRoot string) (Agent, error) {
	switch mode {
	case types.ModeUltraWork:
		// Build Agent - 构建模式，完全访问权限
		return NewBuildAgent(coreAgent, projectRoot)

	case types.ModeSearch:
		// Plan Agent - 规划模式，只读访问权限
		return NewPlanAgent(coreAgent, projectRoot)

	case types.ModeAnalyze:
		// General Agent - 通用模式，复杂任务编排
		return NewGeneralAgent(coreAgent, projectRoot)

	default:
		// 默认使用 Normal 模式（等同于 Build Agent，但配置更保守）
		config := DefaultAgentModeConfig(types.ModeNormal)
		baseAgent, err := NewBaseAgent(types.ModeNormal, config, coreAgent)
		if err != nil {
			return nil, err
		}
		return &BuildAgent{
			BaseAgent: baseAgent,
		}, nil
	}
}

// CreateAgentWithConfig 使用自定义配置创建 Agent
//
// 参数:
//   - mode: Agent 模式
//   - coreAgent: 核心 Agent 实例
//   - config: 自定义配置
//   - projectRoot: 项目根目录
//
// 返回:
//   - Agent: Agent 实例
//   - error: 错误信息
func CreateAgentWithConfig(mode types.AgentMode, coreAgent *core.Agent, config *AgentModeConfig, projectRoot string) (Agent, error) {
	switch mode {
	case types.ModeUltraWork:
		return NewBuildAgentWithConfig(coreAgent, config)

	case types.ModeSearch:
		return NewPlanAgentWithConfig(coreAgent, config)

	case types.ModeAnalyze:
		return NewGeneralAgentWithConfig(coreAgent, config, projectRoot)

	default:
		return NewBuildAgentWithConfig(coreAgent, config)
	}
}

// AgentCapabilities 定义 Agent 能力接口
type AgentCapabilities interface {
	// CanWrite 是否允许写入文件
	CanWrite() bool

	// CanExecuteCommand 是否允许执行命令
	CanExecuteCommand() bool

	// CanInvokeAgents 是否允许调用其他 Agent
	CanInvokeAgents() bool

	// GetCapabilities 获取能力描述
	GetCapabilities() string

	// ValidateToolCall 验证工具调用权限
	ValidateToolCall(toolName string) error

	// GetSummary 获取 Agent 摘要
	GetSummary() map[string]interface{}
}
