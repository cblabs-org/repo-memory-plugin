/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package consolidate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/scrub"
)

func TestValidateAndPrepareOps_StateAwareAndLifecycle(t *testing.T) {
	now := time.Date(2026, 10, 16, 8, 30, 0, 0, time.UTC)

	existing := map[string]*dossiers.Dossier{
		"internal/auth": {
			Scope:   "internal/auth",
			Kind:    dossiers.DossierKindModule,
			Purpose: "Auth module memory.",
			Entries: []dossiers.Entry{
				{
					ID:           "inv-01J00000000000000000000001",
					Type:         dossiers.EntryTypeInvariant,
					DossierScope: "internal/auth",
					Text:         "**Token expiry invariant**\nWhy: tokens must expire.\nDo: enforce TTL.",
				},
				{
					ID:            "got-01J00000000000000000000002",
					Type:          dossiers.EntryTypeGotcha,
					DossierScope:  "internal/auth",
					Text:          "**Old retired gotcha**",
					Retired:       true,
					RetiredReason: "already retired",
				},
			},
			History: []dossiers.Entry{
				{
					ID:            "dec-01J00000000000000000000003",
					Type:          dossiers.EntryTypeDecision,
					DossierScope:  "internal/auth",
					Text:          "**Historical decision**",
					Retired:       true,
					RetiredReason: "superseded",
				},
			},
		},
	}

	// 1. Verify OpAdd auto-generates a typed entry ID and sets default purpose for a new scope.
	addOp := ops.Op{
		Type:         ops.OpAdd,
		DossierScope: "internal/billing",
		Fields: map[string]any{
			"type": "recipe",
			"text": "**Run billing tests**\nDo: run `go test ./internal/billing/...`.",
		},
	}
	validFirst, rejectedFirst := ValidateAndPrepareOps([]ops.Op{addOp}, existing, scrub.Context{}, now)
	if len(rejectedFirst) != 0 {
		t.Fatalf("expected 0 rejected ops, got %d: %+v", len(rejectedFirst), rejectedFirst)
	}
	if len(validFirst) != 1 {
		t.Fatalf("expected 1 valid op, got %d", len(validFirst))
	}
	generatedID := validFirst[0].EntryID
	if !strings.HasPrefix(generatedID, "rec-") {
		t.Fatalf("expected generated recipe entry ID to have prefix 'rec-', got %q", generatedID)
	}
	if validFirst[0].Fields["entry_type"] != "recipe" {
		t.Fatalf("expected entry_type to be populated as 'recipe', got %v", validFirst[0].Fields["entry_type"])
	}
	if validFirst[0].Fields["purpose"] == "" {
		t.Fatalf("expected default purpose on newly created dossier scope")
	}

	// 2. Verify batch lifecycle: OpAdd followed by OpAmend on that newly added entry,
	// plus rejections for duplicate add, non-existent/retired amend/supersede/retire/move/split, and >6-line text.
	batchCandidates := []ops.Op{
		// Index 0: Valid OpAdd with explicit ID, followed by Index 1 OpAmend referencing it in the same batch.
		{
			Type:         ops.OpAdd,
			EntryID:      "got-01J00000000000000000000010",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"entry_type": "gotcha",
				"text":       "**Clock skew gotcha**\nWhy: NTP drift.\nDo: allow 5s skew.",
			},
		},
		// Index 1: Valid OpAmend targeting the entry just added at Index 0.
		{
			Type:         ops.OpAmend,
			EntryID:      "got-01J00000000000000000000010",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"text": "**Clock skew gotcha**\nWhy: NTP drift on edge nodes.\nDo: allow 10s skew.",
			},
		},
		// Index 2: Duplicate OpAdd using an already-known ID from history.
		{
			Type:         ops.OpAdd,
			EntryID:      "dec-01J00000000000000000000003",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"type": "decision",
				"text": "**Duplicate ID attempt**",
			},
		},
		// Index 3: Reject OpAmend against already-retired entry.
		{
			Type:         ops.OpAmend,
			EntryID:      "got-01J00000000000000000000002",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"text": "**Cannot amend retired entry**",
			},
		},
		// Index 4: Reject OpSupersede against non-existent entry.
		{
			Type:         ops.OpSupersede,
			EntryID:      "inv-nonexistent",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"new_text": "**Replacement text**",
			},
		},
		// Index 5: Reject OpRetire against non-existent entry.
		{
			Type:         ops.OpRetire,
			EntryID:      "inv-nonexistent",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"reason": "obsolete",
			},
		},
		// Index 6: Reject OpMove against non-existent entry.
		{
			Type:         ops.OpMove,
			EntryID:      "inv-nonexistent",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"target_scope": "internal/session",
			},
		},
		// Index 7: Reject OpSplit against empty scope.
		{
			Type:         ops.OpSplit,
			DossierScope: "internal/empty",
			Fields: map[string]any{
				"new_scopes":  []string{"internal/empty/a", "internal/empty/b"},
				"entry_moves": map[string]string{"inv-01J00000000000000000000001": "internal/empty/a"},
			},
		},
		// Index 8: Reject > 6 non-empty lines in entry text.
		{
			Type:         ops.OpAdd,
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"type": "gotcha",
				"text": "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7",
			},
		},
		// Index 9: Valid OpSupersede on active entry inv-01J00000000000000000000001, inheriting its type.
		{
			Type:         ops.OpSupersede,
			EntryID:      "inv-01J00000000000000000000001",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"new_text": "**Updated token expiry invariant**\nDo: enforce strict 15m TTL.",
			},
		},
		// Index 10: Reject OpRetire on inv-01J00000000000000000000001 because Index 9 just superseded it!
		{
			Type:         ops.OpRetire,
			EntryID:      "inv-01J00000000000000000000001",
			DossierScope: "internal/auth",
			Fields: map[string]any{
				"reason": "already superseded",
			},
		},
	}

	valid, rejected := ValidateAndPrepareOps(batchCandidates, existing, scrub.Context{}, now)
	if len(valid) != 3 {
		t.Fatalf("expected 3 valid ops (indices 0, 1, 9), got %d: %+v", len(valid), valid)
	}
	if len(rejected) != 8 {
		t.Fatalf("expected 8 rejected ops, got %d: %+v", len(rejected), rejected)
	}

	// Verify the valid supersede inherited type "invariant" and generated an "inv-" new_entry_id.
	supOp := valid[2]
	if supOp.Type != ops.OpSupersede {
		t.Fatalf("expected 3rd valid op to be supersede, got %s", supOp.Type)
	}
	newID, _ := supOp.Fields["new_entry_id"].(string)
	if !strings.HasPrefix(newID, "inv-") {
		t.Fatalf("expected superseded entry new_entry_id to inherit 'inv-' prefix, got %q", newID)
	}
	if supOp.Fields["entry_type"] != "invariant" {
		t.Fatalf("expected superseded entry to inherit entry_type 'invariant', got %v", supOp.Fields["entry_type"])
	}

	// Verify 6-line cap rejection reason at index 8.
	foundSixLineReject := false
	for _, r := range rejected {
		if r.Index == 8 && r.Reason == "entry text exceeds 6-line cap" {
			foundSixLineReject = true
		}
	}
	if !foundSixLineReject {
		t.Fatalf("expected index 8 to be rejected with 'entry text exceeds 6-line cap', got %+v", rejected)
	}
}

