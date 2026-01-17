package tui

import (
	lipgloss "github.com/charmbracelet/lipgloss"
)

// ModalType Modal 类型
type ModalType int

const (
	ModalConfirm ModalType = iota // 确认对话框
	ModalDiff                     // Diff 预览对话框
)

// ModalState Modal 状态
type ModalState struct {
	Type      ModalType
	Title     string
	Content   string
	OnConfirm func() bool
	Reply     chan bool // 用户选择的回复通道
	Visible   bool
}

// ModalStyle Modal 样式
type ModalStyle struct {
	Border     lipgloss.Style
	Background lipgloss.Style
	Title      lipgloss.Style
	Content    lipgloss.Style
	DimStyle   lipgloss.Style // 底层变暗样式
}

// ModalComponent Modal 组件
type ModalComponent struct {
	state ModalState
	style ModalStyle
}

// NewModalComponent 创建新的 Modal 组件
func NewModalComponent() *ModalComponent {
	return &ModalComponent{
		state: ModalState{Visible: false},
		style: DefaultModalStyle(),
	}
}

// DefaultModalStyle 创建默认 Modal 样式（Tokyo Night 主题）
func DefaultModalStyle() ModalStyle {
	// Modal 边框
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7aa2f7")). // Tokyo Night 蓝色
		Padding(1, 2)

	// Modal 背景（Solid Color，不透明）
	background := lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1b26")). // Tokyo Night 深色
		Foreground(lipgloss.Color("#c0caf5"))

	// 标题样式
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7aa2f7")).
		MarginBottom(1)

	// 内容样式
	content := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c0caf5"))

	// 底层变暗样式（模拟 50% 透明度）
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")). // 灰色
		Background(lipgloss.Color("#1a1b26"))

	return ModalStyle{
		Border:     border,
		Background: background,
		Title:      title,
		Content:    content,
		DimStyle:   dimStyle,
	}
}

// Show 显示 Modal
func (m *ModalComponent) Show(modalType ModalType, title, content string, onConfirm func() bool, reply chan bool) {
	m.state = ModalState{
		Type:      modalType,
		Title:     title,
		Content:   content,
		OnConfirm: onConfirm,
		Reply:     reply,
		Visible:   true,
	}
}

// Hide 隐藏 Modal
func (m *ModalComponent) Hide() {
	m.state.Visible = false
}

// IsVisible Modal 是否可见
func (m *ModalComponent) IsVisible() bool {
	return m.state.Visible
}

// GetState 获取 Modal 状态
func (m *ModalComponent) GetState() ModalState {
	return m.state
}

// GetStyle 获取 Modal 样式
func (m *ModalComponent) GetStyle() ModalStyle {
	return m.style
}
