// Package agent 提供 Agent 类型系统测试
package agent

import (
	"context"
	"testing"

	"github.com/yukin371/Kore/internal/core"
	"github.com/yukin371/Kore/internal/types"
)

// mockToolExecutor 模拟工具执行器
type mockToolExecutor struct{}

func (m *mockToolExecutor) Execute(ctx context.Context, call core.ToolCall) (string, error) {
	return `{"result": "success"}`, nil
}

// mockUI 模拟 UI
type mockUI struct{}

func (m *mockUI) SendStream(content string)                                    {}
func (m *mockUI) RequestConfirm(action string, args string) bool              { return true }
func (m *mockUI) RequestConfirmWithDiff(path string, diffText string) bool    { return true }
func (m *mockUI) ShowStatus(status string)                                    {}
func (m *mockUI) StartThinking()                                              {}
func (m *mockUI) StopThinking()                                               {}

// mockLLMProvider 模拟 LLM 提供者
type mockLLMProvider struct{}

func (m *mockLLMProvider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
	// 返回一个关闭的通道，模拟立即完成
	ch := make(chan core.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockLLMProvider) SetModel(model string) {}

func (m *mockLLMProvider) GetModel() string {
	return "mock-model"
}

// 创建测试用的核心 Agent
func createTestCoreAgent(t *testing.T) *core.Agent {
	t.Helper()

	ui := &mockUI{}
	llm := &mockLLMProvider{}
	tools := &mockToolExecutor{}

	return core.NewAgent(ui, llm, tools, "/test/project")
}

// TestCreateAgent 测试工厂方法
func TestCreateAgent(t *testing.T) {
	coreAgent := createTestCoreAgent(t)
	projectRoot := "/test/project"

	tests := []struct {
		name           string
		mode           types.AgentMode
		expectWrite    bool
		expectExecute  bool
		expectInvoke   bool
		expectedMode   types.AgentMode
	}{
		{
			name:          "Build Agent - 完全访问权限",
			mode:          types.ModeUltraWork,
			expectWrite:   true,
			expectExecute: true,
			expectInvoke:  false,
			expectedMode:  types.ModeUltraWork,
		},
		{
			name:          "Plan Agent - 只读访问权限",
			mode:          types.ModeSearch,
			expectWrite:   false,
			expectExecute: false,
			expectInvoke:  false,
			expectedMode:  types.ModeSearch,
		},
		{
			name:          "General Agent - 通用模式",
			mode:          types.ModeAnalyze,
			expectWrite:   true,
			expectExecute: true,
			expectInvoke:  true,
			expectedMode:  types.ModeAnalyze,
		},
		{
			name:          "Normal Agent - 默认模式",
			mode:          types.ModeNormal,
			expectWrite:   true,
			expectExecute: true,
			expectInvoke:  false,
			expectedMode:  types.ModeNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := CreateAgent(tt.mode, coreAgent, projectRoot)
			if err != nil {
				t.Fatalf("创建 Agent 失败: %v", err)
			}

			// 验证 Agent 类型
			if agent.GetMode() != tt.expectedMode {
				t.Errorf("期望模式 %s, 实际 %s", tt.expectedMode, agent.GetMode())
			}

			// 验证 Agent 有效性
			if !agent.IsValid() {
				t.Error("Agent 应该是有效的")
			}

			// 验证能力接口
			if capAgent, ok := agent.(interface {
				CanWrite() bool
				CanExecuteCommand() bool
				CanInvokeAgents() bool
			}); ok {
				if capAgent.CanWrite() != tt.expectWrite {
					t.Errorf("期望 CanWrite() = %v", tt.expectWrite)
				}
				if capAgent.CanExecuteCommand() != tt.expectExecute {
					t.Errorf("期望 CanExecuteCommand() = %v", tt.expectExecute)
				}
				if capAgent.CanInvokeAgents() != tt.expectInvoke {
					t.Errorf("期望 CanInvokeAgents() = %v", tt.expectInvoke)
				}
			} else {
				t.Error("Agent 应该实现能力接口")
			}

			// 验证配置
			config := agent.GetConfig()
			if config == nil {
				t.Error("配置不应该为空")
			}

			// 验证配置有效性
			if err := config.Validate(); err != nil {
				t.Errorf("配置验证失败: %v", err)
			}
		})
	}
}

