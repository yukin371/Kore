package skills

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestSkillManifest 测试清单验证
func TestSkillManifest(t *testing.T) {
	tests := []struct {
		name     string
		manifest *SkillManifest
		wantErr  bool
	}{
		{
			name: "valid builtin skill",
			manifest: &SkillManifest{
				ID:          "test-skill",
				Name:        "Test Skill",
				Version:     "1.0.0",
				Type:        SkillTypeBuiltin,
				Description: "A test skill",
				Author:      "Test Author",
				License:     "MIT",
				Permissions: []Permission{
					{Type: PermissionFilesystem, Resource: "/tmp", Action: "read"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			manifest: &SkillManifest{
				Name:    "Test Skill",
				Version: "1.0.0",
				Type:    SkillTypeBuiltin,
			},
			wantErr: true,
		},
		{
			name: "missing version",
			manifest: &SkillManifest{
				ID:   "test-skill",
				Name: "Test Skill",
				Type: SkillTypeBuiltin,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Registry{}
			err := r.validateManifest(tt.manifest)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestRegistry 测试注册表
func TestRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	config := &RegistryConfig{
		DataDir:   tmpDir,
		AutoLoad:  false,
		MaxSkills: 10,
	}

	registry, err := NewRegistry(config)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	ctx := context.Background()

	// 创建测试清单
	manifest := &SkillManifest{
		ID:          "test-skill",
		Name:        "Test Skill",
		Version:     "1.0.0",
		Type:        SkillTypeBuiltin,
		Description: "A test skill",
		Author:      "Test Author",
		License:     "MIT",
		Permissions: []Permission{
			{Type: PermissionFilesystem, Resource: "/tmp", Action: "read"},
		},
	}

	// 测试注册
	t.Run("Register", func(t *testing.T) {
		if err := registry.Register(ctx, manifest); err != nil {
			t.Fatalf("Failed to register skill: %v", err)
		}

		// 验证已注册
		retrieved, err := registry.Get("test-skill")
		if err != nil {
			t.Fatalf("Failed to get skill: %v", err)
		}

		if retrieved.Name != manifest.Name {
			t.Errorf("Expected name %s, got %s", manifest.Name, retrieved.Name)
		}
	})

	// 测试列出
	t.Run("List", func(t *testing.T) {
		skills := registry.List()
		if len(skills) != 1 {
			t.Errorf("Expected 1 skill, got %d", len(skills))
		}
	})

	// 测试启用/禁用
	t.Run("EnableDisable", func(t *testing.T) {
		if err := registry.Disable(ctx, "test-skill"); err != nil {
			t.Fatalf("Failed to disable skill: %v", err)
		}

		skills := registry.ListByState(StateDisabled)
		if len(skills) != 1 {
			t.Errorf("Expected 1 disabled skill, got %d", len(skills))
		}

		if err := registry.Enable(ctx, "test-skill"); err != nil {
			t.Fatalf("Failed to enable skill: %v", err)
		}
	})

	// 测试注销
	t.Run("Unregister", func(t *testing.T) {
		if err := registry.Unregister(ctx, "test-skill"); err != nil {
			t.Fatalf("Failed to unregister skill: %v", err)
		}

		_, err := registry.Get("test-skill")
		if err == nil {
			t.Error("Expected error after unregister, got nil")
		}
	})
}

// TestPolicyEngine 测试权限引擎
func TestPolicyEngine(t *testing.T) {
	audit := NewAuditLogger(nil)
	engine := NewPolicyEngine(audit)

	// 设置策略
	engine.SetPolicy("test-skill", []PermissionPolicy{
		{
			Type:     PermissionFilesystem,
			Resource: "/tmp/*",
			Action:   "read",
			Allow:    true,
		},
		{
			Type:     PermissionCommand,
			Resource: "ls",
			Action:   "execute",
			Allow:    true,
		},
	})

	ctx := context.Background()

	// 创建测试清单
	manifest := &SkillManifest{
		ID:      "test-skill",
		Name:    "Test Skill",
		Version: "1.0.0",
		Type:    SkillTypeBuiltin,
		Permissions: []Permission{
			{Type: PermissionFilesystem, Resource: "/tmp", Action: "read"},
		},
	}

	t.Run("AllowRead", func(t *testing.T) {
		input := map[string]interface{}{
			"path": "/tmp/test.txt",
		}
		err := engine.Check(ctx, manifest, "read_file", input)
		if err != nil {
			t.Errorf("Expected allow, got error: %v", err)
		}
	})

	t.Run("DenyWrite", func(t *testing.T) {
		input := map[string]interface{}{
			"path": "/tmp/test.txt",
		}
		err := engine.Check(ctx, manifest, "write_file", input)
		if err == nil {
			t.Error("Expected deny, got nil error")
		}
	})
}

// TestAuditLogger 测试审计日志
func TestAuditLogger(t *testing.T) {
	config := &AuditConfig{
		MaxEvents: 100,
		EnableLog: false,
	}
	logger := NewAuditLogger(config)

	// 记录一些事件
	logger.Log(AuditEvent{
		Type:      AuditTypeSkillLoaded,
		SkillID:   "test-skill",
		Timestamp: time.Now(),
		Success:   true,
	})

	logger.Log(AuditEvent{
		Type:      AuditTypeToolExecuted,
		SkillID:   "test-skill",
		Tool:      "test_tool",
		Timestamp: time.Now(),
		Success:   true,
	})

	// 获取事件
	t.Run("GetAllEvents", func(t *testing.T) {
		events := logger.GetEvents("", 0)
		if len(events) != 2 {
			t.Errorf("Expected 2 events, got %d", len(events))
		}
	})

	t.Run("GetSkillEvents", func(t *testing.T) {
		events := logger.GetEvents("test-skill", 0)
		if len(events) != 2 {
			t.Errorf("Expected 2 events for test-skill, got %d", len(events))
		}
	})

	t.Run("GetLimitedEvents", func(t *testing.T) {
		events := logger.GetEvents("", 1)
		if len(events) != 1 {
			t.Errorf("Expected 1 event with limit, got %d", len(events))
		}
	})
}

// TestMarketplace 测试市场
func TestMarketplace(t *testing.T) {
	tmpDir := t.TempDir()

	config := &MarketConfig{
		CacheDir:    tmpDir,
		CacheTTL:    time.Hour,
		AutoRefresh: false,
	}

	registryConfig := &RegistryConfig{DataDir: tmpDir}
	registry, _ := NewRegistry(registryConfig)

	installerConfig := &InstallerConfig{
		InstallDir: filepath.Join(tmpDir, "installed"),
		TempDir:    tmpDir,
	}
	runtime := NewRuntime(&RuntimeConfig{Registry: registry})
	installer := NewInstaller(registry, runtime, installerConfig)

	market, err := NewMarketplace(config, registry, installer)
	if err != nil {
		t.Fatalf("Failed to create marketplace: %v", err)
	}

	// 测试空搜索
	t.Run("SearchEmpty", func(t *testing.T) {
		results := market.Search("")
		if results != nil {
			t.Logf("Search returned %d results", len(results))
		}
	})

	// 测试列表
	t.Run("List", func(t *testing.T) {
		entries := market.List()
		if entries != nil {
			t.Logf("List returned %d entries", len(entries))
		}
	})
}

// BenchmarkPolicyCheck 基准测试权限检查
func BenchmarkPolicyCheck(b *testing.B) {
	audit := NewAuditLogger(nil)
	engine := NewPolicyEngine(audit)

	engine.SetPolicy("test-skill", []PermissionPolicy{
		{Type: PermissionFilesystem, Resource: "/tmp/*", Action: "read", Allow: true},
	})

	manifest := &SkillManifest{
		ID:      "test-skill",
		Name:    "Test Skill",
		Version: "1.0.0",
		Type:    SkillTypeBuiltin,
		Permissions: []Permission{
			{Type: PermissionFilesystem, Resource: "/tmp", Action: "read"},
		},
	}

	ctx := context.Background()
	input := map[string]interface{}{"path": "/tmp/test.txt"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Check(ctx, manifest, "read_file", input)
	}
}
