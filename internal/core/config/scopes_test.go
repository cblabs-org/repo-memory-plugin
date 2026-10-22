/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package config_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func TestLoadScopeConfig_SampleYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
scopes:
  topics:
    - name: persistence-and-sqlite
      keywords: ["sqlite", "database", "ops", "concurrency"]
      description: "Persistence and SQLite concurrency"
  externals:
    - name: antigravity-hooks
      keywords: ["hook", "pre-tool", "post-tool"]
      tool_names: ["mcp_antigravity", "agy_hook"]
      description: "Antigravity hooks and integration"
  procedures:
    - name: release
      keywords: ["version", "tag"]
      commands: ["git tag", "make release"]
      description: "Release and tagging process"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample config: %v", err)
	}

	cfg, err := config.LoadScopeConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading scope config: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil scope config")
	}

	// Topics
	if len(cfg.Topics) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(cfg.Topics))
	}
	topic := cfg.Topics[0]
	if topic.Name != "persistence-and-sqlite" {
		t.Errorf("expected topic name persistence-and-sqlite, got %q", topic.Name)
	}
	if topic.Kind != dossiers.DossierKindTopic {
		t.Errorf("expected topic kind %q, got %q", dossiers.DossierKindTopic, topic.Kind)
	}
	if len(topic.Keywords) != 4 || topic.Keywords[0] != "sqlite" {
		t.Errorf("unexpected keywords: %v", topic.Keywords)
	}

	// Externals
	if len(cfg.Externals) != 1 {
		t.Fatalf("expected 1 external, got %d", len(cfg.Externals))
	}
	ext := cfg.Externals[0]
	if ext.Name != "antigravity-hooks" {
		t.Errorf("expected external name antigravity-hooks, got %q", ext.Name)
	}
	if ext.Kind != dossiers.DossierKindExternal {
		t.Errorf("expected external kind %q, got %q", dossiers.DossierKindExternal, ext.Kind)
	}
	if len(ext.ToolNames) != 2 || ext.ToolNames[0] != "mcp_antigravity" {
		t.Errorf("unexpected tool names: %v", ext.ToolNames)
	}

	// Procedures
	if len(cfg.Procedures) != 1 {
		t.Fatalf("expected 1 procedure, got %d", len(cfg.Procedures))
	}
	proc := cfg.Procedures[0]
	if proc.Name != "release" {
		t.Errorf("expected procedure name release, got %q", proc.Name)
	}
	if proc.Kind != dossiers.DossierKindProcedure {
		t.Errorf("expected procedure kind %q, got %q", dossiers.DossierKindProcedure, proc.Kind)
	}
	if len(proc.Commands) != 2 || proc.Commands[0] != "git tag" {
		t.Errorf("unexpected commands: %v", proc.Commands)
	}
}

func TestLoadScopeConfig_MissingScopesKey(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. YAML file with other keys but missing scopes
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
subsystems:
  core:
    description: "Core infrastructure"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadScopeConfig(configPath)
	if err != nil {
		t.Fatalf("expected no error for missing scopes key, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil ScopeConfig")
	}
	if len(cfg.Topics) != 0 || len(cfg.Externals) != 0 || len(cfg.Procedures) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}

	// 2. Empty file
	emptyPath := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(emptyPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty config: %v", err)
	}
	cfg, err = config.LoadScopeConfig(emptyPath)
	if err != nil || cfg == nil {
		t.Fatalf("expected empty config for empty file, got cfg=%v, err=%v", cfg, err)
	}

	// 3. Non-existent file
	cfg, err = config.LoadScopeConfig(filepath.Join(tmpDir, "does-not-exist.yaml"))
	if err != nil || cfg == nil {
		t.Fatalf("expected empty config for non-existent file, got cfg=%v, err=%v", cfg, err)
	}

	// 4. Empty path
	cfg, err = config.LoadScopeConfig("")
	if err != nil || cfg == nil {
		t.Fatalf("expected empty config for empty path, got cfg=%v, err=%v", cfg, err)
	}
}

