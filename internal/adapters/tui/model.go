package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// ========== 状态枚举定义 ==========

// StatusState 表示当前的操作状态
type StatusState int

const (
	StatusIdle      StatusState = iota // 空闲
	StatusThinking                      // AI 思考中
	StatusReading                       // 读取文件
	StatusSearching                     // 搜索代码
	StatusExecuting                     // 执行工具
	StatusStreaming                     // 生成回复
	StatusSuccess                       // 成功（临时）
	StatusError                         // 错误（临时）
)

// AnimatedStatus 动画状态管理器
type AnimatedStatus struct {
	state       StatusState       // 当前状态
	spinner     spinner.Model     // 动画 spinner
	message     string            // 状态消息
	progress    int               // 进度 0-100
	timestamp   time.Time         // 用于自动重置
	showDetails bool              // 是否显示详细工具信息
	payload     map[string]string // 上下文元数据
}

// ========== 消息类型定义 ==========

// StreamMsg 流式内容消息
type StreamMsg string

// MarkdownMsg Markdown 内容消息
type MarkdownMsg string

// StatusMsg 状态更新消息
type StatusMsg string

// ThinkingStartMsg 开始思考状态
type ThinkingStartMsg struct{}

// ThinkingStopMsg 停止思考状态
type ThinkingStopMsg struct{}

// ========== 新增：状态切换消息 ==========

// StatusChangeMsg 状态切换消息
type StatusChangeMsg struct {
	State    StatusState       // 目标状态
	Message  string            // 状态消息
	Progress int               // 进度 0-100（可选）
	Payload  map[string]string // 上下文元数据（可选）
}

// StatusProgressMsg 进度更新消息
type StatusProgressMsg struct {
	Progress int    // 进度值 0-100
	Detail   string // 详细信息（可选）
}

// ResetStatusMsg 重置状态消息（用于定时器回调）
type ResetStatusMsg struct{}

// ToggleDetailsMsg 切换详情显示消息
type ToggleDetailsMsg struct{}

// ConfirmMsg 确认对话框消息
type ConfirmMsg struct {
	Action string   // 工具名称
	Args   string   // 工具参数
	Reply  chan bool // 用户选择的回复通道
}

// DiffConfirmMsg 带 diff 的确认消息
type DiffConfirmMsg struct {
	Path     string   // 文件路径
	DiffText string   // Diff 内容
	Reply    chan bool // 用户选择的回复通道
}

// ShowModalMsg 显示 Modal 消息
type ShowModalMsg struct {
	Type      ModalType         // Modal 类型
	Title     string            // 标题
	Content   string            // 内容
	OnConfirm func() bool       // 确认回调
	Reply     chan bool         // 用户选择的回复通道
}

// UserInputMsg 用户输入提交消息
type UserInputMsg struct {
	Input string       // 用户输入的内容
	Reply chan bool    // 确认通道（处理完成后通知）
}

// TickMsg 定时器消息（用于刷新 UI）
type TickMsg time.Time

// SpinnerTickMsg Spinner 定时器消息
type SpinnerTickMsg time.Time

// ========== Model 结构体 ==========

// Model 是 Bubble Tea 的核心 Model
type Model struct {
	// 消息内容
	messages []string

	// 当前正在输入的流式内容
	currentStream strings.Builder

	// 状态栏文本
	status string

	// Markdown 渲染器
	markdownRenderer *glamour.TermRenderer

	// 思考状态
	thinking      bool
	thinkingSpinner spinner.Model

	// 【新增】动画状态管理器（将逐步替换 thinking bool）
	animatedStatus AnimatedStatus

	// 【新增】Viewport 用于消息区域滚动和换行
	viewport viewport.Model

	// 用户输入框
	textInput     textinput.Model
	inputActive   bool // 是否激活输入状态
	inputReply    chan bool // 输入提交后的确认通道
	inputCallback func(string) // 输入提交的回调函数

	// 确认对话框状态
	confirming     bool   // 是否显示确认对话框
	confirmAction  string // 要执行的工具名称
	confirmArgs    string // 工具参数
	confirmReply   chan bool
	confirmChoice  int // 0=是, 1=否

	// Diff 确认对话框状态
	diffConfirming    bool
	diffConfirmPath   string
	diffConfirmText   string
	diffConfirmReply  chan bool
	diffConfirmChoice int

	// 【新增】Modal 组件
	modal *ModalComponent

	// 【新增】欢迎界面组件
	welcome *WelcomeComponent

	// 视口设置（支持滚动）
	scrollOffset int

	// 终端尺寸
	width  int
	height int

	// 样式配置
	styles *Styles
}

// ========== 样式配置 ==========

