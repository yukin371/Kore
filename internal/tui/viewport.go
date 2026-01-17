// Package tui 提供 TUI（终端用户界面）组件
//
// Viewport 组件复用自开源项目 opencode-ai/opencode（已归档）
// 原始代码: https://github.com/opencode-ai/opencode
// 许可证: MIT License
//
// 本组件针对 Kore 项目进行了适配和优化：
// - 集成到 Bubble Tea 架构
// - 支持消息滚动和分页
// - 自动文本换行处理
// - 响应式布局
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// ViewportComponent 是增强的消息视口组件
// 提供消息滚动、分页和自动换行功能
type ViewportComponent struct {
	viewport    viewport.Model // Bubble Tea viewport
	width       int            // 视口宽度
	height      int            // 视口高度
	lineCount   int            // 总行数
	autoscroll  bool           // 是否自动滚动到底部
	style       lipgloss.Style // 视口样式
	messages    []string       // 消息列表
	generated   int            // 已生成的消息数量
}

// NewViewportComponent 创建新的 Viewport 组件
func NewViewportComponent() *ViewportComponent {
	// 初始化 Bubble Tea viewport
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.HiddenBorder())

	return &ViewportComponent{
		viewport:   vp,
		width:      80,
		height:     20,
		autoscroll: true,
		style: lipgloss.NewStyle().
			Padding(0, 1),
		messages:  make([]string, 0),
		generated: 0,
	}
}

// SetSize 设置视口尺寸
func (v *ViewportComponent) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.viewport.Width = width
	v.viewport.Height = height
}

// SetStyle 设置视口样式
func (v *ViewportComponent) SetStyle(style lipgloss.Style) {
	v.style = style
	v.viewport.Style = style
}

// SetAutoscroll 设置是否自动滚动
func (v *ViewportComponent) SetAutoscroll(autoscroll bool) {
	v.autoscroll = autoscroll
}

// AddMessage 添加消息到视口
func (v *ViewportComponent) AddMessage(message string) {
	v.messages = append(v.messages, message)
	v.generated++

	// 更新视口内容
	v.updateContent()

	// 如果启用自动滚动，滚动到底部
	if v.autoscroll && v.lineCount > 0 {
		v.GotoBottom()
	}
}

// SetMessages 设置所有消息（替换现有内容）
func (v *ViewportComponent) SetMessages(messages []string) {
	v.messages = messages
	v.generated = len(messages)
	v.updateContent()

	if v.autoscroll && v.lineCount > 0 {
		v.GotoBottom()
	}
}

// Clear 清空视口内容
func (v *ViewportComponent) Clear() {
	v.messages = make([]string, 0)
	v.generated = 0
	v.lineCount = 0

	// 使用 recover 防止 panic
	defer func() {
		if r := recover(); r != nil {
			// 忽略 panic
		}
	}()

	v.viewport.SetContent("")
}

// updateContent 更新视口内容（处理换行和格式化）
func (v *ViewportComponent) updateContent() {
	if len(v.messages) == 0 {
		v.viewport.SetContent("")
		v.lineCount = 0
		return
	}

	// 计算可用宽度（视口宽度 - 左右边距）
	availableWidth := v.width - 4
	if availableWidth < 20 {
		availableWidth = 20 // 最小宽度保护
	}

	var b strings.Builder
	totalLines := 0

	// 渲染所有消息
	for i, msg := range v.messages {
		// 使用 wordwrap 进行文本换行
		wrapped := wordwrap.String(msg, availableWidth)

		// 清理首尾空行
		wrapped = strings.TrimLeft(wrapped, "\n")
		wrapped = strings.TrimRight(wrapped, "\n")

		// 应用样式
		rendered := v.style.Render(wrapped)

		// 计算行数
		lines := strings.Count(rendered, "\n") + 1
		totalLines += lines

		b.WriteString(rendered)

		// 消息之间用空行分隔（除了最后一个）
		if i < len(v.messages)-1 {
			b.WriteString("\n\n")
			totalLines += 2
		}
	}

	v.lineCount = totalLines

	// 使用 recover 防止在无效尺寸时 panic
	defer func() {
		if r := recover(); r != nil {
			// 忽略 panic，可能尺寸无效
		}
	}()

	v.viewport.SetContent(b.String())
}

