package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDataDirCurrentPlatform(t *testing.T) {
	dir := DataDir()
	if dir == "" {
		t.Fatal("DataDir() returned empty string")
	}
	if !strings.HasSuffix(dir, "repo-memory") {
		t.Errorf("expected DataDir() to end with repo-memory, got %q", dir)
	}
}

func TestDataDirForOS(t *testing.T) {
	mockEnv := func(env map[string]string) func(string) string {
		return func(k string) string {
			return env[k]
		}
	}
	mockHome := func(home string) func() (string, error) {
		return func() (string, error) {
			return home, nil
		}
	}

	t.Run("macos darwin", func(t *testing.T) {
		dir := dataDirForOS("darwin", mockEnv(nil), mockHome("/Users/testuser"))
		expected := filepath.Join("/Users/testuser", "Library", "Application Support", "repo-memory")
		if dir != expected {
			t.Errorf("darwin: got %q, want %q", dir, expected)
		}
	})

	t.Run("linux default", func(t *testing.T) {
		dir := dataDirForOS("linux", mockEnv(nil), mockHome("/home/testuser"))
		expected := filepath.Join("/home/testuser", ".local", "share", "repo-memory")
		if dir != expected {
			t.Errorf("linux: got %q, want %q", dir, expected)
		}
	})

	t.Run("linux xdg", func(t *testing.T) {
		env := map[string]string{"XDG_DATA_HOME": "/custom/data"}
		dir := dataDirForOS("linux", mockEnv(env), mockHome("/home/testuser"))
		expected := filepath.Join("/custom/data", "repo-memory")
		if dir != expected {
			t.Errorf("linux xdg: got %q, want %q", dir, expected)
		}
	})

	t.Run("windows localappdata", func(t *testing.T) {
		env := map[string]string{"LOCALAPPDATA": `C:\Users\testuser\AppData\Local`}
		dir := dataDirForOS("windows", mockEnv(env), mockHome(`C:\Users\testuser`))
		expected := filepath.Join(`C:\Users\testuser\AppData\Local`, "repo-memory")
		if dir != expected {
			t.Errorf("windows localappdata: got %q, want %q", dir, expected)
		}
	})

	t.Run("windows fallback", func(t *testing.T) {
		dir := dataDirForOS("windows", mockEnv(nil), mockHome(`C:\Users\testuser`))
		expected := filepath.Join(`C:\Users\testuser`, "AppData", "Local", "repo-memory")
		if dir != expected {
			t.Errorf("windows fallback: got %q, want %q", dir, expected)
		}
	})

	t.Run("repo memory data dir override", func(t *testing.T) {
		env := map[string]string{"REPO_MEMORY_DATA_DIR": "/override/path"}
		dir := dataDirForOS("darwin", mockEnv(env), mockHome("/Users/testuser"))
		if dir != "/override/path" {
			t.Errorf("override: got %q, want %q", dir, "/override/path")
		}
	})
}

func TestMachineConfigDefaults(t *testing.T) {
	cfg := DefaultMachineConfig()
	if !cfg.Enabled {
		t.Error("expected DefaultMachineConfig.Enabled to be true")
	}
	if cfg.HousekeepingExecutor != "agy" {
		t.Errorf("expected HousekeepingExecutor 'agy', got %q", cfg.HousekeepingExecutor)
	}
	if cfg.ReportsSurface != "agentapi" {
		t.Errorf("expected ReportsSurface 'agentapi', got %q", cfg.ReportsSurface)
	}
	if cfg.TickTrigger != "sidecar" {
		t.Errorf("expected TickTrigger 'sidecar', got %q", cfg.TickTrigger)
	}
}

func TestLoadMachineConfig_NotExist(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, err := LoadMachineConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error loading from non-existent config: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected default enabled to be true")
	}
	if cfg.HousekeepingExecutor != "agy" {
		t.Errorf("expected default housekeeping_executor to be 'agy', got %q", cfg.HousekeepingExecutor)
	}
}

func TestLoadMachineConfig_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadMachineConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error loading empty config: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected default enabled to be true")
	}
}

