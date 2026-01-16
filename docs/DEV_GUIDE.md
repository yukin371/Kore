# Kore Development Guide (v1.0)

## Overview

This document establishes the engineering standards for developing Kore, an AI-powered workflow automation platform. The core philosophy is **"Defensive Programming + Automated Verification"**.

## Table of Contents

1. [Repository Structure](#1-repository-structure)
2. [Code Style & Quality](#2-code-style--quality)
3. [Git Workflow](#3-git-workflow)
4. [Testing Strategy](#4-testing-strategy)
5. [Documentation Engineering](#5-documentation-engineering)
6. [CI/CD Automation](#6-ci-cd-automation)
7. [Getting Started](#7-getting-started)

---

## 1. Repository Structure

Adopt **Standard Go Project Layout** with strict separation between private and public code.

```
kore/
├── cmd/                    # 【Entry Layer】
│   └── kore/               # Main entry point, only main.go, no business logic
│       └── main.go
├── internal/               # 【Private Business Layer】(Cannot be imported externally)
│   ├── core/               # Core domain models (Agent, Context, LLM Interface)
│   │   ├── agent.go        # Agent core logic & ReAct loop
│   │   ├── context.go      # ContextManager
│   │   ├── history.go      # ConversationHistory
│   │   └── llm.go          # LLMProvider interface
│   ├── adapters/           # Port adapters (CLI View, TUI View, Providers)
│   │   ├── cli/            # CLIAdapter
│   │   ├── tui/            # TUIAdapter (Bubble Tea)
│   │   ├── openai/         # OpenAI client implementation
│   │   └── ollama/         # Ollama client implementation
│   ├── tools/              # Tool execution framework
│   │   ├── executor.go     # ToolExecutor
│   │   ├── read_file.go
│   │   ├── write_file.go
│   │   ├── run_command.go
│   │   └── security.go     # SecurityInterceptor
│   └── infrastructure/     # Infrastructure services
│       ├── fs/             # File system operations
│       └── config/         # Configuration management
├── pkg/                    # 【Public Library Layer】(Can be imported by external projects)
│   ├── logger/             # Structured logging library
│   └── utils/              # Common utility functions
├── api/                    # 【Protocol Layer】
│   └── prompts/            # System Prompts version management (NEVER hardcode in Go)
│       ├── system.txt      # Base system prompt
│       └── tools.txt       # Tool descriptions
├── configs/                # 【Configuration Layer】Default config templates
│   └── default.yaml
├── docs/                   # 【Documentation Layer】
│   ├── adr/                # Architecture Decision Records
│   └── plans/              # Design documents
├── tests/                  # 【Testing Layer】Integration & E2E tests
│   ├── integration/        # Integration tests
│   ├── testdata/           # Test fixtures
│   └── evals/              # AI evaluation test cases
│       └── test_cases.json
├── scripts/                # 【Build Scripts】Helper scripts
│   ├── install.sh
│   └── build.sh
├── .golangci.yml           # Linter configuration
├── .github/                # GitHub Actions workflows
│   └── workflows/
│       └── ci.yml
├── Makefile                # Unified build entry point
├── go.mod
├── go.sum
├── CONTRIBUTING.md         # This file
└── README.md
```

### ✅ Mandatory Rules

1. **No circular dependencies** in `internal/` packages
2. **All Prompts MUST** be in `api/prompts/` or `internal/core/prompts/`, NEVER scattered in logic code
3. **`cmd/` contains ONLY main.go** - no business logic allowed
4. **`internal/` is private** - external projects cannot import it
5. **`pkg/` is public** - must be stable, well-documented, backward-compatible

---

## 2. Code Style & Quality

Use automated tools to enforce standards, not human review.

### 2.1 Linting (Static Analysis)

**Tool:** `golangci-lint` (industry standard)

**Installation:**
```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
```

**Enabled Linters:**
- `govet` - Go vet static analysis
- `errcheck` - Check unchecked errors
- `staticcheck` - Go static analysis
- `gosec` - Security inspection
- `gocyclo` - Cyclomatic complexity check
- `godot` - Check comments have periods
- `misspell` - Spell checking

**Configuration:** `.golangci.yml`
```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - gosec
    - gocyclo
    - godot
    - misspell

linters-settings:
  gocyclo:
    min-complexity: 15
  govet:
    enable-all: true
```

**Rules:**
- ❌ **NEVER ignore errors** using `_` unless absolutely certain
- ✅ **ALWAYS wrap errors with context:**
  ```go
  return fmt.Errorf("initializing agent: %w", err)
  ```
- ❌ **NO `log.Println` in production code** - use `pkg/logger`
- ✅ **ALWAYS handle context cancellation**

### 2.2 Code Formatting

```bash
# Format code
go fmt ./...

# Or use make (see Section 7)
make fmt
```

### 2.3 Comment Standards (Godoc)

**ALL exported** (capitalized) structs, interfaces, functions MUST have comments.

**Format:**
```go
// Agent represents the core reasoning engine of Kore.
// It manages the lifecycle of the ReAct loop, coordinating
// LLM interactions, tool execution, and context management.
//
// The Agent follows the ReAct (Reasoning + Acting) pattern:
// 1. Observe: Receive user input or tool output
// 2. Reason: Consult LLM for next action
// 3. Act: Execute tools or generate response
// 4. Repeat until completion
type Agent struct {
    UI          UIInterface
    ContextMgr  *ContextManager
    LLMProvider *LLMProvider
    // ...
}

// Run starts the agent's main ReAct loop with the given user message.
// It blocks until the conversation completes or context is cancelled.
func (a *Agent) Run(ctx context.Context, userMessage string) error {
    // ...
}
```

**Comments should:**
- Start with the function/struct name
- Explain WHAT and WHY, not HOW
- Mention important behaviors (concurrency, blocking, etc.)
- Use proper grammar and punctuation

---

## 3. Git Workflow

Adopt **Trunk Based Development** or **GitHub Flow**. Avoid overly complex Git Flow.

### 3.1 Branch Strategy

- **`main`** - Production-ready branch, MUST always be compile-able and runnable
- **`feat/xxx`** - Feature branches
- **`fix/xxx`** - Bugfix branches
- **`docs/xxx`** - Documentation updates
- **`refactor/xxx`** - Refactoring

### 3.2 Commit Messages (Conventional Commits)

**Format:** `<type>(<scope>): <subject>`

**Types:**
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation changes
- `style` - Code style changes (formatting, etc.)
- `refactor` - Code refactoring
- `test` - Adding or updating tests
- `chore` - Build process, dependencies, tooling

**Examples:**
```
feat(core): implement Agent ReAct loop with tool calling
fix(tools): add boundary checks for read_file line ranges
docs(readme): update installation instructions
chore(deps): upgrade golangci-lint to v1.55.2
test(integration): add SecurityInterceptor path traversal tests
```

### 3.3 Code Review Standards

**Principle:** No code can be merged to `main` without review.

**Review Checklist:**
- [ ] Logic matches Agent state machine design?
- [ ] Includes corresponding test cases?
- [ ] No security vulnerabilities (e.g., `rm -rf` risks)?
- [ ] Error handling complete?
- [ ] Comments and documentation updated?
- [ ] No circular dependencies introduced?
- [ ] Linter passes (`make lint`)?

---

## 4. Testing Strategy

Establish a pyramid testing system tailored for AI tools.

### 4.1 Unit Tests - Coverage Target > 80%

**Targets:** ContextManager, SecurityInterceptor, utility functions

**Mocking:** Use `go.uber.org/mock` or hand-written mocks. NEVER call real OpenAI API in unit tests.

```go
// ❌ BAD - Real API call
func TestChat(t *testing.T) {
    client := NewOpenAIClient()
    client.CallOpenAI(...) // Don't do this!
}

// ✅ GOOD - Mocked
type MockLLM struct {
    mock.Mock
}

func (m *MockLLM) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
    // Return fake stream
    stream := make(chan StreamEvent)
    go func() {
        stream <- StreamEvent{Type: EventContent, Content: "test"}
        close(stream)
    }()
    return stream, nil
}

func TestAgentRun(t *testing.T) {
    mockLLM := &MockLLM{}
    agent := &Agent{LLMProvider: mockLLM}
    // Test agent logic without real API
}
```

### 4.2 Integration Tests

**Targets:** File system operations, Git operations

**Environment:** Use temporary directories (`t.TempDir()`)

```go
func TestWriteFileTool(t *testing.T) {
    // Create temp directory
    tmpDir := t.TempDir()

    // Test write_file creates file
    tool := &WriteFileTool{
        fs: &SecurityInterceptor{ProjectRoot: tmpDir},
    }

    result, err := tool.Execute(ctx, args)
    assert.NoError(t, err)

    // Verify file exists
    content, _ := os.ReadFile(filepath.Join(tmpDir, "test.go"))
    assert.NotEmpty(t, content)
}
```

### 4.3 Golden File Testing - For TUI/Diff

**Problem:** TUI UI is hard to assert.

**Solution:** Save UI render output as `.golden` files. Compare future renders against golden files.

```go
func TestTUIRender(t *testing.T) {
    model := NewInitialModel()
    view := model.View()

    expected, _ := os.ReadFile("testdata/view.golden")
    if !reflect.DeepEqual(string(expected), view) {
        // Update golden file if needed: go test ./... -update-golden
        t.Fatalf("View output mismatch")
    }
}
```

### 4.4 AI Evaluation Tests (Evals) - Advanced

Create `tests/evals/test_cases.json` with tasks like "add logging to main.go". Run weekly to check if Agent still completes tasks correctly, preventing model "dumbing" or prompt degradation.

```json
[
  {
    "name": "add_logging_to_main",
    "description": "Add structured logging to main.go",
    "setup": {
      "files": {
        "main.go": "package main\n\nfunc main() {\n    println(\"Hello\")\n}"
      }
    },
    "assertions": [
      "main.go contains import \"log\"",
      "main.go contains log.Info call"
    ]
  }
]
```

### 4.5 Test Commands

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific tests
go test -v ./internal/core/...

# Run race detector
go test -race ./...
```

---

## 5. Documentation Engineering

**ADR (Architecture Decision Records)** are the most valuable asset of large projects.

### 5.1 ADR Format

Create ADRs in `docs/adr/` with format:

**Filename:** `001-choose-bubbletea-for-tui.md`

```markdown
# ADR 001: Choose Bubble Tea as TUI Framework

## Status
Accepted

## Context
We need a cross-platform, high-performance TUI library for Kore's interactive terminal interface.

## Decision
Use [Bubble Tea](https://github.com/charmbracelet/bubbletea) as the TUI framework.

### Alternatives Considered

1. **tview**
   - Pros: Rich widget set
   - Cons: Complex API, harder to test
   - Verdict: Rejected due to learning curve

2. **bubbletea**
   - Pros: Elm architecture, easy to test, excellent ecosystem
   - Cons: Requires learning functional paradigm
   - Verdict: **ACCEPTED**

3. **termui**
   - Pros: Simple
   - Cons: Less actively maintained
   - Verdict: Rejected

## Consequences
- Positive: Testable, maintainable codebase
- Positive: Access to Charm ecosystem (lipgloss, glamour)
- Negative: Team must learn Elm architecture pattern

## Related
- Design Doc: Section 6 (Implementation Phases)
- ADR 002: Agent-centric architecture
```

### 5.2 Required ADRs

1. **ADR 001:** Choose Bubble Tea for TUI framework
2. **ADR 002:** Agent-centric architecture with interface decoupling
3. **ADR 003:** Layered context strategy (directory tree + focused files)
4. **ADR 004:** ReAct loop implementation pattern
5. **ADR 005:** Security sandbox design

### 5.3 Code Comments

Complex logic MUST be explained with inline comments:

```go
// ⚠️ CRITICAL: Must add path separator to prevent /home/user/project
// from matching /home/user/project-evil
if !strings.HasPrefix(absPath, s.ProjectRoot+string(os.PathSeparator)) {
    return "", fmt.Errorf("SECURITY: Path traversal detected: %s", inputPath)
}

// 保存 AI 的完整回复（思考 + 工具意图）
// 这一步至关重要，否则下一轮 LLM 不知道自己为什么调工具
a.History.AddAssistantMessage(fullContent, currentToolCalls)
```

---

## 6. CI/CD Automation

Configure pipeline in GitHub Actions (`.github/workflows/ci.yml`).

### 6.1 Pipeline Stages

```yaml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: v1.55.2

  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run tests
        run: go test -v -race -cover ./...

  security:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Run Gosec Security Scanner
        uses: securego/gosec@master
        with:
          args: ./...

  build:
    name: Build
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [linux, macos, windows]
        arch: [amd64, arm64]
    steps:
      - uses: actions/checkout@v3
      - name: Build
        run: go build -o bin/kore cmd/kore/main.go
```

### 6.2 Quality Gates

- ❌ Linter fails → Block PR
- ❌ Test coverage < 80% → Warning
- ❌ Security scan finds vulnerabilities → Block PR
- ❌ Build fails → Block PR

---

## 7. Getting Started

### Step 1: Install Tools

```bash
# Install Go 1.21+
# Download from https://go.dev/dl/

# Install Linter
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2

# Install Mock generator
go install go.uber.org/mock/mockgen@latest

# Install gosec (security scanner)
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

### Step 2: Create Makefile

**Root Makefile:**

```makefile
.PHONY: all fmt lint test test-coverage build run clean generate help

# Default target
all: fmt lint test build

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

## lint: Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

## test: Run all tests
test:
	@echo "Running tests..."
	@go test -v -race ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## build: Build the binary
build:
	@echo "Building..."
	@go build -o bin/kore cmd/kore/main.go

## run: Run the application
run:
	@echo "Running Kore..."
	@go run cmd/kore/main.go

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html

## generate: Generate mock code
generate:
	@echo "Generating mocks..."
	@go generate ./...

## security: Run security scan
security:
	@echo "Running security scan..."
	@gosec ./...

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
```

### Step 3: Initialize Project

```bash
# Initialize Go module
go mod init github.com/yourusername/kore

# Create directory structure
mkdir -p cmd/kore internal/{core,adapters,tools,infrastructure} pkg/{logger,utils} api/prompts configs docs/{adr,plans} tests/{integration,testdata,evals}

# Create initial files
touch cmd/kore/main.go
touch Makefile
touch .golangci.yml

# Install dependencies
go mod tidy
```

### Step 4: Configure Linter

**`.golangci.yml`:**

```yaml
linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - gosec
    - gocyclo
    - godot
    - misspell
    - goimports
  disable:
    - exhaustive # Too strict for MVP

linters-settings:
  gocyclo:
    min-complexity: 15
  govet:
    enable-all: true
  errcheck:
    check-type-assertions: true
    check-blank: true

run:
  timeout: 5m

issues:
  exclude-use-default: false
  max-issues-per-linter: 0
  max-same-issues: 0
```

### Step 5: Pre-commit Hook (Optional)

Install pre-commit framework:

```bash
# Install pre-commit
pip install pre-commit

# Create .pre-commit-config.yaml
cat > .pre-commit-config.yaml << 'EOF'
repos:
  - repo: local
    hooks:
      - id: go-fmt
        name: go fmt
        entry: gofmt
        language: system
        args: [-w]
        files: \.go$

      - id: go-lint
        name: golangci-lint
        entry: golangci-lint run --fix
        language: system
        pass_filenames: false
EOF

# Install hooks
pre-commit install
```

---

## Summary

This specification's core philosophy is:

**"Defensive Programming + Automated Verification"**

- **Defense:** Prevent architecture rot through directory structure, prevent external dependencies through interfaces
- **Verification:** Verify logic through mock tests, verify UI through Golden File tests, ensure quality through CI

**This document is the "Constitution" of the Kore project.** All contributors MUST follow these standards.

---

## Appendix: Quick Reference

### Common Commands

```bash
# Development cycle
make fmt          # Format code
make lint         # Check code quality
make test         # Run tests
make build        # Build binary
make run          # Run application

# Testing
go test -v ./internal/core/...           # Test specific package
go test -race ./...                       # Race detector
go test -cover ./...                      # Coverage
go test -run TestAgent ./...              # Specific test

# Code generation
go generate ./...                         # Generate mocks
make generate                             # Via makefile

# Build
go build -o bin/kore cmd/kore/main.go    # Direct build
make build                                # Via makefile

# Dependencies
go mod tidy                               # Clean dependencies
go mod download                           # Download dependencies
go mod verify                             # Verify dependencies
```

### Recommended IDE Setup

**VSCode:**
- Install `golang.go` extension
- Enable `formatOnSave`
- Enable `lintOnSave`

**Settings:**
```json
{
  "go.formatTool": "goimports",
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "go.formatOnSave": true
}
```

---

**Document Version:** 1.0
**Last Updated:** 2026-01-16
**Maintainer:** Kore Team
