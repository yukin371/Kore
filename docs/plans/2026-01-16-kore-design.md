# Kore Design Document

**Date:** 2026-01-16
**Status:** Design Approved
**Version:** 1.0

## Overview

Kore is an AI-powered workflow automation platform built with Go, serving as the core中枢 for all development tasks. Featuring a hybrid CLI/TUI/GUI interface, Kore is inspired by Claude Code and aligned with agent platforms like Dify. It provides intelligent code understanding, modification, and automation capabilities through natural language interaction, with extensibility for custom workflows.

## Technology Stack

- **Backend:** Go 1.21+
- **CLI Framework:** Cobra + Viper
- **TUI Framework:** Bubble Tea + Lipgloss + Glamour
- **GUI Framework:** Wails v2 (future)
- **LLM Providers:** OpenAI API + Ollama
- **Diff Library:** github.com/sergi/go-diff/diffmatchpatch

## 1. Architecture

### Agent-Centric Design

The system uses an Agent-centric architecture with interface-based decoupling to support multiple UI modes (CLI/TUI/GUI).

```
┌─────────────────────────────────────────┐
│              UI Interface               │
│  (CLIAdapter | TUIAdapter | GUIAdapter) │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│              Agent                      │
│  ┌─────────────────────────────────┐   │
│  │ ContextManager                  │   │
│  │ LLMProvider                     │   │
│  │ ToolExecutor                    │   │
│  │ ConversationHistory             │   │
│  │ Config                          │   │
│  └─────────────────────────────────┘   │
└─────────────────────────────────────────┘
```

### UI Interface (Decoupling Layer)

```go
// UI 抽象接口 - Agent 不关心具体是 TUI 还是 GUI
type UIInterface interface {
    SendStream(content string)
    RequestConfirm(action string, args string) bool
    RequestConfirmWithDiff(path string, diffText string) bool
    ShowStatus(status string)
}

// Agent 持有接口，而非具体实现
type Agent struct {
    UI          UIInterface
    ContextMgr  *ContextManager
    LLMProvider *LLMProvider
    Tools       *ToolExecutor
    History     *ConversationHistory
    Config      *Config
}
```

### Concurrency Model

- TUI 启动时，通过 `tea.Cmd` 启动 Goroutine 运行 `Agent.Run(ctx)`
- Agent 调用 `UI.SendStream()` 时，实际调用的是 `TUIAdapter.SendStream()`
- `TUIAdapter` 内部使用 `program.Send()` 将消息发回 Bubble Tea 主循环
- Future Wails GUI: 实现 `WailsAdapter`，Agent 代码零修改

### Context Cancellation

```go
func (a *Agent) Run(ctx context.Context, userMessage string) error {
    // ctx.Done() 监听用户取消 (Ctrl+C 或停止按钮)
    // 立即停止 LLM 生成和工具执行
}
```

## 2. Context Management

### Layered Context Strategy

ContextManager adopts a **layered context strategy**, separating directory tree from file contents, enabling AI to dynamically "browse" files.

```go
type ContextManager struct {
    ignoreMatcher *gitignore.IgnoreMatcher
    focusedPaths  map[string]bool  // 焦点文件：读取完整内容
    focusLRU      *lru.Cache       // LRU 淘汰机制
    maxTokens     int              // Token 预算
    maxTreeDepth  int              // 目录树深度限制 (default: 5)
    maxFilesPerDir int             // 每目录最大文件数 (default: 50)
}

type ProjectContext struct {
    FileTree     string            // 完整目录树（低成本，总是包含）
    FocusedFiles []File            // 焦点文件的完整内容（高成本，精确控制）
    TotalTokens  int
}

// 核心方法
func (c *ContextManager) BuildContext(ctx context.Context) (*ProjectContext, error)
func (c *ContextManager) AddFocus(path string)              // AI 调用 read_file 时触发
func (c *ContextManager) ReadFile(path string) (string, error)
```

### Three-Tier Collection Strategy