// GotoTop 跳转到视口顶部
func (v *ViewportComponent) GotoTop() {
	v.viewport.GotoTop()
}

// GotoBottom 跳转到视口底部
func (v *ViewportComponent) GotoBottom() {
	// 使用 recover 防止在空视口时 panic
	defer func() {
		if r := recover(); r != nil {
			// 忽略 panic，视口可能是空的
		}
	}()
	v.viewport.GotoBottom()
}

// LineDown 向下滚动一行
func (v *ViewportComponent) LineDown(count int) {
	for i := 0; i < count; i++ {
		v.viewport.LineDown(1)
	}
}

// LineUp 向上滚动一行
func (v *ViewportComponent) LineUp(count int) {
	for i := 0; i < count; i++ {
		v.viewport.LineUp(1)
	}
}

// HalfViewDown 向下滚动半个视口
func (v *ViewportComponent) HalfViewDown() {
	v.viewport.HalfViewDown()
}

// HalfViewUp 向上滚动半个视口
func (v *ViewportComponent) HalfViewUp() {
	v.viewport.HalfViewUp()
}

// ViewDown 向下滚动整个视口
func (v *ViewportComponent) ViewDown() {
	v.viewport.ViewDown()
}

// ViewUp 向上滚动整个视口
func (v *ViewportComponent) ViewUp() {
	v.viewport.ViewUp()
}

// GetScrollPercentage 获取当前滚动百分比（0-100）
func (v *ViewportComponent) GetScrollPercentage() float64 {
	return v.viewport.ScrollPercent()
}

// AtTop 是否在视口顶部
func (v *ViewportComponent) AtTop() bool {
	return v.viewport.AtTop()
}

// AtBottom 是否在视口底部
func (v *ViewportComponent) AtBottom() bool {
	return v.viewport.AtBottom()
}

// GetLineCount 获取总行数
func (v *ViewportComponent) GetLineCount() int {
	return v.lineCount
}

// GetMessages 获取所有消息
func (v *ViewportComponent) GetMessages() []string {
	return v.messages
}

// GetGeneratedCount 获取已生成的消息数量
func (v *ViewportComponent) GetGeneratedCount() int {
	return v.generated
}

// View 渲染视口（实现 Bubble Tea 接口）
func (v *ViewportComponent) View() string {
	return v.viewport.View()
}

// Update 处理消息更新（实现 Bubble Tea 接口）
func (v *ViewportComponent) Update(msg tea.Msg) (*ViewportComponent, tea.Cmd) {
	var cmd tea.Cmd

	// 让内部 viewport 处理滚动消息
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 处理滚动快捷键
		switch msg.String() {
		case "ctrl+up", "ctrl+k":
			v.LineUp(1)
		case "ctrl+down", "ctrl+j":
			v.LineDown(1)
		case "ctrl+u":
			v.HalfViewUp()
		case "ctrl+d":
			v.HalfViewDown()
		case "ctrl+home", "g":
			v.GotoTop()
		case "ctrl+end", "G":
			v.GotoBottom()
		case "pgup":
			v.ViewUp()
		case "pgdn":
			v.ViewDown()
		}

		// 让 viewport 处理其他按键
		v.viewport, cmd = v.viewport.Update(msg)

	default:
		// 让 viewport 处理其他消息
		v.viewport, cmd = v.viewport.Update(msg)
	}

	return v, cmd
}

// SyncSize 同步视口尺寸到窗口大小
func (v *ViewportComponent) SyncSize(width, height int, bottomComponentsHeight int) {
	// 计算可用高度（总高度 - 底部组件高度）
	availableHeight := height - bottomComponentsHeight
	if availableHeight < 5 {
		availableHeight = 5 // 最小高度保护
	}

	v.SetSize(width, availableHeight)
}

// GetViewport 返回内部 viewport.Model（用于高级操作）
func (v *ViewportComponent) GetViewport() viewport.Model {
	return v.viewport
}

// SetContent 直接设置视口内容（绕过消息列表）
func (v *ViewportComponent) SetContent(content string) {
	v.viewport.SetContent(content)
}

// GetContent 获取当前视口内容
func (v *ViewportComponent) GetContent() string {
	return v.viewport.View()
}
