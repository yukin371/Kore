package config

import (
	"encoding/json"
	"fmt"
)

// Config holds all configuration for Kore
type Config struct {
	LLM      LLMConfig      `json:"llm"`
	Context  ContextConfig  `json:"context"`
	Security SecurityConfig `json:"security"`
	UI       UIConfig       `json:"ui"`
}

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	Provider    string  `json:"provider"`     // "openai" or "ollama"
	Model       string  `json:"model"`        // Model name
	APIKey      string  `json:"api_key"`      // API key for OpenAI
	BaseURL     string  `json:"base_url"`     // Custom base URL
	Temperature float32 `json:"temperature"`  // Temperature for generation
	MaxTokens   int     `json:"max_tokens"`   // Maximum tokens in response
}

// ContextConfig holds context management configuration
type ContextConfig struct {
	MaxTokens      int `json:"max_tokens"`       // Token budget for context
	MaxTreeDepth   int `json:"max_tree_depth"`   // Directory tree depth limit
	MaxFilesPerDir int `json:"max_files_per_dir"` // Files per directory limit
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	BlockedCmds  []string `json:"blocked_cmds"`  // Blocked shell commands
	BlockedPaths []string `json:"blocked_paths"` // Blocked file paths
}

// UIConfig holds UI preferences
type UIConfig struct {
	Mode         string `json:"mode"`           // "cli", "tui", or "gui"
	StreamOutput bool   `json:"stream_output"`  // Enable streaming output
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

// String returns a JSON string representation of the config
func (c *Config) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Sprintf("error marshaling config: %v", err)
	}
	return string(data)
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() (*Config, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var clone Config
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &clone, nil
}