1. **Tier 1 - Directory Tree** (Always included, ~500 tokens)
   - 使用快速遍历库生成项目结构树
   - 显示文件名和大小，让 AI 了解"有什么"
   - 深度限制和文件数限制，防止巨型项目撑爆 token

2. **Tier 2 - Focused Files** (Precise control, user/AI specified)
   - 用户启动时指定：`kore chat -f main.go -f auth.go`
   - AI 动态请求：通过 `read_file` 工具调用，自动加入焦点列表
   - LRU 淘汰机制：当超过 maxTokens 时，自动移除最少使用的文件

3. **Tier 3 - Auto-Fill** (Optional, when tokens available)
   - 按优先级自动填充重要文件（`README.md`、配置文件等）

### Dynamic Retrieval Flow

```
User: "登录逻辑在哪里？"
  ↓
AI (sees FileTree): "应该在 internal/auth/auth.go"
  ↓
AI: calls read_file("internal/auth/auth.go")
  ↓
ContextManager: adds to focusedPaths, regenerates context
  ↓
AI: sees file content, explains login logic
```

### Defensive Features

1. **Context Overflow Protection**
   - 当 focusedPaths 超过 maxTokens 时，LRU 自动淘汰最少使用的文件
   - UI 提示：`"Context full, 3 files evicted"`

2. **Giant Directory Tree Truncation**
   - maxDepth: 5 层（防止过深递归）
   - maxFilesPerDir: 50（防止超大目录）
   - 截断显示：`... (25 more files)`

3. **Performance Optimization**
   - 使用 `github.com/saracen/walker` 替代 `filepath.Walk`（并发遍历，快 10x）
   - Token 估算：`len(content) / 4`（粗略但足够，避免 BPE 编码开销）

## 3. LLM Interaction

### StreamEvent Structured Protocol

```go
type EventType int

const (
    EventContent  EventType = iota  // 普通文本
    EventToolCall                   // 工具调用
    EventError                      // 错误
    EventDone                       // 完成
)

type StreamEvent struct {
    Type     EventType
    Content  string         // 文本内容
    ToolCall *ToolCallDelta // 工具调用增量数据
}

type ToolCallDelta struct {
    ID        string
    Name      string
    Arguments string       // JSON 片段，流式拼接
}

type LLMProvider interface {
    ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    SetModel(model string)
}
```

### Agent Processing Flow (ReAct Loop)

```
用户输入 "帮我优化 main.go"
    ↓
Agent 调用 LLMProvider.ChatStream
    ↓
Event{Type: EventToolCall, Name: "read_file", Args: "main.go"}
    ↓
Agent 组装完整 ToolCall
    ↓
UI.RequestConfirm("Read main.go?") → 用户确认
    ↓
ToolExecutor.ReadFile("main.go") → 返回文件内容
    ↓
Agent 追加 ToolMessage 到 ConversationHistory
    ↓
【关键】Agent 自动发起新一轮 ChatStream (带着工具结果)
    ↓
LLM 看到文件内容，输出 EventContent "我发现了几个优化点..."
    ↓
Event{Type: EventToolCall, Name: "write_file", ...}
    ↓
UI.RequestConfirm("Apply changes?") → 用户确认
    ↓
ToolExecutor.WriteFile() → 返回成功
    ↓
Agent 追加 ToolMessage → 再次调用 ChatStream
    ↓
LLM 输出 EventContent "修改完成！主要优化了..."
    ↓
EventDone → 循环结束
```

### Agent Core Logic (Fixed Version)

