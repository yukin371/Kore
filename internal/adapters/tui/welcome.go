package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// WelcomeMsg 欢迎界面完成消息
type WelcomeMsg struct{}

// WelcomeTickMsg 欢迎界面定时器消息
type WelcomeTickMsg time.Time

// WelcomeComponent 欢迎界面组件
type WelcomeComponent struct {
	visible     bool
	alpha       float64  // 透明度 0.0 - 1.0
	showTime    time.Time
	logo        []string
	subtitle    string
	tips        []string
	currentTip  int
	spinner     spinner.Model
	styles      WelcomeStyle
}

// WelcomeStyle 欢迎界面样式
type WelcomeStyle struct {
	Container   lipgloss.Style
	Title       lipgloss.Style
	Logo        lipgloss.Style
	Subtitle    lipgloss.Style
	Tip         lipgloss.Style
	Version     lipgloss.Style
	KeyHint     lipgloss.Style
	Progress    lipgloss.Style
}

// NewWelcomeComponent 创建欢迎界面组件
func NewWelcomeComponent() *WelcomeComponent {
	return &WelcomeComponent{
		visible:  true,
		alpha:    0.0,
		showTime: time.Now(),
		logo:     getKoreLogo(),
		subtitle: "AI-Powered Development Assistant",
		tips: []string{
			"输入你的问题或任务，我会帮你完成",
			"我可以读取文件、搜索代码、执行命令",
			"使用 Ctrl+↑/↓ 滚动查看历史消息",
			"按 Ctrl+C 退出程序",
		},
		currentTip: 0,
		spinner:    spinner.New(spinner.WithSpinner(spinner.Dot)),
		styles:     DefaultWelcomeStyle(),
	}
}

// DefaultWelcomeStyle 创建默认欢迎界面样式（Tokyo Night 主题）
func DefaultWelcomeStyle() WelcomeStyle {
	// 容器样式
	container := lipgloss.NewStyle().
		Padding(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")).
		Background(lipgloss.Color("#1a1b26")).
		Foreground(lipgloss.Color("#c0caf5"))

	// 标题样式
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7aa2f7")).
		MarginBottom(1)

	// Logo 样式（渐变效果）
	logo := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#bb9af7"))

	// 副标题样式
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9aa5ce")).
		MarginBottom(2)

	// 提示文本样式
	tip := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		MarginBottom(1).
		Italic(true)

	// 版本信息样式
	version := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#414868")).
		MarginTop(2)

	// 快捷键提示样式
	keyHint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#73daca")).
		MarginTop(1)

	// 进度条样式
	progress := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7"))

	return WelcomeStyle{
		Container:   container,
		Title:       title,
		Logo:        logo,
		Subtitle:    subtitle,
		Tip:         tip,
		Version:     version,
		KeyHint:     keyHint,
		Progress:    progress,
	}
}

// getKoreLogo 返回 Kore 的 ASCII 艺术字
func getKoreLogo() []string {
	return []string{
		"  __  __          _____   _____ \n",
		" |  \\/  |   /\\   |  __ \\ / ____|\n",
		" | \\  / |  /  \\  | |__) | |     \n",
		" | |\\/| | / /\\ \\ |  _  /| |     \n",
		" | |  | |/ ____ \\| | \\ \\| |____ \n",
		" |_|  |_/_/    \\_\\_|  \\_\\\\_____|",
	}
}

// Update 更新欢迎界面状态
func (w *WelcomeComponent) Update(msg tea.Msg) (bool, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case WelcomeTickMsg:
		// 更新透明度（淡入效果）
		if w.alpha < 1.0 {
			w.alpha += 0.1
			if w.alpha > 1.0 {
				w.alpha = 1.0
			}
			cmds = append(cmds, w.tickCmd())
		}

		// 更新 spinner
		var cmd tea.Cmd
		w.spinner, cmd = w.spinner.Update(msg)
		cmds = append(cmds, cmd)

		return true, tea.Batch(cmds...)

	case tea.KeyMsg:
		// 任意键关闭欢迎界面
		w.visible = false
		return false, func() tea.Msg { return WelcomeMsg{} }
	}

	return true, nil
}

// tickCmd 创建定时器命令
func (w *WelcomeComponent) tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return WelcomeTickMsg(t)
	})
}

// Render 渲染欢迎界面
func (w *WelcomeComponent) Render(width, height int) string {
	// 构建 Logo 部分
	logoBuilder := strings.Builder{}
	for _, line := range w.logo {
		logoBuilder.WriteString(line)
	}
	logoText := w.styles.Logo.Render(logoBuilder.String())

	// 构建标题
	titleText := w.styles.Title.Render("Kore")

	// 构建副标题
	subtitleText := w.styles.Subtitle.Render(w.subtitle)

	// 构建提示
	tipText := w.styles.Tip.Render("💡 " + w.tips[w.currentTip])

	// 构建版本信息
	versionText := w.styles.Version.Render("v1.0.0")

	// 构建快捷键提示
	keyHintText := w.styles.KeyHint.Render("[按任意键开始]")

	// 构建进度条
	progressWidth := 40
	currentWidth := int(float64(progressWidth) * w.alpha)
	progressBar := strings.Repeat("█", currentWidth) + strings.Repeat("░", progressWidth-currentWidth)
	progressText := w.styles.Progress.Render(progressBar)

	// 组装内容
	contentBuilder := strings.Builder{}
	contentBuilder.WriteString(titleText + "\n")
	contentBuilder.WriteString(logoText + "\n")
	contentBuilder.WriteString(subtitleText + "\n")
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString(tipText + "\n")
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString(versionText + "\n")
	contentBuilder.WriteString(keyHintText + "\n")
	contentBuilder.WriteString("\n")
	contentBuilder.WriteString(progressText)

	// 应用容器样式并居中
	content := w.styles.Container.Render(contentBuilder.String())

	// 使用 lipgloss.Place 居中显示
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		content,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#1a1b26")),
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#1a1b26")),
	)
}

// IsVisible 欢迎界面是否可见
func (w *WelcomeComponent) IsVisible() bool {
	return w.visible
}

// Hide 隐藏欢迎界面
func (w *WelcomeComponent) Hide() {
	w.visible = false
}

// StartTick 启动定时器
func (w *WelcomeComponent) StartTick() tea.Cmd {
	return w.tickCmd()
}

// GetWelcomeWidth 获取欢迎界面所需宽度
func (w *WelcomeComponent) GetWelcomeWidth() int {
	return 60
}

// GetWelcomeHeight 获取欢迎界面所需高度
func (w *WelcomeComponent) GetWelcomeHeight() int {
	return 25
}
