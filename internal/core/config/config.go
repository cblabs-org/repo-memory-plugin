/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/config.go
 * Description: Machine-level (<data-dir>/config.yaml) and repository-level
 *              (<repoRoot>/.memory/config.yaml) configuration schemas, default
 *              builders, atomic persistence helpers, and doc-seed glob expansion.
 *
 * Exported Types / Functions:
 * - BudgetsConfig:              Operational and token budgets for memory operations.
 * - JobSpendConfig:             Rate limits and item caps for a single background LLM job kind.
 * - SpendConfig:                Quota floor and per-job rate-limiting configuration.
 *   - DefaultSpendConfig():     Returns the default spend guard configuration.
 *   - MergeSpendConfig():       Overlays non-zero spend overrides onto a base configuration.
 * - GapsConfig:                 Coverage gap detection and push delivery thresholds.
 *   - DefaultGapsConfig():      Returns the default coverage gap configuration.
 * - MachineConfig:              Machine-wide configuration (<data-dir>/config.yaml).
 *
 *   Own Public API:
 *   - DefaultMachineConfig():   Returns a MachineConfig populated with recommended defaults.
 *
 *   yaml.Unmarshaler:
 *   - UnmarshalYAML():          Decodes flat, dotted, or nested YAML into MachineConfig.
 *
 *   json.Unmarshaler:
 *   - UnmarshalJSON():          Decodes flat or nested JSON into MachineConfig.
 *
 * - SubsystemConfig:            Subsystem title, description, and path globs in .memory/config.yaml.
 * - RepoConfig:                 Repository-level configuration (<repoRoot>/.memory/config.yaml).
 *   - DefaultRepoConfig():      Returns a RepoConfig populated with default maps and thresholds.
 *   - TurnNotesMode():          Returns the normalized turn_notes mode ("on", "fallback-only", or "off").
 *   - IsEnabled():              Reports whether memory is enabled for the repository.
 * - LoadMachineConfig():        Loads MachineConfig from <dataDir>/config.yaml or returns defaults.
 * - LoadRepoConfig():           Loads RepoConfig from <repoRoot>/.memory/config.yaml or returns defaults.
 * - SaveMachineConfig():        Atomically writes MachineConfig to <dataDir>/config.yaml.
 * - SaveRepoConfig():           Atomically writes RepoConfig to <repoRoot>/.memory/config.yaml.
 * - SetMachineEnabled():        Updates the enabled flag in <dataDir>/config.yaml while preserving other keys.
 * - SetRepoEnabled():           Updates the enabled flag in <repoRoot>/.memory/config.yaml while preserving other keys.
 * - DefaultDocSeedGlobs():      Returns the default repository doc seed globs when DocSeeds is unset.
 * - ResolveDocSeedGlobs():      Expands doc seed globs across a repository while applying safety exclusions.
 */

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// BudgetsConfig defines operational and resource budgets for memory operations.
type BudgetsConfig struct {
	MaxDeliveryTokens int            `json:"max_delivery_tokens,omitempty" yaml:"max_delivery_tokens,omitempty"` // Maximum tokens injected per hook delivery.
	MaxHookLatencyMs  int            `json:"max_hook_latency_ms,omitempty" yaml:"max_hook_latency_ms,omitempty"` // Target latency ceiling in milliseconds for hook execution.
	DailyHousekeeping int            `json:"daily_housekeeping,omitempty" yaml:"daily_housekeeping,omitempty"`   // Maximum housekeeping runs permitted per UTC day.
	DailyTokens       int            `json:"daily_tokens,omitempty" yaml:"daily_tokens,omitempty"`               // Maximum background LLM tokens permitted per UTC day.
	DossierCapTokens  int            `json:"dossier_cap_tokens,omitempty" yaml:"dossier_cap_tokens,omitempty"`   // Soft token ceiling for an individual rendered dossier.
	Extra             map[string]any `json:"extra,omitempty" yaml:",inline"`                                     // Unrecognized configuration keys preserved on round-trip.
}

// JobSpendConfig defines rate limits and per-run item caps for a single LLM job kind (Addendum A §6.2).
type JobSpendConfig struct {
	MinInterval string `json:"min_interval,omitempty" yaml:"min_interval,omitempty"` // Minimum duration string (e.g. "1h", "24h") between consecutive job runs.
	MaxItems    int    `json:"max_items,omitempty" yaml:"max_items,omitempty"`       // Maximum items processed in a single job execution.
}

