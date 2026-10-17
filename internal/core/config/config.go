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
	MaxDeliveryTokens int            `json:"max_delivery_tokens,omitempty" yaml:"max_delivery_tokens,omitempty"`
	MaxHookLatencyMs  int            `json:"max_hook_latency_ms,omitempty" yaml:"max_hook_latency_ms,omitempty"`
	DailyHousekeeping int            `json:"daily_housekeeping,omitempty" yaml:"daily_housekeeping,omitempty"`
	DailyTokens       int            `json:"daily_tokens,omitempty" yaml:"daily_tokens,omitempty"`
	DossierCapTokens  int            `json:"dossier_cap_tokens,omitempty" yaml:"dossier_cap_tokens,omitempty"`
	Extra             map[string]any `json:"extra,omitempty" yaml:",inline"`
}

// JobSpendConfig defines rate limits and per-run item caps for a single LLM job kind (Addendum A §6.2).
type JobSpendConfig struct {
	MinInterval string `json:"min_interval,omitempty" yaml:"min_interval,omitempty"`
	MaxItems    int    `json:"max_items,omitempty" yaml:"max_items,omitempty"`
}

// SpendConfig defines rate-limiting and quota guards for background LLM jobs (Addendum A §6.2).
type SpendConfig struct {
	QuotaFloor float64                   `json:"quota_floor,omitempty" yaml:"quota_floor,omitempty"`
	Jobs       map[string]JobSpendConfig `json:"jobs,omitempty" yaml:"jobs,omitempty"`
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
	out := SpendConfig{
		QuotaFloor: base.QuotaFloor,
		Jobs:       make(map[string]JobSpendConfig, len(base.Jobs)),
	}
	for k, v := range base.Jobs {
		out.Jobs[k] = v
	}
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
	Threshold float64 `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	Push      bool    `json:"push,omitempty" yaml:"push,omitempty"`
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
	Enabled              bool          `json:"enabled" yaml:"enabled"`
	HousekeepingExecutor string        `json:"housekeeping_executor,omitempty" yaml:"housekeeping_executor,omitempty"`
	ReportsSurface       string        `json:"reports_surface,omitempty" yaml:"reports_surface,omitempty"`
	TickTrigger          string        `json:"tick_trigger,omitempty" yaml:"tick_trigger,omitempty"`
	Budgets              BudgetsConfig `json:"budgets,omitempty" yaml:"budgets,omitempty"`
	Spend                SpendConfig   `json:"spend,omitempty" yaml:"spend,omitempty"`
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
	type rawMachineConfig struct {
		Enabled              *bool         `yaml:"enabled"`
		HousekeepingExecutor string        `yaml:"housekeeping_executor"`
		ReportsSurface       string        `yaml:"reports_surface"`
		TickTrigger          string        `yaml:"tick_trigger"`
		Budgets              BudgetsConfig `yaml:"budgets"`
		Spend                SpendConfig   `yaml:"spend"`

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
	if c.Spend.QuotaFloor == 0 && len(c.Spend.Jobs) == 0 {
		c.Spend = DefaultSpendConfig()
	}
	c.Spend = MergeSpendConfig(c.Spend, raw.Spend)

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
		Spend                SpendConfig   `json:"spend"`

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
	if c.Spend.QuotaFloor == 0 && len(c.Spend.Jobs) == 0 {
		c.Spend = DefaultSpendConfig()
	}
	c.Spend = MergeSpendConfig(c.Spend, raw.Spend)

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
	Enabled             *bool                      `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	TurnNotes           string                     `json:"turn_notes,omitempty" yaml:"turn_notes,omitempty"`
	TurnNotesRoleOptOut []string                   `json:"turn_notes_role_opt_out,omitempty" yaml:"turn_notes_role_opt_out,omitempty"`
	DocSeeds            []string                   `json:"doc_seeds,omitempty" yaml:"doc_seeds,omitempty"`
	Subsystems          map[string]SubsystemConfig `json:"subsystems,omitempty" yaml:"subsystems,omitempty"`
	Budgets             BudgetsConfig              `json:"budgets,omitempty" yaml:"budgets,omitempty"`
	Spend               SpendConfig                `json:"spend,omitempty" yaml:"spend,omitempty"`
	Gaps                GapsConfig                 `json:"gaps,omitempty" yaml:"gaps,omitempty"`
	Extra               map[string]any             `json:"extra,omitempty" yaml:",inline"`
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

// DefaultDocSeedGlobs returns the default repository doc seed globs when DocSeeds is unset.
func DefaultDocSeedGlobs() []string {
	return []string{"docs/**/*.md", ".agents/skills/**/SKILL.md"}
}

// ResolveDocSeedGlobs expands the configured (or default) doc seed globs across repoRoot,
// applying exclusion rules for PA workspaces, evaluation/experiment artifacts, .memory/,
// and graduated skills containing `generated_by: repo-memory` in their YAML frontmatter.
func ResolveDocSeedGlobs(repoRoot string, cfg RepoConfig) ([]string, error) {
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

func normalizeSlashPattern(raw string) string {
	p := filepath.ToSlash(strings.TrimSpace(raw))
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	if p == "" || p == "." {
		return ""
	}
	return p
}

func shouldExcludeDocSeedPath(relSlash string, isExplicitExact bool) bool {
	if relSlash == ".memory" || strings.HasPrefix(relSlash, ".memory/") || strings.Contains(relSlash, "/.memory/") {
		return true
	}
	if strings.HasPrefix(relSlash, ".agents/skills/principal-architect-workspace/") ||
		strings.Contains(relSlash, "/principal-architect-workspace/") ||
		strings.HasPrefix(relSlash, "principal-architect-workspace/") {
		return true
	}
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

func matchSlashGlob(pattern, target string) bool {
	patSegs := strings.Split(pattern, "/")
	tgtSegs := strings.Split(target, "/")
	return matchSegments(patSegs, tgtSegs)
}

func matchSegments(patSegs, tgtSegs []string) bool {
	if len(patSegs) == 0 {
		return len(tgtSegs) == 0
	}
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

func hasGeneratedByRepoMemoryFrontmatter(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	content := strings.TrimPrefix(string(data), "\uFEFF")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
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


