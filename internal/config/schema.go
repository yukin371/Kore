package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// SchemaLoader handles JSON schema validation
type SchemaLoader struct {
	mu     sync.RWMutex
	schema *jsonschema.Schema
}

// NewSchemaLoader creates a new schema loader
func NewSchemaLoader() *SchemaLoader {
	return &SchemaLoader{}
}

// getSchemaPath returns the path to the JSON schema file
func (sl *SchemaLoader) getSchemaPath() (string, error) {
	// Try to find schema in multiple locations
	// 1. Project root
	if cwd, err := os.Getwd(); err == nil {
		schemaPath := filepath.Join(cwd, "schemas", "config.schema.json")
		if _, err := os.Stat(schemaPath); err == nil {
			return schemaPath, nil
		}
	}

	// 2. Executable directory
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		schemaPath := filepath.Join(exeDir, "schemas", "config.schema.json")
		if _, err := os.Stat(schemaPath); err == nil {
			return schemaPath, nil
		}
	}

	// 3. User config directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		schemaPath := filepath.Join(homeDir, ".kore", "schemas", "config.schema.json")
		if _, err := os.Stat(schemaPath); err == nil {
			return schemaPath, nil
		}
	}

	// Generate schema on-the-fly
	return "", fmt.Errorf("schema file not found, will generate on-the-fly")
}

// loadOrCreateSchema loads the schema or creates it on-the-fly
func (sl *SchemaLoader) loadOrCreateSchema() (*jsonschema.Schema, error) {
	// Try to load from file
	if schemaPath, err := sl.getSchemaPath(); err == nil {
		return jsonschema.Compile(schemaPath)
	}

	// Create schema on-the-fly
	schemaJSON := sl.generateSchema()
	return jsonschema.CompileString("config.schema.json", schemaJSON)
}

// Validate validates a configuration against the JSON schema
func (sl *SchemaLoader) Validate(cfg *Config) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	// Load or create schema
	schema, err := sl.loadOrCreateSchema()
	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	// Convert config to JSON
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config for validation: %w", err)
	}

	// Validate
	var cfgData interface{}
	if err := json.Unmarshal(cfgJSON, &cfgData); err != nil {
		return fmt.Errorf("failed to unmarshal config for validation: %w", err)
	}

	if err := schema.Validate(cfgData); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	return nil
}

// generateSchema generates the JSON schema for configuration validation
func (sl *SchemaLoader) generateSchema() string {
	schema := map[string]interface{}{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"title":   "Kore Configuration",
		"type":    "object",
		"properties": map[string]interface{}{
			"llm": map[string]interface{}{
				"type":        "object",
				"description": "LLM provider configuration",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "LLM provider name (openai, ollama)",
						"enum":        []string{"openai", "ollama"},
					},
					"model": map[string]interface{}{
						"type":        "string",
						"description": "Model name",
					},
					"api_key": map[string]interface{}{
						"type":        "string",
						"description": "API key for authentication",
					},
					"base_url": map[string]interface{}{
						"type":        "string",
						"description": "Custom base URL for API",
					},
					"temperature": map[string]interface{}{
						"type":        "number",
						"minimum":     0,
						"maximum":     2,
						"description": "Temperature for generation (0-2)",
					},
					"max_tokens": map[string]interface{}{
						"type":        "integer",
						"minimum":     1,
						"description": "Maximum tokens in response",
					},
				},
				"required": []string{"provider", "model"},
			},
			"context": map[string]interface{}{
				"type":        "object",
				"description": "Context management configuration",
				"properties": map[string]interface{}{
					"max_tokens": map[string]interface{}{
						"type":        "integer",
						"minimum":     100,
						"description": "Token budget for context",
					},
					"max_tree_depth": map[string]interface{}{
						"type":        "integer",
						"minimum":     1,
						"maximum":     20,
						"description": "Directory tree depth limit",
					},
					"max_files_per_dir": map[string]interface{}{
						"type":        "integer",
						"minimum":     1,
						"description": "Files per directory limit",
					},
				},
			},
			"security": map[string]interface{}{
				"type":        "object",
				"description": "Security settings",
				"properties": map[string]interface{}{
					"blocked_cmds": map[string]interface{}{
						"type":        "array",
						"description": "Blocked shell commands",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"blocked_paths": map[string]interface{}{
						"type":        "array",
						"description": "Blocked file paths",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			"ui": map[string]interface{}{
				"type":        "object",
				"description": "UI preferences",
				"properties": map[string]interface{}{
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "UI mode",
						"enum":        []string{"cli", "tui", "gui"},
					},
					"stream_output": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable streaming output",
					},
				},
			},
		},
		"required": []string{"llm"},
	}

	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
	return string(schemaJSON)
}

// ValidateConfig is a convenience function for validation
func ValidateConfig(cfg *Config) error {
	loader := NewSchemaLoader()
	return loader.Validate(cfg)
}

// SaveSchema saves the JSON schema to a file
func (sl *SchemaLoader) SaveSchema(path string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	schemaJSON := sl.generateSchema()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create schema directory: %w", err)
	}

	// Write schema with pretty formatting
	var buf []byte
	buf, err := json.MarshalIndent(json.RawMessage(schemaJSON), "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format schema: %w", err)
	}

	if err := os.WriteFile(path, buf, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	return nil
}
