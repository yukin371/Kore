package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"github.com/yukin/kore/internal/session"
)

// SQLiteStore SQLite 持久化存储实现
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// 数据库路径
	dbPath := filepath.Join(dataDir, "kore.db")

	// 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(1) // SQLite 不支持并发写入
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// 初始化数据库 Schema
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// initSchema 初始化数据库表结构
func initSchema(db *sql.DB) error {
	schema := `
	-- 会话表
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		agent_mode TEXT NOT NULL,
		status INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		description TEXT,
		tags TEXT,
		statistics TEXT,
		metadata TEXT
	);

	-- 消息表
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		metadata TEXT,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	-- 索引
	CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// 迁移：添加新字段（如果表已存在）
	migration := `
	-- 检查并添加新字段
	ALTER TABLE sessions ADD COLUMN description TEXT;
	ALTER TABLE sessions ADD COLUMN tags TEXT;
	ALTER TABLE sessions ADD COLUMN statistics TEXT;
	`

	// 执行迁移（忽略错误，因为字段可能已存在）
	db.Exec(migration)

	return nil
}

// SaveSession 保存会话元数据
func (s *SQLiteStore) SaveSession(ctx context.Context, sess *session.Session) error {
	// 获取会话数据（使用线程安全的方法）
	id, name, agentMode, status, createdAt, updatedAt, metadata := sess.GetDataForStorage()

	// 获取扩展字段（需要通过方法获取）
	description := sess.GetDescription()
	tags := sess.GetTags()
	statistics := sess.GetStatistics()

	// 序列化元数据
	var metadataJSON []byte
	if metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// 序列化标签
	var tagsJSON []byte
	if len(tags) > 0 {
		var err error
		tagsJSON, err = json.Marshal(tags)
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
	}

	// 序列化统计信息
	var statsJSON []byte
	var err error
	statsJSON, err = json.Marshal(statistics)
	if err != nil {
		return fmt.Errorf("failed to marshal statistics: %w", err)
	}

	// 准备 SQL
	query := `
		INSERT OR REPLACE INTO sessions
		(id, name, agent_mode, status, created_at, updated_at, description, tags, statistics, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// 执行
	_, err = s.db.ExecContext(ctx, query,
		id,
		name,
		string(agentMode),
		status,
		createdAt,
		updatedAt,
		description,
		string(tagsJSON),
		string(statsJSON),
		string(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// LoadSession 加载会话元数据
func (s *SQLiteStore) LoadSession(ctx context.Context, sessionID string) (*session.Session, error) {
	query := `
		SELECT id, name, agent_mode, status, created_at, updated_at, description, tags, statistics, metadata
		FROM sessions
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, sessionID)

	var id, name, agentModeStr string
	var status session.SessionStatus
	var createdAt, updatedAt int64
	var description string
	var tagsJSON, statsJSON, metadataJSON sql.NullString

	err := row.Scan(&id, &name, &agentModeStr, &status, &createdAt, &updatedAt, &description, &tagsJSON, &statsJSON, &metadataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// 反序列化标签
	var tags []string
	if tagsJSON.Valid && tagsJSON.String != "" {
		if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	// 反序列化统计信息
	var statistics session.SessionStats
	if statsJSON.Valid && statsJSON.String != "" {
		if err := json.Unmarshal([]byte(statsJSON.String), &statistics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal statistics: %w", err)
		}
	}

	// 反序列化元数据
	var metadata map[string]interface{}
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// 创建会话对象（注意：Agent 需要外部设置）
	sess := &session.Session{
		ID:         id,
		Name:       name,
		AgentMode:  session.AgentMode(agentModeStr),
		Status:     status,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		Description: description,
		Tags:       tags,
		Statistics: statistics,
		Metadata:   metadata,
		Messages:   make([]session.Message, 0),
	}

	return sess, nil
}

// ListSessions 列出所有会话
func (s *SQLiteStore) ListSessions(ctx context.Context) ([]*session.Session, error) {
	query := `
		SELECT id, name, agent_mode, status, created_at, updated_at, description, tags, statistics, metadata
		FROM sessions
		ORDER BY updated_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*session.Session
	for rows.Next() {
		var id, name, agentModeStr string
		var status session.SessionStatus
		var createdAt, updatedAt int64
		var description string
		var tagsJSON, statsJSON, metadataJSON sql.NullString

		if err := rows.Scan(&id, &name, &agentModeStr, &status, &createdAt, &updatedAt, &description, &tagsJSON, &statsJSON, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		// 反序列化标签
		var tags []string
		if tagsJSON.Valid && tagsJSON.String != "" {
			if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
		}

		// 反序列化统计信息
		var statistics session.SessionStats
		if statsJSON.Valid && statsJSON.String != "" {
			if err := json.Unmarshal([]byte(statsJSON.String), &statistics); err != nil {
				return nil, fmt.Errorf("failed to unmarshal statistics: %w", err)
			}
		}

		// 反序列化元数据
		var metadata map[string]interface{}
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		sess := &session.Session{
			ID:         id,
			Name:       name,
			AgentMode:  session.AgentMode(agentModeStr),
			Status:     status,
			CreatedAt:  createdAt,
			UpdatedAt:  updatedAt,
			Description: description,
			Tags:       tags,
			Statistics: statistics,
			Metadata:   metadata,
			Messages:   make([]session.Message, 0),
		}

		sessions = append(sessions, sess)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// DeleteSession 删除会话
func (s *SQLiteStore) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// SaveMessages 保存会话消息
func (s *SQLiteStore) SaveMessages(ctx context.Context, sessionID string, messages []session.Message) error {
	// 开启事务
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 删除旧消息
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("failed to delete old messages: %w", err)
	}

	// 插入新消息
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO messages (id, session_id, role, content, timestamp, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, msg := range messages {
		// 序列化元数据
		var metadataJSON []byte
		if msg.Metadata != nil {
			metadataJSON, err = json.Marshal(msg.Metadata)
			if err != nil {
				return fmt.Errorf("failed to marshal message metadata: %w", err)
			}
		}

		if _, err := stmt.ExecContext(ctx, msg.ID, msg.SessionID, msg.Role, msg.Content, msg.Timestamp, string(metadataJSON)); err != nil {
			return fmt.Errorf("failed to insert message: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadMessages 加载会话消息
func (s *SQLiteStore) LoadMessages(ctx context.Context, sessionID string) ([]session.Message, error) {
	query := `
		SELECT id, session_id, role, content, timestamp, metadata
		FROM messages
		WHERE session_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}
	defer rows.Close()

	var messages []session.Message
	for rows.Next() {
		var id, sessionID, role, content string
		var timestamp int64
		var metadataJSON sql.NullString

		if err := rows.Scan(&id, &sessionID, &role, &content, &timestamp, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		// 反序列化元数据
		var metadata map[string]interface{}
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal message metadata: %w", err)
			}
		}

		msg := session.Message{
			ID:        id,
			SessionID: sessionID,
			Role:      role,
			Content:   content,
			Timestamp: timestamp,
			Metadata:  metadata,
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// SearchSessions 搜索会话
func (s *SQLiteStore) SearchSessions(ctx context.Context, query string) ([]*session.Session, error) {
	// 搜索会话名称、描述和消息内容
	sqlQuery := `
		SELECT DISTINCT s.id, s.name, s.agent_mode, s.status, s.created_at, s.updated_at, s.description, s.tags, s.statistics, s.metadata
		FROM sessions s
		LEFT JOIN messages m ON s.id = m.session_id
		WHERE s.name LIKE ?
		   OR s.description LIKE ?
		   OR m.content LIKE ?
		ORDER BY s.updated_at DESC
	`

	searchPattern := "%" + query + "%"

	rows, err := s.db.QueryContext(ctx, sqlQuery, searchPattern, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*session.Session
	for rows.Next() {
		var id, name, agentModeStr string
		var status session.SessionStatus
		var createdAt, updatedAt int64
		var description string
		var tagsJSON, statsJSON, metadataJSON sql.NullString

		if err := rows.Scan(&id, &name, &agentModeStr, &status, &createdAt, &updatedAt, &description, &tagsJSON, &statsJSON, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		// 反序列化标签
		var tags []string
		if tagsJSON.Valid && tagsJSON.String != "" {
			if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
		}

		// 反序列化统计信息
		var statistics session.SessionStats
		if statsJSON.Valid && statsJSON.String != "" {
			if err := json.Unmarshal([]byte(statsJSON.String), &statistics); err != nil {
				return nil, fmt.Errorf("failed to unmarshal statistics: %w", err)
			}
		}

		// 反序列化元数据
		var metadata map[string]interface{}
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		sess := &session.Session{
			ID:         id,
			Name:       name,
			AgentMode:  session.AgentMode(agentModeStr),
			Status:     status,
			CreatedAt:  createdAt,
			UpdatedAt:  updatedAt,
			Description: description,
			Tags:       tags,
			Statistics: statistics,
			Metadata:   metadata,
			Messages:   make([]session.Message, 0),
		}

		sessions = append(sessions, sess)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// Close 关闭数据库连接
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
