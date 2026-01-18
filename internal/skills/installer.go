package skills

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Installer Skill 安装器
type Installer struct {
	registry *Registry
	runtime  *Runtime
	config   *InstallerConfig
}

// InstallerConfig 安装器配置
type InstallerConfig struct {
	InstallDir    string `json:"install_dir"`    // 安装目录
	TempDir       string `json:"temp_dir"`       // 临时目录
	VerifySig     bool   `json:"verify_sig"`     // 验证签名
	AutoEnable    bool   `json:"auto_enable"`    // 安装后自动启用
	AllowUpgrade  bool   `json:"allow_upgrade"`  // 允许升级
	MaxRetries    int    `json:"max_retries"`    // 最大重试次数
	Timeout       time.Duration `json:"timeout"` // 超时时间
}

// NewInstaller 创建安装器
func NewInstaller(registry *Registry, runtime *Runtime, config *InstallerConfig) *Installer {
	if config == nil {
		config = &InstallerConfig{
			InstallDir:   filepath.Join(registry.dataDir, "installed"),
			TempDir:      os.TempDir(),
			VerifySig:    true,
			AutoEnable:   true,
			AllowUpgrade: true,
			MaxRetries:   3,
			Timeout:      5 * time.Minute,
		}
	}

	return &Installer{
		registry: registry,
		runtime:  runtime,
		config:   config,
	}
}

// InstallFromURL 从 URL 安装 Skill
func (in *Installer) InstallFromURL(ctx context.Context, url string) (*SkillManifest, error) {
	// 下载文件
	tempFile, err := in.download(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer os.Remove(tempFile)

	// 安装
	return in.InstallFromFile(ctx, tempFile)
}

// InstallFromFile 从文件安装 Skill
func (in *Installer) InstallFromFile(ctx context.Context, filePath string) (*SkillManifest, error) {
	// 解压
	extractDir, err := in.extract(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract: %w", err)
	}
	defer os.RemoveAll(extractDir)

	// 读取清单
	manifest, err := in.loadManifest(extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// 验证清单
	if err := in.validateManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// 复制到安装目录
	installDir := in.getInstallDir(manifest.ID)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create install dir: %w", err)
	}

	// 复制文件
	if err := in.copyFiles(extractDir, installDir); err != nil {
		// 安装失败，回滚
		os.RemoveAll(installDir)
		return nil, fmt.Errorf("failed to copy files: %w", err)
	}

	// 注册到 Registry
	if err := in.registry.Register(ctx, manifest); err != nil {
		// 注册失败，回滚
		os.RemoveAll(installDir)
		return nil, fmt.Errorf("failed to register: %w", err)
	}

	// 自动启用（如果配置允许）
	if in.config.AutoEnable {
		if err := in.registry.Enable(ctx, manifest.ID); err != nil {
			return nil, fmt.Errorf("failed to enable skill: %w", err)
		}
	}

	return manifest, nil
}

// Uninstall 卸载 Skill
func (in *Installer) Uninstall(ctx context.Context, id SkillID) error {
	// 检查是否存在
	_, err := in.registry.Get(id)
	if err != nil {
		return fmt.Errorf("skill not found: %w", err)
	}

	// 如果已加载，先卸载
	if _, ok := in.runtime.skills[id]; ok {
		if err := in.runtime.Unload(ctx, id); err != nil {
			return fmt.Errorf("failed to unload skill: %w", err)
		}
	}

	// 从 Registry 注销
	if err := in.registry.Unregister(ctx, id); err != nil {
		return fmt.Errorf("failed to unregister: %w", err)
	}

	// 删除安装目录
	installDir := in.getInstallDir(id)
	if err := os.RemoveAll(installDir); err != nil {
		// 记录错误但不中断
		fmt.Fprintf(os.Stderr, "Warning: failed to remove install directory %s: %v\n", installDir, err)
	}

	return nil
}

// Upgrade 升级 Skill
func (in *Installer) Upgrade(ctx context.Context, id SkillID, newPackage string) error {
	// 检查是否允许升级
	if !in.config.AllowUpgrade {
		return fmt.Errorf("upgrade is disabled")
	}

	// 获取当前清单
	oldManifest, err := in.registry.Get(id)
	if err != nil {
		return fmt.Errorf("skill not found: %w", err)
	}

	// 创建备份
	backupDir := in.createBackup(oldManifest)
	defer os.RemoveAll(backupDir)

	// 安装新版本
	newManifest, err := in.InstallFromFile(ctx, newPackage)
	if err != nil {
		// 安装失败，恢复备份
		in.restoreBackup(oldManifest.ID, backupDir)
		return fmt.Errorf("failed to install new version: %w", err)
	}

	// 验证升级
	if newManifest.ID != oldManifest.ID {
		// ID 不匹配，回滚
		in.Uninstall(ctx, newManifest.ID)
		in.restoreBackup(oldManifest.ID, backupDir)
		in.registry.Register(ctx, oldManifest)
		return fmt.Errorf("skill ID mismatch: expected %s, got %s", oldManifest.ID, newManifest.ID)
	}

	return nil
}

// download 下载文件到临时目录
func (in *Installer) download(ctx context.Context, url string) (string, error) {
	// 创建临时文件
	tempFile, err := os.CreateTemp(in.config.TempDir, "skill-download-*.zip")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// 下载文件
	hash := sha256.New()
	writer := io.MultiWriter(tempFile, hash)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return "", err
	}

	// 计算校验和
	checksum := hex.EncodeToString(hash.Sum(nil))

	// TODO: 验证签名或校验和
	_ = checksum

	return tempFile.Name(), nil
}

