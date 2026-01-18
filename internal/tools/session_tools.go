package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/yukin371/Kore/internal/session"
)

// SessionManager 会话管理器接口（工具使用）
type SessionManager interface {
	CreateSession(ctx context.Context, name string, agentMode session.AgentMode) (*session.Session, error)
	GetSession(sessionID string) (*session.Session, error)
	ListSessions(ctx context.Context) ([]*session.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	SwitchSession(sessionID string) (*session.Session, error)
	GetCurrentSession() (*session.Session, error)
	RenameSession(ctx context.Context, sessionID string, newName string) error
	SetSessionDescription(ctx context.Context, sessionID string, description string) error
	AddSessionTag(ctx context.Context, sessionID string, tag string) error
	RemoveSessionTag(ctx context.Context, sessionID string, tag string) error
	GetSessionStatistics(sessionID string) (session.SessionStats, error)
}

// SessionTools 会话管理工具集
type SessionTools struct {
	manager SessionManager
	mu      sync.RWMutex
}

// NewSessionTools 创建会话管理工具集
func NewSessionTools(manager SessionManager) *SessionTools {
	return &SessionTools{
		manager: manager,
	}
}

// ==================== 会话列表工具 ====================

// ListSessionsTool 列出所有会话
type ListSessionsTool struct {
	sessionTools *SessionTools
}

func (t *ListSessionsTool) Name() string {
	return "list_sessions"
}

func (t *ListSessionsTool) Description() string {
	return "列出所有会话及其基本信息"
}

func (t *ListSessionsTool) Schema() string {
	return `{
		"name": "list_sessions",
		"description": "列出所有会话及其基本信息",
		"parameters": {
			"type": "object",
			"properties": {}
		}
	}`
}

func (t *ListSessionsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	sessions, err := t.sessionTools.manager.ListSessions(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	// 构建结果
	result := make([]map[string]interface{}, 0, len(sessions))
	for _, sess := range sessions {
		stats := sess.GetStatistics()
		info := map[string]interface{}{
			"id":           sess.ID,
			"name":         sess.Name,
			"agent_mode":   sess.AgentMode,
			"status":       sess.Status,
			"created_at":   sess.CreatedAt,
			"updated_at":   sess.UpdatedAt,
			"description":  sess.GetDescription(),
			"tags":         sess.GetTags(),
			"message_count": stats.MessageCount,
			"token_used":   stats.TokenUsed,
		}
		result = append(result, info)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// ==================== 创建会话工具 ====================

// CreateSessionTool 创建新会话
type CreateSessionTool struct {
	sessionTools *SessionTools
}

func (t *CreateSessionTool) Name() string {
	return "create_session"
}

func (t *CreateSessionTool) Description() string {
	return "创建一个新的会话"
}

func (t *CreateSessionTool) Schema() string {
	return `{
		"name": "create_session",
		"description": "创建一个新的会话",
		"parameters": {
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "会话名称"
				},
				"agent_mode": {
					"type": "string",
					"enum": ["build", "plan", "general"],
					"description": "Agent 模式：build（构建）、plan（规划）、general（通用）"
				},
				"description": {
					"type": "string",
					"description": "会话描述（可选）"
				}
			},
			"required": ["name", "agent_mode"]
		}
	}`
}

func (t *CreateSessionTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name        string `json:"name"`
		AgentMode   string `json:"agent_mode"`
		Description string `json:"description,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 创建会话
	sess, err := t.sessionTools.manager.CreateSession(ctx, params.Name, session.AgentMode(params.AgentMode))
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	// 设置描述（如果提供）
	if params.Description != "" {
		if err := t.sessionTools.manager.SetSessionDescription(ctx, sess.ID, params.Description); err != nil {
			return "", fmt.Errorf("failed to set description: %w", err)
		}
	}

	// 构建结果
	result := map[string]interface{}{
		"id":         sess.ID,
		"name":       sess.Name,
		"agent_mode": sess.AgentMode,
		"status":     sess.Status,
		"created_at": sess.CreatedAt,
		"message":    "会话创建成功",
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// ==================== 切换会话工具 ====================

// SwitchSessionTool 切换到指定会话
type SwitchSessionTool struct {
	sessionTools *SessionTools
}

func (t *SwitchSessionTool) Name() string {
	return "switch_session"
}

func (t *SwitchSessionTool) Description() string {
	return "切换到指定的会话"
}

func (t *SwitchSessionTool) Schema() string {
	return `{
		"name": "switch_session",
		"description": "切换到指定的会话",
		"parameters": {
			"type": "object",
			"properties": {
				"session_id": {
					"type": "string",
					"description": "会话 ID"
				}
			},
			"required": ["session_id"]
		}
	}`
}

func (t *SwitchSessionTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	sess, err := t.sessionTools.manager.SwitchSession(params.SessionID)
	if err != nil {
		return "", fmt.Errorf("failed to switch session: %w", err)
	}

	result := map[string]interface{}{
		"id":         sess.ID,
		"name":       sess.Name,
		"agent_mode": sess.AgentMode,
		"message":    "已切换到该会话",
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// ==================== 删除会话工具 ====================

// DeleteSessionTool 删除会话
type DeleteSessionTool struct {
	sessionTools *SessionTools
}

func (t *DeleteSessionTool) Name() string {
	return "delete_session"
}

func (t *DeleteSessionTool) Description() string {
	return "删除指定的会话"
}

func (t *DeleteSessionTool) Schema() string {
	return `{
		"name": "delete_session",
		"description": "删除指定的会话（不可恢复）",
		"parameters": {
			"type": "object",
			"properties": {
				"session_id": {
					"type": "string",
					"description": "会话 ID"
				}
			},
			"required": ["session_id"]
		}
	}`
}

func (t *DeleteSessionTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	if err := t.sessionTools.manager.DeleteSession(ctx, params.SessionID); err != nil {
		return "", fmt.Errorf("failed to delete session: %w", err)
	}

	result := map[string]interface{}{
		"session_id": params.SessionID,
		"message":    "会话已删除",
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// ==================== 会话详情工具 ====================

// GetSessionDetailsTool 获取会话详情
type GetSessionDetailsTool struct {
	sessionTools *SessionTools
}

func (t *GetSessionDetailsTool) Name() string {
	return "get_session_details"
}

func (t *GetSessionDetailsTool) Description() string {
	return "获取会话的详细信息"
}

func (t *GetSessionDetailsTool) Schema() string {
	return `{
		"name": "get_session_details",
		"description": "获取会话的详细信息，包括统计信息",
		"parameters": {
			"type": "object",
			"properties": {
				"session_id": {
					"type": "string",
					"description": "会话 ID（不提供则返回当前会话）"
				}
			}
		}
	}`
}

func (t *GetSessionDetailsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		SessionID string `json:"session_id,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	var sess *session.Session
	var err error

	if params.SessionID == "" {
		// 获取当前会话
		sess, err = t.sessionTools.manager.GetCurrentSession()
	} else {
		// 获取指定会话
		sess, err = t.sessionTools.manager.GetSession(params.SessionID)
	}

	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	stats := sess.GetStatistics()

	result := map[string]interface{}{
		"id":           sess.ID,
		"name":         sess.Name,
		"agent_mode":   sess.AgentMode,
		"status":       sess.Status,
		"created_at":   sess.CreatedAt,
		"updated_at":   sess.UpdatedAt,
		"description":  sess.GetDescription(),
		"tags":         sess.GetTags(),
		"statistics": map[string]interface{}{
			"message_count":     stats.MessageCount,
			"user_msg_count":    stats.UserMsgCount,
			"assistant_msg_count": stats.AssistantMsgCount,
			"tool_call_count":   stats.ToolCallCount,
			"token_used":        stats.TokenUsed,
			"last_active_at":    stats.LastActiveAt,
		},
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// ==================== 更新会话工具 ====================

// UpdateSessionTool 更新会话信息
type UpdateSessionTool struct {
	sessionTools *SessionTools
}

func (t *UpdateSessionTool) Name() string {
	return "update_session"
}

func (t *UpdateSessionTool) Description() string {
	return "更新会话的名称、描述或标签"
}

func (t *UpdateSessionTool) Schema() string {
	return `{
		"name": "update_session",
		"description": "更新会话的名称、描述或标签",
		"parameters": {
			"type": "object",
			"properties": {
				"session_id": {
					"type": "string",
					"description": "会话 ID"
				},
				"name": {
					"type": "string",
					"description": "新名称（可选）"
				},
				"description": {
					"type": "string",
					"description": "新描述（可选）"
				},
				"add_tags": {
					"type": "array",
					"items": {
						"type": "string"
					},
					"description": "要添加的标签列表（可选）"
				},
				"remove_tags": {
					"type": "array",
					"items": {
						"type": "string"
					},
					"description": "要移除的标签列表（可选）"
				}
			},
			"required": ["session_id"]
		}
	}`
}

func (t *UpdateSessionTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		SessionID   string   `json:"session_id"`
		Name        string   `json:"name,omitempty"`
		Description string   `json:"description,omitempty"`
		AddTags     []string `json:"add_tags,omitempty"`
		RemoveTags  []string `json:"remove_tags,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	// 更新名称
	if params.Name != "" {
		if err := t.sessionTools.manager.RenameSession(ctx, params.SessionID, params.Name); err != nil {
			return "", fmt.Errorf("failed to rename session: %w", err)
		}
	}

	// 更新描述
	if params.Description != "" {
		if err := t.sessionTools.manager.SetSessionDescription(ctx, params.SessionID, params.Description); err != nil {
			return "", fmt.Errorf("failed to set description: %w", err)
		}
	}

	// 添加标签
	for _, tag := range params.AddTags {
		if err := t.sessionTools.manager.AddSessionTag(ctx, params.SessionID, tag); err != nil {
			return "", fmt.Errorf("failed to add tag: %w", err)
		}
	}

	// 移除标签
	for _, tag := range params.RemoveTags {
		if err := t.sessionTools.manager.RemoveSessionTag(ctx, params.SessionID, tag); err != nil {
			return "", fmt.Errorf("failed to remove tag: %w", err)
		}
	}

	result := map[string]interface{}{
		"session_id": params.SessionID,
		"message":    "会话更新成功",
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// ==================== 注册工具 ====================

// RegisterSessionTools 注册所有会话管理工具到工具箱
func RegisterSessionTools(toolbox *ToolBox, manager SessionManager) {
	sessionTools := NewSessionTools(manager)

	toolbox.Register(&ListSessionsTool{sessionTools: sessionTools})
	toolbox.Register(&CreateSessionTool{sessionTools: sessionTools})
	toolbox.Register(&SwitchSessionTool{sessionTools: sessionTools})
	toolbox.Register(&DeleteSessionTool{sessionTools: sessionTools})
	toolbox.Register(&GetSessionDetailsTool{sessionTools: sessionTools})
	toolbox.Register(&UpdateSessionTool{sessionTools: sessionTools})
}
