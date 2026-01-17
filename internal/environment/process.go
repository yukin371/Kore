package environment

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"
)

// ProcessManager 进程管理器
type ProcessManager struct {
	mu       sync.RWMutex
	processes map[int]*ProcessInfo
	nextPID   int
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	Process   *Process
	Cmd       *exec.Cmd
	LogFile   *os.File
	Cancel    context.CancelFunc
	StartTime time.Time
}

// NewProcessManager 创建进程管理器
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[int]*ProcessInfo),
		nextPID:   1000, // 从 1000 开始避免与系统 PID 冲突
	}
}

// StartProcess 启动后台进程
func (pm *ProcessManager) StartProcess(ctx context.Context, cmd *Command, workingDir string, security *SecurityInterceptor) (*Process, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 创建可取消的上下文
	cmdCtx, cancel := context.WithCancel(context.Background())

	// 创建命令
	execCmd := exec.CommandContext(cmdCtx, cmd.Name, cmd.Args...)
	execCmd.Dir = workingDir

	// 设置环境变量
	if cmd.Env != nil {
		safeEnv := security.SanitizeEnvironment(cmd.Env)
		execCmd.Env = mergeEnv(os.Environ(), safeEnv)
	}

	// 创建日志文件
	logPath := filepath.Join(workingDir, fmt.Sprintf(".kore_logs_%d_%s.log", pm.nextPID, time.Now().Format("20060102_150405")))
	logFile, err := os.Create(logPath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}

	// 设置输出
	execCmd.Stdout = logFile
	execCmd.Stderr = logFile

	// 启动命令
	if err := execCmd.Start(); err != nil {
		cancel()
		logFile.Close()
		return nil, fmt.Errorf("启动进程失败: %w", err)
	}

	// 获取实际 PID
	actualPID := execCmd.Process.Pid

	// 创建进程记录
	process := &Process{
		PID:       pm.nextPID, // 使用虚拟 PID
		Command:   cmd,
		Status:    "running",
		StartTime: time.Now(),
		LogPath:   logPath,
	}

	processInfo := &ProcessInfo{
		Process:   process,
		Cmd:       execCmd,
		LogFile:   logFile,
		Cancel:    cancel,
		StartTime: time.Now(),
	}

	pm.processes[pm.nextPID] = processInfo

	// 启动监控 goroutine
	go pm.monitorProcess(pm.nextPID, actualPID, cmdCtx, execCmd, logFile)

	pm.nextPID++

	return process, nil
}

// monitorProcess 监控进程状态
func (pm *ProcessManager) monitorProcess(virtualPID, actualPID int, ctx context.Context, cmd *exec.Cmd, logFile *os.File) {
	err := cmd.Wait()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if info, exists := pm.processes[virtualPID]; exists {
		info.Process.EndTime = time.Now()
		info.Process.Status = "stopped"

		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					info.Process.ExitCode = status.ExitStatus()
				}
				info.Process.Status = "failed"
			}
		} else {
			info.Process.ExitCode = 0
		}

		logFile.Close()
	}
}

// KillProcess 终止进程
func (pm *ProcessManager) KillProcess(ctx context.Context, pid int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	info, exists := pm.processes[pid]
	if !exists {
		return fmt.Errorf("进程不存在: %d", pid)
	}

	// 取消上下文
	if info.Cancel != nil {
		info.Cancel()
	}

	// 尝试优雅终止
	if info.Cmd.Process != nil {
		// 先发送 SIGTERM
		if err := info.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// 如果 SIGTERM 失败，使用 SIGKILL
			if err := info.Cmd.Process.Kill(); err != nil {
				return fmt.Errorf("终止进程失败: %w", err)
			}
		}

		// 等待进程结束
		done := make(chan error, 1)
		go func() {
			_, err := info.Cmd.Process.Wait()
			done <- err
		}()

		select {
		case <-done:
			// 进程已结束
		case <-time.After(5 * time.Second):
			// 超时，强制终止
			info.Cmd.Process.Kill()
		}
	}

	info.Process.Status = "killed"
	info.Process.EndTime = time.Now()

	return nil
}

// GetStatus 获取进程状态
func (pm *ProcessManager) GetStatus(ctx context.Context, pid int) (*ProcessStatus, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	info, exists := pm.processes[pid]
	if !exists {
		return nil, fmt.Errorf("进程不存在: %d", pid)
	}

	status := &ProcessStatus{
		PID:    pid,
		Status: info.Process.Status,
	}

	// 如果进程正在运行，获取实时统计信息
	if info.Process.Status == "running" && info.Cmd.Process != nil {
		actualPID := info.Cmd.Process.Pid

		// 使用 gopsutil 获取进程信息
		p, err := process.NewProcess(int32(actualPID))
		if err == nil {
			// CPU 使用率
			if cpuPercent, err := p.PercentWithContext(ctx, 0); err == nil {
				status.CPUPercent = cpuPercent
			}

			// 内存使用
			if memInfo, err := p.MemoryInfo(); err == nil {
				status.Memory = memInfo.RSS
			}

			// 运行时长
			status.Uptime = time.Since(info.StartTime)
		}
	}

	return status, nil
}

// ListProcesses 列出所有进程
func (pm *ProcessManager) ListProcesses() []*Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	processes := make([]*Process, 0, len(pm.processes))
	for _, info := range pm.processes {
		processes = append(processes, info.Process)
	}

	return processes
}

// Cleanup 清理已结束的进程记录
func (pm *ProcessManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for pid, info := range pm.processes {
		if info.Process.Status == "stopped" || info.Process.Status == "failed" || info.Process.Status == "killed" {
			// 只删除超过 1 小时的已结束进程记录
			if time.Since(info.Process.EndTime) > time.Hour {
				delete(pm.processes, pid)
			}
		}
	}
}
