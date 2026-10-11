package ops

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestValidateOp_AllTypes(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		op      Op
		wantErr bool
		errMsg  string
	}{
		// Add
		{
			name: "add: valid",
			op: Op{
				Type:         OpAdd,
				EntryID:      "inv-01J5K3M2",
				DossierScope: "core/transcript-parser",
				Timestamp:    now,
				Source:       SourceTurnNote,
				Fields: map[string]any{
					"text": "parser must preserve step timestamps",
				},
			},
			wantErr: false,
		},
		{
			name: "add: missing entry_id",
			op: Op{
				Type:         OpAdd,
				DossierScope: "core/transcript-parser",
				Fields: map[string]any{
					"text": "content",
				},
			},
			wantErr: true,
			errMsg:  "entry_id is required",
		},
		{
			name: "add: missing dossier_scope",
			op: Op{
				Type:    OpAdd,
				EntryID: "inv-01J5K3M2",
				Fields: map[string]any{
					"text": "content",
				},
			},
			wantErr: true,
			errMsg:  "dossier_scope is required",
		},
		{
			name: "add: missing fields",
			op: Op{
				Type:         OpAdd,
				EntryID:      "inv-01J5K3M2",
				DossierScope: "core/transcript-parser",
			},
			wantErr: true,
			errMsg:  "fields is required",
		},
		{
			name: "add: missing fields.text",
			op: Op{
				Type:         OpAdd,
				EntryID:      "inv-01J5K3M2",
				DossierScope: "core/transcript-parser",
				Fields: map[string]any{
					"other": "value",
				},
			},
			wantErr: true,
			errMsg:  "fields.text is required",
		},
		{
			name: "add: empty fields.text",
			op: Op{
				Type:         OpAdd,
				EntryID:      "inv-01J5K3M2",
				DossierScope: "core/transcript-parser",
				Fields: map[string]any{
					"text": "   ",
				},
			},
			wantErr: true,
			errMsg:  "fields.text must be a non-empty string",
		},

		// Amend
		{
			name: "amend: valid",
			op: Op{
				Type:    OpAmend,
				EntryID: "inv-01J5K3M2",
				Fields: map[string]any{
					"text": "updated rule text",
				},
			},
			wantErr: false,
		},
		{
			name: "amend: missing entry_id",
			op: Op{
				Type: OpAmend,
				Fields: map[string]any{
					"text": "updated",
				},
			},
			wantErr: true,
			errMsg:  "entry_id is required",
		},
		{
			name: "amend: missing fields",
			op: Op{
				Type:    OpAmend,
				EntryID: "inv-01J5K3M2",
			},
			wantErr: true,
			errMsg:  "fields is required",
		},
		{
			name: "amend: empty fields",
			op: Op{
				Type:    OpAmend,
				EntryID: "inv-01J5K3M2",
				Fields:  map[string]any{},
			},
			wantErr: true,
			errMsg:  "fields is required",
		},

		// Supersede
		{
			name: "supersede: valid",
			op: Op{
				Type:    OpSupersede,
				EntryID: "inv-old",
				Fields: map[string]any{
					"new_entry_id": "inv-new",
					"text":         "new replacement entry",
				},
			},
			wantErr: false,
		},
		{
			name: "supersede: missing entry_id (old)",
			op: Op{
				Type: OpSupersede,
				Fields: map[string]any{
					"new_entry_id": "inv-new",
					"text":         "new replacement entry",
				},
			},
			wantErr: true,
			errMsg:  "entry_id is required",
		},
		{
			name: "supersede: missing new_entry_id",
			op: Op{
				Type:    OpSupersede,
				EntryID: "inv-old",
				Fields: map[string]any{
					"text": "new replacement entry",
				},
			},
			wantErr: true,
			errMsg:  "fields.new_entry_id is required",
		},
		{
			name: "supersede: empty new_entry_id",
			op: Op{
				Type:    OpSupersede,
				EntryID: "inv-old",
				Fields: map[string]any{
					"new_entry_id": "  ",
					"text":         "new replacement entry",
				},
			},
			wantErr: true,
			errMsg:  "fields.new_entry_id must be a non-empty string",
		},
		{
			name: "supersede: missing text",
			op: Op{
				Type:    OpSupersede,
				EntryID: "inv-old",
				Fields: map[string]any{
					"new_entry_id": "inv-new",
				},
			},
			wantErr: true,
			errMsg:  "fields.text is required",
		},

		// Retire
		{
			name: "retire: valid",
			op: Op{
				Type:    OpRetire,
				EntryID: "rul-01",
				Fields: map[string]any{
					"reason": "upstream fixed bug, work-around retired",
				},
			},
			wantErr: false,
		},
		{
			name: "retire: missing entry_id",
			op: Op{
				Type: OpRetire,
				Fields: map[string]any{
					"reason": "obsolete",
				},
			},
			wantErr: true,
			errMsg:  "entry_id is required",
		},
		{
			name: "retire: missing reason",
			op: Op{
				Type:    OpRetire,
				EntryID: "rul-01",
				Fields:  map[string]any{},
			},
			wantErr: true,
			errMsg:  "fields.reason is required",
		},

		// Move
		{
			name: "move: valid",
			op: Op{
				Type:    OpMove,
				EntryID: "inv-01",
				Fields: map[string]any{
					"to_dossier": "external/cli",
				},
			},
			wantErr: false,
		},
		{
			name: "move: missing entry_id",
			op: Op{
				Type: OpMove,
				Fields: map[string]any{
					"to_dossier": "external/cli",
				},
			},
			wantErr: true,
			errMsg:  "entry_id is required",
		},
		{
			name: "move: missing to_dossier",
			op: Op{
				Type:    OpMove,
				EntryID: "inv-01",
				Fields:  map[string]any{},
			},
			wantErr: true,
			errMsg:  "fields.to_dossier is required",
		},

		// Split
		{
			name: "split: valid typed",
			op: Op{
				Type:         OpSplit,
				DossierScope: "core/monolith",
				Fields: map[string]any{
					"new_dossiers": []string{"core/parser", "core/storage"},
					"entry_moves": map[string]string{
						"inv-1": "core/parser",
						"inv-2": "core/storage",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "split: valid any-unmarshaled",
			op: Op{
				Type:         OpSplit,
				DossierScope: "core/monolith",
				Fields: map[string]any{
					"new_dossiers": []any{"core/parser", "core/storage"},
					"entry_moves": map[string]any{
						"inv-1": "core/parser",
						"inv-2": "core/storage",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "split: missing dossier_scope",
			op: Op{
				Type: OpSplit,
				Fields: map[string]any{
					"new_dossiers": []string{"core/parser", "core/storage"},
					"entry_moves": map[string]string{
						"inv-1": "core/parser",
					},
				},
			},
			wantErr: true,
			errMsg:  "dossier_scope is required",
		},
		{
			name: "split: missing new_dossiers",
			op: Op{
				Type:         OpSplit,
				DossierScope: "core/monolith",
				Fields: map[string]any{
					"entry_moves": map[string]string{
						"inv-1": "core/parser",
					},
				},
			},
			wantErr: true,
			errMsg:  "fields.new_dossiers is required",
		},
		{
			name: "split: empty new_dossiers",
			op: Op{
				Type:         OpSplit,
				DossierScope: "core/monolith",
				Fields: map[string]any{
					"new_dossiers": []string{},
					"entry_moves": map[string]string{
						"inv-1": "core/parser",
					},
				},
			},
			wantErr: true,
			errMsg:  "fields.new_dossiers cannot be empty",
		},
		{
			name: "split: missing entry_moves",
			op: Op{
				Type:         OpSplit,
				DossierScope: "core/monolith",
				Fields: map[string]any{
					"new_dossiers": []string{"core/parser", "core/storage"},
				},
			},
			wantErr: true,
			errMsg:  "fields.entry_moves is required",
		},
		{
			name: "split: empty entry_moves",
			op: Op{
				Type:         OpSplit,
				DossierScope: "core/monolith",
				Fields: map[string]any{
					"new_dossiers": []string{"core/parser", "core/storage"},
					"entry_moves":  map[string]string{},
				},
			},
			wantErr: true,
			errMsg:  "fields.entry_moves cannot be empty",
		},

		// General
		{
			name: "unknown op type",
			op: Op{
				Type: OpType("delete"),
			},
			wantErr: true,
			errMsg:  "unknown op type",
		},
		{
			name: "empty op type",
			op: Op{
				Type: "",
			},
			wantErr: true,
			errMsg:  "op type is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOp(tc.op)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errMsg)
				}
				if tc.errMsg != "" && !containsSubstr(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing %q, got %v", tc.errMsg, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateBatch(t *testing.T) {
	validOp := Op{
		Type:         OpAdd,
		EntryID:      "inv-01",
		DossierScope: "core/test",
		Fields:       map[string]any{"text": "some rule"},
	}

	t.Run("valid batch", func(t *testing.T) {
		batch := OpBatch{
			Version:   1,
			BatchID:   NewBatchID(),
			CreatedAt: time.Now().UTC(),
			Ops:       []Op{validOp},
		}
		if err := ValidateBatch(batch); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("version < 1", func(t *testing.T) {
		batch := OpBatch{
			Version:   0,
			BatchID:   NewBatchID(),
			CreatedAt: time.Now().UTC(),
			Ops:       []Op{validOp},
		}
		if err := ValidateBatch(batch); err == nil {
			t.Fatal("expected error for version < 1")
		}
	})

	t.Run("empty batch_id", func(t *testing.T) {
		batch := OpBatch{
			Version:   1,
			BatchID:   "",
			CreatedAt: time.Now().UTC(),
			Ops:       []Op{validOp},
		}
		if err := ValidateBatch(batch); err == nil {
			t.Fatal("expected error for empty batch_id")
		}
	})

	t.Run("zero created_at", func(t *testing.T) {
		batch := OpBatch{
			Version:   1,
			BatchID:   NewBatchID(),
			CreatedAt: time.Time{},
			Ops:       []Op{validOp},
		}
		if err := ValidateBatch(batch); err == nil {
			t.Fatal("expected error for zero created_at")
		}
	})

	t.Run("empty ops", func(t *testing.T) {
		batch := OpBatch{
			Version:   1,
			BatchID:   NewBatchID(),
			CreatedAt: time.Now().UTC(),
			Ops:       nil,
		}
		if err := ValidateBatch(batch); err == nil {
			t.Fatal("expected error for empty ops")
		}
	})

	t.Run("invalid op in batch", func(t *testing.T) {
		batch := OpBatch{
			Version:   1,
			BatchID:   NewBatchID(),
			CreatedAt: time.Now().UTC(),
			Ops: []Op{
				validOp,
				{Type: OpAdd}, // missing required fields
			},
		}
		err := ValidateBatch(batch)
		if err == nil {
			t.Fatal("expected error for invalid op in batch")
		}
		if !containsSubstr(err.Error(), "op[1]") {
			t.Fatalf("expected error referencing op[1], got %v", err)
		}
	})
}

func TestWriterAndReader_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	opsDir := filepath.Join(tempDir, ".memory", "ops")

	writer := NewWriter(opsDir)
	reader := NewReader(opsDir)

	createdAt := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	batchID := NewBatchIDWithTime(createdAt)

	op1 := Op{
		Type:         OpAdd,
		EntryID:      NewEntryID("inv"),
		DossierScope: "core/test",
		Timestamp:    createdAt,
		Source:       SourceTurnNote,
		SessionID:    "sess-123",
		Fields: map[string]any{
			"text": "all tests must pass",
		},
	}

	op2 := Op{
		Type:      OpRetire,
		EntryID:   NewEntryID("rul"),
		Timestamp: createdAt,
		Source:    SourceConsolidation,
		Fields: map[string]any{
			"reason": "superseded by new spec",
		},
	}

	batch := OpBatch{
		Version:   1,
		BatchID:   batchID,
		CreatedAt: createdAt,
		Ops:       []Op{op1, op2},
	}

	if err := writer.Write(batch); err != nil {
		t.Fatalf("write batch: %v", err)
	}

	expectedFilePath := filepath.Join(opsDir, "2026-10", batchID+".json")
	if _, err := os.Stat(expectedFilePath); err != nil {
		t.Fatalf("expected file at %s: %v", expectedFilePath, err)
	}

	// Read using full path
	readBatch, err := reader.ReadBatch(expectedFilePath)
	if err != nil {
		t.Fatalf("read batch with full path: %v", err)
	}

	if readBatch.Version != batch.Version {
		t.Errorf("version mismatch: got %d, want %d", readBatch.Version, batch.Version)
	}
	if readBatch.BatchID != batch.BatchID {
		t.Errorf("batch ID mismatch: got %s, want %s", readBatch.BatchID, batch.BatchID)
	}
	if !readBatch.CreatedAt.Equal(batch.CreatedAt) {
		t.Errorf("created_at mismatch: got %v, want %v", readBatch.CreatedAt, batch.CreatedAt)
	}
	if len(readBatch.Ops) != len(batch.Ops) {
		t.Fatalf("ops len mismatch: got %d, want %d", len(readBatch.Ops), len(batch.Ops))
	}
	for i := range batch.Ops {
		if readBatch.Ops[i].Type != batch.Ops[i].Type {
			t.Errorf("op[%d] type mismatch: got %s, want %s", i, readBatch.Ops[i].Type, batch.Ops[i].Type)
		}
		if readBatch.Ops[i].EntryID != batch.Ops[i].EntryID {
			t.Errorf("op[%d] entry_id mismatch: got %s, want %s", i, readBatch.Ops[i].EntryID, batch.Ops[i].EntryID)
		}
		if !reflect.DeepEqual(readBatch.Ops[i].Fields, batch.Ops[i].Fields) {
			t.Errorf("op[%d] fields mismatch: got %v, want %v", i, readBatch.Ops[i].Fields, batch.Ops[i].Fields)
		}
	}

	// Read using relative path from baseDir
	relPath := filepath.Join("2026-10", batchID+".json")
	readRel, err := reader.ReadBatch(relPath)
	if err != nil {
		t.Fatalf("read batch with relative path: %v", err)
	}
	if readRel.BatchID != batch.BatchID {
		t.Errorf("relative read batch ID mismatch: got %s, want %s", readRel.BatchID, batch.BatchID)
	}
}

func TestReader_Iterate(t *testing.T) {
	tempDir := t.TempDir()
	opsDir := filepath.Join(tempDir, ".memory", "ops")

	writer := NewWriter(opsDir)
	reader := NewReader(opsDir)

	// Create 3 batches across 2 months:
	// Month 1: 2026-08 (1 batch)
	// Month 2: 2026-09 (2 batches)
	t1 := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)

	b1 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t1),
		CreatedAt: t1,
		Ops: []Op{
			{Type: OpAdd, EntryID: "inv-01", DossierScope: "s1", Fields: map[string]any{"text": "op1"}},
		},
	}
	b2 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t2),
		CreatedAt: t2,
		Ops: []Op{
			{Type: OpAdd, EntryID: "inv-02", DossierScope: "s1", Fields: map[string]any{"text": "op2"}},
		},
	}
	b3 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t3),
		CreatedAt: t3,
		Ops: []Op{
			{Type: OpAdd, EntryID: "inv-03", DossierScope: "s1", Fields: map[string]any{"text": "op3"}},
		},
	}

	for _, b := range []OpBatch{b2, b1, b3} { // write out of order to ensure iterator sorts
		if err := writer.Write(b); err != nil {
			t.Fatalf("write batch: %v", err)
		}
	}

	ctx := context.Background()

	// 1. Test push iterator (range over iter.Seq2)
	var gotIDs []string
	for batch, err := range reader.Iterate(ctx) {
		if err != nil {
			t.Fatalf("unexpected error during Iterate: %v", err)
		}
		gotIDs = append(gotIDs, batch.BatchID)
	}

	wantIDs := []string{b1.BatchID, b2.BatchID, b3.BatchID}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("batches not in chronological order:\ngot  %v\nwant %v", gotIDs, wantIDs)
	}

	// 2. Test pull iterator (Next)
	it := NewIterator(reader, ctx)
	defer it.Close()

	var pullIDs []string
	for {
		batch, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("pull iterator error: %v", err)
		}
		pullIDs = append(pullIDs, batch.BatchID)
	}

	if !reflect.DeepEqual(pullIDs, wantIDs) {
		t.Fatalf("pull iterator mismatch:\ngot  %v\nwant %v", pullIDs, wantIDs)
	}
}