// Styles 定义 TUI 样式
type Styles struct {
	// 通用样式
	App       lipgloss.Style
	StatusBar lipgloss.Style
	Message   lipgloss.Style
	Stream    lipgloss.Style
	ToolCall  lipgloss.Style
	Error     lipgloss.Style
	Success   lipgloss.Style

	// 对话框样式
	Dialog         lipgloss.Style
	DialogTitle    lipgloss.Style
	DialogContent lipgloss.Style
	DialogOption   lipgloss.Style
	DialogSelected lipgloss.Style

	// Diff 样式
	DiffAdd    lipgloss.Style // 新增内容（绿色）
	DiffRemove lipgloss.Style // 删除内容（红色）
}

// DefaultStyles 返回默认样式配置（使用 Tokyo Night 主题）
func DefaultStyles() *Styles {
	s := &Styles{}

	// 定义颜色变量（Tokyo Night 配色方案）
	var (
		colorBackground = lipgloss.Color("#1a1b26") // 深色背景
		colorForeground = lipgloss.Color("#c0caf5") // 主要文字
		colorPrimary    = lipgloss.Color("#7aa2f7") // 蓝色（主色调）
		colorSuccess    = lipgloss.Color("#9ece6a") // 绿色（成功）
		colorWarning    = lipgloss.Color("#e0af68") // 橙色（警告）
		colorError      = lipgloss.Color("#f7768e") // 红色（错误）
		colorMuted      = lipgloss.Color("#565f89") // 灰色（次要文字）
		colorBorder     = lipgloss.Color("#414868") // 边框色
	)

	// 应用样式
	s.App = lipgloss.NewStyle().
		Foreground(colorForeground).
		Background(colorBackground).
		Padding(0, 0)

	s.StatusBar = lipgloss.NewStyle().
		Foreground(colorPrimary).
		Background(colorBorder).
		Padding(0, 1).
		Width(0)

	s.Message = lipgloss.NewStyle().
		Foreground(colorForeground).
		Padding(0, 1).
		MarginBottom(1)

	s.Stream = lipgloss.NewStyle().
		Foreground(colorForeground).
		Padding(0, 1)

	s.ToolCall = lipgloss.NewStyle().
		Foreground(colorWarning).
		Padding(0, 1).
		MarginBottom(1)

	s.Error = lipgloss.NewStyle().
		Foreground(colorError).
		Padding(0, 1).
		MarginBottom(1)

	s.Success = lipgloss.NewStyle().
		Foreground(colorSuccess).
		Padding(0, 1).
		MarginBottom(1)

	// 对话框样式
	s.Dialog = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Width(80)

	s.DialogTitle = lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		MarginBottom(1)

	s.DialogContent = lipgloss.NewStyle().
		Foreground(colorForeground).
		MarginBottom(1)

	s.DialogOption = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 2).
		MarginTop(1)

	s.DialogSelected = lipgloss.NewStyle().
		Foreground(colorSuccess).
		Bold(true).
		Padding(0, 2).
		MarginTop(1)

	// Diff 样式
	s.DiffAdd = lipgloss.NewStyle().
		Foreground(colorSuccess)

	s.DiffRemove = lipgloss.NewStyle().
		Foreground(colorError)

	return s
}

// ========== Model 构造函数 ==========

// NewModel 创建新的 Model
func NewModel() *Model {
	ti := textinput.New()
	ti.Placeholder = "输入消息..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 50

	// 创建 Markdown 渲染器（使用默认宽度，窗口大小变化时会更新）
	markdownRenderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80), // 默认宽度，稍后根据窗口大小调整
	)

	// 创建 spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("7aa2f7")) // Tokyo Night 蓝色

	// 【新增】初始化动画状态
	animStatus := AnimatedStatus{
		state:       StatusIdle,
		message:     "准备就绪",
		showDetails: false,
		payload:     make(map[string]string),
		spinner:     sp,
	}

	// 【新增】初始化 viewport
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.HiddenBorder())

	return &Model{
		messages:          make([]string, 0),
		status:            "准备就绪",
		markdownRenderer:  markdownRenderer,
		thinkingSpinner:   sp,
		textInput:         ti,
		inputActive:       true, // 默认激活输入状态
		confirmChoice:     0,
		diffConfirmChoice: 0,
		styles:            DefaultStyles(),
		animatedStatus:    animStatus,           // 【新增】
		viewport:          vp,                   // 【新增】
		modal:             NewModalComponent(),   // 【新增】Modal 组件
		welcome:           NewWelcomeComponent(), // 【新增】欢迎界面组件
	}
}

// SetInputCallback 设置输入回调函数
func (m *Model) SetInputCallback(callback func(string)) {
	m.inputCallback = callback
}

