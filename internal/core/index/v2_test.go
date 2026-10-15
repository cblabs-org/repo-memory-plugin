package index_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func setupTestIndex(t *testing.T) (*index.Index, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	return idx, func() { idx.Close() }
}

func TestEntry_RoundTrip(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	entry := dossiers.Entry{
		ID:           "inv-001",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core/storage",
		Text:         "Storage must always use WAL mode",
		Anchors:      []string{"internal/core/storage/db.go"},
		Source:       ops.SourceHuman,
		Verified:     "verified",
		ClosesWhen:   "",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 1. UpsertEntry
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatalf("UpsertEntry: %v", err)
	}

	// 2. GetEntry
	got, err := idx.GetEntry(ctx, "inv-001")
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil entry")
	}
	if got.ID != entry.ID {
		t.Errorf("expected ID %s, got %s", entry.ID, got.ID)
	}
	if got.Type != entry.Type {
		t.Errorf("expected Type %s, got %s", entry.Type, got.Type)
	}
	if got.DossierScope != entry.DossierScope {
		t.Errorf("expected DossierScope %s, got %s", entry.DossierScope, got.DossierScope)
	}
	if got.Text != entry.Text {
		t.Errorf("expected Text %s, got %s", entry.Text, got.Text)
	}
	if got.Source != entry.Source {
		t.Errorf("expected Source %s, got %s", entry.Source, got.Source)
	}
	if got.Verified != entry.Verified {
		t.Errorf("expected Verified %s, got %s", entry.Verified, got.Verified)
	}
	if len(got.Anchors) != 1 || got.Anchors[0] != "internal/core/storage/db.go" {
		t.Errorf("expected anchors %v, got %v", entry.Anchors, got.Anchors)
	}

	// 3. Update entry
	entry.Text = "Storage must always use WAL mode and busy timeout"
	entry.UpdatedAt = now.Add(time.Minute)
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatalf("UpsertEntry update: %v", err)
	}

	gotUpdated, err := idx.GetEntry(ctx, "inv-001")
	if err != nil {
		t.Fatalf("GetEntry updated: %v", err)
	}
	if gotUpdated.Text != entry.Text {
		t.Errorf("expected updated text %q, got %q", entry.Text, gotUpdated.Text)
	}

	// 4. ListEntriesByScope
	byScope, err := idx.ListEntriesByScope(ctx, "core/storage")
	if err != nil {
		t.Fatalf("ListEntriesByScope: %v", err)
	}
	if len(byScope) != 1 || byScope[0].ID != "inv-001" {
		t.Fatalf("expected 1 entry in core/storage, got %v", byScope)
	}

	emptyScope, err := idx.ListEntriesByScope(ctx, "other/scope")
	if err != nil {
		t.Fatalf("ListEntriesByScope empty: %v", err)
	}
	if len(emptyScope) != 0 {
		t.Errorf("expected 0 entries, got %d", len(emptyScope))
	}

	// 5. DeleteEntry
	if err := idx.DeleteEntry(ctx, "inv-001"); err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}

	_, err = idx.GetEntry(ctx, "inv-001")
	if err == nil {
		t.Fatalf("expected error getting deleted entry, got nil")
	}
	if !errors.Is(err, index.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSearchEntries_FTS5(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()

	entries := []dossiers.Entry{
		{
			ID:           "e1",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "core/index",
			Text:         "SQLite full-text search indexing with FTS5 engine",
		},
		{
			ID:           "e2",
			Type:         dossiers.EntryTypeGotcha,
			DossierScope: "core/network",
			Text:         "Banana peel on the socket buffer allocation path",
		},
		{
			ID:           "e3",
			Type:         dossiers.EntryTypeDecision,
			DossierScope: "core/index",
			Text:         "Use modernc SQLite driver for pure Go portability",
		},
	}

	for _, e := range entries {
		if err := idx.UpsertEntry(ctx, e); err != nil {
			t.Fatalf("UpsertEntry %s: %v", e.ID, err)
		}
	}

	// Simple match
	res, err := idx.SearchEntries(ctx, "banana")
	if err != nil {
		t.Fatalf("SearchEntries 'banana': %v", err)
	}
	if len(res) != 1 || res[0].ID != "e2" {
		t.Fatalf("expected e2, got %v", res)
	}

	// FTS5 boolean match
	res, err = idx.SearchEntries(ctx, "SQLite AND FTS5")
	if err != nil {
		t.Fatalf("SearchEntries 'SQLite AND FTS5': %v", err)
	}
	if len(res) != 1 || res[0].ID != "e1" {
		t.Fatalf("expected e1, got %v", res)
	}

	// Empty query returns empty slice without error
	res, err = idx.SearchEntries(ctx, "")
	if err != nil {
		t.Fatalf("SearchEntries empty: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 results, got %d", len(res))
	}

	// Delete and verify FTS update
	if err := idx.DeleteEntry(ctx, "e2"); err != nil {
		t.Fatalf("DeleteEntry e2: %v", err)
	}
	res, err = idx.SearchEntries(ctx, "banana")
	if err != nil {
		t.Fatalf("SearchEntries after delete: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0 results for deleted entry, got %d", len(res))
	}
}

func TestEntryAnchors_Lookup(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()

	// Need entries first for foreign keys
	_ = idx.UpsertEntry(ctx, dossiers.Entry{ID: "ent-1", Type: dossiers.EntryTypeInvariant, DossierScope: "scope1", Text: "text1"})
	_ = idx.UpsertEntry(ctx, dossiers.Entry{ID: "ent-2", Type: dossiers.EntryTypeInvariant, DossierScope: "scope2", Text: "text2"})

	anchors1 := []string{"internal/core/index/index.go", "internal/core/index/rebuild.go"}
	if err := idx.UpsertEntryAnchors(ctx, "ent-1", anchors1); err != nil {
		t.Fatalf("UpsertEntryAnchors ent-1: %v", err)
	}

	anchors2 := []string{"internal/core/dossiers/entry.go"}
	if err := idx.UpsertEntryAnchors(ctx, "ent-2", anchors2); err != nil {
		t.Fatalf("UpsertEntryAnchors ent-2: %v", err)
	}

	// Prefix match with LIKE
	res, err := idx.LookupByAnchor(ctx, "internal/core/index")
	if err != nil {
		t.Fatalf("LookupByAnchor 'internal/core/index': %v", err)
	}
	if len(res) != 1 || res[0] != "ent-1" {
		t.Fatalf("expected ['ent-1'], got %v", res)
	}

	res, err = idx.LookupByAnchor(ctx, "internal/core")
	if err != nil {
		t.Fatalf("LookupByAnchor 'internal/core': %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(res), res)
	}

	res, err = idx.LookupByAnchor(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LookupByAnchor 'nonexistent': %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0 results, got %d", len(res))
	}

	// Replacement of anchors
	if err := idx.UpsertEntryAnchors(ctx, "ent-1", []string{"cmd/main.go"}); err != nil {
		t.Fatalf("UpsertEntryAnchors replacement: %v", err)
	}
	res, err = idx.LookupByAnchor(ctx, "internal/core/index")
	if err != nil {
		t.Fatalf("LookupByAnchor old anchor: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0 results after replacing anchors, got %v", res)
	}
	res, err = idx.LookupByAnchor(ctx, "cmd/main.go")
	if err != nil {
		t.Fatalf("LookupByAnchor new anchor: %v", err)
	}
	if len(res) != 1 || res[0] != "ent-1" {
		t.Fatalf("expected ['ent-1'], got %v", res)
	}
}

func TestLookupByAnchorExact(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()

	_ = idx.UpsertEntry(ctx, dossiers.Entry{ID: "ent-1", Type: dossiers.EntryTypeInvariant, DossierScope: "s", Text: "t"})
	if err := idx.UpsertEntryAnchors(ctx, "ent-1", []string{"internal/core/index/index.go"}); err != nil {
		t.Fatalf("UpsertEntryAnchors: %v", err)
	}

	// Exact match matches
	res, err := idx.LookupByAnchorExact(ctx, "internal/core/index/index.go")
	if err != nil {
		t.Fatalf("LookupByAnchorExact match: %v", err)
	}
	if len(res) != 1 || res[0] != "ent-1" {
		t.Fatalf("expected ['ent-1'], got %v", res)
	}

	// Prefix does not match exact
	res, err = idx.LookupByAnchorExact(ctx, "internal/core/index")
	if err != nil {
		t.Fatalf("LookupByAnchorExact prefix: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0 results for partial anchor in exact match, got %v", res)
	}
}

func TestErrorPatterns(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()

	_ = idx.UpsertEntry(ctx, dossiers.Entry{ID: "fail-1", Type: dossiers.EntryTypeFailureSignature, DossierScope: "test", Text: "t1"})
	_ = idx.UpsertEntry(ctx, dossiers.Entry{ID: "fail-2", Type: dossiers.EntryTypeFailureSignature, DossierScope: "gitops", Text: "t2"})

	if err := idx.InsertErrorPattern(ctx, "fail-1", "go test", "panic: runtime error", "test"); err != nil {
		t.Fatalf("InsertErrorPattern fail-1: %v", err)
	}
	if err := idx.InsertErrorPattern(ctx, "fail-2", "git push", "rejected.*non-fast-forward", "gitops"); err != nil {
		t.Fatalf("InsertErrorPattern fail-2: %v", err)
	}

	// Match command
	res, err := idx.LookupErrorPatterns(ctx, "go test ./...")
	if err != nil {
		t.Fatalf("LookupErrorPatterns 'go test ./...': %v", err)
	}
	if len(res) != 1 || res[0].EntryID != "fail-1" {
		t.Fatalf("expected fail-1, got %v", res)
	}
	if res[0].Pattern != "panic: runtime error" {
		t.Errorf("expected pattern 'panic: runtime error', got %s", res[0].Pattern)
	}

	// Match exact prefix
	res, err = idx.LookupErrorPatterns(ctx, "git push")
	if err != nil {
		t.Fatalf("LookupErrorPatterns 'git push': %v", err)
	}
	if len(res) != 1 || res[0].EntryID != "fail-2" {
		t.Fatalf("expected fail-2, got %v", res)
	}

	// Non-matching command
	res, err = idx.LookupErrorPatterns(ctx, "cargo build")
	if err != nil {
		t.Fatalf("LookupErrorPatterns 'cargo build': %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected 0 results, got %v", res)
	}
}

func TestAliases_ResolveChain(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()

	if err := idx.UpsertAlias(ctx, "scope-a", "scope-b"); err != nil {
		t.Fatalf("UpsertAlias a->b: %v", err)
	}
	if err := idx.UpsertAlias(ctx, "scope-b", "scope-c"); err != nil {
		t.Fatalf("UpsertAlias b->c: %v", err)
	}

	// Follow chain
	target, err := idx.ResolveAlias(ctx, "scope-a")
	if err != nil {
		t.Fatalf("ResolveAlias scope-a: %v", err)
	}
	if target != "scope-c" {
		t.Errorf("expected scope-c, got %s", target)
	}

	target, err = idx.ResolveAlias(ctx, "scope-b")
	if err != nil {
		t.Fatalf("ResolveAlias scope-b: %v", err)
	}
	if target != "scope-c" {
		t.Errorf("expected scope-c, got %s", target)
	}

	// Non-aliased scope returns itself
	target, err = idx.ResolveAlias(ctx, "scope-c")
	if err != nil {
		t.Fatalf("ResolveAlias scope-c: %v", err)
	}
	if target != "scope-c" {
		t.Errorf("expected scope-c, got %s", target)
	}

	target, err = idx.ResolveAlias(ctx, "other-scope")
	if err != nil {
		t.Fatalf("ResolveAlias other-scope: %v", err)
	}
	if target != "other-scope" {
		t.Errorf("expected other-scope, got %s", target)
	}
}

func TestEntryHistory(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	entry := dossiers.Entry{
		ID:            "inv-001",
		Type:          dossiers.EntryTypeInvariant,
		DossierScope:  "core/storage",
		Text:          "Old invariant text",
		UpdatedAt:     now,
		Retired:       true,
		RetiredReason: "superseded",
		SupersededBy:  "inv-002",
	}

	if err := idx.InsertHistory(ctx, entry, "superseded"); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	hist, err := idx.GetHistory(ctx, "inv-001")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(hist))
	}
	h := hist[0]
	if h.ID != "inv-001" {
		t.Errorf("expected ID inv-001, got %s", h.ID)
	}
	if h.RetiredReason != "superseded" {
		t.Errorf("expected reason superseded, got %s", h.RetiredReason)
	}
	if h.SupersededBy != "inv-002" {
		t.Errorf("expected superseded by inv-002, got %s", h.SupersededBy)
	}
	if !h.Retired {
		t.Errorf("expected retired true")
	}
}