func TestReader_IterateAfter(t *testing.T) {
	tempDir := t.TempDir()
	opsDir := filepath.Join(tempDir, ".memory", "ops")

	writer := NewWriter(opsDir)
	reader := NewReader(opsDir)

	t1 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

	b1 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t1),
		CreatedAt: t1,
		Ops:       []Op{{Type: OpAdd, EntryID: "inv-01", DossierScope: "s1", Fields: map[string]any{"text": "op1"}}},
	}
	b2 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t2),
		CreatedAt: t2,
		Ops:       []Op{{Type: OpAdd, EntryID: "inv-02", DossierScope: "s1", Fields: map[string]any{"text": "op2"}}},
	}
	b3 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t3),
		CreatedAt: t3,
		Ops:       []Op{{Type: OpAdd, EntryID: "inv-03", DossierScope: "s1", Fields: map[string]any{"text": "op3"}}},
	}

	for _, b := range []OpBatch{b1, b2, b3} {
		if err := writer.Write(b); err != nil {
			t.Fatalf("write batch: %v", err)
		}
	}

	ctx := context.Background()

	// Iterate after 1st batch: should return only 2nd and 3rd
	var gotIDs []string
	for batch, err := range reader.IterateAfter(ctx, b1.BatchID) {
		if err != nil {
			t.Fatalf("iterate after error: %v", err)
		}
		gotIDs = append(gotIDs, batch.BatchID)
	}

	wantIDs := []string{b2.BatchID, b3.BatchID}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("IterateAfter(b1):\ngot  %v\nwant %v", gotIDs, wantIDs)
	}

	// Iterate after 3rd batch: should return empty
	var gotIDsAfterLast []string
	for batch, err := range reader.IterateAfter(ctx, b3.BatchID) {
		if err != nil {
			t.Fatalf("iterate after error: %v", err)
		}
		gotIDsAfterLast = append(gotIDsAfterLast, batch.BatchID)
	}
	if len(gotIDsAfterLast) != 0 {
		t.Fatalf("expected 0 batches after last, got %v", gotIDsAfterLast)
	}

	// Iterate after empty string: should return all batches
	var gotAll []string
	for batch, err := range reader.IterateAfter(ctx, "") {
		if err != nil {
			t.Fatalf("iterate after error: %v", err)
		}
		gotAll = append(gotAll, batch.BatchID)
	}
	if len(gotAll) != 3 {
		t.Fatalf("expected 3 batches after empty string, got %d", len(gotAll))
	}
}

