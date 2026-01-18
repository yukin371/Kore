package storage

import "errors"

// Storage errors
var (
	// ErrSessionNotFound 会话未找到
	ErrSessionNotFound = errors.New("session not found")

	// ErrMessageNotFound 消息未找到
	ErrMessageNotFound = errors.New("message not found")

	// ErrStorageClosed 存储已关闭
	ErrStorageClosed = errors.New("storage closed")

	// ErrInvalidData 无效数据
	ErrInvalidData = errors.New("invalid data")
)