// TestBuildAgentCapabilities 测试 Build Agent 的能力
func TestBuildAgentCapabilities(t *testing.T) {
	coreAgent := createTestCoreAgent(t)
	projectRoot := "/test/project"

	agent, err := NewBuildAgent(coreAgent, projectRoot)
	if err != nil {
		t.Fatalf("创建 Build Agent 失败: %v", err)
	}

	// 验证基本能力
	if !agent.CanWrite() {
		t.Error("Build Agent 应该允许写入")
	}
	if !agent.CanExecuteCommand() {
		t.Error("Build Agent 应该允许执行命令")
	}
	if agent.CanInvokeAgents() {
		t.Error("Build Agent 不应该允许调用其他 Agent")
	}

	// 验证工具调用权限
	tests := []struct {
		tool      string
		shouldErr bool
	}{
		{"read_file", false},
		{"write_file", false},
		{"run_command", false},
		{"search_files", false},
		{"list_files", false},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			err := agent.ValidateToolCall(tt.tool)
			if (err != nil) != tt.shouldErr {
				t.Errorf("工具 %s: 期望错误 = %v, 实际 = %v", tt.tool, tt.shouldErr, err)
			}
		})
	}

	// 验证摘要信息
	summary := agent.GetSummary()
	if summary["can_write"] != true {
		t.Error("摘要中 can_write 应该为 true")
	}
	if summary["can_execute"] != true {
		t.Error("摘要中 can_execute 应该为 true")
	}
	if summary["can_invoke"] != false {
		t.Error("摘要中 can_invoke 应该为 false")
	}
}

// TestPlanAgentCapabilities 测试 Plan Agent 的能力
func TestPlanAgentCapabilities(t *testing.T) {
	coreAgent := createTestCoreAgent(t)
	projectRoot := "/test/project"

	agent, err := NewPlanAgent(coreAgent, projectRoot)
	if err != nil {
		t.Fatalf("创建 Plan Agent 失败: %v", err)
	}

	// 验证基本能力
	if agent.CanWrite() {
		t.Error("Plan Agent 不应该允许写入")
	}
	if agent.CanExecuteCommand() {
		t.Error("Plan Agent 不应该允许执行命令")
	}
	if agent.CanInvokeAgents() {
		t.Error("Plan Agent 不应该允许调用其他 Agent")
	}

	// 验证工具调用权限
	tests := []struct {
		tool      string
		shouldErr bool
	}{
		{"read_file", false},
		{"write_file", true},  // 应该被禁止
		{"run_command", true},  // 应该被禁止
		{"search_files", false},
		{"list_files", false},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			err := agent.ValidateToolCall(tt.tool)
			if (err != nil) != tt.shouldErr {
				t.Errorf("工具 %s: 期望错误 = %v, 实际 = %v", tt.tool, tt.shouldErr, err)
			}
		})
	}

	// 验证摘要信息
	summary := agent.GetSummary()
	if summary["can_write"] != false {
		t.Error("摘要中 can_write 应该为 false")
	}
	if summary["can_execute"] != false {
		t.Error("摘要中 can_execute 应该为 false")
	}
	if summary["can_invoke"] != false {
		t.Error("摘要中 can_invoke 应该为 false")
	}
}