func TestReader_IterateBefore_And_BeforeTime(t *testing.T) {
	tempDir := t.TempDir()
	opsDir := filepath.Join(tempDir, ".memory", "ops")

	writer := NewWriter(opsDir)
	reader := NewReader(opsDir)

	t1 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)

	b1 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t1),
		CreatedAt: t1,
		Ops:       []Op{{Type: OpAdd, EntryID: "inv-01", DossierScope: "s1", Fields: map[string]any{"text": "op1"}}},
	}
	b2 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t2),
		CreatedAt: t2,
		Ops:       []Op{{Type: OpAdd, EntryID: "inv-02", DossierScope: "s1", Fields: map[string]any{"text": "op2"}}},
	}
	b3 := OpBatch{
		Version:   1,
		BatchID:   NewBatchIDWithTime(t3),
		CreatedAt: t3,
		Ops:       []Op{{Type: OpAdd, EntryID: "inv-03", DossierScope: "s1", Fields: map[string]any{"text": "op3"}}},
	}

	for _, b := range []OpBatch{b1, b2, b3} {
		if err := writer.Write(b); err != nil {
			t.Fatalf("write batch: %v", err)
		}
	}

	ctx := context.Background()

	// 1. Test IterateBeforeTime directly
	cutoff := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	var gotBeforeTime []string
	for batch, err := range reader.IterateBeforeTime(ctx, cutoff) {
		if err != nil {
			t.Fatalf("IterateBeforeTime error: %v", err)
		}
		gotBeforeTime = append(gotBeforeTime, batch.BatchID)
	}

	wantBeforeTime := []string{b1.BatchID, b2.BatchID}
	if !reflect.DeepEqual(gotBeforeTime, wantBeforeTime) {
		t.Fatalf("IterateBeforeTime mismatch:\ngot  %v\nwant %v", gotBeforeTime, wantBeforeTime)
	}

	// 2. Test IterateBefore with git commit
	gitDir := filepath.Join(tempDir, "gitrepo")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir gitdir: %v", err)
	}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", gitDir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
		return string(out)
	}

	runGit("init")
	runGit("config", "user.name", "Test Runner")
	runGit("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(gitDir, "file.txt"), []byte("commit 1"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit("add", "file.txt")

	// Set commit date explicitly in git
	cmd := exec.Command("git", "-C", gitDir, "commit", "-m", "commit 1")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-09-07T12:00:00Z",
		"GIT_COMMITTER_DATE=2026-09-07T12:00:00Z",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v (output: %s)", err, string(out))
	}

	reader.WithGitDir(gitDir)

	var gotBeforeGit []string
	for batch, err := range reader.IterateBefore(ctx, "HEAD") {
		if err != nil {
			t.Fatalf("IterateBefore git error: %v", err)
		}
		gotBeforeGit = append(gotBeforeGit, batch.BatchID)
	}

	if !reflect.DeepEqual(gotBeforeGit, wantBeforeTime) {
		t.Fatalf("IterateBefore HEAD mismatch:\ngot  %v\nwant %v", gotBeforeGit, wantBeforeTime)
	}

	// 3. Test empty commit string
	for _, err := range reader.IterateBefore(ctx, "") {
		if err == nil {
			t.Fatal("expected error for empty beforeCommit")
		}
	}
}

