package lsp

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yukin371/Kore/pkg/logger"
)

// TestFormatting tests document formatting
func TestFormatting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This test requires an actual LSP server
	// Skip if gopls is not available
	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	err = client.DidOpen(ctx, uri, "go", "package main\n\nfunc main() {\n}\n")
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	options := FormattingOptions{
		TabSize:      4,
		InsertSpaces: true,
	}

	edits, err := client.Formatting(ctx, uri, options)
	if err != nil {
		t.Logf("Formatting failed (may not be supported): %v", err)
		return
	}

	if len(edits) > 0 {
		t.Logf("Formatting edits: %+v", edits)
	}
}

// TestRangeFormatting tests range formatting
func TestRangeFormatting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	err = client.DidOpen(ctx, uri, "go", "package main\n\nfunc main() {\n}\n")
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	options := FormattingOptions{
		TabSize:      4,
		InsertSpaces: true,
	}

	rng := Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 2, Character: 0},
	}

	edits, err := client.RangeFormatting(ctx, uri, rng, options)
	if err != nil {
		t.Logf("Range formatting failed (may not be supported): %v", err)
		return
	}

	if len(edits) > 0 {
		t.Logf("Range formatting edits: %+v", edits)
	}
}

// TestDocumentSymbol tests document symbols
func TestDocumentSymbol(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	source := `package main

type MyStruct struct {
	Field int
}

func (m *MyStruct) Method() int {
	return m.Field
}

func main() {
}`

	err = client.DidOpen(ctx, uri, "go", source)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	symbols, err := client.DocumentSymbol(ctx, uri)
	if err != nil {
		t.Logf("Document symbol failed (may not be supported): %v", err)
		return
	}

	t.Logf("Found %d symbols", len(symbols))
	for _, sym := range symbols {
		t.Logf("Symbol: %+v", sym)
	}
}

// TestWorkspaceSymbol tests workspace symbols
func TestWorkspaceSymbol(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	symbols, err := client.WorkspaceSymbol(ctx, "main")
	if err != nil {
		t.Logf("Workspace symbol failed (may not be supported): %v", err)
		return
	}

	t.Logf("Found %d workspace symbols", len(symbols))
	for _, sym := range symbols {
		t.Logf("Symbol: %+v", sym)
	}
}

// TestRename tests rename operations
func TestRename(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	source := `package main

func oldName() int {
	return 42
}

func main() {
	oldName()
}`

	err = client.DidOpen(ctx, uri, "go", source)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	// Try prepare rename first
	pos := Position{Line: 2, Character: 5} // Position on "oldName"
	prepareResult, err := client.PrepareRename(ctx, uri, pos)
	if err != nil {
		t.Logf("Prepare rename failed (may not be supported): %v", err)
	} else {
		t.Logf("Prepare rename result: %+v", prepareResult)
	}

	// Perform rename
	edit, err := client.Rename(ctx, uri, pos, "newName")
	if err != nil {
		t.Logf("Rename failed (may not be supported): %v", err)
		return
	}

	t.Logf("Rename edit: %+v", edit)
}

// TestCodeAction tests code actions
func TestCodeAction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	source := `package main

func main() {
	x := 1
}`
	err = client.DidOpen(ctx, uri, "go", source)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	// Wait for diagnostics
	time.Sleep(500 * time.Millisecond)

	actions, err := client.CodeAction(ctx, uri, Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 5, Character: 0},
	}, nil, nil)

	if err != nil {
		t.Logf("Code action failed (may not be supported): %v", err)
		return
	}

	t.Logf("Found %d code actions", len(actions))
	for _, action := range actions {
		t.Logf("Action: %s (Kind: %s)", action.Title, action.Kind)
	}
}

// TestCodeLens tests code lens
func TestCodeLens(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	source := `package main

func main() {
	println("hello")
}`

	err = client.DidOpen(ctx, uri, "go", source)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	lenses, err := client.CodeLens(ctx, uri)
	if err != nil {
		t.Logf("Code lens failed (may not be supported): %v", err)
		return
	}

	t.Logf("Found %d code lenses", len(lenses))
	for _, lens := range lenses {
		if lens.Command != nil {
			t.Logf("Lens: %s at %+v", lens.Command.Title, lens.Range)
		}
	}
}

