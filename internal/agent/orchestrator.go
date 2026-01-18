package agent

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// AgentsConfig defines role-model mapping for orchestration.
type AgentsConfig struct {
	Default struct {
		Supervisor string `yaml:"supervisor"`
		Planner    string `yaml:"planner"`
		Reviewer   string `yaml:"reviewer"`
	} `yaml:"default"`
	Roles map[string]RoleConfig `yaml:"roles"`
}

// RoleConfig defines a single role's model and tools.
type RoleConfig struct {
	Model    string   `yaml:"model"`
	Tools    []string `yaml:"tools"`
	Fallback []string `yaml:"fallback,omitempty"`
}

// PolicyConfig defines tool permission policies.
type PolicyConfig struct {
	Default struct {
		DenyByDefault bool   `yaml:"deny_by_default"`
		AuditLevel    string `yaml:"audit_level"`
	} `yaml:"default"`
	Roles map[string]PolicyRole `yaml:"roles"`
}

// PolicyRole defines tool permissions for a role.
type PolicyRole struct {
	Tools map[string]PolicyTool `yaml:"tools"`
}

// PolicyTool defines per-tool constraints.
type PolicyTool struct {
	Allow    *bool    `yaml:"allow,omitempty"`
	Paths    []string `yaml:"paths,omitempty"`
	Commands []string `yaml:"commands,omitempty"`
}

// Stage represents an orchestration stage.
type Stage struct {
	Name  string
	Roles []string
}

// RoleResult records execution output for aggregation.
type RoleResult struct {
	Role         string
	Output       string
	TouchedFiles []string
}

// MergeResult aggregates role results and conflict hints.
type MergeResult struct {
	Conflicts []string
	Results   []RoleResult
}

// Orchestrator provides basic role scheduling and policy evaluation.
type Orchestrator struct {
	Agents AgentsConfig
	Policy PolicyConfig
}

// LoadAgentsConfig loads orchestration roles from YAML.
func LoadAgentsConfig(path string) (AgentsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AgentsConfig{}, err
	}

	var cfg AgentsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return AgentsConfig{}, err
	}

	return cfg, nil
}

// LoadPolicyConfig loads tool policy from YAML.
func LoadPolicyConfig(path string) (PolicyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PolicyConfig{}, err
	}

	var cfg PolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return PolicyConfig{}, err
	}

	return cfg, nil
}

// Schedule builds a plan -> parallel -> review stage ordering.
func (o *Orchestrator) Schedule(executionRoles []string) []Stage {
	stages := make([]Stage, 0, 3)

	if o.Agents.Default.Planner != "" {
		stages = append(stages, Stage{Name: "plan", Roles: []string{o.Agents.Default.Planner}})
	}

	if len(executionRoles) > 0 {
		stages = append(stages, Stage{Name: "execute", Roles: executionRoles})
	}

	if o.Agents.Default.Reviewer != "" {
		stages = append(stages, Stage{Name: "review", Roles: []string{o.Agents.Default.Reviewer}})
	}

	return stages
}

// SelectModel returns the best model for a role with fallback support.
func (o *Orchestrator) SelectModel(role string, unavailable map[string]bool) (string, string, error) {
	rc, ok := o.Agents.Roles[role]
	if !ok {
		return "", "", fmt.Errorf("unknown role: %s", role)
	}

	if rc.Model != "" && !unavailable[rc.Model] {
		return rc.Model, role, nil
	}

	for _, fallback := range rc.Fallback {
		fc, ok := o.Agents.Roles[fallback]
		if ok && fc.Model != "" && !unavailable[fc.Model] {
			return fc.Model, fallback, nil
		}
	}

	return "", "", fmt.Errorf("no available model for role: %s", role)
}

// IsToolAllowed checks role/tool against policy config.
func (o *Orchestrator) IsToolAllowed(role string, tool string) bool {
	rolePolicy, ok := o.Policy.Roles[role]
	if !ok {
		return !o.Policy.Default.DenyByDefault
	}

	toolPolicy, ok := rolePolicy.Tools[tool]
	if !ok {
		return !o.Policy.Default.DenyByDefault
	}

	if toolPolicy.Allow != nil {
		return *toolPolicy.Allow
	}

	return len(toolPolicy.Paths) > 0 || len(toolPolicy.Commands) > 0
}

// MergeResults aggregates role outputs and flags touched file conflicts.
func (o *Orchestrator) MergeResults(results []RoleResult) MergeResult {
	conflictMap := make(map[string][]string)

	for _, result := range results {
		for _, path := range result.TouchedFiles {
			conflictMap[path] = append(conflictMap[path], result.Role)
		}
	}

	conflicts := make([]string, 0)
	for path, roles := range conflictMap {
		if len(roles) > 1 {
			conflicts = append(conflicts, fmt.Sprintf("%s: %s", path, strings.Join(roles, ", ")))
		}
	}

	return MergeResult{
		Conflicts: conflicts,
		Results:   results,
	}
}
