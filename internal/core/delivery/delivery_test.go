package delivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestLedger(t *testing.T) {
	l := NewLedger("conv-123")
	
	if l.ConversationID != "conv-123" {
		t.Errorf("expected conv-123, got %s", l.ConversationID)
	}
	
	now := time.Now()
	rec := InjectionRecord{
		EntryID:     "inv-01",
		Channel:     ChannelSystem,
		Moment:      MomentTaskStart,
		StepIndex:   10,
		ContentHash: "hash1",
		Tag:         "[repo-memory inv-01]",
		Timestamp:   now,
	}
	l.RecordInjection(rec)
	l.RecordPull("got-02")
	
	exposures := l.GetExposureList()
	if len(exposures) != 2 {
		t.Fatalf("expected 2 exposures, got %d", len(exposures))
	}
	if exposures[0].EntryID != "inv-01" {
		t.Errorf("expected inv-01, got %s", exposures[0].EntryID)
	}
	if exposures[1].EntryID != "got-02" || exposures[1].Channel != "pull" {
		t.Errorf("expected got-02 pull, got %v", exposures[1])
	}
	
	l.MarkDossierSeen("scope-A")
	if !l.HasSeenDossier("scope-A") {
		t.Errorf("expected scope-A to be seen")
	}
	
	l.MarkAnchorSeen("anchor-B")
	if !l.HasSeenAnchor("anchor-B") {
		t.Errorf("expected anchor-B to be seen")
	}

	if !l.HasInjected("inv-01") {
		t.Errorf("expected inv-01 to be marked as injected")
	}
	
	l.RearmDedup()
	if l.HasSeenDossier("scope-A") || l.HasSeenAnchor("anchor-B") || l.HasInjected("inv-01") {
		t.Errorf("expected seen lists and injected set to be cleared")
	}
	
	after := l.FindInjectionAfterStep(5)
	if len(after) != 1 {
		t.Errorf("expected 1 injection after step 5, got %d", len(after))
	}
	
	l.AcknowledgeGate("gate-03")
	if !l.IsGateAcknowledged("gate-03") {
		t.Errorf("expected gate-03 to be acknowledged")
	}
}

func TestAnchorVerifier(t *testing.T) {
	tmpDir := t.TempDir()
	
	file1 := filepath.Join(tmpDir, "file1.txt")
	os.WriteFile(file1, []byte("test"), 0644)
	
	dir1 := filepath.Join(tmpDir, "dir1")
	os.Mkdir(dir1, 0755)
	
	if !VerifyAnchor("file1.txt", tmpDir) {
		t.Errorf("expected file1.txt to be verified")
	}
	if !VerifyAnchor("file1.txt:SomeFunc", tmpDir) {
		t.Errorf("expected file1.txt:SomeFunc to be verified")
	}
	if !VerifyAnchor("dir1", tmpDir) {
		t.Errorf("expected dir1 to be verified")
	}
	if VerifyAnchor("nonexistent.txt", tmpDir) {
		t.Errorf("expected nonexistent.txt to fail verification")
	}
	
	valid, suspect := VerifyAnchors([]string{"file1.txt", "nonexistent.txt"}, tmpDir)
	if len(valid) != 1 || valid[0] != "file1.txt" {
		t.Errorf("expected valid to contain file1.txt")
	}
	if len(suspect) != 1 || suspect[0] != "nonexistent.txt" {
		t.Errorf("expected suspect to contain nonexistent.txt")
	}
}

func TestRanker(t *testing.T) {
	entries := []dossiers.Entry{
		{
			ID: "inv-01",
			Type: dossiers.EntryTypeInvariant,
			Anchors: []string{"file1.txt"},
			Evidence: []string{"ev1"},
		},
		{
			ID: "dec-02",
			Type: dossiers.EntryTypeDecision,
			Anchors: []string{"dir1/"},
		},
		{
			ID: "proc-03",
			Type: dossiers.EntryTypeProcedure,
			Anchors: []string{},
		},
	}
	
	ranked := RankEntries(entries, []string{"file1.txt", "dir1/subdir/"})
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked entries, got %d", len(ranked))
	}
	
	if ranked[0].Entry.ID != "inv-01" {
		t.Errorf("expected inv-01 first, got %s", ranked[0].Entry.ID)
	}
	if ranked[1].Entry.ID != "dec-02" {
		t.Errorf("expected dec-02 second, got %s", ranked[1].Entry.ID)
	}
	
	// Test select within budget
	// Let's modify texts to have some length
	for i := range ranked {
		ranked[i].Entry.Text = "fourwords" // ~1 token based on len / 4 = 2 tokens? len is 9, 9/4 = 2.
	}
	ranked[0].Entry.Text = "a b c d e f g h i j k l" // len 23 -> 5 tokens
	ranked[1].Entry.Text = "a b c d e f g h i j k l m n o p" // len 31 -> 7 tokens
	ranked[2].Entry.Text = "a" // len 1 -> 1 token
	
	// budget 6 should select only the first
	selected := SelectWithinBudget(ranked, 6)
	if len(selected) != 2 {
		t.Errorf("expected 2 selected entries, got %d", len(selected))
	}
}

