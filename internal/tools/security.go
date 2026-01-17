// Package tools 提供安全拦截器
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SecurityInterceptor 安全拦截器，防止恶意操作
type SecurityInterceptor struct {
	ProjectRoot  string
	BlockedCmds  []string
	BlockedPaths []string
}

// NewSecurityInterceptor 创建新的安全拦截器
func NewSecurityInterceptor(projectRoot string) *SecurityInterceptor {
	return &SecurityInterceptor{
		ProjectRoot: projectRoot,
		BlockedCmds: []string{
			"rm", "sudo", "shutdown", "format", "del",
			"mkfs", "dd", "reboot", "poweroff", "halt",
			"chmod", "chown", "fdisk", "mount",
		},
		BlockedPaths: []string{
			".git",
			".env",
			"node_modules/.cache",
			".vscode",
			".idea",
		},
	}
}

// ValidatePath 验证路径安全性，防止路径穿越攻击
func (s *SecurityInterceptor) ValidatePath(inputPath string) (string, error) {
	// 构建绝对路径
	absPath, err := filepath.Abs(filepath.Join(s.ProjectRoot, inputPath))
	if err != nil {
		return "", fmt.Errorf("路径解析失败: %w", err)
	}

	// 确保路径在项目根目录内
	// 需要加上路径分隔符，防止 /home/user/project 匹配 /home/user/project-evil
	if !strings.HasPrefix(absPath, s.ProjectRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("安全警告: 路径穿越检测 - 尝试访问项目根目录外的路径: %s", inputPath)
	}

	// 检查敏感路径
	for _, blocked := range s.BlockedPaths {
		if strings.Contains(absPath, blocked) {
			return "", fmt.Errorf("安全警告: 禁止访问敏感路径: %s", blocked)
		}
	}

	return absPath, nil
}

// ValidateCommand 验证命令安全性，防止命令注入
func (s *SecurityInterceptor) ValidateCommand(cmd string) error {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return fmt.Errorf("命令不能为空")
	}

	// 解析命令
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("命令解析失败")
	}

	baseCmd := parts[0]

	// 黑名单检查
	for _, blocked := range s.BlockedCmds {
		if baseCmd == blocked {
			return fmt.Errorf("安全警告: 禁止执行危险命令: %s", baseCmd)
		}
	}

	// 检查命令注入特征
	dangerousPatterns := []string{
		"&&", "|", ";", "`", "$(", "$(",
		">", "<", "&>", "&<",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(cmd, pattern) {
			// 允许管道和重定向（但要在白名单中）
			if pattern == "|" || pattern == ">" {
				continue
			}
			return fmt.Errorf("安全警告: 检测到潜在的命令注入模式: %s", pattern)
		}
	}

	return nil
}

// AddBlockedCmd 添加到命令黑名单
func (s *SecurityInterceptor) AddBlockedCmd(cmd string) {
	s.BlockedCmds = append(s.BlockedCmds, cmd)
}

// AddBlockedPath 添加到路径黑名单
func (s *SecurityInterceptor) AddBlockedPath(path string) {
	s.BlockedPaths = append(s.BlockedPaths, path)
}
