# ADR 002: Agent-Centric Architecture with Interface Decoupling

## Status
**Accepted**

## Context
Kore needs to support multiple UI modes (CLI, TUI, GUI) with the same core business logic. The architecture must:
- Share Agent logic across CLI, TUI, and future GUI (Wails)
- Allow UI layers to be swapped without touching core logic
- Support concurrent updates (LLM streaming + UI rendering)
- Enable independent testing of UI and business logic
- Facilitate future expansion (web interface, etc.)

## Decision
Adopt **Agent-centric architecture** with **interface-based decoupling**.

### Architecture Pattern
```go
// UI abstraction layer - Agent doesn't care about specific UI
type UIInterface interface {
    SendStream(content string)
    RequestConfirm(action string, args string) bool
    RequestConfirmWithDiff(path string, diffText string) bool
    ShowStatus(status string)
}

// Agent holds interface reference, not concrete implementation
type Agent struct {
    UI          UIInterface
    ContextMgr  *ContextManager
    LLMProvider *LLMProvider
    Tools       *ToolExecutor
    History     *ConversationHistory
    Config      *Config
}
```

### Alternatives Considered

#### 1. **Direct Dependency**
```go
// Agent directly depends on TUI
type Agent struct {
    TUI *TUIModel  // ❌ Tight coupling
}
```
- **Pros:** Simple, direct access
- **Cons:** Cannot swap UI, testing requires TUI, violates SRP
- **Verdict:** Rejected - Too rigid

#### 2. **Event Bus (Channel-based)**
```go
// All communication through channels
type Agent struct {
    EventOut chan<- Event
    EventIn  <-chan Event
}
```
- **Pros:** Fully decoupled, async
- **Cons:** Complex, hard to reason about, over-engineering for MVP
- **Verdict:** Rejected - Too complex for current needs

#### 3. **Interface-based Decoupling** ✅
```go
// Agent depends on abstraction
type Agent struct {
    UI UIInterface  // ✅ Loose coupling
}
```
- **Pros:**
  - Clean separation of concerns
  - Easy to test (mock UI)
  - Supports multiple implementations
  - Simple to understand
- **Cons:**
  - Slightly more boilerplate
- **Verdict:** **ACCEPTED** - Best balance

## Consequences

### Positive
- **Flexibility:** UI can be swapped without touching Agent code
- **Testability:** Agent logic can be tested with mock UI
- **Extensibility:** New UI modes (Wails, web) just implement UIInterface
- **Clean Architecture:** Business logic isolated from presentation

### Negative
- **Boilerplate:** Each adapter must implement full UIInterface
- **Interface Bloat:** UIInterface may grow large (consider breaking into smaller interfaces)

### Neutral
- **Performance:** Minimal overhead from interface calls (<1%)
- **Complexity:** Slightly more complex than direct dependency

## Implementation Examples

### CLIAdapter
```go
type CLIAdapter struct {
    stdout io.Writer
}

func (a *CLIAdapter) SendStream(content string) {
    fmt.Fprint(a.stdout, content)
}

func (a *CLIAdapter) RequestConfirm(action string, args string) bool {
    fmt.Printf("Execute %s? [y/N] ", action)
    var response string
    fmt.Scanln(&response)
    return strings.ToLower(response) == "y"
}
```

### TUIAdapter (Bubble Tea)
```go
type TUIAdapter struct {
    program *tea.Program
}

func (a *TUIAdapter) SendStream(content string) {
    // Send message to Bubble Tea main loop
    a.program.Send(StreamMsg{Content: content})
}

func (a *TUIAdapter) RequestConfirm(action string, args string) bool {
    // Bubble Tea handles confirmation in UI
    // Agent blocks until user responds
    return waitForConfirmation()
}
```

### WailsAdapter (Future)
```go
type WailsAdapter struct {
    runtime *wails.Runtime
}

func (a *WailsAdapter) SendStream(content string) {
    a.runtime.EventsEmit("stream", content)
}

func (a *WUIAdapter) RequestConfirm(action string, args string) bool {
    // JavaScript frontend shows modal, returns result
    return a.runtime.DialogConfirm(action, args)
}
```

## Testing Strategy

### Unit Test Agent with Mock UI
```go
type MockUI struct {
    confirmResponse bool
    streamedContent []string
}

func (m *MockUI) SendStream(content string) {
    m.streamedContent = append(m.streamedContent, content)
}

func (m *MockUI) RequestConfirm(action string, args string) bool {
    return m.confirmResponse
}

func TestAgentRun(t *testing.T) {
    mockUI := &MockUI{confirmResponse: true}
    agent := &Agent{UI: mockUI, ...}

    err := agent.Run(ctx, "test message")
    assert.NoError(t, err)
    assert.Contains(t, mockUI.streamedContent, "test")
}
```

## Related
- Design Doc: Section 1 (Architecture) - Agent-centric design
- Design Doc: Section 3 (LLM Interaction) - StreamEvent protocol
- ADR 001: Choose Bubble Tea as TUI framework

## References
- Go Interfaces: https://go.dev/tour/methods/14
- Clean Architecture: https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- Dependency Inversion Principle: https://en.wikipedia.org/wiki/Dependency_inversion_principle