func TestValidateAndPrepareOps_ScrubberRedactionAndQuarantine(t *testing.T) {
	now := time.Date(2026, 10, 16, 8, 45, 0, 0, time.UTC)
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo-id"

	noteID := "note-01JABCDEFGHJKMNPQRSTVWXYZ1"
	entryID := "got-01JABCDEFGHJKMNPQRSTVWXYZ2"

	candidate := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      entryID,
		DossierScope: "internal/cloud",
		Fields: map[string]any{
			"type":     "gotcha",
			"text":     "**AWS key leak in test fixture**\nWhy: fixture used AKIAIOSFODNN7EXAMPLE and ghp_1234567890abcdefghijklmnopqrstuv in notes (" + noteID + ").\nDo: use mock credentials for " + entryID + ".",
			"evidence": []string{noteID},
		},
	}

	scrubCtx := scrub.Context{
		RepoRoot: repoRoot,
		DataDir:  dataDir,
		RepoID:   repoID,
	}

	valid, rejected := ValidateAndPrepareOps([]ops.Op{candidate}, nil, scrubCtx, now)
	if len(rejected) != 0 {
		t.Fatalf("expected 0 rejected ops on clean scrub config, got %+v", rejected)
	}
	if len(valid) != 1 {
		t.Fatalf("expected 1 valid op, got %d", len(valid))
	}

	scrubbedText, _ := valid[0].Fields["text"].(string)
	if strings.Contains(scrubbedText, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("expected AWS key to be redacted, got: %s", scrubbedText)
	}
	if strings.Contains(scrubbedText, "ghp_1234567890abcdefghijklmnopqrstuv") {
		t.Fatalf("expected GitHub token to be redacted, got: %s", scrubbedText)
	}
	if !strings.Contains(scrubbedText, "«secret:aws»") || !strings.Contains(scrubbedText, "«secret:github»") {
		t.Fatalf("expected redaction markers in text, got: %s", scrubbedText)
	}
	if !strings.Contains(scrubbedText, noteID) || !strings.Contains(scrubbedText, entryID) {
		t.Fatalf("expected legitimate noteID %q and entryID %q to be preserved, got: %s", noteID, entryID, scrubbedText)
	}

	// Now write a malformed .memory/scrub.yaml containing a forbidden "allow:" key.
	memDir := filepath.Join(repoRoot, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir .memory: %v", err)
	}
	badConfig := "version: 1\nallow:\n  - AKIAIOSFODNN7EXAMPLE\n"
	if err := os.WriteFile(filepath.Join(memDir, "scrub.yaml"), []byte(badConfig), 0o644); err != nil {
		t.Fatalf("write scrub.yaml: %v", err)
	}

	validFail, rejectedFail := ValidateAndPrepareOps([]ops.Op{candidate}, nil, scrubCtx, now)
	if len(validFail) != 0 {
		t.Fatalf("expected 0 valid ops when scrub.yaml is invalid, got %d", len(validFail))
	}
	if len(rejectedFail) != 1 {
		t.Fatalf("expected 1 rejected op when scrub.yaml is invalid, got %d", len(rejectedFail))
	}
	if !strings.HasPrefix(rejectedFail[0].Reason, "scrubber rejected op:") {
		t.Fatalf("expected scrubber rejection reason, got %q", rejectedFail[0].Reason)
	}

	quarantineDir := filepath.Join(dataDir, repoID, "quarantine")
	qEntries, err := os.ReadDir(quarantineDir)
	if err != nil {
		t.Fatalf("expected quarantine directory to exist at %s: %v", quarantineDir, err)
	}
	if len(qEntries) != 1 {
		t.Fatalf("expected 1 quarantine file, got %d", len(qEntries))
	}
	if !strings.HasSuffix(qEntries[0].Name(), "-op.quarantine") {
		t.Fatalf("expected quarantine file suffix '-op.quarantine', got %q", qEntries[0].Name())
	}
}

