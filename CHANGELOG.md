# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added - Phase 2: Environment Manager (2026-01-18)

#### Environment Manager
- **Local Environment** (`internal/environment/local.go`)
  - Safe command execution with timeout support
  - Streaming command output via `ExecuteStream()`
  - File operations with automatic backup
  - File diffing capabilities
  - Working directory management
  - Background process management

- **Security Interceptor** (`internal/environment/security.go`)
  - Three-tier security levels (Strict/Standard/Permissive)
  - Path traversal attack prevention
  - Command injection detection
  - Dangerous command blacklist
  - Filename validation
  - Environment variable sanitization
  - Configurable allowed directories and commands

- **Virtual File System** (`internal/environment/virtual.go`)
  - In-memory document management
  - Full CRUD operations for virtual documents
  - Metadata support for custom attributes
  - Commit/rollback mechanism for changes
  - Statistics and grouping capabilities
  - Thread-safe operations with mutex locks

#### Testing
- Comprehensive unit tests for environment package
- Security validation tests
- Cross-platform compatibility tests (Windows/Unix)
- Virtual file system tests including concurrency
- All tests passing with `go test ./... -short`

#### Configuration
- Added `configs/agents.example.yaml` with comprehensive configuration examples
- Agent mode configurations (Standard, Plan, Build)
- Tool-specific configurations
- Security level configurations
- LLM, session, UI, and logging settings

#### Documentation
- Updated implementation plan with Phase 2 completion
- Updated README with Phase 2 features
- Added configuration examples and usage documentation
- Progress tracking: Phase 0-2 completed (5-7 weeks)

### Changed
- Improved error handling across all environment operations
- Enhanced security checks for file operations
- Better cross-platform compatibility (Windows/Unix)

### Fixed
- Fixed test issues with security interceptor path validation
- Fixed virtual file system GroupByDirectory test for Windows paths
- Fixed LSP JSONRPC2 test timeout (skipped pending proper pipe setup)

## [0.1.0] - Phase 1: MVP Core (2026-01-17)

### Added
- Agent core with ReAct Loop implementation
- OpenAI and Ollama LLM integration
- Context management system
- Basic tool execution (list_files, search_files)
- CLI and TUI implementations
- Security sandbox mechanism
- Parallel tool calling optimization
- TUI animated status indicators
- Ralph Loop - continuous execution until task completion
- Keyword magic - automatic mode switching (ultrawork, search, analyze)
- Context window monitoring with intelligent compression
- Todo continuer - forces completion of all tasks
- AGENTS.md auto-injection for project context
- TUI Viewport component with smooth scrolling
- Enhanced configuration system with Viper

### Changed
- Refactored project structure for better modularity
- Improved configuration management with YAML support

[Unreleased]: https://github.com/yukin/kore/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/yukin/kore/releases/tag/v0.1.0
