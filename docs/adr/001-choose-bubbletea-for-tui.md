# ADR 001: Choose Bubble Tea as TUI Framework

## Status
**Accepted**

## Context
Kore requires a cross-platform, high-performance TUI (Terminal User Interface) library for its interactive terminal interface. The TUI must support:
- Complex UI state management
- Streaming text display with typewriter effect
- Interactive diff confirmation views
- Concurrent updates (LLM streaming + UI rendering)
- Cross-platform support (Linux, macOS, Windows)
- Testability

## Decision
Use [Bubble Tea](https://github.com/charmbracelet/bubbletea) as the TUI framework.

### Alternatives Considered

#### 1. **tview**
- **Pros:**
  - Rich widget set (tables, trees, forms)
  - Mature and widely used
- **Cons:**
  - Complex API, harder to test
  - Less elegant state management
  - Harder to reason about concurrent updates
- **Verdict:** Rejected due to testing complexity and state management challenges

#### 2. **bubbletea**
- **Pros:**
  - Elm architecture (Model-View-Update) - clear separation of concerns
  - Easy to test (pure functions, deterministic state transitions)
  - Excellent ecosystem (lipgloss for styling, glamour for markdown)
  - Built-in concurrency handling via channels
  - Actively maintained by Charmbracelet
- **Cons:**
  - Requires learning functional programming paradigm
  - More boilerplate for simple UIs
- **Verdict:** **ACCEPTED** - Best fit for complex stateful UI with streaming

#### 3. **termui**
- **Pros:**
  - Simple API
  - Good for dashboards
- **Cons:**
  - Less actively maintained
  - Limited widget set
  - No built-in state management
- **Verdict:** Rejected due to maintenance concerns

## Consequences

### Positive
- **Testability:** Pure functions make UI testing straightforward (Golden File Testing)
- **Maintainability:** Elm architecture prevents spaghetti code
- **Ecosystem:** Access to Charmbracelet ecosystem (lipgloss, glamour, bubbles)
- **Concurrency:** Built-in support for concurrent updates via channels
- **Cross-platform:** Works seamlessly on Linux, macOS, and Windows

### Negative
- **Learning Curve:** Team must learn Elm architecture (Model-View-Update pattern)
- **Boilerplate:** More code required for simple interactions
- **Paradigm Shift:** Requires thinking in functional programming terms

### Neutral
- **Performance:** Bubble Tea is efficient, but may require optimization for very high-frequency updates (>60fps)

## Implementation Notes

### Architecture Pattern
```go
// Model - UI state
type model struct {
    agent      *core.Agent
    messages   []string
    spinner    spinner.Spinner
    quitting   bool
}

// Update - State transition function
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c":
            return m, tea.Quit
        }
    case StreamEvent:
        // Handle streaming events from Agent
        m.messages = append(m.messages, msg.Content)
        return m, nil
    }
    return m, nil
}

// View - Render function
func (m model) View() string {
    return fmt.Sprintf("Messages:\n%s", strings.Join(m.messages, "\n"))
}
```

### Integration with Agent
- TUIAdapter implements `UIInterface` from Agent
- Uses `program.Send()` to send messages from Agent goroutine to Bubble Tea main loop
- Streaming events wrapped as `tea.Msg` for Update loop

## Related
- Design Doc: Section 3 (LLM Interaction) - StreamEvent protocol
- Design Doc: Section 6 (Implementation Phases) - Week 5 (TUI Experience)
- ADR 002: Agent-centric architecture with interface decoupling

## References
- Bubble Tea: https://github.com/charmbracelet/bubbletea
- Charmbracelet Glossary: https://github.com/charmbracelet/lipgloss
- Charmbracelet Glamour: https://github.com/charmbracelet/glamour