func TestReader_Iterate_ContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	opsDir := filepath.Join(tempDir, ".memory", "ops")

	writer := NewWriter(opsDir)
	reader := NewReader(opsDir)

	b := NewBatch([]Op{
		{Type: OpAdd, EntryID: "inv-01", DossierScope: "s1", Fields: map[string]any{"text": "op1"}},
	})
	if err := writer.Write(b); err != nil {
		t.Fatalf("write batch: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	for _, err := range reader.Iterate(ctx) {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled error, got %v", err)
		}
	}
}

func TestMigrateV1Events(t *testing.T) {
	tempDir := t.TempDir()
	eventsDir := filepath.Join(tempDir, ".memory", "events")
	archiveDir := filepath.Join(tempDir, ".memory", "archive", "v1-events")

	// 1. Test when eventsDir does not exist -> no-op
	if err := MigrateV1Events(eventsDir, archiveDir); err != nil {
		t.Fatalf("expected nil for non-existent eventsDir, got %v", err)
	}

	// 2. Create mock events structure
	if err := os.MkdirAll(filepath.Join(eventsDir, "reviews"), 0o755); err != nil {
		t.Fatalf("mkdir events: %v", err)
	}
	reviewFile := filepath.Join(eventsDir, "reviews", "rev1.json")
	if err := os.WriteFile(reviewFile, []byte(`{"id":"rev1"}`), 0o644); err != nil {
		t.Fatalf("write review file: %v", err)
	}

	// Migrate to archiveDir
	if err := MigrateV1Events(eventsDir, archiveDir); err != nil {
		t.Fatalf("migrate events: %v", err)
	}

	// Verify eventsDir is gone
	if _, err := os.Stat(eventsDir); !os.IsNotExist(err) {
		t.Fatalf("expected eventsDir to be moved, but it still exists")
	}

	// Verify archiveDir contains the file
	archivedFile := filepath.Join(archiveDir, "reviews", "rev1.json")
	data, err := os.ReadFile(archivedFile)
	if err != nil {
		t.Fatalf("read archived file %s: %v", archivedFile, err)
	}
	if string(data) != `{"id":"rev1"}` {
		t.Fatalf("archived file content mismatch: got %s", string(data))
	}

	// 3. Test migrating when archiveDir already exists (merge case)
	// Create another eventsDir
	if err := os.MkdirAll(filepath.Join(eventsDir, "answers"), 0o755); err != nil {
		t.Fatalf("mkdir events answers: %v", err)
	}
	answerFile := filepath.Join(eventsDir, "answers", "ans1.json")
	if err := os.WriteFile(answerFile, []byte(`{"id":"ans1"}`), 0o644); err != nil {
		t.Fatalf("write answer file: %v", err)
	}

	if err := MigrateV1Events(eventsDir, archiveDir); err != nil {
		t.Fatalf("migrate into existing archiveDir: %v", err)
	}

	// Verify both reviews and answers exist in archiveDir
	if _, err := os.Stat(archivedFile); err != nil {
		t.Fatalf("existing archived reviews file should still exist: %v", err)
	}
	archivedAns := filepath.Join(archiveDir, "answers", "ans1.json")
	if _, err := os.Stat(archivedAns); err != nil {
		t.Fatalf("newly archived answers file should exist: %v", err)
	}
}