func TestDeriveModuleScopes_RollupAndNaming(t *testing.T) {
	anchors := []string{
		// 3 files in internal/core/transcript_parser/ -> > 2 files, so does not roll up
		"internal/core/transcript_parser/parser.go",
		"internal/core/transcript_parser/lexer.go",
		"internal/core/transcript_parser/ast.go#ASTNode",

		// 3 files in internal/core/config/ -> does not roll up
		"internal/core/config/scopes.go:ScopeConfig",
		"internal/core/config/scopes_test.go",
		"internal/core/config/config.go",

		// 2 files in internal/core/episode/ -> <= 2 files, so rolls up to internal/core
		"internal/core/episode/store.go",
		"internal/core/episode/commit_linkage.go",

		// 3 files in cmd/repo_memory/ -> does not roll up, keeps cmd/ prefix
		"cmd/repo_memory/main.go",
		"cmd/repo_memory/root.go",
		"cmd/repo_memory/version.go",
	}

	scopes := config.DeriveModuleScopes(anchors)
	if len(scopes) != 4 {
		t.Fatalf("expected 4 derived scopes, got %d", len(scopes))
	}

	// Scopes should be sorted by Name
	expectedNames := []string{
		"cmd/repo-memory",
		"core",
		"core/config",
		"core/transcript-parser",
	}

	for i, exp := range expectedNames {
		if scopes[i].Name != exp {
			t.Errorf("scope[%d]: expected Name %q, got %q", i, exp, scopes[i].Name)
		}
		if scopes[i].Kind != dossiers.DossierKindModule {
			t.Errorf("scope[%d]: expected Kind module, got %q", i, scopes[i].Kind)
		}
	}

	// Verify cmd/repo-memory
	cmdScope := scopes[0]
	if len(cmdScope.PathGlobs) != 3 {
		t.Errorf("expected 3 path globs in cmd/repo-memory, got %d", len(cmdScope.PathGlobs))
	}

	// Verify core (rolled up from episode)
	coreScope := scopes[1]
	if len(coreScope.PathGlobs) != 2 {
		t.Errorf("expected 2 rolled-up path globs in core, got %d (%v)", len(coreScope.PathGlobs), coreScope.PathGlobs)
	}

	// Verify core/config and symbols
	configScope := scopes[2]
	if len(configScope.PathGlobs) != 3 {
		t.Errorf("expected 3 path globs in core/config, got %d", len(configScope.PathGlobs))
	}
	if len(configScope.Symbols) != 1 || configScope.Symbols[0] != "ScopeConfig" {
		t.Errorf("expected ScopeConfig symbol in core/config, got %v", configScope.Symbols)
	}

	// Verify core/transcript-parser
	tpScope := scopes[3]
	if len(tpScope.PathGlobs) != 3 {
		t.Errorf("expected 3 path globs in core/transcript-parser, got %d", len(tpScope.PathGlobs))
	}
	if len(tpScope.Symbols) != 1 || tpScope.Symbols[0] != "ASTNode" {
		t.Errorf("expected ASTNode symbol in core/transcript-parser, got %v", tpScope.Symbols)
	}
}