// TestGeneralAgentCapabilities 测试 General Agent 的能力
func TestGeneralAgentCapabilities(t *testing.T) {
	coreAgent := createTestCoreAgent(t)
	projectRoot := "/test/project"

	agent, err := NewGeneralAgent(coreAgent, projectRoot)
	if err != nil {
		t.Fatalf("创建 General Agent 失败: %v", err)
	}

	// 验证基本能力
	if !agent.CanWrite() {
		t.Error("General Agent 应该允许写入")
	}
	if !agent.CanExecuteCommand() {
		t.Error("General Agent 应该允许执行命令")
	}
	if !agent.CanInvokeAgents() {
		t.Error("General Agent 应该允许调用其他 Agent")
	}

	// 验证工具调用权限
	tests := []struct {
		tool      string
		shouldErr bool
	}{
		{"read_file", false},
		{"write_file", false},
		{"run_command", false},
		{"search_files", false},
		{"list_files", false},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			err := agent.ValidateToolCall(tt.tool)
			if (err != nil) != tt.shouldErr {
				t.Errorf("工具 %s: 期望错误 = %v, 实际 = %v", tt.tool, tt.shouldErr, err)
			}
		})
	}

	// 验证摘要信息包含子 Agent
	summary := agent.GetSummary()
	if summary["can_write"] != true {
		t.Error("摘要中 can_write 应该为 true")
	}
	if summary["can_execute"] != true {
		t.Error("摘要中 can_execute 应该为 true")
	}
	if summary["can_invoke"] != true {
		t.Error("摘要中 can_invoke 应该为 true")
	}

	// 验证子 Agent 信息
	if subAgents, ok := summary["sub_agents"].([]string); ok {
		if len(subAgents) != 2 {
			t.Errorf("期望有 2 个子 Agent, 实际 %d", len(subAgents))
		}
	} else {
		t.Error("摘要应该包含 sub_agents 信息")
	}
}

// TestAgentModeConfig 测试模式配置
func TestAgentModeConfig(t *testing.T) {
	tests := []struct {
		name        string
		mode        types.AgentMode
		validateErr bool
	}{
		{"Normal 模式", types.ModeNormal, false},
		{"UltraWork 模式", types.ModeUltraWork, false},
		{"Search 模式", types.ModeSearch, false},
		{"Analyze 模式", types.ModeAnalyze, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultAgentModeConfig(tt.mode)

			// 验证配置
			err := config.Validate()
			if (err != nil) != tt.validateErr {
				t.Errorf("期望验证错误 = %v, 实际 = %v", tt.validateErr, err)
			}

			// 验证模式匹配
			if config.Mode != tt.mode {
				t.Errorf("期望模式 %s, 实际 %s", tt.mode, config.Mode)
			}

			// 验证最大迭代次数
			if config.MaxIterations <= 0 {
				t.Error("最大迭代次数应该大于 0")
			}
		})
	}
}

// TestToolPermissionCheck 测试工具权限检查
func TestToolPermissionCheck(t *testing.T) {
	tests := []struct {
		name        string
		mode        types.AgentMode
		tool        string
		shouldAllow bool
	}{
		{
			name:        "Build Agent - write_file",
			mode:        types.ModeUltraWork,
			tool:        "write_file",
			shouldAllow: true,
		},
		{
			name:        "Build Agent - run_command",
			mode:        types.ModeUltraWork,
			tool:        "run_command",
			shouldAllow: true,
		},
		{
			name:        "Plan Agent - write_file",
			mode:        types.ModeSearch,
			tool:        "write_file",
			shouldAllow: false,
		},
		{
			name:        "Plan Agent - run_command",
			mode:        types.ModeSearch,
			tool:        "run_command",
			shouldAllow: false,
		},
		{
			name:        "Plan Agent - read_file",
			mode:        types.ModeSearch,
			tool:        "read_file",
			shouldAllow: true,
		},
		{
			name:        "General Agent - write_file",
			mode:        types.ModeAnalyze,
			tool:        "write_file",
			shouldAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultAgentModeConfig(tt.mode)
			allowed := config.IsToolAllowed(tt.tool)

			if allowed != tt.shouldAllow {
				t.Errorf("模式 %s 工具 %s: 期望允许 = %v, 实际 = %v",
					tt.mode, tt.tool, tt.shouldAllow, allowed)
			}
		})
	}
}

