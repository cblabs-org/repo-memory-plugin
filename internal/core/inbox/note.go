/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/inbox/note.go
 * Description: Defines Stage 1 inbox note schemas, anchor existence verification,
 *              and atomic JSON persistence for harvested candidate notes.
 *
 * Exported Types / Functions:
 * - NoteSource:         Identifies which Stage 1 path produced an inbox note.
 * - SourceTurnNote:     End-of-turn distillation source ("turn-note").
 * - SourceMCPNote:      Explicit MCP memory_note tool source ("mcp-note").
 * - SourceCheckpoint:   Episode checkpoint harvest source ("checkpoint").
 * - SourceArtifact:     Conversation markdown artifact harvest source ("artifact").
 * - SourceSidecar:      Sidecar distillation source ("sidecar").
 * - SourcePA:           Principal Architect migration source ("pa").
 * - SourceDocSeed:      Day-1 documentation seeding source ("doc-seed").
 * - AnchorCheck:        Records whether an anchor path existed in the working tree at harvest time.
 * - NoteItem:           Single candidate knowledge item inside an inbox Note.
 * - Note:               Stage 1 distilled note stored in evidence/notes/<id>.json.
 * - Store:              Manages inbox note JSON files under <dataDir>/<repoID>/evidence/notes/.
 *   - NewStore():           Creates a new inbox Note Store for a repository.
 *   - Dir():                Returns the directory where inbox notes are stored.
 *   - Write():              Persists a Note atomically to the inbox store.
 *   - Read():               Loads a single Note by ID from the store.
 *   - ListAll():            Returns all notes sorted by CreatedAt ascending (then ID).
 *   - ListUnconsolidated(): Returns all notes where Consolidated is false.
 *   - MarkConsolidated():   Sets Consolidated = true on a note and rewrites it atomically.
 *   - HasDedupKey():        Reports whether any note in the store has the given non-empty DedupKey.
 * - NewNoteID():        Returns a new ULID-based note identifier ("note-<ulid>").
 * - CheckAnchors():     Verifies whether each anchor path exists inside repoRoot.
 */

// Package inbox manages Stage 1 candidate knowledge notes, anchor verification,
// and zero-token checkpoint/artifact harvesting prior to Stage 2 consolidation.
package inbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/oklog/ulid/v2"
)

// NoteSource identifies which Stage 1 path produced an inbox note.
type NoteSource string

const (
	// SourceTurnNote indicates a note distilled at the end of a milestone conversation turn.
	SourceTurnNote NoteSource = "turn-note"
	// SourceMCPNote indicates a note recorded explicitly via the memory_note MCP tool.
	SourceMCPNote NoteSource = "mcp-note"
	// SourceCheckpoint indicates a note harvested deterministically from episode checkpoints.
	SourceCheckpoint NoteSource = "checkpoint"
	// SourceArtifact indicates a note harvested deterministically from conversation markdown artifacts.
	SourceArtifact NoteSource = "artifact"
	// SourceSidecar indicates a note produced by the background LLM sidecar distiller.
	SourceSidecar NoteSource = "sidecar"
	// SourcePA indicates a note imported from a Principal Architect memory bank.
	SourcePA NoteSource = "pa"
	// SourceDocSeed indicates a note extracted during Day-1 repository documentation seeding.
	SourceDocSeed NoteSource = "doc-seed"
)

// AnchorCheck records whether an anchor path existed in the working tree at harvest time.
type AnchorCheck struct {
	Anchor     string    `json:"anchor"`      // Raw anchor string inspected against the repository root.
	Exists     bool      `json:"exists"`      // True if the resolved repository-relative file or directory existed on disk.
	VerifiedAt time.Time `json:"verified_at"` // UTC timestamp when the filesystem check was performed.
}

// NoteItem is a single candidate knowledge item inside an inbox Note.
type NoteItem struct {
	Type          dossiers.EntryType `json:"type" yaml:"type"`                                         // Cognitive entry category (invariant, gotcha, ruled-out, etc.).
	Title         string             `json:"title" yaml:"title"`                                       // Concise one-line summary of the candidate item.
	Why           string             `json:"why,omitempty" yaml:"why,omitempty"`                       // Causal rationale or root cause behind the item.
	Do            string             `json:"do,omitempty" yaml:"do,omitempty"`                         // Actionable imperative instruction or remediation step.
	Anchors       []string           `json:"anchors,omitempty" yaml:"anchors,omitempty"`               // Repository-relative file, directory, or symbol anchors.
	AnchorChecks  []AnchorCheck      `json:"anchor_checks,omitempty" yaml:"anchor_checks,omitempty"`   // Filesystem existence verification results for Anchors.
	Evidence      string             `json:"evidence,omitempty" yaml:"evidence,omitempty"`             // Provenance reference (e.g. step range, artifact path, or doc path).
	ClosesWhen    string             `json:"closes_when,omitempty" yaml:"closes_when,omitempty"`       // Deterministic auto-closure predicate for open-issue entries.
	ErrorPattern  string             `json:"error_pattern,omitempty" yaml:"error_pattern,omitempty"`   // Diagnostic substring or regex associated with a failure signature.
	Commands      []string           `json:"commands,omitempty" yaml:"commands,omitempty"`             // Shell commands associated with procedural recipes.
	StepCitations []int              `json:"step_citations,omitempty" yaml:"step_citations,omitempty"` // Episode step indices cited as evidence for this item.
	Verified      string             `json:"verified,omitempty" yaml:"verified,omitempty"`             // Verification stamp (date, commit, and verification method).
	Scope         string             `json:"scope,omitempty" yaml:"scope,omitempty"`                   // Optional target dossier scope or tier-prefixed scope override.
}

