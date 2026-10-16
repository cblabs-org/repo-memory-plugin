package seed

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestResolveDocSeedGlobs(t *testing.T) {
	repoRoot := t.TempDir()

	// Curated docs that SHOULD match default globs
	writeTestFile(t, repoRoot, "docs/architecture.md", "# Architecture\n- Core invariant.\n")
	writeTestFile(t, repoRoot, "docs/guides/hooks-guide.md", "# Hooks Guide\n- Hook rule.\n")
	writeTestFile(t, repoRoot, ".agents/skills/custom-skill/SKILL.md", "---\nname: custom-skill\ndescription: Hand-written skill\n---\n# Custom Skill\n")

	// Files that MUST be excluded by default globs
	writeTestFile(t, repoRoot, ".agents/skills/principal-architect-workspace/pr-1/wo1/WORKORDER.md", "# WO1\n")
	writeTestFile(t, repoRoot, ".agents/skills/principal-architect-workspace/SKILL.md", "# PA Skill\n")
	writeTestFile(t, repoRoot, "docs/evaluations/eval-01.md", "# Eval\n")
	writeTestFile(t, repoRoot, "docs/experiments/exp-01.md", "# Experiment\n")
	writeTestFile(t, repoRoot, ".memory/dossiers/core.md", "# Dossier\n")
	writeTestFile(t, repoRoot, ".agents/skills/rm-auto/SKILL.md", "---\nname: rm-auto\ngenerated_by: repo-memory\n---\n# Auto Skill\n")
	writeTestFile(t, repoRoot, "docs/notes.txt", "not markdown\n")

	got, err := config.ResolveDocSeedGlobs(repoRoot, config.DefaultRepoConfig())
	if err != nil {
		t.Fatalf("ResolveDocSeedGlobs default failed: %v", err)
	}

	want := []string{
		".agents/skills/custom-skill/SKILL.md",
		"docs/architecture.md",
		"docs/guides/hooks-guide.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default ResolveDocSeedGlobs:\ngot:  %v\nwant: %v", got, want)
	}

	// Custom doc_seeds globs (including explicit exact path in docs/evaluations/)
	writeTestFile(t, repoRoot, "SPEC.md", "# Spec\n")
	customCfg := config.RepoConfig{
		DocSeeds: []string{
			"SPEC.md",
			"docs/guides/*.md",
			"docs/evaluations/eval-01.md",
			".agents/skills/rm-auto/SKILL.md", // still excluded because generated_by: repo-memory
		},
	}
	gotCustom, err := config.ResolveDocSeedGlobs(repoRoot, customCfg)
	if err != nil {
		t.Fatalf("ResolveDocSeedGlobs custom failed: %v", err)
	}
	wantCustom := []string{
		"SPEC.md",
		"docs/evaluations/eval-01.md",
		"docs/guides/hooks-guide.md",
	}
	if !reflect.DeepEqual(gotCustom, wantCustom) {
		t.Fatalf("custom ResolveDocSeedGlobs:\ngot:  %v\nwant: %v", gotCustom, wantCustom)
	}
}