// extract 解压文件
func (in *Installer) extract(filePath string) (string, error) {
	// 创建临时目录
	extractDir, err := os.MkdirTemp(in.config.TempDir, "skill-extract-")
	if err != nil {
		return "", err
	}

	// 打开 ZIP 文件
	r, err := zip.OpenReader(filePath)
	if err != nil {
		os.RemoveAll(extractDir)
		return "", err
	}
	defer r.Close()

	// 解压文件
	for _, f := range r.File {
		// 构造目标路径
		targetPath := filepath.Join(extractDir, f.Name)

		// 安全检查：防止路径遍历
		if !filepath.HasPrefix(targetPath, filepath.Clean(extractDir)+string(os.PathSeparator)) {
			os.RemoveAll(extractDir)
			return "", fmt.Errorf("invalid file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			// 创建目录
			os.MkdirAll(targetPath, f.Mode())
			continue
		}

		// 创建文件
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			os.RemoveAll(extractDir)
			return "", err
		}

		// 解压文件
		src, err := f.Open()
		if err != nil {
			os.RemoveAll(extractDir)
			return "", err
		}
		defer src.Close()

		dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			os.RemoveAll(extractDir)
			return "", err
		}

		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			os.RemoveAll(extractDir)
			return "", err
		}
		dst.Close()
	}

	return extractDir, nil
}

// loadManifest 加载清单文件
func (in *Installer) loadManifest(dir string) (*SkillManifest, error) {
	// 查找 manifest.json 或 manifest.yaml
	manifestPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		manifestPath = filepath.Join(dir, "manifest.yaml")
	}

	// 读取文件
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	// 解析
	var manifest SkillManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// validateManifest 验证清单
func (in *Installer) validateManifest(manifest *SkillManifest) error {
	// 基本验证
	if manifest.ID == "" {
		return fmt.Errorf("skill ID is required")
	}
	if manifest.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if manifest.Version == "" {
		return fmt.Errorf("skill version is required")
	}
	if manifest.Type == "" {
		return fmt.Errorf("skill type is required")
	}

	// 检查依赖
	for _, dep := range manifest.Dependencies {
		if dep.ID == "" {
			return fmt.Errorf("dependency ID is required")
		}
	}

	return nil
}

// copyFiles 复制文件
func (in *Installer) copyFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// 构造目标路径
		targetPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// 复制文件
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, src)
		return err
	})
}

// getInstallDir 获取安装目录
func (in *Installer) getInstallDir(id SkillID) string {
	return filepath.Join(in.config.InstallDir, string(id))
}

// createBackup 创建备份
func (in *Installer) createBackup(manifest *SkillManifest) string {
	backupDir := filepath.Join(in.config.TempDir, fmt.Sprintf("skill-backup-%s-%d", manifest.ID, time.Now().Unix()))
	_ = in.getInstallDir(manifest.ID) // 预留用于备份

	// 复制整个安装目录
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return ""
	}

	// TODO: 实现完整的备份
	return backupDir
}

// restoreBackup 恢复备份
func (in *Installer) restoreBackup(id SkillID, backupDir string) error {
	// TODO: 实现恢复逻辑
	return nil
}
