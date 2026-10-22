/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package inbox_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
)

func setupHarvestRepoRoot(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()

	files := map[string]string{
		"internal/core/tick/pipeline.go": "package tick\n",
		"internal/core/inbox/note.go":    "package inbox\n",
		"cmd/repo-memory/main.go":        "package main\n",
		"Makefile":                       "all:\n",
	}
	for rel, content := range files {
		full := filepath.Join(repoRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return repoRoot
}

func TestHarvestCheckpointsAndArtifacts_CheckpointExtraction(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "repo-harvest-chk"
	repoRoot := setupHarvestRepoRoot(t)

	epStore := episode.NewStore(dataDir, repoID)
	inboxStore := inbox.NewStore(dataDir, repoID)

	ep := &episode.Episode{
		Header: episode.Header{
			EpisodeID:      "ep-01",
			SessionID:      "conv-chk-01",
			RepoID:         repoID,
			Branch:         "main",
			HeadStart:      "abc1234",
			HeadEnd:        "def5678",
			Model:          "gemini-2.5-pro",
			HarnessVersion: "1.3.3",
			Checkpoints: map[string]string{
				"Model Knowledge": strings.Join([]string{
					"- **Boundary Rule**: `internal/core/tick/pipeline.go` must never import concrete harness adapters directly.",
					"- **Relative Hook Paths**: Relative paths in `cmd/repo-memory/main.go` were ruled out because hooks execute outside repo root.",
					"- Unanchored general observation without any repo files.",
					"- (none)",
				}, "\n"),
				"User Knowledge": strings.Join([]string{
					"- **Note Format Gotcha**: Caveat in `internal/core/inbox/note.go#CheckAnchors` when stripping line numbers.",
				}, "\n"),
				"Work Accomplished": strings.Join([]string{
					"- Updated test files.",
					"- Fixed panic in `Makefile` target execution when env vars are missing.",
				}, "\n"),
				"Files and Code": "- `internal/core/tick/pipeline.go`\n- `internal/core/inbox/note.go`",
			},
		},
	}

	if _, err := epStore.WriteEpisode(ep); err != nil {
		t.Fatalf("WriteEpisode: %v", err)
	}

	created, err := inbox.HarvestCheckpointsAndArtifacts(
		context.Background(),
		dataDir,
		repoID,
		repoRoot,
		epStore,
		inboxStore,
	)
	if err != nil {
		t.Fatalf("HarvestCheckpointsAndArtifacts failed: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 note created, got %d", created)
	}

	uncons, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatalf("ListUnconsolidated failed: %v", err)
	}
	if len(uncons) != 1 {
		t.Fatalf("expected 1 unconsolidated note, got %d", len(uncons))
	}

	note := uncons[0]
	if note.Source != inbox.SourceCheckpoint {
		t.Errorf("expected Source %q, got %q", inbox.SourceCheckpoint, note.Source)
	}
	if note.ConversationID != "conv-chk-01" {
		t.Errorf("expected ConversationID conv-chk-01, got %q", note.ConversationID)
	}
	if note.HeadCommit != "def5678" {
		t.Errorf("expected HeadCommit def5678, got %q", note.HeadCommit)
	}
	if !strings.HasPrefix(note.DedupKey, "checkpoint:conv-chk-01:") {
		t.Errorf("unexpected DedupKey %q", note.DedupKey)
	}

	if len(note.Items) != 4 {
		t.Fatalf("expected 4 anchored items (skipping unanchored, junk, and routine Work Accomplished), got %d: %+v", len(note.Items), note.Items)
	}

	wantTypes := []dossiers.EntryType{
		dossiers.EntryTypeInvariant,
		dossiers.EntryTypeRuledOut,
		dossiers.EntryTypeGotcha,
		dossiers.EntryTypeFailureSignature,
	}
	for i, wantType := range wantTypes {
		if note.Items[i].Type != wantType {
			t.Errorf("item[%d] (%q): got Type=%q, want %q", i, note.Items[i].Title, note.Items[i].Type, wantType)
		}
		if len(note.Items[i].Anchors) == 0 {
			t.Errorf("item[%d]: expected non-empty Anchors", i)
		}
		if len(note.Items[i].AnchorChecks) == 0 || !note.Items[i].AnchorChecks[0].Exists {
			t.Errorf("item[%d]: expected verified existing AnchorChecks, got %+v", i, note.Items[i].AnchorChecks)
		}
		if !strings.HasPrefix(note.Items[i].Evidence, "checkpoint (") {
			t.Errorf("item[%d]: unexpected Evidence %q", i, note.Items[i].Evidence)
		}
	}
}

func TestHarvestCheckpointsAndArtifacts_ArtifactExtractionAndIdempotency(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "repo-harvest-art"
	repoRoot := setupHarvestRepoRoot(t)
	convID := "conv-art-99"

	epStore := episode.NewStore(dataDir, repoID)
	inboxStore := inbox.NewStore(dataDir, repoID)

	artDir := filepath.Join(dataDir, repoID, "evidence", "artifacts", convID)
	if err := os.MkdirAll(filepath.Join(artDir, ".system_generated"), 0755); err != nil {
		t.Fatal(err)
	}

	analysisContent := strings.Join([]string{
		"# Root Cause Analysis",
		"",
		"## Findings",
		"- **Pipeline Hook Order**: `internal/core/tick/pipeline.go` must always run free harvest right after capture conversion.",
		"- **Abandoned Approach**: Direct file polling in `cmd/repo-memory/main.go` failed under concurrent runs and was ruled out.",
		"- Generic narrative bullet without any file path reference.",
		"- (none)",
	}, "\n")

	if err := os.WriteFile(filepath.Join(artDir, "analysis.md"), []byte(analysisContent), 0644); err != nil {
		t.Fatal(err)
	}
	// Noise files that must be skipped
	if err := os.WriteFile(filepath.Join(artDir, "analysis.md.metadata.json"), []byte(`{"summary":"ignore `+"`Makefile`"+`"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artDir, "analysis.md.resolved"), []byte("- `Makefile` must never be harvested from resolved files."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artDir, ".system_generated", "hidden.md"), []byte("- `Makefile` must never be harvested from system_generated."), 0644); err != nil {
		t.Fatal(err)
	}

	created, err := inbox.HarvestCheckpointsAndArtifacts(
		context.Background(),
		dataDir,
		repoID,
		repoRoot,
		epStore,
		inboxStore,
	)
	if err != nil {
		t.Fatalf("first HarvestCheckpointsAndArtifacts failed: %v", err)
	}
	if created != 1 {
		t.Fatalf("expected 1 artifact note created, got %d", created)
	}

	notes, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatalf("ListUnconsolidated: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 unconsolidated note, got %d", len(notes))
	}

	note := notes[0]
	if note.Source != inbox.SourceArtifact {
		t.Errorf("expected Source %q, got %q", inbox.SourceArtifact, note.Source)
	}
	if note.ConversationID != convID {
		t.Errorf("expected ConversationID %q, got %q", convID, note.ConversationID)
	}
	if !strings.HasPrefix(note.DedupKey, "artifact:"+convID+":analysis.md:") {
		t.Errorf("unexpected DedupKey %q", note.DedupKey)
	}
	if len(note.Items) != 2 {
		t.Fatalf("expected 2 anchored items from analysis.md, got %d: %+v", len(note.Items), note.Items)
	}
	for i, it := range note.Items {
		if it.Evidence != "artifact:analysis.md" {
			t.Errorf("item[%d]: got Evidence=%q, want artifact:analysis.md", i, it.Evidence)
		}
		if len(it.Anchors) == 0 || len(it.AnchorChecks) == 0 || !it.AnchorChecks[0].Exists {
			t.Errorf("item[%d]: expected valid verified anchor, got Anchors=%v Checks=%+v", i, it.Anchors, it.AnchorChecks)
		}
	}

	// Second run: idempotency check
	createdSecond, err := inbox.HarvestCheckpointsAndArtifacts(
		context.Background(),
		dataDir,
		repoID,
		repoRoot,
		epStore,
		inboxStore,
	)
	if err != nil {
		t.Fatalf("second HarvestCheckpointsAndArtifacts failed: %v", err)
	}
	if createdSecond != 0 {
		t.Errorf("expected 0 notes created on idempotent second run, got %d", createdSecond)
	}

	allAfterSecond, err := inboxStore.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(allAfterSecond) != 1 {
		t.Errorf("expected 1 total note after second run, got %d", len(allAfterSecond))
	}
}
