package environment

import (
	"context"
	"io"
	"time"
)

// Manager 定义环境管理器接口
// 提供安全的命令执行和文件操作能力
type Manager interface {
	// 命令执行
	Execute(ctx context.Context, cmd *Command) (*Result, error)
	ExecuteStream(ctx context.Context, cmd *Command) (io.ReadCloser, error)

	// 文件操作
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, path string, content []byte, opts *WriteOptions) error
	Diff(ctx context.Context, path1, path2 string) (*DiffResult, error)

	// 进程管理
	StartBackgroundProcess(ctx context.Context, cmd *Command) (*Process, error)
	KillProcess(ctx context.Context, pid int) error
	GetProcessStatus(ctx context.Context, pid int) (*ProcessStatus, error)

	// 虚拟文件系统
	CreateVirtualDocument(ctx context.Context, path string, content []byte) error
	ReadVirtualDocument(ctx context.Context, path string) ([]byte, error)
	UpdateVirtualDocument(ctx context.Context, path string, content []byte) error
	DeleteVirtualDocument(ctx context.Context, path string) error
	ListVirtualDocuments(ctx context.Context) ([]string, error)

	// 设置工作目录
	SetWorkingDir(dir string) error
	GetWorkingDir() string
}

// Command 表示要执行的命令
type Command struct {
	Name    string            // 命令名称
	Args    []string          // 命令参数
	Env     map[string]string // 环境变量
	Timeout time.Duration     // 超时时间
}

// Result 表示命令执行结果
type Result struct {
	ExitCode int    // 退出码
	Stdout   string // 标准输出
	Stderr   string // 标准错误输出
}

// WriteOptions 文件写入选项
type WriteOptions struct {
	CreateMissingDirs bool // 是否创建缺失的目录
	Confirm           bool // 是否需要确认（用于虚拟文档）
	Backup            bool // 是否备份原文件
}

// DiffResult 文件差异结果
type DiffResult struct {
	Path1    string   // 第一个文件路径
	Path2    string   // 第二个文件路径
	Unified  string   // 统一格式差异
	Hunks    []Hunk   // 差异块
	HasDiff  bool     // 是否有差异
}

// Hunks 表示一个差异块
type Hunks struct {
	OldStart int    // 旧文件起始行
	OldCount int    // 旧文件行数
	NewStart int    // 新文件起始行
	NewCount int    // 新文件行数
	Lines    []string // 差异行
}

// Process 表示后台进程
type Process struct {
	PID        int       // 进程 ID
	Command    *Command  // 执行的命令
	Status     string    // 状态: running, stopped, failed
	StartTime  time.Time // 启动时间
	EndTime    time.Time // 结束时间
	ExitCode   int       // 退出码
	LogPath    string    // 日志路径
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	PID        int       // 进程 ID
	Status     string    // 状态
	CPUPercent float64   // CPU 使用率
	Memory     uint64    // 内存使用（字节）
	Uptime     time.Duration // 运行时长
}

// SecurityLevel 安全级别
type SecurityLevel int

const (
	SecurityLevelStrict SecurityLevel = iota // 严格模式：所有操作都需要验证
	SecurityLevelStandard                    // 标准模式：基本安全检查
	SecurityLevelPermissive                  // 宽松模式：最小限制
)
