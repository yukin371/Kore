# Kore 项目

## 项目概述

Kore 是一个用 Go 语言构建的 AI 驱动的工作流自动化平台，作为所有开发任务的核心中枢。

## 架构说明

### 目录结构

```
kore-foundation/
├── cmd/                    # 应用程序入口
├── internal/               # 内部包（不对外暴露）
│   ├── adapters/          # 适配器层（CLI、TUI、LLM Provider）
│   ├── agent/             # 智能体编排系统
│   ├── core/              # 核心领域模型
│   ├── tools/             # 工具层
│   └── infrastructure/    # 基础设施层
├── pkg/                   # 公共库（可被外部使用）
├── api/                   # API 资源（系统提示词）
└── docs/                  # 文档
```

### 核心组件

- **Agent**: AI 代理，实现 ReAct（推理+行动）循环
- **ContextManager**: 智能上下文管理，分层加载项目文件
- **LLMProvider**: 统一的 LLM 接口，支持多种提供商
- **ToolExecutor**: 安全的工具执行沙箱

## 代码规范

### 命名约定

- 文件名：使用小写字母和下划线（如 `context_manager.go`）
- 包名：使用小写单词（如 `agent`、`core`）
- 导出的类型和函数：使用 PascalCase（如 `ContextManager`）
- 私有的类型和函数：使用 camelCase（如 `buildContext`）

### 错误处理

所有可能失败的操作都必须返回 `error`。使用 `fmt.Errorf` 包装错误：

```go
if err != nil {
    return nil, fmt.Errorf("failed to build file tree: %w", err)
}
```

### 并发安全

使用 `sync.RWMutex` 保护共享数据：

```go
type ContextManager struct {
    mu sync.RWMutex
    data map[string]string
}
```

## 开发指南

### 添加新功能

1. 在 `internal/` 下创建或修改包
2. 更新相关文档
3. 确保通过所有测试
4. 提交前运行 `go mod tidy`

### 添加新工具

1. 在 `internal/tools/` 下创建新工具文件
2. 实现 `ToolExecutor` 接口
3. 在 `executor.go` 中注册工具

## 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/core/...

# 查看覆盖率
go test -cover ./...
```

## 相关文档

- 架构文档: `docs/ARCHITECTURE.md`
- 实施计划: `docs/plans/2026-01-17-kore-2.0-implementation.md`
- 改进分析: `docs/improvements-from-opensource.md`
