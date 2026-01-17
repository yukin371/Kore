# Kore 2.0 - 从开源项目学习的改进建议

**创建日期**: 2026-01-17
**基于项目**:
- [oh-my-opencode](https://github.com/code-yeongyu/oh-my-opencode)
- [opencode-ai/opencode](https://github.com/opencode-ai/opencode) (已归档)

---

## 目录

1. [oh-my-opencode 可吸收的特性](#oh-my-opencode-可吸收的特性)
2. [opencode-ai/opencode 可复用的代码](#opencode-aiopencode-可复用的代码)
3. [实施优先级](#实施优先级)
4. [架构调整建议](#架构调整建议)

---

## oh-my-opencode 可吸收的特性

### 🔥 高优先级（立即可用）

#### 1. Ralph Loop - 自引用开发循环 ⭐⭐⭐⭐⭐

**功能描述**：智能体持续执行直到任务完成，不会中途放弃。

**实现方式**：
```go
// internal/agent/ralph_loop.go
package agent

import (
    "context"
    "strings"
)

type RalphLoopConfig struct {
    Enabled           bool
    MaxIterations     int  // 默认 100
    DoneToken         string // 默认 "DONE"
}

func (lc *LoopController) RunRalphLoop(ctx context.Context, prompt string) error {
    for iteration := 0; iteration < lc.maxIterations; iteration++ {
        // 执行 Agent
        result := lc.Run(ctx, prompt)

        // 检查是否完成
        if lc.isTaskComplete(result) {
            return nil
        }

        // 检测是否应该停止（避免无限循环）
        if lc.shouldStop(result) {
            break
        }

        // 继续下一轮
        prompt = lc.generateNextPrompt(result)
    }

    return fmt.Errorf("达到最大循环次数 %d", lc.maxIterations)
}

func (lc *LoopController) isTaskComplete(result *AgentResult) bool {
    // 检查输出中是否包含 DONE 标记
    return strings.Contains(result.Output, lc.config.DoneToken)
}
```

**集成到 Kore 2.0**：
- 在 Agent Loop Controller 中添加 Ralph Loop 模式
- 通过配置开关启用：`agent.ralph_loop.enabled: true`
- 默认使用 `/ralph-loop` 命令触发

---

#### 2. 关键词魔法 - `ultrawork` 模式 ⭐⭐⭐⭐⭐

**功能描述**：检测提示中的关键词并自动激活专门模式。

**实现方式**：
```go
// internal/agent/keyword_detector.go
package agent

type KeywordDetector struct {
    keywords map[string]AgentMode
}

type AgentMode string

const (
    ModeNormal   AgentMode = "normal"
    ModeUltraWork AgentMode = "ultrawork"  // 最大性能模式
    ModeSearch    AgentMode = "search"     // 搜索模式
    ModeAnalyze  AgentMode = "analyze"    // 分析模式
)

func NewKeywordDetector() *KeywordDetector {
    return &KeywordDetector{
        keywords: map[string]AgentMode{
            "ultrawork": ModeUltraWork,
            "ulw":       ModeUltraWork,
            "search":    ModeSearch,
            "find":      ModeSearch,
            "analyze":   ModeAnalyze,
        },
    }
}

func (kd *KeywordDetector) Detect(prompt string) (AgentMode, bool) {
    promptLower := strings.ToLower(prompt)

    for keyword, mode := range kd.keywords {
        if strings.Contains(promptLower, keyword) {
            return mode, true
        }
    }

    return ModeNormal, false
}
```

**集成到 Kore 2.0**：
- 在 Loop Controller 开始时检测关键词
- 根据模式调整配置（并行度、工具权限等）
- 在用户提示中自动启用

---

#### 3. 上下文窗口监控 ⭐⭐⭐⭐⭐

**功能描述**：监控 token 使用率，在达到阈值时提醒或自动压缩。

**实现方式**：
```go
// internal/agent/context_monitor.go
package agent

type ContextMonitor struct {
    warningThreshold float64  // 0.7 = 70%
    compressThreshold float64  // 0.85 = 85%
}

func (cm *ContextMonitor) Check(history *ConversationHistory, modelMaxTokens int) MonitorAction {
    usage := cm.calculateUsage(history, modelMaxTokens)

    if usage >= cm.compressThreshold {
        return ActionCompress  // 自动压缩
    }

    if usage >= cm.warningThreshold {
        return ActionWarn     // 警告用户
    }

    return ActionNone
}

func (cm *ContextMonitor) calculateUsage(history *ConversationHistory, maxTokens int) float64 {
    totalTokens := 0
    for _, msg := range history.Messages {
        totalTokens += estimateTokens(msg.Content)
    }
    return float64(totalTokens) / float64(maxTokens)
}
```

**集成到 Kore 2.0**：
- 在 Agent Loop 的 Observe 阶段检查上下文使用率
- 达到 85% 时触发预防性压缩
- 在 TUI 状态栏显示上下文使用率

---

### 🌟 中优先级（Phase 2-3 考虑）

#### 4. 专业智能体系统 ⭐⭐⭐⭐

**功能描述**：多个专业智能体，各司其职。

**智能体映射**：
- **Oracle** (GPT-5.2): 架构、代码审查
- **Librarian** (GLM-4.7): 文档、代码库探索
- **Explore** (Gemini Flash): 快速代码库探索
- **Frontend Engineer** (Gemini Pro): UI 开发

**集成到 Kore 2.0**：
```go
// internal/agent/specialist.go
type SpecialistAgent struct {
    Name      string
    Model     string
    Expertise []string
    Temperature float32
}

var specialists = []SpecialistAgent{
    {Name: "oracle", Model: "openai/gpt-5.2", Expertise: []string{"architecture", "review"}},
    {Name: "librarian", Model: "glm-4.7-free", Expertise: []string{"documentation", "exploration"}},
    {Name: "explore", Model: "google/gemini-3-flash", Expertise: []string{"search", "pattern-matching"}},
}
```

---

#### 5. Todo 继续执行器 ⭐⭐⭐⭐

**功能描述**：强制智能体完成未完成的 TODO 项。

**实现方式**：
```go
type TodoContinuator struct {
    enabled bool
}

func (tc *TodoContinuator) Enforce(history *ConversationHistory) error {
    todos := tc.extractTodos(history.Messages)

    for _, todo := range todos {
        if !todo.Done {
            return fmt.Errorf("任务未完成: %s", todo.Description)
        }
    }

    return nil
}
```

---

#### 6. AGENTS.md 自动注入 ⭐⭐⭐⭐

**功能描述**：读取文件时自动向上遍历目录，注入所有 AGENTS.md。

**实现方式**：
```go
// internal/context/injector.go
func (ci *ContextInjector) InjectForFile(filePath string) []string {
    var contexts []string

    // 向上遍历到项目根目录
    dir := filepath.Dir(filePath)
    for dir != ci.projectRoot {
        agentsPath := filepath.Join(dir, "AGENTS.md")
        if _, err := os.Stat(agentsPath); err == nil {
            content := ci.readFile(agentsPath)
            contexts = append(contexts, content)
        }
        dir = filepath.Dir(dir)
    }

    return contexts
}
```

---

### 🌱 低优先级（未来考虑）

#### 7. MCP 支持 ⭐⭐⭐

**功能描述**：支持 Model Context Protocol，集成外部工具。

**内置 MCP**：
- **websearch** (Exa AI): 实时网络搜索
- **context7**: 官方文档查询
- **grep_app**: GitHub 代码搜索

**集成到 Kore 2.0**：
- 作为额外的工具类型
- 通过配置文件启用/禁用

---

#### 8. 多模态化 - 节省 Token ⭐⭐⭐

**功能描述**：让另一个智能体提取文件内容，而不是直接读取大文件。

---

## opencode-ai/opencode 可复用的代码

### 📦 可直接复用的组件

#### 1. TUI 组件库 ⭐⭐⭐⭐⭐

**项目位置**: `internal/tui/`

**可复用组件**：

```go
// 组件列表
- viewport.go     // 消息视口（滚动、分页）
- editor.go       // 文本编辑器
- session.go      // 会话管理界面
- model.go        // Bubble Tea Model
- styles.go       // 样式定义（lipgloss）
- input.go        // 输入框组件
- dialog.go       // 对话框
```

**复用方式**：
```bash
# 直接引用（如果许可证兼容）
go get github.com/opencode-ai/opencode@latest

# 或复制代码到 internal/client/tui/opencode/
```

**推荐复用**：
- **Viewport 组件**：消息滚动和分页
- **Editor 组件**：多行文本输入
- **Session 管理界面**：会话切换、历史记录

---

#### 2. LSP 客户端实现 ⭐⭐⭐⭐⭐

**项目位置**: `internal/lsp/`

**基础实现**：
- 基于 `mcp-language-server`
- JSON-RPC 通信
- 支持 Diagnostics、Completion、Hover、Definition

**复用价值**：
- 可直接参考其 LSP 客户端架构
- 学习其 Stdio 通信实现
- 借鉴其错误处理和重连逻辑

---

#### 3. 配置管理 ⭐⭐⭐⭐

**项目位置**: `internal/config/`

**特性**：
- 多位置配置加载
- JSONC 支持
- Schema 验证
- 环境变量展开

**复用方式**：
```go
// 参考其配置加载逻辑
// internal/config/loader.go
func LoadConfig(configPath string) (*Config, error) {
    // 尝试多个位置
    paths := []string{
        configPath,
        filepath.Join(os.Getenv("HOME"), ".config", "kore", "config.yaml"),
        "./.kore/config.yaml",
    }

    for _, path := range paths {
        if cfg, err := loadConfigFile(path); err == nil {
            return cfg, nil
        }
    }

    return DefaultConfig(), nil
}
```

---

#### 4. 数据库层 ⭐⭐⭐

**项目位置**: `internal/db/`

**特性**：
- SQLite 存储会话和消息
- 数据库迁移
- 事务支持

**可借鉴**：
- 表结构设计
- 索引策略
- 查询优化

---

## 实施优先级

### 时间线与负责人

| Phase | 目标日期 | 负责人 |
|------|----------|--------|
| Phase 1 | 2026-01-24 | TBD |
| Phase 2 | 2026-02-07 | TBD |
| Phase 3 | 2026-02-21 | TBD |
| Phase 4 | TBD | TBD |

### Phase 1（立即实施）
1. [ ] 上下文窗口监控
2. [ ] Ralph Loop 基础实现
3. [ ] 关键词检测（`ultrawork`）

### Phase 2（1-2 周内）
4. [x] Todo 继续执行器
5. [x] AGENTS.md 自动注入
6. [x] 复用 TUI Viewport 组件

### Phase 3（2-4 周内）
7. [ ] 专业智能体系统（Oracle、Librarian）
8. [ ] LSP 工具增强（prepare_rename、rename）
9. [ ] MCP 支持

### Phase 4（未来考虑）
10. [ ] 多模态化
11. [ ] 完整的 Claude Code 兼容层

---

## 验收与测试

- Ralph Loop：迭代上限生效；DONE 触发完成；触发 shouldStop 可终止；无死循环
- Context Monitor：阈值触发准确；压缩后上下文可读；TUI 状态栏数值正确
- 关键词检测：大小写不敏感；误触发率可接受；模式切换可回退
- AGENTS.md 注入：向上遍历到项目根；去重；文件过大有策略（截断/摘要）
- LSP 复用：最小功能集可用（Completion/Diagnostics）；失败可降级

## 风险与回滚

- 无限制循环风险：默认最大迭代 + shouldStop 终止；日志可观测
- 上下文压缩误伤：保留关键轮次；支持手动回滚到未压缩版本
- 模式误触发：关键词白名单可配置；提供显式关闭

## 合规与来源记录

- 复用前确认许可证类型（MIT/Apache 等）与兼容性
- 复制代码时保留原始版权声明与 NOTICE
- 在 `internal/client/tui/opencode-compat/README.md` 记录来源、版本与变更

---

## 架构调整建议

### 1. Agent 增加模式枚举

```go
// internal/agent/mode.go
type ExecutionMode string

const (
    ModeNormal   ExecutionMode = "normal"
    ModeUltraWork ExecutionMode = "ultrawork"
    ModeSearch   ExecutionMode = "search"
    ModeAnalyze  ExecutionMode = "analyze"
)
```

### 2. 增加配置结构

```yaml
# config.yaml
agent:
  mode: "normal"  # normal, ultrawork, search, analyze

  ralph_loop:
    enabled: true
    max_iterations: 100
    done_token: "DONE"

  context_monitor:
    warning_threshold: 0.7
    compress_threshold: 0.85

  specialists:
    oracle:
      model: "openai/gpt-5.2"
      enabled: true
    librarian:
      model: "glm-4.7-free"
      enabled: true
```

---

## 复用 Go 代码的具体步骤

### 方案 A：Go Module 引用（推荐）

**优点**：自动获取更新

```bash
# 在 go.mod 中添加
go get github.com/opencode-ai/opencode@latest

# 引入代码
import (
    opencodetui "github.com/opencode-ai/opencode/internal/tui"
)
```

**注意事项**：
- 检查许可证兼容性（opencode-ai/opencode 已归档，可能使用 MIT）
- 需要处理依赖冲突

---

### 方案 B：代码复制（更可控）

**优点**：完全可控，可定制

```bash
# 创建 opencode-compat 目录
mkdir -p internal/client/tui/opencode-compat

# 复制需要的文件
# viewport.go, editor.go, styles.go 等
```

**具体文件列表**：
```
internal/client/tui/opencode-compat/
├── viewport.go       # 从 opencode 复制
├── editor.go         # 从 opencode 复制
├── styles.go         # 从 opencode 复制
└── README.md         # 记录来源
```

---

### 方案 C：混合方案（最佳）

**策略**：
1. TUI 组件：复制并适配
2. LSP 客户端：参考实现，重写
3. 配置管理：参考架构，实现自己的版本

**理由**：
- TUI 组件需要深度定制以匹配 Kore 的设计
- LSP 客户端需要适配 Kore 的架构
- 配置管理需要适配 Kore 的需求

---

## 总结

### 立即可用（Phase 1）

| 特性 | 来源 | 复杂度 | 价值 |
|------|------|--------|------|
| 上下文窗口监控 | oh-my-opencode | 低 | ⭐⭐⭐⭐⭐ |
| Ralph Loop | oh-my-opencode | 低 | ⭐⭐⭐⭐⭐ |
| 关键词魔法 | oh-my-opencode | 低 | ⭐⭐⭐⭐⭐ |
| Todo 继续执行器 | oh-my-opencode | 中 | ⭐⭐⭐⭐ |
| AGENTS.md 注入 | oh-my-opencode | 中 | ⭐⭐⭐⭐ |

### 值得复用（Phase 2）

| 组件 | 来源 | 复用方式 | 价值 |
|------|------|---------|------|
| TUI Viewport | opencode-ai | 复制代码 | ⭐⭐⭐⭐ |
| LSP 客户端 | opencode-ai | 参考架构 | ⭐⭐⭐⭐ |
| 配置管理 | opencode-ai | 参考架构 | ⭐⭐⭐ |

### 长期考虑

| 特性 | 来源 | 优先级 |
|------|------|--------|
| 专业智能体系统 | oh-my-opencode | Phase 3 |
| MCP 支持 | oh-my-opencode | Phase 4 |
| 多模态化 | oh-my-opencode | Phase 4 |

---

**文档版本**: 1.0
**最后更新**: 2026-01-17
**维护者**: Kore Team
