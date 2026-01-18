package lsp

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yukin371/Kore/pkg/logger"
)

// TestManagerNew tests creating a new manager
func TestManagerNew(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.INFO, "")

	config := &ManagerConfig{
		RootPath:             ".",
		HealthCheckInterval:  5 * time.Second,
		AutoRestart:          true,
		ServerConfigs:        DefaultServerConfigs,
	}

	manager := NewManager(config, log)
	if manager == nil {
		t.Fatal("Failed to create manager")
	}

	if manager.config != config {
		t.Error("Manager config not set correctly")
	}
}

// TestManagerSingleton tests the singleton pattern
func TestManagerSingleton(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.INFO, "")

	config := &ManagerConfig{
		RootPath: ".",
	}

	manager1 := GetManager(config, log)
	manager2 := GetManager(nil, log)

	if manager1 != manager2 {
		t.Error("GetManager should return the same singleton instance")
	}
}

// TestManagerStartStop tests starting and stopping the manager
func TestManagerStartStop(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.INFO, "")

	config := &ManagerConfig{
		RootPath:            ".",
		HealthCheckInterval: 1 * time.Second,
		AutoRestart:         false,
	}

	manager := NewManager(config, log)
	ctx := context.Background()

	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	// Try to start again
	err = manager.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running manager")
	}

	err = manager.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	// Try to stop again
	err = manager.Stop(ctx)
	if err != nil {
		t.Errorf("Expected no error when stopping already stopped manager: %v", err)
	}
}

// TestManagerRegisterServer tests registering custom server configurations
func TestManagerRegisterServer(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.INFO, "")

	manager := NewManager(&ManagerConfig{
		RootPath: ".",
	}, log)

	customConfig := ServerConfig{
		Command:  "my-custom-lsp",
		Args:     []string{"--stdio"},
		Enabled:  true,
		Priority: 50,
		EnvVars: map[string]string{
			"MY_VAR": "value",
		},
	}

	manager.RegisterServer("mylang", customConfig)

	retrievedConfig, exists := manager.serverRegistry["mylang"]
	if !exists {
		t.Fatal("Server not registered")
	}

	if retrievedConfig.Command != customConfig.Command {
		t.Errorf("Expected command %q, got %q", customConfig.Command, retrievedConfig.Command)
	}

	if retrievedConfig.Priority != customConfig.Priority {
		t.Errorf("Expected priority %d, got %d", customConfig.Priority, retrievedConfig.Priority)
	}
}

// TestManagerUnregisterServer tests unregistering server configurations
func TestManagerUnregisterServer(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.INFO, "")

	manager := NewManager(&ManagerConfig{
		RootPath: ".",
		ServerConfigs: map[string]ServerConfig{
			"testlang": {
				Command: "test-lsp",
				Args:    []string{},
				Enabled: true,
			},
		},
	}, log)

	err := manager.UnregisterServer("testlang")
	if err != nil {
		t.Fatalf("Failed to unregister server: %v", err)
	}

	_, exists := manager.serverRegistry["testlang"]
	if exists {
		t.Error("Server should be unregistered")
	}

	err = manager.UnregisterServer("nonexistent")
	if err == nil {
		t.Error("Expected error when unregistering non-existent server")
	}
}

// TestManagerGetStatus tests getting server status
func TestManagerGetStatus(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.INFO, "")

	manager := NewManager(&ManagerConfig{
		RootPath: ".",
		ServerConfigs: map[string]ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{},
				Enabled: true,
			},
			"python": {
				Command: "pyright-langserver",
				Args:    []string{},
				Enabled: true,
			},
		},
	}, log)

	statuses := manager.GetStatus()

	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(statuses))
	}

	for _, status := range statuses {
		if status.Running {
			t.Errorf("Server %s should not be running yet", status.Language)
		}
	}
}

// TestManagerNonExistentLanguage tests requesting a non-existent language
func TestManagerNonExistentLanguage(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.INFO, "")

	manager := NewManager(&ManagerConfig{
		RootPath: ".",
	}, log)

	ctx := context.Background()

	_, err := manager.GetOrCreateClient(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent language")
	}
}

// BenchmarkManagerGetOrCreateClient benchmarks client creation/retrieval
func BenchmarkManagerGetOrCreateClient(b *testing.B) {
	log := logger.New(os.Stdout, os.Stderr, logger.WARN, "")

	manager := NewManager(&ManagerConfig{
		RootPath: ".",
		ServerConfigs: map[string]ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
				Enabled: true,
			},
		},
	}, log)

	ctx := context.Background()

	// Create client once
	_, err := manager.GetOrCreateClient(ctx, "go")
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer manager.CloseClient(ctx, "go")

	b.ResetTimer()

	// Benchmark retrieving existing client
	for i := 0; i < b.N; i++ {
		c, ok := manager.GetClient("go")
		if !ok {
			b.Fatalf("Failed to get client")
		}
		if c == nil {
			b.Fatal("Client is nil")
		}
	}
}
