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
	SourceTurnNote   NoteSource = "turn-note"
	SourceMCPNote    NoteSource = "mcp-note"
	SourceCheckpoint NoteSource = "checkpoint"
	SourceArtifact   NoteSource = "artifact"
	SourceSidecar    NoteSource = "sidecar"
	SourcePA         NoteSource = "pa"
	SourceDocSeed    NoteSource = "doc-seed"
)

// AnchorCheck records whether an anchor path existed in the working tree at harvest time.
type AnchorCheck struct {
	Anchor     string    `json:"anchor"`
	Exists     bool      `json:"exists"`
	VerifiedAt time.Time `json:"verified_at"`
}

// NoteItem is a single candidate knowledge item inside an inbox Note.
type NoteItem struct {
	Type          dossiers.EntryType `json:"type" yaml:"type"`
	Title         string             `json:"title" yaml:"title"`
	Why           string             `json:"why,omitempty" yaml:"why,omitempty"`
	Do            string             `json:"do,omitempty" yaml:"do,omitempty"`
	Anchors       []string           `json:"anchors,omitempty" yaml:"anchors,omitempty"`
	AnchorChecks  []AnchorCheck      `json:"anchor_checks,omitempty" yaml:"anchor_checks,omitempty"`
	Evidence      string             `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	ClosesWhen    string             `json:"closes_when,omitempty" yaml:"closes_when,omitempty"`
	ErrorPattern  string             `json:"error_pattern,omitempty" yaml:"error_pattern,omitempty"`
	Commands      []string           `json:"commands,omitempty" yaml:"commands,omitempty"`
	StepCitations []int              `json:"step_citations,omitempty" yaml:"step_citations,omitempty"`
	Verified      string             `json:"verified,omitempty" yaml:"verified,omitempty"`
	Scope         string             `json:"scope,omitempty" yaml:"scope,omitempty"`
}

// Note represents a Stage 1 distilled note stored in <data-dir>/<repo-id>/evidence/notes/<id>.json.
type Note struct {
	ID             string                    `json:"id"`
	Source         NoteSource                `json:"source"`
	ConversationID string                    `json:"conversation_id,omitempty"`
	StepsCovered   [2]int                    `json:"steps_covered,omitempty"`
	Model          string                    `json:"model,omitempty"`
	HarnessVersion string                    `json:"harness_version,omitempty"`
	Branch         string                    `json:"branch,omitempty"`
	HeadCommit     string                    `json:"head_commit,omitempty"`
	ExposureList   []delivery.ExposureRecord `json:"exposure_list,omitempty"`
	AnchorChecks   []AnchorCheck             `json:"anchor_checks,omitempty"`
	Items          []NoteItem                `json:"items"`
	CreatedAt      time.Time                 `json:"created_at"`
	Consolidated   bool                      `json:"consolidated"`
	DedupKey       string                    `json:"dedup_key,omitempty"`
}

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
	if len(anchors) == 0 {
		return nil
	}
	now := time.Now().UTC()
	checks := make([]AnchorCheck, 0, len(anchors))
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
	dir string
	mu  sync.Mutex
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
	if note == nil {
		return "", errors.New("note cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(note.ID) == "" {
		note.ID = NewNoteID()
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = time.Now().UTC()
	}

	if err := s.writeNoteLocked(note); err != nil {
		return "", err
	}
	return note.ID, nil
}

func (s *Store) writeNoteLocked(note *Note) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("creating inbox notes directory: %w", err)
	}

	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling inbox note: %w", err)
	}

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

func (s *Store) listAllLocked() ([]*Note, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Note{}, nil
		}
		return nil, fmt.Errorf("reading inbox notes directory: %w", err)
	}

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
