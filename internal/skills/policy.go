package skills

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PolicyEngine 权限策略引擎
type PolicyEngine struct {
	mu          sync.RWMutex
	policies    map[SkillID][]PermissionPolicy
	defaultDeny bool
	audit       *AuditLogger
}

// PermissionPolicy 权限策略
type PermissionPolicy struct {
	Type      PermissionType `json:"type"`
	Resource  string         `json:"resource"`  // 支持通配符
	Action    string         `json:"action"`    // read, write, execute, *
	Allow     bool           `json:"allow"`      // true=允许, false=拒绝
	Reason    string         `json:"reason"`     // 拒绝原因
	ExpiresAt *time.Time     `json:"expires_at,omitempty"` // 过期时间
}

// NewPolicyEngine 创建策略引擎
func NewPolicyEngine(audit *AuditLogger) *PolicyEngine {
	return &PolicyEngine{
		policies:    make(map[SkillID][]PermissionPolicy),
		defaultDeny: true, // 默认拒绝策略
		audit:       audit,
	}
}

// SetPolicy 设置 Skill 的权限策略
func (pe *PolicyEngine) SetPolicy(skillID SkillID, policies []PermissionPolicy) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// 过滤掉已过期的策略
	validPolicies := make([]PermissionPolicy, 0, len(policies))
	now := time.Now()
	for _, policy := range policies {
		if policy.ExpiresAt == nil || policy.ExpiresAt.After(now) {
			validPolicies = append(validPolicies, policy)
		}
	}

	pe.policies[skillID] = validPolicies
}

// Check 检查权限
func (pe *PolicyEngine) Check(ctx context.Context, manifest *SkillManifest, tool string, input map[string]interface{}) error {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// 获取策略
	policies, ok := pe.policies[manifest.ID]
	if !ok || len(policies) == 0 {
		// 没有策略，检查是否使用默认拒绝
		if pe.defaultDeny {
			return fmt.Errorf("no permission policy configured for skill %s (default deny)", manifest.ID)
		}
		return nil
	}

	// 检查每个策略
	for _, policy := range policies {
		// 检查是否匹配
		if !pe.matchesPolicy(policy, manifest, tool, input) {
			continue
		}

		// 匹配到策略
		if policy.Allow {
			return nil // 允许
		}

		// 拒绝
		return fmt.Errorf("permission denied: %s", policy.Reason)
	}

	// 没有匹配的策略
	if pe.defaultDeny {
		return fmt.Errorf("no matching permission policy (default deny)")
	}

	return nil
}

