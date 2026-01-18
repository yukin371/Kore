package lsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/yukin/kore/pkg/logger"
)

var (
	globalManager *Manager
	managerOnce   sync.Once
)

// Manager manages multiple LSP servers for different languages
type Manager struct {
	log     *logger.Logger
	servers map[string]*Client // languageID -> client
	mu      sync.RWMutex

	config      *ManagerConfig
	serverRegistry map[string]ServerConfig
	healthCheckInterval time.Duration
	stopCh              chan struct{}
	running             bool
}

// ManagerConfig configures the LSP manager
type ManagerConfig struct {
	RootPath             string
	HealthCheckInterval  time.Duration
	AutoRestart          bool
	ServerConfigs        map[string]ServerConfig
}

// ServerConfig configures a language server
type ServerConfig struct {
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Enabled   bool     `json:"enabled"`
	Priority  int      `json:"priority"`
	EnvVars   map[string]string `json:"envVars,omitempty"`
}

// ServerStatus represents the status of a language server
type ServerStatus struct {
	Language      string    `json:"language"`
	Running       bool      `json:"running"`
	PID           int       `json:"pid,omitempty"`
	StartTime     time.Time `json:"startTime,omitempty"`
	ErrorCount    int       `json:"errorCount"`
	LastError     string    `json:"lastError,omitempty"`
	Capabilities  ServerCapabilities `json:"capabilities,omitempty"`
}

// DefaultServerConfigs returns default server configurations for common languages
var DefaultServerConfigs = map[string]ServerConfig{
	"go": {
		Command:  "gopls",
		Args:     []string{"serve"},
		Enabled:  true,
		Priority: 100,
	},
	"python": {
		Command:  "pyright-langserver",
		Args:     []string{"--stdio"},
		Enabled:  true,
		Priority: 100,
	},
	"javascript": {
		Command:  "typescript-language-server",
		Args:     []string{"--stdio"},
		Enabled:  true,
		Priority: 90,
	},
	"typescript": {
		Command:  "typescript-language-server",
		Args:     []string{"--stdio"},
		Enabled:  true,
		Priority: 90,
	},
	"rust": {
		Command:  "rust-analyzer",
		Args:     []string{},
		Enabled:  true,
		Priority: 100,
	},
	"json": {
		Command:  "vscode-json-language-server",
		Args:     []string{"--stdio"},
		Enabled:  true,
		Priority: 80,
	},
	"yaml": {
		Command:  "yaml-language-server",
		Args:     []string{"--stdio"},
		Enabled:  true,
		Priority: 80,
	},
	"cpp": {
		Command:  "clangd",
		Args:     []string{"--background-index"},
		Enabled:  true,
		Priority: 100,
	},
	"c": {
		Command:  "clangd",
		Args:     []string{"--background-index"},
		Enabled:  true,
		Priority: 100,
	},
	"java": {
		Command:  "jdtls",
		Args:     []string{},
		Enabled:  true,
		Priority: 90,
	},
}

// NewManager creates a new LSP manager
func NewManager(config *ManagerConfig, log *logger.Logger) *Manager {
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}

	if config.ServerConfigs == nil {
		config.ServerConfigs = DefaultServerConfigs
	}

	return &Manager{
		log:                log,
		servers:            make(map[string]*Client),
		config:             config,
		serverRegistry:     config.ServerConfigs,
		healthCheckInterval: config.HealthCheckInterval,
		stopCh:             make(chan struct{}),
		running:            false,
	}
}

// GetManager returns the global LSP manager singleton
func GetManager(config *ManagerConfig, log *logger.Logger) *Manager {
	managerOnce.Do(func() {
		if config == nil {
			config = &ManagerConfig{
				RootPath:            ".",
				HealthCheckInterval: 30 * time.Second,
				AutoRestart:         true,
				ServerConfigs:       DefaultServerConfigs,
			}
		}
		globalManager = NewManager(config, log)
	})
	return globalManager
}