func TestRebuildFromOpLog_EndToEnd(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()
	opsDir := t.TempDir()

	writer := ops.NewWriter(opsDir)
	now := time.Now().UTC()

	// Write ops to op log
	batch := ops.OpBatch{
		Version:   1,
		BatchID:   ops.NewBatchID(),
		CreatedAt: now,
		Ops: []ops.Op{
			{
				Type:         ops.OpAdd,
				EntryID:      "inv-010",
				DossierScope: "core/index",
				Timestamp:    now,
				Source:       ops.SourceHuman,
				Fields: map[string]any{
					"entry_type": "invariant",
					"text":       "SQLite index must always be rebuilt from op log",
					"anchors":    []any{"internal/core/index/rebuild.go"},
					"verified":   "verified",
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "fail-010",
				DossierScope: "core/build",
				Timestamp:    now,
				Source:       ops.SourceHuman,
				Fields: map[string]any{
					"entry_type": "failure-signature",
					"text":       "command: go test\npattern: panic: runtime error",
					"anchors":    []any{"internal/core/build/test.go"},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "dec-010",
				DossierScope: "core/old-scope",
				Timestamp:    now,
				Source:       ops.SourceHuman,
				Fields: map[string]any{
					"entry_type": "decision",
					"text":       "Original decision before split",
				},
			},
			{
				Type:         ops.OpRetire,
				EntryID:      "dec-010",
				DossierScope: "core/old-scope",
				Timestamp:    now.Add(time.Second),
				Source:       ops.SourceHuman,
				Fields: map[string]any{
					"reason": "obsolete after architecture update",
				},
			},
			{
				Type:         ops.OpSplit,
				DossierScope: "core/old-scope",
				Timestamp:    now.Add(2 * time.Second),
				Source:       ops.SourceHuman,
				Fields: map[string]any{
					"new_dossiers": []any{"core/new-scope-1", "core/new-scope-2"},
					"entry_moves":  map[string]any{"dec-010": "core/new-scope-1"},
				},
			},
		},
	}

	if err := writer.Write(batch); err != nil {
		t.Fatalf("Write: %v", err)
	}

	opsReader := ops.NewReader(opsDir)

	// Rebuild from op log
	if err := idx.RebuildFromOpLog(ctx, opsReader); err != nil {
		t.Fatalf("RebuildFromOpLog: %v", err)
	}

	// Verify entries and anchors populated
	invEntry, err := idx.GetEntry(ctx, "inv-010")
	if err != nil {
		t.Fatalf("GetEntry inv-010: %v", err)
	}
	if invEntry.Text != "SQLite index must always be rebuilt from op log" {
		t.Errorf("unexpected text: %s", invEntry.Text)
	}
	if len(invEntry.Anchors) != 1 || invEntry.Anchors[0] != "internal/core/index/rebuild.go" {
		t.Errorf("unexpected anchors: %v", invEntry.Anchors)
	}

	// Verify anchor lookup
	anchorMatches, err := idx.LookupByAnchor(ctx, "internal/core/index/rebuild.go")
	if err != nil {
		t.Fatalf("LookupByAnchor: %v", err)
	}
	if len(anchorMatches) != 1 || anchorMatches[0] != "inv-010" {
		t.Errorf("expected inv-010, got %v", anchorMatches)
	}

	// Verify error pattern populated
	patterns, err := idx.LookupErrorPatterns(ctx, "go test ./...")
	if err != nil {
		t.Fatalf("LookupErrorPatterns: %v", err)
	}
	if len(patterns) != 1 || patterns[0].EntryID != "fail-010" {
		t.Errorf("expected fail-010 error pattern, got %v", patterns)
	}

	// Verify history populated from retired entry
	hist, err := idx.GetHistory(ctx, "dec-010")
	if err != nil {
		t.Fatalf("GetHistory dec-010: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(hist))
	}
	if hist[0].RetiredReason != "obsolete after architecture update" {
		t.Errorf("expected obsolete reason, got %s", hist[0].RetiredReason)
	}

	// Verify alias populated from split op
	resolved, err := idx.ResolveAlias(ctx, "core/old-scope")
	if err != nil {
		t.Fatalf("ResolveAlias: %v", err)
	}
	if resolved != "core/new-scope-1" {
		t.Errorf("expected resolved alias core/new-scope-1, got %s", resolved)
	}

	// Test rebuild again to verify clearing and idempotency
	if err := idx.RebuildFromOpLog(ctx, opsReader); err != nil {
		t.Fatalf("RebuildFromOpLog second pass: %v", err)
	}
	invEntry2, err := idx.GetEntry(ctx, "inv-010")
	if err != nil {
		t.Fatalf("GetEntry after second rebuild: %v", err)
	}
	if invEntry2.ID != "inv-010" {
		t.Errorf("expected inv-010, got %s", invEntry2.ID)
	}
}

func TestBackwardCompatibility_Entities(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Entities Registry
	ent := entities.Entity{
		ID:   "pkg:foo",
		Kind: entities.KindFile,
		Name: "foo.go",
	}
	if err := idx.PutEntity(ctx, ent); err != nil {
		t.Fatalf("PutEntity: %v", err)
	}
	gotEnt, err := idx.GetEntity(ctx, "pkg:foo")
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if gotEnt.Name != "foo.go" {
		t.Errorf("expected foo.go, got %s", gotEnt.Name)
	}

	allEnts, err := idx.ListEntities(ctx, entities.KindFile)
	if err != nil {
		t.Fatalf("ListEntities: %v", err)
	}
	if len(allEnts) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(allEnts))
	}

	rel := entities.Relation{
		SourceID: "pkg:foo",
		TargetID: "pkg:bar",
		Kind:     entities.RelationContains,
	}
	if err := idx.PutRelation(ctx, rel); err != nil {
		t.Fatalf("PutRelation: %v", err)
	}
	rels, err := idx.GetRelations(ctx, "pkg:foo")
	if err != nil {
		t.Fatalf("GetRelations: %v", err)
	}
	if len(rels) != 1 || rels[0].TargetID != "pkg:bar" {
		t.Fatalf("expected relation to pkg:bar, got %v", rels)
	}

	invRels, err := idx.GetInverseRelations(ctx, "pkg:bar")
	if err != nil {
		t.Fatalf("GetInverseRelations: %v", err)
	}
	if len(invRels) != 1 || invRels[0].SourceID != "pkg:foo" {
		t.Fatalf("expected inverse relation from pkg:foo, got %v", invRels)
	}
}

func TestUpsertAndReplaceEntry_NoOrphanFTSOrDuplicateChildren(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Insert an entry with unique FTS token "orphancheckalpha", anchors, and error pattern,
	// then replace/update it with "orphancheckbeta".
	entry := dossiers.Entry{
		ID:           "fail-orphan-001",
		Type:         dossiers.EntryTypeFailureSignature,
		DossierScope: "core/storage",
		Text:         "command: go test\npattern: orphancheckalpha failure signature",
		Anchors:      []string{"internal/core/storage/alpha.go", "internal/core/storage/alpha.go"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatalf("UpsertEntry initial: %v", err)
	}
	if err := idx.InsertErrorPattern(ctx, entry.ID, "go test", "orphancheckalpha failure signature", entry.DossierScope); err != nil {
		t.Fatalf("InsertErrorPattern first: %v", err)
	}
	// Calling InsertErrorPattern again with identical values must be idempotent.
	if err := idx.InsertErrorPattern(ctx, entry.ID, "go test", "orphancheckalpha failure signature", entry.DossierScope); err != nil {
		t.Fatalf("InsertErrorPattern second: %v", err)
	}

	patterns, err := idx.LookupErrorPatterns(ctx, "go test ./...")
	if err != nil {
		t.Fatalf("LookupErrorPatterns initial: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected 1 error pattern after duplicate InsertErrorPattern calls, got %d: %v", len(patterns), patterns)
	}

	// Update via UpsertEntry with "orphancheckbeta" and new anchor
	entry.Text = "command: go test\npattern: orphancheckbeta replacement signature"
	entry.Anchors = []string{"internal/core/storage/beta.go"}
	entry.UpdatedAt = now.Add(time.Minute)
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatalf("UpsertEntry update: %v", err)
	}

	alphaMatches, err := idx.SearchEntries(ctx, "orphancheckalpha")
	if err != nil {
		t.Fatalf("SearchEntries orphancheckalpha: %v", err)
	}
	if len(alphaMatches) != 0 {
		t.Errorf("expected 0 FTS matches for orphancheckalpha, got %d: %v", len(alphaMatches), alphaMatches)
	}

	betaMatches, err := idx.SearchEntries(ctx, "orphancheckbeta")
	if err != nil {
		t.Fatalf("SearchEntries orphancheckbeta: %v", err)
	}
	if len(betaMatches) != 1 || betaMatches[0].ID != entry.ID {
		t.Errorf("expected 1 FTS match for orphancheckbeta, got %v", betaMatches)
	}

	gotEntry, err := idx.GetEntry(ctx, entry.ID)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if len(gotEntry.Anchors) != 1 || gotEntry.Anchors[0] != "internal/core/storage/beta.go" {
		t.Errorf("expected 1 replaced anchor, got %v", gotEntry.Anchors)
	}

	// 2. Call InsertEntry twice with updated text/anchors/error patterns and verify clean replacement.
	entryV1 := dossiers.Entry{
		ID:           "fail-orphan-002",
		Type:         dossiers.EntryTypeFailureSignature,
		DossierScope: "core/index",
		Text:         "command: go build\npattern: insertentrygamma error",
		Anchors:      []string{"internal/core/index/gamma1.go", "internal/core/index/gamma2.go"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := idx.InsertEntry(ctx, entryV1); err != nil {
		t.Fatalf("InsertEntry v1: %v", err)
	}

	entryV2 := dossiers.Entry{
		ID:           "fail-orphan-002",
		Type:         dossiers.EntryTypeFailureSignature,
		DossierScope: "core/index",
		Text:         "command: go vet\npattern: insertentrydelta warning",
		Anchors:      []string{"internal/core/index/delta.go"},
		CreatedAt:    now,
		UpdatedAt:    now.Add(2 * time.Minute),
	}
	if err := idx.InsertEntry(ctx, entryV2); err != nil {
		t.Fatalf("InsertEntry v2: %v", err)
	}

	gammaMatches, err := idx.SearchEntries(ctx, "insertentrygamma")
	if err != nil {
		t.Fatalf("SearchEntries insertentrygamma: %v", err)
	}
	if len(gammaMatches) != 0 {
		t.Errorf("expected 0 FTS matches for insertentrygamma after InsertEntry replacement, got %d", len(gammaMatches))
	}

	deltaMatches, err := idx.SearchEntries(ctx, "insertentrydelta")
	if err != nil {
		t.Fatalf("SearchEntries insertentrydelta: %v", err)
	}
	if len(deltaMatches) != 1 || deltaMatches[0].ID != "fail-orphan-002" {
		t.Errorf("expected 1 FTS match for insertentrydelta, got %v", deltaMatches)
	}

	gotV2, err := idx.GetEntry(ctx, "fail-orphan-002")
	if err != nil {
		t.Fatalf("GetEntry fail-orphan-002: %v", err)
	}
	if len(gotV2.Anchors) != 1 || gotV2.Anchors[0] != "internal/core/index/delta.go" {
		t.Errorf("expected [internal/core/index/delta.go], got %v", gotV2.Anchors)
	}

	oldAnchorLookup, err := idx.LookupByAnchor(ctx, "internal/core/index/gamma1.go")
	if err != nil {
		t.Fatalf("LookupByAnchor gamma1.go: %v", err)
	}
	if len(oldAnchorLookup) != 0 {
		t.Errorf("expected 0 matches for old anchor gamma1.go, got %v", oldAnchorLookup)
	}

	buildPatterns, err := idx.LookupErrorPatterns(ctx, "go build ./...")
	if err != nil {
		t.Fatalf("LookupErrorPatterns 'go build ./...': %v", err)
	}
	if len(buildPatterns) != 0 {
		t.Errorf("expected 0 error patterns for old command 'go build', got %v", buildPatterns)
	}

	vetPatterns, err := idx.LookupErrorPatterns(ctx, "go vet ./...")
	if err != nil {
		t.Fatalf("LookupErrorPatterns 'go vet ./...': %v", err)
	}
	if len(vetPatterns) != 1 || vetPatterns[0].EntryID != "fail-orphan-002" || vetPatterns[0].Pattern != "insertentrydelta warning" {
		t.Errorf("expected 1 error pattern for 'go vet', got %v", vetPatterns)
	}
}

func TestV2_2_CognitiveLifecyclesAndProvenance(t *testing.T) {
	idx, cleanup := setupTestIndex(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	recEntry := dossiers.Entry{
		ID:               "rec-v22-001",
		Type:             dossiers.EntryTypeRecipe,
		Kind:             dossiers.CognitiveKindProcedural,
		DossierScope:     "core/index",
		Text:             "Rebuild SQLite index safely with migration checks",
		Anchors:          []string{"internal/core/index/index.go"},
		Source:           ops.SourceDocSeed,
		SourceDocPath:    "docs/index-guide.md",
		SourceDocBlobSHA: "deadbeef12345678",
		SuccessCount:     7,
		FailureCount:     2,
		LastSucceededAt:  "2026-10-15T15:00:00Z",
		StaleSuspect:     true,
		RenderAsSkill:    "rm-rebuild-index",
		Verified:         "2026-10-15",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// 1. UpsertEntry & GetEntry
	if err := idx.UpsertEntry(ctx, recEntry); err != nil {
		t.Fatalf("UpsertEntry: %v", err)
	}
	got, err := idx.GetEntry(ctx, recEntry.ID)
	if err != nil {
		t.Fatalf("GetEntry: %v", err)
	}
	if got.Kind != dossiers.CognitiveKindProcedural ||
		got.SourceDocPath != "docs/index-guide.md" ||
		got.SourceDocBlobSHA != "deadbeef12345678" ||
		got.SuccessCount != 7 ||
		got.FailureCount != 2 ||
		got.LastSucceededAt != "2026-10-15T15:00:00Z" ||
		!got.StaleSuspect ||
		got.RenderAsSkill != "rm-rebuild-index" {
		t.Errorf("GetEntry mismatch on v2.2 fields: %+v", got)
	}

	// 2. InsertEntry & ListEntriesByScope & ListEntriesBySource
	conEntry := dossiers.Entry{
		ID:               "con-v22-002",
		Type:             dossiers.EntryTypeContract,
		DossierScope:     "core/index",
		Text:             "All index migrations must be idempotent",
		Anchors:          []string{"internal/core/index/index.go"},
		Source:           ops.SourceDocSeed,
		SourceDocPath:    "docs/contracts.md",
		SourceDocBlobSHA: "cafebabe87654321",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := idx.InsertEntry(ctx, conEntry); err != nil {
		t.Fatalf("InsertEntry: %v", err)
	}

	byScope, err := idx.ListEntriesByScope(ctx, "core/index")
	if err != nil {
		t.Fatalf("ListEntriesByScope: %v", err)
	}
	if len(byScope) != 2 {
		t.Fatalf("expected 2 entries by scope, got %d", len(byScope))
	}
	if byScope[0].ID != "con-v22-002" || byScope[0].Kind != dossiers.CognitiveKindSemantic || byScope[0].SourceDocBlobSHA != "cafebabe87654321" {
		t.Errorf("unexpected conEntry in ListEntriesByScope: %+v", byScope[0])
	}

	bySource, err := idx.ListEntriesBySource(ctx, ops.SourceDocSeed)
	if err != nil {
		t.Fatalf("ListEntriesBySource: %v", err)
	}
	if len(bySource) != 2 {
		t.Fatalf("expected 2 entries by source doc-seed, got %d", len(bySource))
	}
	if len(bySource[1].Anchors) != 1 || bySource[1].Anchors[0] != "internal/core/index/index.go" || !bySource[1].StaleSuspect {
		t.Errorf("unexpected recEntry in ListEntriesBySource: %+v", bySource[1])
	}

	emptyBySource, err := idx.ListEntriesBySource(ctx, "nonexistent-source")
	if err != nil {
		t.Fatalf("ListEntriesBySource empty: %v", err)
	}
	if emptyBySource == nil || len(emptyBySource) != 0 {
		t.Errorf("expected non-nil empty slice, got %v", emptyBySource)
	}

	// 3. SearchEntriesWithScore preserves v2.2 fields
	scored, err := idx.SearchEntriesWithScore(ctx, "migration")
	if err != nil {
		t.Fatalf("SearchEntriesWithScore: %v", err)
	}
	if len(scored) != 1 || scored[0].Entry.ID != "rec-v22-001" || scored[0].Entry.RenderAsSkill != "rm-rebuild-index" || scored[0].Entry.SuccessCount != 7 {
		t.Errorf("unexpected SearchEntriesWithScore result: %+v", scored)
	}

	// 4. RebuildFromOpLog preserves v2.2 fields
	opsDir := t.TempDir()
	writer := ops.NewWriter(opsDir)
	batch := ops.OpBatch{
		Version:   1,
		BatchID:   ops.NewBatchID(),
		CreatedAt: now,
		Ops: []ops.Op{
			{
				Type:         ops.OpAdd,
				EntryID:      "rec-rebuild-001",
				DossierScope: "core/index",
				Timestamp:    now,
				Source:       ops.SourceDocSeed,
				Fields: map[string]any{
					"entry_type":          "recipe",
					"text":                "Run go test -race before committing",
					"anchors":             []any{"internal/core/index/entries.go"},
					"source_doc_path":     "docs/testing.md",
					"source_doc_blob_sha": "feedface9999",
					"success_count":       4,
					"failure_count":       1,
					"last_succeeded_at":   "2026-10-15T16:00:00Z",
					"stale_suspect":       true,
					"render_as_skill":     "rm-test-race",
				},
			},
		},
	}
	if err := writer.Write(batch); err != nil {
		t.Fatalf("Write batch: %v", err)
	}
	if err := idx.RebuildFromOpLog(ctx, ops.NewReader(opsDir)); err != nil {
		t.Fatalf("RebuildFromOpLog: %v", err)
	}
	rebuilt, err := idx.GetEntry(ctx, "rec-rebuild-001")
	if err != nil {
		t.Fatalf("GetEntry after rebuild: %v", err)
	}
	if rebuilt.Kind != dossiers.CognitiveKindProcedural ||
		rebuilt.SourceDocPath != "docs/testing.md" ||
		rebuilt.SourceDocBlobSHA != "feedface9999" ||
		rebuilt.SuccessCount != 4 ||
		rebuilt.FailureCount != 1 ||
		rebuilt.LastSucceededAt != "2026-10-15T16:00:00Z" ||
		!rebuilt.StaleSuspect ||
		rebuilt.RenderAsSkill != "rm-test-race" {
		t.Errorf("RebuildFromOpLog did not preserve v2.2 fields: %+v", rebuilt)
	}
}

func TestNewIndex_AutoMigratesLegacyEntriesTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	// Create a pre-v2.2 entries table without the 8 new columns
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open legacy: %v", err)
	}
	_, err = rawDB.Exec(`
		CREATE TABLE entries (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			dossier_scope TEXT NOT NULL,
			text TEXT NOT NULL,
			source TEXT,
			verified TEXT,
			closes_when TEXT,
			created_at TEXT,
			updated_at TEXT
		);
		INSERT INTO entries (id, type, dossier_scope, text, source, verified, closes_when, created_at, updated_at)
		VALUES ('inv-legacy-1', 'invariant', 'core/legacy', 'Legacy invariant entry', 'seed', '', '', '2026-10-01T00:00:00Z', '2026-10-01T00:00:00Z');
	`)
	if err != nil {
		rawDB.Close()
		t.Fatalf("creating legacy schema: %v", err)
	}
	rawDB.Close()

	// Open via NewIndex — must auto-migrate via ALTER TABLE without error
	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex on legacy db failed: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	got, err := idx.GetEntry(ctx, "inv-legacy-1")
	if err != nil {
		t.Fatalf("GetEntry on migrated legacy row: %v", err)
	}
	if got.Kind != dossiers.CognitiveKindSemantic {
		t.Errorf("expected derived Kind %q for legacy row with empty kind column, got %q", dossiers.CognitiveKindSemantic, got.Kind)
	}

	// Verify writing a new v2.2 entry succeeds on the migrated table
	if err := idx.UpsertEntry(ctx, dossiers.Entry{
		ID:               "con-migrated-2",
		Type:             dossiers.EntryTypeContract,
		DossierScope:     "core/legacy",
		Text:             "New contract after migration",
		Source:           ops.SourceDocSeed,
		SourceDocPath:    "docs/README.md",
		SourceDocBlobSHA: "abcdef012345",
	}); err != nil {
		t.Fatalf("UpsertEntry after migration failed: %v", err)
	}
}