func TestBuildAliasMap_SplitOps(t *testing.T) {
	tmpDir := t.TempDir()
	writer := ops.NewWriter(tmpDir)

	now := time.Now().UTC()

	// Batch 1: split "core/transcript-parser" -> "core/transcript-v2"
	op1 := ops.Op{
		Type:         ops.OpSplit,
		DossierScope: "core/transcript-parser",
		Timestamp:    now.Add(1 * time.Minute),
		Source:       ops.SourceConsolidation,
		Fields: map[string]any{
			"new_dossiers": []string{"core/transcript-v2"},
			"entry_moves":  map[string]string{"inv-01": "core/transcript-v2"},
		},
	}
	b1 := ops.NewBatch([]ops.Op{op1})
	b1.CreatedAt = now.Add(1 * time.Minute)
	if err := writer.Write(b1); err != nil {
		t.Fatalf("write batch 1: %v", err)
	}

	// Batch 2: split "core/transcript-v2" -> "core/transcript-v3"
	op2 := ops.Op{
		Type:         ops.OpSplit,
		DossierScope: "core/transcript-v2",
		Timestamp:    now.Add(2 * time.Minute),
		Source:       ops.SourceConsolidation,
		Fields: map[string]any{
			"new_dossiers": []string{"core/transcript-v3"},
			"entry_moves":  map[string]string{"inv-01": "core/transcript-v3"},
		},
	}
	b2 := ops.NewBatch([]ops.Op{op2})
	b2.CreatedAt = now.Add(2 * time.Minute)
	if err := writer.Write(b2); err != nil {
		t.Fatalf("write batch 2: %v", err)
	}

	reader := ops.NewReader(tmpDir)
	aliasMap, err := config.BuildAliasMap(reader, context.Background())
	if err != nil {
		t.Fatalf("BuildAliasMap failed: %v", err)
	}
	if aliasMap == nil {
		t.Fatal("expected non-nil alias map")
	}

	// Verify alias chain: "core/transcript-parser" -> "core/transcript-v2" -> "core/transcript-v3"
	if got := aliasMap.Resolve("core/transcript-parser"); got != "core/transcript-v3" {
		t.Errorf("expected chain resolution to core/transcript-v3, got %q", got)
	}
	if got := aliasMap.Resolve("core/transcript-v2"); got != "core/transcript-v3" {
		t.Errorf("expected resolution to core/transcript-v3, got %q", got)
	}
	if got := aliasMap.Resolve("core/transcript-v3"); got != "core/transcript-v3" {
		t.Errorf("expected current scope to resolve to itself, got %q", got)
	}
	if got := aliasMap.Resolve("unknown-scope"); got != "unknown-scope" {
		t.Errorf("expected unknown scope to resolve to itself, got %q", got)
	}
}

func TestAliasMap_ResolveChainAndCycle(t *testing.T) {
	aliasMap := config.NewAliasMap()

	// 1. Multi-step chain A -> B -> C -> D
	aliasMap.SetAlias("scope-a", "scope-b")
	aliasMap.SetAlias("scope-b", "scope-c")
	aliasMap.SetAlias("scope-c", "scope-d")

	if got := aliasMap.Resolve("scope-a"); got != "scope-d" {
		t.Errorf("expected scope-a to resolve to scope-d, got %q", got)
	}
	if got := aliasMap.Resolve("scope-b"); got != "scope-d" {
		t.Errorf("expected scope-b to resolve to scope-d, got %q", got)
	}

	// 2. Cycle prevention X -> Y -> X
	cyclicMap := config.NewAliasMap()
	cyclicMap.SetAlias("cycle-x", "cycle-y")
	cyclicMap.SetAlias("cycle-y", "cycle-x")

	// Must terminate safely and not hang
	resolved := cyclicMap.Resolve("cycle-x")
	if resolved != "cycle-x" && resolved != "cycle-y" {
		t.Errorf("expected cyclic resolution to terminate at cycle-x or cycle-y, got %q", resolved)
	}

	// 3. Nil alias map
	var nilMap *config.AliasMap
	if got := nilMap.Resolve("foo"); got != "foo" {
		t.Errorf("expected nil alias map to return original scope, got %q", got)
	}
}

