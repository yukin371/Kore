package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

// Loader handles loading configuration from multiple sources
type Loader struct {
	mu            sync.RWMutex
	config        *Config
	configPaths   []string
	schemaLoader  *SchemaLoader
	loadedSources []string
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		config:       DefaultConfig(),
		schemaLoader: NewSchemaLoader(),
		configPaths:  getConfigPaths(),
	}
}

// getConfigPaths returns the list of configuration file paths to check, in priority order
func getConfigPaths() []string {
	var paths []string

	// 1. Environment variable override
	if envPath := os.Getenv("KORE_CONFIG"); envPath != "" {
		paths = append(paths, envPath)
	}

	// 2. Project root directory (.kore.jsonc)
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".kore.jsonc"))
	}

	// 3. User home directory (~/.kore/config.jsonc)
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".kore", "config.jsonc"))
	}

	return paths
}

// Load loads configuration from multiple sources with priority
// Later sources override earlier ones (env > project root > user home)
func (l *Loader) Load() (*Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Start with default config
	mergedConfig := DefaultConfig()
	var loadedSources []string

	// Try to load from each path, accumulating configurations
	for _, path := range l.configPaths {
		if cfg, err := l.loadFromFile(path); err == nil {
			// Merge the loaded config with the current config
			mergedConfig = l.mergeConfigs(mergedConfig, cfg)
			loadedSources = append(loadedSources, path)
		}
	}

	// Apply environment variable overrides
	if envOverrides := l.loadFromEnv(); envOverrides != nil {
		mergedConfig = l.mergeConfigs(mergedConfig, envOverrides)
		loadedSources = append(loadedSources, "environment variables")
	}

	l.loadedSources = loadedSources

	// Validate the final configuration
	if err := l.Validate(mergedConfig); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	l.config = mergedConfig
	return mergedConfig, nil
}

// loadFromFile loads configuration from a single file
func (l *Loader) loadFromFile(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Remove JSONC comments
	cleanedContent, err := l.stripComments(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSONC in %s: %w", path, err)
	}

	// Parse JSON
	var cfg Config
	if err := json.Unmarshal([]byte(cleanedContent), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON in %s: %w", path, err)
	}

	return &cfg, nil
}

// stripComments removes JSONC comments (// and /* */ style)
func (l *Loader) stripComments(content string) (string, error) {
	// Use gjson to parse JSONC (it supports comments natively)
	if !gjson.Valid(content) {
		// If gjson validation fails, try to strip comments manually
		cleaned := l.manualStripComments(content)
		if !gjson.Valid(cleaned) {
			return "", fmt.Errorf("invalid JSONC format")
		}
		return cleaned, nil
	}

	// Use gjson to parse and re-serialize to clean JSON
	result := gjson.Parse(content)
	return result.Raw, nil
}

// manualStripComments manually removes JavaScript-style comments
func (l *Loader) manualStripComments(content string) string {
	lines := strings.Split(content, "\n")
	var cleaned []string
	inBlockComment := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Handle block comments
		if strings.HasPrefix(trimmed, "/*") {
			inBlockComment = true
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
				// Extract text after block comment
				parts := strings.SplitN(trimmed, "*/", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
					cleaned = append(cleaned, strings.TrimSpace(parts[1]))
				}
			}
			continue
		}

		if inBlockComment {
			if strings.Contains(trimmed, "*/") {
				inBlockComment = false
				// Extract text after block comment
				parts := strings.SplitN(trimmed, "*/", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
					cleaned = append(cleaned, strings.TrimSpace(parts[1]))
				}
			}
			continue
		}

		// Handle inline comments (//)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Handle inline comments after content
		if idx := strings.Index(trimmed, "//"); idx > 0 {
			cleaned = append(cleaned, strings.TrimSpace(trimmed[:idx]))
			continue
		}

		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}

