package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAGENTSMDLoader_LoadFromDirectory(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试用的 AGENTS.md 文件
	agentsMDContent := "# Test Project\n\nThis is a test AGENTS.md file."
	agentsMDPath := filepath.Join(tempDir, "AGENTS.md")
	err := os.WriteFile(agentsMDPath, []byte(agentsMDContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test AGENTS.md: %v", err)
	}

	// 创建加载器
	loader := NewAGENTSMDLoader(tempDir)

	// 加载 AGENTS.md
	agentsMDs, err := loader.LoadFromDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to load AGENTS.md: %v", err)
	}

	// 验证加载了 1 个文件
	if len(agentsMDs) != 1 {
		t.Errorf("expected 1 AGENTS.md file, got %d", len(agentsMDs))
	}

	// 验证内容
	if agentsMDs[0].Content != agentsMDContent {
		t.Errorf("content mismatch:\ngot: %s\nwant: %s", agentsMDs[0].Content, agentsMDContent)
	}

	// 验证优先级
	if agentsMDs[0].Priority != 0 {
		t.Errorf("expected priority 0, got %d", agentsMDs[0].Priority)
	}
}

func TestAGENTSMDLoader_GenerateContext(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试用的 AGENTS.md 文件
	agentsMDContent := "# Test Project\n\nThis is a test."
	agentsMDPath := filepath.Join(tempDir, "AGENTS.md")
	err := os.WriteFile(agentsMDPath, []byte(agentsMDContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test AGENTS.md: %v", err)
	}

	// 创建加载器
	loader := NewAGENTSMDLoader(tempDir)

	// 生成上下文
	context, err := loader.GenerateContext(tempDir)
	if err != nil {
		t.Fatalf("failed to generate context: %v", err)
	}

	// 验证上下文包含预期内容
	if context == "" {
		t.Error("expected non-empty context")
	}

	// 验证包含 AGENTS.md 标题
	if !contains(context, "## 📋 AGENTS.md 上下文") {
		t.Error("expected AGENTS.md context header")
	}

	// 验证包含文件内容
	if !contains(context, agentsMDContent) {
		t.Errorf("expected context to contain AGENTS.md content")
	}
}

func TestAGENTSMDLoader_Cache(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试用的 AGENTS.md 文件
	agentsMDContent := "# Test\nContent"
	agentsMDPath := filepath.Join(tempDir, "AGENTS.md")
	err := os.WriteFile(agentsMDPath, []byte(agentsMDContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test AGENTS.md: %v", err)
	}

	// 创建加载器
	loader := NewAGENTSMDLoader(tempDir)

	// 第一次加载
	agentsMDs1, err := loader.LoadFromDirectory(tempDir)
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}

	// 验证第一次加载的结果
	if len(agentsMDs1) != 1 {
		t.Fatalf("expected 1 AGENTS.md on first load, got %d", len(agentsMDs1))
	}

	// 获取缓存数量
	cachedCount := loader.GetCachedCount()
	if cachedCount != 1 {
		t.Errorf("expected 1 cached item, got %d", cachedCount)
	}

	// 验证缓存的时间戳在合理范围内
	if time.Since(agentsMDs1[0].LoadedAt) > time.Second {
		t.Error("cached timestamp seems too old")
	}
}

func TestAGENTSMDLoader_EnableDisable(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建测试用的 AGENTS.md 文件
	agentsMDPath := filepath.Join(tempDir, "AGENTS.md")
	err := os.WriteFile(agentsMDPath, []byte("# Test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test AGENTS.md: %v", err)
	}

	// 创建加载器
	loader := NewAGENTSMDLoader(tempDir)

	// 禁用
	loader.Disable()
	if loader.IsEnabled() {
		t.Error("expected loader to be disabled")
	}

	// 加载应该返回空
	agentsMDs, err := loader.LoadFromDirectory(tempDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(agentsMDs) != 0 {
		t.Errorf("expected 0 files when disabled, got %d", len(agentsMDs))
	}

	// 启用
	loader.Enable()
	if !loader.IsEnabled() {
		t.Error("expected loader to be enabled")
	}

	// 加载应该返回文件
	agentsMDs, err = loader.LoadFromDirectory(tempDir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(agentsMDs) != 1 {
		t.Errorf("expected 1 file when enabled, got %d", len(agentsMDs))
	}
}

func TestAGENTSMDLoader_Validate(t *testing.T) {
	loader := NewAGENTSMDLoader("")

	// 测试空内容
	warnings := loader.Validate("")
	if len(warnings) == 0 {
		t.Error("expected warning for empty content")
	}

	// 测试没有标题
	warnings = loader.Validate("Some content without title")
	if len(warnings) == 0 {
		t.Error("expected warning for missing title")
	}

	// 测试大文件
	largeContent := string(make([]byte, 11000))
	warnings = loader.Validate(largeContent)
	if len(warnings) == 0 {
		t.Error("expected warning for large file")
	}

	// 测试正常内容
	warnings = loader.Validate("# Good Title\n\nGood content.")
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for good content, got %d", len(warnings))
	}
}

func TestAGENTSMDLoader_FindAllAgentsMD(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建多个 AGENTS.md 文件
	dirs := []string{"", "internal", "internal/agent", "pkg"}
	for _, dir := range dirs {
		fullPath := filepath.Join(tempDir, dir)
		if dir != "" {
			err := os.MkdirAll(fullPath, 0755)
			if err != nil {
				t.Fatalf("failed to create directory: %v", err)
			}
		}
		agentsMDPath := filepath.Join(fullPath, "AGENTS.md")
		err := os.WriteFile(agentsMDPath, []byte("# "+dir), 0644)
		if err != nil {
			t.Fatalf("failed to create AGENTS.md: %v", err)
		}
	}

	// 创建加载器
	loader := NewAGENTSMDLoader(tempDir)

	// 查找所有 AGENTS.md
	agentsMDPaths, err := loader.FindAllAgentsMD(tempDir)
	if err != nil {
		t.Fatalf("failed to find AGENTS.md files: %v", err)
	}

	// 验证找到所有文件
	if len(agentsMDPaths) != len(dirs) {
		t.Errorf("expected %d AGENTS.md files, got %d", len(dirs), len(agentsMDPaths))
	}
}

func TestCachedAgentsMD(t *testing.T) {
	cached := &CachedAgentsMD{
		Path:     "/test/AGENTS.md",
		Content:  "# Test",
		LoadedAt: time.Now(),
		Priority: 5,
	}

	if cached.Path != "/test/AGENTS.md" {
		t.Errorf("Path mismatch: got %s", cached.Path)
	}

	if cached.Content != "# Test" {
		t.Errorf("Content mismatch: got %s", cached.Content)
	}

	if cached.Priority != 5 {
		t.Errorf("Priority mismatch: got %d", cached.Priority)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