func TestHelpers(t *testing.T) {
	now := time.Now().UTC()
	eid := NewEntryIDWithTime("rul", now)
	if !containsSubstr(eid, "rul-") {
		t.Fatalf("expected prefix rul- in %s", eid)
	}

	batch := NewBatch([]Op{
		{Type: OpAdd, EntryID: eid, DossierScope: "s1", Fields: map[string]any{"text": "val"}},
	})
	if batch.Version != 1 {
		t.Errorf("expected version 1, got %d", batch.Version)
	}
	if batch.BatchID == "" {
		t.Errorf("expected non-empty batch ID")
	}
	if batch.CreatedAt.IsZero() {
		t.Errorf("expected non-zero created at")
	}
}

func TestReader_ReadBatch_Errors(t *testing.T) {
	tempDir := t.TempDir()
	r := NewReader(tempDir)

	// Non-existent file
	if _, err := r.ReadBatch("missing.json"); err == nil {
		t.Fatal("expected error reading missing file")
	}

	// Corrupt JSON
	badFile := filepath.Join(tempDir, "bad.json")
	if err := os.WriteFile(badFile, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadBatch(badFile); err == nil {
		t.Fatal("expected error reading corrupt json")
	}

	// Invalid batch (version 0)
	invalidBatchFile := filepath.Join(tempDir, "invalid.json")
	if err := os.WriteFile(invalidBatchFile, []byte(`{"version":0,"batch_id":"b1","created_at":"2026-10-06T12:00:00Z","ops":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadBatch(invalidBatchFile); err == nil {
		t.Fatal("expected error reading invalid batch")
	}
}

func TestWriter_Write_Errors(t *testing.T) {
	tempDir := t.TempDir()
	w := NewWriter(tempDir)

	// Invalid batch
	invalidBatch := OpBatch{Version: 0}
	if err := w.Write(invalidBatch); err == nil {
		t.Fatal("expected error writing invalid batch")
	}
}

func TestReader_Iterate_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	r := NewReader(tempDir)

	// Base dir contains non-month directory and non-json file
	if err := os.MkdirAll(filepath.Join(tempDir, "not-a-month"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "random.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	monthDir := filepath.Join(tempDir, "2026-10")
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Add a non-json file in month dir
	if err := os.WriteFile(filepath.Join(monthDir, "ignored.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write 1 valid batch
	w := NewWriter(tempDir)
	b := NewBatch([]Op{
		{Type: OpAdd, EntryID: "inv-01", DossierScope: "s1", Fields: map[string]any{"text": "val"}},
	})
	b.CreatedAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := w.Write(b); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var count int
	for batch, err := range r.Iterate(ctx) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if batch.BatchID != b.BatchID {
			t.Errorf("expected batch ID %s, got %s", b.BatchID, batch.BatchID)
		}
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 batch, got %d", count)
	}

	// Non-existent directory returns 0 items without error
	rMissing := NewReader(filepath.Join(tempDir, "does-not-exist"))
	var missingCount int
	for _, err := range rMissing.Iterate(ctx) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		missingCount++
	}
	if missingCount != 0 {
		t.Errorf("expected 0 batches for missing dir, got %d", missingCount)
	}
}

func TestMigrateV1Events_NotDir(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "events-file")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateV1Events(filePath, filepath.Join(tempDir, "archive")); err == nil {
		t.Fatal("expected error when events path is not a directory")
	}
}

func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
