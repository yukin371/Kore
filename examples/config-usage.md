# Kore 配置系统使用指南

## Phase 1.7 - 配置系统更新

Kore 现在支持 JSONC 格式的配置文件，提供更好的可读性和灵活性。

## 配置文件位置

Kore 按以下优先级顺序加载配置（后者覆盖前者）：

1. **项目根目录**: `.kore.jsonc`
2. **用户配置目录**: `~/.kore/config.jsonc`
3. **环境变量**: `KORE_CONFIG`（指定配置文件路径）
4. **环境变量覆盖**: `KORE_*` 前缀的环境变量

## JSONC 格式示例

JSONC 是带注释的 JSON 格式，支持 `//` 和 `/* */` 注释：

```jsonc
{
  // LLM 提供商配置
  "llm": {
    "provider": "openai",  // openai 或 ollama
    "model": "gpt-4",
    "api_key": "your-api-key",  // 可选，也可通过 KORE_LLM_API_KEY 设置
    "temperature": 0.7,
    "max_tokens": 4000
  },

  // 上下文管理配置
  "context": {
    "max_tokens": 8000,
    "max_tree_depth": 5,
    "max_files_per_dir": 50
  },

  // 安全设置
  "security": {
    "blocked_cmds": ["rm", "sudo", "shutdown"],
    "blocked_paths": [".git", ".env"]
  },

  // UI 设置
  "ui": {
    "mode": "cli",  // cli, tui, 或 gui
    "stream_output": true
  }
}
```

## 环境变量覆盖

可以通过环境变量覆盖任何配置：

```bash
# LLM 配置
export KORE_LLM_PROVIDER=ollama
export KORE_LLM_MODEL=llama2
export KORE_LLM_API_KEY=sk-...
export KORE_LLM_TEMPERATURE=0.8

# UI 配置
export KORE_UI_MODE=tui

# 上下文配置
export KORE_CONTEXT_MAX_TOKENS=16000
```

## 配置验证

所有配置都会根据 JSON Schema 进行验证。如果配置无效，Kore 会显示详细的错误信息。

Schema 文件位置：`schemas/config.schema.json`

## 向后兼容

Kore 仍然支持旧的 YAML 配置格式（`~/.config/kore/config.yaml`）。如果没有找到 JSONC 配置文件，系统会自动回退到旧的配置系统。

## 使用示例

### 1. 创建项目配置

在项目根目录创建 `.kore.jsonc`：

```bash
cp examples/.kore.jsonc .kore.jsonc
# 编辑配置
vim .kore.jsonc
```

### 2. 使用环境变量

```bash
# 临时设置
KORE_LLM_PROVIDER=ollama kore chat

# 或导出为环境变量
export KORE_LLM_PROVIDER=ollama
export KORE_LLM_MODEL=llama2
kore chat
```

### 3. 验证配置

配置验证会在启动时自动进行。如果需要手动验证：

```go
import koreconfig "github.com/yukin/kore/internal/config"

loader := koreconfig.NewLoader()
cfg, err := loader.Load()
if err != nil {
    log.Fatalf("配置加载失败: %v", err)
}
```

## 配置优先级示例

假设有以下配置：

1. `~/.kore/config.jsonc`: `provider: "openai"`, `model: "gpt-4"`
2. `.kore.jsonc`: `provider: "ollama"`, `temperature: 0.8`
3. `KORE_LLM_MODEL=llama2`

最终配置将是：
- `provider: "ollama"` (来自项目根目录)
- `model: "llama2"` (来自环境变量，优先级最高)
- `temperature: 0.8` (来自项目根目录)
- 其他配置来自 `~/.kore/config.jsonc`

## 技术实现

- **JSONC 解析**: 使用 `github.com/tidwall/gjson`
- **Schema 验证**: 使用 `github.com/santhosh-tekuri/jsonschema/v5`
- **配置合并**: 支持多层配置覆盖

## 测试

运行配置系统测试：

```bash
go test ./internal/config/... -v
```

测试覆盖率：67.6%