// SpendConfig defines rate-limiting and quota guards for background LLM jobs (Addendum A §6.2).
type SpendConfig struct {
	QuotaFloor float64                   `json:"quota_floor,omitempty" yaml:"quota_floor,omitempty"` // Minimum remaining quota fraction in [0.0, 1.0] required to run background jobs.
	Jobs       map[string]JobSpendConfig `json:"jobs,omitempty" yaml:"jobs,omitempty"`               // Per-job-kind rate limits and item caps keyed by job name.
}

// DefaultSpendConfig returns the Addendum A §6.2 default spend guard configuration.
func DefaultSpendConfig() SpendConfig {
	return SpendConfig{
		QuotaFloor: 0.25,
		Jobs: map[string]JobSpendConfig{
			"fallback":      {MinInterval: "1h", MaxItems: 10},
			"consolidation": {MinInterval: "24h", MaxItems: 40},
			"backfill":      {MinInterval: "6h", MaxItems: 25},
			"gardening":     {MinInterval: "168h", MaxItems: 20},
		},
	}
}

// MergeSpendConfig overlays non-zero fields from override onto base, returning a new SpendConfig.
func MergeSpendConfig(base, override SpendConfig) SpendConfig {
	// --- Step 1: Clone the base spend configuration ---
	out := SpendConfig{
		QuotaFloor: base.QuotaFloor,
		Jobs:       make(map[string]JobSpendConfig, len(base.Jobs)),
	}
	for k, v := range base.Jobs {
		out.Jobs[k] = v
	}

	// --- Step 2: Overlay non-zero quota floor and per-job overrides ---
	if override.QuotaFloor > 0 {
		out.QuotaFloor = override.QuotaFloor
	}
	for k, ov := range override.Jobs {
		cur := out.Jobs[k]
		if ov.MinInterval != "" {
			cur.MinInterval = ov.MinInterval
		}
		if ov.MaxItems > 0 {
			cur.MaxItems = ov.MaxItems
		}
		out.Jobs[k] = cur
	}
	return out
}

// GapsConfig defines coverage gap detection and push delivery thresholds (SPEC.md §5.2 & §9.2).
type GapsConfig struct {
	Threshold float64 `json:"threshold,omitempty" yaml:"threshold,omitempty"` // Minimum gap score threshold in [0.0, 1.0] for flagging under-documented scopes.
	Push      bool    `json:"push,omitempty" yaml:"push,omitempty"`           // Whether coverage gap notices may be surfaced in pushed hook context.
}

// DefaultGapsConfig returns the default repository coverage gap configuration.
func DefaultGapsConfig() GapsConfig {
	return GapsConfig{
		Threshold: 0.5,
		Push:      false,
	}
}

// MachineConfig represents the machine-level configuration (<data-dir>/config.yaml).
type MachineConfig struct {
	Enabled              bool          `json:"enabled" yaml:"enabled"`                                             // Global kill-switch controlling whether repo-memory is active on this machine.
	HousekeepingExecutor string        `json:"housekeeping_executor,omitempty" yaml:"housekeeping_executor,omitempty"` // Executor backend for background LLM jobs (e.g. "agy").
	ReportsSurface       string        `json:"reports_surface,omitempty" yaml:"reports_surface,omitempty"`         // Surface used for operator reports (e.g. "agentapi").
	TickTrigger          string        `json:"tick_trigger,omitempty" yaml:"tick_trigger,omitempty"`               // Mechanism that triggers background housekeeping ticks (e.g. "sidecar").
	Budgets              BudgetsConfig `json:"budgets,omitempty" yaml:"budgets,omitempty"`                         // Machine-level operational and token budgets.
	Spend                SpendConfig   `json:"spend,omitempty" yaml:"spend,omitempty"`                             // Machine-level quota floor and per-job spend limits.
}

// DefaultMachineConfig returns a MachineConfig with recommended defaults.
func DefaultMachineConfig() MachineConfig {
	return MachineConfig{
		Enabled:              true,
		HousekeepingExecutor: "agy",
		ReportsSurface:       "agentapi",
		TickTrigger:          "sidecar",
		Budgets:              BudgetsConfig{},
		Spend:                DefaultSpendConfig(),
	}
}

