package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// BudgetsConfig defines operational and resource budgets for memory operations.
type BudgetsConfig struct {
	MaxDeliveryTokens int            `json:"max_delivery_tokens,omitempty" yaml:"max_delivery_tokens,omitempty"`
	MaxHookLatencyMs  int            `json:"max_hook_latency_ms,omitempty" yaml:"max_hook_latency_ms,omitempty"`
	DailyHousekeeping int            `json:"daily_housekeeping,omitempty" yaml:"daily_housekeeping,omitempty"`
	DailyTokens       int            `json:"daily_tokens,omitempty" yaml:"daily_tokens,omitempty"`
	Extra             map[string]any `json:"extra,omitempty" yaml:",inline"`
}

// MachineConfig represents the machine-level configuration (<data-dir>/config.yaml).
type MachineConfig struct {
	Enabled              bool          `json:"enabled" yaml:"enabled"`
	HousekeepingExecutor string        `json:"housekeeping_executor,omitempty" yaml:"housekeeping_executor,omitempty"`
	ReportsSurface       string        `json:"reports_surface,omitempty" yaml:"reports_surface,omitempty"`
	TickTrigger          string        `json:"tick_trigger,omitempty" yaml:"tick_trigger,omitempty"`
	Budgets              BudgetsConfig `json:"budgets,omitempty" yaml:"budgets,omitempty"`
}

// DefaultMachineConfig returns a MachineConfig with recommended defaults.
func DefaultMachineConfig() MachineConfig {
	return MachineConfig{
		Enabled:              true,
		HousekeepingExecutor: "agy",
		ReportsSurface:       "agentapi",
		TickTrigger:          "sidecar",
		Budgets:              BudgetsConfig{},
	}
}

// UnmarshalYAML supports flat, dotted, and nested forms of MachineConfig in YAML.
func (c *MachineConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawMachineConfig struct {
		Enabled              *bool         `yaml:"enabled"`
		HousekeepingExecutor string        `yaml:"housekeeping_executor"`
		ReportsSurface       string        `yaml:"reports_surface"`
		TickTrigger          string        `yaml:"tick_trigger"`
		Budgets              BudgetsConfig `yaml:"budgets"`

		HousekeepingExecutorDot string `yaml:"housekeeping.executor"`
		ReportsSurfaceDot       string `yaml:"reports.surface"`
		TickTriggerDot          string `yaml:"tick.trigger"`

		Housekeeping struct {
			Executor string `yaml:"executor"`
		} `yaml:"housekeeping"`
		Reports struct {
			Surface string `yaml:"surface"`
		} `yaml:"reports"`
		Tick struct {
			Trigger string `yaml:"trigger"`
		} `yaml:"tick"`
	}

	var raw rawMachineConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	if raw.Enabled != nil {
		c.Enabled = *raw.Enabled
	}

	if raw.HousekeepingExecutor != "" {
		c.HousekeepingExecutor = raw.HousekeepingExecutor
	} else if raw.HousekeepingExecutorDot != "" {
		c.HousekeepingExecutor = raw.HousekeepingExecutorDot
	} else if raw.Housekeeping.Executor != "" {
		c.HousekeepingExecutor = raw.Housekeeping.Executor
	}

	if raw.ReportsSurface != "" {
		c.ReportsSurface = raw.ReportsSurface
	} else if raw.ReportsSurfaceDot != "" {
		c.ReportsSurface = raw.ReportsSurfaceDot
	} else if raw.Reports.Surface != "" {
		c.ReportsSurface = raw.Reports.Surface
	}

	if raw.TickTrigger != "" {
		c.TickTrigger = raw.TickTrigger
	} else if raw.TickTriggerDot != "" {
		c.TickTrigger = raw.TickTriggerDot
	} else if raw.Tick.Trigger != "" {
		c.TickTrigger = raw.Tick.Trigger
	}

	c.Budgets = raw.Budgets

	return nil
}

