// Package agent 提供关键词检测功能
//
// 检测提示中的关键词并自动激活专门模式
// 灵感来自: https://github.com/code-yeongyu/oh-my-opencode
package agent

import (
	"strings"

	"github.com/yukin/kore/internal/types"
)

// KeywordDetector 关键词检测器
type KeywordDetector struct {
	keywords map[types.AgentMode]string
}

// NewKeywordDetector 创建关键词检测器
func NewKeywordDetector() *KeywordDetector {
	return &KeywordDetector{
		keywords: map[types.AgentMode]string{
			// ultrawork 及其缩写
			types.ModeUltraWork: "ultrawork|ulw|超级工作|最大化性能|开足马力",

			// search 及其变体
			types.ModeSearch: "search|find|搜索|查找|检索",

			// analyze 及其变体
			types.ModeAnalyze: "analyze|investigate|分析|调查|深度分析",
		},
	}
}

// Detect 检测提示中的关键词
func (kd *KeywordDetector) Detect(prompt string) (types.AgentMode, bool) {
	if prompt == "" {
		return types.ModeNormal, false
	}

	// 转换为小写进行匹配
	promptLower := strings.ToLower(prompt)

	// 检查所有关键词
	for mode, keywords := range kd.keywords {
		// 分割关键词（用 | 分隔）
		keywordList := strings.Split(keywords, "|")
		for _, keyword := range keywordList {
			if strings.Contains(promptLower, strings.ToLower(keyword)) {
				return mode, true
			}
		}
	}

	return types.ModeNormal, false
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
	Mode     types.AgentMode
	Detected bool
	Keyword  string
}

func (kd *KeywordDetector) extractMatchedKeyword(prompt string) string {
	_, detected := kd.Detect(prompt)
	if !detected {
		return ""
	}

	promptLower := strings.ToLower(prompt)
	for mode, keywords := range kd.keywords {
		keywordList := strings.Split(keywords, "|")
		for _, keyword := range keywordList {
			if strings.Contains(promptLower, strings.ToLower(keyword)) {
				return keyword
			}
		}
		_ = mode // 避免未使用变量警告
	}
	return ""
}
