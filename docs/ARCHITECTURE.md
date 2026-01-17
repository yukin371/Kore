# Kore 架构文档

## 目录

1. [架构概览](#架构概览)
2. [设计原则](#设计原则)
3. [目录结构](#目录结构)
4. [核心模块](#核心模块)
5. [数据流](#数据流)
6. [设计模式](#设计模式)
7. [扩展指南](#扩展指南)
8. [开发规范](#开发规范)
9. [性能优化](#性能优化)
10. [故障排查](#故障排查)

---

## 架构概览

### 整体架构

Kore 采用 **Agent-Centric（代理中心）** 架构，通过 UI 抽象层实现多种交互模式（CLI/TUI/GUI）的统一接入。

```
┌─────────────────────────────────────────────────────────────┐
│                        用户交互层                             │
├─────────────────────────────────────────────────────────────┤
│   CLI Adapter   │   TUI Adapter   │   GUI Adapter (Future)   │
└────────────┬──────────────┬──────────────┬─────────────────────┘
             │              │              │
             └──────────────┼──────────────┘
                            ▼
                  ┌─────────────────────┐
                  │    UI Interface     │ (抽象接口)
                  └─────────┬───────────┘
                            ▼
                  ┌─────────────────────┐
                  │       Agent          │ (核心代理)
                  │  - ReAct Loop        │
                  │  - Context Manager   │
                  │  - Tool Executor     │
                  └─────────┬───────────┘
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌─────────┐      ┌──────────┐      ┌──────────┐
   │ LLM     │      │ Tools    │      │ History  │
   │Provider │      │ Executor │      │          │
   └─────────┘      └──────────┘      └──────────┘
```

### 核心理念

1. **UI 抽象**: 通过 `UIInterface` 接口解耦 UI 实现
2. **Agent 中心**: 所有业务逻辑在 Agent 中，UI 只负责交互
3. **插件化工具**: 工具通过 `Tool` 接口注册，易于扩展
4. **Provider 模式**: LLM 通过 `LLMProvider` 接口，支持多提供商

---

## 设计原则

### 1. 单一职责原则 (SRP)

每个模块只负责一个功能领域：

- `internal/core`: 领域模型和业务逻辑
- `internal/adapters`: UI 和 LLM 的适配器实现
- `internal/tools`: 工具实现和安全防护
- `internal/infrastructure`: 基础设施（配置、文件系统）

### 2. 开闭原则 (OCP)

- **对扩展开放**:
  - 新增 UI 模式：实现 `UIInterface`
  - 新增 LLM Provider：实现 `LLMProvider`
  - 新增工具：实现 `Tool` 接口并注册

- **对修改封闭**:
  - 核心逻辑不需要修改即可扩展

### 3. 依赖倒置原则 (DIP)

高层模块（Agent）不依赖低层模块（CLI、TUI），都依赖抽象（UIInterface）。

```go
// 高层模块依赖抽象
type Agent struct {
    UI UIInterface  // 不依赖具体的 CLI 或 TUI
}

// 低层模块实现抽象
type CLIAdapter struct {}
func (c *CLIAdapter) SendStream(content string) { ... }
```

### 4. 接口隔离原则 (ISP)

`UIInterface` 只定义必要的方法，不强迫实现类依赖不需要的方法。

```go
type UIInterface interface {
    SendStream(content string)
    RequestConfirm(action string, args string) bool
    RequestConfirmWithDiff(path string, diffText string) bool
    ShowStatus(status string)
}
```

---

## 目录结构

```
kore-foundation/
├── cmd/                    # 应用程序入口
│   └── kore/
│       └── main.go       # 主函数和 CLI 命令
│
├── internal/              # 内部包（不对外暴露）
│   ├── adapters/          # 适配器层
│   │   ├── cli/          # CLI 适配器
│   │   │   └── adapter.go
│   │   ├── tui/          # TUI 适配器
│   │   │   ├── model.go  # Bubble Tea Model
│   │   │   └── adapter.go
│   │   ├── openai/       # OpenAI 兼容 Provider
│   │   │   └── provider.go
│   │   └── ollama/       # Ollama Provider
│   │       └── provider.go
│   │
│   ├── core/             # 核心领域模型
│   │   ├── agent.go      # Agent 实现（ReAct Loop）
│   │   ├── context.go    # 上下文管理
│   │   ├── history.go    # 对话历史
│   │   └── llm.go        # LLM 抽象接口
│   │
│   ├── tools/            # 工具层
│   │   ├── executor.go   # 工具执行器
│   │   ├── security.go   # 安全拦截器
│   │   ├── read_file.go
│   │   ├── write_file.go
│   │   ├── run_command.go
│   │   ├── search_files.go
│   │   └── list_files.go
│   │
│   └── infrastructure/   # 基础设施层
│       ├── config/      # 配置管理
│       │   └── config.go
│       └── fs/          # 文件系统工具
│           └── walker.go
│
├── pkg/                   # 公共库（可被外部使用）
│   ├── logger/           # 日志工具
│   │   └── logger.go
│   └── utils/            # 通用工具
│       └── utils.go
│
├── api/                   # API 资源
│   └── prompts/          # 系统提示词
│       ├── system.txt    # 系统提示词
│       └── tools.txt     # 工具说明
│
├── docs/                  # 文档
│   ├── plans/            # 设计计划
│   └── USER_GUIDE.md     # 使用指南
│
├── configs/               # 配置文件模板
│   └── default.yaml
│
├── go.mod                # Go 模块定义
├── go.sum                # 依赖锁定
├── Makefile              # 构建脚本（可选）
└── README.md             # 项目说明
```

### 目录职责说明

| 目录 | 职责 | 可见性 |
|------|------|--------|
| `cmd/` | 应用入口点 | 公开 |
| `internal/` | 内部实现，不对外暴露 | 私有 |
| `pkg/` | 可重用的公共库 | 公开 |
| `api/` | 静态资源和配置 | 公开 |
| `docs/` | 项目文档 | 公开 |
| `configs/` | 配置模板 | 公开 |

---

## 核心模块

### 1. Core - 核心领域模型

#### 1.1 Agent (internal/core/agent.go)

**职责**: 协调所有组件，实现 ReAct Loop

**关键方法**:
```go
// Run - 主执行循环
func (a *Agent) Run(ctx context.Context, userMessage string) error

// executeToolsSequential - 顺序执行工具
func (a *Agent) executeToolsSequential(ctx context.Context, toolCalls []*ToolCall)

// executeToolsParallel - 并行执行工具
func (a *Agent) executeToolsParallel(ctx context.Context, toolCalls []*ToolCall)
```

**数据流**:
```
用户输入
  → History.AddUserMessage()
  → ContextMgr.BuildContext()
  → buildSystemPrompt()
  → LLMProvider.ChatStream()
  → 处理工具调用
  → 循环直到完成
```

#### 1.2 Context Manager (internal/core/context.go)

**职责**: 管理项目上下文，智能选择文件

**关键方法**:
```go
// BuildContext - 构建项目上下文
func (c *ContextManager) BuildContext(ctx context.Context) (*ProjectContext, error)

// calculateFilePriority - 计算文件优先级
func (c *ContextManager) calculateFilePriority(path string) int
```

**策略**:
- 分层上下文：目录树 + 文件内容
- LRU 缓存：避免重复读取
- 智能评分：根据文件名、扩展名、位置评分

#### 1.3 History (internal/core/history.go)

**职责**: 管理对话历史

**关键方法**:
```go
// AddUserMessage - 添加用户消息
func (h *ConversationHistory) AddUserMessage(content string)

// AddAssistantMessage - 添加助手消息
func (h *ConversationHistory) AddAssistantMessage(content string, toolCalls []ToolCall)

// BuildRequest - 构建 LLM 请求
func (h *ConversationHistory) BuildRequest() ChatRequest
```

**线程安全**: 使用 `sync.RWMutex` 保护并发访问

#### 1.4 LLM (internal/core/llm.go)

**职责**: LLM Provider 抽象接口

**关键接口**:
```go
type LLMProvider interface {
    ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    SetModel(model string)
    GetModel() string
}
```

**事件类型**:
```go
const (
    EventContent  EventType = "content"   // 文本内容
    EventToolCall EventType = "tool_call" // 工具调用
    EventError    EventType = "error"     // 错误
    EventDone     EventType = "done"      // 完成
)
```

### 2. Adapters - 适配器层

#### 2.1 UI 适配器

**接口定义** (internal/core/agent.go):
```go
type UIInterface interface {
    SendStream(content string)
    RequestConfirm(action string, args string) bool
    RequestConfirmWithDiff(path string, diffText string) bool
    ShowStatus(status string)
}
```

**CLI 实现** (internal/adapters/cli/adapter.go):
- 使用 `fmt.Print` 输出
- 使用 `fmt.Scanln` 读取输入
- 简单直接的实现

**TUI 实现** (internal/adapters/tui/):
- 使用 Bubble Tea 框架
- Model-Update-View 架构
- 支持流式显示、对话框、滚动

#### 2.2 LLM Provider

**OpenAI Provider** (internal/adapters/openai/provider.go):
- 兼容 OpenAI API
- 支持流式响应（SSE）
- 工具调用支持

**Ollama Provider** (internal/adapters/ollama/provider.go):
- 本地模型支持
- JSONL 流式响应
- XML 工具格式（回退方案）

### 3. Tools - 工具层

#### 3.1 工具接口

```go
type Tool interface {
    Name() string
    Description() string
    Schema() string
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

#### 3.2 安全拦截器

**职责**:
- 路径遍历防护
- 命令注入防护
- 敏感路径/命令黑名单

**关键方法**:
```go
// ValidatePath - 验证文件路径
func (s *SecurityInterceptor) ValidatePath(inputPath string) (string, error)

// ValidateCommand - 验证命令
func (s *SecurityInterceptor) ValidateCommand(cmd string) error
```

#### 3.3 工具列表

| 工具 | 文件 | 功能 |
|------|------|------|
| read_file | read_file.go | 读取文件内容 |
| write_file | write_file.go | 写入文件 |
| run_command | run_command.go | 执行命令 |
| search_files | search_files.go | 搜索文件 |
| list_files | list_files.go | 列出文件 |

### 4. Infrastructure - 基础设施层

#### 4.1 配置管理 (internal/infrastructure/config/)

**配置结构**:
```go
type Config struct {
    LLM struct {
        Provider    string
        Model       string
        APIKey      string
        BaseURL     string
        Temperature float32
        MaxTokens   int
    }
    UI struct {
        Mode string
    }
    Context struct {
        MaxTokens      int
        MaxTreeDepth   int
        MaxFilesPerDir int
    }
    Security struct {
        BlockedCmds  []string
        BlockedPaths []string
    }
}
```

**配置加载**:
```go
// Load 加载配置文件
func Load() (*Config, error)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config
```

#### 4.2 文件系统工具 (internal/infrastructure/fs/)

**FastWalk**:
```go
// FastWalk 快速遍历目录树
func FastWalk(config WalkConfig) (*WalkResult, error)
```

**特性**:
- 深度限制
- 文件数量限制
- 忽略规则支持

---

## 数据流

### 1. 用户交互流程

```
用户输入
  ↓
UI Adapter (CLI/TUI)
  ↓
Agent.Run()
  ↓
┌─────────────────────┐
│ 1. 构建上下文         │
│ 2. 添加用户消息       │
│ 3. 构建系统提示      │
│ 4. 调用 LLM          │
│ 5. 处理流式响应      │
│ 6. 执行工具（如需）   │
│ 7. 循环直到完成      │
└─────────────────────┘
  ↓
UI 显示响应
```

### 2. 工具调用流程

```
LLM 返回工具调用
  ↓
Agent 收集 ToolCall
  ↓
UI.RequestConfirm()
  ↓
[用户确认]
  ↓
ToolExecutor.Execute()
  ↓
┌─────────────────────┐
│ 1. 路径验证          │
│ 2. 命令验证          │
│ 3. 执行工具          │
│ 4. 格式化结果        │
└─────────────────────┘
  ↓
添加工具结果到历史
  ↓
继续 LLM 循环
```

### 3. 并行工具执行流程

```
多个工具调用
  ↓
Agent.executeToolsParallel()
  ↓
┌──────────────────────────────┐
│ goroutine 1: Tool 1          │
│ goroutine 2: Tool 2          │
│ goroutine 3: Tool 3          │
│      ↓        ↓        ↓       │
│   WaitGroup 同步               │
└──────────────────────────────┘
  ↓
结果通道收集
  ↓
添加到历史（按完成顺序）
```

---

## 设计模式

### 1. Strategy Pattern - 策略模式

**场景**: LLM Provider

```go
// 策略接口
type LLMProvider interface {
    ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}

// 具体策略
type OpenAIProvider struct { ... }
type OllamaProvider struct { ... }

// 使用
var provider LLMProvider
switch config.LLM.Provider {
case "openai":
    provider = NewOpenAIProvider(...)
case "ollama":
    provider = NewOllamaProvider(...)
}
```

### 2. Adapter Pattern - 适配器模式

**场景**: UI 适配

```go
// 目标接口
type UIInterface interface {
    SendStream(content string)
}

// 适配器
type CLIAdapter struct { ... }
type TUIAdapter struct { ... }

// 使用
var ui UIInterface
switch mode {
case "cli":
    ui = NewCLIAdapter()
case "tui":
    ui = NewTUIAdapter()
}
```

### 3. Observer Pattern - 观察者模式

**场景**: Bubble Tea TUI

```go
// Model（被观察者）
type Model struct { ... }

// Update（状态更新）
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case StreamMsg:
        // 处理流式内容
    case StatusMsg:
        // 处理状态更新
    }
}
```

### 4. Builder Pattern - 建造者模式

**场景**: 请求构建

```go
// History 建造者
func (h *ConversationHistory) BuildRequest() ChatRequest {
    req := ChatRequest{
        Messages: h.messages,
        // ...
    }
    return req
}
```

### 5. Chain of Responsibility - 责任链模式

**场景**: 安全拦截器

```go
// 责任链
func (t *ToolExecutor) Execute(ctx context.Context, call ToolCall) (string, error) {
    // 1. 工具存在性检查
    // 2. 安全验证
    // 3. 工具执行
}
```

### 6. Template Method - 模板方法模式

**场景**: Agent 执行流程

```go
// 模板方法
func (a *Agent) Run(ctx context.Context, userMessage string) error {
    // 算法骨架
    for {
        // 1. 构建请求
        // 2. 调用 LLM
        // 3. 处理响应
        // 4. 执行工具
        // 5. 判断是否继续
    }
}
```

---

## 扩展指南

### 1. 添加新的 UI 模式

#### 步骤 1: 实现 UIInterface

```go
// internal/adapters/gui/adapter.go
package gui

import "github.com/yukin/kore/internal/core"

type Adapter struct {
    // GUI 特定字段
}

func NewAdapter() *Adapter {
    return &Adapter{}
}

func (a *Adapter) SendStream(content string) {
    // GUI 实现
}

func (a *Adapter) RequestConfirm(action string, args string) bool {
    // GUI 实现
    return true
}

func (a *Adapter) RequestConfirmWithDiff(path string, diffText string) bool {
    // GUI 实现
    return true
}

func (a *Adapter) ShowStatus(status string) {
    // GUI 实现
}
```

#### 步骤 2: 注册新模式

```go
// cmd/kore/main.go
switch mode {
case "gui":
    uiAdapter = gui.NewAdapter()
    // ...
}
```

### 2. 添加新的 LLM Provider

#### 步骤 1: 实现 LLMProvider 接口

```go
// internal/adapters/anthropic/provider.go
package anthropic

import "github.com/yukin/kore/internal/core"

type Provider struct {
    apiKey string
    model  string
}

func NewProvider(apiKey, model string) *Provider {
    return &Provider{
        apiKey: apiKey,
        model:  model,
    }
}

func (p *Provider) ChatStream(ctx context.Context, req core.ChatRequest) (<-chan core.StreamEvent, error) {
    // 实现 Claude API 调用
}

func (p *Provider) SetModel(model string) {
    p.model = model
}

func (p *Provider) GetModel() string {
    return p.model
}
```

#### 步骤 2: 注册 Provider

```go
// cmd/kore/main.go
case "anthropic":
    llmProvider = anthropic.NewProvider(cfg.LLM.APIKey, cfg.LLM.Model)
```

### 3. 添加新工具

#### 步骤 1: 实现 Tool 接口

```go
// internal/tools/git_commit.go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
)

type GitCommitTool struct {
    projectRoot string
    security    *SecurityInterceptor
}

func (t *GitCommitTool) Name() string {
    return "git_commit"
}

func (t *GitCommitTool) Description() string {
    return "创建 Git 提交"
}

func (t *GitCommitTool) Schema() string {
    schema := map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "message": map[string]interface{}{
                "type":        "string",
                "description": "提交消息",
            },
        },
        "required": []string{"message"},
    }

    jsonBytes, _ := json.Marshal(schema)
    return string(jsonBytes)
}

func (t *GitCommitTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var params struct {
        Message string `json:"message"`
    }

    if err := json.Unmarshal(args, &params); err != nil {
        return "", err
    }

    // 执行 git commit
    cmd := exec.Command("git", "commit", "-m", params.Message)
    cmd.Dir = t.projectRoot

    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("git commit 失败: %w", err)
    }

    return string(output), nil
}
```

#### 步骤 2: 注册工具

```go
// internal/tools/executor.go
func (te *ToolExecutor) RegisterDefaultTools() {
    // ... 现有工具
    te.RegisterTool(&GitCommitTool{
        projectRoot: te.projectRoot,
        security:    te.security,
    })
}
```

### 4. 自定义上下文策略

```go
// internal/core/context.go
type ContextManager struct {
    projectRoot      string
    tokenBudget      int
    filePriorityRule map[string]int
}

// 自定义文件优先级规则
func (c *ContextManager) calculateFilePriority(path string) int {
    // 添加自定义规则
    if strings.Contains(path, "critical") {
        return 100
    }
    // ...
}
```

---

## 开发规范

### 1. 代码组织

#### 文件命名

- Go 文件：`snake_case.go`
- 测试文件：`xxx_test.go`
- 包文档：`doc.go`

#### 包命名

- 全小写
- 简洁描述性
- 避免下划线（除非必要）

```go
package core         // ✅
package http_server  // ✅
package http_server  // ❌ 改为 httpserver
```

### 2. 错误处理

#### 错误包装

```go
// ✅ 好的做法
file, err := os.Open(path)
if err != nil {
    return fmt.Errorf("打开文件失败: %w", err)
}

// ❌ 不好的做法
if err != nil {
    return err
}
```

#### 错误检查

```go
// 立即处理错误
result, err := someFunc()
if err != nil {
    return err
}
// 使用 result
```

### 3. 并发安全

#### 互斥锁

```go
type SafeStruct struct {
    mu sync.RWMutex
    data map[string]string
}

func (s *SafeStruct) Read(key string) (string, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    val, ok := s.data[key]
    return val, ok
}

func (s *SafeStruct) Write(key, val string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.data[key] = val
}
```

#### 通道使用

```go
// ✅ 好：缓冲通道
resultChan := make(chan ToolResult, len(toolCalls))

// ❌ 不好：忘记关闭通道
go func() {
    for val := range resultChan {
        // 处理结果
    }
    // 忘记 close(resultChan)
}()

// ✅ 好：确保关闭
go func() {
    defer close(resultChan)
    for val := range resultChan {
        // 处理结果
    }
}()
```

### 4. 注释规范

#### 包注释

```go
// Package core 提供 Kore 的核心领域模型和业务逻辑
//
// 核心组件包括：
//   - Agent: AI 代理，实现 ReAct 循环
//   - ContextManager: 上下文管理
//   - ConversationHistory: 对话历史
//   - LLM: LLM Provider 抽象接口
package core
```

#### 函数注释

```go
// Run 执行 Agent 主循环
// 实现完整的 ReAct（推理+行动）循环：
// 1. 构建项目上下文
// 2. 发送用户消息和上下文给 LLM
// 3. 处理流式响应
// 4. 执行工具调用
// 5. 循环直到完成
//
// 参数：
//   ctx - 上下文对象，用于取消操作
//   userMessage - 用户输入的消息
//
// 返回：
//   error - 执行过程中的错误
func (a *Agent) Run(ctx context.Context, userMessage string) error {
    // ...
}
```

#### 代码注释

```go
// 验证路径是否在项目根目录内
if !strings.HasPrefix(absPath, c.projectRoot) {
    return "", fmt.Errorf("安全错误：路径遍历检测到 %s", inputPath)
}

// 检查是否在黑名单中
for _, blocked := range s.blockedCmds {
    if baseCmd == blocked {
        return "", fmt.Errorf("禁止命令: %s", baseCmd)
    }
}
```

### 5. 测试规范

#### 单元测试

```go
// internal/core/agent_test.go
package core

import "testing"

func TestAgent_NewAgent(t *testing.T) {
    ui := &MockUI{}
    llm := &MockLLM{}
    tools := &MockToolExecutor{}

    agent := NewAgent(ui, llm, tools, ".")

    if agent == nil {
        t.Error("NewAgent 返回 nil")
    }
}
```

#### 表驱动测试

```go
func TestValidatePath(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        wantErr bool
    }{
        {
            name:    "valid path",
            path:    "main.go",
            wantErr: false,
        },
        {
            name:    "path traversal",
            path:    "../../../etc/passwd",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePath(tt.path)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

---

## 性能优化

### 1. 上下文管理优化

#### 问题：大项目上下文过大

**解决方案**:
```go
// 1. 分层上下文策略
type LayeredContext struct {
    Tree     string  // 目录树（小）
    Contents []string // 关键文件内容（按需）
}

// 2. LRU 缓存
type ContextCache struct {
    cache    *lru.Cache
    maxSize  int
}

// 3. 智能文件选择
func (c *ContextManager) SelectFiles(files []string) []string {
    // 根据优先级排序
    sort.Slice(files, func(i, j int) bool {
        return c.calculateFilePriority(files[i]) > c.calculateFilePriority(files[j])
    })

    // 取前 N 个
    maxFiles := 20
    if len(files) > maxFiles {
        return files[:maxFiles]
    }
    return files
}
```

### 2. 并发工具执行

#### 问题：工具顺序执行效率低

**解决方案**:
```go
// 配置启用并行执行
agent.Config.ParallelTools = true

// 实现细节
func (a *Agent) executeToolsParallel(ctx context.Context, toolCalls []*ToolCall) {
    resultChan := make(chan ToolResult, len(toolCalls))
    var wg sync.WaitGroup

    // 并发执行
    for _, call := range toolCalls {
        wg.Add(1)
        go func(toolCall *ToolCall) {
            defer wg.Done()
            result := a.executeTool(ctx, toolCall)
            resultChan <- result
        }(call)
    }

    // 等待完成
    go func() {
        wg.Wait()
        close(resultChan)
    }()

    // 收集结果
    for result := range resultChan {
        a.History.AddToolOutput(result.ID, result.Output)
    }
}
```

### 3. 流式响应处理

#### 问题：等待完整响应才显示

**解决方案**:
```go
// 流式处理
eventChan := llm.ChatStream(ctx, request)

for event := range eventChan {
    switch event.Type {
    case EventContent:
        // 立即显示
        ui.SendStream(event.Content)

    case EventToolCall:
        // 处理工具调用
        // ...
    }
}
```

### 4. 文件系统遍历优化

#### 问题：递归遍历性能差

**解决方案**:
```go
// 使用 FastWalk
result, err := fs.FastWalk(fs.WalkConfig{
    Root:       root,
    MaxDepth:   5,
    MaxFiles:   1000,
    IgnoreFunc: func(path string) bool {
        return shouldIgnore(path)
    },
})
```

---

## 故障排查

### 1. 调试模式

#### 启用详细日志

```go
// pkg/logger/logger.go
func SetLevel(level Level) {
    currentLevel = level
}

// 使用
logger.SetLevel(logger.DEBUG)
```

#### 日志级别

```go
const (
    DEBUG Level = iota // 详细调试信息
    INFO              // 一般信息
    WARN              // 警告
    ERROR             // 错误
)
```

### 2. 常见问题

#### 问题 1: 工具调用失败

**症状**:
```
[Error: 未知工具: search_files]
```

**原因**: 工具未注册

**解决**:
```go
// 检查 RegisterDefaultTools()
func (te *ToolExecutor) RegisterDefaultTools() {
    te.RegisterTool(NewSearchFilesTool(...))
}
```

#### 问题 2: 并发竞态

**症状**:
```
fatal error: concurrent map writes
```

**原因**: 并发访问未保护的数据

**解决**:
```go
type SafeMap struct {
    mu   sync.RWMutex
    data map[string]string
}

func (m *SafeMap) Write(key, val string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.data[key] = val
}
```

#### 问题 3: 内存泄漏

**症状**: 内存持续增长

**排查**:
```go
// 检查 goroutine 泄漏
// 检查缓存未清理
// 检查文件句柄未关闭

// 使用 pprof
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}
```

### 3. 性能分析

#### CPU 分析

```bash
# 1. 添加 pprof
import _ "net/http/pprof"

# 2. 运行程序
go run cmd/kore/main.go chat

# 3. 分析
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

#### 内存分析

```bash
# 堆内存分析
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## 附录

### A. 依赖说明

| 依赖 | 版本 | 用途 |
|------|------|------|
| cobra | ^2 | CLI 框架 |
| bubbletea | ^1.3 | TUI 框架 |
| lipgloss | ^1.1 | 样式库 |
| glamour | ^0.10 | Markdown 渲染 |
| bubbles | ^0.21 | TUI 组件 |

### B. 环境变量

```bash
# Kore 配置路径
KORE_CONFIG_PATH=~/.config/kore/config.yaml

# API 密钥（覆盖配置文件）
KORE_API_KEY=your-api-key

# LLM Provider
KORE_LLM_PROVIDER=openai

# 模型名称
KORE_MODEL=glm-4

# UI 模式
KORE_UI_MODE=tui
```

### C. 退出码

| 代码 | 含义 |
|------|------|
| 0 | 成功 |
| 1 | 一般错误 |
| 2 | 配置错误 |
| 3 | LLM 调用失败 |

### D. 相关资源

- [Bubble Tea 文档](https://github.com/charmbracelet/bubbletea)
- [Cobra 文档](https://github.com/spf13/cobra)
- [Go 并发模式](https://go.dev/doc/effective_go.html#concurrency)
- [Go 代码审查](https://github.com/golang/go/wiki/CodeReviewComments)

---

**文档版本**: 1.0
**最后更新**: 2025-01-16
**维护者**: Kore Team
