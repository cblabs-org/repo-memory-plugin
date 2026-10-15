package seed_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

func TestRunOpsGC_PrunesJunkStaleAndRepairsScopes(t *testing.T) {
	tmpDir := t.TempDir()
	opsDir := filepath.Join(tmpDir, "ops")
	monthDir := filepath.Join(opsDir, "2026-10")
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create stale polluted dossiers to verify RunOpsGC removes them before re-projecting.
	staleDossiersDir := filepath.Join(tmpDir, "dossiers")
	for _, rel := range []string{".agents.md", "modernc.org.md", "pending.md", filepath.Join("<data-dir>", "<repo-id>.md")} {
		p := filepath.Join(staleDossiersDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)

	// Batch 1: contains only junk/stale ops -> should be deleted entirely.
	emptyAfterGCPath := filepath.Join(monthDir, "01_junk_only.json")
	batch1 := ops.OpBatch{
		Version:   1,
		CreatedAt: now,
		BatchID:   "01JUNKONLY",
		Ops: []ops.Op{
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-b1617c95",
				DossierScope: "harness/antigravity",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "ruled-out",
					"text":       "Antigravity completely ignores systemMessage and userMessage",
					"anchors":    []string{"internal/harness/antigravity/preinvocation.go"},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-junk0001",
				DossierScope: "pending",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "decision",
					"text":       "None.",
					"anchors":    []string{"internal/core/ops/op.go"},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-junk0002",
				DossierScope: "pr-08-v2-delivery",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "open-issue",
					"text":       "(pending)",
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-junk0003",
				DossierScope: "core/ops",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "open-issue",
					"text":       "(none)",
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-junk0004",
				DossierScope: "core/ops",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "open-issue",
					"text":       "(none).",
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-junk0005",
				DossierScope: "core/ops",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "decision",
					"text":       "(n/a)",
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-junk0006",
				DossierScope: "core/ops",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "ruled-out",
					"text":       "-",
				},
			},
		},
	}

	// Batch 2: contains valid ops with polluted scopes and mixed valid/invalid anchors.
	mixedBatchPath := filepath.Join(monthDir, "02_mixed.json")
	batch2 := ops.OpBatch{
		Version:   1,
		CreatedAt: now,
		BatchID:   "02MIXED",
		Ops: []ops.Op{
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-valid001",
				DossierScope: "<data-dir>/<repo-id>",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "decision",
					"text":       "Use deterministic repo IDs",
					"anchors": []string{
						"internal/core/config/config.go",
						"internal/core/config/registry.go",
						"internal/core/config/datadir.go",
						"<data-dir>/<repo-id>/metrics/*.jsonl",
						"*claims.Claim",
						"cand.EpisodeID",
						"*_test.go",
						"~/.gemini/antigravity/brain",
						"modernc.org/sqlite",
					},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-valid002",
				DossierScope: "modernc.org",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "decision",
					"text":       "No anchors remain after stripping module path",
					"anchors": []string{
						"modernc.org/sqlite",
						"~/.gemini/config.json",
					},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "seed-customprune",
				DossierScope: "core/config",
				Timestamp:    now,
				Source:       ops.SourceSeed,
				Fields: map[string]any{
					"entry_type": "decision",
					"text":       "Should be pruned via PruneIDs",
					"anchors":    []string{"internal/core/config/config.go"},
				},
			},
		},
	}

	writeBatch := func(path string, b ops.OpBatch) {
		data, err := json.MarshalIndent(b, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeBatch(emptyAfterGCPath, batch1)
	writeBatch(mixedBatchPath, batch2)

	stats, err := seed.RunOpsGC(context.Background(), seed.GCOptions{
		OpsDir:      opsDir,
		DossiersDir: tmpDir,
		PruneIDs:    []string{"seed-customprune"},
	})
	if err != nil {
		t.Fatalf("RunOpsGC failed: %v", err)
	}

	if stats.BatchesScanned != 2 {
		t.Errorf("BatchesScanned = %d, want 2", stats.BatchesScanned)
	}
	if stats.BatchesDeleted != 1 {
		t.Errorf("BatchesDeleted = %d, want 1", stats.BatchesDeleted)
	}
	if stats.BatchesModified != 1 {
		t.Errorf("BatchesModified = %d, want 1", stats.BatchesModified)
	}
	if stats.OpsRemoved != 8 {
		t.Errorf("OpsRemoved = %d, want 8", stats.OpsRemoved)
	}
	if stats.AnchorsRemoved != 8 {
		t.Errorf("AnchorsRemoved = %d, want 8", stats.AnchorsRemoved)
	}
	if stats.ScopesRepaired != 2 {
		t.Errorf("ScopesRepaired = %d, want 2", stats.ScopesRepaired)
	}

	if _, err := os.Stat(emptyAfterGCPath); !os.IsNotExist(err) {
		t.Errorf("expected empty batch %s to be removed, stat err = %v", emptyAfterGCPath, err)
	}

	updatedData, err := os.ReadFile(mixedBatchPath)
	if err != nil {
		t.Fatal(err)
	}
	var updatedBatch ops.OpBatch
	if err := json.Unmarshal(updatedData, &updatedBatch); err != nil {
		t.Fatal(err)
	}
	if len(updatedBatch.Ops) != 2 {
		t.Fatalf("got %d surviving ops, want 2", len(updatedBatch.Ops))
	}

	op1 := updatedBatch.Ops[0]
	if op1.DossierScope != "core/config" {
		t.Errorf("op1.DossierScope = %q, want %q", op1.DossierScope, "core/config")
	}
	rawAnchors1, _ := op1.Fields["anchors"].([]any)
	if len(rawAnchors1) != 3 {
		t.Errorf("op1 anchors = %v, want 3 valid config paths", rawAnchors1)
	}

	op2 := updatedBatch.Ops[1]
	if op2.DossierScope != "root" {
		t.Errorf("op2.DossierScope = %q, want %q fallback", op2.DossierScope, "root")
	}

	// Verify stale dossier files were removed and valid ones rendered.
	for _, stale := range []string{".agents.md", "modernc.org.md", "pending.md", filepath.Join("<data-dir>", "<repo-id>.md")} {
		if _, err := os.Stat(filepath.Join(staleDossiersDir, stale)); !os.IsNotExist(err) {
			t.Errorf("stale dossier %s still exists", stale)
		}
	}
	if _, err := os.Stat(filepath.Join(staleDossiersDir, "core", "config.md")); err != nil {
		t.Errorf("expected projected core/config.md to exist: %v", err)
	}
	atlasBytes, err := os.ReadFile(filepath.Join(tmpDir, "ATLAS.md"))
	if err != nil {
		t.Fatalf("expected ATLAS.md to be written: %v", err)
	}
	if strings.Contains(string(atlasBytes), "modernc.org") || strings.Contains(string(atlasBytes), "<data-dir>") {
		t.Errorf("ATLAS.md still contains polluted scopes:\n%s", string(atlasBytes))
	}
}
