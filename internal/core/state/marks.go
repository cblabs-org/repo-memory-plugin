/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/state/marks.go
 * Description: Persists and updates per-conversation high-water distillation marks,
 *              detects transcript rewinds, and counts daily inbox notes.
 *
 * Exported Types / Functions:
 * - Mark:                     Tracks the high-water distillation mark and note state for a conversation.
 *   - ClampIfRewound():       Resets watermarks when a conversation transcript rewinds.
 * - SanitizeConversationID(): Normalizes or hashes a conversation ID into a safe filename stem.
 * - MarkPath():               Returns the filesystem path for a conversation's mark JSON file.
 * - LoadMark():               Reads a conversation's Mark from disk or returns an initialized Mark.
 * - SaveMark():               Atomically writes a conversation's Mark to disk.
 * - CountTurnNotesToday():    Counts turn-note inbox entries created on the given UTC calendar date.
 * - CountNotesToday():        Counts all inbox notes created on the given UTC calendar date.
 */

// Package state manages per-conversation distillation watermarks and parent-child subagent registries.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	markMutex     sync.Mutex                                 // Serializes in-process reads and writes to conversation mark files.
	safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`) // Matches safe filename characters for unhashed conversation IDs.
)

// Mark tracks the high-water distillation mark for a single conversation.
type Mark struct {
	ConversationID      string    `json:"conversation_id"`                // Unique identifier of the tracked conversation.
	StepIndex           int       `json:"step_index"`                     // Highest transcript step index distilled so far.
	ByteOffset          int64     `json:"byte_offset"`                    // Byte offset into transcript.jsonl corresponding to StepIndex.
	StepTimestamp       string    `json:"step_timestamp,omitempty"`       // ISO-8601 timestamp of the step at StepIndex for rewind detection.
	LastNoteAt          time.Time `json:"last_note_at,omitempty"`         // Timestamp of the most recent memory_note call in this conversation.
	LastFallbackAt      time.Time `json:"last_fallback_at,omitempty"`     // Timestamp of the most recent fallback distillation pass.
	NoteTitles          []string  `json:"note_titles,omitempty"`          // Titles of notes captured during the conversation.
	NoteCount           int       `json:"note_count"`                     // Total notes captured in this conversation.
	NotesToday          int       `json:"notes_today,omitempty"`          // Cached count of notes captured on the current UTC day.
	PendingRequestStep  int       `json:"pending_request_step,omitempty"` // Step index where a distillation request was injected.
	PendingContinuation bool      `json:"pending_continuation,omitempty"` // True when awaiting the agent's follow-up turn after a prompt.
	UpdatedAt           time.Time `json:"updated_at"`                     // UTC timestamp when the mark file was last persisted.
}

// SanitizeConversationID ensures the conversation ID is safe for use in a filesystem path.
func SanitizeConversationID(id string) string {
	if id == "" {
		return ""
	}
	if safeIDPattern.MatchString(id) && len(id) <= 255 {
		return id
	}
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:16])
}

// MarkPath returns the filesystem path for a conversation's mark file:
// <dataDir>/<repoID>/state/marks/<safeConvID>.json.
func MarkPath(dataDir, repoID, convID string) string {
	safeID := SanitizeConversationID(convID)
	return filepath.Join(dataDir, repoID, "state", "marks", safeID+".json")
}

// LoadMark loads a conversation's mark from disk. If the file does not exist
// or convID is empty, it returns an initialized zero-value Mark.
func LoadMark(dataDir, repoID, convID string) (*Mark, error) {
	// --- Step 1: Return an empty initialized Mark when convID is blank ---
	if convID == "" {
		return &Mark{ConversationID: convID, NoteTitles: []string{}}, nil
	}

	markMutex.Lock()
	defer markMutex.Unlock()

	// --- Step 2: Read and unmarshal the mark file from disk ---
	path := MarkPath(dataDir, repoID, convID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Mark{ConversationID: convID, NoteTitles: []string{}}, nil
		}
		return nil, fmt.Errorf("reading mark file: %w", err)
	}

	var m Mark
	if err := json.Unmarshal(data, &m); err != nil {
		return &Mark{ConversationID: convID, NoteTitles: []string{}}, nil
	}
	if m.ConversationID == "" {
		m.ConversationID = convID
	}
	if m.NoteTitles == nil {
		m.NoteTitles = []string{}
	}
	return &m, nil
}

// SaveMark atomically persists a conversation's mark to disk.
func SaveMark(dataDir, repoID string, m *Mark) error {
	// --- Step 1: Validate mark and ensure target directory exists ---
	if m == nil || m.ConversationID == "" {
		return nil
	}

	markMutex.Lock()
	defer markMutex.Unlock()

	m.UpdatedAt = time.Now().UTC()
	path := MarkPath(dataDir, repoID, m.ConversationID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating marks directory: %w", err)
	}

	// --- Step 2: Marshal mark to JSON and write to a temporary file ---
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mark: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".mark-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for mark: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing mark temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing mark temp file: %w", err)
	}

	// --- Step 3: Atomically rename temporary file into place ---
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming mark file: %w", err)
	}

	return nil
}

// ClampIfRewound checks whether the conversation transcript has rewound relative
// to the mark (either step count shrank at or below m.StepIndex, or the step
// timestamp at m.StepIndex changed). If rewound, it resets the step/byte/timestamp
// watermarks and returns true. Note: checkpoints never move the mark.
func (m *Mark) ClampIfRewound(stepCount int, stepAtMarkTimestamp string) bool {
	// --- Step 1: Check whether any watermark is set and whether a rewind occurred ---
	if m == nil {
		return false
	}
	if m.StepIndex <= 0 && m.ByteOffset <= 0 && m.PendingRequestStep <= 0 && !m.PendingContinuation {
		return false
	}

	stepShrank := stepCount <= m.StepIndex
	timestampMismatch := m.StepTimestamp != "" && stepAtMarkTimestamp != "" && stepAtMarkTimestamp != m.StepTimestamp
	if !stepShrank && !timestampMismatch {
		return false
	}

	// --- Step 2: Reset transcript position and pending request watermarks ---
	if stepCount < 0 {
		stepCount = 0
	}
	m.StepIndex = 0
	m.ByteOffset = 0
	m.StepTimestamp = ""
	m.PendingRequestStep = 0
	m.PendingContinuation = false
	return true
}

// noteTimestampHeader captures the minimal JSON fields needed to filter inbox notes by source and date.
type noteTimestampHeader struct {
	Source    string    `json:"source"`     // Origin of the note (e.g. "turn-note").
	CreatedAt time.Time `json:"created_at"` // UTC creation timestamp of the note.
}

// CountTurnNotesToday counts how many turn-note inbox notes were created on now.UTC()'s calendar date.
func CountTurnNotesToday(dataDir, repoID string, now time.Time) (int, error) {
	return countNotesTodayFiltered(dataDir, repoID, now, "turn-note")
}

// CountNotesToday counts how many inbox notes (any source) were created on now.UTC()'s calendar date.
func CountNotesToday(dataDir, repoID string, now time.Time) (int, error) {
	return countNotesTodayFiltered(dataDir, repoID, now, "")
}

// countNotesTodayFiltered counts JSON note files under <dataDir>/<repoID>/evidence/notes created on now's UTC date.
func countNotesTodayFiltered(dataDir, repoID string, now time.Time, sourceFilter string) (int, error) {
	// --- Step 1: Read the evidence/notes directory ---
	notesDir := filepath.Join(dataDir, repoID, "evidence", "notes")
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading notes directory: %w", err)
	}

	// --- Step 2: Parse note headers and count entries matching targetDay and sourceFilter ---
	targetDay := now.UTC().Format("2006-01-02")
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(notesDir, name))
		if err != nil {
			continue
		}
		var hdr noteTimestampHeader
		if err := json.Unmarshal(data, &hdr); err != nil {
			continue
		}
		if sourceFilter != "" && hdr.Source != sourceFilter {
			continue
		}
		if !hdr.CreatedAt.IsZero() && hdr.CreatedAt.UTC().Format("2006-01-02") == targetDay {
			count++
		}
	}
	return count, nil
}