func TestRecordAndRenderTier_EndToEndProjectionAndMarkConsolidated(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

	tierBaseDir := t.TempDir()
	dataDir := t.TempDir()
	repoID := "repo-e2e"

	inboxStore := inbox.NewStore(dataDir, repoID)
	note1ID, err := inboxStore.Write(&inbox.Note{
		Source: inbox.SourceTurnNote,
		Items: []inbox.NoteItem{
			{
				Type:  dossiers.EntryTypeInvariant,
				Title: "Database connection pooling",
				Why:   "Prevent connection exhaustion",
				Do:    "Always share a single sql.DB pool per service.",
			},
		},
		CreatedAt: now.Add(-10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("write note1: %v", err)
	}
	note2ID, err := inboxStore.Write(&inbox.Note{
		Source: inbox.SourceTurnNote,
		Items: []inbox.NoteItem{
			{
				Type:  dossiers.EntryTypeGotcha,
				Title: "Legacy retry loop",
				Why:   "Causes thundering herd",
				Do:    "Use exponential backoff with jitter.",
			},
		},
		CreatedAt: now.Add(-5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("write note2: %v", err)
	}

	rawOps := []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-01J00000000000000000000101",
			DossierScope: "internal/db",
			Fields: map[string]any{
				"type":     "invariant",
				"text":     "**Database connection pooling**\nWhy: Prevent connection exhaustion.\nDo: Always share a single sql.DB pool.",
				"evidence": []string{note1ID},
			},
		},
		{
			Type:         ops.OpAdd,
			EntryID:      "got-01J00000000000000000000102",
			DossierScope: "internal/db",
			Fields: map[string]any{
				"type":     "gotcha",
				"text":     "**Legacy retry loop**\nWhy: Causes thundering herd.\nDo: Use backoff.",
				"evidence": []string{note2ID},
			},
		},
		{
			Type:         ops.OpSupersede,
			EntryID:      "inv-01J00000000000000000000101",
			DossierScope: "internal/db",
			Fields: map[string]any{
				"new_entry_id": "inv-01J00000000000000000000103",
				"new_text":     "**Database connection pooling**\nWhy: Prevent connection exhaustion under burst load.\nDo: Share a single sql.DB pool with MaxOpenConns=32.",
				"evidence":     []string{note1ID, note2ID},
			},
		},
		{
			Type:         ops.OpRetire,
			EntryID:      "got-01J00000000000000000000102",
			DossierScope: "internal/db",
			Fields: map[string]any{
				"reason": "retry loop removed in v2 client",
			},
		},
	}

	validOps, rejected := ValidateAndPrepareOps(rawOps, nil, scrub.Context{
		DataDir: dataDir,
		RepoID:  repoID,
	}, now)
	if len(rejected) != 0 {
		t.Fatalf("expected 0 rejected ops, got %+v", rejected)
	}
	if len(validOps) != 4 {
		t.Fatalf("expected 4 valid ops, got %d", len(validOps))
	}

	res, err := RecordAndRenderTier(ctx, tierBaseDir, validOps, inboxStore, []string{note1ID, note2ID, note1ID}, now)
	if err != nil {
		t.Fatalf("RecordAndRenderTier failed: %v", err)
	}
	if res.Batch == nil {
		t.Fatalf("expected non-nil Batch in result")
	}

	// 1. Verify the .json batch file exists under <tierBaseDir>/ops/<yyyy-mm>/<batch-id>.json.
	expectedBatchPath := filepath.Join(tierBaseDir, "ops", now.Format("2006-01"), res.Batch.BatchID+".json")
	if _, err := os.Stat(expectedBatchPath); err != nil {
		t.Fatalf("expected batch file at %s: %v", expectedBatchPath, err)
	}

	// 2. Verify <tierBaseDir>/dossiers/internal/db.md, .history.md, and ATLAS.md are rendered on disk.
	dossierPath := filepath.Join(tierBaseDir, "dossiers", "internal", "db.md")
	historyPath := filepath.Join(tierBaseDir, "dossiers", "internal", "db.history.md")
	atlasPath := filepath.Join(tierBaseDir, "ATLAS.md")

	for _, p := range []string{dossierPath, historyPath, atlasPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected rendered file at %s: %v", p, err)
		}
	}

	// 3. Verify len(res.LintWarnings) == 0.
	if len(res.LintWarnings) != 0 {
		t.Fatalf("expected 0 lint warnings, got %+v", res.LintWarnings)
	}

	// 4. Verify inboxStore.ListUnconsolidated() returns 0 notes (Consolidated == true on both notes).
	unconsolidated, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatalf("ListUnconsolidated: %v", err)
	}
	if len(unconsolidated) != 0 {
		t.Fatalf("expected 0 unconsolidated notes, got %d", len(unconsolidated))
	}
	if len(res.MarkedNotes) != 2 {
		t.Fatalf("expected 2 deduplicated MarkedNotes, got %v", res.MarkedNotes)
	}
}