// Note represents a Stage 1 distilled note stored in <data-dir>/<repo-id>/evidence/notes/<id>.json.
type Note struct {
	ID             string                    `json:"id"`                        // Unique ULID-backed note identifier ("note-<ulid>").
	Source         NoteSource                `json:"source"`                    // Stage 1 capture pathway that produced the note.
	ConversationID string                    `json:"conversation_id,omitempty"` // Originating agent conversation or session identifier.
	StepsCovered   [2]int                    `json:"steps_covered,omitempty"`   // Inclusive [start, end] episode step range distilled into this note.
	Model          string                    `json:"model,omitempty"`           // Model identifier active during the originating session.
	HarnessVersion string                    `json:"harness_version,omitempty"` // Host harness version string recorded on the episode.
	Branch         string                    `json:"branch,omitempty"`          // Git branch active when the note was captured.
	HeadCommit     string                    `json:"head_commit,omitempty"`     // Git HEAD commit SHA active when the note was captured.
	ExposureList   []delivery.ExposureRecord `json:"exposure_list,omitempty"`   // Memory entries delivered to the session prior to capture (for echo filtering).
	AnchorChecks   []AnchorCheck             `json:"anchor_checks,omitempty"`   // Note-level filesystem anchor verification records.
	Items          []NoteItem                `json:"items"`                     // Candidate knowledge items distilled in this note.
	CreatedAt      time.Time                 `json:"created_at"`                // UTC timestamp when the note was written to the inbox.
	Consolidated   bool                      `json:"consolidated"`              // True once Stage 2 consolidation has processed this note.
	DedupKey       string                    `json:"dedup_key,omitempty"`       // Optional idempotency key preventing duplicate harvest across ticks.
}

// ulidMu serializes ULID generation in NewNoteID for monotonic entropy safety.
var ulidMu sync.Mutex

// NewNoteID returns a new ULID-based note identifier ("note-<ulid>").
func NewNoteID() string {
	ulidMu.Lock()
	defer ulidMu.Unlock()
	return "note-" + ulid.Make().String()
}

// CheckAnchors verifies whether each anchor path (after stripping optional #symbol or :line suffixes)
// exists inside repoRoot.
func CheckAnchors(repoRoot string, anchors []string) []AnchorCheck {
	// --- Step 1: Return early when no anchors are provided ---
	if len(anchors) == 0 {
		return nil
	}
	now := time.Now().UTC()
	checks := make([]AnchorCheck, 0, len(anchors))

	// --- Step 2: Strip symbol/line suffixes and normalize each anchor path ---
	for _, rawAnchor := range anchors {
		trimmed := strings.TrimSpace(rawAnchor)
		if trimmed == "" {
			continue
		}
		pathPart := trimmed
		if idx := strings.IndexByte(pathPart, '#'); idx >= 0 {
			pathPart = pathPart[:idx]
		}
		if idx := strings.IndexByte(pathPart, ':'); idx >= 0 {
			pathPart = pathPart[:idx]
		}
		pathPart = strings.TrimSpace(pathPart)
		normalized := strings.ReplaceAll(pathPart, "\\", "/")
		normalized = strings.TrimPrefix(normalized, "./")

		// --- Step 3: Stat the cleaned repository-relative path while rejecting path traversal ---
		exists := false
		if repoRoot != "" && normalized != "" && !strings.HasPrefix(normalized, "/") {
			cleanRel := filepath.Clean(filepath.FromSlash(normalized))
			if cleanRel != "." && cleanRel != ".." && !strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) && !filepath.IsAbs(cleanRel) {
				fullPath := filepath.Join(repoRoot, cleanRel)
				if _, err := os.Stat(fullPath); err == nil {
					exists = true
				}
			}
		}

		checks = append(checks, AnchorCheck{
			Anchor:     trimmed,
			Exists:     exists,
			VerifiedAt: now,
		})
	}
	return checks
}