func TestLoadMachineConfig_YAMLVariants(t *testing.T) {
	tests := []struct {
		name       string
		yamlData   string
		wantExec   string
		wantReport string
		wantTick   string
		wantEnable bool
	}{
		{
			name: "flat underscore",
			yamlData: `
enabled: true
housekeeping_executor: agy
reports_surface: agentapi
tick_trigger: sidecar
`,
			wantExec:   "agy",
			wantReport: "agentapi",
			wantTick:   "sidecar",
			wantEnable: true,
		},
		{
			name: "dotted",
			yamlData: `
enabled: false
housekeeping.executor: claude
reports.surface: stdout
tick.trigger: cron
`,
			wantExec:   "claude",
			wantReport: "stdout",
			wantTick:   "cron",
			wantEnable: false,
		},
		{
			name: "nested",
			yamlData: `
enabled: true
housekeeping:
  executor: agy-fast
reports:
  surface: cli
tick:
  trigger: timer
`,
			wantExec:   "agy-fast",
			wantReport: "cli",
			wantTick:   "timer",
			wantEnable: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(tc.yamlData), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadMachineConfig(tmpDir)
			if err != nil {
				t.Fatalf("LoadMachineConfig failed: %v", err)
			}
			if cfg.Enabled != tc.wantEnable {
				t.Errorf("enabled: got %v, want %v", cfg.Enabled, tc.wantEnable)
			}
			if cfg.HousekeepingExecutor != tc.wantExec {
				t.Errorf("housekeeping executor: got %q, want %q", cfg.HousekeepingExecutor, tc.wantExec)
			}
			if cfg.ReportsSurface != tc.wantReport {
				t.Errorf("reports surface: got %q, want %q", cfg.ReportsSurface, tc.wantReport)
			}
			if cfg.TickTrigger != tc.wantTick {
				t.Errorf("tick trigger: got %q, want %q", cfg.TickTrigger, tc.wantTick)
			}
		})
	}
}

func TestLoadMachineConfig_Budgets(t *testing.T) {
	tmpDir := t.TempDir()
	yamlContent := `
enabled: true
budgets:
  max_delivery_tokens: 1500
  max_hook_latency_ms: 60
  daily_housekeeping: 50
  daily_tokens: 100000
`
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadMachineConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadMachineConfig failed: %v", err)
	}

	if cfg.Budgets.MaxDeliveryTokens != 1500 {
		t.Errorf("MaxDeliveryTokens: got %d, want 1500", cfg.Budgets.MaxDeliveryTokens)
	}
	if cfg.Budgets.MaxHookLatencyMs != 60 {
		t.Errorf("MaxHookLatencyMs: got %d, want 60", cfg.Budgets.MaxHookLatencyMs)
	}
	if cfg.Budgets.DailyHousekeeping != 50 {
		t.Errorf("DailyHousekeeping: got %d, want 50", cfg.Budgets.DailyHousekeeping)
	}
	if cfg.Budgets.DailyTokens != 100000 {
		t.Errorf("DailyTokens: got %d, want 100000", cfg.Budgets.DailyTokens)
	}
}

func TestLoadMachineConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte("enabled: [broken"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMachineConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error parsing invalid YAML, got nil")
	}
}

func TestMachineConfig_JSON(t *testing.T) {
	jsonData := `{
		"enabled": false,
		"housekeeping_executor": "claude",
		"reports_surface": "agentapi",
		"tick_trigger": "sidecar",
		"budgets": {
			"max_delivery_tokens": 2000
		}
	}`

	var cfg MachineConfig
	if err := json.Unmarshal([]byte(jsonData), &cfg); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	if cfg.Enabled {
		t.Error("expected enabled to be false")
	}
	if cfg.HousekeepingExecutor != "claude" {
		t.Errorf("got %q, want 'claude'", cfg.HousekeepingExecutor)
	}
	if cfg.Budgets.MaxDeliveryTokens != 2000 {
		t.Errorf("got %d, want 2000", cfg.Budgets.MaxDeliveryTokens)
	}

	// Marshaling back to JSON
	bytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	if !strings.Contains(string(bytes), `"housekeeping_executor":"claude"`) {
		t.Errorf("expected marshaled JSON to have housekeeping_executor, got %s", string(bytes))
	}
}