// UnmarshalYAML supports flat, dotted, and nested forms of MachineConfig in YAML.
func (c *MachineConfig) UnmarshalYAML(value *yaml.Node) error {
	// --- Step 1: Decode into an auxiliary struct capturing flat, dotted, and nested keys ---
	type rawMachineConfig struct {
		Enabled              *bool         `yaml:"enabled"`               // Optional explicit enabled state.
		HousekeepingExecutor string        `yaml:"housekeeping_executor"` // Flat housekeeping executor key.
		ReportsSurface       string        `yaml:"reports_surface"`       // Flat reports surface key.
		TickTrigger          string        `yaml:"tick_trigger"`          // Flat tick trigger key.
		Budgets              BudgetsConfig `yaml:"budgets"`               // Nested budgets section.
		Spend                SpendConfig   `yaml:"spend"`                 // Nested spend section.

		HousekeepingExecutorDot string `yaml:"housekeeping.executor"` // Dotted housekeeping executor key.
		ReportsSurfaceDot       string `yaml:"reports.surface"`       // Dotted reports surface key.
		TickTriggerDot          string `yaml:"tick.trigger"`          // Dotted tick trigger key.

		Housekeeping struct {
			Executor string `yaml:"executor"` // Nested housekeeping.executor value.
		} `yaml:"housekeeping"` // Nested housekeeping block.
		Reports struct {
			Surface string `yaml:"surface"` // Nested reports.surface value.
		} `yaml:"reports"` // Nested reports block.
		Tick struct {
			Trigger string `yaml:"trigger"` // Nested tick.trigger value.
		} `yaml:"tick"` // Nested tick block.
	}

	var raw rawMachineConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	// --- Step 2: Reconcile flat, dotted, and nested string settings by precedence ---
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

	// --- Step 3: Apply budgets and merge spend configuration over defaults ---
	c.Budgets = raw.Budgets
	if c.Spend.QuotaFloor == 0 && len(c.Spend.Jobs) == 0 {
		c.Spend = DefaultSpendConfig()
	}
	c.Spend = MergeSpendConfig(c.Spend, raw.Spend)

	return nil
}

// UnmarshalJSON supports flat and nested forms of MachineConfig in JSON.
func (c *MachineConfig) UnmarshalJSON(b []byte) error {
	// --- Step 1: Decode JSON payload into an auxiliary struct supporting flat and nested keys ---
	type rawMachineJSON struct {
		Enabled              *bool         `json:"enabled"`               // Explicit JSON enabled toggle (nil when omitted).
		HousekeepingExecutor string        `json:"housekeeping_executor"` // Top-level JSON housekeeping_executor override.
		ReportsSurface       string        `json:"reports_surface"`       // Top-level JSON reports_surface override.
		TickTrigger          string        `json:"tick_trigger"`          // Top-level JSON tick_trigger override.
		Budgets              BudgetsConfig `json:"budgets"`               // JSON token and latency budgets object.
		Spend                SpendConfig   `json:"spend"`                 // JSON background spend guard settings.

		Housekeeping struct {
			Executor string `json:"executor"` // Nested JSON housekeeping.executor string.
		} `json:"housekeeping"` // Nested JSON housekeeping object.
		Reports struct {
			Surface string `json:"surface"` // Nested JSON reports.surface string.
		} `json:"reports"` // Nested JSON reports object.
		Tick struct {
			Trigger string `json:"trigger"` // Nested JSON tick.trigger string.
		} `json:"tick"` // Nested JSON tick object.
	}

	var raw rawMachineJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	// --- Step 2: Reconcile flat and nested fields ---
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

	// --- Step 3: Apply budgets and merge spend configuration over defaults ---
	c.Budgets = raw.Budgets
	if c.Spend.QuotaFloor == 0 && len(c.Spend.Jobs) == 0 {
		c.Spend = DefaultSpendConfig()
	}
	c.Spend = MergeSpendConfig(c.Spend, raw.Spend)

	return nil
}

// SubsystemConfig describes a subsystem defined in .memory/config.yaml.
type SubsystemConfig struct {
	Title       string   `json:"title,omitempty" yaml:"title,omitempty"`             // Human-readable subsystem title.
	Description string   `json:"description,omitempty" yaml:"description,omitempty"` // Summary of the subsystem's responsibilities and boundaries.
	Globs       []string `json:"globs,omitempty" yaml:"globs,omitempty"`             // Repository-relative file globs belonging to this subsystem.
}

