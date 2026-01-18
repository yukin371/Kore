package skills

import (
	"context"
	"fmt"
	"time"
)

// SkillID 唯一标识一个 Skill
type SkillID string

// SkillVersion Skill 版本号（语义化版本）
type SkillVersion string

// SkillState Skill 状态
type SkillState int

const (
	// StateInstalled 已安装
	StateInstalled SkillState = iota
	// StateEnabled 已启用
	StateEnabled
	// StateDisabled 已禁用
	StateDisabled
	// StateError 错误状态
	StateError
)

func (s SkillState) String() string {
	switch s {
	case StateInstalled:
		return "installed"
	case StateEnabled:
		return "enabled"
	case StateDisabled:
		return "disabled"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// SkillManifest Skill 清单（描述文件）
type SkillManifest struct {
	// 基本信息
	ID          SkillID      `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"`
	Version     SkillVersion `json:"version" yaml:"version"`
	Description string       `json:"description" yaml:"description"`
	Author      string       `json:"author" yaml:"author"`
	License     string       `json:"license" yaml:"license"`
	Homepage    string       `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	Repository  string       `json:"repository,omitempty" yaml:"repository,omitempty"`

	// 入口与类型
	Type        SkillType `json:"type" yaml:"type"`                 // builtin, mcp, external
	EntryPoint  string    `json:"entry_point" yaml:"entry_point"`   // 可执行文件或脚本路径
	Interpreter string    `json:"interpreter,omitempty" yaml:"interpreter,omitempty"` // python, node, bash 等

	// 依赖与兼容性
	KoreVersion  string         `json:"kore_version" yaml:"kore_version"`   // 最低 Kore 版本
	Dependencies []SkillDependency `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`

	// 权限声明
	Permissions []Permission `json:"permissions" yaml:"permissions"`

	// 工具定义（builtin 类型使用）
	Tools []ToolDefinition `json:"tools,omitempty" yaml:"tools,omitempty"`

	// 安装信息（运行时填充）
	InstalledAt  time.Time `json:"installed_at,omitempty" yaml:"-"`
	UpdatedAt    time.Time `json:"updated_at,omitempty" yaml:"-"`
	State        SkillState `json:"state,omitempty" yaml:"-"`
}

// SkillType Skill 类型
type SkillType string

const (
	// SkillTypeBuiltin 内置技能（Go 代码实现）
	SkillTypeBuiltin SkillType = "builtin"
	// SkillTypeMCP MCP 协议插件
	SkillTypeMCP SkillType = "mcp"
	// SkillTypeExternal 外部可执行文件
	SkillTypeExternal SkillType = "external"
)

// SkillDependency 依赖声明
type SkillDependency struct {
	ID      SkillID      `json:"id" yaml:"id"`
	Version SkillVersion `json:"version" yaml:"version"` // 最低版本
}

// Permission 权限声明
type Permission struct {
	Type     PermissionType `json:"type" yaml:"type"`
	Resource string        `json:"resource" yaml:"resource"` // 路径、命令名、API 资源等
	Action   string        `json:"action,omitempty" yaml:"action,omitempty"` // read, write, execute 等
	Reason   string        `json:"reason,omitempty" yaml:"reason,omitempty"`   // 为什么需要此权限
}

// PermissionType 权限类型
type PermissionType string

const (
	// PermissionFilesystem 文件系统访问
	PermissionFilesystem PermissionType = "filesystem"
	// PermissionCommand 命令执行
	PermissionCommand PermissionType = "command"
	// PermissionNetwork 网络访问
	PermissionNetwork PermissionType = "network"
	// PermissionLLM LLM 调用
	PermissionLLM PermissionType = "llm"
	// PermissionSession 会话访问
	PermissionSession PermissionType = "session"
)

// ToolDefinition 工具定义（用于 builtin Skill）
type ToolDefinition struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Parameters  map[string]Parameter   `json:"parameters" yaml:"parameters"`
 Handler     string                 `json:"handler" yaml:"handler"` // 处理函数名
}

// Parameter 参数定义
type Parameter struct {
	Type        string   `json:"type" yaml:"type"`         // string, number, boolean, array, object
	Description string   `json:"description" yaml:"description"`
	Required    bool     `json:"required" yaml:"required"`
	Default     any      `json:"default,omitempty" yaml:"default,omitempty"`
	Enum        []string `json:"enum,omitempty" yaml:"enum,omitempty"` // 枚举值
}

// Skill Skill 接口
type Skill interface {
	// ID 获取 Skill ID
	ID() SkillID

	// Manifest 获取清单
	Manifest() *SkillManifest

	// Initialize 初始化 Skill
	Initialize(ctx context.Context, config map[string]string) error

	// Execute 执行 Skill
	Execute(ctx context.Context, tool string, input map[string]interface{}) (map[string]interface{}, error)

	// Shutdown 关闭 Skill
	Shutdown(ctx context.Context) error

	// Health 健康检查
	Health(ctx context.Context) error
}

// BuiltinSkill 内置 Skill 基类
type BuiltinSkill struct {
	manifest *SkillManifest
	config   map[string]string
}

// NewBuiltinSkill 创建内置 Skill
func NewBuiltinSkill(manifest *SkillManifest) *BuiltinSkill {
	return &BuiltinSkill{
		manifest: manifest,
		config:   make(map[string]string),
	}
}

// ID 实现 Skill 接口
func (s *BuiltinSkill) ID() SkillID {
	return s.manifest.ID
}

// Manifest 实现 Skill 接口
func (s *BuiltinSkill) Manifest() *SkillManifest {
	return s.manifest
}

// Initialize 实现 Skill 接口
func (s *BuiltinSkill) Initialize(ctx context.Context, config map[string]string) error {
	s.config = config
	return nil
}

// Shutdown 实现 Skill 接口
func (s *BuiltinSkill) Shutdown(ctx context.Context) error {
	return nil
}

// Health 实现 Skill 接口
func (s *BuiltinSkill) Health(ctx context.Context) error {
	return nil
}

// Execute 实现 Skill 接口（默认实现，子类可覆盖）
func (s *BuiltinSkill) Execute(ctx context.Context, tool string, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("tool %s not implemented", tool)
}
