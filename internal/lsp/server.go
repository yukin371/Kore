package lsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/yukin/kore/pkg/logger"
)

// Manager manages multiple LSP servers for different languages
type Manager struct {
	log     *logger.Logger
	servers map[string]*Client // languageID -> client
	mu      sync.RWMutex

	config *ManagerConfig
}

// ManagerConfig configures the LSP manager
type ManagerConfig struct {
	RootPath string
}

// ServerConfig configures a language server
type ServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// DefaultServerConfigs returns default server configurations for common languages
var DefaultServerConfigs = map[string]ServerConfig{
	"go": {
		Command: "gopls",
		Args:    []string{"serve"},
	},
	"python": {
		Command: "pyright-langserver",
		Args:    []string{"--stdio"},
	},
	"javascript": {
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	},
	"typescript": {
		Command: "typescript-language-server",
		Args:    []string{"--stdio"},
	},
	"rust": {
		Command: "rust-analyzer",
		Args:    []string{},
	},
	"json": {
		Command: "vscode-json-language-server",
		Args:    []string{"--stdio"},
	},
	"yaml": {
		Command: "yaml-language-server",
		Args:    []string{"--stdio"},
	},
}

// NewManager creates a new LSP manager
func NewManager(config *ManagerConfig, log *logger.Logger) *Manager {
	return &Manager{
		log:     log,
		servers: make(map[string]*Client),
		config:  config,
	}
}

// Start starts the manager
func (m *Manager) Start(ctx context.Context) error {
	m.log.Info("Starting LSP manager")
	return nil
}

// Stop stops all language servers
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log.Info("Stopping LSP manager")

	for lang, client := range m.servers {
		m.log.Info("Stopping %s language server", lang)
		if err := client.Close(ctx); err != nil {
			m.log.Warn("Failed to stop %s server: %v", lang, err)
		}
	}

	m.servers = make(map[string]*Client)
	return nil
}

// GetOrCreateClient gets or creates a language server client for the given language
func (m *Manager) GetOrCreateClient(ctx context.Context, languageID string) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if client already exists
	if client, ok := m.servers[languageID]; ok {
		return client, nil
	}

	// Get server config
	serverConfig, ok := DefaultServerConfigs[languageID]
	if !ok {
		return nil, fmt.Errorf("no language server configured for: %s", languageID)
	}

	// Check if server command exists
	if _, err := lookPath(serverConfig.Command); err != nil {
		return nil, fmt.Errorf("language server not found: %s (error: %w)", serverConfig.Command, err)
	}

	m.log.Info("Starting language server for %s: %s %v", languageID, serverConfig.Command, serverConfig.Args)

	// Create client config
	clientConfig := &ClientConfig{
		ServerCommand: serverConfig.Command,
		ServerArgs:    serverConfig.Args,
		RootURI:       pathToURI(m.config.RootPath),
	}

	// Create and start client
	client := NewClient(clientConfig, m.log)
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start language server: %w", err)
	}

	m.servers[languageID] = client
	m.log.Info("Language server started for %s", languageID)

	return client, nil
}

// GetClient returns a language server client if it exists
func (m *Manager) GetClient(languageID string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.servers[languageID]
	return client, ok
}

// CloseClient closes a language server client
func (m *Manager) CloseClient(ctx context.Context, languageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.servers[languageID]
	if !ok {
		return fmt.Errorf("no server for language: %s", languageID)
	}

	if err := client.Close(ctx); err != nil {
		return err
	}

	delete(m.servers, languageID)
	m.log.Info("Closed language server for %s", languageID)

	return nil
}

// GetLanguageID returns the language ID for a file
func GetLanguageID(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".sh":
		return "shellscript"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	default:
		// Try to detect by filename
		switch filepath.Base(filename) {
		case "Dockerfile":
			return "dockerfile"
		case "Makefile":
			return "makefile"
		case "CMakeLists.txt":
			return "cmake"
		default:
			return "plaintext"
		}
	}
}

// PathToURI converts a file path to a file URI（导出版本）
func PathToURI(path string) string {
	return pathToURI(path)
}

// pathToURI converts a file path to a file URI
func pathToURI(path string) string {
	// Convert absolute path to URI
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Convert to URI format
	// For Windows, convert drive letter to uppercase
	// For Unix, just add file:// prefix
	if len(absPath) >= 2 && absPath[1] == ':' {
		// Windows path: C:\path
		// Convert to: file:///C:/path
		return "file:///" + absPath[0:1] + "/" + toURISlash(absPath[3:])
	}

	// Unix path: /path
	// Convert to: file:///path
	return "file://" + absPath
}

// toURISlash converts path separators to forward slashes
func toURISlash(path string) string {
	// Convert backslashes to forward slashes
	result := make([]byte, len(path))
	for i, c := range path {
		if c == '\\' {
			result[i] = '/'
		} else {
			result[i] = byte(c)
		}
	}
	return string(result)
}

// lookPath checks if a command exists in PATH
func lookPath(file string) (string, error) {
	// Try exec.LookPath
	path, err := exec.LookPath(file)
	if err == nil {
		return path, nil
	}

	// Check common paths
	commonPaths := []string{
		"/usr/bin/" + file,
		"/usr/local/bin/" + file,
		"/snap/bin/" + file,
		"C:\\Program Files\\" + file + "\\" + file + ".exe",
		"C:\\Program Files (x86)\\" + file + "\\" + file + ".exe",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command not found: %s", file)
}
