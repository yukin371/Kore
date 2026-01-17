package environment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// LocalEnvironment 本地环境实现
type LocalEnvironment struct {
	workingDir      string
	security        *SecurityInterceptor
	processManager  *ProcessManager
	virtualFS       *VirtualFileSystem
}

// NewLocalEnvironment 创建本地环境实例
func NewLocalEnvironment(workingDir string, securityLevel SecurityLevel) (*LocalEnvironment, error) {
	// 转换为绝对路径
	absDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("无法解析工作目录: %w", err)
	}

	// 检查目录是否存在
	if _, err := os.Stat(absDir); err != nil {
		return nil, fmt.Errorf("工作目录不存在: %w", err)
	}

	security := NewSecurityInterceptor(absDir, securityLevel)
	processManager := NewProcessManager()
	virtualFS := NewVirtualFileSystem()

	return &LocalEnvironment{
		workingDir:     absDir,
		security:       security,
		processManager: processManager,
		virtualFS:      virtualFS,
	}, nil
}

// Execute 执行命令并返回结果
func (env *LocalEnvironment) Execute(ctx context.Context, cmd *Command) (*Result, error) {
	// 安全验证
	if err := env.security.ValidateCommand(cmd.Name, cmd.Args); err != nil {
		return nil, fmt.Errorf("命令安全验证失败: %w", err)
	}

	// 创建命令
	execCmd := exec.CommandContext(ctx, cmd.Name, cmd.Args...)

	// 设置工作目录
	execCmd.Dir = env.workingDir

	// 设置环境变量
	if cmd.Env != nil {
		safeEnv := env.security.SanitizeEnvironment(cmd.Env)
		execCmd.Env = mergeEnv(os.Environ(), safeEnv)
	}

	// 执行命令
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	// 设置超时
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	err := execCmd.Run()

	// 获取退出码
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// 对于超时等错误，仍然返回结果（包含已输出的内容）
			// 不返回 nil，这样调用者可以获得部分输出
			return &Result{
				ExitCode: -1, // 使用 -1 表示异常终止（如超时）
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
			}, fmt.Errorf("命令执行失败: %w", err)
		}
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

// ExecuteStream 流式执行命令
func (env *LocalEnvironment) ExecuteStream(ctx context.Context, cmd *Command) (io.ReadCloser, error) {
	// 安全验证
	if err := env.security.ValidateCommand(cmd.Name, cmd.Args); err != nil {
		return nil, fmt.Errorf("命令安全验证失败: %w", err)
	}

	// 创建命令
	execCmd := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	execCmd.Dir = env.workingDir

	// 设置环境变量
	if cmd.Env != nil {
		safeEnv := env.security.SanitizeEnvironment(cmd.Env)
		execCmd.Env = mergeEnv(os.Environ(), safeEnv)
	}

	// 创建管道
	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建输出管道失败: %w", err)
	}

	stderr, err := execCmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建错误管道失败: %w", err)
	}

	// 启动命令
	if err := execCmd.Start(); err != nil {
		return nil, fmt.Errorf("启动命令失败: %w", err)
	}

	// 合并 stdout 和 stderr
	reader, writer := io.Pipe()
	go func() {
		defer writer.Close()
		io.Copy(writer, io.MultiReader(stdout, stderr))
		execCmd.Wait()
	}()

	return reader, nil
}

// ReadFile 读取文件
func (env *LocalEnvironment) ReadFile(ctx context.Context, path string) ([]byte, error) {
	// 安全验证
	if err := env.security.ValidatePath(path); err != nil {
		return nil, fmt.Errorf("路径验证失败: %w", err)
	}

	// 如果是相对路径，转换为绝对路径
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(env.workingDir, path)
	}

	// 读取文件
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return content, nil
}