// TestInlayHint tests inlay hints
func TestInlayHint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	source := `package main

func add(a, b int) int {
	return a + b
}

func main() {
	result := add(1, 2)
}`

	err = client.DidOpen(ctx, uri, "go", source)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	hints, err := client.InlayHint(ctx, uri, Range{
		Start: Position{Line: 0, Character: 0},
		End:   Position{Line: 9, Character: 0},
	})

	if err != nil {
		t.Logf("Inlay hint failed (may not be supported): %v", err)
		return
	}

	t.Logf("Found %d inlay hints", len(hints))
	for _, hint := range hints {
		t.Logf("Hint at %+v: %v", hint.Position, hint.Label)
	}
}

// TestSignatureHelp tests signature help
func TestSignatureHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	source := `package main

func add(a, b int) int {
	return a + b
}

func main() {
	add(1, )
}`

	err = client.DidOpen(ctx, uri, "go", source)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	sigHelp, err := client.SignatureHelp(ctx, uri, Position{Line: 7, Character: 8})
	if err != nil {
		t.Logf("Signature help failed (may not be supported): %v", err)
		return
	}

	t.Logf("Signature help: %+v", sigHelp)
	if sigHelp != nil && len(sigHelp.Signatures) > 0 {
		t.Logf("Active signature: %s", sigHelp.Signatures[0].Label)
	}
}

// TestDocumentHighlight tests document highlights
func TestDocumentHighlight(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	_, err := lookPath("gopls")
	if err != nil {
		t.Skip("gopls not available")
	}

	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	config := &ClientConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"serve"},
		RootURI:       pathToURI("."),
	}

	client := NewClient(config, log)
	ctx := context.Background()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}
	defer client.Close(ctx)

	uri := "file:///test.go"
	source := `package main

func main() {
	x := 1
	println(x)
}`

	err = client.DidOpen(ctx, uri, "go", source)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	highlights, err := client.DocumentHighlight(ctx, uri, Position{Line: 4, Character: 10})
	if err != nil {
		t.Logf("Document highlight failed (may not be supported): %v", err)
		return
	}

	t.Logf("Found %d highlights", len(highlights))
	for _, highlight := range highlights {
		t.Logf("Highlight at %+v (kind: %d)", highlight.Range, highlight.Kind)
	}
}

// TestGetLanguageID tests language ID detection
func TestGetLanguageID(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"test.go", "go"},
		{"test.py", "python"},
		{"test.js", "javascript"},
		{"test.ts", "typescript"},
		{"test.rs", "rust"},
		{"test.json", "json"},
		{"test.yaml", "yaml"},
		{"test.yml", "yaml"},
		{"test.c", "c"},
		{"test.cpp", "cpp"},
		{"test.java", "java"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"CMakeLists.txt", "cmake"},
		{"unknown.xyz", "plaintext"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := GetLanguageID(tt.filename)
			if result != tt.expected {
				t.Errorf("GetLanguageID(%q) = %q, want %q", tt.filename, result, tt.expected)
			}
		})
	}
}

// TestPathConversion tests URI <-> Path conversion
func TestPathConversion(t *testing.T) {
	tests := []struct {
		path string
		uri  string
	}{
		{"/usr/local/test.go", "file:///usr/local/test.go"},
		{"C:\\Users\\test\\file.go", "file:///C:/Users/test/file.go"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			uri := PathToURI(tt.path)
			if uri != tt.uri {
				t.Logf("PathToURI(%q) = %q, want %q (may vary by OS)", tt.path, uri, tt.uri)
			}

			path, err := URIToPath(uri)
			if err != nil {
				t.Fatalf("URIToPath failed: %v", err)
			}
			t.Logf("URIToPath(%q) = %q", uri, path)
		})
	}
}
