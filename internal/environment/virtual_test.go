package environment

import (
	"testing"
	"time"
)

func TestVirtualFileSystem_Create(t *testing.T) {
	vfs := NewVirtualFileSystem()

	content := []byte("test content")
	err := vfs.Create("/test/file.txt", content)

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify document exists
	if !vfs.Exists("/test/file.txt") {
		t.Error("Create() document should exist")
	}

	// Verify content
	read, err := vfs.Read("/test/file.txt")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if string(read) != string(content) {
		t.Errorf("Read() content = %s, want %s", string(read), string(content))
	}
}

func TestVirtualFileSystem_CreateDuplicate(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file.txt", []byte("content"))
	err := vfs.Create("/test/file.txt", []byte("new content"))

	if err == nil {
		t.Error("Create() should return error for duplicate document")
	}
}

func TestVirtualFileSystem_Read(t *testing.T) {
	vfs := NewVirtualFileSystem()

	content := []byte("test content")
	_ = vfs.Create("/test/file.txt", content)

	read, err := vfs.Read("/test/file.txt")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if string(read) != string(content) {
		t.Errorf("Read() content = %s, want %s", string(read), string(content))
	}
}

func TestVirtualFileSystem_ReadNonExistent(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_, err := vfs.Read("/nonexistent/file.txt")
	if err == nil {
		t.Error("Read() should return error for non-existent document")
	}
}

func TestVirtualFileSystem_Update(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file.txt", []byte("old content"))
	err := vfs.Update("/test/file.txt", []byte("new content"))

	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	read, _ := vfs.Read("/test/file.txt")
	if string(read) != "new content" {
		t.Errorf("Update() content = %s, want %s", string(read), "new content")
	}
}

func TestVirtualFileSystem_Delete(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file.txt", []byte("content"))
	err := vfs.Delete("/test/file.txt")

	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if vfs.Exists("/test/file.txt") {
		t.Error("Delete() document should not exist after deletion")
	}
}

func TestVirtualFileSystem_List(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file1.txt", []byte("content1"))
	_ = vfs.Create("/test/file2.txt", []byte("content2"))
	_ = vfs.Create("/other/file3.txt", []byte("content3"))

	list, err := vfs.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(list) != 3 {
		t.Errorf("List() returned %d documents, want 3", len(list))
	}
}

func TestVirtualFileSystem_Metadata(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file.txt", []byte("content"))

	metadata := map[string]interface{}{
		"author": "test",
		"tags":   []string{"doc", "test"},
	}

	err := vfs.SetMetadata("/test/file.txt", metadata)
	if err != nil {
		t.Fatalf("SetMetadata() error = %v", err)
	}

	retrieved, err := vfs.GetMetadata("/test/file.txt")
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if retrieved["author"] != "test" {
		t.Errorf("GetMetadata() author = %v, want %v", retrieved["author"], "test")
	}
}

func TestVirtualFileSystem_Diff(t *testing.T) {
	vfs := NewVirtualFileSystem()

	virtualContent := []byte("line 1\nline 2\nline 3\n")
	diskContent := []byte("line 1\nline 2 modified\nline 3\n")

	_ = vfs.Create("/test/file.txt", virtualContent)

	result, err := vfs.Diff("/test/file.txt", diskContent)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if !result.HasDiff {
		t.Error("Diff() should detect differences")
	}

	if result.Path1 != "/test/file.txt (virtual)" {
		t.Errorf("Diff() Path1 = %s, want %s", result.Path1, "/test/file.txt (virtual)")
	}
}

func TestVirtualFileSystem_Commit(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file.txt", []byte("content"))

	doc, _ := vfs.GetDocument("/test/file.txt")
	if doc.IsCommitted {
		t.Error("New document should not be committed")
	}

	err := vfs.Commit("/test/file.txt")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	doc, _ = vfs.GetDocument("/test/file.txt")
	if !doc.IsCommitted {
		t.Error("Document should be committed after Commit()")
	}
}

