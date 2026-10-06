package ops

import (
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// OpType represents the type of knowledge operation.
type OpType string

const (
	OpAdd       OpType = "add"
	OpAmend     OpType = "amend"
	OpSupersede OpType = "supersede"
	OpRetire    OpType = "retire"
	OpMove      OpType = "move"
	OpSplit     OpType = "split"
)

// Provenance source constants.
const (
	SourceTurnNote      = "turn-note"
	SourceSidecar       = "sidecar"
	SourceSeed          = "seed"
	SourcePA            = "pa"
	SourceHuman         = "human"
	SourceConsolidation = "consolidation"
)

// Op represents a single knowledge operation targeting an entry by ID or dossier.
type Op struct {
	Type         OpType         `json:"type"`
	EntryID      string         `json:"entry_id,omitempty"`
	DossierScope string         `json:"dossier_scope,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
	Source       string         `json:"source"`
	SessionID    string         `json:"session_id,omitempty"`
	Fields       map[string]any `json:"fields,omitempty"`
}

// OpBatch represents an append-only batch of ops stored as a single file.
type OpBatch struct {
	Version   int       `json:"version"`
	Ops       []Op      `json:"ops"`
	CreatedAt time.Time `json:"created_at"`
	BatchID   string    `json:"batch_id"`
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
