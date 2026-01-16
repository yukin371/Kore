# Kore

**Kore** is an AI-powered workflow automation platform built with Go, serving as the core中枢 for all development tasks. Inspired by Claude Code and aligned with agent platforms like Dify, Kore provides intelligent code understanding, modification, and automation capabilities through natural language interaction.

## Features

- 🤖 **AI-Powered**: Integrates with OpenAI API and Ollama for intelligent code assistance
- 🎨 **Hybrid Interface**: CLI, TUI (Terminal UI), and GUI support
- 🔧 **Tool Execution**: Safe file operations and command execution with confirmation
- 📁 **Context Management**: Smart project context loading with dynamic file browsing
- 🛡️ **Security Sandbox**: Built-in security layers to prevent malicious operations
- 🚀 **Extensible**: Plugin-based tool system for custom workflows

## Installation

### From Source

```bash
go install github.com/yourusername/kore/cmd/kore@latest
```

### Build from Source

```bash
git clone https://github.com/yourusername/kore.git
cd kore
make build
```

## Quick Start

```bash
# Initialize Kore in your project
cd /path/to/your/project
kore init

# Start interactive chat
kore chat

# Ask a single question
kore ask "Explain how the authentication system works"

# Focus on specific files
kore chat -f main.go -f auth.go

# Run a command with AI assistance
kore do "Run tests and fix any failures"
```

## Configuration

Kore looks for configuration in `~/.config/kore/config.yaml`:

```yaml
llm:
  provider: "openai"  # or "ollama"
  model: "gpt-4"
  api_key: "your-api-key"
  base_url: "https://api.openai.com/v1"
  temperature: 0.7

context:
  max_tokens: 16000
  max_tree_depth: 5
  max_files_per_dir: 50

security:
  blocked_cmds:
    - "rm"
    - "sudo"
    - "shutdown"
  blocked_paths:
    - ".git"
    - ".env"
    - "node_modules/.cache"
```

## Development

See [CONTRIBUTING.md](docs/DEV_GUIDE.md) for detailed development guidelines.

### Setup Development Environment

```bash
# Install development tools
make install-tools

# Run tests
make test

# Run linter
make lint

# Build
make build
```

### Project Structure

```
kore/
├── cmd/                    # Entry point
├── internal/               # Private business logic
│   ├── core/              # Core domain models
│   ├── adapters/          # Port adapters
│   ├── tools/             # Tool execution
│   └── infrastructure/    # Infrastructure services
├── pkg/                   # Public libraries
├── api/prompts/           # System prompts
├── docs/                  # Documentation
└── tests/                 # Tests
```

## Architecture

Kore follows an **Agent-centric architecture** with **interface-based decoupling**:

- **Agent**: Core reasoning engine managing the ReAct loop
- **ContextManager**: Smart project context loading
- **LLMProvider**: Unified interface for multiple LLM backends
- **ToolExecutor**: Safe tool execution with security sandbox

See [docs/plans/2026-01-16-kore-design.md](docs/plans/2026-01-16-kore-design.md) for detailed design documentation.

## Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run integration tests
make test-integration

# Run AI evaluation tests
make test-evals
```

## Roadmap

### Phase 1: MVP Core (Current)
- [x] Project structure and engineering standards
- [ ] Agent core with ReAct loop
- [ ] OpenAI integration
- [ ] Context management
- [ ] Basic tool execution (read_file, write_file, run_command)
- [ ] CLI and TUI implementation

### Phase 2: Advanced Features
- [ ] Ollama integration
- [ ] Advanced tools (search, list_files)
- [ ] Parallel tool calling
- [ ] Conversation history persistence

### Phase 3: GUI
- [ ] Wails integration
- [ ] React frontend
- [ ] Monaco Editor integration
- [ ] Terminal emulator (xterm.js)

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](docs/DEV_GUIDE.md) for guidelines.

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Inspired by [Claude Code](https://claude.ai/code)
- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- LLM integration via [go-openai](https://github.com/sashabaranov/go-openai)

## Contact

- GitHub Issues: [github.com/yourusername/kore/issues](https://github.com/yourusername/kore/issues)
- Documentation: [docs/](docs/)

---

**Note**: Kore is currently in active development. The API and features may change before v1.0 release.
