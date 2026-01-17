// Package agent 提供关键词检测功能
//
// 检测提示中的关键词并自动激活专门模式
// 灵感来自: https://github.com/code-yeongyu/oh-my-opencode
package agent

import (
	"strings"
)

// KeywordDetector 关键词检测器
type KeywordDetector struct {
	keywords map[string]AgentMode
}

// AgentMode Agent 模式
type AgentMode string

const (
	ModeNormal   AgentMode = "normal"
	ModeUltraWork AgentMode = "ultrawork"  // 最大性能模式
	ModeSearch   AgentMode = "search"     // 搜索模式
	ModeAnalyze  AgentMode = "analyze"    // 分析模式
)

// NewKeywordDetector 创建关键词检测器
func NewKeywordDetector() *KeywordDetector {
	return &KeywordDetector{
		keywords: map[string]AgentMode{
			// ultrawork 及其缩写
			"ultrawork":     ModeUltraWork,
			"ulw":           ModeUltraWork,
			"超级工作":      ModeUltraWork,
			"最大化性能":    ModeUltraWork,
			"开足马力":      ModeUltraWork,

			// search 及其变体
			"search":       ModeSearch,
			"find":         ModeSearch,
			"搜索":         ModeSearch,
			"查找":         ModeSearch,
			"检索":         ModeSearch,

			// analyze 及其变体
			"analyze":      ModeAnalyze,
			"investigate":   ModeAnalyze,
			"分析":         ModeAnalyze,
			"调查":         ModeAnalyze,
		"深度分析":     ModeAnalyze,
	},
	}
}

// Detect 检测提示中的关键词
func (kd *KeywordDetector) Detect(prompt string) (AgentMode, bool) {
	if prompt == "" {
		return ModeNormal, false
	}

	// 转换为小写进行匹配
	promptLower := strings.ToLower(prompt)

	// 检查所有关键词
	for keyword, mode := range kd.keywords {
		if strings.Contains(promptLower, strings.ToLower(keyword)) {
			return mode, true
		}
	}

	return ModeNormal, false
}

// DetectWithDetails 检测并返回详细信息
func (kd *KeywordDetector) DetectWithDetails(prompt string) *KeywordMatch {
	mode, detected := kd.Detect(prompt)

	return &KeywordMatch{
		Mode:     mode,
	Detected: detected,
	Keyword:  kd.extractMatchedKeyword(prompt),
	}
}

// KeywordMatch 关键词匹配结果
type KeywordMatch struct {
	Mode     AgentMode
	Detected bool
	Keyword string
}

func (kd *KeywordDetector) extractMatchedKeyword(prompt string) string {
	if !kd.Detect(prompt) {
		return ""
	}

	promptLower := strings.ToLower(prompt)
	for keyword := range kd.keywords {
		if strings.Contains(promptLower, strings.ToLower(keyword)) {
			return keyword
		}
	}
	return ""
}

// GetModeDescription 获取模式描述
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

// GetModeConfiguration 获取模式对应的配置调整
func (am AgentMode) GetConfiguration() *ModeConfiguration {
	switch am {
	case ModeUltraWork:
		return &ModeConfiguration{
			ParallelTools:        true,
			MaxConcurrency:       10,
			AggressiveSearch:     true,
			UseAllSpecialists:    true,
			ContextStrategy:      "minimal", // 最小化上下文
			TemperatureBoost:      0.1,    // 提高创造性
		}

	case ModeSearch:
		return &ModeConfiguration{
			ParallelTools:        true,
			MaxConcurrency:       5,
			AggressiveSearch:     true,
			UseSpecialists:       []string{"explore", "librarian"},
			ContextStrategy:      "search_focused",
			EnableASTGrep:        true,
		}

	case ModeAnalyze:
		return &ModeConfiguration{
			ParallelTools:        false,
			MaxConcurrency:       1,
			AggressiveSearch:     false,
			UseSpecialists:       []string{"oracle"},
			ContextStrategy:      "comprehensive",
			EnableDeepThinking:    true,
			ReasoningEffort:     "high",
		}

	default:
		return &ModeConfiguration{
			ParallelTools:        false,
			MaxConcurrency:       1,
			AggressiveSearch:     false,
			UseAllSpecialists:    false,
			ContextStrategy:      "balanced",
		}
}

// ModeConfiguration 模式配置
type ModeConfiguration struct {
	ParallelTools        bool              // 是否并行执行工具
	MaxConcurrency       int               // 最大并发数
	AggressiveSearch     bool              // 激进搜索
	UseAllSpecialists    bool              // 使用所有专业智能体
	UseSpecialists       []string          // 指定使用的智能体
	ContextStrategy      string            // 上下文策略
	TemperatureBoost      float32           // 温度提升
	EnableASTGrep        bool              // 启用 AST Grep
	EnableDeepThinking    bool              // 启用深度思考
	ReasoningEffort     string            // 推理努力度
}
