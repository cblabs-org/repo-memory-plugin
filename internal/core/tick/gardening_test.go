package tick

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func TestRunGardening_BrokenAnchorsAndDuplicateMerge(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo"
	initTestGitRepo(t, repoRoot)

	// Create valid file anchors on disk.
	validFileA := filepath.Join(repoRoot, "internal", "auth", "token.go")
	validFileB := filepath.Join(repoRoot, "internal", "auth", "verify.go")
	if err := os.MkdirAll(filepath.Dir(validFileA), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(validFileA, []byte("package auth\nfunc IssueToken() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(validFileB, []byte("package auth\nfunc VerifyToken() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	seedOps := []ops.Op{
		// 1. Non-episodic entry with ALL missing anchors -> should be retired.
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-broken",
			DossierScope: "internal/auth",
			Timestamp:    t0,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeInvariant),
				"text":       "**Deleted file invariant**\nDo: Keep deleted_file.go intact.",
				"anchors":    []string{"internal/auth/deleted_file.go"},
				"evidence":   []string{"ep-1"},
			},
		},
		// 2. Episodic entry with missing anchor -> should NOT be retired by Step 1.
		{
			Type:         ops.OpAdd,
			EntryID:      "rul-episodic",
			DossierScope: "internal/auth",
			Timestamp:    t0,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeRuledOut),
				"text":       "**Ruled out legacy session cache**\nWhy: Removed legacy_cache.go.",
				"anchors":    []string{"internal/auth/legacy_cache.go"},
				"evidence":   []string{"ep-1"},
			},
		},
		// 3. Entry with partial missing anchors -> should be amended to prune missing anchor.
		{
			Type:         ops.OpAdd,
			EntryID:      "con-partial",
			DossierScope: "internal/auth",
			Timestamp:    t0,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeContract),
				"text":       "**Token validation contract**\nDo: Validate signature before expiry.",
				"anchors":    []string{"internal/auth/token.go", "internal/auth/gone.go"},
				"evidence":   []string{"ep-2"},
			},
		},
		// 4. Canonical survivor entry for near-duplicate merge.
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-dup-1",
			DossierScope: "internal/auth",
			Timestamp:    t0.Add(1 * time.Minute),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type":    string(dossiers.EntryTypeInvariant),
				"text":          "**Always sign tokens with Ed25519 keys**\nWhy: Prevents alg confusion.\nDo: Use IssueToken with Ed25519.",
				"anchors":       []string{"internal/auth/token.go"},
				"evidence":      []string{"ep-a"},
				"verified":      "2026-10-15 (test-pass)",
				"success_count": 2,
			},
		},
		// 5. Near-duplicate entry (same normalized title, overlapping/additional anchor, newer verified & evidence).
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-dup-2",
			DossierScope: "internal/auth",
			Timestamp:    t0.Add(2 * time.Minute),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type":    string(dossiers.EntryTypeInvariant),
				"text":          "**Always sign tokens with Ed25519 keys**\nWhy: Prevents algorithm confusion.\nDo: Enforce Ed25519 in IssueToken and VerifyToken.",
				"anchors":       []string{"internal/auth/token.go", "internal/auth/verify.go"},
				"evidence":      []string{"ep-b"},
				"verified":      "2026-10-16 (test-pass)",
				"success_count": 3,
			},
		},
	}

	if _, err := consolidate.RecordRepoTier(ctx, repoRoot, seedOps, nil, nil, t0); err != nil {
		t.Fatalf("seeding repo tier: %v", err)
	}

	exec := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, episode.NewStore(dataDir, repoID))
	exec.Now = func() time.Time { return t0.Add(10 * time.Minute) }

	// First test DryRun: true (no mutation on disk).
	dryRes, err := exec.RunGardening(ctx, GardeningOptions{DryRun: true})
	if err != nil {
		t.Fatalf("RunGardening (dry-run) failed: %v", err)
	}
	if len(dryRes.RetiredBrokenAnchors) != 1 || dryRes.RetiredBrokenAnchors[0] != "inv-broken" {
		t.Fatalf("expected RetiredBrokenAnchors=[inv-broken], got %v", dryRes.RetiredBrokenAnchors)
	}
	if len(dryRes.PrunedAnchorEntries) != 1 || dryRes.PrunedAnchorEntries[0] != "con-partial" {
		t.Fatalf("expected PrunedAnchorEntries=[con-partial], got %v", dryRes.PrunedAnchorEntries)
	}
	if len(dryRes.MergedDuplicates) != 1 || dryRes.MergedDuplicates[0] != "inv-dup-2" {
		t.Fatalf("expected MergedDuplicates=[inv-dup-2], got %v", dryRes.MergedDuplicates)
	}

	// Verify dry-run did not mutate .memory/ops/.
	beforeProj, err := dossiers.NewProjector(ops.NewReader(filepath.Join(repoRoot, ".memory", "ops"))).Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeProj["internal/auth"].Entries) != 5 {
		t.Fatalf("expected 5 entries untouched after dry-run, got %d", len(beforeProj["internal/auth"].Entries))
	}

	// Now run with DryRun: false.
	res, err := exec.RunGardening(ctx, GardeningOptions{DryRun: false})
	if err != nil {
		t.Fatalf("RunGardening (live) failed: %v", err)
	}
	if res.OpsEmitted != dryRes.OpsEmitted {
		t.Fatalf("expected %d ops emitted, got %d", dryRes.OpsEmitted, res.OpsEmitted)
	}

	afterProj, err := dossiers.NewProjector(ops.NewReader(filepath.Join(repoRoot, ".memory", "ops"))).Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	d := afterProj["internal/auth"]
	if d == nil {
		t.Fatalf("expected internal/auth dossier to exist")
	}

	activeByID := make(map[string]dossiers.Entry)
	for _, e := range d.Entries {
		activeByID[e.ID] = e
	}
	if _, exists := activeByID["inv-broken"]; exists {
		t.Fatalf("expected inv-broken to be retired")
	}
	if _, exists := activeByID["rul-episodic"]; !exists {
		t.Fatalf("expected episodic entry rul-episodic to remain active despite missing anchor")
	}
	if partial, exists := activeByID["con-partial"]; !exists || len(partial.Anchors) != 1 || partial.Anchors[0] != "internal/auth/token.go" {
		t.Fatalf("expected con-partial anchors pruned to [internal/auth/token.go], got %+v", partial.Anchors)
	}
	if _, exists := activeByID["inv-dup-2"]; exists {
		t.Fatalf("expected duplicate inv-dup-2 to be retired")
	}
	survivor, exists := activeByID["inv-dup-1"]
	if !exists {
		t.Fatalf("expected survivor inv-dup-1 to remain active")
	}
	if len(survivor.Anchors) != 2 || len(survivor.Evidence) != 2 || survivor.SuccessCount != 5 || survivor.Verified != "2026-10-16 (test-pass)" {
		t.Fatalf("expected survivor inv-dup-1 to have combined anchors/evidence/verified/success_count, got %+v", survivor)
	}
}