func TestRepoConfig_Defaults(t *testing.T) {
	cfg := DefaultRepoConfig()
	if cfg.Subsystems == nil {
		t.Error("expected Subsystems map to be non-nil")
	}
	if len(cfg.Subsystems) != 0 {
		t.Errorf("expected 0 subsystems, got %d", len(cfg.Subsystems))
	}
}

func TestLoadRepoConfig_NotExist(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error loading repo config from non-existent path: %v", err)
	}
	if len(cfg.Subsystems) != 0 {
		t.Errorf("expected empty subsystems, got %d", len(cfg.Subsystems))
	}
}

func TestLoadRepoConfig_ValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, ".memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
subsystems:
  ota:
    title: OTA updates
    description: Manifest-first signed firmware updates
    globs:
      - modules/OTAUpdateManager/**
      - lib/OTADownloadEngine/**
  ble:
    title: Bluetooth LE
    globs:
      - modules/BLEManager/**
      - sdkconfig.ble.defaults
`
	if err := os.WriteFile(filepath.Join(memDir, "config.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig failed: %v", err)
	}

	if len(cfg.Subsystems) != 2 {
		t.Fatalf("expected 2 subsystems, got %d", len(cfg.Subsystems))
	}

	ota, ok := cfg.Subsystems["ota"]
	if !ok {
		t.Fatal("missing 'ota' subsystem")
	}
	if ota.Title != "OTA updates" {
		t.Errorf("ota title: got %q, want 'OTA updates'", ota.Title)
	}
	if ota.Description != "Manifest-first signed firmware updates" {
		t.Errorf("ota description: got %q", ota.Description)
	}
	if len(ota.Globs) != 2 {
		t.Errorf("ota globs: got %d globs, want 2", len(ota.Globs))
	}

	ble, ok := cfg.Subsystems["ble"]
	if !ok {
		t.Fatal("missing 'ble' subsystem")
	}
	if ble.Title != "Bluetooth LE" {
		t.Errorf("ble title: got %q, want 'Bluetooth LE'", ble.Title)
	}
}

func TestLoadRepoConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	memDir := filepath.Join(tmpDir, ".memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "config.yaml"), []byte("subsystems: [invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRepoConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error parsing invalid YAML, got nil")
	}
}

func TestEnsureStorageLayout_Idempotency(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-test-12345"

	// Run 1: Create storage layout
	if err := EnsureStorageLayout(dataDir, repoID, repoRoot); err != nil {
		t.Fatalf("first EnsureStorageLayout failed: %v", err)
	}

	// Verify all expected data-dir subdirectories exist
	expectedDataDirs := []string{
		"spool",
		"queue",
		"runs",
		"episodes",
		"pending",
		"alerts",
		"metrics",
		"logs",
		"state",
	}
	for _, sub := range expectedDataDirs {
		p := filepath.Join(dataDir, repoID, sub)
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected data dir %s to exist: %v", p, err)
		} else if !fi.IsDir() {
			t.Errorf("expected %s to be directory", p)
		}
	}

	// Verify all expected repo .memory subdirectories exist
	expectedRepoDirs := []string{
		"claims",
		filepath.Join("events", "reviews"),
		filepath.Join("events", "answers"),
		filepath.Join("views", "index"),
		filepath.Join("views", "subsystems"),
		".cache",
	}
	for _, sub := range expectedRepoDirs {
		p := filepath.Join(repoRoot, ".memory", sub)
		fi, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected repo dir %s to exist: %v", p, err)
		} else if !fi.IsDir() {
			t.Errorf("expected %s to be directory", p)
		}
	}

	// Verify .memory/.gitignore exists
	gitignorePath := filepath.Join(repoRoot, ".memory", ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Errorf("expected .gitignore to exist: %v", err)
	} else if !strings.Contains(string(data), ".cache") {
		t.Errorf("expected .gitignore to contain .cache, got %q", string(data))
	}

	// Place a sentinel file inside one directory to verify it isn't wiped
	sentinel := filepath.Join(dataDir, repoID, "spool", "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("sentinel-test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run 2: Verify idempotency by calling EnsureStorageLayout again
	if err := EnsureStorageLayout(dataDir, repoID, repoRoot); err != nil {
		t.Fatalf("second EnsureStorageLayout failed: %v", err)
	}

	// Sentinel file must still exist
	sentinelData, err := os.ReadFile(sentinel)
	if err != nil || string(sentinelData) != "sentinel-test" {
		t.Errorf("sentinel file was corrupted or deleted during idempotent run")
	}

	// Run 3: Without repoRoot specified, only dataDir layout is ensured
	anotherDataDir := t.TempDir()
	if err := EnsureStorageLayout(anotherDataDir, "repo-solo"); err != nil {
		t.Fatalf("EnsureStorageLayout with only dataDir and repoID failed: %v", err)
	}
	for _, sub := range expectedDataDirs {
		p := filepath.Join(anotherDataDir, "repo-solo", sub)
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			t.Errorf("expected %s to exist as directory", p)
		}
	}
}

func TestEnsureStorageLayout_Validation(t *testing.T) {
	dataDir := t.TempDir()
	if err := EnsureStorageLayout(dataDir, ""); err == nil {
		t.Error("expected error when repoID is empty")
	}
}

func TestMachineConfig_YAMLMarshal(t *testing.T) {
	cfg := MachineConfig{
		Enabled:              true,
		HousekeepingExecutor: "agy",
		ReportsSurface:       "agentapi",
		TickTrigger:          "sidecar",
		Budgets: BudgetsConfig{
			MaxDeliveryTokens: 2000,
			MaxHookLatencyMs:  50,
		},
	}

	bytes, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	var parsed MachineConfig
	if err := yaml.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	if !parsed.Enabled {
		t.Error("expected Enabled to be true")
	}
	if parsed.HousekeepingExecutor != "agy" {
		t.Errorf("got %q, want 'agy'", parsed.HousekeepingExecutor)
	}
	if parsed.Budgets.MaxDeliveryTokens != 2000 {
		t.Errorf("got %d, want 2000", parsed.Budgets.MaxDeliveryTokens)
	}
}

func TestRepoConfig_TurnNotes(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"", "on"},
		{"on", "on"},
		{"ON", "on"},
		{"  on  ", "on"},
		{"true", "on"},
		{"unknown", "on"},
		{"off", "off"},
		{"OFF", "off"},
		{"false", "off"},
		{"FALSE", "off"},
		{"disabled", "off"},
		{"  disabled ", "off"},
		{"fallback-only", "fallback-only"},
		{"FALLBACK-ONLY", "fallback-only"},
		{"fallback_only", "fallback-only"},
	}

	for _, tc := range tests {
		cfg := RepoConfig{TurnNotes: tc.raw}
		if got := cfg.TurnNotesMode(); got != tc.want {
			t.Errorf("TurnNotesMode(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}

	tmpDir := t.TempDir()
	orig := RepoConfig{
		TurnNotes:           "fallback-only",
		TurnNotesRoleOptOut: []string{"principal-architect", "chief-architect"},
	}
	if err := SaveRepoConfig(tmpDir, orig); err != nil {
		t.Fatalf("SaveRepoConfig failed: %v", err)
	}
	loaded, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig failed: %v", err)
	}
	if loaded.TurnNotesMode() != "fallback-only" {
		t.Errorf("loaded TurnNotesMode() = %q, want 'fallback-only'", loaded.TurnNotesMode())
	}
	if len(loaded.TurnNotesRoleOptOut) != 2 || loaded.TurnNotesRoleOptOut[0] != "principal-architect" || loaded.TurnNotesRoleOptOut[1] != "chief-architect" {
		t.Errorf("loaded TurnNotesRoleOptOut = %v, want [principal-architect chief-architect]", loaded.TurnNotesRoleOptOut)
	}
}