```go
func (a *Agent) Run(ctx context.Context, userMessage string) error {
    // 1. 初始用户消息
    a.History.AddUserMessage(userMessage)

    // 大循环：ReAct 循环
    for {
        stream, err := a.LLM.ChatStream(ctx, a.History.BuildRequest())
        if err != nil { return err }

        var currentToolCalls []*ToolCall      // 支持并发工具调用
        var contentBuilder strings.Builder    // 收集 AI 回复文本

        // 小循环：处理流式响应
        for event := range stream {
            switch event.Type {
            case EventContent:
                a.UI.SendStream(event.Content)           // 实时显示
                contentBuilder.WriteString(event.Content) // 存入历史
            case EventToolCall:
                updateToolCalls(&currentToolCalls, event) // 流式组装
            case EventDone:
                break  // ⚠️ 不要 return！只是话完了
            }
        }

        // 2. ⚠️ 关键：保存 AI 的完整回复（思考 + 工具意图）
        fullContent := contentBuilder.String()
        a.History.AddAssistantMessage(fullContent, currentToolCalls)

        // 3. 执行工具（如果有）
        if len(currentToolCalls) > 0 {
            for _, call := range currentToolCalls {
                // 用户确认
                if !a.UI.RequestConfirm(call.Name, call.Arguments) {
                    a.History.AddToolOutput(call.ID, "User rejected")
                    continue
                }

                // 执行工具
                result := a.Tools.Execute(ctx, call)
                a.History.AddToolOutput(call.ID, result)
            }
            continue  // 下一轮 ChatStream
        }

        break  // 无工具调用，任务完成
    }
    return nil
}
```

### Three Critical Fixes

1. **不要在 EventDone 时 return**：流结束≠任务结束
2. **保存 AI 完整回复**：保证 ReAct 链条不断裂（Reasoning）
3. **支持并发工具调用**：处理 `[]*ToolCall`（GPT-4o, Claude 3.5 可并发调用工具）

### Prompt Strategy (Hard Constraints)

```go
const SystemPrompt = `You are Kore, an expert programming assistant and workflow automation agent.

Project Context:
{FILE_TREE}

Current Session Files:
{FOCUSED_FILES_SUMMARY}

Available Tools:
- read_file(path, line_start?, line_end?): Read file content
- write_file(path, content): Write file content (full overwrite)
- run_command(cmd): Execute shell command

Rules:
- When you need to read a file, call read_file immediately
- When ready to modify code, call write_file immediately
- DO NOT ask for permission. Just call the tool. The system handles confirmation automatically.
- Think step-by-step in chat before calling tools
- For large files (>100 lines), use line_start and line_end to read specific sections`
```

**Key Principle:** System handles confirmation (hard constraint), LLM only takes action. Avoid "ASK USER FIRST" in prompt - it breaks the automation flow.

### Ollama Compatibility

```go
type OllamaProvider struct {
    supportsNativeTools bool
}

// 对于不支持 Native Tool 的模型，注入 XML 工具格式
if !p.supportsNativeTools {
    systemPrompt += "\n\nUse XML tags for tools: <tool name='read_file'>path</tool>"
    // 解析 XML 转换为标准 ToolCall 结构
}
```

## 4. Tool Execution & Security

### Tool Definition & Registration

```go
// 基础工具接口
type Tool interface {
    Name() string
    Description() string
    Schema() string // 返回 OpenAI 兼容的 JSON Schema
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// 工具箱注册表
type ToolBox struct {
    tools map[string]Tool
}

func (tb *ToolBox) Register(tool Tool) {
    tb.tools[tool.Name()] = tool
}
```

### Core Tool Set (MVP)

#### 1. read_file - Read file content with range support

```go
type ReadFileTool struct {
    fs SecurityInterceptor
}

func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var params struct {
        Path      string `json:"path"`
        LineStart int    `json:"line_start,omitempty"`
        LineEnd   int    `json:"line_end,omitempty"`
    }
    json.Unmarshal(args, &params)

    safePath, err := t.fs.ValidatePath(params.Path)
    if err != nil { return "", err }

    content, _ := os.ReadFile(safePath)
    lines := strings.Split(string(content), "\n")

    // ⚠️ 边界保护
    start := params.LineStart
    end := params.LineEnd
    if start < 0 { start = 0 }
    if end > len(lines) || end == 0 { end = len(lines) }
    if start > end { start = end }

    return strings.Join(lines[start:end], "\n"), nil
}
```

#### 2. write_file - Full overwrite with diff confirmation