// UnmarshalJSON supports flat and nested forms of MachineConfig in JSON.
func (c *MachineConfig) UnmarshalJSON(b []byte) error {
	type rawMachineJSON struct {
		Enabled              *bool         `json:"enabled"`
		HousekeepingExecutor string        `json:"housekeeping_executor"`
		ReportsSurface       string        `json:"reports_surface"`
		TickTrigger          string        `json:"tick_trigger"`
		Budgets              BudgetsConfig `json:"budgets"`

		Housekeeping struct {
			Executor string `json:"executor"`
		} `json:"housekeeping"`
		Reports struct {
			Surface string `json:"surface"`
		} `json:"reports"`
		Tick struct {
			Trigger string `json:"trigger"`
		} `json:"tick"`
	}

	var raw rawMachineJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if raw.Enabled != nil {
		c.Enabled = *raw.Enabled
	}

	if raw.HousekeepingExecutor != "" {
		c.HousekeepingExecutor = raw.HousekeepingExecutor
	} else if raw.Housekeeping.Executor != "" {
		c.HousekeepingExecutor = raw.Housekeeping.Executor
	}

	if raw.ReportsSurface != "" {
		c.ReportsSurface = raw.ReportsSurface
	} else if raw.Reports.Surface != "" {
		c.ReportsSurface = raw.Reports.Surface
	}

	if raw.TickTrigger != "" {
		c.TickTrigger = raw.TickTrigger
	} else if raw.Tick.Trigger != "" {
		c.TickTrigger = raw.Tick.Trigger
	}

	c.Budgets = raw.Budgets

	return nil
}

// SubsystemConfig describes a subsystem defined in .memory/config.yaml.
type SubsystemConfig struct {
	Title       string   `json:"title,omitempty" yaml:"title,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Globs       []string `json:"globs,omitempty" yaml:"globs,omitempty"`
}

// RepoConfig represents the repository-level configuration (<repoRoot>/.memory/config.yaml).
type RepoConfig struct {
	Enabled    *bool                      `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Subsystems map[string]SubsystemConfig `json:"subsystems,omitempty" yaml:"subsystems,omitempty"`
	Budgets    BudgetsConfig              `json:"budgets,omitempty" yaml:"budgets,omitempty"`
	Extra      map[string]any             `json:"extra,omitempty" yaml:",inline"`
}

// IsEnabled returns whether memory is enabled for this repository.
// Returns true by default unless explicitly disabled.
func (c RepoConfig) IsEnabled() bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	return true
}

// DefaultRepoConfig returns an empty default RepoConfig.
func DefaultRepoConfig() RepoConfig {
	return RepoConfig{
		Subsystems: make(map[string]SubsystemConfig),
	}
}


// LoadMachineConfig loads the MachineConfig from <dataDir>/config.yaml.
// If dataDir is empty, DataDir() is used.
// If the config.yaml file does not exist, a default configuration is returned without error.
func LoadMachineConfig(dataDir string) (MachineConfig, error) {
	if dataDir == "" {
		dataDir = DataDir()
	}

	configFile := filepath.Join(dataDir, "config.yaml")
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultMachineConfig(), nil
		}
		return MachineConfig{}, fmt.Errorf("reading machine config: %w", err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return DefaultMachineConfig(), nil
	}

	cfg := DefaultMachineConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return MachineConfig{}, fmt.Errorf("parsing machine config %s: %w", configFile, err)
	}

	return cfg, nil
}

// LoadRepoConfig loads the RepoConfig from <repoRoot>/.memory/config.yaml.
// If the config.yaml file does not exist, an empty default configuration is returned without error.
func LoadRepoConfig(repoRoot string) (RepoConfig, error) {
	if repoRoot == "" {
		repoRoot = "."
	}

	configFile := filepath.Join(repoRoot, ".memory", "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if filepath.Base(repoRoot) == ".memory" {
			configFile = filepath.Join(repoRoot, "config.yaml")
		} else if strings.HasSuffix(repoRoot, "config.yaml") {
			configFile = repoRoot
		}
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRepoConfig(), nil
		}
		return RepoConfig{}, fmt.Errorf("reading repo config: %w", err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return DefaultRepoConfig(), nil
	}

	cfg := DefaultRepoConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return RepoConfig{}, fmt.Errorf("parsing repo config %s: %w", configFile, err)
	}

	return cfg, nil
}

