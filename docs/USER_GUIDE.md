# Kore 使用指南

## 目录

1. [项目简介](#项目简介)
2. [快速开始](#快速开始)
3. [安装配置](#安装配置)
4. [基本使用](#基本使用)
5. [工具说明](#工具说明)
6. [UI 模式](#ui-模式)
7. [配置选项](#配置选项)
8. [使用示例](#使用示例)
9. [故障排查](#故障排查)
10. [最佳实践](#最佳实践)

---

## 项目简介

**Kore** 是一个 AI 驱动的自动化工作流平台，采用混合 CLI/TUI/GUI 界面。通过自然语言交互，Kore 可以帮助你：

- 📖 理解和解释代码
- ✏️ 修改和创建文件
- 🔍 搜索代码和文本
- 🚀 执行命令和测试
- 📂 浏览项目结构

### 核心特性

- ✅ **AI 驱动**: 支持多种 LLM 提供商（智谱 AI、OpenAI、Ollama）
- ✅ **工具调用**: 5 个内置工具，安全可靠
- ✅ **优雅界面**: CLI 和 TUI 两种交互模式
- ✅ **智能上下文**: 自动理解项目结构
- ✅ **安全防护**: 路径验证、命令黑名单

---

## 快速开始

### 前置要求

- Go 1.23 或更高版本
- 一个 LLM API 密钥（智谱 AI / OpenAI）或本地 Ollama

### 5 分钟快速体验

```bash
# 1. 克隆项目
git clone https://github.com/yukin/kore-foundation.git
cd kore-foundation

# 2. 安装依赖
go mod download

# 3. 构建项目
go build -o bin/kore.exe ./cmd/kore

# 4. 配置 API 密钥
# 编辑 ~/.config/kore/config.yaml，设置你的 API 密钥

# 5. 启动 TUI 模式
./bin/kore.exe chat --ui tui

# 6. 输入你的问题
> 请帮我解释这个项目的结构
```

---

## 安装配置

### 1. 构建应用

```bash
# 克隆仓库
git clone https://github.com/yukin/kore-foundation.git
cd kore-foundation

# 下载依赖
go mod download

# 构建可执行文件
go build -o bin/kore.exe ./cmd/kore
```

### 2. 配置文件

Kore 的配置文件位于：`~/.config/kore/config.yaml`

#### Windows 配置路径
```
C:\Users\<用户名>\AppData\Roaming\kore\config.yaml
```

#### Linux/Mac 配置路径
```
~/.config/kore/config.yaml
```

### 3. 配置示例

#### 使用智谱 AI（推荐）

```yaml
llm:
  provider: openai
  model: glm-4 # 智谱 AI 的模型名称，请参考官方文档
  api_key: your-zhipu-api-key # 这里输入你的 API 密钥
  base_url: https://open.bigmodel.cn/api/paas/v4/
  temperature: 0.7
  max_tokens: 4000

ui:
  mode: tui  # cli, tui, or gui
```

#### 使用 Ollama（本地模型）

```yaml
llm:
  provider: ollama
  model: llama3.1
  base_url: http://localhost:11434
  temperature: 0.7
  max_tokens: 4000

ui:
  mode: tui
```

#### 使用 OpenAI

```yaml
llm:
  provider: openai
  model: gpt-4
  api_key: your-openai-api-key
  base_url: https://api.openai.com/v1
  temperature: 0.7
  max_tokens: 4000

ui:
  mode: tui
```

---

## 基本使用

### 命令行语法

```bash
# 启动交互式聊天（使用配置文件中的 UI 模式）
./bin/kore.exe chat

# 启动 TUI 模式
./bin/kore.exe chat --ui tui

# 启动 CLI 模式
./bin/kore.exe chat --ui cli

# 单次消息模式
./bin/kore.exe chat "你的问题"

# 查看版本
./bin/kore.exe version
```

### 第一次运行

1. **启动应用**

```bash
./bin/kore.exe chat --ui tui
```

2. **输入你的问题**

在 TUI 模式下，直接在输入框中输入：

```
> 请帮我解释这个项目的结构
```

3. **确认工具调用**

当 Kore 需要执行工具时（如读取文件），会弹出确认对话框：

```
⚠️  确认操作
工具: read_file
参数: {"path":"README.md"}

› 是 (Y)  › 否 (N)
```

使用左右箭头键选择，回车确认。

4. **查看结果**

Kore 会将结果显示在屏幕上，支持：
- Markdown 渲染（代码高亮）
- 语法高亮
- 自动换行

---

## 工具说明

Kore 内置 5 个强大的工具，帮助 AI 理解和操作你的项目。

### 1. read_file - 读取文件

**功能**: 读取文件内容，支持指定行范围。

**参数**:
```json
{
  "path": "main.go",           // 文件路径（必需）
  "line_start": 10,            // 起始行号（可选）
  "line_end": 20                // 结束行号（可选）
}
```

**使用场景**:
- 查看代码文件
- 阅读文档
- 检查配置文件
- 查看特定函数实现

**示例**:
```
# 读取整个文件
read_file({"path": "README.md"})

# 读取特定行范围
read_file({"path": "main.go", "line_start": 10, "line_end": 20})
```

### 2. write_file - 写入文件

**功能**: 写入或创建文件，支持 diff 预览。

**参数**:
```json
{
  "path": "main.go",    // 文件路径（必需）
  "content": "..."      // 文件内容（必需）
}
```

**使用场景**:
- 创建新文件
- 修改现有代码
- 更新配置文件
- 生成文档

**安全特性**:
- ✅ 写入前显示 diff
- ✅ 用户确认后才执行
- ✅ 路径验证防止越界

### 3. run_command - 执行命令

**功能**: 执行 shell 命令，支持跨平台。

**参数**:
```json
{
  "cmd": "ls -la"      // 要执行的命令（必需）
}
```

**使用场景**:
- 运行测试
- 构建项目
- 查看 git 状态
- 执行脚本

**安全特性**:
- ✅ 命令黑名单（rm, sudo, format 等）
- ✅ 超时保护（120 秒）
- ✅ 输出截断（最大 10KB）
- ✅ 跨平台支持

**示例**:
```
# 列出文件
run_command({"cmd": "ls -la"})

# 运行测试
run_command({"cmd": "go test ./..."})

# Git 状态
run_command({"cmd": "git status"})
```

### 4. search_files - 搜索文件

**功能**: 在项目文件中搜索文本内容。

**参数**:
```json
{
  "pattern": "func main",        // 搜索模式（必需）
  "file_pattern": "*.go",        // 文件类型过滤（可选）
  "case_sensitive": false,       // 是否区分大小写（可选）
  "max_results": 50              // 最大结果数（可选）
}
```

**使用场景**:
- 搜索函数定义
- 查找变量使用
- 定位 bug 位置
- 代码审计

**特性**:
- ✅ 支持正则表达式
- ✅ 优先使用 ripgrep（性能优化）
- ✅ 显示行号和上下文
- ✅ 文件类型过滤

**示例**:
```
# 搜索所有 Go 文件中的 main 函数
search_files({
  "pattern": "func main",
  "file_pattern": "*.go"
})

# 搜索包含 "TODO" 的所有文件
search_files({
  "pattern": "TODO",
  "case_sensitive": false
})
```

### 5. list_files - 列出文件

**功能**: 列出目录结构和文件。

**参数**:
```json
{
  "path": ".",                   // 起始路径（必需）
  "recursive": true,            // 是否递归（可选）
  "max_depth": 3,               // 最大深度（可选）
  "show_hidden": false,         // 显示隐藏文件（可选）
  "pattern": "*.go"              // 文件名过滤（可选）
}
```

**使用场景**:
- 浏览项目结构
- 查找特定类型文件
- 了解代码组织
- 检查测试覆盖

**特性**:
- ✅ 递归遍历
- ✅ 文件大小显示
- ✅ 目录分组显示
- ✅ 按类型排序

**示例**:
```
# 列出当前目录的 Go 文件
list_files({
  "path": ".",
  "pattern": "*.go"
})

# 递归列出所有文件（最多 3 层）
list_files({
  "path": ".",
  "recursive": true,
  "max_depth": 3
})
```

---

## UI 模式

Kore 支持两种交互模式：CLI 和 TUI。

### CLI 模式

**特点**:
- ✅ 简洁的命令行界面
- ✅ 适合脚本和自动化
- ✅ 流式输出

**使用方法**:
```bash
./bin/kore.exe chat --ui cli
```

**交互方式**:
```
> 请帮我列出项目的 Go 文件

[AI 回复...]

> exit
```

### TUI 模式（推荐）

**特点**:
- ✅ 美观的终端界面
- ✅ Tokyo Night 主题
- ✅ Markdown 渲染
- ✅ 交互式确认对话框
- ✅ 思考状态指示器
- ✅ 消息历史滚动

**使用方法**:
```bash
./bin/kore.exe chat --ui tui
```

**快捷键**:
- `Enter` - 发送消息
- `Ctrl+↑/↓` - 滚动消息历史
- `ESC` - 切换输入焦点
- `Ctrl+d` 或 `Tab` - 切换工具执行详情显示
- `Ctrl+C` - 退出程序
- `←/→` - 在确认对话框中选择

**TUI 界面元素**:
```
┌─────────────────────────────────────────┐
│                                         │
│  [消息显示区域]                         │
│  - AI 回复（支持 Markdown）            │
│  - 代码块高亮                          │
│                                         │
│  >> 输入你的问题...                      │
│                                         │
│  ●●● 读取文件...                        │
│  └─ 文件: main.go                       │
│  [Ctrl+↑/↓:滚动] [Ctrl+D:隐藏详情]      │
└─────────────────────────────────────────┘
```

**状态指示器说明**:

TUI 底部显示当前操作状态，具有以下特性：

- **颜色编码**:
  - 🟦 蓝色 - AI 思考中
  - 🔷 青色 - 读取文件
  - 🟪 紫色 - 搜索代码
  - 🟧 橙色 - 执行工具
  - 🟩 绿青色 - 生成回复
  - 🟢 绿色 - 操作成功
  - 🟥 红色 - 操作错误
  - ⚪ 灰色 - 准备就绪

- **动画效果**: 每个状态都有独特的 spinner 动画

- **自动重置**: 成功/错误状态会在 2 秒后自动恢复为"准备就绪"

- **详情视图**: 按 `Ctrl+d` 或 `Tab` 可以切换显示工具执行的详细信息（如文件名、搜索模式等）

---

## 配置选项

### LLM 配置

```yaml
llm:
  # LLM 提供商（openai 或 ollama）
  provider: openai

  # 模型名称
  model: glm-4

  # API 密钥
  api_key: your-api-key

  # API 地址
  base_url: https://open.bigmodel.cn/api/paas/v4/

  # 温度（0.0 - 2.0，越高越随机）
  temperature: 0.7

  # 最大 token 数
  max_tokens: 4000
```

### UI 配置

```yaml
ui:
  # 界面模式（cli, tui, gui）
  mode: tui
```

### 上下文配置

```yaml
context:
  # 最大 token 数
  max_tokens: 8000

  # 最大树深度
  max_tree_depth: 5

  # 每个目录最大文件数
  max_files_per_dir: 50
```

### 安全配置

```yaml
security:
  # 阻止执行的命令
  blocked_cmds:
    - rm
    - sudo
    - format
    - del

  # 阻止访问的路径
  blocked_paths:
    - .git
    - .env
    - node_modules/.cache
```

---

## 使用示例

### 示例 1: 理解代码

```
> 请帮我解释 main.go 中的逻辑

Kore 会：
1. 使用 read_file 读取 main.go
2. 分析代码结构
3. 用通俗易懂的语言解释
```

### 示例 2: 修改代码

```
> 请在 main.go 中添加日志功能

Kore 会：
1. read_file 查看当前代码
2. 提出修改方案
3. write_file 写入修改（带 diff 确认）
```

### 示例 3: 搜索代码

```
> 帮我找到所有包含 "database" 的 Go 文件

Kore 会：
1. search_files 搜索 "database"
2. 过滤 *.go 文件
3. 列出所有匹配结果
```

### 示例 4: 运行测试

```
> 请运行所有测试并报告结果

Kore 会：
1. list_files 查找测试文件
2. run_command 执行 go test
3. 分析测试结果
```

### 示例 5: 项目重构

```
> 请帮我将 utils.go 重复的函数提取到新文件

Kore 会：
1. read_file 读取 utils.go
2. 分析重复代码
3. 提出重构方案
4. create_new_file 创建新文件
5. write_file 更新旧文件
```

### 示例 6: 代码审查

```
> 请审查这个项目的代码质量

Kore 会：
1. list_files 浏览项目结构
2. search_files 查找潜在问题
3. read_file 审查关键文件
4. 提供改进建议
```

---

## 故障排查

### 问题 1: 配置文件未找到

**症状**:
```
Failed to load config: config file not found
```

**解决方案**:
1. 检查配置文件路径：
   - Windows: `C:\Users\<用户名>\AppData\Roaming\kore\config.yaml`
   - Linux/Mac: `~/.config/kore/config.yaml`
2. 如果不存在，创建配置文件：
   ```bash
   mkdir -p ~/.config/kore
   cp configs/default.yaml ~/.config/kore/config.yaml
   ```
3. 编辑配置文件，填入你的 API 密钥

### 问题 2: API 密钥错误

**症状**:
```
API 返回错误 401: Unauthorized
```

**解决方案**:
1. 检查 API 密钥是否正确
2. 确认 API 密钥有足够的额度
3. 检查 `base_url` 是否正确
4. 尝试重新生成 API 密钥

### 问题 3: TUI 显示异常

**症状**:
- TUI 界面错乱
- 颜色显示不正确
- 无法输入

**解决方案**:
1. 确保终端支持 ANSI 颜色
2. Windows 用户推荐使用 Windows Terminal
3. 尝试调整终端窗口大小
4. 使用 `--ui cli` 切换到 CLI 模式

### 问题 4: 工具执行失败

**症状**:
```
[Error: SECURITY: Path traversal detected]
```

**解决方案**:
1. 检查文件路径是否在项目根目录内
2. 不要使用 `../` 尝试访问上级目录
3. 使用相对路径而非绝对路径

### 问题 5: Ollama 连接失败

**症状**:
```
[Error: 发送请求失败: dial tcp 127.0.0.1:11434: connect refused]
```

**解决方案**:
1. 确认 Ollama 服务已启动：
   ```bash
   ollama serve
   ```
2. 检查端口是否正确（默认 11434）
3. 拉取模型：
   ```bash
   ollama pull llama3.1
   ```

---

## 最佳实践

### 1. 提示词编写

**好的提示词**:
```
✅ 请帮我重构 main.go 中的错误处理逻辑
✅ 分析当前代码的性能瓶颈并提出优化建议
✅ 帮我编写单元测试，覆盖 greet 函数
```

**不好的提示词**:
```
❌ 修复代码
❌ 改进一下
❌ 做这个
```

### 2. 复杂任务分解

对于复杂任务，建议分步执行：

```
> 第一步：请帮我列出项目的 Go 文件
> 第二步：请帮我查看 main.go 的结构
> 第三步：请帮我添加错误处理
```

### 3. 利用工具组合

```
> 请帮我找到所有测试文件并运行它们

Kore 会：
1. list_files 找到测试文件
2. run_command 运行测试
3. 分析结果
```

### 4. 验证修改

```
> 请帮我修改这个函数，然后运行测试验证

Kore 会：
1. read_file 查看代码
2. write_file 修改文件
3. run_command 运行测试
4. 报告结果
```

### 5. 代码审查

```
> 请帮我审查以下文件的代码质量：
- main.go
- utils.go
- config.go

重点关注：
1. 错误处理
2. 资源泄漏
3. 性能问题
```

### 6. 文档生成

```
> 请为这个项目生成 README.md

Kore 会：
1. list_files 了解项目结构
2. read_file 查看关键文件
3. 分析项目功能
4. write_file 生成 README
```

---

## 高级技巧

### 1. 使用 TUI 历史记录

在 TUI 模式下：
- 使用 `Ctrl+↑/↓` 查看之前的对话
- 使用 `Home/End` 跳转到顶部/底部
- 使用 `ESC` 在输入和历史间切换

### 2. 流式输出中断

当 AI 输出过长时：
- `Ctrl+C` 可以中断当前输出
- 输入新问题继续对话

### 3. 并行工具执行

在代码中启用：
```go
agent.Config.ParallelTools = true
```

这样多个工具调用会并发执行，提升效率。

### 4. 自定义工具

注册自定义工具：
```go
type MyTool struct {
    // 工具字段
}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    // 工具逻辑
    return "执行成功", nil
}

// 注册工具
toolExecutor.RegisterTool(&MyTool{})
```

---

## 常见问题 (FAQ)

### Q: 支持哪些 LLM 提供商？

A: 目前支持：
- 智谱 AI (glm-4)
- OpenAI (gpt-4, gpt-3.5)
- Ollama (llama3.1, qwen2.5, deepseek-coder 等)

### Q: 如何切换模型？

A: 编辑配置文件：
```yaml
llm:
  model: gpt-4  # 或其他模型
```

### Q: TUI 和 CLI 模式有什么区别？

A:
- **CLI**: 简洁的命令行界面，适合脚本
- **TUI**: 图形化终端界面，支持滚动、Markdown 渲染、交互式确认

### Q: 如何禁用某个工具？

A: 在 `RegisterDefaultTools()` 中注释掉不需要的工具：
```go
// te.RegisterTool(&RunCommandTool{security: te.security})
```

### Q: 可以在 Docker 中运行吗？

A: 可以，创建 `Dockerfile`:
```dockerfile
FROM golang:1.23-alpine
WORKDIR /app
COPY . .
RUN go build -o kore ./cmd/kore
ENTRYPOINT ["./kore"]
```

---

## 获取帮助

- **文档**: 查看 `docs/` 目录
- **示例**: 查看 `README.md`
- **问题**: 提交 GitHub Issue

---

## 版本信息

```
Kore version 0.6.0
Built with Go 1.23+
```

---

**享受使用 Kore！** 🚀
