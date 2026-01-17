package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/yukin/kore/pkg/logger"
)

// Client represents an LSP client
type Client struct {
	log       *logger.Logger
	rpc       *JSONRPC2
	cmd       *exec.Cmd
	serverCmd string
	serverArgs []string

	mu             sync.RWMutex
	capabilities   ServerCapabilities
	rootURI        string
	initialized    bool
	documents      map[string]*Document
	docVersion     int

	diagnosticHandlers []func(PublishDiagnosticsParams)
}

// Document represents an open document
type Document struct {
	URI        string
	LanguageID string
	Version    int
	Text       string
}

// ClientConfig configures an LSP client
type ClientConfig struct {
	ServerCommand string
	ServerArgs    []string
	RootURI       string
	InitializationOptions interface{}
}

// NewClient creates a new LSP client
func NewClient(config *ClientConfig, log *logger.Logger) *Client {
	return &Client{
		log:            log,
		serverCmd:      config.ServerCommand,
		serverArgs:     config.ServerArgs,
		rootURI:        config.RootURI,
		documents:      make(map[string]*Document),
		diagnosticHandlers: make([]func(PublishDiagnosticsParams), 0),
	}
}

// Start starts the LSP server and initializes the client
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.log.Info("Starting LSP server: %s %v", c.serverCmd, c.serverArgs)

	// Start server process
	c.cmd = exec.Command(c.serverCmd, c.serverArgs...)

	// Create pipes for stdin/stdout
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start copying stderr to log
	go func() {
		_, _ = io.Copy(os.Stderr, stderr)
	}()

	// Start the server
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	c.log.Info("LSP server started with PID: %d", c.cmd.Process.Pid)

	// Create JSON-RPC client
	c.rpc = NewJSONRPC2(stdout, stdin, c.log)

	// Register handlers
	c.registerHandlers()

	// Start message processing
	if err := c.rpc.Start(ctx); err != nil {
		c.killServer()
		return fmt.Errorf("failed to start RPC: %w", err)
	}

	// Initialize
	if err := c.initialize(ctx); err != nil {
		c.killServer()
		return fmt.Errorf("failed to initialize: %w", err)
	}

	return nil
}

// registerHandlers registers JSON-RPC handlers
func (c *Client) registerHandlers() {
	// Handle window/showMessage notifications
	c.rpc.Handle("window/showMessage", func(ctx context.Context, params interface{}) (interface{}, error) {
		c.log.Info("[LSP] showMessage: %v", params)
		return nil, nil
	})

	// Handle window/logMessage notifications
	c.rpc.Handle("window/logMessage", func(ctx context.Context, params interface{}) (interface{}, error) {
		c.log.Debug("[LSP] logMessage: %v", params)
		return nil, nil
	})

	// Handle textDocument/publishDiagnostics notifications
	c.rpc.Handle("textDocument/publishDiagnostics", func(ctx context.Context, params interface{}) (interface{}, error) {
		var diagnostics PublishDiagnosticsParams
		if err := unmarshalParams(params, &diagnostics); err != nil {
			return nil, err
		}

		c.log.Debug("[LSP] Diagnostics for %s: %d items", diagnostics.URI, len(diagnostics.Diagnostics))

		// Notify handlers
		c.mu.RLock()
		handlers := c.diagnosticHandlers
		c.mu.RUnlock()

		for _, handler := range handlers {
			handler(diagnostics)
		}

		return nil, nil
	})

	// Handle workspace/applyEdit requests
	c.rpc.Handle("workspace/applyEdit", func(ctx context.Context, params interface{}) (interface{}, error) {
		c.log.Info("[LSP] applyEdit: %v", params)
		return map[string]interface{}{"applied": true}, nil
	})
}

