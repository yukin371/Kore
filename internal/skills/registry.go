package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Registry Skill 注册表
type Registry struct {
	mu         sync.RWMutex
	skills     map[SkillID]*SkillManifest
	dataDir    string
	config     *RegistryConfig
	lastLoad   time.Time
}

// RegistryConfig 注册表配置
type RegistryConfig struct {
	DataDir       string        `json:"data_dir"`        // Skills 数据目录
	AutoLoad      bool          `json:"auto_load"`       // 自动加载所有 Skills
	MaxSkills     int           `json:"max_skills"`      // 最大 Skill 数量
	EnableMetrics bool          `json:"enable_metrics"`  // 启用指标收集
}

// NewRegistry 创建注册表
func NewRegistry(config *RegistryConfig) (*Registry, error) {
	if config.DataDir == "" {
		homeDir, _ := os.UserHomeDir()
		config.DataDir = filepath.Join(homeDir, ".kore", "skills")
	}

	// 确保目录存在
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create skills directory: %w", err)
	}

	registry := &Registry{
		skills:  make(map[SkillID]*SkillManifest),
		dataDir: config.DataDir,
		config:  config,
	}

	// 自动加载
	if config.AutoLoad {
		if err := registry.LoadAll(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to load skills: %w", err)
		}
	}

	return registry, nil
}

// Register 注册一个 Skill
func (r *Registry) Register(ctx context.Context, manifest *SkillManifest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 验证清单
	if err := r.validateManifest(manifest); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	// 检查版本冲突
	if existing, ok := r.skills[manifest.ID]; ok {
		if existing.Version == manifest.Version {
			return fmt.Errorf("skill %s version %s already registered", manifest.ID, manifest.Version)
		}
		// 版本升级：检查兼容性
		if !r.isCompatibleUpgrade(existing.Version, manifest.Version) {
			return fmt.Errorf("incompatible version upgrade: %s -> %s", existing.Version, manifest.Version)
		}
	}

	// 检查数量限制
	if r.config.MaxSkills > 0 && len(r.skills) >= r.config.MaxSkills {
		return fmt.Errorf("maximum number of skills (%d) reached", r.config.MaxSkills)
	}

	// 保存清单到文件
	if err := r.saveManifest(manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// 更新内存中的清单
	manifest.State = StateInstalled
	manifest.InstalledAt = time.Now()
	manifest.UpdatedAt = time.Now()
	r.skills[manifest.ID] = manifest

	return nil
}

// Unregister 注销一个 Skill
func (r *Registry) Unregister(ctx context.Context, id SkillID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.skills[id]; !ok {
		return fmt.Errorf("skill %s not found", id)
	}

	// 删除清单文件
	manifestPath := r.getManifestPath(id)
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove manifest: %w", err)
	}

	delete(r.skills, id)
	return nil
}

// Get 获取 Skill 清单
func (r *Registry) Get(id SkillID) (*SkillManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	manifest, ok := r.skills[id]
	if !ok {
		return nil, fmt.Errorf("skill %s not found", id)
	}

	return manifest, nil
}

// List 列出所有 Skills
func (r *Registry) List() []*SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*SkillManifest, 0, len(r.skills))
	for _, manifest := range r.skills {
		list = append(list, manifest)
	}
	return list
}

// ListByType 按类型列出 Skills
func (r *Registry) ListByType(skillType SkillType) []*SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*SkillManifest, 0)
	for _, manifest := range r.skills {
		if manifest.Type == skillType {
			list = append(list, manifest)
		}
	}
	return list
}

// ListByState 按状态列出 Skills
func (r *Registry) ListByState(state SkillState) []*SkillManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*SkillManifest, 0)
	for _, manifest := range r.skills {
		if manifest.State == state {
			list = append(list, manifest)
		}
	}
	return list
}

// Enable 启用 Skill
func (r *Registry) Enable(ctx context.Context, id SkillID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	manifest, ok := r.skills[id]
	if !ok {
		return fmt.Errorf("skill %s not found", id)
	}

	manifest.State = StateEnabled
	manifest.UpdatedAt = time.Now()

	return r.saveManifest(manifest)
}

// Disable 禁用 Skill
func (r *Registry) Disable(ctx context.Context, id SkillID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	manifest, ok := r.skills[id]
	if !ok {
		return fmt.Errorf("skill %s not found", id)
	}

	manifest.State = StateDisabled
	manifest.UpdatedAt = time.Now()

	return r.saveManifest(manifest)
}

// LoadAll 从磁盘加载所有 Skills
func (r *Registry) LoadAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := os.ReadDir(r.dataDir)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
			}

		// 只处理 .json 文件
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// 读取清单文件
		manifestPath := filepath.Join(r.dataDir, entry.Name())
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			// 记录错误但继续加载其他 Skills
			fmt.Fprintf(os.Stderr, "Failed to read manifest %s: %v\n", manifestPath, err)
			continue
		}

		var manifest SkillManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse manifest %s: %v\n", manifestPath, err)
			continue
		}

		r.skills[manifest.ID] = &manifest
	}

	r.lastLoad = time.Now()
	return nil
}

// validateManifest 验证清单
func (r *Registry) validateManifest(manifest *SkillManifest) error {
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
	if manifest.EntryPoint == "" && manifest.Type != SkillTypeBuiltin {
		return fmt.Errorf("entry_point is required for non-builtin skills")
	}

	// 验证权限
	for i, perm := range manifest.Permissions {
		if perm.Type == "" {
			return fmt.Errorf("permission %d: type is required", i)
		}
	}

	return nil
}

// saveManifest 保存清单到文件
func (r *Registry) saveManifest(manifest *SkillManifest) error {
	manifestPath := r.getManifestPath(manifest.ID)

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, 0644)
}

// getManifestPath 获取清单文件路径
func (r *Registry) getManifestPath(id SkillID) string {
	return filepath.Join(r.dataDir, fmt.Sprintf("%s.json", id))
}

// isCompatibleUpgrade 检查版本升级是否兼容
func (r *Registry) isCompatibleUpgrade(oldVersion, newVersion SkillVersion) bool {
	// 简单实现：只允许补丁版本升级
	// 完整实现应该使用语义化版本比较
	return true
}
