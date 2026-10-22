/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestExtractWorkorderPaths(t *testing.T) {
	md := `# WORKORDER: WO1 — PA Summaries and Workorder Briefs

## 2. Context
See ` + "`docs/design-v2-distillation-retrieval.md`" + ` and https://example.com/spec.
Run ` + "`go test -race ./...`" + ` with ` + "`--json`" + ` or ` + "`--max-tokens`" + `.

## 3. Strict Scope & File Boundaries

### Permitted Mutable Files
- ` + "`internal/core/dossiers/lint.go`" + ` *(export EstimateTokens)*
- ` + "`internal/core/episode/summary.go`" + ` *(NEW)*
- ` + "`internal/core/delivery/workorder_brief.go`" + ` *(NEW)*
- ` + "`cmd/repo-memory/cmd_brief.go`" + ` *(NEW)*

### Forbidden Files (Read-Only Context)
- [engine.go](file:///workspace/repo/internal/core/delivery/engine.go#L10-L20)
- ` + "`./internal/core/coverage/coverage.go`" + `
`

	got := ExtractWorkorderPaths(md)
	wantOrder := []string{
		"internal/core/dossiers/lint.go",
		"internal/core/episode/summary.go",
		"internal/core/delivery/workorder_brief.go",
		"cmd/repo-memory/cmd_brief.go",
		"internal/core/delivery/engine.go",
		"internal/core/coverage/coverage.go",
		"docs/design-v2-distillation-retrieval.md",
	}

	if len(got) != len(wantOrder) {
		t.Fatalf("ExtractWorkorderPaths len = %d (%v), want %d (%v)", len(got), got, len(wantOrder), wantOrder)
	}
	for i, w := range wantOrder {
		if got[i] != w {
			t.Errorf("ExtractWorkorderPaths[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestBuildWorkorderBrief_FullWorkflow(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo-brief"

	idxPath := filepath.Join(dataDir, repoID, "index.db")
	if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatalf("NewIndex failed: %v", err)
	}
	defer idx.Close()

	// Populate entries in SQLite index for internal/core/delivery, leaving internal/core/episode without a dossier
	entries := []dossiers.Entry{
		{
			ID:           "inv-deliv-001",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "internal/core/delivery",
			Text:         "Delivery package must never import internal/core/inbox, internal/core/coverage, or internal/core/tick.",
			Anchors:      []string{"internal/core/delivery/workorder_brief.go"},
			Evidence:     []string{"commit:abc1234"},
			Verified:     "2026-04-15",
			Source:       "consolidation",
			UpdatedAt:    time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:           "got-deliv-002",
			Type:         dossiers.EntryTypeGotcha,
			DossierScope: "internal/core/delivery",
			Text:         "Always bound workorder brief output to <= 600 tokens using dossiers.EstimateTokens.",
			Anchors:      []string{"internal/core/delivery/ranker.go"},
			Source:       "consolidation",
			StaleSuspect: true,
			UpdatedAt:    time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		},
	}
	for _, e := range entries {
		if err := idx.InsertEntry(ctx, e); err != nil {
			t.Fatalf("InsertEntry(%s) failed: %v", e.ID, err)
		}
	}

	// Write freshness state
	stateDir := filepath.Join(dataDir, repoID, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	freshnessJSON := `{"index_current_to_oplog":true,"index_head_sha":"deadbeef123456","unconsolidated_notes":2,"suspect_entries":1}`
	if err := os.WriteFile(filepath.Join(stateDir, "freshness.json"), []byte(freshnessJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write an unconsolidated note for internal/core/episode
	notesDir := filepath.Join(dataDir, repoID, "evidence", "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	notePayload := map[string]any{
		"id":         "note-ep-01",
		"kind":       "invariant",
		"scope":      "internal/core/episode",
		"statement":  "Episode summary parser must handle both Antigravity and Claude Code JSONL transcripts.",
		"created_at": "2026-04-16T10:00:00Z",
	}
	noteBytes, _ := json.MarshalIndent(notePayload, "", "  ")
	if err := os.WriteFile(filepath.Join(notesDir, "note-ep-01.json"), noteBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// Write WORKORDER.md
	woPath := filepath.Join(repoRoot, "WORKORDER.md")
	woContent := `# WORKORDER: Test Brief
### Permitted Mutable Files
- ` + "`internal/core/delivery/workorder_brief.go`" + `
- ` + "`internal/core/episode/summary.go`" + `
`
	if err := os.WriteFile(woPath, []byte(woContent), 0o644); err != nil {
		t.Fatal(err)
	}

	brief, err := BuildWorkorderBrief(ctx, idx, repoRoot, dataDir, repoID, woPath, "", 600)
	if err != nil {
		t.Fatalf("BuildWorkorderBrief failed: %v", err)
	}

	if !strings.HasPrefix(brief.StatusLine, "memory: index current (HEAD deadbee)") {
		t.Errorf("StatusLine = %q, want prefix 'memory: index current (HEAD deadbee)'", brief.StatusLine)
	}
	if len(brief.CoverageGaps) == 0 {
		t.Fatalf("expected CoverageGaps to report missing dossier for internal/core/episode and/or suspect entries")
	}
	foundEpGap := false
	for _, g := range brief.CoverageGaps {
		if strings.Contains(g, "coverage: internal/core/episode has no dossier yet (1 unconsolidated notes pending)") {
			foundEpGap = true
		}
	}
	if !foundEpGap {
		t.Errorf("CoverageGaps = %v, expected missing dossier gap for internal/core/episode", brief.CoverageGaps)
	}

	if len(brief.Entries) != 2 {
		t.Fatalf("Entries len = %d, want 2", len(brief.Entries))
	}
	if brief.Entries[0].Entry.ID != "inv-deliv-001" {
		t.Errorf("top ranked entry = %s, want inv-deliv-001", brief.Entries[0].Entry.ID)
	}
	if len(brief.UnconsolidatedNotes) != 1 || brief.UnconsolidatedNotes[0].ID != "note-ep-01" {
		t.Errorf("UnconsolidatedNotes = %v, want [note-ep-01]", brief.UnconsolidatedNotes)
	}
	if brief.TokenCount <= 0 || brief.TokenCount > 600 {
		t.Errorf("TokenCount = %d, want in (0, 600]", brief.TokenCount)
	}
	if dossiers.EstimateTokens(brief.BriefMarkdown) != brief.TokenCount {
		t.Errorf("EstimateTokens(BriefMarkdown) = %d != TokenCount %d", dossiers.EstimateTokens(brief.BriefMarkdown), brief.TokenCount)
	}
}

func TestBuildWorkorderBrief_ScopeOnlyAndBudgetEnforcement(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo-budget"

	// Populate .memory/dossiers/ markdown directly (testing index-less fallback)
	dossiersDir := filepath.Join(repoRoot, ".memory", "dossiers")
	if err := os.MkdirAll(dossiersDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var entries []dossiers.Entry
	for i := 0; i < 40; i++ {
		entries = append(entries, dossiers.Entry{
			ID:           fmt.Sprintf("inv-tick-%03d", i),
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "internal/core/tick",
			Text:         fmt.Sprintf("Invariant %03d: Consolidation pipeline step %d must preserve deterministic ordering across all multi-tier dossier projections.", i, i),
			Anchors:      []string{fmt.Sprintf("internal/core/tick/step_%03d.go", i)},
			Verified:     "2026-04-16",
		})
	}
	rendered := dossiers.RenderDossier(&dossiers.Dossier{
		Scope:   "internal/core/tick",
		Kind:    dossiers.DossierKindModule,
		Purpose: "Tick pipeline execution and warm consolidation.",
		Entries: entries,
	})
	if err := os.WriteFile(filepath.Join(dossiersDir, "internal__core__tick.md"), []byte(rendered), 0o644); err != nil {
		t.Fatal(err)
	}

	// Default 600 token budget in --scope only mode without an open index
	b600, err := BuildWorkorderBrief(ctx, nil, repoRoot, dataDir, repoID, "", "internal/core/tick", 600)
	if err != nil {
		t.Fatalf("BuildWorkorderBrief failed: %v", err)
	}
	if b600.TokenCount <= 0 || b600.TokenCount > 600 {
		t.Errorf("b600.TokenCount = %d, want in (0, 600]", b600.TokenCount)
	}
	if len(b600.ResolvedScopes) != 1 || b600.ResolvedScopes[0] != "internal/core/tick" {
		t.Errorf("ResolvedScopes = %v, want [internal/core/tick]", b600.ResolvedScopes)
	}

	// Tight 110 token budget
	b110, err := BuildWorkorderBrief(ctx, nil, repoRoot, dataDir, repoID, "", "internal/core/tick", 110)
	if err != nil {
		t.Fatalf("BuildWorkorderBrief(110) failed: %v", err)
	}
	if b110.TokenCount <= 0 || b110.TokenCount > 110 {
		t.Errorf("b110.TokenCount = %d, want in (0, 110]", b110.TokenCount)
	}
}

func TestExtractWorkorderPaths_RejectsDotDotTraversal(t *testing.T) {
	md := `# WORKORDER: Path Security Test
### Permitted Mutable Files
- ` + "`internal/core/delivery/workorder_brief.go`" + `
- ` + "`../secret.go`" + `
- ` + "`internal/../../etc/passwd.go`" + `
- ` + "`internal/core/../delivery/other.go`" + `
`
	got := ExtractWorkorderPaths(md)
	if len(got) != 1 || got[0] != "internal/core/delivery/workorder_brief.go" {
		t.Fatalf("ExtractWorkorderPaths = %v, want only [internal/core/delivery/workorder_brief.go]", got)
	}
}

func TestBuildWorkorderBrief_GraduatedSkillPointer(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo-skill-pointer"

	idxPath := filepath.Join(dataDir, repoID, "index.db")
	if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatalf("NewIndex failed: %v", err)
	}
	defer idx.Close()

	procEntry := dossiers.Entry{
		ID:           "rec-deliv-010",
		Type:         dossiers.EntryTypeRecipe,
		DossierScope: "internal/core/delivery",
		Text:         "Run delivery package verification\n1. go vet ./internal/core/delivery/...\n2. go test -race ./internal/core/delivery/...",
		Anchors:      []string{"internal/core/delivery/workorder_brief.go"},
		Verified:     "2026-04-16",
		SuccessCount: 3,
	}
	if err := idx.InsertEntry(ctx, procEntry); err != nil {
		t.Fatalf("InsertEntry failed: %v", err)
	}

	// Write a graduated SKILL.md under .agents/skills/rm-run-delivery-package-verification/SKILL.md
	skillDir := filepath.Join(repoRoot, ".agents", "skills", "rm-run-delivery-package-verification")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: rm-run-delivery-package-verification\ngenerated_by: repo-memory\nsource_entry_id: rec-deliv-010\n---\n# Run delivery package verification\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	brief, err := BuildWorkorderBrief(ctx, idx, repoRoot, dataDir, repoID, "", "internal/core/delivery", 600)
	if err != nil {
		t.Fatalf("BuildWorkorderBrief failed: %v", err)
	}

	wantPointer := "- [rec-deliv-010] (recipe, internal/core/delivery) skill rm-run-delivery-package-verification covers this procedure: Run delivery package verification"
	if !strings.Contains(brief.BriefMarkdown, wantPointer) {
		t.Errorf("BriefMarkdown missing graduated skill pointer %q;\ngot:\n%s", wantPointer, brief.BriefMarkdown)
	}
	if strings.Contains(brief.BriefMarkdown, "go test -race ./internal/core/delivery/...") {
		t.Errorf("BriefMarkdown should omit multi-step recipe body when graduated skill exists;\ngot:\n%s", brief.BriefMarkdown)
	}
}