// Start starts the manager and health check routine
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("manager already running")
	}
	m.running = true
	m.mu.Unlock()

	m.log.Info("Starting LSP manager with health check interval: %v", m.healthCheckInterval)

	// Start health check routine
	go m.healthCheckLoop(ctx)

	return nil
}

// Stop stops all language servers and the health check routine
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = false
	m.mu.Unlock()

	m.log.Info("Stopping LSP manager")

	// Stop health check routine
	close(m.stopCh)

	// Stop all language servers
	m.mu.Lock()
	defer m.mu.Unlock()

	for lang, client := range m.servers {
		m.log.Info("Stopping %s language server", lang)
		if err := client.Close(ctx); err != nil {
			m.log.Warn("Failed to stop %s server: %v", lang, err)
		}
	}

	m.servers = make(map[string]*Client)
	return nil
}

// healthCheckLoop performs periodic health checks on all running servers
func (m *Manager) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.log.Debug("Health check loop stopped: context done")
			return
		case <-m.stopCh:
			m.log.Debug("Health check loop stopped: manager stopped")
			return
		case <-ticker.C:
			m.checkAllServersHealth(ctx)
		}
	}
}

// checkAllServersHealth checks the health of all running servers
func (m *Manager) checkAllServersHealth(ctx context.Context) {
	m.mu.RLock()
	clients := make(map[string]*Client)
	for k, v := range m.servers {
		clients[k] = v
	}
	m.mu.RUnlock()

	for lang, client := range clients {
		if err := m.checkServerHealth(ctx, lang, client); err != nil {
			m.log.Warn("Health check failed for %s: %v", lang, err)
			if m.config.AutoRestart {
				m.restartServer(ctx, lang)
			}
		}
	}
}

// checkServerHealth checks the health of a single server
func (m *Manager) checkServerHealth(ctx context.Context, languageID string, client *Client) error {
	// Perform a simple operation to check if server is responsive
	// We'll try to get completion at a known position
	doc, ok := client.GetDocument("file://dummy")
	if !ok {
		// No documents open, try to open one
		return nil // Consider as healthy if no documents
	}

	// Try to get completion at beginning of document
	_, err := client.Completion(ctx, doc.URI, Position{Line: 0, Character: 0})
	if err != nil {
		return fmt.Errorf("completion check failed: %w", err)
	}

	return nil
}

// restartServer restarts a crashed server
func (m *Manager) restartServer(ctx context.Context, languageID string) {
	m.log.Warn("Attempting to restart %s language server", languageID)

	m.mu.Lock()
	client, exists := m.servers[languageID]
	if exists {
		delete(m.servers, languageID)
	}
	m.mu.Unlock()

	// Close old client if exists
	if exists && client != nil {
		if err := client.Close(ctx); err != nil {
			m.log.Warn("Failed to close old %s server: %v", languageID, err)
		}
	}

	// Start new server
	newClient, err := m.GetOrCreateClient(ctx, languageID)
	if err != nil {
		m.log.Error("Failed to restart %s server: %v", languageID, err)
		return
	}

	m.log.Info("Successfully restarted %s language server (PID: %d)",
		languageID, newClient.cmd.Process.Pid)
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
	serverConfig, ok := m.serverRegistry[languageID]
	if !ok || !serverConfig.Enabled {
		return nil, fmt.Errorf("no language server configured for: %s", languageID)
	}

	// Check if server command exists
	if _, err := lookPath(serverConfig.Command); err != nil {
		return nil, fmt.Errorf("language server not found: %s (error: %w)", serverConfig.Command, err)
	}

	m.log.Info("Starting language server for %s: %s %v", languageID, serverConfig.Command, serverConfig.Args)

	// Create client config
	clientConfig := &ClientConfig{
		ServerCommand:        serverConfig.Command,
		ServerArgs:           serverConfig.Args,
		RootURI:              pathToURI(m.config.RootPath),
		InitializationOptions: nil,
	}

	// Create and start client
	client := NewClient(clientConfig, m.log)
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start language server: %w", err)
	}

	m.servers[languageID] = client
	m.log.Info("Language server started for %s (PID: %d)", languageID, client.cmd.Process.Pid)

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

