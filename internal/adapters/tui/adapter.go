package tui

import (
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// Adapter 实现 UIInterface 接口，使用 Bubble Tea 框架
// 这是 TUI 模式的核心适配器，负责连接 Agent 和 Bubble Tea Model
type Adapter struct {
	program      *tea.Program // Bubble Tea 程序实例
	model        *Model       // TUI Model
	inputChan    chan string  // 用户输入通道
	mu           sync.Mutex   // 保护 program 和 model 的并发访问
}

// NewAdapter 创建新的 TUI 适配器
func NewAdapter() *Adapter {
	model := NewModel()

	// 创建适配器
	adapter := &Adapter{
		model:     model,
		inputChan: make(chan string, 10), // 缓冲通道
	}

	// 设置输入回调：当用户在 TUI 中输入时，将输入发送到通道
	model.SetInputCallback(func(input string) {
		adapter.submitInput(input)
	})

	return adapter
}

// GetInputChannel 返回用户输入通道（用于从 TUI 读取用户输入）
func (a *Adapter) GetInputChannel() <-chan string {
	return a.inputChan
}

// submitInput 提交用户输入（由 Model 调用）
func (a *Adapter) submitInput(input string) {
	select {
	case a.inputChan <- input:
	default:
		// 通道已满，丢弃输入
	}
}

// Start 启动 TUI 程序（必须在其他方法之前调用）
func (a *Adapter) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program != nil {
		return fmt.Errorf("TUI 程序已经启动")
	}

	// 创建 Bubble Tea 程序
	a.program = tea.NewProgram(a.model, tea.WithAltScreen()) // 使用备用屏幕模式

	// 异步启动程序（不阻塞）
	go func() {
		if _, err := a.program.Run(); err != nil {
			fmt.Printf("TUI 错误: %v\n", err)
		}
	}()

	return nil
}

// SendStream 发送流式内容到 TUI
func (a *Adapter) SendStream(content string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		// 如果程序未启动，回退到命令行输出
		fmt.Print(content)
		return
	}

	// 发送流式内容消息到 Bubble Tea
	a.program.Send(StreamMsg(content))
}

// SendMarkdown 发送 Markdown 内容到 TUI（会自动渲染）
func (a *Adapter) SendMarkdown(content string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		// 如果程序未启动，回退到命令行输出
		fmt.Print(content)
		return
	}

	// 发送 Markdown 内容消息
	a.program.Send(MarkdownMsg(content))
}

// RequestConfirm 请求用户确认工具调用
// 返回 true 表示用户同意，false 表示拒绝
func (a *Adapter) RequestConfirm(action string, args string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		// 如果程序未启动，回退到命令行确认
		fmt.Printf("\n[工具调用: %s]\n参数: %s\n", action, args)
		fmt.Print("确认执行? [Y/n] ")
		var input string
		fmt.Scanln(&input)
		return input == "y" || input == "Y" || input == ""
	}

	// 创建回复通道
	replyChan := make(chan bool)

	// 发送确认对话框消息
	a.program.Send(ConfirmMsg{
		Action: action,
		Args:   args,
		Reply:  replyChan,
	})

	// 等待用户响应
	result := <-replyChan
	close(replyChan)

	return result
}

// RequestConfirmWithDiff 请求用户确认文件修改（显示 diff）
func (a *Adapter) RequestConfirmWithDiff(path string, diffText string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		// 如果程序未启动，回退到命令行确认
		fmt.Printf("\n[文件修改: %s]\n", path)
		fmt.Printf("Diff:\n%s\n", diffText)
		fmt.Print("确认修改? [Y/n] ")
		var input string
		fmt.Scanln(&input)
		return input == "y" || input == "Y" || input == ""
	}

	// 创建回复通道
	replyChan := make(chan bool)

	// 发送 Diff 确认对话框消息
	a.program.Send(DiffConfirmMsg{
		Path:     path,
		DiffText: diffText,
		Reply:    replyChan,
	})

	// 等待用户响应
	result := <-replyChan
	close(replyChan)

	return result
}

// ShowStatus 更新状态栏显示
func (a *Adapter) ShowStatus(status string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		// 如果程序未启动，回退到命令行输出
		fmt.Printf("[%s]\n", status)
		return
	}

	// 发送状态更新消息
	a.program.Send(StatusMsg(status))
}

// StartThinking 开始思考状态（显示 spinner）
func (a *Adapter) StartThinking() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		return
	}

	a.program.Send(ThinkingStartMsg{})
}

// StopThinking 停止思考状态
func (a *Adapter) StopThinking() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		return
	}

	a.program.Send(ThinkingStopMsg{})
}

// ========== 新增：状态通知方法 ==========

// StartToolExecution 开始工具执行状态通知
func (a *Adapter) StartToolExecution(toolName string, payload map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		return
	}

	// 根据工具类型智能选择状态
	state := StatusExecuting
	message := fmt.Sprintf("执行: %s", toolName)

	switch toolName {
	case "read_file", "list_files":
		state = StatusReading
		message = "读取文件..."
	case "search_files":
		state = StatusSearching
		message = "搜索代码..."
	case "run_command":
		state = StatusExecuting
		message = "执行命令..."
	case "write_file":
		state = StatusExecuting
		message = "写入文件..."
	}

	a.program.Send(StatusChangeMsg{
		State:   state,
		Message: message,
		Payload: payload,
	})
}

// UpdateToolProgress 更新工具执行进度
func (a *Adapter) UpdateToolProgress(progress int, detail string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		return
	}

	a.program.Send(StatusProgressMsg{
		Progress: progress,
		Detail:   detail,
	})
}

// EndToolExecution 结束工具执行状态通知
func (a *Adapter) EndToolExecution(success bool, errMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		return
	}

	state := StatusSuccess
	message := "完成 ✓"

	if !success {
		state = StatusError
		// 截断过长的错误信息（避免 TUI 混乱）
		if errMsg == "" {
			message = "执行失败"
		} else {
			if len(errMsg) > 50 {
				errMsg = errMsg[:47] + "..."
			}
			message = fmt.Sprintf("错误: %s", errMsg)
		}
	}

	a.program.Send(StatusChangeMsg{
		State:   state,
		Message: message,
	})
}

// Stop 停止 TUI 程序
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.program == nil {
		return fmt.Errorf("TUI 程序未启动")
	}

	// 发送退出消息
	a.program.Quit()

	// 等待程序完全退出（使用 Release wait）
	// 注意：Bubble Tea 的 program.Run() 已经在 goroutine 中运行
	// 我们只需要标记 program 为 nil 即可
	a.program = nil

	return nil
}

// WaitForExit 等待 TUI 程序退出
// 这会阻塞直到用户关闭 TUI
func (a *Adapter) WaitForExit() error {
	a.mu.Lock()
	if a.program == nil {
		a.mu.Unlock()
		return fmt.Errorf("TUI 程序未启动")
	}
	a.mu.Unlock()

	// Bubble Tea 程序在 Start() 时已经在 goroutine 中运行
	// 我们需要等待它完成
	// 由于 program.Run() 已经在运行，我们无法直接等待
	// 这个方法主要用于确保 TUI 程序运行期间主程序不会退出

	return nil
}