// ========== Bubble Tea 接口实现 ==========

// Init 实现 tea.Model 接口 - 初始化
func (m *Model) Init() tea.Cmd {
	// 启动定时器：UI 刷新、Spinner 动画、欢迎界面动画
	return tea.Batch(
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return TickMsg(t)
		}),
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return SpinnerTickMsg(t)
		}),
		m.welcome.StartTick(), // 【新增】启动欢迎界面动画
	)
}

// Update 实现 tea.Model 接口 - 处理消息更新
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// 【新增】欢迎界面状态处理
	if m.welcome != nil && m.welcome.IsVisible() {
		visible, cmd := m.welcome.Update(msg)
		if !visible {
			// 欢迎界面已关闭
			m.welcome.Hide()
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// 如果欢迎界面仍然可见，返回（不处理其他消息）
		if visible {
			return m, tea.Batch(cmds...)
		}
		// 欢迎界面已关闭，继续处理其他消息
	}

	// 【新增】Modal 状态下，拦截所有按键
	if m.modal != nil && m.modal.IsVisible() {
		return m.handleModalInput(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 【新增】让 viewport 处理滚动（Ctrl+↑/↓）
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

		// 处理其他按键
		model, cmd := m.handleKeyMsg(msg)
		cmds = append(cmds, cmd)
		return model, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		// 窗口尺寸变化
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case StreamMsg:
		// 【优化】流式内容立即显示（实现逐行显示）
		content := string(msg)

		// 如果内容包含换行符，立即刷新到显示
		if strings.Contains(content, "\n") {
			m.currentStream.WriteString(content)
			// 立即刷新所有内容
			streamContent := m.currentStream.String()
			streamContent = strings.TrimLeft(streamContent, "\n")
			streamContent = strings.TrimRight(streamContent, "\n")
			if streamContent != "" {
				m.messages = append(m.messages, streamContent)
				m.currentStream.Reset()
				m.scrollToBottom()
			}
		} else {
			// 没有换行符，追加到缓冲区
			m.currentStream.WriteString(content)
		}
		return m, nil

	case MarkdownMsg:
		// Markdown 内容：渲染后显示
		rendered, err := m.renderMarkdown(string(msg))
		if err != nil {
			// 如果渲染失败，直接显示原始内容
			content := string(msg)
			// 【修复】清理首尾空行
			content = strings.TrimLeft(content, "\n")
			content = strings.TrimRight(content, "\n")
			m.messages = append(m.messages, content)
		} else {
			// 【修复】清理渲染后的首尾空行
			rendered = strings.TrimLeft(rendered, "\n")
			rendered = strings.TrimRight(rendered, "\n")
			m.messages = append(m.messages, rendered)
		}
		m.scrollToBottom()
		return m, nil

	case StatusMsg:
		// 状态更新
		m.status = string(msg)
		return m, nil

	case ThinkingStartMsg:
		// 开始思考状态
		m.thinking = true
		return m, nil

	case ThinkingStopMsg:
		// 停止思考状态
		m.thinking = false
		return m, nil

	// ========== 新增：状态切换相关消息处理 ==========

	case StatusChangeMsg:
		// 处理状态切换
		return m.handleStatusChange(msg)

	case StatusProgressMsg:
		// 处理进度更新
		m.animatedStatus.progress = msg.Progress
		if msg.Detail != "" {
			m.animatedStatus.message = fmt.Sprintf("%s (%d%%)",
				m.animatedStatus.message, msg.Progress)
		}
		return m, nil

	case ResetStatusMsg:
		// 处理自动重置（成功/错误状态 2 秒后）
		return m.handleStatusReset()

	case ToggleDetailsMsg:
		// 切换详情显示
		m.animatedStatus.showDetails = !m.animatedStatus.showDetails
		return m, nil

	case ConfirmMsg:
		// 显示确认对话框
		m.confirming = true
		m.confirmAction = msg.Action
		m.confirmArgs = msg.Args
		m.confirmReply = msg.Reply
		m.confirmChoice = 0 // 默认选择"是"
		return m, nil

	case DiffConfirmMsg:
		// 显示 Diff 确认对话框
		m.diffConfirming = true
		m.diffConfirmPath = msg.Path
		m.diffConfirmText = msg.DiffText
		m.diffConfirmReply = msg.Reply
		m.diffConfirmChoice = 0
		return m, nil

	case ShowModalMsg:
		// 【新增】显示 Modal
		if m.modal != nil {
			m.modal.Show(msg.Type, msg.Title, msg.Content, msg.OnConfirm, msg.Reply)
		}
		return m, nil

	case TickMsg:
		// 定时刷新：如果有流式内容，刷新到消息列表
		if m.currentStream.Len() > 0 {
			content := m.currentStream.String()

			// 【修复】清理流式内容的首尾空行
			// 保留中间的换行，但移除首尾的多余空行
			content = strings.TrimLeft(content, "\n")
			content = strings.TrimRight(content, "\n")

			m.messages = append(m.messages, content)
			m.currentStream.Reset()
			// 自动滚动到底部
			m.scrollToBottom()
		}
		return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})

	case SpinnerTickMsg:
		// Spinner 动画更新
		var cmd tea.Cmd
		m.thinkingSpinner, cmd = m.thinkingSpinner.Update(msg)
		// 返回 spinner 的 tick 命令
		return m, tea.Batch(
			cmd,
			tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
				return SpinnerTickMsg(t)
			}),
		)
	}

	return m, nil
}

