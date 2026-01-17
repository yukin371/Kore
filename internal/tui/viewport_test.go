package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestNewViewportComponent 测试 Viewport 组件创建
func TestNewViewportComponent(t *testing.T) {
	vp := NewViewportComponent()

	if vp == nil {
		t.Fatal("NewViewportComponent returned nil")
	}

	if vp.width != 80 {
		t.Errorf("expected default width 80, got %d", vp.width)
	}

	if vp.height != 20 {
		t.Errorf("expected default height 20, got %d", vp.height)
	}

	if !vp.autoscroll {
		t.Error("expected autoscroll to be enabled by default")
	}

	if len(vp.messages) != 0 {
		t.Errorf("expected empty messages, got %d", len(vp.messages))
	}
}

// TestViewportComponent_SetSize 测试尺寸设置
func TestViewportComponent_SetSize(t *testing.T) {
	vp := NewViewportComponent()

	vp.SetSize(100, 30)

	if vp.width != 100 {
		t.Errorf("expected width 100, got %d", vp.width)
	}

	if vp.height != 30 {
		t.Errorf("expected height 30, got %d", vp.height)
	}
}

// TestViewportComponent_AddMessage 测试添加消息
func TestViewportComponent_AddMessage(t *testing.T) {
	vp := NewViewportComponent()

	vp.AddMessage("Hello, World!")
	vp.AddMessage("Second message")

	if len(vp.messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(vp.messages))
	}

	if vp.messages[0] != "Hello, World!" {
		t.Errorf("expected first message 'Hello, World!', got '%s'", vp.messages[0])
	}

	if vp.GetGeneratedCount() != 2 {
		t.Errorf("expected generated count 2, got %d", vp.GetGeneratedCount())
	}
}

// TestViewportComponent_SetMessages 测试设置所有消息
func TestViewportComponent_SetMessages(t *testing.T) {
	vp := NewViewportComponent()

	messages := []string{"Message 1", "Message 2", "Message 3"}
	vp.SetMessages(messages)

	if len(vp.messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(vp.messages))
	}

	if vp.GetGeneratedCount() != 3 {
		t.Errorf("expected generated count 3, got %d", vp.GetGeneratedCount())
	}
}

// TestViewportComponent_Clear 测试清空视口
func TestViewportComponent_Clear(t *testing.T) {
	vp := NewViewportComponent()

	vp.AddMessage("Test message")
	vp.Clear()

	if len(vp.messages) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(vp.messages))
	}

	if vp.GetGeneratedCount() != 0 {
		t.Errorf("expected generated count 0 after clear, got %d", vp.GetGeneratedCount())
	}
}

// TestViewportComponent_Scrolling 测试滚动功能
func TestViewportComponent_Scrolling(t *testing.T) {
	vp := NewViewportComponent()

	// 设置尺寸，确保视口有效
	vp.SetSize(80, 20)

	// 添加多条消息
	for i := 0; i < 10; i++ {
		vp.AddMessage("Message " + string(rune('0'+i)))
	}

	// 测试向下滚动
	vp.LineDown(1)
	// 测试向上滚动
	vp.LineUp(1)

	// 测试半页滚动
	vp.HalfViewDown()
	vp.HalfViewUp()

	// 测试整页滚动
	vp.ViewDown()
	vp.ViewUp()

	// 测试跳转到顶部和底部
	vp.GotoTop()
	if !vp.AtTop() {
		t.Error("expected to be at top after GotoTop()")
	}

	vp.GotoBottom()
	if !vp.AtBottom() {
		t.Error("expected to be at bottom after GotoBottom()")
	}
}

// TestViewportComponent_Autoscroll 测试自动滚动
func TestViewportComponent_Autoscroll(t *testing.T) {
	vp := NewViewportComponent()
	vp.SetSize(80, 20)

	// 启用自动滚动
	vp.SetAutoscroll(true)

	vp.AddMessage("First message")
	vp.AddMessage("Second message")

	// 自动滚动应该在添加消息后跳转到底部
	// 这个测试主要是确保不会 panic
	vp.GotoBottom()
}

// TestViewportComponent_ScrollPercentage 测试滚动百分比
func TestViewportComponent_ScrollPercentage(t *testing.T) {
	vp := NewViewportComponent()
	vp.SetSize(80, 20)

	// 添加一些消息
	for i := 0; i < 5; i++ {
		vp.AddMessage("Message " + string(rune('0'+i)))
	}

	// 获取滚动百分比（应该在 0-100 之间）
	percentage := vp.GetScrollPercentage()
	if percentage < 0 || percentage > 100 {
		t.Errorf("expected scroll percentage between 0 and 100, got %f", percentage)
	}
}