func TestRunGardening_DossierCapSplit(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo"
	initTestGitRepo(t, repoRoot)

	// Create anchor files in two sub-directories under internal/bigmod.
	subAFile := filepath.Join(repoRoot, "internal", "bigmod", "alpha", "a.go")
	subBFile := filepath.Join(repoRoot, "internal", "bigmod", "beta", "b.go")
	if err := os.MkdirAll(filepath.Dir(subAFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(subBFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subAFile, []byte("package alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subBFile, []byte("package beta\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 16, 16, 0, 0, 0, time.UTC)
	distinctEntries := []struct {
		id     string
		anchor string
		text   string
	}{
		{
			id:     "inv-alpha-0",
			anchor: "internal/bigmod/alpha/a.go",
			text:   "**Alpha parser must reject malformed UTF-8 sequences**\nWhy: Prevents downstream buffer corruption during lexical analysis.\nDo: Validate input bytes with utf8.Valid before scanning.",
		},
		{
			id:     "inv-alpha-1",
			anchor: "internal/bigmod/alpha/a.go",
			text:   "**Alpha AST nodes require immutable source span offsets**\nWhy: Incremental formatting depends on monotonic byte positions.\nDo: Compute start and end offsets during node construction.",
		},
		{
			id:     "inv-alpha-2",
			anchor: "internal/bigmod/alpha/a.go",
			text:   "**Alpha symbol table lookups must be deterministic across runs**\nWhy: Map iteration order causes non-reproducible compiler output.\nDo: Sort exported symbol keys lexicographically before emission.",
		},
		{
			id:     "inv-beta-3",
			anchor: "internal/bigmod/beta/b.go",
			text:   "**Beta scheduler enforces exponential backoff on worker retries**\nWhy: Avoids thundering herd spikes against upstream RPC endpoints.\nDo: Multiply base jitter interval by power of two per attempt.",
		},
		{
			id:     "inv-beta-4",
			anchor: "internal/bigmod/beta/b.go",
			text:   "**Beta connection pool closes idle sockets after five minutes**\nWhy: Prevents file descriptor exhaustion under bursty traffic.\nDo: Configure MaxIdleConns and IdleConnTimeout on transport.",
		},
		{
			id:     "inv-beta-5",
			anchor: "internal/bigmod/beta/b.go",
			text:   "**Beta circuit breaker trips after ten consecutive timeouts**\nWhy: Protects local queue depth when downstream storage degrades.\nDo: Return ErrCircuitOpen immediately while half-open probe runs.",
		},
	}

	var seedOps []ops.Op
	for i, item := range distinctEntries {
		seedOps = append(seedOps, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      item.id,
			DossierScope: "internal/bigmod",
			Timestamp:    now.Add(time.Duration(i) * time.Second),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeInvariant),
				"text":       item.text,
				"anchors":    []string{item.anchor},
				"evidence":   []string{fmt.Sprintf("ep-%d", i)},
			},
		})
	}

	if _, err := consolidate.RecordRepoTier(ctx, repoRoot, seedOps, nil, nil, now); err != nil {
		t.Fatalf("seeding oversized dossier: %v", err)
	}

	idxPath := index.IndexPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	exec := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, episode.NewStore(dataDir, repoID))
	exec.SetIndex(idx)
	exec.Now = func() time.Time { return now.Add(5 * time.Minute) }

	// Set MaxDossierTokens to 100 so our 6-entry dossier triggers Step 3 cap split.
	res, err := exec.RunGardening(ctx, GardeningOptions{
		DryRun:           false,
		MaxDossierTokens: 100,
	})
	if err != nil {
		t.Fatalf("RunGardening failed: %v", err)
	}

	if len(res.SplitDossiers) != 1 || res.SplitDossiers[0] != "internal/bigmod" {
		t.Fatalf("expected SplitDossiers=[internal/bigmod], got %v", res.SplitDossiers)
	}

	// Verify projected dossiers were split into internal/bigmod/alpha and internal/bigmod/beta.
	projected, err := dossiers.NewProjector(ops.NewReader(filepath.Join(repoRoot, ".memory", "ops"))).Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projected["internal/bigmod/alpha"].Entries) != 3 {
		t.Fatalf("expected 3 entries in internal/bigmod/alpha, got %d", len(projected["internal/bigmod/alpha"].Entries))
	}
	if len(projected["internal/bigmod/beta"].Entries) != 3 {
		t.Fatalf("expected 3 entries in internal/bigmod/beta, got %d", len(projected["internal/bigmod/beta"].Entries))
	}

	// Verify scope_aliases recorded in SQLite index.
	resolvedScope, err := idx.ResolveAlias(ctx, "internal/bigmod")
	if err != nil {
		t.Fatalf("ResolveAlias failed: %v", err)
	}
	if resolvedScope != "internal/bigmod/alpha" && resolvedScope != "internal/bigmod/beta" {
		t.Fatalf("expected alias for internal/bigmod to resolve to alpha or beta sub-scope, got %q", resolvedScope)
	}
}