func TestSeedDocs_InitialAndIdempotent(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	opsDir := filepath.Join(repoRoot, ".memory", "ops")

	// Create realistic repository source files for anchor validation
	writeTestFile(t, repoRoot, "internal/core/ops/op.go", "package ops\n")
	writeTestFile(t, repoRoot, "packaging/antigravity/hooks.json", "{}\n")

	hooksGuideContent := `---
title: Hooks Guide
---
# Antigravity Hooks Guide

## Invariants & Rules
- **Hook Timeout**: All hooks configured in ` + "`packaging/antigravity/hooks.json`" + ` must complete within 50ms to avoid blocking the turn loop.
- **Gotcha — Stale State**: Beware the lifetime quirk in ` + "`packaging/antigravity/hooks.json`" + ` where unescaped paths are ignored if not relative.
- **Failure Signature**: Calling an invalid hook command in ` + "`internal/core/ops/op.go`" + ` fails with error code 404 when the binary path is missing.
- Unanchored general prose remark with no repository path or scope match at all.
`
	writeTestFile(t, repoRoot, "docs/hooks-guide.md", hooksGuideContent)

	skillContent := `---
name: test-skill
description: Test procedure skill
---
# Test Skill

References ` + "`internal/core/ops/op.go`" + `.

## Verification Procedure
- Run the core ops test suite via ` + "`go test ./internal/core/ops/...`" + ` to validate batch serialization.
`
	writeTestFile(t, repoRoot, ".agents/skills/test-skill/SKILL.md", skillContent)

	reader := ops.NewReader(opsDir)
	writer := ops.NewWriter(opsDir)

	opts := DocSeedOptions{
		RepoRoot: repoRoot,
		OpsDir:   opsDir,
		RepoCfg:  config.DefaultRepoConfig(),
	}

	// First run: should seed entries from both docs
	res1, err := SeedDocs(ctx, opts, &config.ScopeConfig{}, reader, writer)
	if err != nil {
		t.Fatalf("SeedDocs initial run failed: %v", err)
	}

	if res1.FilesScanned != 2 {
		t.Errorf("FilesScanned: got %d, want 2", res1.FilesScanned)
	}
	if res1.FilesSeeded != 2 {
		t.Errorf("FilesSeeded: got %d, want 2", res1.FilesSeeded)
	}
	if res1.FilesSkipped != 0 {
		t.Errorf("FilesSkipped: got %d, want 0", res1.FilesSkipped)
	}
	if res1.EntriesAdded < 4 {
		t.Fatalf("expected at least 4 entries added, got %d", res1.EntriesAdded)
	}

	expectedHooksSHA := ComputeGitBlobSHA(repoRoot, "docs/hooks-guide.md", []byte(hooksGuideContent))
	expectedSkillSHA := ComputeGitBlobSHA(repoRoot, ".agents/skills/test-skill/SKILL.md", []byte(skillContent))

	proj := dossiers.NewProjector(reader)
	dossiersMap, err := proj.Project(ctx)
	if err != nil {
		t.Fatalf("Project failed: %v", err)
	}

	var allEntries []dossiers.Entry
	for _, d := range dossiersMap {
		allEntries = append(allEntries, d.Entries...)
	}
	if len(allEntries) != res1.EntriesAdded {
		t.Fatalf("projected active entries (%d) != EntriesAdded (%d)", len(allEntries), res1.EntriesAdded)
	}

	foundTypes := make(map[dossiers.EntryType]bool)
	foundKinds := make(map[dossiers.CognitiveKind]bool)

	for _, e := range allEntries {
		if e.Source != ops.SourceDocSeed {
			t.Errorf("entry %s Source = %q, want %q", e.ID, e.Source, ops.SourceDocSeed)
		}
		if !strings.HasPrefix(e.ID, "docseed-") {
			t.Errorf("entry ID %q does not have docseed- prefix", e.ID)
		}
		if len(e.Anchors) == 0 {
			t.Errorf("entry %s has empty anchors", e.ID)
		}
		if !e.Kind.Valid() {
			t.Errorf("entry %s has invalid kind %q", e.ID, e.Kind)
		}
		foundTypes[e.Type] = true
		foundKinds[e.Kind] = true

		switch e.SourceDocPath {
		case "docs/hooks-guide.md":
			if e.SourceDocBlobSHA != expectedHooksSHA {
				t.Errorf("entry %s blob SHA = %q, want %q", e.ID, e.SourceDocBlobSHA, expectedHooksSHA)
			}
		case ".agents/skills/test-skill/SKILL.md":
			if e.SourceDocBlobSHA != expectedSkillSHA {
				t.Errorf("entry %s blob SHA = %q, want %q", e.ID, e.SourceDocBlobSHA, expectedSkillSHA)
			}
		default:
			t.Errorf("unexpected SourceDocPath %q on entry %s", e.SourceDocPath, e.ID)
		}
	}

	if !foundTypes[dossiers.EntryTypeInvariant] {
		t.Error("expected at least one invariant entry")
	}
	if !foundTypes[dossiers.EntryTypeGotcha] {
		t.Error("expected at least one gotcha entry")
	}
	if !foundTypes[dossiers.EntryTypeFailureSignature] {
		t.Error("expected at least one failure-signature entry")
	}
	if !foundTypes[dossiers.EntryTypeRecipe] {
		t.Error("expected at least one recipe entry")
	}
	if !foundKinds[dossiers.CognitiveKindSemantic] || !foundKinds[dossiers.CognitiveKindProcedural] {
		t.Errorf("expected both semantic and procedural kinds, got %v", foundKinds)
	}

	// Second run with unchanged files: must be 100% idempotent
	res2, err := SeedDocs(ctx, opts, &config.ScopeConfig{}, reader, writer)
	if err != nil {
		t.Fatalf("SeedDocs second run failed: %v", err)
	}
	if res2.FilesSkipped != 2 {
		t.Errorf("second run FilesSkipped: got %d, want 2", res2.FilesSkipped)
	}
	if len(res2.Ops) != 0 {
		t.Errorf("second run emitted %d ops, want 0", len(res2.Ops))
	}
}