```go
type WriteFileTool struct {
    fs    SecurityInterceptor
    agent *Agent  // 用于请求 UI 确认
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var params struct {
        Path    string `json:"path"`
        Content string `json:"content"`
    }
    json.Unmarshal(args, &params)

    safePath, err := t.fs.ValidatePath(params.Path)
    if err != nil { return "", err }

    // 读取旧内容
    oldContent, _ := os.ReadFile(safePath)

    // 生成 Diff
    diffText := generateDiff(string(oldContent), params.Content)

    // ⚠️ 请求 UI 确认
    if !t.agent.UI.RequestConfirmWithDiff(params.Path, diffText) {
        return "User rejected write operation", nil
    }

    // 写入
    os.WriteFile(safePath, []byte(params.Content), 0644)
    return "File written successfully", nil
}
```

#### 3. run_command - Execute shell commands with timeout and truncation

```go
type RunCommandTool struct {
    fs      SecurityInterceptor
    timeout time.Duration
}

func (t *RunCommandTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var params struct {
        Cmd string `json:"cmd"`
    }
    json.Unmarshal(args, &params)

    if err := t.fs.ValidateCommand(params.Cmd); err != nil {
        return "", err
    }

    // 带超时的执行
    ctx, cancel := context.WithTimeout(ctx, t.timeout)
    defer cancel()

    // ⚠️ 跨平台兼容
    var cmd *exec.Cmd
    if runtime.GOOS == "windows" {
        cmd = exec.CommandContext(ctx, "cmd", "/C", params.Cmd)
    } else {
        cmd = exec.CommandContext(ctx, "sh", "-c", params.Cmd)
    }

    output, _ := cmd.CombinedOutput()

    // 截断过长输出 (max 10KB)
    if len(output) > 10000 {
        output = output[:10000]
        output = append(output, []byte("... (truncated)")...)
    }

    return string(output), nil
}
```

### Security Sandbox Layer

```go
type SecurityInterceptor struct {
    ProjectRoot  string
    BlockedCmds  []string  // ["rm", "sudo", "shutdown", "format", "del"]
    BlockedPaths []string  // [".git", ".env", "node_modules/.cache"]
}

// 1. 路径防御 (Path Traversal Protection)
func (s *SecurityInterceptor) ValidatePath(inputPath string) (string, error) {
    absPath, _ := filepath.Abs(filepath.Join(s.ProjectRoot, inputPath))

    // ⚠️ 需要加路径分隔符，防止 /home/user/project 匹配 /home/user/project-evil
    if !strings.HasPrefix(absPath, s.ProjectRoot+string(os.PathSeparator)) {
        return "", fmt.Errorf("SECURITY: Path traversal detected: %s", inputPath)
    }

    // 敏感路径防护
    for _, blocked := range s.BlockedPaths {
        if strings.Contains(absPath, blocked) {
            return "", fmt.Errorf("SECURITY: Access to sensitive path blocked: %s", blocked)
        }
    }

    return absPath, nil
}

// 2. 命令防御 (Command Injection Protection)
func (s *SecurityInterceptor) ValidateCommand(cmd string) error {
    cmd = strings.TrimSpace(cmd)
    parts := strings.Fields(cmd)
    if len(parts) == 0 { return fmt.Errorf("empty command") }

    baseCmd := parts[0]

    // 黑名单检查
    for _, blocked := range s.BlockedCmds {
        if baseCmd == blocked {
            return fmt.Errorf("SECURITY: Forbidden command: %s", baseCmd)
        }
    }

    return nil
}
```

### Diff Generation with Context