// GetStatus returns the status of all servers
func (m *Manager) GetStatus() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]ServerStatus, 0, len(m.serverRegistry))

	for lang := range m.serverRegistry {
		status := ServerStatus{
			Language: lang,
			Running:  false,
		}

		if client, ok := m.servers[lang]; ok {
			status.Running = true
			if client.cmd != nil && client.cmd.Process != nil {
				status.PID = client.cmd.Process.Pid
			}
			status.Capabilities = client.capabilities
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// RegisterServer registers a new language server configuration
func (m *Manager) RegisterServer(languageID string, config ServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.serverRegistry[languageID] = config
	m.log.Info("Registered language server for %s: %s", languageID, config.Command)
}

// UnregisterServer unregisters a language server configuration
func (m *Manager) UnregisterServer(languageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.serverRegistry[languageID]; !ok {
		return fmt.Errorf("no server registered for language: %s", languageID)
	}

	// Stop server if running
	if client, ok := m.servers[languageID]; ok {
		if err := client.Close(context.Background()); err != nil {
			m.log.Warn("Failed to stop %s server: %v", languageID, err)
		}
		delete(m.servers, languageID)
	}

	delete(m.serverRegistry, languageID)
	m.log.Info("Unregistered language server for %s", languageID)

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
	case ".js", ".jsx", ".mjs":
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
	case ".cpp", ".cc", ".hpp", ".cxx", ".hxx":
		return "cpp"
	case ".java":
		return "java"
	case ".sh", ".bash":
		return "shellscript"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".dart":
		return "dart"
	case ".lua":
		return "lua"
	case ".r", ".R":
		return "r"
	case ".toml":
		return "toml"
	case ".xml":
		return "xml"
	case ".vue":
		return "vue"
	case ".svelte":
		return "svelte"
	default:
		// Try to detect by filename
		switch filepath.Base(filename) {
		case "Dockerfile", "dockerfile":
			return "dockerfile"
		case "Makefile", "makefile":
			return "makefile"
		case "CMakeLists.txt":
			return "cmake"
		case "package.json", "tsconfig.json":
			return "json"
		case "go.mod", "go.sum":
			return "go"
		case "Cargo.toml", "Cargo.lock":
			return "rust"
		case "pom.xml":
			return "xml"
		case "build.gradle", "settings.gradle":
			return "groovy"
		default:
			// Check for shebang
			return "plaintext"
		}
	}
}

// PathToURI converts a file path to a file URI (exported version)
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

// URIToPath converts a file URI to a file path
func URIToPath(uri string) (string, error) {
	// Remove file:// prefix
	if len(uri) < 7 || uri[:7] != "file://" {
		return "", fmt.Errorf("invalid file URI: %s", uri)
	}

	path := uri[7:]

	// Handle Windows URIs: file:///C:/path
	if len(path) >= 3 && path[2] == ':' {
		// Extract drive letter and convert slashes
		drive := path[0:1]
		rest := path[3:]
		return drive + ":\\ " + fromURISlash(rest), nil
	}

	// Unix path: file:///path
	return fromURISlash(path), nil
}

// fromURISlash converts forward slashes to OS-specific path separators
func fromURISlash(path string) string {
	// Convert forward slashes to backslashes on Windows
	if filepath.Separator == '\\' {
		result := make([]byte, len(path))
		for i, c := range path {
			if c == '/' {
				result[i] = '\\'
			} else {
				result[i] = byte(c)
			}
		}
		return string(result)
	}
	return path
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
		"C:\\Program Files\\" + file + "\\" + file + ".cmd",
		"C:\\Program Files\\" + file + "\\" + file + ".bat",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command not found: %s", file)
}
