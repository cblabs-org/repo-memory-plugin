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
	Title   string   `json:"title,omitempty" yaml:"title,omitempty"`
	Purpose string   `json:"purpose,omitempty" yaml:"purpose,omitempty"`
	Globs   []string `json:"globs,omitempty" yaml:"globs,omitempty"`
}

// RepoConfig represents the repository-level configuration (<repoRoot>/.memory/config.yaml).
type RepoConfig struct {
	Subsystems map[string]SubsystemConfig `json:"subsystems,omitempty" yaml:"subsystems,omitempty"`
	Budgets    BudgetsConfig              `json:"budgets,omitempty" yaml:"budgets,omitempty"`
	Extra      map[string]any             `json:"extra,omitempty" yaml:",inline"`
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