```go
func generateDiff(oldContent, newContent string) string {
    dmp := diffmatchpatch.New()
    diffs := dmp.DiffMain(oldContent, newContent, false)

    // 保留语义上的无损清理
    diffs = dmp.DiffCleanupSemanticLossless(diffs)

    // 渲染时保留 +/- 行周围的 3 行上下文
    return renderWithContext(diffs, 3)
}

func renderWithContext(diffs []diffmatchpatch.Diff, contextLines int) string {
    var builder strings.Builder
    contextBuffer := make([]string, 0, contextLines*2)

    for _, diff := range diffs {
        switch diff.Type {
        case diffmatchpatch.DiffDelete:
            // Flush context before deletion
            flushContext(&builder, contextBuffer)
            builder.WriteString(color.RedString("- " + diff.Text))

        case diffmatchpatch.DiffInsert:
            // Flush context before insertion
            flushContext(&builder, contextBuffer)
            builder.WriteString(color.GreenString("+ " + diff.Text))

        case diffmatchpatch.DiffEqual:
            // Collect as context
            lines := strings.Split(diff.Text, "\n")
            contextBuffer = append(contextBuffer, lines...)
            if len(contextBuffer) > contextLines*2 {
                // Keep only contextLines before and after
                contextBuffer = contextBuffer[len(contextBuffer)-contextLines:]
            }
        }
    }

    return builder.String()
}
```

## 5. Project Structure

**Standard Go Project Layout** (符合企业级工程规范)

```
kore/
├── cmd/                         # 【Entry Layer】主程序入口
│   └── kore/                    # Only main.go, no business logic
│       └── main.go
├── internal/                    # 【Private Business Layer】私有业务层
│   ├── core/                    # Core domain models
│   │   ├── agent.go             # Agent 核心逻辑 & ReAct loop
│   │   ├── context.go           # ContextManager
│   │   ├── history.go           # ConversationHistory
│   │   └── llm.go               # LLMProvider interface & StreamEvent
│   ├── adapters/                # Port adapters
│   │   ├── cli/
│   │   │   └── adapter.go       # CLIAdapter
│   │   ├── tui/
│   │   │   ├── adapter.go       # TUIAdapter
│   │   │   ├── model.go         # Bubble Tea Model
│   │   │   └── view.go          # Bubble Tea View
│   │   ├── openai/
│   │   │   └── provider.go      # OpenAI implementation
│   │   └── ollama/
│   │       └── provider.go      # Ollama implementation
│   ├── tools/                   # Tool execution framework
│   │   ├── executor.go          # ToolExecutor & ToolBox
│   │   ├── read_file.go         # read_file tool
│   │   ├── write_file.go        # write_file tool (with diff)
│   │   ├── run_command.go       # run_command tool
│   │   └── security.go          # SecurityInterceptor
│   └── infrastructure/          # Infrastructure services
│       ├── fs/
│       │   └── walker.go        # Fast directory traversal
│       └── config/
│           └── config.go        # Viper configuration
├── pkg/                         # 【Public Library Layer】公共库层
│   ├── logger/
│   │   └── logger.go            # Structured logging
│   └── utils/
│       └── utils.go             # Common utilities
├── api/                         # 【Protocol Layer】协议层
│   └── prompts/                 # System Prompts (NEVER hardcode)
│       ├── system.txt           # Base system prompt
│       └── tools.txt            # Tool descriptions
├── configs/                     # 【Configuration Layer】配置层
│   └── default.yaml             # Default config template
├── docs/                        # 【Documentation Layer】文档层
│   ├── adr/                     # Architecture Decision Records
│   │   ├── 001-choose-bubbletea.md
│   │   ├── 002-agent-architecture.md
│   │   └── ...
│   └── plans/                   # Design documents
│       └── 2026-01-16-kore-design.md
├── tests/                       # 【Testing Layer】测试层
│   ├── integration/             # Integration tests
│   ├── testdata/                # Test fixtures
│   └── evals/                   # AI evaluation tests
│       └── test_cases.json
├── scripts/                     # 【Build Scripts】构建脚本
│   ├── install.sh
│   └── build.sh
├── .golangci.yml                # Linter configuration
├── .github/
│   └── workflows/
│       └── ci.yml               # GitHub Actions CI
├── Makefile                     # Unified build entry point
├── go.mod
├── go.sum
├── CONTRIBUTING.md              # Development guide
└── README.md
```

### ✅ Mandatory Rules (强制规则)