// RepoConfig represents the repository-level configuration (<repoRoot>/.memory/config.yaml).
type RepoConfig struct {
	Enabled             *bool                      `json:"enabled,omitempty" yaml:"enabled,omitempty"`                             // Optional repository-level enable/disable override.
	TurnNotes           string                     `json:"turn_notes,omitempty" yaml:"turn_notes,omitempty"`                       // Turn note capture mode ("on", "fallback-only", or "off").
	TurnNotesRoleOptOut []string                   `json:"turn_notes_role_opt_out,omitempty" yaml:"turn_notes_role_opt_out,omitempty"` // Agent roles excluded from turn note capture.
	DocSeeds            []string                   `json:"doc_seeds,omitempty" yaml:"doc_seeds,omitempty"`                         // Explicit file paths or globs for Day-1 documentation seeding.
	Subsystems          map[string]SubsystemConfig `json:"subsystems,omitempty" yaml:"subsystems,omitempty"`                       // Declared repository subsystems keyed by subsystem slug.
	Budgets             BudgetsConfig              `json:"budgets,omitempty" yaml:"budgets,omitempty"`                             // Repository-specific budget overrides.
	Spend               SpendConfig                `json:"spend,omitempty" yaml:"spend,omitempty"`                                 // Repository-specific spend guard overrides.
	Gaps                GapsConfig                 `json:"gaps,omitempty" yaml:"gaps,omitempty"`                                   // Repository-specific coverage gap thresholds.
	Extra               map[string]any             `json:"extra,omitempty" yaml:",inline"`                                         // Unrecognized YAML keys preserved on round-trip.
}

// TurnNotesMode returns the normalized turn_notes mode ("on", "fallback-only", or "off").
// Defaults to "on" when unset or unrecognized.
func (c RepoConfig) TurnNotesMode() string {
	mode := strings.ToLower(strings.TrimSpace(c.TurnNotes))
	switch mode {
	case "off", "false", "disabled":
		return "off"
	case "fallback-only", "fallback_only":
		return "fallback-only"
	default:
		return "on"
	}
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
		Spend:      DefaultSpendConfig(),
		Gaps:       DefaultGapsConfig(),
	}
}

// LoadMachineConfig loads the MachineConfig from <dataDir>/config.yaml.
// If dataDir is empty, DataDir() is used.
// If the config.yaml file does not exist, a default configuration is returned without error.
func LoadMachineConfig(dataDir string) (MachineConfig, error) {
	// --- Step 1: Resolve target config file path and read contents ---
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

	// --- Step 2: Unmarshal YAML onto a default-initialized MachineConfig ---
	cfg := DefaultMachineConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return MachineConfig{}, fmt.Errorf("parsing machine config %s: %w", configFile, err)
	}

	return cfg, nil
}

// LoadRepoConfig loads the RepoConfig from <repoRoot>/.memory/config.yaml.
// If the config.yaml file does not exist, an empty default configuration is returned without error.
func LoadRepoConfig(repoRoot string) (RepoConfig, error) {
	// --- Step 1: Resolve the repository config file path ---
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

	// --- Step 2: Read config file bytes or return defaults if absent/empty ---
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

	// --- Step 3: Unmarshal YAML and normalize default maps and thresholds ---
	var raw RepoConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return RepoConfig{}, fmt.Errorf("parsing repo config %s: %w", configFile, err)
	}
	if raw.Subsystems == nil {
		raw.Subsystems = make(map[string]SubsystemConfig)
	}
	raw.Spend = MergeSpendConfig(DefaultSpendConfig(), raw.Spend)
	if raw.Gaps.Threshold <= 0 {
		raw.Gaps.Threshold = DefaultGapsConfig().Threshold
	}

	return raw, nil
}

// SaveMachineConfig saves the MachineConfig to <dataDir>/config.yaml.
func SaveMachineConfig(dataDir string, cfg MachineConfig) error {
	// --- Step 1: Ensure data directory exists and marshal config to YAML ---
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

	// --- Step 2: Write to a PID-scoped temporary file and rename atomically ---
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
	// --- Step 1: Ensure .memory directory exists and marshal config to YAML ---
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

	// --- Step 2: Write to a PID-scoped temporary file and rename atomically ---
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
	// --- Step 1: Ensure data directory exists and load existing YAML map ---
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

	// --- Step 2: Update the enabled key and marshal back to YAML ---
	if m == nil {
		m = make(map[string]any)
	}
	m["enabled"] = enabled

	out, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling machine config: %w", err)
	}

	// --- Step 3: Atomically replace config.yaml via temporary file ---
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