// View 实现 tea.Model 接口 - 渲染视图
func (m *Model) View() string {
	// 【新增】欢迎界面优先级最高（启动时显示）
	if m.welcome != nil && m.welcome.IsVisible() {
		return m.welcome.Render(m.width, m.height)
	}

	// 【新增】Modal 模式优先级次高
	if m.modal != nil && m.modal.IsVisible() {
		// 1. 渲染变暗的底层视图
		dimmedView := m.renderDimmedView()

		// 2. 渲染 Modal 框
		modalView := m.renderModal()

		// 3. 使用 lipgloss.Place 将 Modal 居中放置
		finalView := lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
			lipgloss.WithWhitespaceBackground(lipgloss.Color("#1a1b26")),
		)

		// 叠加底层视图（变暗）
		// 注意：由于终端限制，我们简单地返回组合视图
		// 在实际显示时，Modal 的 solid background 会遮挡底层
		return lipgloss.JoinVertical(lipgloss.Left, dimmedView, finalView)
	}

	// 如果在确认对话框模式，显示确认对话框（保留兼容）
	if m.confirming {
		return m.viewConfirmDialog()
	}

	// 如果在 Diff 确认对话框模式，显示 Diff 对话框（保留兼容）
	if m.diffConfirming {
		return m.viewDiffConfirmDialog()
	}

	// 【重写】正常模式：使用动态高度计算
	// 1. 先渲染底部组件（高度不固定）
	inputView := m.renderInputArea()
	statusBarView := m.renderAnimatedStatusBar()
	helpView := m.styles.App.Render(m.renderHelpText())

	// 2. 计算底部总高度（使用 lipgloss.Height）
	bottomHeight := lipgloss.Height(inputView) +
		lipgloss.Height(statusBarView) +
		lipgloss.Height(helpView)

	// 3. 动态调整 viewport 高度（剩余空间）
	availableHeight := m.height - bottomHeight
	if availableHeight < 5 { // 最小高度保护
		availableHeight = 5
	}
	m.viewport.Height = availableHeight
	m.viewport.Width = m.width

	// 4. 更新 viewport 内容
	m.viewport.SetContent(m.renderMessagesContent())

	// 5. 使用 lipgloss.JoinVertical 组装（稳健布局）
	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		inputView,
		statusBarView,
		helpView,
	)
}

// ========== 消息处理方法 ==========

// handleKeyMsg 处理键盘输入
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 如果在确认对话框模式
	if m.confirming {
		return m.handleConfirmKeyMsg(msg)
	}

	// 如果在 Diff 确认对话框模式
	if m.diffConfirming {
		return m.handleDiffConfirmKeyMsg(msg)
	}

	// 普通模式下的按键处理
	switch msg.String() {
	case "ctrl+c", "q":
		// 退出程序
		return m, tea.Quit

	case "ctrl+d", "tab":
		// 切换详情显示（D for Details, Tab 也直观）
		return m, func() tea.Msg {
			return ToggleDetailsMsg{}
		}

	case "enter":
		// 提交输入（如果输入框激活）
		if m.inputActive {
			input := m.textInput.Value()
			if strings.TrimSpace(input) != "" && m.inputCallback != nil {
				// 调用回调函数发送用户输入
				m.inputCallback(input)
				m.textInput.Reset()
			}
		}
		return m, nil

	case "ctrl+up", "ctrl+k":
		// 向上滚动
		m.scrollUp()
		return m, nil

	case "ctrl+down", "ctrl+j":
		// 向下滚动
		m.scrollDown()
		return m, nil

	case "esc":
		// 切换输入焦点
		if m.inputActive {
			m.inputActive = false
			m.textInput.Blur()
		} else {
			m.inputActive = true
			m.textInput.Focus()
		}
		return m, nil
	}

	// 如果输入框激活，让 textinput 处理按键
	if m.inputActive {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleConfirmKeyMsg 处理确认对话框的按键
func (m *Model) handleConfirmKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		// 选择"是"
		m.confirmChoice = 0
		return m, nil

	case "right", "l":
		// 选择"否"
		m.confirmChoice = 1
		return m, nil

	case "enter", " ":
		// 确认选择
		if m.confirmReply != nil {
			m.confirmReply <- m.confirmChoice == 0
		}
		m.confirming = false
		m.confirmReply = nil
		return m, nil

	case "ctrl+c", "q", "esc":
		// 取消（视为拒绝）
		if m.confirmReply != nil {
			m.confirmReply <- false
		}
		m.confirming = false
		m.confirmReply = nil
		return m, nil
	}

	return m, nil
}