// matchesPolicy 检查是否匹配策略
func (pe *PolicyEngine) matchesPolicy(policy PermissionPolicy, manifest *SkillManifest, tool string, input map[string]interface{}) bool {
	// 检查类型
	declaredPerms := manifest.Permissions
	hasPermission := false
	for _, perm := range declaredPerms {
		if perm.Type == policy.Type {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		return false
	}

	// 检查资源匹配（支持通配符）
	if policy.Resource != "*" {
		resource := pe.extractResource(tool, input, policy.Type)
		if !pe.matchResource(policy.Resource, resource) {
			return false
		}
	}

	// 检查动作
	if policy.Action != "*" {
		action := pe.extractAction(tool, input, policy.Type)
		if policy.Action != action {
			return false
		}
	}

	return true
}

// extractResource 从输入中提取资源
func (pe *PolicyEngine) extractResource(tool string, input map[string]interface{}, permType PermissionType) string {
	switch permType {
	case PermissionFilesystem:
		if path, ok := input["path"].(string); ok {
			return path
		}
		if file, ok := input["file"].(string); ok {
			return file
		}
		return ""
	case PermissionCommand:
		if cmd, ok := input["command"].(string); ok {
			return cmd
		}
		return tool
	case PermissionNetwork:
		if url, ok := input["url"].(string); ok {
			return url
		}
		return ""
	default:
		return ""
	}
}

// extractAction 从输入中提取动作
func (pe *PolicyEngine) extractAction(tool string, input map[string]interface{}, permType PermissionType) string {
	// 尝试从 input 中获取 action
	if action, ok := input["action"].(string); ok {
		return action
	}

	// 根据工具名推断动作
	switch permType {
	case PermissionFilesystem:
		if strings.Contains(tool, "read") {
			return "read"
		}
		if strings.Contains(tool, "write") {
			return "write"
		}
		return "read"
	case PermissionCommand:
		return "execute"
	default:
		return "*"
	}
}

// matchResource 匹配资源（支持通配符）
func (pe *PolicyEngine) matchResource(pattern, resource string) bool {
	if pattern == "*" {
		return true
	}

	if strings.Contains(pattern, "*") {
		// 简单的通配符匹配
		patternParts := strings.Split(pattern, "*")
		for _, part := range patternParts {
			if !strings.Contains(resource, part) {
				return false
			}
		}
		return true
	}

	// 精确匹配
	return pattern == resource || pe.isSubPath(pattern, resource)
}

// isSubPath 检查是否为子路径
func (pe *PolicyEngine) isSubPath(parent, child string) bool {
	rel := filepath.Clean(child)
	if filepath.IsAbs(rel) {
		return false
	}
	base := filepath.Clean(parent)
	return strings.HasPrefix(rel, base)
}

// AuditType 审计事件类型
type AuditType string

const (
	AuditTypeSkillLoaded      AuditType = "skill.loaded"
	AuditTypeSkillUnloaded    AuditType = "skill.unloaded"
	AuditTypeToolExecuted     AuditType = "tool.executed"
	AuditTypeToolCompleted    AuditType = "tool.completed"
	AuditTypePermissionDenied AuditType = "permission.denied"
	AuditTypePermissionGranted AuditType = "permission.granted"
)

// AuditEvent 审计事件
type AuditEvent struct {
	Type      AuditType            `json:"type"`
	SkillID   SkillID              `json:"skill_id"`
	Tool      string               `json:"tool,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Reason    string               `json:"reason,omitempty"`
	Error     string               `json:"error,omitempty"`
	Timestamp time.Time            `json:"timestamp"`
	Success   bool                 `json:"success"`
}

// AuditLogger 审计日志记录器
type AuditLogger struct {
	mu     sync.Mutex
	events []AuditEvent
	config *AuditConfig
}

// AuditConfig 审计配置
type AuditConfig struct {
	MaxEvents int  `json:"max_events"` // 最大事件数量
	EnableLog  bool `json:"enable_log"` // 启用日志
	LogPath    string `json:"log_path"` // 日志文件路径
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(config *AuditConfig) *AuditLogger {
	if config == nil {
		config = &AuditConfig{
			MaxEvents: 1000,
			EnableLog: false,
		}
	}

	return &AuditLogger{
		events: make([]AuditEvent, 0, config.MaxEvents),
		config: config,
	}
}

// Log 记录审计事件
func (al *AuditLogger) Log(event AuditEvent) {
	al.mu.Lock()
	defer al.mu.Unlock()

	// 添加到内存
	al.events = append(al.events, event)

	// 限制事件数量
	if len(al.events) > al.config.MaxEvents {
		// 删除最旧的事件
		al.events = al.events[1:]
	}

	// 写入文件（如果启用）
	if al.config.EnableLog && al.config.LogPath != "" {
		al.writeToFile(event)
	}
}

// GetEvents 获取审计事件
func (al *AuditLogger) GetEvents(skillID SkillID, limit int) []AuditEvent {
	al.mu.Lock()
	defer al.mu.Unlock()

	if skillID == "" {
		// 返回所有事件
		if limit > 0 && len(al.events) > limit {
			return al.events[len(al.events)-limit:]
		}
		return al.events
	}

	// 过滤特定 Skill 的事件
	var filtered []AuditEvent
	for _, event := range al.events {
		if event.SkillID == skillID {
			filtered = append(filtered, event)
		}
	}

	if limit > 0 && len(filtered) > limit {
		return filtered[len(filtered)-limit:]
	}
	return filtered
}

// writeToFile 写入日志文件
func (al *AuditLogger) writeToFile(event AuditEvent) {
	// TODO: 实现文件写入
	_ = event
}
