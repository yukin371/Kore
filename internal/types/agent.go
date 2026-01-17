// Package types provides shared types used across different packages
package types

// AgentMode represents different agent operation modes
type AgentMode string

const (
	ModeNormal    AgentMode = "normal"
	ModeUltraWork AgentMode = "ultrawork" // 最大性能模式
	ModeSearch    AgentMode = "search"    // 搜索模式
	ModeAnalyze   AgentMode = "analyze"   // 分析模式
)

// String returns the description of the mode
func (am AgentMode) String() string {
	switch am {
	case ModeUltraWork:
		return "超级工作模式 (ultrawork) - 最大性能，并行智能体编排"
	case ModeSearch:
		return "搜索模式 - 最大化搜索力度，并行 explore 和 librarian"
	case ModeAnalyze:
		return "分析模式 - 深度分析，多阶段专家咨询"
	default:
		return "正常模式"
	}
}

// ModeConfiguration 模式配置
type ModeConfiguration struct {
	ParallelTools     bool     // 是否并行执行工具
	MaxConcurrency    int      // 最大并发数
	AggressiveSearch  bool     // 激进搜索
	UseAllSpecialists bool     // 使用所有专业智能体
	UseSpecialists    []string // 指定使用的智能体
	ContextStrategy   string   // 上下文策略
	TemperatureBoost  float32  // 温度提升
	EnableASTGrep     bool     // 启用 AST Grep
	EnableDeepThinking bool    // 启用深度思考
	ReasoningEffort   string   // 推理努力度
}

// GetConfiguration 获取模式对应的配置调整
func (am AgentMode) GetConfiguration() *ModeConfiguration {
	switch am {
	case ModeUltraWork:
		return &ModeConfiguration{
			ParallelTools:     true,
			MaxConcurrency:    10,
			AggressiveSearch:  true,
			UseAllSpecialists: true,
			ContextStrategy:   "minimal", // 最小化上下文
			TemperatureBoost:  0.1,       // 提高创造性
		}

	case ModeSearch:
		return &ModeConfiguration{
			ParallelTools:    true,
			MaxConcurrency:   5,
			AggressiveSearch: true,
			UseSpecialists:   []string{"explore", "librarian"},
			ContextStrategy:  "search_focused",
			EnableASTGrep:    true,
		}

	case ModeAnalyze:
		return &ModeConfiguration{
			ParallelTools:     false,
			MaxConcurrency:    1,
			AggressiveSearch:  false,
			UseSpecialists:    []string{"oracle"},
			ContextStrategy:   "comprehensive",
			EnableDeepThinking: true,
			ReasoningEffort:   "high",
		}

	default:
		return &ModeConfiguration{
			ParallelTools:     false,
			MaxConcurrency:    1,
			AggressiveSearch:  false,
			UseAllSpecialists: false,
			ContextStrategy:   "balanced",
		}
	}
}
