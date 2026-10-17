package delivery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestEngine_ExcludesOpenIssueWithoutClosesWhenFromPushDelivery(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "index.db")

	anchorFile := filepath.Join(workspaceRoot, "internal", "service.go")
	if err := os.MkdirAll(filepath.Dir(anchorFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(anchorFile, []byte("package internal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	now := time.Now().UTC()
	issuePullOnly := dossiers.Entry{
		ID:           "iss-pull-only",
		DossierScope: "internal",
		Type:         dossiers.EntryTypeOpenIssue,
		Kind:         dossiers.CognitiveKindEpisodic,
		Text:         "**Unverified service timeout issue**\nWhy: Observed once during manual run (`gate: edit`).\nDo: Check timeout settings.",
		Anchors:      []string{"internal/service.go"},
		ClosesWhen:   "",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	issueWithClosesWhen := dossiers.Entry{
		ID:           "iss-push-eligible",
		DossierScope: "internal",
		Type:         dossiers.EntryTypeOpenIssue,
		Kind:         dossiers.CognitiveKindEpisodic,
		Text:         "**Service migration open issue**\nWhy: Waiting on v2 endpoint (`gate: edit`).\nDo: Do not remove v1 handler yet.",
		Anchors:      []string{"internal/service.go"},
		ClosesWhen:   "path_exists:internal/service_v2.go",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := idx.UpsertEntry(ctx, issuePullOnly); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpsertEntry(ctx, issueWithClosesWhen); err != nil {
		t.Fatal(err)
	}

	// 1. Verify pull-only open-issue IS still searchable via MCP/CLI index search.
	searchHits, err := idx.SearchEntries(ctx, "timeout")
	if err != nil {
		t.Fatal(err)
	}
	foundInPull := false
	for _, h := range searchHits {
		if h.ID == "iss-pull-only" {
			foundInPull = true
			break
		}
	}
	if !foundInPull {
		t.Fatalf("expected iss-pull-only to remain searchable via pull index.SearchEntries")
	}

	// 2. Verify TaskStartEntries excludes iss-pull-only and includes iss-push-eligible.
	eng := NewEngine(idx, workspaceRoot, WithMinScore(0))
	taskStart, err := eng.TaskStartEntries(ctx, []string{"service", "timeout", "migration"}, []string{"internal/service.go"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range taskStart {
		if r.Entry.ID == "iss-pull-only" {
			t.Fatalf("expected iss-pull-only (empty closes_when) to be excluded from TaskStartEntries push delivery")
		}
	}
	foundPushEligible := false
	for _, r := range taskStart {
		if r.Entry.ID == "iss-push-eligible" {
			foundPushEligible = true
			break
		}
	}
	if !foundPushEligible {
		t.Fatalf("expected iss-push-eligible (non-empty closes_when) to be included in TaskStartEntries")
	}

	// 3. Verify AnchorEntries excludes iss-pull-only and includes iss-push-eligible.
	eng.ResetInjected()
	anchorHits, err := eng.AnchorEntries(ctx, "internal/service.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range anchorHits {
		if r.Entry.ID == "iss-pull-only" {
			t.Fatalf("expected iss-pull-only to be excluded from AnchorEntries push delivery")
		}
	}

	// 4. Verify GateEntries excludes iss-pull-only and includes iss-push-eligible.
	gateHits, err := eng.GateEntries(ctx, "internal/service.go", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range gateHits {
		if r.Entry.ID == "iss-pull-only" {
			t.Fatalf("expected iss-pull-only to be excluded from GateEntries push delivery")
		}
	}
}

func TestEngine_MidTurnPrioritizesSemanticOverDecisionDiary(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "index.db")

	anchorFile := filepath.Join(workspaceRoot, "internal", "service.go")
	if err := os.MkdirAll(filepath.Dir(anchorFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(anchorFile, []byte("package internal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	now := time.Now().UTC()
	entries := []dossiers.Entry{
		{
			ID:           "dec-diary",
			DossierScope: "internal",
			Type:         dossiers.EntryTypeDecision,
			Kind:         dossiers.CognitiveKindSemantic,
			Text:         "Chose helper X during initial refactor.",
			Anchors:      []string{"internal/service.go"},
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "rul-diary",
			DossierScope: "internal",
			Type:         dossiers.EntryTypeRuledOut,
			Kind:         dossiers.CognitiveKindEpisodic,
			Text:         "Ruled out synchronous polling in service.go.",
			Anchors:      []string{"internal/service.go"},
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "got-actionable",
			DossierScope: "internal",
			Type:         dossiers.EntryTypeGotcha,
			Kind:         dossiers.CognitiveKindSemantic,
			Text:         "Always hold mu before mutating service state.",
			Anchors:      []string{"internal/service.go"},
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	for _, e := range entries {
		if err := idx.UpsertEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
		if len(e.Anchors) > 0 {
			if err := idx.UpsertEntryAnchors(ctx, e.ID, e.Anchors); err != nil {
				t.Fatal(err)
			}
		}
	}

	eng := NewEngine(idx, workspaceRoot)

	// 1. AnchorEntries (MomentFirstTouch) excludes decision diary entries and ranks gotcha above ruled-out.
	touchHits, err := eng.AnchorEntries(ctx, "internal/service.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(touchHits) != 2 {
		t.Fatalf("expected 2 AnchorEntries (gotcha + ruled-out, excluding decision), got %d: %+v", len(touchHits), touchHits)
	}
	if touchHits[0].Entry.ID != "got-actionable" {
		t.Errorf("expected got-actionable to rank first ahead of ruled-out, got %s", touchHits[0].Entry.ID)
	}
	for _, h := range touchHits {
		if h.Entry.ID == "dec-diary" {
			t.Errorf("expected decision entry dec-diary to be excluded from mid-turn AnchorEntries")
		}
	}

	// 2. DossierEntries (MomentFirstEntry) includes gotcha and ranks it above ruled-out.
	eng.ResetInjected()
	dossierHits, err := eng.DossierEntries(ctx, "internal")
	if err != nil {
		t.Fatal(err)
	}
	if len(dossierHits) != 2 {
		t.Fatalf("expected 2 DossierEntries (gotcha + ruled-out), got %d: %+v", len(dossierHits), dossierHits)
	}
	if dossierHits[0].Entry.ID != "got-actionable" {
		t.Errorf("expected got-actionable to rank first in DossierEntries, got %s", dossierHits[0].Entry.ID)
	}
}