func TestSeedDocs_ModifyAndDelete(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	opsDir := filepath.Join(repoRoot, ".memory", "ops")

	writeTestFile(t, repoRoot, "internal/core/ops/op.go", "package ops\n")
	writeTestFile(t, repoRoot, "packaging/antigravity/hooks.json", "{}\n")

	initialHooksDoc := `# Antigravity Hooks Guide

## Rules
- **Hook Timeout**: All hooks configured in ` + "`packaging/antigravity/hooks.json`" + ` must complete within 50ms.
- **Legacy Fallback**: Old fallback in ` + "`internal/core/ops/op.go`" + ` must never be called directly by external callers.
`
	writeTestFile(t, repoRoot, "docs/hooks-guide.md", initialHooksDoc)

	skillDoc := `# Test Skill
- Run the ops test suite in ` + "`internal/core/ops/op.go`" + ` via ` + "`go test ./internal/core/ops/...`" + ` before committing.
`
	writeTestFile(t, repoRoot, ".agents/skills/test-skill/SKILL.md", skillDoc)

	reader := ops.NewReader(opsDir)
	writer := ops.NewWriter(opsDir)
	opts := DocSeedOptions{
		RepoRoot: repoRoot,
		OpsDir:   opsDir,
		RepoCfg:  config.DefaultRepoConfig(),
	}

	res1, err := SeedDocs(ctx, opts, &config.ScopeConfig{}, reader, writer)
	if err != nil {
		t.Fatalf("initial SeedDocs failed: %v", err)
	}
	if res1.EntriesAdded != 3 {
		t.Fatalf("expected 3 entries added initially, got %d", res1.EntriesAdded)
	}

	// Modify docs/hooks-guide.md (change Hook Timeout from 50ms to 35ms, remove Legacy Fallback)
	modifiedHooksDoc := `# Antigravity Hooks Guide

## Rules
- **Hook Timeout**: All hooks configured in ` + "`packaging/antigravity/hooks.json`" + ` must complete within 35ms.
`
	writeTestFile(t, repoRoot, "docs/hooks-guide.md", modifiedHooksDoc)
	newHooksSHA := ComputeGitBlobSHA(repoRoot, "docs/hooks-guide.md", []byte(modifiedHooksDoc))

	// Delete .agents/skills/test-skill/SKILL.md
	if err := os.Remove(filepath.Join(repoRoot, ".agents", "skills", "test-skill", "SKILL.md")); err != nil {
		t.Fatalf("removing skill file: %v", err)
	}

	res2, err := SeedDocs(ctx, opts, &config.ScopeConfig{}, reader, writer)
	if err != nil {
		t.Fatalf("second SeedDocs after modify+delete failed: %v", err)
	}

	if res2.EntriesAmend != 1 {
		t.Errorf("expected 1 amended entry (Hook Timeout), got %d", res2.EntriesAmend)
	}
	if res2.EntriesRetire != 2 {
		t.Errorf("expected 2 retired entries (1 removed bullet + 1 deleted skill doc), got %d", res2.EntriesRetire)
	}

	proj := dossiers.NewProjector(reader)
	dossiersMap, err := proj.Project(ctx)
	if err != nil {
		t.Fatalf("Project after modify+delete failed: %v", err)
	}

	var activeEntries []dossiers.Entry
	var retiredEntries []dossiers.Entry
	for _, d := range dossiersMap {
		activeEntries = append(activeEntries, d.Entries...)
		retiredEntries = append(retiredEntries, d.History...)
	}

	if len(activeEntries) != 1 {
		t.Fatalf("expected 1 active entry remaining, got %d", len(activeEntries))
	}
	if activeEntries[0].SourceDocBlobSHA != newHooksSHA {
		t.Errorf("active entry blob SHA = %q, want %q", activeEntries[0].SourceDocBlobSHA, newHooksSHA)
	}
	if !strings.Contains(activeEntries[0].Text, "35ms") {
		t.Errorf("expected active entry text to contain updated '35ms', got %q", activeEntries[0].Text)
	}

	if len(retiredEntries) != 2 {
		t.Fatalf("expected 2 retired entries in history, got %d", len(retiredEntries))
	}

	foundDeletedReason := false
	foundUpdatedReason := false
	for _, re := range retiredEntries {
		if strings.Contains(re.RetiredReason, "source_doc_deleted: .agents/skills/test-skill/SKILL.md") {
			foundDeletedReason = true
		}
		if strings.Contains(re.RetiredReason, "source_doc_updated: docs/hooks-guide.md") {
			foundUpdatedReason = true
		}
	}
	if !foundDeletedReason {
		t.Errorf("expected retired entry with source_doc_deleted reason, got %+v", retiredEntries)
	}
	if !foundUpdatedReason {
		t.Errorf("expected retired entry with source_doc_updated reason, got %+v", retiredEntries)
	}
}