// SetRepoEnabled modifies <repoRoot>/.memory/config.yaml to set enabled to true or false,
// retaining all other repository-level configuration keys (subsystems, scopes, doc_seeds) intact.
func SetRepoEnabled(repoRoot string, enabled bool) error {
	// --- Step 1: Ensure .memory directory exists and load existing YAML map ---
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

	// --- Step 2: Update the enabled key and marshal back to YAML ---
	if m == nil {
		m = make(map[string]any)
	}
	m["enabled"] = enabled

	out, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling repo config: %w", err)
	}

	// --- Step 3: Atomically replace .memory/config.yaml via temporary file ---
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

// DefaultDocSeedGlobs returns the default repository doc seed globs when DocSeeds is unset.
func DefaultDocSeedGlobs() []string {
	return []string{"docs/**/*.md", ".agents/skills/**/SKILL.md"}
}

// ResolveDocSeedGlobs expands the configured (or default) doc seed globs across repoRoot,
// applying exclusion rules for PA workspaces, evaluation/experiment artifacts, .memory/,
// and graduated skills containing `generated_by: repo-memory` in their YAML frontmatter.
func ResolveDocSeedGlobs(repoRoot string, cfg RepoConfig) ([]string, error) {
	// --- Step 1: Normalize repoRoot and determine active doc seed patterns ---
	if repoRoot == "" {
		repoRoot = "."
	}

	patterns := cfg.DocSeeds
	if len(patterns) == 0 {
		patterns = DefaultDocSeedGlobs()
	}

	explicitExact := make(map[string]bool)
	for _, rawPat := range cfg.DocSeeds {
		norm := normalizeSlashPattern(rawPat)
		if norm != "" && !strings.ContainsAny(norm, "*?[") {
			explicitExact[norm] = true
		}
	}

	// --- Step 2: Expand each exact path or wildcard glob across the repository ---
	matchedSet := make(map[string]bool)

	for _, rawPat := range patterns {
		pat := normalizeSlashPattern(rawPat)
		if pat == "" {
			continue
		}

		// Exact non-wildcard path
		if !strings.ContainsAny(pat, "*?[") {
			full := filepath.Join(repoRoot, filepath.FromSlash(pat))
			info, err := os.Stat(full)
			if err == nil && info.Mode().IsRegular() {
				matchedSet[pat] = true
			}
			continue
		}

		// Determine prefix directory before any wildcard
		segs := strings.Split(pat, "/")
		var prefixSegs []string
		for _, s := range segs {
			if strings.ContainsAny(s, "*?[") {
				break
			}
			prefixSegs = append(prefixSegs, s)
		}
		prefixRel := strings.Join(prefixSegs, "/")
		walkRoot := repoRoot
		if prefixRel != "" {
			walkRoot = filepath.Join(repoRoot, filepath.FromSlash(prefixRel))
		}
		if _, err := os.Stat(walkRoot); err != nil {
			continue
		}

		err := filepath.WalkDir(walkRoot, func(fullPath string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) || os.IsPermission(walkErr) {
					return nil
				}
				return walkErr
			}
			rel, err := filepath.Rel(repoRoot, fullPath)
			if err != nil {
				return nil
			}
			relSlash := filepath.ToSlash(rel)
			if relSlash == "." {
				return nil
			}

			if d.IsDir() {
				base := d.Name()
				if base == ".git" || base == ".memory" || relSlash == ".agents/skills/principal-architect-workspace" || strings.HasSuffix(relSlash, "/principal-architect-workspace") {
					return filepath.SkipDir
				}
				return nil
			}

			if !d.Type().IsRegular() {
				return nil
			}
			if matchSlashGlob(pat, relSlash) {
				matchedSet[relSlash] = true
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("expanding doc seed glob %q: %w", rawPat, err)
		}
	}

	// --- Step 3: Filter out excluded paths and repo-memory graduated skills ---
	var result []string
	for relSlash := range matchedSet {
		if !strings.HasSuffix(strings.ToLower(relSlash), ".md") {
			continue
		}
		if shouldExcludeDocSeedPath(relSlash, explicitExact[relSlash]) {
			continue
		}
		fullPath := filepath.Join(repoRoot, filepath.FromSlash(relSlash))
		if hasGeneratedByRepoMemoryFrontmatter(fullPath) {
			continue
		}
		result = append(result, relSlash)
	}

	sort.Strings(result)
	return result, nil
}

