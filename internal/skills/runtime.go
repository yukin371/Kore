package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// Runtime Skill 运行时
type Runtime struct {
	registry   *Registry
	permission *PolicyEngine
	audit      *AuditLogger

	mu     sync.RWMutex
	skills map[SkillID]Skill
}

// RuntimeConfig 运行时配置
type RuntimeConfig struct {
	Registry *Registry
	Policy   *PolicyEngine
	Audit    *AuditLogger
}

// NewRuntime 创建运行时
func NewRuntime(config *RuntimeConfig) *Runtime {
	return &Runtime{
		registry:   config.Registry,
		permission: config.Policy,
		audit:      config.Audit,
		skills:     make(map[SkillID]Skill),
	}
}

// Load 加载一个 Skill
func (rt *Runtime) Load(ctx context.Context, id SkillID) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	// 检查是否已加载
	if _, ok := rt.skills[id]; ok {
		return fmt.Errorf("skill %s already loaded", id)
	}

	// 获取清单
	manifest, err := rt.registry.Get(id)
	if err != nil {
		return fmt.Errorf("failed to get manifest: %w", err)
	}

	// 检查状态
	if manifest.State != StateEnabled {
		return fmt.Errorf("skill %s is not enabled (state: %s)", id, manifest.State)
	}

	// 创建 Skill 实例
	skill, err := rt.createSkill(manifest)
	if err != nil {
		return fmt.Errorf("failed to create skill: %w", err)
	}

	// 初始化 Skill
	if err := skill.Initialize(ctx, make(map[string]string)); err != nil {
		return fmt.Errorf("failed to initialize skill: %w", err)
	}

	// 健康检查
	if err := skill.Health(ctx); err != nil {
		return fmt.Errorf("skill health check failed: %w", err)
	}

	rt.skills[id] = skill

	// 记录审计日志
	if rt.audit != nil {
		rt.audit.Log(AuditEvent{
			Type:      AuditTypeSkillLoaded,
			SkillID:   id,
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// Unload 卸载一个 Skill
func (rt *Runtime) Unload(ctx context.Context, id SkillID) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	skill, ok := rt.skills[id]
	if !ok {
		return fmt.Errorf("skill %s not loaded", id)
	}

	// 关闭 Skill
	if err := skill.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown skill: %w", err)
	}

	delete(rt.skills, id)

	// 记录审计日志
	if rt.audit != nil {
		rt.audit.Log(AuditEvent{
			Type:      AuditTypeSkillUnloaded,
			SkillID:   id,
			Timestamp: time.Now(),
			Success:   true,
		})
	}

	return nil
}

// Execute 执行 Skill 工具
func (rt *Runtime) Execute(ctx context.Context, skillID SkillID, tool string, input map[string]interface{}) (map[string]interface{}, error) {
	rt.mu.RLock()
	skill, ok := rt.skills[skillID]
	rt.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("skill %s not loaded", skillID)
	}

	// 权限检查
	if rt.permission != nil {
		manifest := skill.Manifest()
		if err := rt.permission.Check(ctx, manifest, tool, input); err != nil {
			// 记录拒绝日志
			if rt.audit != nil {
				rt.audit.Log(AuditEvent{
					Type:      AuditTypePermissionDenied,
					SkillID:   skillID,
					Tool:      tool,
					Input:     input,
					Reason:    err.Error(),
					Timestamp: time.Now(),
					Success:   false,
				})
			}
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	// 记录审计日志
	if rt.audit != nil {
		rt.audit.Log(AuditEvent{
			Type:      AuditTypeToolExecuted,
			SkillID:   skillID,
			Tool:      tool,
			Input:     input,
			Timestamp: time.Now(),
		})
	}

	// 执行工具
	output, err := skill.Execute(ctx, tool, input)

	// 记录结果
	if rt.audit != nil {
		rt.audit.Log(AuditEvent{
			Type:      AuditTypeToolCompleted,
			SkillID:   skillID,
			Tool:      tool,
			Output:    output,
			Timestamp: time.Now(),
			Success:   err == nil,
			Error:     errorMsg(err),
		})
	}

	return output, err
}

// ListTools 列出所有可用的工具
func (rt *Runtime) ListTools() []ToolInfo {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var tools []ToolInfo
	for _, skill := range rt.skills {
		manifest := skill.Manifest()
		for _, toolDef := range manifest.Tools {
			tools = append(tools, ToolInfo{
				SkillID:    manifest.ID,
				SkillName:  manifest.Name,
				ToolName:   toolDef.Name,
				Description: toolDef.Description,
				Parameters: toolDef.Parameters,
			})
		}
	}
	return tools
}

// createSkill 创建 Skill 实例
func (rt *Runtime) createSkill(manifest *SkillManifest) (Skill, error) {
	switch manifest.Type {
	case SkillTypeBuiltin:
		return NewBuiltinSkill(manifest), nil
	case SkillTypeMCP:
		return NewMCPSkill(manifest)
	case SkillTypeExternal:
		return NewExternalSkill(manifest)
	default:
		return nil, fmt.Errorf("unsupported skill type: %s", manifest.Type)
	}
}

// ToolInfo 工具信息
type ToolInfo struct {
	SkillID     SkillID             `json:"skill_id"`
	SkillName   string              `json:"skill_name"`
	ToolName    string              `json:"tool_name"`
	Description string              `json:"description"`
	Parameters  map[string]Parameter `json:"parameters"`
}

// errorMsg 从错误中提取消息
func errorMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// MCPSkill MCP 协议 Skill
type MCPSkill struct {
	*BuiltinSkill
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// NewMCPSkill 创建 MCP Skill
func NewMCPSkill(manifest *SkillManifest) (*MCPSkill, error) {
	base := NewBuiltinSkill(manifest)
	return &MCPSkill{BuiltinSkill: base}, nil
}

// Initialize 初始化 MCP Skill
func (s *MCPSkill) Initialize(ctx context.Context, config map[string]string) error {
	// 启动 MCP 服务器进程
	s.cmd = exec.CommandContext(ctx, s.manifest.EntryPoint)

	// 创建管道
	stdin, err := s.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	s.stdin = stdin
	s.stdout = stdout

	// 启动进程
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// TODO: 初始化握手

	return s.BuiltinSkill.Initialize(ctx, config)
}

// Execute 执行 MCP 工具
func (s *MCPSkill) Execute(ctx context.Context, tool string, input map[string]interface{}) (map[string]interface{}, error) {
	// 构造 JSON-RPC 请求
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      tool,
			"arguments": input,
		},
	}

	// 发送请求
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	if _, err := s.stdin.Write(data); err != nil {
		return nil, err
	}

	// 读取响应
	decoder := json.NewDecoder(s.stdout)
	var response map[string]interface{}
	if err := decoder.Decode(&response); err != nil {
		return nil, err
	}

	// 检查错误
	if errMsg, ok := response["error"]; ok {
		return nil, fmt.Errorf("MCP error: %v", errMsg)
	}

	// 返回结果
	if result, ok := response["result"]; ok {
		if resultMap, ok := result.(map[string]interface{}); ok {
			return resultMap, nil
		}
		return map[string]interface{}{"result": result}, nil
	}

	return nil, fmt.Errorf("no result in response")
}

// Shutdown 关闭 MCP Skill
func (s *MCPSkill) Shutdown(ctx context.Context) error {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	return s.BuiltinSkill.Shutdown(ctx)
}

// ExternalSkill 外部可执行文件 Skill
type ExternalSkill struct {
	*BuiltinSkill
}

// NewExternalSkill 创建外部 Skill
func NewExternalSkill(manifest *SkillManifest) (*ExternalSkill, error) {
	base := NewBuiltinSkill(manifest)
	return &ExternalSkill{BuiltinSkill: base}, nil
}

// Execute 执行外部工具
func (s *ExternalSkill) Execute(ctx context.Context, tool string, input map[string]interface{}) (map[string]interface{}, error) {
	// 构造命令
	args := []string{tool}

	// 添加输入参数（作为 JSON）
	if len(input) > 0 {
		inputJSON, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		args = append(args, string(inputJSON))
	}

	// 执行命令
	cmd := exec.CommandContext(ctx, s.manifest.EntryPoint, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	// 解析输出
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		// 如果不是 JSON，返回原始输出
		return map[string]interface{}{"output": string(output)}, nil
	}

	return result, nil
}
