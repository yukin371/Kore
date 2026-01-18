// Package main 是 Kore 的入口点
//
// Kore 是一个 AI 驱动的自动化工作流平台，采用混合 CLI/TUI/GUI 界面。
// 通过自然语言交互提供智能代码理解、修改和自动化能力。
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yukin371/Kore/internal/adapters/cli"
	ollamaadapter "github.com/yukin371/Kore/internal/adapters/ollama"
	openaiadapter "github.com/yukin371/Kore/internal/adapters/openai"
	"github.com/yukin371/Kore/internal/adapters/tui"
	agentpkg "github.com/yukin371/Kore/internal/agent"
	koreconfig "github.com/yukin371/Kore/internal/config"
	"github.com/yukin371/Kore/internal/core"
	"github.com/yukin371/Kore/internal/infrastructure/config"
	"github.com/yukin371/Kore/internal/tools"
	"github.com/yukin371/Kore/pkg/logger"
	"github.com/yukin371/Kore/pkg/utils"
)

// version is set by build flags during release
var version = "dev"

var (
	cfgFile string
	verbose bool
	uiMode  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kore",
	Short: "AI-powered workflow automation platform",
	Long: `Kore is an AI-powered workflow automation platform built with Go.

Serving as the core中枢 for all development tasks, Kore features a hybrid CLI/TUI/GUI
interface and provides intelligent code understanding, modification, and automation
capabilities through natural language interaction.`,
	Version: version,
}

// chatCmd starts an interactive chat session
var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Start an interactive chat session",
	Long:  "Start an interactive chat session with Kore. If a message is provided, it will be processed directly.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runChat,
}

// versionCmd prints version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Kore version %s\n", version)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/kore/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&uiMode, "ui", "u", "cli", "UI mode: cli, tui, or gui")

	// Add subcommands
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if verbose {
		logger.SetLevel(logger.DEBUG)
	}

	// Try new JSONC configuration loader first
	var cfg *koreconfig.Config
	var err error

	// Check if JSONC config exists
	if _, statErr := os.Stat(".kore.jsonc"); statErr == nil {
		// Use new JSONC loader
		loader := koreconfig.NewLoader()
		cfg, err = loader.Load()
		if err != nil {
			logger.Warn("Failed to load JSONC config: %v. Trying legacy config.", err)
		} else {
			logger.Info("JSONC configuration loaded successfully")
			logger.Debug("LLM Provider: %s, Model: %s", cfg.LLM.Provider, cfg.LLM.Model)
			return
		}
	}

	// Fallback to legacy configuration
	legacyCfg, legacyErr := config.Load()
	if legacyErr != nil {
		logger.Warn("Failed to load legacy config: %v. Using defaults.", legacyErr)
		legacyCfg = config.DefaultConfig()
	}

	logger.Info("Legacy configuration loaded successfully")
	logger.Debug("LLM Provider: %s, Model: %s", legacyCfg.LLM.Provider, legacyCfg.LLM.Model)
}