// normalizeSlashPattern converts raw to forward slashes, trims leading "./" or "/",
// and returns an empty string for root-only patterns.
func normalizeSlashPattern(raw string) string {
	p := filepath.ToSlash(strings.TrimSpace(raw))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	if p == "" || p == "." {
		return ""
	}
	return p
}

// shouldExcludeDocSeedPath reports whether relSlash must be excluded from doc
// seeding due to protected directories (.memory, PA workspaces) or default exclusions.
func shouldExcludeDocSeedPath(relSlash string, isExplicitExact bool) bool {
	// --- Step 1: Always exclude .memory and Principal Architect workspaces ---
	if relSlash == ".memory" || strings.HasPrefix(relSlash, ".memory/") || strings.Contains(relSlash, "/.memory/") {
		return true
	}
	if strings.HasPrefix(relSlash, ".agents/skills/principal-architect-workspace/") ||
		strings.Contains(relSlash, "/principal-architect-workspace/") ||
		strings.HasPrefix(relSlash, "principal-architect-workspace/") {
		return true
	}

	// --- Step 2: Exclude agent instruction files and eval/experiment docs unless explicitly requested ---
	if !isExplicitExact {
		baseUpper := strings.ToUpper(filepath.Base(relSlash))
		if baseUpper == "GEMINI.MD" || baseUpper == "CLAUDE.MD" || baseUpper == "AGENTS.MD" {
			return true
		}
		if strings.HasPrefix(relSlash, "docs/evaluations/") || strings.HasPrefix(relSlash, "docs/experiments/") {
			return true
		}
	}
	return false
}

// matchSlashGlob reports whether target matches the slash-separated glob pattern
// supporting "**" multi-segment wildcards.
func matchSlashGlob(pattern, target string) bool {
	patSegs := strings.Split(pattern, "/")
	tgtSegs := strings.Split(target, "/")
	return matchSegments(patSegs, tgtSegs)
}

// matchSegments recursively matches slash-split pattern segments against target segments.
func matchSegments(patSegs, tgtSegs []string) bool {
	// --- Step 1: Handle base case when pattern segments are exhausted ---
	if len(patSegs) == 0 {
		return len(tgtSegs) == 0
	}

	// --- Step 2: Expand recursive "**" wildcard across zero or more target segments ---
	if patSegs[0] == "**" {
		// Collapse consecutive **
		for len(patSegs) > 1 && patSegs[1] == "**" {
			patSegs = patSegs[1:]
		}
		// ** at end matches everything remaining
		if len(patSegs) == 1 {
			return true
		}
		// Match 0 or more target segments
		for i := 0; i <= len(tgtSegs); i++ {
			if matchSegments(patSegs[1:], tgtSegs[i:]) {
				return true
			}
		}
		return false
	}

	// --- Step 3: Match single segment with path.Match and recurse ---
	if len(tgtSegs) == 0 {
		return false
	}
	segPat := strings.ReplaceAll(patSegs[0], "**", "*")
	matched, err := path.Match(segPat, tgtSegs[0])
	if err != nil || !matched {
		return false
	}
	return matchSegments(patSegs[1:], tgtSegs[1:])
}

// hasGeneratedByRepoMemoryFrontmatter inspects YAML frontmatter at the start of
// filePath and reports whether `generated_by: repo-memory` is present.
func hasGeneratedByRepoMemoryFrontmatter(filePath string) bool {
	// --- Step 1: Read file and verify leading YAML frontmatter fence ---
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	content := strings.TrimPrefix(string(data), "\uFEFF")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}

	// --- Step 2: Scan frontmatter lines for generated_by: repo-memory ---
	for i := 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if trim == "---" {
			break
		}
		if strings.HasPrefix(trim, "generated_by:") {
			val := strings.TrimSpace(strings.TrimPrefix(trim, "generated_by:"))
			val = strings.Trim(val, `"'`)
			if val == "repo-memory" {
				return true
			}
		}
	}
	return false
}