// initialize initializes the LSP server
func (c *Client) initialize(ctx context.Context) error {
	c.log.Info("Initializing LSP server...")

	// Create initialize params
	params := InitializeParams{
		ProcessID: os.Getpid(),
		RootURI:   c.rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{
				Synchronization: &SynchronizationCapabilities{
					DidSave: true,
				},
				Completion: &CompletionCapabilities{
					CompletionItem: &CompletionItemCapabilities{
						SnippetSupport: false,
						DocumentationFormat: []string{"plaintext", "markdown"},
					},
				},
				Hover: &HoverCapabilities{
					ContentFormat: []string{"plaintext", "markdown"},
				},
				Definition: &DefinitionCapabilities{
					LinkSupport: false,
				},
				References: &ReferencesCapabilities{},
				DocumentHighlight: &DocumentHighlightCapabilities{},
			},
			Workspace: &WorkspaceClientCapabilities{
				ApplyEdit: false,
			},
		},
	}

	// Send initialize request
	var result InitializeResult
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := c.rpc.Request(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	c.capabilities = result.Capabilities
	c.log.Info("Server capabilities initialized: %+v", result.ServerInfo)

	// Send initialized notification
	if err := c.rpc.Notify("initialized", map[string]interface{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	c.initialized = true
	c.log.Info("LSP client initialized successfully")

	return nil
}

// Close closes the LSP client
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return nil
	}

	c.log.Info("Shutting down LSP client...")

	// Shutdown request
	var shutdownResult interface{}
	if err := c.rpc.Request(ctx, "shutdown", nil, &shutdownResult); err != nil {
		c.log.Warn("Shutdown request failed: %v", err)
	}

	// Exit notification
	if err := c.rpc.Notify("exit", nil); err != nil {
		c.log.Warn("Exit notification failed: %v", err)
	}

	// Close RPC
	if err := c.rpc.Close(); err != nil {
		c.log.Warn("Failed to close RPC: %v", err)
	}

	// Kill server process
	c.killServer()

	c.initialized = false
	c.log.Info("LSP client closed")

	return nil
}

// killServer kills the server process
func (c *Client) killServer() {
	if c.cmd != nil && c.cmd.Process != nil {
		c.log.Debug("Killing LSP server process (PID: %d)", c.cmd.Process.Pid)
		if err := c.cmd.Process.Kill(); err != nil {
			c.log.Warn("Failed to kill server process: %v", err)
		}
		if err := c.cmd.Wait(); err != nil {
			c.log.Warn("Server wait failed: %v", err)
		}
		c.cmd = nil
	}
}

// DidOpen opens a document
func (c *Client) DidOpen(ctx context.Context, uri, languageID, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	c.docVersion++
	doc := &Document{
		URI:        uri,
		LanguageID: languageID,
		Version:    c.docVersion,
		Text:       text,
	}
	c.documents[uri] = doc

	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    doc.Version,
			Text:       text,
		},
	}

	c.log.Debug("DidOpen: %s (version %d)", uri, doc.Version)
	return c.rpc.Notify("textDocument/didOpen", params)
}

// DidChange changes a document
func (c *Client) DidChange(ctx context.Context, uri string, changes []TextDocumentContentChangeEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	doc, ok := c.documents[uri]
	if !ok {
		return fmt.Errorf("document not open: %s", uri)
	}

	c.docVersion++
	doc.Version = c.docVersion

	// Update document text for full document changes
	if len(changes) == 1 && changes[0].Range == nil {
		doc.Text = changes[0].Text
	}

	params := DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     uri,
			Version: doc.Version,
		},
		ContentChanges: changes,
	}

	c.log.Debug("DidChange: %s (version %d, %d changes)", uri, doc.Version, len(changes))
	return c.rpc.Notify("textDocument/didChange", params)
}

// DidClose closes a document
func (c *Client) DidClose(ctx context.Context, uri string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	if _, ok := c.documents[uri]; !ok {
		return fmt.Errorf("document not open: %s", uri)
	}

	delete(c.documents, uri)

	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	c.log.Debug("DidClose: %s", uri)
	return c.rpc.Notify("textDocument/didClose", params)
}

