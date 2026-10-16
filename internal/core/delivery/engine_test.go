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