func TestFormatter(t *testing.T) {
	e := dossiers.Entry{
		ID: "inv-123",
		Type: dossiers.EntryTypeInvariant,
		Text: "Hello world",
	}
	
	f1 := FormatEntry(e)
	expected1 := "[repo-memory inv-123] invariant: Hello world"
	if f1 != expected1 {
		t.Errorf("expected %q, got %q", expected1, f1)
	}
	
	ranked := []RankedEntry{{Entry: e}}
	f2 := FormatEntries(ranked)
	if f2 == "" {
		t.Errorf("FormatEntries empty")
	}
	
	f3 := FormatGateDeny(e)
	expected3 := "[REPO-MEMORY inv-123] Hello world. Re-issue the edit after checking this."
	if f3 != expected3 {
		t.Errorf("expected %q, got %q", expected3, f3)
	}
	
	h := ContentHash("test")
	if len(h) != 64 {
		t.Errorf("expected hash length 64, got %d", len(h))
	}
}

func TestEngineIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	
	idx, err := index.NewIndex(filepath.Join(tmpDir, "index.db"))
	if err != nil {
		t.Fatalf("failed to open index: %v", err)
	}
	defer idx.Close()
	
	// Prepare test data
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("data"), 0644)
	
	e1 := dossiers.Entry{
		ID:           "inv-01",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "global",
		Text:         "test data here gate: edit",
		Anchors:      []string{"test.txt"},
	}
	e2 := dossiers.Entry{
		ID:           "dec-02",
		Type:         dossiers.EntryTypeDecision,
		DossierScope: "global",
		Text:         "something else",
	}
	e3 := dossiers.Entry{
		ID:           "inv-03",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "global",
		Text:         "dossier invariant entry",
	}
	e4 := dossiers.Entry{
		ID:           "inv-04",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "other",
		Text:         "anchor invariant entry",
		Anchors:      []string{"test.txt"},
	}
	idx.UpsertEntry(context.Background(), e1)
	idx.UpsertEntry(context.Background(), e2)
	idx.UpsertEntry(context.Background(), e3)
	idx.UpsertEntry(context.Background(), e4)
	idx.UpsertEntryAnchors(context.Background(), e1.ID, e1.Anchors)
	idx.UpsertEntryAnchors(context.Background(), e4.ID, e4.Anchors)
	
	eng := NewEngine(idx, tmpDir)
	
	// TaskStartEntries
	res, err := eng.TaskStartEntries(context.Background(), []string{"test"}, nil)
	if err != nil {
		t.Errorf("TaskStartEntries failed: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("TaskStartEntries expected results")
	}
	
	// DossierEntries (e1 is deduplicated; e3 is returned)
	res, err = eng.DossierEntries(context.Background(), "global")
	if err != nil {
		t.Errorf("DossierEntries failed: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("DossierEntries expected results")
	}
	
	// AnchorEntries (e1 is deduplicated; e4 is returned)
	res, err = eng.AnchorEntries(context.Background(), "test.txt")
	if err != nil {
		t.Errorf("AnchorEntries failed: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("AnchorEntries expected results")
	}
	
	// GateEntries
	l := NewLedger("conv-1")
	res, err = eng.GateEntries(context.Background(), "test.txt", l)
	if err != nil {
		t.Errorf("GateEntries failed: %v", err)
	}
	if len(res) == 0 {
		t.Errorf("GateEntries expected results")
	}
	
	l.AcknowledgeGate("inv-01")
	res, err = eng.GateEntries(context.Background(), "test.txt", l)
	if err != nil {
		t.Errorf("GateEntries failed: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("GateEntries expected 0 results after ack, got %d", len(res))
	}
}

func TestRanker_BM25Relevance(t *testing.T) {
	// Procedure has low type score (20), but high FTS bm25 relevance (2.5 -> +250 = 270)
	highBM25 := dossiers.Entry{
		ID:   "proc-01",
		Type: dossiers.EntryTypeProcedure,
		Text: "Specific procedure for banana buffer",
	}
	// Invariant has high type score (80), but low FTS bm25 relevance (0.1 -> +10 = 90)
	highType := dossiers.Entry{
		ID:   "inv-02",
		Type: dossiers.EntryTypeInvariant,
		Text: "Global system invariant",
	}

	scores := map[string]float64{
		"proc-01": 2.5,
		"inv-02":  0.1,
	}

	ranked := RankEntries([]dossiers.Entry{highType, highBM25}, nil, scores)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked entries, got %d", len(ranked))
	}

	if ranked[0].Entry.ID != "proc-01" {
		t.Errorf("expected high BM25 entry proc-01 to outrank high-type entry, got %s first", ranked[0].Entry.ID)
	}
	if ranked[0].Score <= ranked[1].Score {
		t.Errorf("expected score %d > %d", ranked[0].Score, ranked[1].Score)
	}
	if ranked[0].FTSScore != 2.5 {
		t.Errorf("expected FTSScore 2.5, got %f", ranked[0].FTSScore)
	}
}

func TestEngine_PerSessionDeduplication(t *testing.T) {
	tmpDir := t.TempDir()
	idx, err := index.NewIndex(filepath.Join(tmpDir, "index.db"))
	if err != nil {
		t.Fatalf("failed to open index: %v", err)
	}
	defer idx.Close()

	os.WriteFile(filepath.Join(tmpDir, "file.go"), []byte("package main"), 0644)

	e1 := dossiers.Entry{
		ID:           "inv-dup",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core",
		Text:         "important invariant about file.go",
		Anchors:      []string{"file.go"},
	}
	ctx := context.Background()
	if err := idx.UpsertEntry(ctx, e1); err != nil {
		t.Fatalf("UpsertEntry: %v", err)
	}
	if err := idx.UpsertEntryAnchors(ctx, e1.ID, e1.Anchors); err != nil {
		t.Fatalf("UpsertEntryAnchors: %v", err)
	}

	eng := NewEngine(idx, tmpDir)

	// Moment 1: TaskStartEntries injects e1
	res1, err := eng.TaskStartEntries(ctx, []string{"important"}, nil)
	if err != nil {
		t.Fatalf("TaskStartEntries: %v", err)
	}
	if len(res1) != 1 || res1[0].Entry.ID != "inv-dup" {
		t.Fatalf("expected inv-dup injected at task start, got %v", res1)
	}

	// Moment 2: AnchorEntries touches file.go — e1 must be skipped as duplicate
	res2, err := eng.AnchorEntries(ctx, "file.go")
	if err != nil {
		t.Fatalf("AnchorEntries: %v", err)
	}
	if len(res2) != 0 {
		t.Errorf("expected 0 entries (deduplicated), got %d", len(res2))
	}

	// Moment 3: DossierEntries for scope core — e1 must be skipped as duplicate
	res3, err := eng.DossierEntries(ctx, "core")
	if err != nil {
		t.Fatalf("DossierEntries: %v", err)
	}
	if len(res3) != 0 {
		t.Errorf("expected 0 entries (deduplicated), got %d", len(res3))
	}

	// Reset per-session state — e1 should now be eligible again
	eng.ResetInjected()
	res4, err := eng.AnchorEntries(ctx, "file.go")
	if err != nil {
		t.Fatalf("AnchorEntries after reset: %v", err)
	}
	if len(res4) != 1 || res4[0].Entry.ID != "inv-dup" {
		t.Errorf("expected inv-dup after reset, got %v", res4)
	}
}

func TestEngine_SurfaceFTSErrors(t *testing.T) {
	tmpDir := t.TempDir()
	idx, err := index.NewIndex(filepath.Join(tmpDir, "index.db"))
	if err != nil {
		t.Fatalf("failed to open index: %v", err)
	}
	eng := NewEngine(idx, tmpDir)

	// Closing the underlying index forces an error on query
	idx.Close()

	ctx := context.Background()
	_, err = eng.TaskStartEntries(ctx, []string{"query"}, nil)
	if err == nil {
		t.Fatalf("expected FTS search error to be surfaced, got nil")
	}
	if !strings.Contains(err.Error(), "FTS search") {
		t.Errorf("expected error message to mention 'FTS search', got %v", err)
	}
}

func TestSelectWithinBudget_RelevanceThreshold(t *testing.T) {
	entries := []dossiers.Entry{
		{
			ID:      "inv-01",
			Type:    dossiers.EntryTypeInvariant, // type score 80 (>= 50)
			Text:    "important rule",
			Anchors: []string{"other.txt"},
		},
		{
			ID:      "proc-02",
			Type:    dossiers.EntryTypeProcedure, // type score 20 (< 50)
			Text:    "minor procedure",
			Anchors: []string{"other.txt"},
		},
		{
			ID:      "issue-03",
			Type:    dossiers.EntryTypeOpenIssue, // type score 5 (< 50)
			Text:    "open question",
			Anchors: []string{"other.txt"},
		},
	}

	// Target anchors don't match other.txt, so anchor match is "" (0 points)
	ranked := RankEntries(entries, []string{"target.txt"})
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked entries, got %d", len(ranked))
	}

	// Default threshold is 50. Only inv-01 (score 80) should pass.
	selectedDefault := SelectWithinBudget(ranked, 1000)
	if len(selectedDefault) != 1 {
		t.Fatalf("expected 1 entry with default threshold 50, got %d", len(selectedDefault))
	}
	if selectedDefault[0].Entry.ID != "inv-01" {
		t.Errorf("expected inv-01, got %s", selectedDefault[0].Entry.ID)
	}

	// Custom threshold 10: inv-01 (80) and proc-02 (20) should pass, issue-03 (5) filtered
	selectedCustom := SelectWithinBudget(ranked, 1000, 10)
	if len(selectedCustom) != 2 {
		t.Fatalf("expected 2 entries with threshold 10, got %d", len(selectedCustom))
	}

	// Custom threshold 0: all pass
	selectedZero := SelectWithinBudget(ranked, 1000, 0)
	if len(selectedZero) != 3 {
		t.Fatalf("expected 3 entries with threshold 0, got %d", len(selectedZero))
	}
}
