package environment

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLocalEnvironment(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	env, err := NewLocalEnvironment(tmpDir, SecurityLevelStandard)
	if err != nil {
		t.Fatalf("NewLocalEnvironment() error = %v", err)
	}

	if env == nil {
		t.Fatal("NewLocalEnvironment() returned nil")
	}

	if env.GetWorkingDir() != tmpDir {
		t.Errorf("GetWorkingDir() = %s, want %s", env.GetWorkingDir(), tmpDir)
	}
}

func TestNewLocalEnvironment_NonExistentDir(t *testing.T) {
	nonExistentDir := "/tmp/nonexistent_kore_test_dir_12345"

	_, err := NewLocalEnvironment(nonExistentDir, SecurityLevelStandard)
	if err == nil {
		t.Error("NewLocalEnvironment() should return error for non-existent directory")
	}
}

func TestLocalEnvironment_Execute(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()

	// Test simple command
	cmd := &Command{
		Name: "echo",
		Args: []string{"hello", "world"},
	}

	result, err := env.Execute(ctx, cmd)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Execute() ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Stdout != "hello world\n" {
		t.Errorf("Execute() Stdout = %s, want 'hello world\\n'", result.Stdout)
	}
}

func TestLocalEnvironment_Execute_InvalidCommand(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()

	cmd := &Command{
		Name: "nonexistent_command_xyz",
		Args: []string{},
	}

	_, err := env.Execute(ctx, cmd)
	if err == nil {
		t.Error("Execute() should return error for non-existent command")
	}
}

func TestLocalEnvironment_Execute_DangerousCommand(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()

	cmd := &Command{
		Name: "rm",
		Args: []string{"-rf", "/"},
	}

	_, err := env.Execute(ctx, cmd)
	if err == nil {
		t.Error("Execute() should return error for dangerous command")
	}
}

func TestLocalEnvironment_Execute_WithTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()

	cmd := &Command{
		Name:    "sleep",
		Args:    []string{"10"},
		Timeout: 100 * time.Millisecond,
	}

	result, err := env.Execute(ctx, cmd)
	if err != nil {
		// Timeout is expected
		if result == nil {
			t.Error("Execute() should return result even on timeout")
		}
	}
}

func TestLocalEnvironment_ReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Read file
	ctx := context.Background()
	read, err := env.ReadFile(ctx, "test.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(read) != string(content) {
		t.Errorf("ReadFile() content = %s, want %s", string(read), string(content))
	}
}

func TestLocalEnvironment_ReadFile_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()

	// Try to read file outside working directory
	_, err := env.ReadFile(ctx, "../../../etc/passwd")
	if err == nil {
		t.Error("ReadFile() should return error for path traversal attempt")
	}
}

func TestLocalEnvironment_WriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()
	content := []byte("test content")

	opts := &WriteOptions{
		CreateMissingDirs: true,
	}

	err := env.WriteFile(ctx, "subdir/test.txt", content, opts)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Verify file was written
	testFile := filepath.Join(tmpDir, "subdir", "test.txt")
	read, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(read) != string(content) {
		t.Errorf("WriteFile() content mismatch")
	}
}

func TestLocalEnvironment_WriteFile_Backup(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	// Create initial file
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := []byte("initial")
	_ = os.WriteFile(testFile, initialContent, 0644)

	// Write with backup
	ctx := context.Background()
	newContent := []byte("new content")

	opts := &WriteOptions{
		Backup: true,
	}

	err := env.WriteFile(ctx, "test.txt", newContent, opts)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Check backup exists
	backupFile := testFile + ".bak"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Error("WriteFile() should create backup when Backup option is true")
	}

	// Verify backup content
	backupContent, _ := os.ReadFile(backupFile)
	if string(backupContent) != string(initialContent) {
		t.Error("Backup content mismatch")
	}
}

func TestLocalEnvironment_Diff(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	// Create two test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	_ = os.WriteFile(file1, []byte("line 1\nline 2\nline 3\n"), 0644)
	_ = os.WriteFile(file2, []byte("line 1\nline 2 modified\nline 3\n"), 0644)

	ctx := context.Background()
	result, err := env.Diff(ctx, "file1.txt", "file2.txt")
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if !result.HasDiff {
		t.Error("Diff() should detect differences")
	}

	if result.Path1 != "file1.txt" || result.Path2 != "file2.txt" {
		t.Error("Diff() path mismatch")
	}
}

func TestLocalEnvironment_SetWorkingDir(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	newDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(newDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	err = env.SetWorkingDir(newDir)
	if err != nil {
		t.Fatalf("SetWorkingDir() error = %v", err)
	}

	if env.GetWorkingDir() != newDir {
		t.Errorf("GetWorkingDir() = %s, want %s", env.GetWorkingDir(), newDir)
	}
}

func TestLocalEnvironment_SetWorkingDir_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	nonExistentDir := filepath.Join(tmpDir, "nonexistent")

	err := env.SetWorkingDir(nonExistentDir)
	if err == nil {
		t.Error("SetWorkingDir() should return error for non-existent directory")
	}
}

func TestLocalEnvironment_VirtualFileSystem(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()

	// Create virtual document
	content := []byte("virtual content")
	err := env.CreateVirtualDocument(ctx, "/virtual/test.txt", content)
	if err != nil {
		t.Fatalf("CreateVirtualDocument() error = %v", err)
	}

	// Read virtual document
	read, err := env.ReadVirtualDocument(ctx, "/virtual/test.txt")
	if err != nil {
		t.Fatalf("ReadVirtualDocument() error = %v", err)
	}

	if string(read) != string(content) {
		t.Errorf("ReadVirtualDocument() content mismatch")
	}

	// Update virtual document
	newContent := []byte("updated content")
	err = env.UpdateVirtualDocument(ctx, "/virtual/test.txt", newContent)
	if err != nil {
		t.Fatalf("UpdateVirtualDocument() error = %v", err)
	}

	// List virtual documents
	docs, err := env.ListVirtualDocuments(ctx)
	if err != nil {
		t.Fatalf("ListVirtualDocuments() error = %v", err)
	}

	if len(docs) != 1 {
		t.Errorf("ListVirtualDocuments() returned %d documents, want 1", len(docs))
	}

	// Delete virtual document
	err = env.DeleteVirtualDocument(ctx, "/virtual/test.txt")
	if err != nil {
		t.Fatalf("DeleteVirtualDocument() error = %v", err)
	}

	docs, _ = env.ListVirtualDocuments(ctx)
	if len(docs) != 0 {
		t.Errorf("ListVirtualDocuments() should return 0 documents after deletion, got %d", len(docs))
	}
}

func TestLocalEnvironment_ExecuteStream(t *testing.T) {
	tmpDir := t.TempDir()
	env, _ := NewLocalEnvironment(tmpDir, SecurityLevelStandard)

	ctx := context.Background()

	cmd := &Command{
		Name: "echo",
		Args: []string{"stream", "test"},
	}

	reader, err := env.ExecuteStream(ctx, cmd)
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}
	defer reader.Close()

	// Read from stream
	buffer := make([]byte, 100)
	n, err := reader.Read(buffer)
	if err != nil {
		t.Fatalf("Read() from stream error = %v", err)
	}

	output := string(buffer[:n])
	if output != "stream test\n" {
		t.Errorf("ExecuteStream() output = %s, want 'stream test\\n'", output)
	}
}