// TestCreateAgentWithConfig 测试使用自定义配置创建 Agent
func TestCreateAgentWithConfig(t *testing.T) {
	coreAgent := createTestCoreAgent(t)
	projectRoot := "/test/project"

	// 创建自定义配置
	config := &AgentModeConfig{
		Mode:          types.ModeUltraWork,
		AllowedTools:  []string{"read_file", "search_files"},
		DeniedTools:   []string{"write_file"},
		MaxIterations: 50,
	}

	agent, err := CreateAgentWithConfig(types.ModeUltraWork, coreAgent, config, projectRoot)
	if err != nil {
		t.Fatalf("创建 Agent 失败: %v", err)
	}

	// 验证配置被应用
	appliedConfig := agent.GetConfig()
	if appliedConfig.MaxIterations != 50 {
		t.Errorf("期望 MaxIterations = 50, 实际 = %d", appliedConfig.MaxIterations)
	}

	// 验证工具权限 - BuildAgent 的 ValidateToolCall 允许所有工具
	// 但配置中的 DeniedTools 会被 IsToolAllowed 检查
	if appliedConfig.IsToolAllowed("write_file") {
		t.Error("write_file 应该被配置禁止")
	}

	// 验证允许的工具
	if !appliedConfig.IsToolAllowed("read_file") {
		t.Error("read_file 应该被允许")
	}
}

// TestGeneralAgentTaskAnalysis 测试 General Agent 任务分析
func TestGeneralAgentTaskAnalysis(t *testing.T) {
	coreAgent := createTestCoreAgent(t)
	projectRoot := "/test/project"

	agent, err := NewGeneralAgent(coreAgent, projectRoot)
	if err != nil {
		t.Fatalf("创建 General Agent 失败: %v", err)
	}

	tests := []struct {
		name     string
		prompt   string
		taskType TaskType
	}{
		{
			name:     "分析任务",
			prompt:   "帮我分析这个模块的架构",
			taskType: TaskTypeAnalysis,
		},
		{
			name:     "构建任务",
			prompt:   "创建一个新的用户认证功能",
			taskType: TaskTypeBuild,
		},
		{
			name:     "混合任务",
			prompt:   "先分析这个架构，然后重构它",
			taskType: TaskTypeMixed,
		},
		{
			name:     "混合任务 - 英文",
			prompt:   "Analyze this code and then refactor it",
			taskType: TaskTypeMixed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.analyzeTask(tt.prompt)
			if result != tt.taskType {
				t.Errorf("期望任务类型 %d, 实际 %d", tt.taskType, result)
			}
		})
	}
}

// TestAgentCapabilitiesString 测试能力描述
func TestAgentCapabilitiesString(t *testing.T) {
	coreAgent := createTestCoreAgent(t)
	projectRoot := "/test/project"

	tests := []struct {
		name             string
		createAgent      func() (Agent, error)
		expectedContains []string
	}{
		{
			name: "Build Agent",
			createAgent: func() (Agent, error) {
				return NewBuildAgent(coreAgent, projectRoot)
			},
			expectedContains: []string{
				"Build Agent",
				"完全访问权限",
				"读取文件",
				"写入文件",
				"执行命令",
			},
		},
		{
			name: "Plan Agent",
			createAgent: func() (Agent, error) {
				return NewPlanAgent(coreAgent, projectRoot)
			},
			expectedContains: []string{
				"Plan Agent",
				"只读访问权限",
				"读取文件",
				"写入文件",
				"禁止",
			},
		},
		{
			name: "General Agent",
			createAgent: func() (Agent, error) {
				return NewGeneralAgent(coreAgent, projectRoot)
			},
			expectedContains: []string{
				"General Agent",
				"完全访问权限",
				"Agent 编排能力",
				"调用 Plan Agent",
				"调用 Build Agent",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := tt.createAgent()
			if err != nil {
				t.Fatalf("创建 Agent 失败: %v", err)
			}

			capAgent, ok := agent.(interface{ GetCapabilities() string })
			if !ok {
				t.Fatal("Agent 应该实现 GetCapabilities 方法")
			}

			capabilities := capAgent.GetCapabilities()

			for _, expected := range tt.expectedContains {
				if !contains(capabilities, expected) {
					t.Errorf("能力描述应该包含 '%s'\n实际描述:\n%s", expected, capabilities)
				}
			}
		})
	}
}

// contains 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsString(s, substr)))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
