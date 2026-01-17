package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.LLM.Provider != "openai" {
		t.Errorf("Expected default provider to be 'openai', got '%s'", cfg.LLM.Provider)
	}

	if cfg.LLM.Model != "gpt-4" {
		t.Errorf("Expected default model to be 'gpt-4', got '%s'", cfg.LLM.Model)
	}

	if cfg.LLM.Temperature != 0.7 {
		t.Errorf("Expected default temperature to be 0.7, got %f", cfg.LLM.Temperature)
	}

	if cfg.Context.MaxTokens != 8000 {
		t.Errorf("Expected default max_tokens to be 8000, got %d", cfg.Context.MaxTokens)
	}

	if cfg.UI.Mode != "cli" {
		t.Errorf("Expected default UI mode to be 'cli', got '%s'", cfg.UI.Mode)
	}
}

func TestStripComments(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "remove single line comments",
			input: `{
				// This is a comment
				"key": "value"
			}`,
			expected: `{"key": "value"}`,
		},
		{
			name: "remove block comments",
			input: `{
				/* This is a
				   multi-line comment */
				"key": "value"
			}`,
			expected: `{"key": "value"}`,
		},
		{
			name: "mixed comments",
			input: `{
				// Line comment
				"key1": "value1",
				/* Block comment */
				"key2": "value2"
			}`,
			expected: `{"key1":"value1","key2":"value2"}`,
		},
		{
			name: "no comments",
			input: `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.stripComments(tt.input)
			if err != nil {
				t.Fatalf("stripComments() error = %v", err)
			}

			// Parse both as JSON to compare structure
			var resultJSON, expectedJSON interface{}
			if err := json.Unmarshal([]byte(result), &resultJSON); err != nil {
				t.Fatalf("Failed to parse result JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedJSON); err != nil {
				t.Fatalf("Failed to parse expected JSON: %v", err)
			}

			resultBytes, _ := json.Marshal(resultJSON)
			expectedBytes, _ := json.Marshal(expectedJSON)

			if string(resultBytes) != string(expectedBytes) {
				t.Errorf("stripComments() = %s, want %s", resultBytes, expectedBytes)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create test config file
	configContent := `{
		// Test configuration
		"llm": {
			"provider": "ollama",
			"model": "llama2"
		}
	}`

	configPath := filepath.Join(tmpDir, "test.jsonc")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Load configuration
	loader := NewLoader()
	cfg, err := loader.loadFromFile(configPath)
	if err != nil {
		t.Fatalf("loadFromFile() error = %v", err)
	}

	if cfg.LLM.Provider != "ollama" {
		t.Errorf("Expected provider 'ollama', got '%s'", cfg.LLM.Provider)
	}

	if cfg.LLM.Model != "llama2" {
		t.Errorf("Expected model 'llama2', got '%s'", cfg.LLM.Model)
	}
}

func TestMergeConfigs(t *testing.T) {
	loader := NewLoader()

	cfg1 := &Config{
		LLM: LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   4000,
		},
		Context: ContextConfig{
			MaxTokens:      8000,
			MaxTreeDepth:   5,
			MaxFilesPerDir: 50,
		},
	}

	cfg2 := &Config{
		LLM: LLMConfig{
			Provider:  "ollama", // Override
			Model:     "llama2", // Override
			APIKey:    "test-key", // New value
			BaseURL:   "http://localhost:11434", // New value
		},
		Context: ContextConfig{
			MaxTokens: 16000, // Override
		},
	}

	merged := loader.mergeConfigs(cfg1, cfg2)

	if merged.LLM.Provider != "ollama" {
		t.Errorf("Expected provider 'ollama', got '%s'", merged.LLM.Provider)
	}

	if merged.LLM.Model != "llama2" {
		t.Errorf("Expected model 'llama2', got '%s'", merged.LLM.Model)
	}

	if merged.LLM.APIKey != "test-key" {
		t.Errorf("Expected API key 'test-key', got '%s'", merged.LLM.APIKey)
	}

	if merged.LLM.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7 (from cfg1), got %f", merged.LLM.Temperature)
	}

	if merged.Context.MaxTokens != 16000 {
		t.Errorf("Expected max_tokens 16000, got %d", merged.Context.MaxTokens)
	}

	if merged.Context.MaxTreeDepth != 5 {
		t.Errorf("Expected max_tree_depth 5 (from cfg1), got %d", merged.Context.MaxTreeDepth)
	}
}

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name: "no environment variables",
			envVars: map[string]string{},
			expected: nil,
		},
		{
			name: "provider override",
			envVars: map[string]string{
				"KORE_LLM_PROVIDER": "ollama",
			},
			expected: &Config{
				LLM: LLMConfig{
					Provider: "ollama",
				},
			},
		},
		{
			name: "multiple overrides",
			envVars: map[string]string{
				"KORE_LLM_PROVIDER":    "ollama",
				"KORE_LLM_MODEL":       "llama2",
				"KORE_LLM_TEMPERATURE": "0.5",
				"KORE_UI_MODE":         "tui",
			},
			expected: &Config{
				LLM: LLMConfig{
					Provider:    "ollama",
					Model:       "llama2",
					Temperature: 0.5,
				},
				UI: UIConfig{
					Mode: "tui",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			loader := NewLoader()
			cfg := loader.loadFromEnv()

			if tt.expected == nil {
				if cfg != nil {
					t.Errorf("Expected nil, got %+v", cfg)
				}
				return
			}

			if cfg == nil {
				t.Fatal("Expected config, got nil")
			}

			if cfg.LLM.Provider != tt.expected.LLM.Provider {
				t.Errorf("Expected provider '%s', got '%s'", tt.expected.LLM.Provider, cfg.LLM.Provider)
			}

			if cfg.LLM.Model != tt.expected.LLM.Model {
				t.Errorf("Expected model '%s', got '%s'", tt.expected.LLM.Model, cfg.LLM.Model)
			}

			if cfg.LLM.Temperature != tt.expected.LLM.Temperature {
				t.Errorf("Expected temperature %f, got %f", tt.expected.LLM.Temperature, cfg.LLM.Temperature)
			}

			if cfg.UI.Mode != tt.expected.UI.Mode {
				t.Errorf("Expected UI mode '%s', got '%s'", tt.expected.UI.Mode, cfg.UI.Mode)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid default config",
			cfg:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "valid custom config",
			cfg: &Config{
				LLM: LLMConfig{
					Provider:    "ollama",
					Model:       "llama2",
					Temperature: 0.8,
					MaxTokens:   2000,
				},
				Context: ContextConfig{
					MaxTokens:      16000, // Must be >= 100
					MaxTreeDepth:   5,     // Must be >= 1
					MaxFilesPerDir: 100,   // Must be >= 1
				},
				Security: SecurityConfig{
					BlockedCmds:  []string{"rm"},
					BlockedPaths: []string{".git"},
				},
				UI: UIConfig{
					Mode: "cli", // Must be one of: cli, tui, gui
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - missing provider",
			cfg: &Config{
				LLM: LLMConfig{
					Model: "gpt-4",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - temperature out of range",
			cfg: &Config{
				LLM: LLMConfig{
					Provider:    "openai",
					Model:       "gpt-4",
					Temperature: 3.0, // Out of range [0, 2]
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSave(t *testing.T) {
	loader := NewLoader()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.jsonc")

	cfg := &Config{
		LLM: LLMConfig{
			Provider:    "ollama",
			Model:       "llama2",
			Temperature: 0.8,
			MaxTokens:   2000,
		},
		Context: ContextConfig{
			MaxTokens:      16000,
			MaxTreeDepth:   5,     // Must be >= 1
			MaxFilesPerDir: 100,   // Must be >= 1
		},
		Security: SecurityConfig{
			BlockedCmds:  []string{"rm"},
			BlockedPaths: []string{".git"},
		},
		UI: UIConfig{
			Mode: "cli",
		},
	}

	// Save config
	if err := loader.Save(cfg, configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Save() did not create config file")
	}

	// Load and verify
	loadedCfg, err := loader.loadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedCfg.LLM.Provider != cfg.LLM.Provider {
		t.Errorf("Expected provider '%s', got '%s'", cfg.LLM.Provider, loadedCfg.LLM.Provider)
	}

	if loadedCfg.LLM.Model != cfg.LLM.Model {
		t.Errorf("Expected model '%s', got '%s'", cfg.LLM.Model, loadedCfg.LLM.Model)
	}

	if loadedCfg.Context.MaxTokens != cfg.Context.MaxTokens {
		t.Errorf("Expected max_tokens %d, got %d", cfg.Context.MaxTokens, loadedCfg.Context.MaxTokens)
	}
}

func TestClone(t *testing.T) {
	original := &Config{
		LLM: LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.7,
			MaxTokens:   4000,
		},
	}

	clone, err := original.Clone()
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	if clone.LLM.Provider != original.LLM.Provider {
		t.Errorf("Clone() provider mismatch")
	}

	if clone.LLM.Model != original.LLM.Model {
		t.Errorf("Clone() model mismatch")
	}

	// Modify clone
	clone.LLM.Provider = "ollama"

	// Original should be unchanged
	if original.LLM.Provider == "ollama" {
		t.Error("Clone() did not create a deep copy")
	}
}

func TestLoadConfig(t *testing.T) {
	// Test the convenience function
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig() returned nil")
	}

	// Should have default values
	if cfg.LLM.Provider == "" {
		t.Error("LoadConfig() did not provide default provider")
	}

	if cfg.LLM.Model == "" {
		t.Error("LoadConfig() did not provide default model")
	}
}
