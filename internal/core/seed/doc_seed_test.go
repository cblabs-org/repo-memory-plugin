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