// handleDiffConfirmKeyMsg 处理 Diff 确认对话框的按键
func (m *Model) handleDiffConfirmKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		// 选择"确认修改"
		m.diffConfirmChoice = 0
		return m, nil

	case "right", "l":
		// 选择"取消"
		m.diffConfirmChoice = 1
		return m, nil

	case "enter", " ":
		// 确认选择
		if m.diffConfirmReply != nil {
			m.diffConfirmReply <- m.diffConfirmChoice == 0
		}
		m.diffConfirming = false
		m.diffConfirmReply = nil
		return m, nil

	case "ctrl+c", "q", "esc":
		// 取消（视为拒绝）
		if m.diffConfirmReply != nil {
			m.diffConfirmReply <- false
		}
		m.diffConfirming = false
		m.diffConfirmReply = nil
		return m, nil

	case "up", "k":
		// 向上滚动 Diff 内容
		m.scrollUp()
		return m, nil

	case "down", "j":
		// 向下滚动 Diff 内容
		m.scrollDown()
		return m, nil
	}

	return m, nil
}

// ========== 视图渲染方法 ==========

// viewConfirmDialog 渲染确认对话框
func (m *Model) viewConfirmDialog() string {
	title := "⚠️  确认操作"
	content := fmt.Sprintf("工具: %s\n参数: %s", m.confirmAction, m.confirmArgs)

	yesStyle := m.styles.DialogOption
	noStyle := m.styles.DialogOption

	if m.confirmChoice == 0 {
		yesStyle = m.styles.DialogSelected // 选中"是"
	} else {
		noStyle = m.styles.DialogSelected // 选中"否"
	}

	dialog := m.styles.Dialog.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.styles.DialogTitle.Render(title),
			"",
			m.styles.DialogContent.Render(content),
			"",
			lipgloss.JoinHorizontal(lipgloss.Left,
				yesStyle.Render("› 是 (Y)"),
				noStyle.Render("› 否 (N)"),
			),
		),
	)

	// 居中显示
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

// viewDiffConfirmDialog 渲染 Diff 确认对话框
func (m *Model) viewDiffConfirmDialog() string {
	title := "📝 确认文件修改"
	pathInfo := fmt.Sprintf("文件: %s", m.diffConfirmPath)

	// 简化的 Diff 显示（仅显示前 20 行）
	diffLines := strings.Split(m.diffConfirmText, "\n")
	if len(diffLines) > 20 {
		diffLines = append(diffLines[:20], "... (更多内容未显示)")
	}
	diffContent := strings.Join(diffLines, "\n")

	yesStyle := m.styles.DialogOption
	noStyle := m.styles.DialogOption

	if m.diffConfirmChoice == 0 {
		yesStyle = m.styles.DialogSelected // 选中"确认修改"
	} else {
		noStyle = m.styles.DialogSelected // 选中"取消"
	}

	dialog := m.styles.Dialog.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.styles.DialogTitle.Render(title),
			"",
			m.styles.DialogContent.Render(pathInfo),
			"",
			m.styles.DialogContent.Render(diffContent),
			"",
			lipgloss.JoinHorizontal(lipgloss.Left,
				yesStyle.Render("› 确认修改 (Y)"),
				noStyle.Render("› 取消 (N)"),
			),
		),
	)

	// 居中显示
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

// getVisibleMessages 获取可见的消息（用于滚动）
func (m *Model) getVisibleMessages(maxHeight int) []string {
	if len(m.messages) == 0 {
		return []string{"\n\n等待输入...\n"}
	}

	// 显示所有消息（简化版本，后续可以添加精确的行数计算和滚动）
	result := make([]string, 0, len(m.messages))
	for _, msg := range m.messages {
		result = append(result, m.styles.Message.Render(msg))
	}

	return result
}

// ========== 渲染辅助方法 ==========

