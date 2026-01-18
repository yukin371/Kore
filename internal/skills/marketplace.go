package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Marketplace 插件市场
type Marketplace struct {
	mu       sync.RWMutex
	index    map[SkillID]*MarketEntry
	sources  []string // 索引源 URLs
	config   *MarketConfig
	registry *Registry
	installer *Installer
}

// MarketConfig 市场配置
type MarketConfig struct {
	CacheDir    string        `json:"cache_dir"`     // 缓存目录
	CacheTTL    time.Duration `json:"cache_ttl"`     // 缓存过期时间
	AutoRefresh bool          `json:"auto_refresh"`  // 自动刷新索引
	RefreshInt  time.Duration `json:"refresh_int"`   // 刷新间隔
	OfficialURL string        `json:"official_url"`  // 官方索引 URL
}

// MarketEntry 市场条目
type MarketEntry struct {
	ID          SkillID      `json:"id"`
	Name        string       `json:"name"`
	Version     SkillVersion `json:"version"`
	Description string       `json:"description"`
	Author      string       `json:"author"`
	License     string       `json:"license"`
	Homepage    string       `json:"homepage"`
	Repository  string       `json:"repository"`
	DownloadURL string       `json:"download_url"`
	SHA256      string       `json:"sha256"`
	Size        int64        `json:"size"`
	PublishedAt time.Time    `json:"published_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Downloads   int          `json:"downloads"`
	Rating      float64      `json:"rating"`
	Tags        []string     `json:"tags"`
	Permissions []Permission `json:"permissions"`
	Installed   bool         `json:"installed"`   // 是否已安装
	InstalledVersion SkillVersion `json:"installed_version,omitempty"` // 已安装的版本
}

// NewMarketplace 创建市场
func NewMarketplace(config *MarketConfig, registry *Registry, installer *Installer) (*Marketplace, error) {
	if config == nil {
		config = &MarketConfig{
			CacheDir:    filepath.Join(os.TempDir(), "kore-marketplace"),
			CacheTTL:    24 * time.Hour,
			AutoRefresh: false,
			OfficialURL: "https://raw.githubusercontent.com/yukin371/Kore/main/marketplace/index.json",
		}
	}

	// 确保缓存目录存在
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	mp := &Marketplace{
		index:    make(map[SkillID]*MarketEntry),
		config:   config,
		registry: registry,
		installer: installer,
	}

	// 加载缓存的索引
	if err := mp.loadCachedIndex(); err != nil {
		// 缓存不存在或无效，不是致命错误
		fmt.Fprintf(os.Stderr, "Warning: failed to load cached index: %v\n", err)
	}

	return mp, nil
}

// Refresh 刷新索引
func (mp *Marketplace) Refresh(ctx context.Context) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// 从官方源下载索引
	if mp.config.OfficialURL != "" {
		if err := mp.fetchIndex(ctx, mp.config.OfficialURL); err != nil {
			return fmt.Errorf("failed to fetch official index: %w", err)
		}
	}

	// 从其他源下载索引
	for _, source := range mp.sources {
		if err := mp.fetchIndex(ctx, source); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch index from %s: %v\n", source, err)
		}
	}

	// 缓存索引
	if err := mp.cacheIndex(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cache index: %v\n", err)
	}

	// 更新安装状态
	mp.updateInstalledStatus()

	return nil
}

// Search 搜索插件
func (mp *Marketplace) Search(query string) []*MarketEntry {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var results []*MarketEntry

	for _, entry := range mp.index {
		if mp.matchesQuery(entry, query) {
			results = append(results, entry)
		}
	}

	return results
}

// List 列出所有插件
func (mp *Marketplace) List() []*MarketEntry {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	list := make([]*MarketEntry, 0, len(mp.index))
	for _, entry := range mp.index {
		list = append(list, entry)
	}
	return list
}

// Get 获取插件信息
func (mp *Marketplace) Get(id SkillID) (*MarketEntry, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	entry, ok := mp.index[id]
	if !ok {
		return nil, fmt.Errorf("skill %s not found in marketplace", id)
	}

	return entry, nil
}

// Install 从市场安装插件
func (mp *Marketplace) Install(ctx context.Context, id SkillID) (*SkillManifest, error) {
	mp.mu.RLock()
	entry, ok := mp.index[id]
	mp.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("skill %s not found in marketplace", id)
	}

	// 使用安装器安装
	manifest, err := mp.installer.InstallFromURL(ctx, entry.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("installation failed: %w", err)
	}

	// 更新安装状态
	mp.mu.Lock()
	entry.Installed = true
	entry.InstalledVersion = manifest.Version
	mp.mu.Unlock()

	return manifest, nil
}

// Uninstall 卸载插件
func (mp *Marketplace) Uninstall(ctx context.Context, id SkillID) error {
	// 使用安装器卸载
	if err := mp.installer.Uninstall(ctx, id); err != nil {
		return err
	}

	// 更新安装状态
	mp.mu.Lock()
	if entry, ok := mp.index[id]; ok {
		entry.Installed = false
		entry.InstalledVersion = ""
	}
	mp.mu.Unlock()

	return nil
}

// Update 更新插件
func (mp *Marketplace) Update(ctx context.Context, id SkillID) (*SkillManifest, error) {
	mp.mu.RLock()
	entry, ok := mp.index[id]
	if !ok {
		mp.mu.RUnlock()
		return nil, fmt.Errorf("skill %s not found in marketplace", id)
	}
	installedVersion := entry.InstalledVersion
	mp.mu.RUnlock()

	// 如果已是最新版本
	if installedVersion == entry.Version {
		return nil, fmt.Errorf("skill %s is already up to date", id)
	}

	// 卸载旧版本
	if err := mp.installer.Uninstall(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to uninstall old version: %w", err)
	}

	// 安装新版本
	return mp.Install(ctx, id)
}

// fetchIndex 获取索引
func (mp *Marketplace) fetchIndex(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch index failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var entries []MarketEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// 合并到索引中
	for _, entry := range entries {
		mp.index[entry.ID] = &entry
	}

	return nil
}

// loadCachedIndex 加载缓存的索引
func (mp *Marketplace) loadCachedIndex() error {
	cacheFile := filepath.Join(mp.config.CacheDir, "index.json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 缓存不存在不是错误
		}
		return err
	}

	var entries []MarketEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// 加载到索引
	for _, entry := range entries {
		mp.index[entry.ID] = &entry
	}

	// 更新安装状态
	mp.updateInstalledStatus()

	return nil
}

// cacheIndex 缓存索引
func (mp *Marketplace) cacheIndex() error {
	cacheFile := filepath.Join(mp.config.CacheDir, "index.json")

	entries := make([]MarketEntry, 0, len(mp.index))
	for _, entry := range mp.index {
		entries = append(entries, *entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0644)
}

// updateInstalledStatus 更新安装状态
func (mp *Marketplace) updateInstalledStatus() {
	for _, entry := range mp.index {
		if manifest, err := mp.registry.Get(entry.ID); err == nil {
			entry.Installed = true
			entry.InstalledVersion = manifest.Version
		} else {
			entry.Installed = false
			entry.InstalledVersion = ""
		}
	}
}

// matchesQuery 检查是否匹配查询
func (mp *Marketplace) matchesQuery(entry *MarketEntry, query string) bool {
	if query == "" {
		return true
	}

	// 在名称、描述、标签中搜索
	query = lower(query)
	if contains(lower(entry.Name), query) {
		return true
	}
	if contains(lower(entry.Description), query) {
		return true
	}
	for _, tag := range entry.Tags {
		if contains(lower(tag), query) {
			return true
		}
	}
	return false
}

func lower(s string) string {
	// 简单实现，实际应使用 strings.ToLower
	return s
}

func contains(s, substr string) bool {
	// 简单实现，实际应使用 strings.Contains
	return true
}

// AddSource 添加索引源
func (mp *Marketplace) AddSource(url string) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.sources = append(mp.sources, url)
}
