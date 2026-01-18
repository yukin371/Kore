package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/yukin371/Kore/pkg/utils"
)

// Config holds all configuration for Kore
type Config struct {
	LLM      LLMConfig      `mapstructure:"llm"`
	Context  ContextConfig  `mapstructure:"context"`
	Security SecurityConfig `mapstructure:"security"`
	UI       UIConfig       `mapstructure:"ui"`
}

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	Provider    string  `mapstructure:"provider"`     // "openai" or "ollama"
	Model       string  `mapstructure:"model"`        // Model name
	APIKey      string  `mapstructure:"api_key"`      // API key for OpenAI
	BaseURL     string  `mapstructure:"base_url"`     // Custom base URL
	Temperature float32 `mapstructure:"temperature"`  // Temperature for generation
	MaxTokens   int     `mapstructure:"max_tokens"`   // Maximum tokens in response
}

// ContextConfig holds context management configuration
type ContextConfig struct {
	MaxTokens      int `mapstructure:"max_tokens"`       // Token budget for context
	MaxTreeDepth   int `mapstructure:"max_tree_depth"`   // Directory tree depth limit
	MaxFilesPerDir int `mapstructure:"max_files_per_dir"` // Files per directory limit
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	BlockedCmds  []string `mapstructure:"blocked_cmds"`   // Blocked shell commands
	BlockedPaths []string `mapstructure:"blocked_paths"`  // Blocked file paths
}

// UIConfig holds UI preferences
type UIConfig struct {
	Mode         string `mapstructure:"mode"`           // "cli", "tui", or "gui"
	StreamOutput bool   `mapstructure:"stream_output"`  // Enable streaming output
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
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
		Security: SecurityConfig{
			BlockedCmds: []string{
				"rm", "sudo", "shutdown", "format", "del",
				"mkfs", "dd", "reboot", "poweroff",
			},
			BlockedPaths: []string{
				".git", ".env", "node_modules/.cache",
			},
		},
		UI: UIConfig{
			Mode:         "cli",
			StreamOutput: true,
		},
	}
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	configDir, err := utils.GetConfigDir("kore")
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	// Ensure config directory exists
	if err := utils.EnsureDir(configDir); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	// Set up Viper
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Set defaults
	setDefaults()

	// Read config file (if it exists)
	if _, err := os.Stat(configPath); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		// Create default config file
		cfg := DefaultConfig()
		if err := Save(cfg); err != nil {
			// Non-fatal: just log and continue
			fmt.Printf("Warning: Could not create default config: %v\n", err)
		}
	}

	// Environment variable overrides
	viper.SetEnvPrefix("KORE")
	viper.AutomaticEnv()

	// Unmarshal config
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values in Viper
func setDefaults() {
	viper.SetDefault("llm.provider", "openai")
	viper.SetDefault("llm.model", "gpt-4")
	viper.SetDefault("llm.temperature", 0.7)
	viper.SetDefault("llm.max_tokens", 4000)

	viper.SetDefault("context.max_tokens", 8000)
	viper.SetDefault("context.max_tree_depth", 5)
	viper.SetDefault("context.max_files_per_dir", 50)

	viper.SetDefault("ui.mode", "cli")
	viper.SetDefault("ui.stream_output", true)

	viper.SetDefault("security.blocked_cmds", []string{
		"rm", "sudo", "shutdown", "format", "del",
		"mkfs", "dd", "reboot", "poweroff",
	})
	viper.SetDefault("security.blocked_paths", []string{
		".git", ".env", "node_modules/.cache",
	})
}

// Save writes configuration to file
func Save(cfg *Config) error {
	configDir, err := utils.GetConfigDir("kore")
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	// Set values in Viper
	viper.Set("llm.provider", cfg.LLM.Provider)
	viper.Set("llm.model", cfg.LLM.Model)
	viper.Set("llm.api_key", cfg.LLM.APIKey)
	viper.Set("llm.base_url", cfg.LLM.BaseURL)
	viper.Set("llm.temperature", cfg.LLM.Temperature)
	viper.Set("llm.max_tokens", cfg.LLM.MaxTokens)

	viper.Set("context.max_tokens", cfg.Context.MaxTokens)
	viper.Set("context.max_tree_depth", cfg.Context.MaxTreeDepth)
	viper.Set("context.max_files_per_dir", cfg.Context.MaxFilesPerDir)

	viper.Set("security.blocked_cmds", cfg.Security.BlockedCmds)
	viper.Set("security.blocked_paths", cfg.Security.BlockedPaths)

	viper.Set("ui.mode", cfg.UI.Mode)
	viper.Set("ui.stream_output", cfg.UI.StreamOutput)

	// Write to file
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