// renderMessagesContent 渲染所有消息为单个字符串（供 viewport 使用）
func (m *Model) renderMessagesContent() string {
	if len(m.messages) == 0 {
		// 如果没有消息，显示提示
		return m.styles.Message.Render("\n\n等待输入...\n")
	}

	var b strings.Builder

	// 计算可用宽度（viewport 宽度 - 左右边距）
	// viewport 默认有 Padding(0, 1)，所以减去 2
	availableWidth := m.width - 4
	if availableWidth < 20 {
		availableWidth = 20 // 最小宽度保护
	}

	for i, msg := range m.messages {
		// 【新增】使用 wordwrap.String 进行文本换行
		// 这样长文本会自动换行，不会超出视口宽度
		wrapped := wordwrap.String(msg, availableWidth)

		// 渲染消息
		rendered := m.styles.Message.Render(wrapped)

		// 【修复】清理消息开头和结尾的多余换行
		// 保留内容的换行，但移除首尾的空行
		rendered = strings.TrimLeft(rendered, "\n")
		rendered = strings.TrimRight(rendered, "\n")

		b.WriteString(rendered)

		// 【修复】消息之间用单个空行分隔（除了最后一个消息）
		if i < len(m.messages)-1 {
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

// renderInputArea 渲染输入区域
func (m *Model) renderInputArea() string {
	var input string
	if m.inputActive {
		input = ">> " + m.textInput.View()
	} else {
		input = ">> (按 ESC 激活输入)"
	}
	return m.styles.Message.Render(input)
}

// renderAnimatedStatusBar 渲染动画状态栏
func (m *Model) renderAnimatedStatusBar() string {
	status := m.animatedStatus
	var statusText strings.Builder

	// 根据状态决定渲染内容
	switch status.state {
	case StatusIdle:
		// 空闲状态：灰色文字 + 无动画
		statusText.WriteString("○ 准备就绪")

	case StatusThinking, StatusReading, StatusSearching,
		StatusExecuting, StatusStreaming:
		// 执行状态：颜色 spinner + 进度信息
		spinnerView := status.spinner.View()

		if status.progress > 0 {
			// 显示进度条
			statusText.WriteString(fmt.Sprintf("%s %s [%d%%]",
				spinnerView, status.message, status.progress))
		} else {
			// 无进度，仅显示 spinner 和消息
			statusText.WriteString(fmt.Sprintf("%s %s",
				spinnerView, status.message))
		}

		// 【详情显示】如果启用了详情显示
		if status.showDetails {
			details := m.getOperationDetails()
			if details != "" {
				statusText.WriteString("\n  └─ " + details)
			}
		}

	case StatusSuccess:
		// 成功状态：绿色 ✓ + 消息
		statusText.WriteString(fmt.Sprintf("✓ %s", status.message))

	case StatusError:
		// 错误状态：红色 ✗ + 错误信息
		statusText.WriteString(fmt.Sprintf("✗ %s", status.message))
	}

	// 应用颜色和样式
	coloredText := m.styles.App.
		Foreground(colorForState(status.state)).
		Render(statusText.String())

	return m.styles.StatusBar.
		Width(m.width).
		Render(coloredText)
}

// renderHelpText 渲染帮助文本
func (m *Model) renderHelpText() string {
	var parts []string

	parts = append(parts, "[Ctrl+↑/↓:滚动]")
	parts = append(parts, "[ESC:输入]")

	// 【新增】详情切换提示
	if m.animatedStatus.showDetails {
		parts = append(parts, "[Ctrl+D:隐藏详情]")
	} else {
		parts = append(parts, "[Ctrl+D/Tab:显示详情]")
	}

	parts = append(parts, "[Enter:发送]")
	parts = append(parts, "[Ctrl+C:退出]")

	return " " + strings.Join(parts, " ") + " "
}

// getOperationDetails 获取当前操作的详细信息
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
		return "读取中..."

	case StatusSearching:
		if pattern, ok := p["pattern"]; ok {
			return fmt.Sprintf("搜索: %s", pattern)
		}
		return "搜索中..."

	case StatusExecuting:
		if tool, ok := p["tool"]; ok {
			return fmt.Sprintf("工具: %s", tool)
		}
		return "执行中..."

	case StatusStreaming:
		if tokens, ok := p["tokens"]; ok {
			return fmt.Sprintf("已生成: %s tokens", tokens)
		}
		return "生成中..."

	default:
		return ""
	}
}

// ========== 滚动控制方法 ==========

// scrollUp 向上滚动一行
func (m *Model) scrollUp() {
	if m.scrollOffset > 0 {
		m.scrollOffset--
	}
}

// scrollDown 向下滚动一行
func (m *Model) scrollDown() {
	m.scrollOffset++
}

// scrollToTop 跳转到顶部
func (m *Model) scrollToTop() {
	m.scrollOffset = 0
}

// scrollToBottom 跳转到底部
func (m *Model) scrollToBottom() {
	m.scrollOffset = len(m.messages)
}

// ========== 辅助方法 ==========

// spinnerForState 根据状态返回对应的 spinner 类型
func spinnerForState(state StatusState) spinner.Spinner {
	switch state {
	case StatusThinking:
		return spinner.Dot
	case StatusReading:
		return spinner.Points
	case StatusSearching:
		return spinner.Line
	case StatusExecuting:
		return spinner.Jump
	case StatusStreaming:
		return spinner.MiniDot
	default:
		return spinner.Dot
	}
}

// colorForState 根据 Tokyo Night 主题返回状态对应的颜色
func colorForState(state StatusState) lipgloss.Color {
	colors := map[StatusState]lipgloss.Color{
		StatusIdle:      "#565f89", // 灰色（待机）
		StatusThinking:  "#7aa2f7", // 蓝色（思考）
		StatusReading:   "#2ac3de", // 青色（读取）
		StatusSearching: "#bb9af7", // 紫色（搜索）
		StatusExecuting: "#e0af68", // 橙色（执行）
		StatusStreaming: "#73daca", // 绿青色（流式）
		StatusSuccess:   "#9ece6a", // 绿色（成功）
		StatusError:     "#f7768e", // 红色（错误）
	}

	if c, ok := colors[state]; ok {
		return c
	}
	return "#565f89" // 默认灰色
}

// renderMarkdown 渲染 Markdown 内容
func (m *Model) renderMarkdown(markdown string) (string, error) {
	if m.markdownRenderer == nil {
		// 如果渲染器未初始化，返回原始内容
		return markdown, nil
	}

	// 使用 glamour 渲染 Markdown
	rendered, err := m.markdownRenderer.Render(markdown)
	if err != nil {
		return "", err
	}

	return rendered, nil
}

// ========== 状态处理方法 ==========

// handleStatusChange 处理状态切换
func (m *Model) handleStatusChange(msg StatusChangeMsg) (tea.Model, tea.Cmd) {
	// 更新状态
	m.animatedStatus.state = msg.State
	m.animatedStatus.message = msg.Message
	m.animatedStatus.progress = msg.Progress
	m.animatedStatus.payload = msg.Payload
	m.animatedStatus.timestamp = time.Now()

	// 重新创建 spinner（应用新类型和颜色）
	newSpinner := spinner.New()
	newSpinner.Spinner = spinnerForState(msg.State)
	newSpinner.Style = lipgloss.NewStyle().Foreground(colorForState(msg.State))
	m.animatedStatus.spinner = newSpinner

	// 启动定时器：临时状态（成功/错误）2 秒后自动重置
	if msg.State == StatusSuccess || msg.State == StatusError {
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return ResetStatusMsg{}
		})
	}

	// 其他状态：继续 spinner 动画
	return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return SpinnerTickMsg(t)
	})
}

