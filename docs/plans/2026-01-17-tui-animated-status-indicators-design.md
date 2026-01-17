# TUI Animated Status Indicators Design

**Date**: 2026-01-17
**Author**: Claude + User Collaboration
**Status**: Design Approved
**Target**: Kore v0.7.0

---

## Executive Summary

Enhance Kore's TUI with OpenCode-inspired animated status indicators, providing context-aware visual feedback during AI operations. This design introduces a 7-state animation system with color-coded spinners, progress tracking, and tool execution details toggle.

**Key Benefits**:
- Context-aware status messages (reading, searching, executing)
- Color-coded visual states (Tokyo Night theme)
- Automatic state reset with 2-second timer
- Tool execution details toggle (Ctrl+d / Tab)
- Responsive layout with viewport integration

**Estimated Effort**: 6-10 hours across 4 incremental phases

---

## Table of Contents

1. [Requirements](#requirements)
2. [Architecture](#architecture)
3. [State Machine Design](#state-machine-design)
4. [Message Flow](#message-flow)
5. [View Rendering](#view-rendering)
6. [Implementation Plan](#implementation-plan)
7. [Testing Strategy](#testing-strategy)
8. [References](#references)

---

## Requirements

### Functional Requirements

**FR-1**: TUI must display 7 distinct operational states
- Idle (准备就绪)
- Thinking (思考中)
- Reading (读取文件)
- Searching (搜索代码)
- Executing (执行工具)
- Streaming (生成回复)
- Success (完成)
- Error (错误)

**FR-2**: Each state must use a unique spinner type and color
- Thinking: Blue dot spinner
- Reading: Cyan points spinner
- Searching: Magenta line spinner
- Executing: Orange jump spinner
- Streaming: Green mini-dot spinner

**FR-3**: Success and error states must auto-reset to idle after 2 seconds

**FR-4**: Users can toggle tool execution details via Ctrl+d or Tab

**FR-5**: Status bar must dynamically adjust height for details view

### Non-Functional Requirements

**NFR-1**: Performance
- No goroutine leaks
- Smooth 60fps animations
- <50ms response to state changes

**NFR-2**: Maintainability
- Incremental implementation phases
- Backward compatible with existing `thinking bool`
- Clear separation of concerns

**NFR-3**: User Experience
- Responsive to window resizing
- Intuitive keyboard shortcuts
- Clear visual feedback

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────┐
│                   TUI Adapter                        │
│  (Sends StatusChangeMsg to Bubble Tea program)      │
└────────────────────┬────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────┐
│                    Model                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐  │
│  │ AnimatedStatus│  │  Viewport   │  │Styles    │  │
│  └──────────────┘  └──────────────┘  └──────────┘  │
└────────────────────┬────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────┐
│              Terminal Output                         │
│  • Message area (with viewport scrolling)            │
│  • Input area                                        │
│  • Animated status bar (color-coded)                 │
│  • Help text                                         │
└─────────────────────────────────────────────────────┘
```

### Data Structures

```go
// StatusState represents current operational state
type StatusState int

const (
    StatusIdle StatusState = iota
    StatusThinking
    StatusReading
    StatusSearching
    StatusExecuting
    StatusStreaming
    StatusSuccess
    StatusError
)

// AnimatedStatus manages animation state
type AnimatedStatus struct {
    state       StatusState
    spinner     spinner.Model
    message     string
    progress    int           // 0-100
    timestamp   time.Time     // For auto-reset
    showDetails bool          // Details toggle
    payload     map[string]string // Context metadata
}

// Model extends existing structure
type Model struct {
    // ... existing fields ...

    // New: Replaces thinking bool
    animatedStatus AnimatedStatus

    // New: Handles scrolling and word-wrap
    viewport viewport.Model
}
```

---

## State Machine Design

### State Transition Diagram

```
User Input
    ↓
StatusThinking (Blue + Dot spinner)
    ↓
┌─────────────────────────────────────┐
│ LLM Response                        │
│   ├─ Reasoning → Thinking           │
│   ├─ Tool Call → Executing          │
│   └─ Streaming → Streaming          │
└─────────────────────────────────────┘
    ↓
Tool Execution
    ├─ read_file → Reading (Cyan + Points)
    ├─ search_files → Searching (Magenta + Line)
    └─ run_command → Executing (Orange + Jump)
    ↓
Success (Green ✓, 2s → Idle)
    OR
Error (Red ✗, 2s → Idle)
```

### Color Mapping (Tokyo Night Theme)

| State | Color | Hex | Usage |
|-------|-------|-----|-------|
| Idle | Muted Gray | `#565f89` | Waiting for input |
| Thinking | Primary Blue | `#7aa2f7` | AI reasoning |
| Reading | Cyan | `#2ac3de` | File operations |
| Searching | Magenta | `#bb9af7` | Code search |
| Executing | Orange | `#e0af68` | Tool execution |
| Streaming | Green Cyan | `#73daca` | Response streaming |
| Success | Green | `#9ece6a` | Operation success |
| Error | Red | `#f7768e` | Operation failure |

### Spinner Type Mapping

```go
func spinnerForState(state StatusState) spinner.Spinner {
    switch state {
    case StatusThinking:
        return spinner.Dot        // ●●●
    case StatusReading:
        return spinner.Points     // ...
    case StatusSearching:
        return spinner.Line       // │ ─ │
    case StatusExecuting:
        return spinner.Jump       // ▁ ▃ ▅ ▆ ▅ ▃ ▁
    case StatusStreaming:
        return spinner.MiniDot    // Lightweight
    default:
        return spinner.Dot
    }
}
```

---

## Message Flow

### Message Types

```go
// StatusChangeMsg signals state transition
type StatusChangeMsg struct {
    State    StatusState
    Message  string
    Progress int              // 0-100, optional
    Payload  map[string]string // Context metadata
}

// StatusProgressMsg updates progress for current state
type StatusProgressMsg struct {
    Progress int
    Detail   string
}

// ResetStatusMsg triggers auto-reset (ephemeral states)
type ResetStatusMsg struct{}

// ToggleDetailsMsg switches detail view visibility
type ToggleDetailsMsg struct{}
```

### State Change Logic

```go
case StatusChangeMsg:
    // Update state
    m.animatedStatus.state = msg.State
    m.animatedStatus.message = msg.Message
    m.animatedStatus.progress = msg.Progress
    m.animatedStatus.payload = msg.Payload
    m.animatedStatus.timestamp = time.Now()

    // Recreate spinner for new state
    newSpinner := spinner.New()
    newSpinner.Spinner = spinnerForState(msg.State)
    newSpinner.Style = lipgloss.NewStyle().
        Foreground(colorForState(msg.State))
    m.animatedStatus.spinner = newSpinner

    // Start timer for ephemeral states
    if msg.State == StatusSuccess || msg.State == StatusError {
        return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
            return ResetStatusMsg{}
        })
    }

    // Continue animation
    return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
        return SpinnerTickMsg(t)
    })

case ResetStatusMsg:
    // Auto-reset to idle
    if m.animatedStatus.state == StatusSuccess ||
       m.animatedStatus.state == StatusError {
        m.animatedStatus.state = StatusIdle
        m.animatedStatus.message = "准备就绪"
        m.animatedStatus.progress = 0
    }
    return m, nil
```

### Error Handling

**Bug Fix**: Properly propagate error messages to UI

```go
// In ToolExecutor.Execute
result, err := te.executeWithProgress(ctx, call)

// Extract error message
var errMsg string
if err != nil {
    errMsg = err.Error() // Critical: Get actual error text
}

// Notify UI
if te.ui != nil {
    te.ui.EndToolExecution(err == nil, errMsg)
}
```

### UI Integration

```go
// Adapter methods for Agent integration

func (a *Adapter) StartToolExecution(toolName string, payload map[string]string) {
    state := StatusExecuting
    message := fmt.Sprintf("执行: %s", toolName)

    switch toolName {
    case "read_file", "list_files":
        state = StatusReading
        message = "读取文件..."
    case "search_files":
        state = StatusSearching
        message = "搜索代码..."
    }

    a.program.Send(StatusChangeMsg{
        State:   state,
        Message: message,
        Payload: payload,
    })
}

func (a *Adapter) EndToolExecution(success bool, errMsg string) {
    state := StatusSuccess
    message := "完成 ✓"

    if !success {
        state = StatusError
        // Truncate long error messages
        if len(errMsg) > 50 {
            errMsg = errMsg[:47] + "..."
        }
        message = fmt.Sprintf("错误: %s", errMsg)
    }

    a.program.Send(StatusChangeMsg{
        State:   state,
        Message: message,
    })
}
```

---

## View Rendering

### Responsive Layout

**Key Principle**: Calculate bottom height first, then allocate remaining space to viewport.

```go
func (m *Model) View() string {
    // 1. Render bottom components (variable height)
    inputView := m.renderInputArea()
    statusBarView := m.renderStatusBar()
    helpView := m.styles.App.Render(m.renderHelpText())

    // 2. Calculate total bottom height
    bottomHeight := lipgloss.Height(inputView) +
                    lipgloss.Height(statusBarView) +
                    lipgloss.Height(helpView)

    // 3. Dynamically adjust viewport
    availableHeight := m.height - bottomHeight
    if availableHeight < 5 { // Minimum height protection
        availableHeight = 5
    }
    m.viewport.Height = availableHeight
    m.viewport.Width = m.width

    // 4. Update viewport content
    m.viewport.SetContent(m.renderMessagesContent())

    // 5. Assemble with JoinVertical (robust layout)
    return lipgloss.JoinVertical(lipgloss.Left,
        m.viewport.View(),
        inputView,
        statusBarView,
        helpView,
    )
}
```

### Status Bar Rendering

```go
func (m *Model) renderStatusBar() string {
    status := m.animatedStatus
    var statusText strings.Builder

    switch status.state {
    case StatusIdle:
        statusText.WriteString("○ 准备就绪")

    case StatusThinking, StatusReading, StatusSearching,
         StatusExecuting, StatusStreaming:
        spinnerView := status.spinner.View()

        if status.progress > 0 {
            statusText.WriteString(fmt.Sprintf("%s %s [%d%%]",
                spinnerView, status.message, status.progress))
        } else {
            statusText.WriteString(fmt.Sprintf("%s %s",
                spinnerView, status.message))
        }

        // Detail view (may add a line)
        if status.showDetails {
            details := m.getOperationDetails()
            if details != "" {
                statusText.WriteString("\n  └─ " + details)
            }
        }

    case StatusSuccess:
        statusText.WriteString(fmt.Sprintf("✓ %s", status.message))

    case StatusError:
        statusText.WriteString(fmt.Sprintf("✗ %s", status.message))
    }

    // Apply color
    coloredText := m.styles.App.
        Foreground(colorForState(status.state)).
        Render(statusText.String())

    return m.styles.StatusBar.
        Width(m.width).
        Render(coloredText)
}
```

### Detail View with Payload

```go
func (m *Model) getOperationDetails() string {
    p := m.animatedStatus.payload
    if p == nil {
        return ""
    }

    switch m.animatedStatus.state {
    case StatusReading:
        if file, ok := p["file"]; ok {
            return fmt.Sprintf("文件: %s", file)
        }
    case StatusSearching:
        if pattern, ok := p["pattern"]; ok {
            return fmt.Sprintf("搜索: %s", pattern)
        }
    case StatusExecuting:
        if tool, ok := p["tool"]; ok {
            return fmt.Sprintf("工具: %s", tool)
        }
    case StatusStreaming:
        if tokens, ok := p["tokens"]; ok {
            return fmt.Sprintf("已生成: %s tokens", tokens)
        }
    }

    return ""
}
```

### Keyboard Shortcuts

```go
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "ctrl+d", "tab": // Both work for details toggle
        return m, func() tea.Msg {
            return ToggleDetailsMsg{}
        }

    // ... other shortcuts ...
    }

    return m, nil
}
```

**Help Text**:
```
[Ctrl+↑/↓:滚动] [ESC:输入] [Ctrl+D/Tab:显示详情] [Enter:发送] [Ctrl+C:退出]
```

---

## Implementation Plan

### Phase 1: Core State Machine (1-2 hours)

**Goal**: Add state enums and data structures without breaking existing functionality

**Tasks**:
1. Add `StatusState` enum (8 states)
2. Add `AnimatedStatus` struct
3. Update `Model` initialization
4. Add helper functions (`spinnerForState`, `colorForState`)
5. Compilation test

**Files Modified**:
- `internal/adapters/tui/model.go` (+80 lines)

**Acceptance Criteria**:
- ✅ Code compiles
- ✅ No impact on existing functionality
- ✅ Can run `kore chat --ui tui`

---

### Phase 2: Event Flow Integration (2-3 hours)

**Goal**: Add message types and Update logic

**Tasks**:
1. Define new message types (StatusChangeMsg, StatusProgressMsg, ResetStatusMsg, ToggleDetailsMsg)
2. Implement `handleStatusChange()` with timer logic
3. Implement `handleStatusReset()` for auto-reset
4. Add keyboard shortcuts (Ctrl+d, Tab)
5. Extend Adapter interface with new methods
6. Integrate into ToolExecutor

**Files Modified**:
- `internal/adapters/tui/model.go` (+80 lines)
- `internal/adapters/tui/adapter.go` (+60 lines)
- `internal/core/agent.go` (+20 lines)

**Acceptance Criteria**:
- ✅ Status changes trigger animations
- ✅ Ctrl+d / Tab toggles details
- ✅ Success/Error auto-reset after 2 seconds

---

### Phase 3: View Rendering Upgrade (2-3 hours)

**Goal**: Integrate Viewport and dynamic height calculation

**Tasks**:
1. Add viewport dependency: `go get github.com/charmbracelet/bubbles/viewport`
2. Update `Model` struct with `viewport.Model`
3. Initialize viewport in `NewModel()`
4. Rewrite `View()` with dynamic height calculation
5. Implement helper rendering functions:
   - `renderMessagesContent()`
   - `renderInputArea()`
   - `renderStatusBar()`
   - `renderHelpText()`
   - `getOperationDetails()`

**Files Modified**:
- `go.mod` (+1 dependency)
- `internal/adapters/tui/model.go` (+120 lines)

**Acceptance Criteria**:
- ✅ Message area scrolls (Ctrl+↑/↓)
- ✅ Window resize adapts automatically
- ✅ Status bar height adjusts for details view

---

### Phase 4: Testing & Optimization (1-2 hours)

**Goal**: Integration testing and polish

**Tasks**:
1. Update Agent integration with tool execution callbacks
2. Manual testing checklist (11 test cases)
3. Performance optimization (goroutine cleanup)
4. Update documentation (`docs/USER_GUIDE.md`)

**Files Modified**:
- `internal/core/agent.go` (+30 lines)
- `docs/USER_GUIDE.md` (+20 lines)

**Manual Testing Checklist**:
- [ ] Launch TUI: `kore chat --ui tui`
- [ ] Test thinking state (after sending message)
- [ ] Test reading file state
- [ ] Test searching file state
- [ ] Test success state (tool execution success)
- [ ] Test error state (tool execution failure)
- [ ] Test 2-second auto-reset
- [ ] Test Ctrl+d / Tab details toggle
- [ ] Test scrolling (Ctrl+↑/↓)
- [ ] Test window resize
- [ ] Test long-running operations (memory leaks)

**Acceptance Criteria**:
- ✅ All manual tests pass
- ✅ No memory leaks
- ✅ Documentation updated

---

### Summary of Changes

| File | Type | Lines |
|------|------|-------|
| `internal/adapters/tui/model.go` | Add + Modify | +280 |
| `internal/adapters/tui/adapter.go` | Add methods | +80 |
| `internal/core/agent.go` | Integration | +50 |
| `go.mod` | Add dependency | +1 |
| `docs/USER_GUIDE.md` | Documentation | +20 |
| **Total** | | **~430 lines** |

---

## Testing Strategy

### Unit Testing

```go
// internal/adapters/tui/model_test.go

func TestSpinnerForState(t *testing.T) {
    tests := []struct {
        state    StatusState
        expected spinner.Spinner
    }{
        {StatusThinking, spinner.Dot},
        {StatusReading, spinner.Points},
        {StatusSearching, spinner.Line},
        {StatusExecuting, spinner.Jump},
        {StatusStreaming, spinner.MiniDot},
    }

    for _, tt := range tests {
        t.Run(tt.state.String(), func(t *testing.T) {
            result := spinnerForState(tt.state)
            if result != tt.expected {
                t.Errorf("spinnerForState(%v) = %v, want %v",
                    tt.state, result, tt.expected)
            }
        })
    }
}

func TestStatusChangeWithAutoReset(t *testing.T) {
    model := NewModel()
    model.animatedStatus.state = StatusIdle

    // Simulate status change
    msg := StatusChangeMsg{
        State:   StatusSuccess,
        Message: "完成 ✓",
    }

    newModel, cmd := model.Update(msg)

    // Check state changed
    if newModel.(*Model).animatedStatus.state != StatusSuccess {
        t.Error("State did not change to Success")
    }

    // Check timer command returned
    if cmd == nil {
        t.Error("Expected timer command for ephemeral state")
    }
}
```

### Integration Testing

```go
// internal/adapters/tui/adapter_test.go

func TestToolExecutionStatusFlow(t *testing.T) {
    adapter := NewAdapter()
    adapter.Start()
    defer adapter.Stop()

    // Start execution
    adapter.StartToolExecution("read_file", map[string]string{
        "file": "main.go",
    })

    // Update progress
    adapter.UpdateToolProgress(50, "")

    // End execution
    adapter.EndToolExecution(true, "")

    // Allow time for message processing
    time.Sleep(100 * time.Millisecond)

    // Verify state transitions
    // (implementation-specific)
}
```

### Manual Testing

See Phase 4 checklist above.

### Performance Testing

```bash
# Run with race detector
go run -race ./cmd/kore chat --ui tui

# Profile memory
go tool pprof http://localhost:6060/debug/pprof/heap

# Check for goroutine leaks
go test -run TestGoroutineLeaks ./...
```

---

## Rollback Plan

### Backward Compatibility

Preserve existing `thinking bool` field as fallback:

```go
type Model struct {
    // New animated status
    animatedStatus AnimatedStatus

    // Legacy field (deprecated)
    thinking bool // TODO: Remove in v0.8.0
}
```

### Feature Flag

Optional environment variable to disable new system:

```go
const enableAnimatedStatus = os.Getenv("KORE_ANIMATED_STATUS") != "false"

func (m *Model) renderStatusBar() string {
    if !enableAnimatedStatus {
        return m.renderLegacyStatusBar()
    }
    return m.renderAnimatedStatusBar()
}
```

### Rollback Steps

1. Set `KORE_ANIMATED_STATUS=false`
2. Restart Kore
3. Revert to Phase 1 commit if needed

---

## Future Enhancements

### Short-term (v0.8.0)

- [ ] Add progress bar for multi-file operations
- [ ] Support multiple concurrent operations (operation queue)
- [ ] Add `/thinking` command (OpenCode-style)
- [ ] Scroll acceleration

### Long-term (v1.0.0)

- [ ] GPU-accelerated animations (OpenGL)
- [ ] Status history panel
- [ ] Custom theme support
- [ ] Animation speed controls

---

## References

### Design Inspiration

- [OpenCode TUI Documentation](https://opencode.ai/docs/tui/)
- [OpenCode GitHub Repository](https://github.com/opencode-ai/opencode)
- [Claude Code](https://claude.ai/code) - Original inspiration

### Technical Documentation

- [Bubble Tea Framework](https://github.com/charmbracelet/bubbletea)
- [Bubbletea Viewport](https://github.com/charmbracelet/bubbles/tree/master/viewport)
- [Lipgloss Styling](https://github.com/charmbracelet/lipgloss)
- [Bubbletea Spinners Example](https://github.com/charmbracelet/bubbletea/blob/main/examples/spinners/main.go)

### Related Standards

- [Tokyo Night Color Palette](https://github.com/folke/tokyonight.nvim)
- [Unicode Box Drawing Characters](https://en.wikipedia.org/wiki/Box-drawing_character)
- [ANSI Escape Codes](https://en.wikipedia.org/wiki/ANSI_escape_code)

---

## Appendix

### Example: Complete Operation Flow

```go
// User: "Read main.go and explain"

// 1. User submits input
[Status: Idle → Thinking]
[Message: "思考中..."]
[Spinner: Blue dot ●●●]

// 2. Agent decides to read file
[Status: Thinking → Reading]
[Message: "读取文件..."]
[Spinner: Cyan points ...]
[Payload: {"file": "main.go"}]

// 3. Tool execution completes
[Status: Reading → Success]
[Message: "完成 ✓"]
[Color: Green #9ece6a]
[Timer: 2 seconds]

// 4. Auto-reset to idle
[Status: Success → Idle]
[Message: "准备就绪"]
[Color: Gray #565f89]

// 5. Agent streams response
[Status: Idle → Streaming]
[Message: "生成回复..."]
[Spinner: Green cyan mini-dot]
[Payload: {"tokens": "150"}]

// 6. Response complete
[Status: Streaming → Success]
[Message: "完成 ✓"]
[Timer: 2 seconds → Idle]
```

### ASCII Layout Preview

```
┌─────────────────────────────────────────────────────────┐
│                                                          │
│ [Messages - Scrollable with Ctrl+↑/↓]                    │
│ • AI responses (Markdown rendered)                      │
│ • Tool results                                           │
│                                                          │
└─────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────┐
│ >> 读取 main.go 并解释主要功能                            │
└─────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────┐
│ ●●● 读取文件...                                          │
│ └─ 文件: main.go                                        │
└─────────────────────────────────────────────────────────┘
 [Ctrl+↑/↓:滚动] [ESC:输入] [Ctrl+D/Tab:显示详情] [Enter:发送]
```

---

**End of Design Document**

**Next Steps**:
1. Review and approve this design
2. Begin Phase 1 implementation
3. Create feature branch: `feature/tui-animated-status`
4. Incremental commits after each phase