// SaveMachineConfig saves the MachineConfig to <dataDir>/config.yaml.
func SaveMachineConfig(dataDir string, cfg MachineConfig) error {
	if dataDir == "" {
		dataDir = DataDir()
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory %s: %w", dataDir, err)
	}

	configFile := filepath.Join(dataDir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling machine config: %w", err)
	}

	tmpFile := filepath.Join(dataDir, fmt.Sprintf("config-%d.tmp", os.Getpid()))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("writing machine config tmp file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, configFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming machine config to %s: %w", configFile, err)
	}

	return nil
}

// SaveRepoConfig saves the RepoConfig to <repoRoot>/.memory/config.yaml.
func SaveRepoConfig(repoRoot string, cfg RepoConfig) error {
	if repoRoot == "" {
		repoRoot = "."
	}
	memDir := filepath.Join(repoRoot, ".memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return fmt.Errorf("creating .memory directory in %s: %w", repoRoot, err)
	}

	configFile := filepath.Join(memDir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling repo config: %w", err)
	}

	tmpFile := filepath.Join(memDir, fmt.Sprintf("config-%d.tmp", os.Getpid()))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("writing repo config tmp file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, configFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming repo config to %s: %w", configFile, err)
	}

	return nil
}

// SetMachineEnabled modifies <data-dir>/config.yaml to set enabled to true or false.
// It preserves other existing configuration fields.
func SetMachineEnabled(dataDir string, enabled bool) error {
	if dataDir == "" {
		dataDir = DataDir()
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory %s: %w", dataDir, err)
	}

	configFile := filepath.Join(dataDir, "config.yaml")
	var m map[string]any
	data, err := os.ReadFile(configFile)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing machine config %s: %w", configFile, err)
		}
	} else {
		def := DefaultMachineConfig()
		defBytes, _ := yaml.Marshal(def)
		_ = yaml.Unmarshal(defBytes, &m)
	}

	if m == nil {
		m = make(map[string]any)
	}
	m["enabled"] = enabled

	out, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling machine config: %w", err)
	}

	tmpFile := filepath.Join(dataDir, fmt.Sprintf("config-%d.tmp", os.Getpid()))
	if err := os.WriteFile(tmpFile, out, 0644); err != nil {
		return fmt.Errorf("writing machine config tmp file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, configFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming machine config to %s: %w", configFile, err)
	}

	return nil
}

// SetRepoEnabled modifies <repoRoot>/.memory/config.yaml to set enabled to true or false.
// It preserves other existing configuration fields.
func SetRepoEnabled(repoRoot string, enabled bool) error {
	if repoRoot == "" {
		repoRoot = "."
	}
	memDir := filepath.Join(repoRoot, ".memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return fmt.Errorf("creating .memory directory in %s: %w", repoRoot, err)
	}

	configFile := filepath.Join(memDir, "config.yaml")
	var m map[string]any
	data, err := os.ReadFile(configFile)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing repo config %s: %w", configFile, err)
		}
	}

	if m == nil {
		m = make(map[string]any)
	}
	m["enabled"] = enabled

	out, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling repo config: %w", err)
	}

	tmpFile := filepath.Join(memDir, fmt.Sprintf("config-%d.tmp", os.Getpid()))
	if err := os.WriteFile(tmpFile, out, 0644); err != nil {
		return fmt.Errorf("writing repo config tmp file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, configFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming repo config to %s: %w", configFile, err)
	}

	return nil
}