func runChat(cmd *cobra.Command, args []string) error {
	message := ""
	if len(args) > 0 {
		message = strings.Join(args, " ")
	}

	// 加载配置 - 优先使用 JSONC 配置
	var cfg *koreconfig.Config
	var err error

	if _, statErr := os.Stat(".kore.jsonc"); statErr == nil {
		// Use new JSONC loader
		loader := koreconfig.NewLoader()
		cfg, err = loader.Load()
		if err != nil {
			return fmt.Errorf("加载 JSONC 配置失败: %w", err)
		}
	} else {
		// Use legacy config
		legacyCfg, legacyErr := config.Load()
		if legacyErr != nil {
			return fmt.Errorf("加载配置失败: %w", legacyErr)
		}
		// Convert legacy config to new config format
		cfg = convertLegacyConfig(legacyCfg)
	}

	// 确定 UI 模式
	mode := uiMode
	if mode == "" {
		mode = cfg.UI.Mode
	}

	logger.Info("在 %s 模式下启动聊天会话", mode)

	// 创建 UI 适配器
	var uiAdapter core.UIInterface
	var tuiAdapter *tui.Adapter // 保存 TUI 适配器引用（用于需要特殊处理的场景）

	switch mode {
	case "tui":
		tuiAdapter = tui.NewAdapter()
		uiAdapter = tuiAdapter

		// 启动 TUI 程序
		if err := tuiAdapter.Start(); err != nil {
			return fmt.Errorf("启动 TUI 失败: %w", err)
		}
		defer tuiAdapter.Stop() // 确保程序退出时停止 TUI
	case "cli":
		uiAdapter = cli.NewAdapter()
	default:
		return fmt.Errorf("未知的 UI 模式: %s", mode)
	}

	// 获取项目根目录
	projectRoot, err := utils.GetProjectRoot()
	if err != nil {
		return fmt.Errorf("无法找到项目根目录: %w", err)
	}

	// 创建 LLM Provider
	var llmProvider core.LLMProvider
	switch cfg.LLM.Provider {
	case "openai":
		llmProvider = openaiadapter.NewProvider(cfg.LLM.APIKey, cfg.LLM.Model)
		if cfg.LLM.BaseURL != "" {
			provider := llmProvider.(*openaiadapter.Provider)
			provider.SetBaseURL(cfg.LLM.BaseURL)
		}
	case "ollama":
		baseURL := cfg.LLM.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434" // Ollama 默认地址
		}
		llmProvider = ollamaadapter.NewProvider(baseURL, cfg.LLM.Model)
	default:
		return fmt.Errorf("不支持的 LLM 提供商: %s", cfg.LLM.Provider)
	}

	// 创建工具执行器
	toolExecutor := tools.NewToolExecutor(projectRoot)

	// 创建 Agent
	agent := core.NewAgent(uiAdapter, llmProvider, toolExecutor, projectRoot)
	agent.Config.LLM.Model = cfg.LLM.Model
	agent.Config.LLM.Temperature = cfg.LLM.Temperature
	agent.Config.LLM.MaxTokens = cfg.LLM.MaxTokens

	orchestrator := loadOrchestrator(projectRoot)

	// 启动会话
	uiAdapter.ShowStatus("Kore 正在初始化...")

	if message != "" {
		// 单次消息模式
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := runWithOrchestration(ctx, agent, orchestrator, message); err != nil {
			return fmt.Errorf("Agent 运行失败: %w", err)
		}
	} else {
		// 交互模式
		if mode == "tui" {
			// TUI 模式：从 TUI 通道读取用户输入
			uiAdapter.SendStream("交互式聊天模式已启动 - 在输入框中输入消息 (Ctrl+C 退出)\n")

			inputChan := tuiAdapter.GetInputChannel()
			for {
				select {
				case input := <-inputChan:
					// 处理用户输入
					if input == "quit" || input == "exit" {
						uiAdapter.SendStream("再见!\n")
						return nil
					}

					if strings.TrimSpace(input) != "" {
						uiAdapter.ShowStatus("处理中...")
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
						if err := runWithOrchestration(ctx, agent, orchestrator, input); err != nil {
							uiAdapter.SendStream(fmt.Sprintf("\n错误: %v\n", err))
						}
						cancel()
						uiAdapter.ShowStatus("准备就绪")
					}
				}
			}
		} else {
			// CLI 模式：使用标准输入读取
			uiAdapter.SendStream("\n交互式聊天模式 (输入 'quit' 或 'exit' 退出)\n\n")

			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Print("> ")
				if !scanner.Scan() {
					break
				}

				input := scanner.Text()
				if input == "quit" || input == "exit" {
					uiAdapter.SendStream("再见!\n")
					break
				}

				if strings.TrimSpace(input) == "" {
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				if err := runWithOrchestration(ctx, agent, orchestrator, input); err != nil {
					uiAdapter.SendStream(fmt.Sprintf("\n错误: %v\n", err))
				}
				cancel()

				uiAdapter.SendStream("\n")
			}

			if err := scanner.Err(); err != nil {
				return fmt.Errorf("读取输入失败: %w", err)
			}
		}
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadOrchestrator(projectRoot string) *agentpkg.Orchestrator {
	agentsPath := filepath.Join(projectRoot, "configs", "agents.yaml")
	if _, err := os.Stat(agentsPath); err != nil {
		return nil
	}

	agentsCfg, err := agentpkg.LoadAgentsConfig(agentsPath)
	if err != nil {
		logger.Warn("加载 agents.yaml 失败: %v", err)
		return nil
	}

	orchestrator := &agentpkg.Orchestrator{Agents: agentsCfg}

	policyPath := filepath.Join(projectRoot, "configs", "policy.yaml")
	if _, err := os.Stat(policyPath); err == nil {
		policyCfg, err := agentpkg.LoadPolicyConfig(policyPath)
		if err != nil {
			logger.Warn("加载 policy.yaml 失败: %v", err)
		} else {
			orchestrator.Policy = policyCfg
		}
	}

	return orchestrator
}

func runWithOrchestration(ctx context.Context, agent *core.Agent, orchestrator *agentpkg.Orchestrator, input string) error {
	if orchestrator == nil {
		return agent.Run(ctx, input)
	}

	execRoles := executionRoles(orchestrator.Agents)
	execModel := modelForRole(orchestrator, pickRole(execRoles))
	planModel := modelForRole(orchestrator, orchestrator.Agents.Default.Planner)
	reviewModel := modelForRole(orchestrator, orchestrator.Agents.Default.Reviewer)

	runner := &agentpkg.SimpleReActRunner{
		Agent:        agent,
		PlanModel:    planModel,
		ExecuteModel: execModel,
		ReviewModel:  reviewModel,
	}

	controller := &agentpkg.LoopController{
		Runner:          runner,
		MaxLoops:        1,
		AllowIncomplete: true,
	}

	return controller.Run(ctx, input)
}

func executionRoles(cfg agentpkg.AgentsConfig) []string {
	skip := map[string]bool{
		cfg.Default.Supervisor: true,
		cfg.Default.Planner:    true,
		cfg.Default.Reviewer:   true,
	}

	roles := make([]string, 0, len(cfg.Roles))
	for role := range cfg.Roles {
		if skip[role] {
			continue
		}
		roles = append(roles, role)
	}

	sort.Strings(roles)
	return roles
}

func pickRole(roles []string) string {
	if len(roles) == 0 {
		return ""
	}
	return roles[0]
}

func modelForRole(orchestrator *agentpkg.Orchestrator, role string) string {
	if orchestrator == nil || role == "" {
		return ""
	}

	model, _, err := orchestrator.SelectModel(role, map[string]bool{})
	if err != nil {
		return ""
	}
	return model
}

// convertLegacyConfig converts legacy config to new config format
func convertLegacyConfig(legacy *config.Config) *koreconfig.Config {
	return &koreconfig.Config{
		LLM: koreconfig.LLMConfig{
			Provider:    legacy.LLM.Provider,
			Model:       legacy.LLM.Model,
			APIKey:      legacy.LLM.APIKey,
			BaseURL:     legacy.LLM.BaseURL,
			Temperature: legacy.LLM.Temperature,
			MaxTokens:   legacy.LLM.MaxTokens,
		},
		Context: koreconfig.ContextConfig{
			MaxTokens:      legacy.Context.MaxTokens,
			MaxTreeDepth:   legacy.Context.MaxTreeDepth,
			MaxFilesPerDir: legacy.Context.MaxFilesPerDir,
		},
		Security: koreconfig.SecurityConfig{
			BlockedCmds:  legacy.Security.BlockedCmds,
			BlockedPaths: legacy.Security.BlockedPaths,
		},
		UI: koreconfig.UIConfig{
			Mode:         legacy.UI.Mode,
			StreamOutput: legacy.UI.StreamOutput,
		},
	}
}
