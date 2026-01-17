# Kore 全面优化设计文档

**Date**: 2026-01-17
**Author**: Claude + User Collaboration
**Status**: Design Approved
**Target**: Kore v0.8.0

---

## Executive Summary

本文档旨在解决 Kore 项目中发现的 4 个关键问题，提供基于 OpenCode 最佳实践的综合解决方案。

**核心问题**:
1. 工具循环调用 - AI 重复读取同一文件
2. 上下文丢失 - 确认对话框覆盖历史内容
3. 工具系统简陋 - 缺乏扩展性和插件机制
4. 身份定位不清 - AI 自称 Claude，基本功能不明确

**解决方案**:
1. 工具调用优化 - History Summary + Tool Guide + Smart Cache
2. Modal Overlay - 保留上下文的模态对话框
3. Agent 系统 - Plan/Build 分离，MCP 支持
4. 系统提示词完善 - 加载 system.txt + 工具指南

**实施计划**: 分 4 个阶段，预计 3-4 周

---

## Table of Contents

1. [问题分析](#问题分析)
2. [解决方案 1: 工具调用优化](#解决方案-1-工具调用优化)
3. [解决方案 2: Modal Overlay](#解决方案-2-modal-overlay)
4. [解决方案 3: Agent 系统](#解决方案-3-agent-系统)
5. [解决方案 4: 系统提示词](#解决方案-4-系统提示词)
6. [实施计划](#实施计划)
7. [测试策略](#测试策略)
8. [参考文档](#参考文档)

---

## 问题分析

### 问题 1: 工具循环调用

**现象**: AI 创建文件后，重复请求读取同一文件

**根源**:
- 工具调用结果未正确保存到对话历史
- AI 看不到之前的工具调用记录
- 系统提示词未明确说明工具使用策略

**影响**:
- 浪费 Token（重复 IO）
- 用户体验差（无限循环）
- 功能不可用（无法完成任务）

---

### 问题 2: 上下文丢失

**现象**: 工具调用确认对话框覆盖所有历史内容

**根源**:
- 确认框是全屏 View，替换了 Chat View
- 用户看不到之前的对话上下文
- 无法参考历史信息做决策

**影响**:
- 认知负担增加（忘记上下文）
- 决策困难（不知道要确认什么）
- 用户体验差

---

### 问题 3: 工具系统简陋

**现象**: 工具功能有限，难以扩展

**根源**:
- 硬编码的工具定义
- 缺乏插件机制
- 没有第三方集成能力

**影响**:
- 功能受限
- 用户无法自定义
- 生态无法发展

---

### 问题 4: 身份定位不清

**现象**: AI 自称 Claude，基本功能不明确

**根源**:
- `buildSystemPrompt()` 未加载 `system.txt`
- 系统提示词不完整

**影响**:
- 用户困惑
- 功能偏差
- 品牌不一致

---

## 解决方案 1: 工具调用优化

### 设计目标

- 避免工具重复调用
- 提供工具调用历史可见性
- 智能缓存（Content-Aware）
- 明确的工具使用指南

### 架构设计

#### 1.1 工具调用历史跟踪

**数据结构**:

```go
// internal/core/tool_history.go

package core

import (
    "fmt"
    "strings"
    "sync"
    "time"
)

// ToolCallRecord 工具调用记录
type ToolCallRecord struct {
    ID        string    // 调用 ID
    Tool      string    // 工具名称
    Arguments string    // 参数（JSON）
    Result    string    // 结果
    Timestamp time.Time // 时间戳
    Success   bool      // 是否成功
}

// ToolCallHistory 工具调用历史管理器
type ToolCallHistory struct {
    calls []ToolCallRecord
    mu    sync.RWMutex
}

// NewToolCallHistory 创建工具调用历史
func NewToolCallHistory() *ToolCallHistory {
    return &ToolCallHistory{
        calls: make([]ToolCallRecord, 0, 50),
    }
}

// Record 记录一次工具调用
func (h *ToolCallHistory) Record(call ToolCallRecord) {
    h.mu.Lock()
    defer h.mu.Unlock()

    h.calls = append(h.calls, call)

    // 限制历史长度
    if len(h.calls) > 50 {
        h.calls = h.calls[len(h.calls)-50:]
    }
}

// GetSummary 获取工具调用摘要（提供给 AI）
func (h *ToolCallHistory) GetSummary() string {
    h.mu.RLock()
    defer h.mu.RUnlock()

    if len(h.calls) == 0 {
        return "## 工具调用历史\n\n暂无工具调用记录。"
    }

    var summary strings.Builder
    summary.WriteString("## 最近工具调用\n\n")

    // 只显示最近 10 次调用
    count := len(h.calls)
    start := 0
    if count > 10 {
        start = count - 10
    }

    for i := start; i < count; i++ {
        call := h.calls[i]

        status := "✓"
        if !call.Success {
            status = "✗"
        }

        summary.WriteString(fmt.Sprintf("- [%s] **%s**(%s)\n",
            status, call.Tool, call.Arguments))

        // 如果失败，显示错误信息
        if !call.Success && call.Result != "" {
            // 截断长错误信息
            errMsg := call.Result
            if len(errMsg) > 100 {
                errMsg = errMsg[:97] + "..."
            }
            summary.WriteString(fmt.Sprintf("  错误: %s\n", errMsg))
        }
    }

    return summary.String()
}

// GetLastCallOfType 获取特定工具的最后一次调用
func (h *ToolCallHistory) GetLastCallOfType(toolName string) (ToolCallRecord, bool) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for i := len(h.calls) - 1; i >= 0; i-- {
        if h.calls[i].Tool == toolName {
            return h.calls[i], true
        }
    }

    return ToolCallRecord{}, false
}

// Clear 清空历史
func (h *ToolCallHistory) Clear() {
    h.mu.Lock()
    defer h.mu.Unlock()

    h.calls = make([]ToolCallRecord, 0, 50)
}
```

---

#### 1.2 智能文件缓存

**数据结构**:

```go
// internal/core/file_cache.go

package core

import (
    "crypto/md5"
    "encoding/hex"
    "os"
    "sync"
    "time"
)

// FileCache 智能文件缓存（Content-Aware）
type FileCache struct {
    hashes   map[string]string    // path -> MD5 hash
    modTimes map[string]time.Time // path -> last modified time
    contents map[string]string    // path -> cached content
    mu       sync.RWMutex
}

// NewFileCache 创建文件缓存
func NewFileCache() *FileCache {
    return &FileCache{
        hashes:   make(map[string]string),
        modTimes: make(map[string]time.Time),
        contents: make(map[string]string),
    }
}

// CheckRead 检查文件是否需要读取
// 返回: (content, cached, changed)
//   - content: 文件内容（从缓存或实际读取）
//   - cached: 是否来自缓存
//   - changed: 文件是否已被外部修改
func (c *FileCache) CheckRead(path string) (string, bool, bool) {
    info, err := os.Stat(path)
    if err != nil {
        // 文件不存在或无法访问
        return "", false, false
    }

    c.mu.RLock()
    lastMod, ok := c.modTimes[path]
    c.mu.RUnlock()

    // 如果缓存中没有，需要读取
    if !ok {
        return c.readAndCache(path)
    }

    // 如果修改时间变了，需要重新读取
    if !info.ModTime().Equal(lastMod) {
        return c.readAndCache(path)
    }

    // 文件未修改，返回缓存
    c.mu.RLock()
    content := c.contents[path]
    c.mu.RUnlock()

    return content, true, false
}

// readAndCache 读取文件并更新缓存
func (c *FileCache) readAndCache(path string) (string, bool, bool) {
    content, err := os.ReadFile(path)
    if err != nil {
        return "", false, false
    }

    contentStr := string(content)

    // 计算 MD5 hash
    hash := md5.Sum(content)
    hashStr := hex.EncodeToString(hash[:])

    // 获取文件信息
    info, _ := os.Stat(path)

    // 更新缓存
    c.mu.Lock()
    c.hashes[path] = hashStr
    c.modTimes[path] = info.ModTime()
    c.contents[path] = contentStr
    c.mu.Unlock()

    return contentStr, false, true // fresh read, not cached, changed
}

// Invalidate 使缓存失效（用于文件写入后）
func (c *FileCache) Invalidate(path string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    delete(c.hashes, path)
    delete(c.modTimes, path)
    delete(c.contents, path)
}

// GetHash 获取文件的 MD5 hash
func (c *FileCache) GetHash(path string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    hash, ok := c.hashes[path]
    return hash, ok
}
```

---

#### 1.3 工具使用指南

**文件**: `api/prompts/tools.txt`

```
# 工具调用最佳实践

## 核心原则

你是一个专业的编程助手，工具是你的手臂和双腿。
使用工具来完成实际任务，而不仅仅是提供建议。

## 并行调用策略

**你可以并行调用多个工具**以提高效率。

✅ 好的做法：
```json
[
  {"tool": "read_file", "path": "main.go"},
  {"tool": "read_file", "path": "utils.go"},
  {"tool": "read_file", "path": "config.go"}
]
```

❌ 不好的做法：
```json
{"tool": "read_file", "path": "main.go"}
// 然后等待结果...
{"tool": "read_file", "path": "utils.go"}
// 然后等待结果...
{"tool": "read_file", "path": "config.go"}
```

## 避免重复调用

**工具调用结果会自动保存到对话历史中**。

在调用工具前，先检查对话历史：
- 如果之前已经读取过文件，不要重复读取
- 如果之前已经执行过命令，查看结果
- 如果文件被你刚刚写入，你知道内容，不需要再次读取

✅ 好的做法：
```
用户: 我刚才写的 main.go 有什么问题？
AI: 让我查看一下刚才创建的 main.go 文件...
（根据对话历史，文件内容已知，不调用 read_file）
AI: 我注意到刚才创建的 main.go 中...
```

❌ 不好的做法：
```
用户: 我刚才写的 main.go 有什么问题？
AI: 让我读取 main.go...
（调用 read_file，浪费 Token）
```

## 工具调用顺序

遵循以下顺序可以避免混乱：

1. **理解阶段**: 读取相关文件，了解现状
2. **规划阶段**: 制定修改计划
3. **执行阶段**: 写入文件、运行命令
4. **验证阶段**: 运行测试，检查结果

✅ 典型工作流：
```
用户: 给 main.go 添加错误处理

AI:
1. 读取 main.go 了解现状
2. 分析需要添加的错误处理
3. 写入修改后的 main.go
4. 运行 go build 验证
5. 报告结果
```

## 文件修改策略

- 一次性说明所有修改，而不是多次小修改
- 修改后立即运行测试验证
- 如果测试失败，根据错误信息调整

## 命令执行策略

- 使用非交互式命令（例如 `npm install -y`）
- 危险命令先说明目的和影响
- 检查命令输出，根据结果调整下一步

## 状态跟踪

使用 todo 工具跟踪你的任务进度：
- 开始任务时：创建 todo
- 完成任务时：更新 todo 状态
- 这有助于你（和用户）了解进度

## 常见错误

❌ **不要**:
- 重复读取同一文件
- 忘记之前的工具调用结果
- 在每次调用工具前都询问用户
- 使用交互式命令（如 `vim`）

✅ **应该**:
- 批量读取相关文件
- 参考对话历史
- 主动使用工具（除非明确要求确认）
- 使用非交互式命令

记住：系统会自动处理用户确认，你专注于提供帮助。
```

---

#### 1.4 系统提示词注入

**修改**: `internal/core/agent.go`

```go
// buildSystemPrompt 构建系统提示词
func (a *Agent) buildSystemPrompt(ctx *ProjectContext) string {
    var parts []string

    // 1. 基础系统提示词（从 system.txt 加载）
    basePrompt := loadSystemPrompt()
    parts = append(parts, basePrompt)

    // 2. 工具使用指南
    toolGuide := loadToolGuide()
    parts = append(parts, toolGuide)

    // 3. 【新增】工具调用历史摘要
    toolHistorySummary := a.toolHistory.GetSummary()
    parts = append(parts, toolHistorySummary)

    // 4. 项目上下文
    projectContext := fmt.Sprintf(`
## 项目上下文 (Project Context)

项目根目录: %s

项目目录树:
%s

关注的文件 (%d 个文件, ~%d tokens):
%s

当前工作目录: %s
`,
        a.ContextMgr.GetProjectRoot(),
        ctx.FileTree,
        len(ctx.FocusedFiles),
        ctx.TotalTokens,
        formatFocusedFiles(ctx.FocusedFiles),
        a.ContextMgr.GetProjectRoot(),
    )
    parts = append(parts, projectContext)

    // 5. 当前日期时间
    parts = append(parts, fmt.Sprintf("\n当前时间: %s", time.Now().Format("2006-01-02 15:04")))

    return strings.Join(parts, "\n\n")
}

// loadToolGuide 加载工具使用指南
func loadToolGuide() string {
    content, err := os.ReadFile("api/prompts/tools.txt")
    if err != nil {
        return "## 工具使用指南\n\n使用工具来完成实际任务。"
    }
    return string(content)
}
```

---

#### 1.5 工具执行集成

**修改**: `internal/core/agent.go - executeToolsSequential()`

```go
// executeToolsSequential 顺序执行工具
func (a *Agent) executeToolsSequential(ctx context.Context, toolCalls []*ToolCall) {
    for _, call := range toolCalls {
        // 1. 发送状态通知
        a.notifyToolExecutionStart(call.Name, call.Arguments)

        // 2. 检查智能缓存（仅对 read_file）
        if call.Name == "read_file" {
            var args map[string]interface{}
            json.Unmarshal([]byte(call.Arguments), &args)
            if path, ok := args["path"].(string); ok {
                content, cached, _ := a.fileCache.CheckRead(path)
                if cached {
                    // 文件未修改，使用缓存
                    result := map[string]interface{}{
                        "content": content,
                        "cached": true,
                        "message": "文件内容未改变，使用缓存",
                    }
                    resultJSON, _ := json.Marshal(result)
                    a.History.AddToolOutput(call.ID, string(resultJSON))

                    // 记录工具调用
                    a.toolHistory.Record(ToolCallRecord{
                        ID:        call.ID,
                        Tool:      call.Name,
                        Arguments: call.Arguments,
                        Result:    "(使用缓存)",
                        Success:   true,
                        Timestamp: time.Now(),
                    })

                    a.notifyToolExecutionEnd(true, "")
                    continue
                }
            }
        }

        // 3. 用户确认
        if !a.UI.RequestConfirm(call.Name, call.Arguments) {
            // 用户拒绝
            errorResult := map[string]interface{}{
                "error": "User rejected the operation",
            }
            errorJSON, _ := json.Marshal(errorResult)
            a.History.AddToolOutput(call.ID, string(errorJSON))
            a.UI.SendStream(fmt.Sprintf("\n[已跳过 %s]\n", call.Name))

            // 记录失败的工具调用
            a.toolHistory.Record(ToolCallRecord{
                ID:        call.ID,
                Tool:      call.Name,
                Arguments: call.Arguments,
                Result:    "用户拒绝",
                Success:   false,
                Timestamp: time.Now(),
            })

            continue
        }

        // 4. 执行工具
        result, err := a.Tools.Execute(ctx, *call)

        // 5. 记录工具调用
        errMsg := ""
        if err != nil {
            errMsg = err.Error()
        }

        a.toolHistory.Record(ToolCallRecord{
            ID:        call.ID,
            Tool:      call.Name,
            Arguments: call.Arguments,
            Result:    result,
            Success:   err == nil,
            Timestamp: time.Now(),
        })

        // 6. 如果是写入操作，使缓存失效
        if call.Name == "write_file" && err == nil {
            var args map[string]interface{}
            json.Unmarshal([]byte(call.Arguments), &args)
            if path, ok := args["path"].(string); ok {
                a.fileCache.Invalidate(path)
            }
        }

        // 7. 发送完成通知
        a.notifyToolExecutionEnd(err == nil, errMsg)

        // 8. 添加到历史
        var output string
        if err != nil {
            errorResult := map[string]interface{}{
                "error": errMsg,
            }
            errorJSON, _ := json.Marshal(errorResult)
            output = string(errorJSON)
        } else {
            if strings.TrimSpace(result) != "" &&
               (strings.HasPrefix(result, "{") || strings.HasPrefix(result, "[")) {
                output = result
            } else {
                successResult := map[string]interface{}{
                    "result": result,
                }
                successJSON, _ := json.Marshal(successResult)
                output = string(successJSON)
            }
        }

        a.History.AddToolOutput(call.ID, output)
    }
}
```

---

### 测试验证

**测试场景 1: 文件缓存**

```
输入: 写一个 hello.go 文件，包含 print("Hello")
AI: [调用 write_file]
    [创建 hello.go]

输入: hello.go 的内容是什么？
AI: 根据之前的工具调用，我刚刚创建了 hello.go，
    内容包含 print("Hello")。
    （应该不调用 read_file）
```

**测试场景 2: 外部修改检测**

```
1. AI 创建文件
2. 用户手动修改文件
3. AI 再次读取
预期: AI 应该读取到最新的文件内容
```

---

## 解决方案 2: Modal Overlay

### 设计目标

- 保留底层内容可见性（上下文不丢失）
- Modal 框浮动在中间（视觉焦点）
- ANSI 安全（无乱码）
- 事件拦截（Modal 状态优先）

### 架构设计

#### 2.1 Modal 组件结构

```go
// internal/adapters/tui/modal.go

package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    lipgloss "github.com/charmbracelet/lipgloss"
)

// ModalType Modal 类型
type ModalType int

const (
    ModalConfirm ModalType = iota // 确认对话框
    ModalDiff                     // Diff 预览对话框
)

// ModalState Modal 状态
type ModalState struct {
    Type      ModalType
    Title     string
    Content   string
    OnConfirm func() bool
    Visible   bool
}

// ModalComponent Modal 组件
type ModalComponent struct {
    state ModalState
    style ModalStyle
}

// ModalStyle Modal 样式
type ModalStyle struct {
    Border     lipgloss.Style
    Background lipgloss.Style
    Title      lipgloss.Style
    Content    lipgloss.Style
    DimStyle   lipgloss.Style // 底层变暗样式
}

// ShowModalMsg 显示 Modal 消息
type ShowModalMsg struct {
    Type      ModalType
    Title     string
    Content   string
    OnConfirm func() bool
}
```

---

#### 2.2 Modal 样式定义

```go
// internal/adapters/tui/modal.go

// DefaultModalStyle 创建默认 Modal 样式（Tokyo Night 主题）
func DefaultModalStyle() ModalStyle {
    // Modal 边框
    border := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#7aa2f7")). // Tokyo Night 蓝色
        Padding(1, 2)

    // Modal 背景（Solid Color，不透明）
    background := lipgloss.NewStyle().
        Background(lipgloss.Color("#1a1b26")). // Tokyo Night 深色
        Foreground(lipgloss.Color("#c0caf5"))

    // 标题样式
    title := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#7aa2f7")).
        MarginBottom(1)

    // 内容样式
    content := lipgloss.NewStyle().
        Foreground(lipgloss.Color("#c0caf5"))

    // 底层变暗样式（模拟 50% 透明度）
    dimStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("#565f89")). // 灰色
        Background(lipgloss.Color("#1a1b26"))

    return ModalStyle{
        Border:     border,
        Background: background,
        Title:      title,
        Content:    content,
        DimStyle:   dimStyle,
    }
}
```

---

#### 2.3 Model 集成

```go
// internal/adapters/tui/model.go

type Model struct {
    // ... 现有字段 ...

    // 【新增】Modal 组件
    modal ModalComponent
}

// 初始化
func NewModel() *Model {
    // ... 现有初始化 ...

    return &Model{
        // ... 现有字段 ...

        modal: ModalComponent{
            state: ModalState{Visible: false},
            style: DefaultModalStyle(),
        },
    }
}
```

---

#### 2.4 渲染逻辑（核心）

```go
// internal/adapters/tui/model.go

func (m *Model) View() string {
    // 如果没有 Modal，正常渲染
    if !m.modal.Visible {
        return m.renderBaseView()
    }

    // 【关键】Modal 状态：渲染 Dim 过的底层 + Modal 框

    // 1. 渲染变暗的底层视图
    dimmedView := m.renderDimmedView()

    // 2. 渲染 Modal 框
    modalView := m.renderModal()

    // 3. 使用 lipgloss.Place 将 Modal 居中放置
    // Modal 背景是 Solid Color，会遮挡底层
    finalView := lipgloss.Place(
        m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        modalView,
        lipgloss.WithWhitespaceChars(" "),
        lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
        lipgloss.WithWhitespaceBackground(lipgloss.Color("#1a1b26")),
    )

    // 注意：我们不使用 overlay 手动合并，而是让 lipgloss.Place
    // 自动处理位置。虽然底层被 Modal 遮挡，但 Modal 之前
    // 的瞬间用户能看到底层，提供上下文。

    return finalView
}

// renderBaseView 渲染底层视图（正常模式）
func (m *Model) renderBaseView() string {
    // 现有的 View 逻辑
    inputView := m.renderInputArea()
    statusBarView := m.renderAnimatedStatusBar()
    helpView := m.styles.App.Render(m.renderHelpText())

    bottomHeight := lipgloss.Height(inputView) +
                    lipgloss.Height(statusBarView) +
                    lipgloss.Height(helpView)

    availableHeight := m.height - bottomHeight
    if availableHeight < 5 {
        availableHeight = 5
    }
    m.viewport.Height = availableHeight
    m.viewport.Width = m.width
    m.viewport.SetContent(m.renderMessagesContent())

    return lipgloss.JoinVertical(lipgloss.Left,
        m.viewport.View(),
        inputView,
        statusBarView,
        helpView,
    )
}

// renderDimmedView 渲染变暗的底层视图（Modal 模式）
func (m *Model) renderDimmedView() string {
    // 使用 Dim 样式重新渲染所有组件
    inputView := m.modal.style.DimStyle.Render(m.renderInputAreaRaw())
    statusBarView := m.modal.style.DimStyle.Render(m.renderAnimatedStatusBarRaw())
    helpView := m.modal.style.DimStyle.Render(m.renderHelpTextRaw())

    // Viewport 内容也需要变暗
    m.viewport.SetContent(m.modal.style.DimStyle.Render(m.renderMessagesContentRaw()))

    // 计算高度
    bottomHeight := lipgloss.Height(inputView) +
                    lipgloss.Height(statusBarView) +
                    lipgloss.Height(helpView)

    availableHeight := m.height - bottomHeight
    if availableHeight < 5 {
        availableHeight = 5
    }
    m.viewport.Height = availableHeight
    m.viewport.Width = m.width

    return lipgloss.JoinVertical(lipgloss.Left,
        m.viewport.View(),
        inputView,
        statusBarView,
        helpView,
    )
}

// renderModal 渲染 Modal 框
func (m *Model) renderModal() string {
    var b strings.Builder

    // 标题
    title := m.modal.style.Title.Render(m.modal.Title)
    b.WriteString(title)
    b.WriteString("\n")

    // 内容
    switch m.modal.Type {
    case ModalConfirm:
        content := m.modal.style.Content.Render(m.modal.Content)
        b.WriteString(content)

    case ModalDiff:
        // Diff 内容，需要语法高亮
        diff := m.renderDiffContent(m.modal.Content)
        b.WriteString(diff)
    }

    b.WriteString("\n")

    // 提示
    hint := m.modal.style.Content.Render(
        "[Enter: 确认] [Esc: 取消]",
    )
    b.WriteString(hint)

    // 应用边框和 Solid 背景
    return m.modal.style.Border.Render(
        m.modal.style.Background.Render(b.String()),
    )
}

// render*Raw 方法：提供未渲染的原始内容
func (m *Model) renderInputAreaRaw() string {
    if m.inputActive {
        return ">> " + m.textInput.Value()
    }
    return ">> (按 ESC 激活输入)"
}

func (m *Model) renderAnimatedStatusBarRaw() string {
    status := m.animatedStatus
    var statusText strings.Builder

    switch status.state {
    case StatusIdle:
        statusText.WriteString("○ 准备就绪")
    case StatusThinking, StatusReading, StatusSearching, StatusExecuting, StatusStreaming:
        spinnerView := status.spinner.View()
        if status.progress > 0 {
            statusText.WriteString(fmt.Sprintf("%s %s [%d%%]",
                spinnerView, status.message, status.progress))
        } else {
            statusText.WriteString(fmt.Sprintf("%s %s",
                spinnerView, status.message))
        }
    case StatusSuccess:
        statusText.WriteString(fmt.Sprintf("✓ %s", status.message))
    case StatusError:
        statusText.WriteString(fmt.Sprintf("✗ %s", status.message))
    }

    return statusText.String()
}

func (m *Model) renderHelpTextRaw() string {
    var parts []string
    parts = append(parts, "[Ctrl+↑/↓:滚动]")
    parts = append(parts, "[ESC:输入]")

    if m.animatedStatus.showDetails {
        parts = append(parts, "[Ctrl+D:隐藏详情]")
    } else {
        parts = append(parts, "[Ctrl+D/Tab:显示详情]")
    }

    parts = append(parts, "[Enter:发送]")
    parts = append(parts, "[Ctrl+C:退出]")

    return " " + strings.Join(parts, " ") + " "
}
```

---

#### 2.5 事件处理

```go
// internal/adapters/tui/model.go

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // 【关键】Modal 状态下，拦截所有按键
    if m.modal.Visible {
        return m.handleModalInput(msg)
    }

    // 非 Modal 状态，正常处理
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKeyMsg(msg)

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    // ... 其他消息处理
    }
}

// handleModalInput 处理 Modal 状态下的输入
func (m *Model) handleModalInput(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "enter", " ":
            // 确认
            if m.modal.OnConfirm != nil {
                if m.modal.OnConfirm() {
                    // 确认成功，关闭 Modal
                    m.modal.Visible = false
                } else {
                    // 确认失败，保持 Modal
                    // 可以添加错误提示
                }
            } else {
                // 没有 OnConfirm，直接关闭
                m.modal.Visible = false
            }
            return m, nil

        case "esc", "q":
            // 取消，关闭 Modal
            m.modal.Visible = false
            return m, nil

        case "ctrl+c":
            // 强制退出程序
            return m, tea.Quit
        }
    }

    return m, nil
}
```

---

#### 2.6 显示 Modal

```go
// internal/adapters/tui/model.go

case ShowModalMsg:
    // 显示 Modal
    m.modal.state = ModalState{
        Type:      msg.Type,
        Title:     msg.Title,
        Content:   msg.Content,
        OnConfirm: msg.OnConfirm,
        Visible:   true,
    }
    return m, nil
```

---

#### 2.7 Adapter 集成

```go
// internal/adapters/tui/adapter.go

func (a *Adapter) RequestConfirm(action string, args string) bool {
    a.mu.Lock()
    defer a.mu.Unlock()

    if a.program == nil {
        // Fallback: 非 TUI 模式，命令行确认
        fmt.Printf("确认执行 %s(%s)? [y/N]: ", action, args)
        var response string
        fmt.Scanln(&response)
        return strings.ToLower(response) == "y"
    }

    // 创建确认通道
    replyChan := make(chan bool)

    // 发送 ShowModalMsg
    a.program.Send(ShowModalMsg{
        Type:    ModalConfirm,
        Title:   "⚠️  确认工具执行",
        Content: fmt.Sprintf("工具: %s\n参数: %s\n\n是否允许执行？", action, args),
        OnConfirm: func() bool {
            result := <-replyChan
            close(replyChan)
            return result
        },
    })

    // 等待用户响应
    // 注意：这需要在 UI 层有机制来关闭通道
    // 简化版：直接使用现有的 confirmChoice 机制

    // 实际实现需要配合 Update 逻辑
    // 这里简化处理
    result := <-replyChan
    close(replyChan)
    return result
}
```

---

### 视觉效果

```
┌─────────────────────────────────────────────────────┐
│  [消息区域 - 略暗，灰色]                           │
│  用户: 请帮我优化这个函数                          │
│  AI: 好的，我来帮你...                             │
│  [工具调用历史可见]                                  │
│                                                     │
│         ┌─────────────────────────────┐             │
│         │  ⚠️  确认工具执行              │  ← Modal 框
│         │                             │  (高亮，深色背景)
│         │  工具: write_file            │             │
│         │  路径: main.go               │             │
│         │                             │             │
│         │  [Enter: 确认] [Esc: 取消]    │             │
│         └─────────────────────────────┘             │
└─────────────────────────────────────────────────────┘
```

---

## 解决方案 3: Agent 系统

### 设计目标

- 分离只读和编辑权限
- 支持 `/plan` 和 `/build` 命令
- 为未来的 MCP 生态做准备

### 核心概念

借鉴 OpenCode 的 Agent 系统：

| Agent | 权限 | 用途 |
|-------|------|------|
| **Plan** | read, list, search, webfetch | 代码分析、规划 |
| **Build** | 所有工具 | 实际修改、测试 |
| **Subagent** | 自定义 | 专项任务（如 code-review） |

---

### 数据结构

```go
// internal/core/agent_config.go

package core

import (
    "github.com/charmbracelet/lipgloss"
)

// AgentType Agent 类型
type AgentType int

const (
    AgentPrimary AgentType = iota // 主 Agent（当前）
    AgentPlan                      // 只读 Agent
    AgentBuild                     // 编辑 Agent
    AgentSubagent                  // 子 Agent
)

// AgentConfig Agent 配置
type AgentConfig struct {
    Type     AgentType
    Name     string
    Model    string
    Tools    map[string]bool // 工具权限
    Prompt   string          // 自定义提示
    Color    lipgloss.Color  // UI 标识色
}

// DefaultAgentConfigs 默认 Agent 配置
var DefaultAgentConfigs = map[string]AgentConfig{
    "plan": {
        Type:   AgentPlan,
        Name:   "plan",
        Model:  "claude-sonnet-4-20250514",
        Tools: map[string]bool{
            "read_file":   true,
            "list_files":  true,
            "search_files": true,
            "webfetch":    true,
            "write_file":  false,
            "run_command": false,
        },
        Prompt: `
你是 Plan Agent，专注于代码分析和规划。

你的职责：
- 理解代码结构和逻辑
- 制定修改计划
- 识别潜在问题

你不能：
- 修改文件
- 运行命令

只进行分析和规划。
        `,
        Color: lipgloss.Color("#7aa2f7"), // 蓝色
    },
    "build": {
        Type:   AgentBuild,
        Name:   "build",
        Model:  "claude-sonnet-4-20250514",
        Tools: map[string]bool{
            "read_file":    true,
            "list_files":   true,
            "search_files": true,
            "write_file":   true,
            "run_command":  true,
        },
        Prompt: `
你是 Build Agent，专注于实际修改和测试。

你的职责：
- 根据 Plan Agent 的分析进行修改
- 运行测试验证修改
- 报告结果

你可以：
- 修改文件
- 运行命令
- 执行测试

完成修改后，总结变更。
        `,
        Color: lipgloss.Color("#bb9af7"), // 紫色
    },
}
```

---

### Agent 切换逻辑

```go
// internal/core/agent.go

// SwitchAgent 切换 Agent
func (a *Agent) SwitchAgent(agentName string) error {
    config, ok := DefaultAgentConfigs[agentName]
    if !ok {
        return fmt.Errorf("未知的 Agent: %s", agentName)
    }

    // 更新 Agent 配置
    a.Config.Agent = config

    // 发送模式切换通知（如果是 TUI）
    if a.UI != nil {
        a.UI.SendStream(fmt.Sprintf("\n[切换到 %s Agent]\n", agentName))
    }

    return nil
}

// GetCurrentAgent 获取当前 Agent 配置
func (a *Agent) GetCurrentAgent() AgentConfig {
    if a.Config.Agent == nil {
        // 默认使用 build agent
        return DefaultAgentConfigs["build"]
    }
    return *a.Config.Agent
}
```

---

### 用户命令

添加 `/plan` 和 `/build` 命令：

```go
// internal/core/slash_commands.go

// SlashCommandHandler 处理斜杠命令
func (a *Agent) SlashCommandHandler(cmd string, args string) error {
    switch cmd {
    case "/plan":
        return a.SwitchAgent("plan")

    case "/build":
        return a.SwitchAgent("build")

    case "/agent":
        if args == "" {
            // 列出所有 Agent
            current := a.GetCurrentAgent()
            a.UI.SendStream(fmt.Sprintf("\n当前 Agent: %s\n", current.Name))
            a.UI.SendStream("\n可用 Agent:\n")
            for name, config := range DefaultAgentConfigs {
                a.UI.SendStream(fmt.Sprintf("- %s\n", name))
            }
        } else {
            // 切换到指定 Agent
            return a.SwitchAgent(args)
        }

    default:
        return fmt.Errorf("未知命令: %s", cmd)
    }

    return nil
}
```

---

## 解决方案 4: 系统提示词

### 问题

当前 `buildSystemPrompt()` 只包含项目上下文，**没有加载** `api/prompts/system.txt`。

### 解决方案

已经在**解决方案 1**中提供完整实现：

```go
func (a *Agent) buildSystemPrompt(ctx *ProjectContext) string {
    // 1. 加载 system.txt
    basePrompt := loadSystemPrompt()

    // 2. 加载 tools.txt
    toolGuide := loadToolGuide()

    // 3. 工具调用历史
    toolHistorySummary := a.toolHistory.GetSummary()

    // 4. 项目上下文
    projectContext := ...

    return basePrompt + "\n\n" + toolGuide + "\n\n" +
           toolHistorySummary + "\n\n" + projectContext
}
```

---

## 实施计划

### 阶段概览

| 阶段 | 时间 | 目标 | 优先级 |
|------|------|------|--------|
| **Phase 1** | 1-2 天 | 工具调用优化 | 🔴 高 |
| **Phase 2** | 2-3 天 | Modal Overlay | 🔴 高 |
| **Phase 3** | 5-7 天 | Agent 系统 | 🟡 中 |
| **Phase 4** | 10-14 天 | MCP 生态 | 🟢 低 |

---

### Phase 1: 工具调用优化（1-2 天）

#### 任务清单

**1.1 添加工具调用历史**（4 小时）
- [ ] 创建 `internal/core/tool_history.go`
- [ ] 实现 `ToolCallHistory` 结构
- [ ] 实现 `Record()` 方法
- [ ] 实现 `GetSummary()` 方法
- [ ] 单元测试

**1.2 创建工具使用指南**（2 小时）
- [ ] 创建 `api/prompts/tools.txt`
- [ ] 编写工具使用指南
- [ ] 添加示例和反例

**1.3 改进系统提示词注入**（3 小时）
- [ ] 修改 `buildSystemPrompt()`
- [ ] 添加 `loadToolGuide()` 函数
- [ ] 添加 `loadSystemPrompt()` 函数
- [ ] 集成工具历史摘要

**1.4 改进工具描述**（2 小时）
- [ ] 更新 `read_file` 描述
- [ ] 更新 `write_file` 描述
- [ ] 更新 `run_command` 描述
- [ ] 添加并行调用说明

**1.5 实现智能文件缓存**（4 小时）
- [ ] 创建 `internal/core/file_cache.go`
- [ ] 实现 `FileCache` 结构
- [ ] 实现 `CheckRead()` 方法
- [ ] 实现 MD5 hash 计算
- [ ] 集成到 `executeToolsSequential()`

**1.6 集成到 Agent**（2 小时）
- [ ] 在 `Agent` 结构中添加 `toolHistory` 和 `fileCache`
- [ ] 在 `executeToolsSequential()` 中记录工具调用
- [ ] 在 `executeToolsSequential()` 中检查缓存

**1.7 测试验证**（2 小时）
- [ ] 测试文件缓存
- [ ] 测试工具历史
- [ ] 验证 AI 不再重复调用

**验收标准**:
- ✅ AI 不再重复调用同一工具
- ✅ 工具调用历史可见
- ✅ 文件缓存生效
- ✅ 外部修改文件时能读取最新内容

---

### Phase 2: Modal Overlay（2-3 天）

#### 任务清单

**2.1 添加 Modal 数据结构**（2 小时）
- [ ] 创建 `internal/adapters/tui/modal.go`
- [ ] 定义 `ModalType`
- [ ] 定义 `ModalState`
- [ ] 定义 `ModalComponent`
- [ ] 定义 `ModalStyle`

**2.2 实现 Modal 样式**（2 小时）
- [ ] 实现 `DefaultModalStyle()`
- [ ] Tokyo Night 主题配色
- [ ] Solid Background
- [ ] Dim 样式

**2.3 实现 Dimmed View 渲染**（3 小时）
- [ ] 实现 `renderDimmedView()`
- [ ] 实现 `renderInputAreaRaw()`
- [ ] 实现 `renderAnimatedStatusBarRaw()`
- [ ] 实现 `renderHelpTextRaw()`
- [ ] 实现 `renderMessagesContentRaw()`

**2.4 实现 Modal 渲染**（2 小时）
- [ ] 实现 `renderModal()`
- [ ] 处理 Confirm Modal
- [ ] 处理 Diff Modal
- [ ] 应用边框和背景

**2.5 修改 View() 方法**（3 小时）
- [ ] 添加 Modal 状态检查
- [ ] 使用 lipgloss.Place 居中
- [ ] 避免 ANSI 乱码

**2.6 实现事件拦截**（2 小时）
- [ ] 修改 `Update()` 方法
- [ ] 实现 `handleModalInput()`
- [ ] 处理 Enter/Esc/Ctrl+C

**2.7 添加 ShowModalMsg**（1 小时）
- [ ] 定义 `ShowModalMsg`
- [ ] 在 `Update()` 中处理
- [ ] 更新 `ModalState`

**2.8 集成到 Adapter**（3 小时）
- [ ] 修改 `RequestConfirm()`
- [ ] 修改 `RequestConfirmWithDiff()`
- [ ] 发送 `ShowModalMsg`

**2.9 测试验证**（2 小时）
- [ ] 测试 Confirm Modal
- [ ] 测试 Diff Modal
- [ ] 验证底层内容可见
- [ ] 验证键盘事件

**验收标准**:
- ✅ Modal 正确显示在中间
- ✅ 底层内容可见（变暗）
- ✅ 无 ANSI 乱码
- ✅ 键盘事件正确拦截

---

### Phase 3: Agent 系统（5-7 天）

#### 任务清单

**3.1 定义 Agent 类型**（2 小时）
- [ ] 创建 `internal/core/agent_config.go`
- [ ] 定义 `AgentType`
- [ ] 定义 `AgentConfig`
- [ ] 定义 `DefaultAgentConfigs`

**3.2 实现 Agent 切换**（3 小时）
- [ ] 实现 `SwitchAgent()`
- [ ] 实现 `GetCurrentAgent()`
- [ ] 添加模式切换通知

**3.3 添加斜杠命令**（3 小时）
- [ ] 创建 `internal/core/slash_commands.go`
- [ ] 实现 `/plan` 命令
- [ ] 实现 `/build` 命令
- [ ] 实现 `/agent` 命令

**3.4 工具权限检查**（4 小时）
- [ ] 在 `executeToolsSequential()` 中检查权限
- [ ] 禁用 Plan Agent 的编辑工具
- [ ] 提示用户切换 Agent

**3.5 UI 集成**（3 小时）
- [ ] 显示当前 Agent
- [ ] 添加 Agent 切换提示
- [ ] 显示 Agent 权限

**3.6 测试验证**（4 小时）
- [ ] 测试 Plan Agent（只读）
- [ ] 测试 Build Agent（编辑）
- [ ] 测试 Agent 切换

**验收标准**:
- ✅ Plan Agent 不能修改文件
- ✅ Build Agent 可以修改文件
- ✅ `/plan` 和 `/build` 命令正常工作
- ✅ Agent 信息正确显示

---

### Phase 4: MCP 生态（10-14 天）

**目标**: 实现 Model Context Protocol 支持，允许第三方工具集成。

**核心组件**:
- MCP 客户端
- 工具动态加载
- Skill 系统

**详细设计**: 待 Phase 1-3 完成后展开

---

## 测试策略

### 单元测试

```go
// internal/core/tool_history_test.go

func TestToolCallHistory(t *testing.T) {
    h := NewToolCallHistory()

    // 测试记录
    h.Record(ToolCallRecord{
        ID:      "1",
        Tool:    "read_file",
        Success: true,
    })

    // 测试摘要
    summary := h.GetSummary()
    if !strings.Contains(summary, "read_file") {
        t.Error("摘要应包含工具名称")
    }
}
```

---

### 集成测试

**场景 1: 工具缓存**
```
1. 写入文件
2. 读取文件
3. 验证：第二次读取使用缓存
```

**场景 2: Modal Overlay**
```
1. 触发工具调用
2. 观察：底层内容可见（变暗）
3. Modal 框在中间
4. 确认：Modal 消失
```

**场景 3: Agent 切换**
```
1. 使用 /plan 命令
2. 尝试写入文件
3. 验证：被拒绝，提示切换到 build
```

---

### 性能测试

- 工具调用响应时间 < 100ms
- Modal 渲染时间 < 50ms
- 内存增长 < 10MB/小时

---

## 风险与依赖

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| ANSI 字符串处理 | Modal 渲染乱码 | 使用 lipgloss.Place，避免手动切片 |
| 工具缓存失效 | AI 读到过期内容 | 使用 modTime 检测文件变更 |
| Agent 权限复杂 | 用户困惑 | 提供清晰的 Agent 说明和提示 |
| 工具调用历史增长 | Token 消耗 | 限制历史长度（50 条记录） |

---

## 参考文档

### 设计灵感

- [OpenCode TUI Documentation](https://opencode.ai/docs/tui/)
- [How Coding Agents Actually Work: Inside OpenCode](https://cefboud.com/posts/coding-agents-internals-opencode-deepdive/)
- [OpenCode GitHub Repository](https://github.com/opencode-ai/opencode)

### 技术文档

- [Bubble Tea Framework](https://github.com/charmbracelet/bubbletea)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)
- [Muesli Reflow](https://github.com/muesli/reflow) - ANSI 安全的字符串处理
- [Mattn Go-Runewidth](https://github.com/mattn/go-runewidth) - 字符宽度计算

### 相关标准

- [Model Context Protocol (MCP)](https://modelcontextprotocol.io/)
- [Tokyo Night Color Palette](https://github.com/folke/tokyonight.nvim)

---

## 附录

### 代码统计

| Phase | 文件数 | 新增代码 | 修改代码 |
|-------|--------|---------|---------|
| Phase 1 | 5 | ~300 行 | ~100 行 |
| Phase 2 | 3 | ~400 行 | ~150 行 |
| Phase 3 | 8 | ~600 行 | ~200 行 |
| Phase 4 | 12 | ~1200 行 | ~300 行 |
| **总计** | **28** | **~2500 行** | **~750 行** |

### 实施时间线

```
Week 1: Phase 1 (工具调用优化)
Week 2: Phase 2 (Modal Overlay)
Week 3-4: Phase 3 (Agent 系统)
Week 5-7: Phase 4 (MCP 生态)
```

---

**End of Design Document**

**Next Steps**:
1. Review and approve this design
2. Begin Phase 1 implementation
3. Create feature branch: `feature/kore-optimization`
4. Incremental commits after each phase