func TestParseDocToOps_FiltersTablesFragmentsAndExcludedAnchors(t *testing.T) {
	repoRoot := t.TempDir()
	writeTestFile(t, repoRoot, "internal/core/scrub/scrub.go", "package scrub\n")
	writeTestFile(t, repoRoot, "docs/experiments/spike-summary.md", "# Spike\n")
	writeTestFile(t, repoRoot, "docs/evaluations/x6-lite-verdict.md", "# Verdict\n")
	writeTestFile(t, repoRoot, ".agents/skills/principal-architect-workspace/pr-1/WORKORDER.md", "# WO\n")

	docContent := `# Hardening Guide

## Scrubber Rules (` + "`internal/core/scrub/scrub.go`" + `)

| Stage | Description |
|---|---|
| 1 | **Structural** — Private-key blocks in ` + "`internal/core/scrub/scrub.go`" + ` |

Paragraph mentioning ` + "`internal/core/scrub/scrub.go`" + ` before a sentence-fragment list:
- every UUID is replaced, and that includes conversation IDs;
- input prepared for the lean agent (v2 §5.6);
- Short fragment.
- **Valid Invariant**: Every scrubber rule in ` + "`internal/core/scrub/scrub.go`" + ` must run deterministically before persistence.

## Experiment Citations
- **Experiment Citation**: Validated in ` + "`docs/experiments/spike-summary.md`" + ` and ` + "`docs/evaluations/x6-lite-verdict.md`" + ` and ` + "`.agents/skills/principal-architect-workspace/pr-1/WORKORDER.md`" + `.
`
	isValid := buildRepoPathValidator(SeedOptions{RepoRoot: repoRoot})
	opsList := ParseDocToOps("docs/hardening.md", docContent, "abc12345", time.Now().UTC(), nil, isValid)

	if len(opsList) != 1 {
		t.Fatalf("expected exactly 1 high-signal op, got %d: %+v", len(opsList), opsList)
	}
	gotText, _ := opsList[0].Fields["text"].(string)
	if !strings.Contains(gotText, "Valid Invariant") {
		t.Errorf("unexpected op text: %q", gotText)
	}
}