// loadFromEnv loads configuration overrides from environment variables
func (l *Loader) loadFromEnv() *Config {
	cfg := &Config{}

	// LLM configuration
	if v := os.Getenv("KORE_LLM_PROVIDER"); v != "" {
		cfg.LLM.Provider = v
	}
	if v := os.Getenv("KORE_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("KORE_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("KORE_LLM_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("KORE_LLM_TEMPERATURE"); v != "" {
		var temp float32
		if _, err := fmt.Sscanf(v, "%f", &temp); err == nil {
			cfg.LLM.Temperature = temp
		}
	}
	if v := os.Getenv("KORE_LLM_MAX_TOKENS"); v != "" {
		var tokens int
		if _, err := fmt.Sscanf(v, "%d", &tokens); err == nil {
			cfg.LLM.MaxTokens = tokens
		}
	}

	// Context configuration
	if v := os.Getenv("KORE_CONTEXT_MAX_TOKENS"); v != "" {
		var tokens int
		if _, err := fmt.Sscanf(v, "%d", &tokens); err == nil {
			cfg.Context.MaxTokens = tokens
		}
	}

	// UI configuration
	if v := os.Getenv("KORE_UI_MODE"); v != "" {
		cfg.UI.Mode = v
	}
	if v := os.Getenv("KORE_UI_STREAM_OUTPUT"); v != "" {
		cfg.UI.StreamOutput = strings.ToLower(v) == "true" || v == "1"
	}

	// Return nil if no environment variables were set
	if cfg.LLM.Provider == "" && cfg.LLM.Model == "" && cfg.LLM.APIKey == "" &&
		cfg.LLM.BaseURL == "" && cfg.LLM.Temperature == 0 && cfg.LLM.MaxTokens == 0 &&
		cfg.Context.MaxTokens == 0 && cfg.UI.Mode == "" && !cfg.UI.StreamOutput {
		return nil
	}

	return cfg
}

// mergeConfigs merges two configurations, with cfg2 overriding cfg1
func (l *Loader) mergeConfigs(cfg1, cfg2 *Config) *Config {
	if cfg2 == nil {
		return cfg1
	}

	merged := *cfg1

	// Merge LLM config
	if cfg2.LLM.Provider != "" {
		merged.LLM.Provider = cfg2.LLM.Provider
	}
	if cfg2.LLM.Model != "" {
		merged.LLM.Model = cfg2.LLM.Model
	}
	if cfg2.LLM.APIKey != "" {
		merged.LLM.APIKey = cfg2.LLM.APIKey
	}
	if cfg2.LLM.BaseURL != "" {
		merged.LLM.BaseURL = cfg2.LLM.BaseURL
	}
	if cfg2.LLM.Temperature != 0 {
		merged.LLM.Temperature = cfg2.LLM.Temperature
	}
	if cfg2.LLM.MaxTokens != 0 {
		merged.LLM.MaxTokens = cfg2.LLM.MaxTokens
	}

	// Merge Context config
	if cfg2.Context.MaxTokens != 0 {
		merged.Context.MaxTokens = cfg2.Context.MaxTokens
	}
	if cfg2.Context.MaxTreeDepth != 0 {
		merged.Context.MaxTreeDepth = cfg2.Context.MaxTreeDepth
	}
	if cfg2.Context.MaxFilesPerDir != 0 {
		merged.Context.MaxFilesPerDir = cfg2.Context.MaxFilesPerDir
	}

	// Merge Security config
	if len(cfg2.Security.BlockedCmds) > 0 {
		merged.Security.BlockedCmds = cfg2.Security.BlockedCmds
	}
	if len(cfg2.Security.BlockedPaths) > 0 {
		merged.Security.BlockedPaths = cfg2.Security.BlockedPaths
	}

	// Merge UI config
	if cfg2.UI.Mode != "" {
		merged.UI.Mode = cfg2.UI.Mode
	}
	// Note: StreamOutput is a bool, so we need special handling
	// Only override if explicitly set (we can't distinguish between default false and not set)

	return &merged
}

// Validate validates the configuration against the JSON schema
func (l *Loader) Validate(cfg *Config) error {
	return l.schemaLoader.Validate(cfg)
}

// Save saves the configuration to the specified path
func (l *Loader) Save(cfg *Config, path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate before saving
	if err := l.Validate(cfg); err != nil {
		return fmt.Errorf("cannot save invalid configuration: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetLoadedSources returns the list of sources that were successfully loaded
func (l *Loader) GetLoadedSources() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	sources := make([]string, len(l.loadedSources))
	copy(sources, l.loadedSources)
	return sources
}

// LoadConfig is a convenience function that loads configuration with default settings
func LoadConfig() (*Config, error) {
	loader := NewLoader()
	return loader.Load()
}
