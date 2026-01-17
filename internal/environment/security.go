package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SecurityInterceptor 安全拦截器
// 提供路径验证、命令过滤等安全机制
type SecurityInterceptor struct {
	allowedDirs      []string        // 允许访问的目录列表
	allowedCommands  map[string]bool // 允许执行的命令白名单
	dangerousCommands []string       // 危险命令黑名单
	securityLevel    SecurityLevel   // 安全级别
}

// NewSecurityInterceptor 创建安全拦截器
func NewSecurityInterceptor(workingDir string, level SecurityLevel) *SecurityInterceptor {
	si := &SecurityInterceptor{
		allowedDirs:     []string{workingDir},
		allowedCommands: make(map[string]bool),
		securityLevel:   level,
	}

	// 根据安全级别设置默认值
	si.setupDefaults()

	return si
}

// setupDefaults 设置默认安全配置
func (si *SecurityInterceptor) setupDefaults() {
	switch si.securityLevel {
	case SecurityLevelStrict:
		// 严格模式：只允许安全命令
		safeCommands := []string{
			"git", "go", "python", "node", "npm", "cargo", "rustc",
			"ls", "dir", "cd", "pwd", "echo", "cat", "head", "tail",
			"grep", "find", "wc", "sort", "uniq", "cut",
			"mkdir", "touch", "cp", "mv",
			"git status", "git diff", "git log", "git add",
			"go build", "go test", "go run", "go mod",
		}
		for _, cmd := range safeCommands {
			si.allowedCommands[cmd] = true
		}

	case SecurityLevelStandard:
		// 标准模式：允许常用开发工具
		standardCommands := []string{
			"git", "go", "python", "node", "npm", "cargo", "rustc",
			"make", "cmake", "gcc", "clang",
			"ls", "dir", "cd", "pwd", "echo", "cat", "head", "tail",
			"grep", "find", "sed", "awk", "wc", "sort", "uniq", "cut",
			"mkdir", "touch", "cp", "mv", "rm",
			"curl", "wget", "ssh", "scp",
			"ping", // 网络诊断工具
		}
		for _, cmd := range standardCommands {
			si.allowedCommands[cmd] = true
		}

	case SecurityLevelPermissive:
		// 宽松模式：允许大多数命令
		// 除了特别危险的命令外，都允许
	}

	// 危险命令黑名单（所有级别都禁止）
	si.dangerousCommands = []string{
		"rm -rf /", "rm -rf /*", "rm -rf \\",
		"mkfs", "format", "fdisk",
		"dd if=/dev/zero", "dd if=/dev/random",
		":(){ :|:& };:",  // fork bomb
		"chmod 000", "chown",
		"shutdown", "reboot", "poweroff", "halt",
		"mv /", "mv /*",
	}
}

// ValidatePath 验证路径是否安全
// 防止路径遍历攻击（../../../etc/passwd）
func (si *SecurityInterceptor) ValidatePath(path string) error {
	// 清理路径
	cleanPath := filepath.Clean(path)

	// 检查路径遍历攻击
	// 如果包含 .. 则需要验证
	if strings.Contains(cleanPath, "..") {
		// 转换为绝对路径以解析 ..
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return fmt.Errorf("无法解析路径: %w", err)
		}

		// 检查是否在允许的目录内
		for _, allowedDir := range si.allowedDirs {
			allowedAbs, err := filepath.Abs(allowedDir)
			if err != nil {
				continue
			}

			// 检查路径是否在允许的目录下
			if strings.HasPrefix(absPath, allowedAbs+string(os.PathSeparator)) || absPath == allowedAbs {
				return nil
			}
		}

		return fmt.Errorf("路径访问被拒绝: %s 尝试路径遍历", path)
	}

	// 如果是绝对路径，需要验证是否在允许的目录内
	if filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return fmt.Errorf("无法解析路径: %w", err)
		}

		// 检查是否在允许的目录内
		for _, allowedDir := range si.allowedDirs {
			allowedAbs, err := filepath.Abs(allowedDir)
			if err != nil {
				continue
			}

			// 检查路径是否在允许的目录下
			if strings.HasPrefix(absPath, allowedAbs+string(os.PathSeparator)) || absPath == allowedAbs {
				return nil
			}
		}

		return fmt.Errorf("路径访问被拒绝: %s 不在允许的目录列表中", path)
	}

	// 相对路径且不包含 .. （例如 "test/file.txt"）是安全的
	// 它会被 LocalEnvironment 转换为工作目录下的路径
	return nil
}