// handleStatusReset 处理自动重置（用于定时器回调）
func (m *Model) handleStatusReset() (tea.Model, tea.Cmd) {
	// 防止覆盖新的操作状态
	if m.animatedStatus.state == StatusSuccess ||
		m.animatedStatus.state == StatusError {
		m.animatedStatus.state = StatusIdle
		m.animatedStatus.message = "准备就绪"
		m.animatedStatus.progress = 0
	}
	return m, nil
}

// ========== Modal Overlay 渲染方法 ==========

// renderDimmedView 渲染变暗的底层视图（Modal 模式）
func (m *Model) renderDimmedView() string {
	if m.modal == nil {
		return ""
	}

	dimStyle := m.modal.GetStyle().DimStyle

	// 渲染组件原始内容，然后应用 Dim 样式
	inputView := dimStyle.Render(m.renderInputAreaRaw())
	statusBarView := dimStyle.Render(m.renderAnimatedStatusBarRaw())
	helpView := dimStyle.Render(m.renderHelpTextRaw())

	// Viewport 内容也需要变暗
	m.viewport.SetContent(dimStyle.Render(m.renderMessagesContentRaw()))

	// 计算高度
	bottomHeight := lipgloss.Height(inputView) +
		lipgloss.Height(statusBarView) +
		lipgloss.Height(helpView)

	availableHeight := m.height - bottomHeight
	if availableHeight < 5 {
		availableHeight = 5
	}
	m.viewport.Height = availableHeight
	m.viewport.Width = m.width

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		inputView,
		statusBarView,
		helpView,
	)
}

