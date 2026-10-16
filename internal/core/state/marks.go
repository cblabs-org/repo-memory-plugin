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
	markMutex     sync.Mutex
	safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
)

// Mark tracks the high-water distillation mark for a single conversation.
type Mark struct {
	ConversationID      string    `json:"conversation_id"`
	StepIndex           int       `json:"step_index"`
	ByteOffset          int64     `json:"byte_offset"`
	StepTimestamp       string    `json:"step_timestamp,omitempty"`
	LastNoteAt          time.Time `json:"last_note_at,omitempty"`
	LastFallbackAt      time.Time `json:"last_fallback_at,omitempty"`
	NoteTitles          []string  `json:"note_titles,omitempty"`
	NoteCount           int       `json:"note_count"`
	NotesToday          int       `json:"notes_today,omitempty"`
	PendingRequestStep  int       `json:"pending_request_step,omitempty"`
	PendingContinuation bool      `json:"pending_continuation,omitempty"`
	UpdatedAt           time.Time `json:"updated_at"`
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
	if convID == "" {
		return &Mark{ConversationID: convID, NoteTitles: []string{}}, nil
	}

	markMutex.Lock()
	defer markMutex.Unlock()

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

type noteTimestampHeader struct {
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// CountTurnNotesToday counts how many turn-note inbox notes were created on now.UTC()'s calendar date.
func CountTurnNotesToday(dataDir, repoID string, now time.Time) (int, error) {
	return countNotesTodayFiltered(dataDir, repoID, now, "turn-note")
}

// CountNotesToday counts how many inbox notes (any source) were created on now.UTC()'s calendar date.
func CountNotesToday(dataDir, repoID string, now time.Time) (int, error) {
	return countNotesTodayFiltered(dataDir, repoID, now, "")
}

func countNotesTodayFiltered(dataDir, repoID string, now time.Time, sourceFilter string) (int, error) {
	notesDir := filepath.Join(dataDir, repoID, "evidence", "notes")
	entries, err := os.ReadDir(notesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading notes directory: %w", err)
	}

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