// TestViewportComponent_LineCount 测试行数计算
func TestViewportComponent_LineCount(t *testing.T) {
	vp := NewViewportComponent()
	vp.SetSize(80, 20)

	// 添加消息
	vp.AddMessage("Short message")
	vp.AddMessage("Another message")

	lineCount := vp.GetLineCount()
	if lineCount <= 0 {
		t.Errorf("expected positive line count, got %d", lineCount)
	}
}

// TestViewportComponent_Style 测试样式设置
func TestViewportComponent_Style(t *testing.T) {
	vp := NewViewportComponent()

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("red")).
		Background(lipgloss.Color("blue"))

	vp.SetStyle(style)

	// 验证样式已设置（主要是确保不会 panic）
	// lipgloss.Style 包含函数，不能直接比较，所以只测试不会 panic
	_ = vp.style
}

// TestViewportComponent_Update 测试消息更新
func TestViewportComponent_Update(t *testing.T) {
	vp := NewViewportComponent()

	// 测试键盘消息
	msg := tea.KeyMsg{Type: tea.KeyCtrlUp}
	newVp, cmd := vp.Update(msg)

	if newVp == nil {
		t.Error("Update returned nil viewport")
	}

	// cmd 可以为 nil 或非 nil，取决于消息是否产生副作用
	_ = cmd // 显式忽略 cmd 以避免 unused 警告
}

// TestViewportComponent_SyncSize 测试尺寸同步
func TestViewportComponent_SyncSize(t *testing.T) {
	vp := NewViewportComponent()

	// 模拟窗口尺寸
	width := 100
	height := 40
	bottomComponentsHeight := 10

	vp.SyncSize(width, height, bottomComponentsHeight)

	// 视口高度应该是总高度减去底部组件高度
	expectedHeight := height - bottomComponentsHeight
	if expectedHeight < 5 {
		expectedHeight = 5
	}

	if vp.height != expectedHeight {
		t.Errorf("expected height %d, got %d", expectedHeight, vp.height)
	}

	if vp.width != width {
		t.Errorf("expected width %d, got %d", width, vp.width)
	}
}

// TestViewportComponent_LongMessage 测试长消息处理
func TestViewportComponent_LongMessage(t *testing.T) {
	vp := NewViewportComponent()
	vp.SetSize(80, 20)

	// 添加超长消息
	longMessage := ""
	for i := 0; i < 1000; i++ {
		longMessage += "word "
	}

	vp.AddMessage(longMessage)

	// 确保消息被添加
	if len(vp.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(vp.messages))
	}

	// 换行处理应该产生了多行
	lineCount := vp.GetLineCount()
	if lineCount <= 1 {
		t.Errorf("expected multiple lines for long message, got %d", lineCount)
	}
}

// TestViewportComponent_MultilineMessage 测试多行消息处理
func TestViewportComponent_MultilineMessage(t *testing.T) {
	vp := NewViewportComponent()

	multilineMessage := "Line 1\nLine 2\nLine 3"
	vp.AddMessage(multilineMessage)

	if len(vp.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(vp.messages))
	}

	// 多行消息应该产生多行
	lineCount := vp.GetLineCount()
	if lineCount <= 1 {
		t.Errorf("expected multiple lines, got %d", lineCount)
	}
}

// TestViewportComponent_SetContent 测试直接设置内容
func TestViewportComponent_SetContent(t *testing.T) {
	vp := NewViewportComponent()

	content := "Direct content"
	vp.SetContent(content)

	// 直接设置内容不会影响消息列表
	if len(vp.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(vp.messages))
	}

	// 但会更新视口内容
	viewContent := vp.GetContent()
	if viewContent == "" {
		t.Error("expected non-empty content after SetContent")
	}
}

// TestViewportComponent_GetMessages 测试获取消息
func TestViewportComponent_GetMessages(t *testing.T) {
	vp := NewViewportComponent()

	vp.AddMessage("Message 1")
	vp.AddMessage("Message 2")

	messages := vp.GetMessages()

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	if messages[0] != "Message 1" {
		t.Errorf("expected first message 'Message 1', got '%s'", messages[0])
	}
}

// BenchmarkViewportComponent_AddMessage 性能测试：添加消息
func BenchmarkViewportComponent_AddMessage(b *testing.B) {
	vp := NewViewportComponent()

	for i := 0; i < b.N; i++ {
		vp.AddMessage("Benchmark message")
	}
}

// BenchmarkViewportComponent_UpdateContent 性能测试：更新内容
func BenchmarkViewportComponent_UpdateContent(b *testing.B) {
	vp := NewViewportComponent()
	vp.SetSize(80, 20)

	// 添加一些消息
	for i := 0; i < 100; i++ {
		vp.AddMessage("Message " + string(rune('0'+i%10)))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		vp.updateContent()
	}
}