1. **No circular dependencies** in `internal/` packages (禁止循环依赖)
2. **All Prompts MUST** be in `api/prompts/` (禁止在逻辑代码中硬编码 Prompt)
3. **`cmd/` contains ONLY main.go** - no business logic allowed
4. **`internal/` is private** - external projects cannot import it
5. **`pkg/` is public** - must be stable, well-documented, backward-compatible

## 6. Implementation Phases

### Phase 1: MVP Core (CLI + TUI)

**Week 1: Foundation**
- [ ] Initialize project structure (`go mod init kore`)
- [ ] Set up Cobra CLI framework
- [ ] Implement basic Agent.Run() without LLM
- [ ] Create UIInterface and CLIAdapter
- [ ] Implement basic ConversationHistory

**Week 2: LLM Integration**
- [ ] Implement LLMProvider interface
- [ ] Integrate OpenAI API client
- [ ] Implement StreamEvent protocol
- [ ] Build basic ChatStream with ReAct loop
- [ ] Add context cancellation support

**Week 3: Context Management**
- [ ] Implement ContextManager
- [ ] Add .gitignore parsing
- [ ] Implement directory tree generation
- [ ] Add focused paths tracking
- [ ] Implement LRU cache for token overflow

**Week 4: Tools & Security**
- [ ] Implement ToolExecutor framework
- [ ] Build read_file tool
- [ ] Build write_file tool with diff
- [ ] Build run_command tool
- [ ] Implement SecurityInterceptor

**Week 5: TUI Experience**
- [ ] Set up Bubble Tea framework
- [ ] Implement TUIAdapter
- [ ] Add Markdown rendering (glamour)
- [ ] Implement streaming display
- [ ] Add diff confirmation UI
- [ ] Implement spinner for "thinking" state

### Phase 2: Advanced Features

**Week 6-7: Ollama Integration**
- [ ] Implement OllamaProvider
- [ ] Add XML tool format fallback
- [ ] Support local model switching

**Week 8-9: Advanced Tools**
- [ ] Add search tool (ripgrep integration)
- [ ] Add list_files tool
- [ ] Implement parallel tool calling optimization

### Phase 3: GUI (Future)

**Week 10+: Wails Integration**
- [ ] Initialize Wails app
- [ ] Implement WailsAdapter
- [ ] Build React frontend
- [ ] Integrate Monaco Editor
- [ ] Add terminal emulator (xterm.js + PTY)

## 7. Configuration Management

```go
type Config struct {
    LLM struct {
        Provider    string  `yaml:"provider"    // "openai" or "ollama"`
        Model       string  `yaml:"model"`
        APIKey      string  `yaml:"api_key"`
        BaseURL     string  `yaml:"base_url"`
        Temperature float32 `yaml:"temperature"`
    } `yaml:"llm"`

    Context struct {
        MaxTokens      int `yaml:"max_tokens"`
        MaxTreeDepth   int `yaml:"max_tree_depth"`
        MaxFilesPerDir int `yaml:"max_files_per_dir"`
    } `yaml:"context"`

    Security struct {
        BlockedCmds  []string `yaml:"blocked_cmds"`
        BlockedPaths []string `yaml:"blocked_paths"`
    } `yaml:"security"`
}
```

Config file location: `~/.config/kore/config.yaml`

## 8. Success Criteria

- [ ] Can read project files and explain code
- [ ] Can modify files with user confirmation (diff view)
- [ ] Can run simple commands (ls, grep, go test)
- [ ] TUI provides smooth streaming experience
- [ ] Context manager handles large projects gracefully
- [ ] Security sandbox prevents malicious operations
- [ ] Supports both OpenAI and Ollama seamlessly

## References

- Bubble Tea: https://github.com/charmbracelet/bubbletea
- Cobra: https://github.com/spf13/cobra
- Wails: https://wails.io
- OpenAI Go SDK: https://github.com/sashabaranov/go-openai
- go-diff: https://github.com/sergi/go-diff
- walker: https://github.com/saracen/walker