func TestScopeResolver_ResolveScope(t *testing.T) {
	cfg := &config.ScopeConfig{
		Topics: []config.DeclaredScope{
			{
				Name:        "persistence-and-sqlite",
				Keywords:    []string{"sqlite", "database", "sql"},
				Description: "SQLite storage and queries",
			},
		},
		Externals: []config.DeclaredScope{
			{
				Name:        "antigravity-hooks",
				Keywords:    []string{"hook"},
				ToolNames:   []string{"mcp_antigravity", "agy_hook"},
				Description: "Antigravity hook integration",
			},
		},
		Procedures: []config.DeclaredScope{
			{
				Name:        "release",
				Keywords:    []string{"release"},
				Commands:    []string{"git tag", "make release"},
				Description: "Production release procedure",
			},
		},
	}

	modules := []config.ModuleScope{
		{
			Name:      "core/transcript-parser",
			PathGlobs: []string{"internal/core/transcript_parser/parser.go"},
			Symbols:   []string{"ParseTranscript"},
		},
		{
			Name:      "core/config",
			PathGlobs: []string{"internal/core/config/scopes.go", "internal/core/config/config.go"},
			Symbols:   []string{"ScopeConfig"},
		},
	}

	aliases := config.NewAliasMap()
	aliases.SetAlias("old-sqlite-topic", "persistence-and-sqlite")
	aliases.SetAlias("legacy-parser", "core/transcript-parser")

	resolver := config.NewScopeResolver(cfg, modules, aliases)

	tests := []struct {
		name     string
		anchors  []string
		keywords []string
		expected string
	}{
		{
			name:     "1. Alias direct match in anchors",
			anchors:  []string{"old-sqlite-topic"},
			expected: "persistence-and-sqlite",
		},
		{
			name:     "2. Alias direct match in keywords",
			keywords: []string{"legacy-parser"},
			expected: "core/transcript-parser",
		},
		{
			name:     "3. Anchor matching module by path prefix",
			anchors:  []string{"internal/core/config/scopes.go"},
			expected: "core/config",
		},
		{
			name:     "4. Anchor matching module by relative slug",
			anchors:  []string{"internal/core/transcript_parser/other.go"},
			expected: "core/transcript-parser",
		},
		{
			name:     "5. Anchor matching module by symbol",
			anchors:  []string{"symbol:ParseTranscript"},
			expected: "core/transcript-parser",
		},
		{
			name:     "6. Keyword matching declared topic",
			keywords: []string{"sqlite", "query"},
			expected: "persistence-and-sqlite",
		},
		{
			name:     "7. Keyword matching external by tool name",
			keywords: []string{"mcp_antigravity"},
			expected: "antigravity-hooks",
		},
		{
			name:     "8. Keyword matching procedure by command",
			keywords: []string{"git tag"},
			expected: "release",
		},
		{
			name:     "9. No match returns empty string",
			anchors:  []string{"docs/unknown.md"},
			keywords: []string{"unrelated"},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolver.ResolveScope(tc.anchors, tc.keywords)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestScopeResolver_AllScopesAndScopeKind(t *testing.T) {
	cfg := &config.ScopeConfig{
		Topics: []config.DeclaredScope{
			{Name: "persistence-and-sqlite"},
		},
		Externals: []config.DeclaredScope{
			{Name: "antigravity-hooks"},
		},
		Procedures: []config.DeclaredScope{
			{Name: "release"},
		},
	}

	modules := []config.ModuleScope{
		{Name: "core/config"},
		{Name: "core/transcript-parser"},
	}

	aliases := config.NewAliasMap()
	aliases.SetAlias("legacy-topic", "persistence-and-sqlite")

	resolver := config.NewScopeResolver(cfg, modules, aliases)

	all := resolver.AllScopes()
	expected := []string{
		"antigravity-hooks",
		"core/config",
		"core/transcript-parser",
		"legacy-topic",
		"persistence-and-sqlite",
		"release",
	}

	if !reflect.DeepEqual(all, expected) {
		t.Errorf("expected AllScopes %v, got %v", expected, all)
	}

	// Verify ScopeKind
	if k := resolver.ScopeKind("core/config"); k != dossiers.DossierKindModule {
		t.Errorf("expected module kind, got %q", k)
	}
	if k := resolver.ScopeKind("persistence-and-sqlite"); k != dossiers.DossierKindTopic {
		t.Errorf("expected topic kind, got %q", k)
	}
	if k := resolver.ScopeKind("legacy-topic"); k != dossiers.DossierKindTopic {
		t.Errorf("expected aliased topic kind, got %q", k)
	}
	if k := resolver.ScopeKind("antigravity-hooks"); k != dossiers.DossierKindExternal {
		t.Errorf("expected external kind, got %q", k)
	}
	if k := resolver.ScopeKind("release"); k != dossiers.DossierKindProcedure {
		t.Errorf("expected procedure kind, got %q", k)
	}
	if k := resolver.ScopeKind("nonexistent"); k != "" {
		t.Errorf("expected empty kind for nonexistent scope, got %q", k)
	}
}