// renderModal 渲染 Modal 框
func (m *Model) renderModal() string {
	if m.modal == nil {
		return ""
	}

	state := m.modal.GetState()
	style := m.modal.GetStyle()

	var b strings.Builder

	// 标题
	title := style.Title.Render(state.Title)
	b.WriteString(title)
	b.WriteString("\n")

	// 内容
	switch state.Type {
	case ModalConfirm:
		content := style.Content.Render(state.Content)
		b.WriteString(content)

	case ModalDiff:
		// Diff 内容，直接显示
		diff := style.Content.Render(state.Content)
		b.WriteString(diff)
	}

	b.WriteString("\n")

	// 提示
	hint := style.Content.Render(
		"[Enter: 确认] [Esc: 取消]",
	)
	b.WriteString(hint)

	// 应用边框和 Solid 背景
	return style.Border.Render(
		style.Background.Render(b.String()),
	)
}

// renderInputAreaRaw 返回未渲染的输入区域原始内容
func (m *Model) renderInputAreaRaw() string {
	if m.inputActive {
		return ">> " + m.textInput.Value()
	}
	return ">> (按 ESC 激活输入)"
}

// renderAnimatedStatusBarRaw 返回未渲染的状态栏原始内容
func (m *Model) renderAnimatedStatusBarRaw() string {
	status := m.animatedStatus
	var statusText strings.Builder

	switch status.state {
	case StatusIdle:
		statusText.WriteString("○ 准备就绪")
	case StatusThinking, StatusReading, StatusSearching, StatusExecuting, StatusStreaming:
		spinnerView := status.spinner.View()
		if status.progress > 0 {
			statusText.WriteString(fmt.Sprintf("%s %s [%d%%]",
				spinnerView, status.message, status.progress))
		} else {
			statusText.WriteString(fmt.Sprintf("%s %s",
				spinnerView, status.message))
		}
	case StatusSuccess:
		statusText.WriteString(fmt.Sprintf("✓ %s", status.message))
	case StatusError:
		statusText.WriteString(fmt.Sprintf("✗ %s", status.message))
	}

	return statusText.String()
}

// renderHelpTextRaw 返回未渲染的帮助文本原始内容
func (m *Model) renderHelpTextRaw() string {
	var parts []string
	parts = append(parts, "[Ctrl+↑/↓:滚动]")
	parts = append(parts, "[ESC:输入]")

	if m.animatedStatus.showDetails {
		parts = append(parts, "[Ctrl+D:隐藏详情]")
	} else {
		parts = append(parts, "[Ctrl+D/Tab:显示详情]")
	}

	parts = append(parts, "[Enter:发送]")
	parts = append(parts, "[Ctrl+C:退出]")

	return " " + strings.Join(parts, " ") + " "
}

// renderMessagesContentRaw 返回未渲染的消息内容
func (m *Model) renderMessagesContentRaw() string {
	if len(m.messages) == 0 {
		return "暂无消息..."
	}

	// 计算可用宽度
	availableWidth := m.width - 4
	if availableWidth < 20 {
		availableWidth = 20
	}

	var b strings.Builder
	for _, msg := range m.messages {
		// 【新增】使用 wordwrap.String 进行文本换行
		wrapped := wordwrap.String(msg, availableWidth)
		b.WriteString(wrapped)
		b.WriteString("\n\n")
	}

	// 添加当前流式内容
	if m.currentStream.Len() > 0 {
		streamContent := m.currentStream.String()
		streamContent = strings.TrimLeft(streamContent, "\n")
		streamContent = strings.TrimRight(streamContent, "\n")
		if streamContent != "" {
			// 【新增】流式内容也需要换行
			wrappedStream := wordwrap.String(streamContent, availableWidth)
			b.WriteString(wrappedStream)
		}
	}

	return b.String()
}

// handleModalInput 处理 Modal 状态下的输入（事件拦截）
func (m *Model) handleModalInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modal == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			// 确认
			state := m.modal.GetState()

			// 发送 true 到回复通道
			if state.Reply != nil {
				select {
				case state.Reply <- true:
					// 成功发送确认信号
				default:
					// 通道已关闭或已满，忽略
				}
				close(state.Reply)
			}

			// 关闭 Modal
			m.modal.Hide()
			return m, nil

		case "esc", "q":
			// 取消，关闭 Modal
			state := m.modal.GetState()

			// 发送 false 到回复通道
			if state.Reply != nil {
				select {
				case state.Reply <- false:
					// 成功发送取消信号
				default:
					// 通道已关闭或已满，忽略
				}
				close(state.Reply)
			}

			m.modal.Hide()
			return m, nil

		case "ctrl+c":
			// 强制退出程序
			return m, tea.Quit
		}
	}

	return m, nil
}