// Completion requests completion items
func (c *Client) Completion(ctx context.Context, uri string, pos Position) (*CompletionList, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	if c.capabilities.CompletionProvider == nil {
		return nil, fmt.Errorf("completion not supported")
	}

	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	}

	var result interface{}

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.rpc.Request(ctx, "textDocument/completion", params, &result); err != nil {
		return nil, fmt.Errorf("completion request failed: %w", err)
	}

	// Handle both CompletionList and CompletionItem[] responses
	switch v := result.(type) {
	case []interface{}:
		// Array of CompletionItem
		list := &CompletionList{IsIncomplete: false}
		for _, item := range v {
			var completionItem CompletionItem
			if err := unmarshalParams(item, &completionItem); err != nil {
				return nil, err
			}
			list.Items = append(list.Items, completionItem)
		}
		return list, nil
	case map[string]interface{}:
		// CompletionList
		var list CompletionList
		if err := unmarshalParams(result, &list); err != nil {
			return nil, err
		}
		return &list, nil
	default:
		return nil, fmt.Errorf("unexpected completion result type: %T", result)
	}
}

// Definition requests definition locations
func (c *Client) Definition(ctx context.Context, uri string, pos Position) ([]Location, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	if c.capabilities.DefinitionProvider == nil {
		return nil, fmt.Errorf("definition not supported")
	}

	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	}

	var result interface{}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.rpc.Request(ctx, "textDocument/definition", params, &result); err != nil {
		return nil, fmt.Errorf("definition request failed: %w", err)
	}

	// Handle Location, Location[], or LocationLink[]
	return unmarshalLocations(result)
}

// Hover requests hover information
func (c *Client) Hover(ctx context.Context, uri string, pos Position) (*Hover, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	if c.capabilities.HoverProvider == nil {
		return nil, fmt.Errorf("hover not supported")
	}

	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
	}

	var result Hover

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.rpc.Request(ctx, "textDocument/hover", params, &result); err != nil {
		return nil, fmt.Errorf("hover request failed: %w", err)
	}

	return &result, nil
}

// References 查找引用
func (c *Client) References(ctx context.Context, uri string, pos Position) ([]Location, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	if c.capabilities.ReferencesProvider == nil {
		return nil, fmt.Errorf("references not supported")
	}

	params := ReferencesParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     pos,
		Context: ReferenceContext{
			IncludeDeclaration: false,
		},
	}

	var result interface{}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.rpc.Request(ctx, "textDocument/references", params, &result); err != nil {
		return nil, fmt.Errorf("references request failed: %w", err)
	}

	// Handle Location array
	return unmarshalLocations(result)
}

// DidSave 保存文档
func (c *Client) DidSave(ctx context.Context, uri string, text *string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	if _, ok := c.documents[uri]; !ok {
		return fmt.Errorf("document not open: %s", uri)
	}

	params := DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Text:         text,
	}

	c.log.Debug("DidSave: %s", uri)
	return c.rpc.Notify("textDocument/didSave", params)
}

// OnDiagnostics registers a handler for diagnostic notifications
func (c *Client) OnDiagnostics(handler func(PublishDiagnosticsParams)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.diagnosticHandlers = append(c.diagnosticHandlers, handler)
}

// GetDocument returns an open document
func (c *Client) GetDocument(uri string) (*Document, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	doc, ok := c.documents[uri]
	return doc, ok
}

// Helper function to unmarshal parameters
func unmarshalParams(params interface{}, v interface{}) error {
	data, err := marshalJSON(params)
	if err != nil {
		return err
	}
	return unmarshalJSON(data, v)
}

func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func unmarshalLocations(result interface{}) ([]Location, error) {
	switch v := result.(type) {
	case []interface{}:
		// Location array
		var locations []Location
		for _, item := range v {
			var loc Location
			if err := unmarshalParams(item, &loc); err != nil {
				return nil, err
			}
			locations = append(locations, loc)
		}
		return locations, nil
	case map[string]interface{}:
		// Single Location
		var loc Location
		if err := unmarshalParams(result, &loc); err != nil {
			return nil, err
		}
		return []Location{loc}, nil
	default:
		return nil, fmt.Errorf("unexpected location type: %T", result)
	}
}