func TestVirtualFileSystem_Rollback(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file.txt", []byte("content"))

	err := vfs.Rollback("/test/file.txt")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	if vfs.Exists("/test/file.txt") {
		t.Error("Document should not exist after Rollback()")
	}
}

func TestVirtualFileSystem_Clear(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file1.txt", []byte("content1"))
	_ = vfs.Create("/test/file2.txt", []byte("content2"))

	err := vfs.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	list, _ := vfs.List()
	if len(list) != 0 {
		t.Errorf("Clear() should remove all documents, got %d", len(list))
	}
}

func TestVirtualFileSystem_GetStats(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file1.txt", []byte("content1"))
	_ = vfs.Create("/test/file2.txt", []byte("content2"))

	stats := vfs.GetStats()

	totalDocs := stats["total_documents"].(int)
	if totalDocs != 2 {
		t.Errorf("GetStats() total_documents = %d, want 2", totalDocs)
	}

	uncommitted := stats["uncommitted"].(int)
	if uncommitted != 2 {
		t.Errorf("GetStats() uncommitted = %d, want 2", uncommitted)
	}

	totalSize := stats["total_size"].(int)
	if totalSize != 16 { // "content1" (8) + "content2" (8)
		t.Errorf("GetStats() total_size = %d, want 16", totalSize)
	}
}

func TestVirtualFileSystem_GroupByDirectory(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file1.txt", []byte("content1"))
	_ = vfs.Create("/test/file2.txt", []byte("content2"))
	_ = vfs.Create("/other/file3.txt", []byte("content3"))

	groups := vfs.GroupByDirectory()

	if len(groups) != 2 {
		t.Errorf("GroupByDirectory() returned %d groups, want 2", len(groups))
	}

	testFiles := groups["/test"]
	if len(testFiles) != 2 {
		t.Errorf("Group /test should have 2 files, got %d", len(testFiles))
	}

	otherFiles := groups["/other"]
	if len(otherFiles) != 1 {
		t.Errorf("Group /other should have 1 file, got %d", len(otherFiles))
	}
}

func TestVirtualFileSystem_GetDocument(t *testing.T) {
	vfs := NewVirtualFileSystem()

	content := []byte("test content")
	_ = vfs.Create("/test/file.txt", content)

	doc, err := vfs.GetDocument("/test/file.txt")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}

	if doc.Path != "/test/file.txt" {
		t.Errorf("GetDocument() Path = %s, want /test/file.txt", doc.Path)
	}

	if string(doc.Content) != string(content) {
		t.Errorf("GetDocument() Content = %s, want %s", string(doc.Content), string(content))
	}

	if doc.IsCommitted {
		t.Error("GetDocument() new document should not be committed")
	}

	// Modify returned document should not affect original
	doc.Content = []byte("modified")

	original, _ := vfs.Read("/test/file.txt")
	if string(original) == "modified" {
		t.Error("Modifying returned document should not affect original")
	}
}

func TestVirtualFileSystem_Concurrency(t *testing.T) {
	vfs := NewVirtualFileSystem()

	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(index int) {
			path := "/test/file.txt"
			content := []byte("content")
			_ = vfs.Create(path, content)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have created the document (first write wins)
	if !vfs.Exists("/test/file.txt") {
		t.Error("Document should exist after concurrent writes")
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(index int) {
			_, _ = vfs.Read("/test/file.txt")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestVirtualFileSystem_Timestamps(t *testing.T) {
	vfs := NewVirtualFileSystem()

	_ = vfs.Create("/test/file.txt", []byte("content"))

	doc, _ := vfs.GetDocument("/test/file.txt")

	if doc.CreatedAt.After(time.Now()) {
		t.Error("CreatedAt should not be in the future")
	}

	if doc.UpdatedAt.After(time.Now()) {
		t.Error("UpdatedAt should not be in the future")
	}

	// Update document
	time.Sleep(10 * time.Millisecond)
	_ = vfs.Update("/test/file.txt", []byte("new content"))

	doc, _ = vfs.GetDocument("/test/file.txt")

	if doc.UpdatedAt.Before(doc.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt after update")
	}
}