// ValidateCommand 验证命令是否安全
func (si *SecurityInterceptor) ValidateCommand(cmd string, args []string) error {
	// 检查黑名单
	fullCmd := cmd
	if len(args) > 0 {
		fullCmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}

	for _, dangerous := range si.dangerousCommands {
		if strings.Contains(fullCmd, dangerous) {
			return fmt.Errorf("危险命令被拦截: %s", dangerous)
		}
	}

	// 检查白名单（如果是严格模式或标准模式）
	if si.securityLevel == SecurityLevelStrict || si.securityLevel == SecurityLevelStandard {
		// 检查基础命令
		if !si.allowedCommands[cmd] {
			// 检查是否是带参数的命令
			allowed := false
			for allowedCmd := range si.allowedCommands {
				if strings.HasPrefix(cmd, allowedCmd) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("命令不在白名单中: %s", cmd)
			}
		}
	}

	// 检查命令注入（通过特殊字符）
	cmdInjectionPattern := regexp.MustCompile(`[;&|]`)
	if cmdInjectionPattern.MatchString(cmd) {
		return fmt.Errorf("检测到命令注入尝试: %s", cmd)
	}

	for _, arg := range args {
		if cmdInjectionPattern.MatchString(arg) {
			return fmt.Errorf("检测到命令注入尝试在参数中: %s", arg)
		}
	}

	return nil
}

// AddAllowedDir 添加允许访问的目录
func (si *SecurityInterceptor) AddAllowedDir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("无法解析目录: %w", err)
	}

	// 检查目录是否存在
	if _, err := os.Stat(absDir); err != nil {
		return fmt.Errorf("目录不存在: %w", err)
	}

	si.allowedDirs = append(si.allowedDirs, absDir)
	return nil
}

// AddAllowedCommand 添加允许执行的命令
func (si *SecurityInterceptor) AddAllowedCommand(cmd string) {
	si.allowedCommands[cmd] = true
}

// RemoveAllowedCommand 移除允许执行的命令
func (si *SecurityInterceptor) RemoveAllowedCommand(cmd string) {
	delete(si.allowedCommands, cmd)
}

// AddDangerousCommand 添加危险命令到黑名单
func (si *SecurityInterceptor) AddDangerousCommand(cmd string) {
	si.dangerousCommands = append(si.dangerousCommands, cmd)
}

// SetSecurityLevel 设置安全级别
func (si *SecurityInterceptor) SetSecurityLevel(level SecurityLevel) {
	si.securityLevel = level
	si.setupDefaults()
}

// SanitizeEnvironment 清理环境变量
// 移除潜在危险的环境变量
func (si *SecurityInterceptor) SanitizeEnvironment(env map[string]string) map[string]string {
	safeEnv := make(map[string]string)

	// 危险环境变量黑名单
	dangerousVars := []string{
		"PATH",    // 可能被篡改
		"LD_LIBRARY_PATH",
		"IFS",
	}

	// 允许的安全环境变量
	safeVars := []string{
		"HOME", "USER", "SHELL", "LANG", "LC_ALL",
		"GOOS", "GOARCH", "GOPATH", "GOROOT",
		"NODE_PATH", "PYTHONPATH",
	}

	// 复制安全的环境变量
	for key, value := range env {
		isSafe := true
		for _, dangerous := range dangerousVars {
			if key == dangerous {
				isSafe = false
				break
			}
		}

		if isSafe {
			// 对于严格模式，只允许明确安全的变量
			if si.securityLevel == SecurityLevelStrict {
				allowed := false
				for _, safeVar := range safeVars {
					if key == safeVar {
						allowed = true
						break
					}
				}
				if allowed {
					safeEnv[key] = value
				}
			} else {
				safeEnv[key] = value
			}
		}
	}

	return safeEnv
}

// ValidateFilename 验证文件名是否安全
func (si *SecurityInterceptor) ValidateFilename(filename string) error {
	// 检查特殊字符
	specialChars := regexp.MustCompile(`[<>:"|?*\x00-\x1f]`)
	if specialChars.MatchString(filename) {
		return fmt.Errorf("文件名包含非法字符: %s", filename)
	}

	// 检查路径遍历
	if strings.Contains(filename, "..") {
		return fmt.Errorf("文件名包含路径遍历序列: %s", filename)
	}

	// 检查保留名称（Windows）
	reservedNames := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}
	base := strings.ToUpper(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	for _, reserved := range reservedNames {
		if base == reserved {
			return fmt.Errorf("文件名是系统保留名称: %s", filename)
		}
	}

	return nil
}