// Store manages inbox note JSON files under <dataDir>/<repoID>/evidence/notes/.
type Store struct {
	dir string     // Filesystem directory path holding <note-id>.json files.
	mu  sync.Mutex // Mutex serializing concurrent reads and atomic note writes within a process.
}

// NewStore creates a new inbox Note Store for a repository.
func NewStore(dataDir, repoID string) *Store {
	return &Store{
		dir: filepath.Join(dataDir, repoID, "evidence", "notes"),
	}
}

// Dir returns the directory where inbox notes are stored.
func (s *Store) Dir() string {
	return s.dir
}

// Write persists a Note atomically to the inbox store.
func (s *Store) Write(note *Note) (string, error) {
	// --- Step 1: Validate input and acquire the store lock ---
	if note == nil {
		return "", errors.New("note cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// --- Step 2: Populate default ULID and UTC creation timestamp if unset ---
	if strings.TrimSpace(note.ID) == "" {
		note.ID = NewNoteID()
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now().UTC()
	}

	// --- Step 3: Atomically write the JSON file to disk ---
	if err := s.writeNoteLocked(note); err != nil {
		return "", err
	}
	return note.ID, nil
}

// writeNoteLocked marshals note as indented JSON and writes it atomically via a temporary file rename.
func (s *Store) writeNoteLocked(note *Note) error {
	// --- Step 1: Ensure the inbox notes directory exists and marshal the note payload ---
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("creating inbox notes directory: %w", err)
	}

	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling inbox note: %w", err)
	}

	// --- Step 2: Write the marshaled payload to a temporary file in the same directory ---
	tmp, err := os.CreateTemp(s.dir, ".note-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for inbox note: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing inbox note temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing inbox note temp file: %w", err)
	}

	// --- Step 3: Atomically rename the temporary file to <note.ID>.json ---
	targetPath := filepath.Join(s.dir, note.ID+".json")
	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming inbox note file: %w", err)
	}
	return nil
}

// Read loads a single Note by ID from the store.
func (s *Store) Read(id string) (*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readNoteLocked(id)
}

// readNoteLocked reads and unmarshals a single Note JSON file while holding s.mu.
func (s *Store) readNoteLocked(id string) (*Note, error) {
	cleanID := strings.TrimSuffix(filepath.Base(id), ".json")
	path := filepath.Join(s.dir, cleanID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading inbox note %s: %w", cleanID, err)
	}
	var n Note
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("unmarshaling inbox note %s: %w", cleanID, err)
	}
	return &n, nil
}

// ListAll returns all notes in the inbox store sorted by CreatedAt ascending (then ID).
func (s *Store) ListAll() ([]*Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listAllLocked()
}

// listAllLocked scans s.dir for note JSON files and returns them sorted by CreatedAt ascending.
func (s *Store) listAllLocked() ([]*Note, error) {
	// --- Step 1: Read directory entries and handle missing directory gracefully ---
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Note{}, nil
		}
		return nil, fmt.Errorf("reading inbox notes directory: %w", err)
	}

	// --- Step 2: Parse all non-hidden .json note files ---
	notes := make([]*Note, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		var n Note
		if err := json.Unmarshal(data, &n); err != nil {
			continue
		}
		notes = append(notes, &n)
	}

	// --- Step 3: Sort notes deterministically by CreatedAt ascending, breaking ties by ID ---
	sort.Slice(notes, func(i, j int) bool {
		if !notes[i].CreatedAt.Equal(notes[j].CreatedAt) {
			return notes[i].CreatedAt.Before(notes[j].CreatedAt)
		}
		return notes[i].ID < notes[j].ID
	})

	return notes, nil
}

// ListUnconsolidated returns all notes where Consolidated is false, sorted by CreatedAt ascending.
func (s *Store) ListUnconsolidated() ([]*Note, error) {
	all, err := s.ListAll()
	if err != nil {
		return nil, err
	}
	unconsolidated := make([]*Note, 0, len(all))
	for _, n := range all {
		if !n.Consolidated {
			unconsolidated = append(unconsolidated, n)
		}
	}
	return unconsolidated, nil
}

// MarkConsolidated sets Consolidated = true on the specified note and rewrites it atomically.
func (s *Store) MarkConsolidated(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, err := s.readNoteLocked(id)
	if err != nil {
		return err
	}
	n.Consolidated = true
	return s.writeNoteLocked(n)
}

// HasDedupKey returns true if any note in the store has the given non-empty DedupKey.
func (s *Store) HasDedupKey(dedupKey string) (bool, error) {
	if dedupKey == "" {
		return false, nil
	}
	all, err := s.ListAll()
	if err != nil {
		return false, err
	}
	for _, n := range all {
		if n.DedupKey == dedupKey {
			return true, nil
		}
	}
	return false, nil
}