// WriteFile 写入文件
func (env *LocalEnvironment) WriteFile(ctx context.Context, path string, content []byte, opts *WriteOptions) error {
	// 安全验证
	if err := env.security.ValidatePath(path); err != nil {
		return fmt.Errorf("路径验证失败: %w", err)
	}

	// 验证文件名
	if err := env.security.ValidateFilename(filepath.Base(path)); err != nil {
		return fmt.Errorf("文件名验证失败: %w", err)
	}

	// 转换为绝对路径
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(env.workingDir, path)
	}

	// 创建缺失的目录
	if opts != nil && opts.CreateMissingDirs {
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
	}

	// 备份原文件
	if opts != nil && opts.Backup {
		if _, err := os.Stat(absPath); err == nil {
			backupPath := absPath + ".bak"
			if err := copyFile(absPath, backupPath); err != nil {
				return fmt.Errorf("备份文件失败: %w", err)
			}
		}
	}

	// 写入文件
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// Diff 对比两个文件的差异
func (env *LocalEnvironment) Diff(ctx context.Context, path1, path2 string) (*DiffResult, error) {
	// 验证路径
	if err := env.security.ValidatePath(path1); err != nil {
		return nil, fmt.Errorf("路径1验证失败: %w", err)
	}
	if err := env.security.ValidatePath(path2); err != nil {
		return nil, fmt.Errorf("路径2验证失败: %w", err)
	}

	// 转换为绝对路径
	absPath1 := path1
	if !filepath.IsAbs(path1) {
		absPath1 = filepath.Join(env.workingDir, path1)
	}

	absPath2 := path2
	if !filepath.IsAbs(path2) {
		absPath2 = filepath.Join(env.workingDir, path2)
	}

	// 读取文件内容
	content1, err := os.ReadFile(absPath1)
	if err != nil {
		return nil, fmt.Errorf("读取文件1失败: %w", err)
	}

	content2, err := os.ReadFile(absPath2)
	if err != nil {
		return nil, fmt.Errorf("读取文件2失败: %w", err)
	}

	// 使用 go-diff 生成差异
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(content1), string(content2), false)

	// 构建结果
	result := &DiffResult{
		Path1:   path1,
		Path2:   path2,
		HasDiff: len(diffs) > 0 && !(len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual),
	}

	// 生成统一格式差异
	var unified strings.Builder
	unified.WriteString(fmt.Sprintf("--- %s\n", path1))
	unified.WriteString(fmt.Sprintf("+++ %s\n", path2))

	hunks := make([]Hunks, 0)
	lineNum := 1

	for _, d := range diffs {
		if d.Type == diffmatchpatch.DiffEqual { // Equal
			lines := strings.Split(d.Text, "\n")
			for _, line := range lines {
				if line != "" {
					unified.WriteString(fmt.Sprintf(" %s\n", line))
					lineNum++
				}
			}
		} else if d.Type == diffmatchpatch.DiffDelete { // Deleted
			lines := strings.Split(d.Text, "\n")
			hunk := Hunks{
				OldStart: lineNum,
				OldCount: len(lines) - 1,
				NewStart: lineNum,
				NewCount: 0,
			}
			for _, line := range lines {
				if line != "" {
					unified.WriteString(fmt.Sprintf("-%s\n", line))
					hunk.Lines = append(hunk.Lines, "-"+line)
					lineNum++
				}
			}
			hunks = append(hunks, hunk)
		} else if d.Type == diffmatchpatch.DiffInsert { // Inserted
			lines := strings.Split(d.Text, "\n")
			hunk := Hunks{
				OldStart: lineNum,
				OldCount: 0,
				NewStart: lineNum,
				NewCount: len(lines) - 1,
			}
			for _, line := range lines {
				if line != "" {
					unified.WriteString(fmt.Sprintf("+%s\n", line))
					hunk.Lines = append(hunk.Lines, "+"+line)
				}
			}
			hunks = append(hunks, hunk)
		}
	}

	result.Unified = unified.String()
	result.Hunks = hunks

	return result, nil
}

// StartBackgroundProcess 启动后台进程
func (env *LocalEnvironment) StartBackgroundProcess(ctx context.Context, cmd *Command) (*Process, error) {
	// 安全验证
	if err := env.security.ValidateCommand(cmd.Name, cmd.Args); err != nil {
		return nil, fmt.Errorf("命令安全验证失败: %w", err)
	}

	// 使用进程管理器启动
	process, err := env.processManager.StartProcess(ctx, cmd, env.workingDir, env.security)
	if err != nil {
		return nil, fmt.Errorf("启动后台进程失败: %w", err)
	}

	return process, nil
}

// KillProcess 终止进程
func (env *LocalEnvironment) KillProcess(ctx context.Context, pid int) error {
	return env.processManager.KillProcess(ctx, pid)
}

// GetProcessStatus 获取进程状态
func (env *LocalEnvironment) GetProcessStatus(ctx context.Context, pid int) (*ProcessStatus, error) {
	return env.processManager.GetStatus(ctx, pid)
}

// CreateVirtualDocument 创建虚拟文档
func (env *LocalEnvironment) CreateVirtualDocument(ctx context.Context, path string, content []byte) error {
	return env.virtualFS.Create(path, content)
}

// ReadVirtualDocument 读取虚拟文档
func (env *LocalEnvironment) ReadVirtualDocument(ctx context.Context, path string) ([]byte, error) {
	return env.virtualFS.Read(path)
}

// UpdateVirtualDocument 更新虚拟文档
func (env *LocalEnvironment) UpdateVirtualDocument(ctx context.Context, path string, content []byte) error {
	return env.virtualFS.Update(path, content)
}

// DeleteVirtualDocument 删除虚拟文档
func (env *LocalEnvironment) DeleteVirtualDocument(ctx context.Context, path string) error {
	return env.virtualFS.Delete(path)
}

// ListVirtualDocuments 列出虚拟文档
func (env *LocalEnvironment) ListVirtualDocuments(ctx context.Context) ([]string, error) {
	return env.virtualFS.List()
}

// SetWorkingDir 设置工作目录
func (env *LocalEnvironment) SetWorkingDir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("无法解析目录: %w", err)
	}

	if _, err := os.Stat(absDir); err != nil {
		return fmt.Errorf("目录不存在: %w", err)
	}

	env.workingDir = absDir
	env.security.AddAllowedDir(absDir)

	return nil
}

// GetWorkingDir 获取工作目录
func (env *LocalEnvironment) GetWorkingDir() string {
	return env.workingDir
}

// mergeEnv 合并环境变量
func mergeEnv(base []string, override map[string]string) []string {
	result := make([]string, 0, len(base)+len(override))

	// 复制基础环境变量
	baseMap := make(map[string]string)
	for _, env := range base {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			baseMap[parts[0]] = parts[1]
		}
	}

	// 应用覆盖
	for key, value := range override {
		baseMap[key] = value
	}

	// 转换为切片
	for key, value := range baseMap {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}

	return result
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, 0644)
}