func TestParseDocToOps_NoPollutedScopes(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	opsDir := filepath.Join(repoRoot, ".memory", "ops")

	// Create dot-prefixed and root-level files in repoRoot
	writeTestFile(t, repoRoot, ".memory/config.yaml", "doc_seeds:\n  - docs/**/*.md\n")
	writeTestFile(t, repoRoot, ".agents/skills/custom/SKILL.md", "---\nname: custom\ndescription: custom\n---\n# Custom\n")
	writeTestFile(t, repoRoot, ".agents/agents/distiller.md", "---\nname: distiller\ndescription: distiller\n---\n")
	writeTestFile(t, repoRoot, "Makefile", "build:\n\tgo build ./...\n")
	writeTestFile(t, repoRoot, "internal/core/ops/op.go", "package ops\n")

	docContent := `# Build & Configuration Guide

## Root & Dotfile Rules
- **Memory Config Schema**: Store workspace glob configuration inside ` + "`.memory/config.yaml`" + ` so doc seeding stays deterministic.
- **Skill Layout**: Place reusable workspace skills under ` + "`.agents/skills`" + ` with valid YAML frontmatter headers.
- **Agent Definition**: Define headless distillation agents inside ` + "`.agents/agents/distiller.md`" + ` without default tools.
- **Makefile Entrypoint**: Run the top-level ` + "`Makefile`" + ` target to build all binaries cleanly across platforms.
`
	writeTestFile(t, repoRoot, "docs/config-guide.md", docContent)
	// Also create a markdown file with zero extractable anchored bullets to verify FilesSkipped
	writeTestFile(t, repoRoot, "docs/empty-guide.md", "# Empty Guide\n\nJust introductory prose with no bullets.\n")

	reader := ops.NewReader(opsDir)
	writer := ops.NewWriter(opsDir)
	opts := DocSeedOptions{
		RepoRoot: repoRoot,
		OpsDir:   opsDir,
		RepoCfg:  config.DefaultRepoConfig(),
	}

	res, err := SeedDocs(ctx, opts, &config.ScopeConfig{}, reader, writer)
	if err != nil {
		t.Fatalf("SeedDocs failed: %v", err)
	}
	if res.EntriesAdded != 4 {
		t.Fatalf("expected 4 entries added from docs/config-guide.md, got %d", res.EntriesAdded)
	}
	// docs/empty-guide.md and .agents/skills/custom/SKILL.md have 0 candidate ops -> both should be marked skipped
	if res.FilesSeeded != 1 {
		t.Errorf("FilesSeeded = %d, want 1", res.FilesSeeded)
	}
	if res.FilesSkipped != 2 {
		t.Errorf("FilesSkipped = %d, want 2", res.FilesSkipped)
	}
	for _, f := range res.Files {
		if f.Added == 0 && f.Amended == 0 && f.Retired == 0 && !f.Skipped {
			t.Errorf("expected file %s with 0 changes to have Skipped=true", f.Path)
		}
	}

	for _, op := range res.Ops {
		if isPollutedScope(op.DossierScope) {
			t.Errorf("op %s assigned polluted DossierScope %q", op.EntryID, op.DossierScope)
		}
		if op.DossierScope != "root" {
			t.Errorf("op %s DossierScope = %q, want \"root\"", op.EntryID, op.DossierScope)
		}
		anchors, _ := op.Fields["anchors"].([]string)
		if len(anchors) == 0 {
			t.Errorf("expected op %s to preserve its file anchor in Fields[\"anchors\"]", op.EntryID)
		}
	}

	// Verify mutual idempotence with RunOpsGC: zero scopes should need repair
	gcStats, err := RunOpsGC(ctx, GCOptions{
		OpsDir:     opsDir,
		ConfigPath: filepath.Join(repoRoot, ".memory", "config.yaml"),
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("RunOpsGC failed: %v", err)
	}
	if gcStats.ScopesRepaired != 0 {
		t.Errorf("expected RunOpsGC ScopesRepaired == 0 after SeedDocs, got %d", gcStats.ScopesRepaired)
	}
}

func TestSeedDocs_ESP32SkillAndDeveloperDocs(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	opsDir := filepath.Join(repoRoot, ".memory", "ops")

	// Create ESP32 App Foundation repository files across modules, interfaces, and root
	writeTestFile(t, repoRoot, "platformio.ini", "[env:native]\nplatform = native\n")
	writeTestFile(t, repoRoot, "include/AppFoundation.h", "#pragma once\n")
	writeTestFile(t, repoRoot, "include/interfaces/IDataSource.hpp", "#pragma once\n")
	writeTestFile(t, repoRoot, "include/interfaces/IWebContributor.hpp", "#pragma once\n")
	writeTestFile(t, repoRoot, "include/interfaces/IMCPContributor.hpp", "#pragma once\n")
	writeTestFile(t, repoRoot, "include/interfaces/IDisplayBoard.hpp", "#pragma once\n")
	writeTestFile(t, repoRoot, "include/types/FetchContext.hpp", "#pragma once\n")
	writeTestFile(t, repoRoot, "modules/Application/Application.hpp", "#pragma once\nclass Application { public: void registerModule(void*); void begin(); };\n")
	writeTestFile(t, repoRoot, "modules/Application/Application.cpp", "// Application\n")
	writeTestFile(t, repoRoot, "modules/DataManager/DataManager.hpp", "#pragma once\n#include \"interfaces/IDataSource.hpp\"\nclass DataManager { void add(IDataSource* s, FetchContext* c); };\n")
	writeTestFile(t, repoRoot, "modules/DataManager/DataManager.cpp", "// DataManager\n")
	writeTestFile(t, repoRoot, "modules/U8G2Display/U8G2Display.hpp", "#pragma once\n#include \"interfaces/IDisplayBoard.hpp\"\nclass U8G2Display { void addBoard(IDisplayBoard* b); };\n")
	writeTestFile(t, repoRoot, "modules/U8G2Display/U8G2Display.cpp", "// U8G2Display\n")
	writeTestFile(t, repoRoot, "modules/WebServer/WebServer.hpp", "#pragma once\n#include \"interfaces/IWebContributor.hpp\"\nclass WebServer { void add(IWebContributor* w); };\n")
	writeTestFile(t, repoRoot, "modules/MCPServer/MCPServer.hpp", "#pragma once\n#include \"interfaces/IMCPContributor.hpp\"\nclass MCPServer { void add(IMCPContributor* m); };\n")

	frameworkExpertSkill := `---
name: framework-expert
description: Expert assistant for the ESP32 App Foundation firmware framework
---
# ESP32 App Foundation Expert

## Quick Answers

### How do I register a module?
Call ` + "`app.registerModule(&myModule);`" + ` inside ` + "`setup()`" + ` before ` + "`app.begin()`" + ` so compile-time introspection wires every implemented contract.

### What include do I use?
Always use ` + "`#include \"AppFoundation.h\"`" + ` rather than coupling directly to internal framework header paths.

## Data Source Resilience

### The FetchContext Contract
- **Shared TLS Client**: Always use ` + "`ctx.client`" + ` inside ` + "`IDataSource::fetchUpdate`" + ` managed by ` + "`DataManager`" + ` instead of constructing a raw TLS client.
- **Display Boards**: Implement ` + "`IDisplayBoard`" + ` to register carousel pages with ` + "`U8G2DisplayModule`" + `.

## Trigger Reference
- **Getting started**: "how do I create a module", "register a module", "weather station".
`
	writeTestFile(t, repoRoot, ".agents/skills/framework-expert/SKILL.md", frameworkExpertSkill)

	creatingModuleDoc := `# Creating a Module

## 1. Module Registration Rules
See [IDataSource](../../include/interfaces/IDataSource.hpp) and [IWebContributor](../../include/interfaces/IWebContributor.hpp).

> [!IMPORTANT]
> **Rule:** Every module header must be self-contained and use ` + "`#pragma once`" + ` while registering MCP tools via ` + "`IMCPContributor`" + `.

- **Late Registration Panics**: Calling ` + "`app.registerModule()`" + ` after ` + "`app.begin()`" + ` triggers an immediate panic because registries are already locked.

## 2. Native Unit Testing
- **Run Native Tests**: Validate module contracts on the host before flashing hardware:
` + "```bash\npio test -e native\n```" + `
`
	writeTestFile(t, repoRoot, "docs/developer/creating-a-module.md", creatingModuleDoc)

	reader := ops.NewReader(opsDir)
	writer := ops.NewWriter(opsDir)
	opts := DocSeedOptions{
		RepoRoot: repoRoot,
		OpsDir:   opsDir,
		RepoCfg:  config.DefaultRepoConfig(),
	}

	res, err := SeedDocs(ctx, opts, &config.ScopeConfig{}, reader, writer)
	if err != nil {
		t.Fatalf("SeedDocs failed: %v", err)
	}
	if res.FilesSeeded != 2 {
		t.Fatalf("FilesSeeded = %d, want 2", res.FilesSeeded)
	}
	if res.EntriesAdded < 6 {
		t.Fatalf("expected at least 6 entries extracted from framework-expert + creating-a-module, got %d", res.EntriesAdded)
	}

	foundPioRecipe := false
	foundDataManagerScope := false
	foundU8G2Scope := false
	foundApplicationScope := false

	for _, op := range res.Ops {
		text, _ := op.Fields["text"].(string)
		entryType, _ := op.Fields["entry_type"].(string)
		if strings.Contains(text, "pio test -e native") && entryType == string(dossiers.EntryTypeRecipe) {
			foundPioRecipe = true
		}
		if strings.Contains(text, "weather station") {
			t.Errorf("Trigger Reference keyword catalog bullet should not be extracted: %q", text)
		}
		switch op.DossierScope {
		case "modules/DataManager":
			foundDataManagerScope = true
		case "modules/U8G2Display":
			foundU8G2Scope = true
		case "modules/Application":
			foundApplicationScope = true
		}
	}

	if !foundPioRecipe {
		t.Error("expected `pio test -e native` code fence to be extracted as a recipe entry")
	}
	if !foundDataManagerScope {
		t.Error("expected DataManager / IDataSource rule to resolve to modules/DataManager scope")
	}
	if !foundU8G2Scope {
		t.Error("expected IDisplayBoard / U8G2DisplayModule rule to resolve to modules/U8G2Display scope")
	}
	if !foundApplicationScope {
		t.Error("expected app.registerModule rule to resolve to modules/Application scope")
	}

	// Ensure RunOpsGC agrees with all scopes and anchors (0 repairs, 0 removed)
	gcStats, err := RunOpsGC(ctx, GCOptions{
		OpsDir:     opsDir,
		ConfigPath: filepath.Join(repoRoot, ".memory", "config.yaml"),
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("RunOpsGC failed: %v", err)
	}
	if gcStats.ScopesRepaired != 0 || gcStats.AnchorsRemoved != 0 {
		t.Errorf("expected RunOpsGC ScopesRepaired=0 and AnchorsRemoved=0, got %+v", gcStats)
	}
}
