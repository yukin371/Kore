// Package storage 提供 SQLite 存储测试
package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/yukin/kore/internal/core"
	"github.com/yukin/kore/internal/session"
)

func TestSQLiteStore(t *testing.T) {
	// 创建临时数据库目录
	tmpDir := t.TempDir()

	// 创建存储实例
	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// 测试数据库初始化
	t.Run("Initialize", func(t *testing.T) {
		// 数据库应该已经被初始化
		if store.db == nil {
			t.Error("Database not initialized")
		}
	})

	// 测试保存和加载会话
	t.Run("SaveAndLoadSession", func(t *testing.T) {
		ctx := context.Background()

		// 创建测试 Agent
		agent := core.NewAgent(nil, nil, nil, tmpDir)

		// 创建测试会话
		sess := session.NewSession("test-session", "Test Session", session.ModeBuild, agent)
		sess.Description = "Test Description"

		// 添加消息
		userMsg := session.Message{
			SessionID: sess.ID,
			Role:      "user",
			Content:   "Hello",
			Timestamp: 0,
		}
		sess.AddMessage(userMsg)

		asstMsg := session.Message{
			SessionID: sess.ID,
			Role:      "assistant",
			Content:   "Hi there",
			Timestamp: 0,
		}
		sess.AddMessage(asstMsg)

		// 保存会话
		if err := store.SaveSession(ctx, sess); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// 加载会话
		loadedSess, err := store.LoadSession(ctx, sess.ID)
		if err != nil {
			t.Fatalf("Failed to load session: %v", err)
		}

		// 验证加载的数据
		if loadedSess.Name != sess.Name {
			t.Errorf("Expected name %s, got %s", sess.Name, loadedSess.Name)
		}

		if loadedSess.Description != sess.Description {
			t.Errorf("Expected description %s, got %s", sess.Description, loadedSess.Description)
		}

		// 验证消息数量
		messages := loadedSess.GetMessages()
		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}
	})

	// 测试列出会话
	t.Run("ListSessions", func(t *testing.T) {
		ctx := context.Background()
		agent := core.NewAgent(nil, nil, nil, tmpDir)

		// 创建多个测试会话
		sess1 := session.NewSession("session-1", "Session 1", session.ModePlan, agent)
		if err := store.SaveSession(ctx, sess1); err != nil {
			t.Fatalf("Failed to save session 1: %v", err)
		}

		sess2 := session.NewSession("session-2", "Session 2", session.ModeGeneral, agent)
		if err := store.SaveSession(ctx, sess2); err != nil {
			t.Fatalf("Failed to save session 2: %v", err)
		}

		// 列出所有会话
		sessions, err := store.ListSessions(ctx)
		if err != nil {
			t.Fatalf("Failed to list sessions: %v", err)
		}

		// 验证会话数量（至少有 2 个）
		if len(sessions) < 2 {
			t.Errorf("Expected at least 2 sessions, got %d", len(sessions))
		}
	})

	// 测试删除会话
	t.Run("DeleteSession", func(t *testing.T) {
		ctx := context.Background()
		agent := core.NewAgent(nil, nil, nil, tmpDir)

		// 创建测试会话
		sess := session.NewSession("delete-test", "To Be Deleted", session.ModeBuild, agent)
		if err := store.SaveSession(ctx, sess); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// 删除会话
		if err := store.DeleteSession(ctx, sess.ID); err != nil {
			t.Fatalf("Failed to delete session: %v", err)
		}

		// 验证会话已删除
		_, err = store.LoadSession(ctx, sess.ID)
		if err == nil {
			t.Error("Expected error when loading deleted session, got nil")
		}
	})

	// 测试消息历史
	t.Run("MessageHistory", func(t *testing.T) {
		ctx := context.Background()
		agent := core.NewAgent(nil, nil, nil, tmpDir)

		// 创建会话并添加消息
		sess := session.NewSession("history-test", "History Test", session.ModeBuild, agent)

		msg1 := session.Message{SessionID: sess.ID, Role: "user", Content: "First message", Timestamp: 0}
		sess.AddMessage(msg1)

		msg2 := session.Message{SessionID: sess.ID, Role: "assistant", Content: "First response", Timestamp: 0}
		sess.AddMessage(msg2)

		msg3 := session.Message{SessionID: sess.ID, Role: "user", Content: "Second message", Timestamp: 0}
		sess.AddMessage(msg3)

		msg4 := session.Message{SessionID: sess.ID, Role: "assistant", Content: "Second response", Timestamp: 0}
		sess.AddMessage(msg4)

		// 保存会话
		if err := store.SaveSession(ctx, sess); err != nil {
			t.Fatalf("Failed to save session: %v", err)
		}

		// 加载会话
		loadedSess, err := store.LoadSession(ctx, sess.ID)
		if err != nil {
			t.Fatalf("Failed to load session: %v", err)
		}

		// 验证消息历史
		messages := loadedSess.GetMessages()
		if len(messages) != 4 {
			t.Errorf("Expected 4 messages, got %d", len(messages))
		}

		// 验证消息顺序
		if messages[0].Content != "First message" {
			t.Errorf("First message mismatch: got %s", messages[0].Content)
		}

		if messages[3].Content != "Second response" {
			t.Errorf("Fourth message mismatch: got %s", messages[3].Content)
		}
	})
}

func TestSQLiteStoreErrors(t *testing.T) {
	t.Run("InvalidPath", func(t *testing.T) {
		_, err := NewSQLiteStore("/invalid/path/db")
		if err == nil {
			t.Error("Expected error for invalid path, got nil")
		}
	})

	t.Run("LoadNonExistentSession", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := NewSQLiteStore(tmpDir)
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer store.Close()

		ctx := context.Background()
		_, err = store.LoadSession(ctx, "non-existent-id")
		if err == nil {
			t.Error("Expected error for non-existent session, got nil")
		}
	})
}

// BenchmarkSQLiteStore 性能测试
func BenchmarkSQLiteStoreSave(b *testing.B) {
	ctx := context.Background()
	tmpDir := b.TempDir()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	agent := core.NewAgent(nil, nil, nil, tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess := session.NewSession(fmt.Sprintf("bench-%d", i), "Benchmark Session", session.ModeBuild, agent)
		msg := session.Message{SessionID: sess.ID, Role: "user", Content: "Benchmark message", Timestamp: 0}
		sess.AddMessage(msg)

		if err := store.SaveSession(ctx, sess); err != nil {
			b.Fatalf("Failed to save session: %v", err)
		}
	}
}

func BenchmarkSQLiteStoreLoad(b *testing.B) {
	ctx := context.Background()
	tmpDir := b.TempDir()

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	agent := core.NewAgent(nil, nil, nil, tmpDir)

	// 预先创建一个会话
	sess := session.NewSession("bench-session", "Benchmark Session", session.ModeBuild, agent)
	msg := session.Message{SessionID: sess.ID, Role: "user", Content: "Benchmark message", Timestamp: 0}
	sess.AddMessage(msg)

	if err := store.SaveSession(ctx, sess); err != nil {
		b.Fatalf("Failed to save session: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.LoadSession(ctx, sess.ID); err != nil {
			b.Fatalf("Failed to load session: %v", err)
		}
	}
}
