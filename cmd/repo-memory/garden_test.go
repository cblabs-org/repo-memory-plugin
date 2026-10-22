/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

func TestGardenCmd_DryRunJSON(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	now := time.Date(2026, 10, 16, 17, 0, 0, 0, time.UTC)
	seedOps := []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-missing-anchor",
			DossierScope: "core",
			Timestamp:    now,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeInvariant),
				"text":       "**Missing anchor invariant**\nDo: Check missing.go.",
				"anchors":    []string{"internal/missing.go"},
				"evidence":   []string{"ep-1"},
			},
		},
	}
	if _, err := consolidate.RecordRepoTier(ctx, repoRoot, seedOps, nil, nil, now); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"garden", repoRoot, "--dry-run", "--json"})
	defer func() {
		gardenRepoDir = ""
		gardenDryRun = false
		gardenJSON = false
		gardenMaxTokens = 3000
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("garden command failed: %v\nOutput: %s", err, out.String())
	}

	var res tick.GardeningResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal garden JSON output: %v\nRaw: %s", err, out.String())
	}
	if len(res.RetiredBrokenAnchors) != 1 || res.RetiredBrokenAnchors[0] != "inv-missing-anchor" {
		t.Fatalf("expected RetiredBrokenAnchors=[inv-missing-anchor], got %+v", res)
	}

	// Verify dry-run left dossier file on disk intact.
	if _, err := os.Stat(filepath.Join(repoRoot, ".memory", "dossiers", "core.md")); err != nil {
		t.Fatalf("expected core.md to remain on disk after dry-run: %v", err)
	}
}
