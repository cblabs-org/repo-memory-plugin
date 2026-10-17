/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/ops/op.go
 * Description: Defines knowledge operation types, provenance source constants,
 *              Op and OpBatch data structures, and ULID-based identifier and
 *              batch constructors.
 *
 * Exported Types / Functions:
 * - OpType:               Enumerates supported knowledge log operation types.
 * - OpAdd:                Adds a new knowledge entry to a dossier scope.
 * - OpAmend:              Amends fields on an existing knowledge entry.
 * - OpSupersede:          Replaces an existing entry with a new entry ID.
 * - OpRetire:             Retires an obsolete or invalid entry with a reason.
 * - OpMove:               Moves an entry to another dossier scope.
 * - OpSplit:              Splits a dossier scope across multiple target scopes.
 * - SourceTurnNote:       Provenance source for turn-level notes.
 * - SourceSidecar:        Provenance source for sidecar distillation.
 * - SourceSeed:           Provenance source for general seeding.
 * - SourceDocSeed:        Provenance source for documentation seeding.
 * - SourcePA:             Provenance source for Principal Architect history migration.
 * - SourceHuman:          Provenance source for direct human authoring.
 * - SourceConsolidation:  Provenance source for background note consolidation.
 * - SourceTick:           Provenance source for background maintenance ticks.
 * - SourceGardening:      Provenance source for dossier gardening passes.
 * - Op:                   Single knowledge operation targeting an entry ID or dossier scope.
 * - OpBatch:              Append-only batch of operations persisted as a single JSON file.
 * - NewBatchID():         Generates a new ULID batch identifier at the current time.
 * - NewBatchIDWithTime(): Generates a new ULID batch identifier at a specified timestamp.
 * - NewEntryID():         Generates a prefixed ULID entry identifier at the current time.
 * - NewEntryIDWithTime(): Generates a prefixed ULID entry identifier at a specified timestamp.
 * - NewBatch():           Constructs a v1 OpBatch timestamped at the current UTC time.
 * - NewBatchWithTime():   Constructs a v1 OpBatch timestamped at a specified time.
 */

package ops

import (
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// OpType represents the type of knowledge operation.
type OpType string

const (
	// OpAdd creates a new dossier entry.
	OpAdd OpType = "add"
	// OpAmend updates fields on an existing dossier entry in place.
	OpAmend OpType = "amend"
	// OpSupersede replaces an existing dossier entry with a newly created entry ID.
	OpSupersede OpType = "supersede"
	// OpRetire marks an existing dossier entry as retired with a rationale.
	OpRetire OpType = "retire"
	// OpMove reassigns an entry from its current dossier scope to a new target scope.
	OpMove OpType = "move"
	// OpSplit partitions entries from an overgrown dossier scope into new child scopes.
	OpSplit OpType = "split"
)

// Provenance source constants.
const (
	// SourceTurnNote indicates an operation captured from an explicit mid-turn note.
	SourceTurnNote = "turn-note"
	// SourceSidecar indicates an operation distilled by the background sidecar.
	SourceSidecar = "sidecar"
	// SourceSeed indicates an operation created during initial repository seeding.
	SourceSeed = "seed"
	// SourceDocSeed indicates an operation seeded from repository documentation.
	SourceDocSeed = "doc-seed"
	// SourcePA indicates an operation migrated from Principal Architect history.
	SourcePA = "pa"
	// SourceHuman indicates an operation authored directly by a human user.
	SourceHuman = "human"
	// SourceConsolidation indicates an operation produced by note consolidation.
	SourceConsolidation = "consolidation"
	// SourceTick indicates an operation produced by a periodic maintenance tick.
	SourceTick = "tick"
	// SourceGardening indicates an operation produced by dossier gardening.
	SourceGardening = "gardening"
)

// Op represents a single knowledge operation targeting an entry by ID or dossier.
type Op struct {
	Type         OpType         `json:"type"`                    // Operation kind (add, amend, supersede, retire, move, split).
	EntryID      string         `json:"entry_id,omitempty"`      // Target entry identifier (required for all ops except split).
	DossierScope string         `json:"dossier_scope,omitempty"` // Target module dossier scope (required for add and split).
	Timestamp    time.Time      `json:"timestamp"`               // UTC timestamp when the individual operation was recorded.
	Source       string         `json:"source"`                  // Provenance source identifier (e.g. SourceConsolidation).
	SessionID    string         `json:"session_id,omitempty"`    // Optional agent conversation or session identifier.
	Fields       map[string]any `json:"fields,omitempty"`        // Operation-specific payload fields (e.g. text, anchors, reason).
}

// OpBatch represents an append-only batch of ops stored as a single file.
type OpBatch struct {
	Version   int       `json:"version"`    // Schema version of the batch file (>= 1).
	Ops       []Op      `json:"ops"`        // Ordered list of operations contained in the batch.
	CreatedAt time.Time `json:"created_at"` // UTC creation timestamp of the batch.
	BatchID   string    `json:"batch_id"`   // Lexicographically sortable ULID batch identifier.
}

// NewBatchID generates a new ULID string for a batch.
func NewBatchID() string {
	return ulid.Make().String()
}

// NewBatchIDWithTime generates a new ULID string for a batch using the specified timestamp.
func NewBatchIDWithTime(t time.Time) string {
	return ulid.MustNew(ulid.Timestamp(t), ulid.DefaultEntropy()).String()
}

// NewEntryID creates a new entry ID with the given prefix and a ULID.
func NewEntryID(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix != "" && !strings.HasSuffix(prefix, "-") {
		prefix += "-"
	}
	return prefix + ulid.Make().String()
}

// NewEntryIDWithTime creates a new entry ID with the given prefix and a ULID at time t.
func NewEntryIDWithTime(prefix string, t time.Time) string {
	prefix = strings.TrimSpace(prefix)
	if prefix != "" && !strings.HasSuffix(prefix, "-") {
		prefix += "-"
	}
	return prefix + ulid.MustNew(ulid.Timestamp(t), ulid.DefaultEntropy()).String()
}

// NewBatch creates an OpBatch initialized with version 1, current UTC time, and a generated batch ID.
func NewBatch(ops []Op) OpBatch {
	now := time.Now().UTC()
	return OpBatch{
		Version:   1,
		Ops:       ops,
		CreatedAt: now,
		BatchID:   NewBatchIDWithTime(now),
	}
}

// NewBatchWithTime creates an OpBatch using the given timestamp for CreatedAt and BatchID.
// Use this for back-dated batches (e.g. seeding from historical data) so that
// IterateBefore/IterateBeforeTime filters work correctly.
// Falls back to time.Now() if t is zero (e.g. uncommitted workorder files).
func NewBatchWithTime(batchOps []Op, t time.Time) OpBatch {
	if t.IsZero() {
		t = time.Now()
	}
	ts := t.UTC()
	return OpBatch{
		Version:   1,
		Ops:       batchOps,
		CreatedAt: ts,
		BatchID:   NewBatchIDWithTime(ts),
	}
}
