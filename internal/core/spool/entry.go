package spool

import (
	"encoding/json"
	"time"
)

// EntryKind specifies the category of work requested via the spool.
type EntryKind string

const (
	// KindCapture indicates a finished agent turn / session requiring transcript capture.
	KindCapture EntryKind = "capture"
	// KindBrief indicates a request for task brief generation.
	KindBrief EntryKind = "brief"
	// KindDejavu indicates a tier-2 semantic déjà-vu check request.
	KindDejavu EntryKind = "dejavu"
	// KindRecheck indicates a re-check requested following dependency or git changes.
	KindRecheck EntryKind = "recheck"
)

// Entry represents a durable unit of work spooled by hooks for ingestion by the tick.
type Entry struct {
	ID             string         `json:"id,omitempty"`
	Kind           EntryKind      `json:"kind,omitempty"`
	Timestamp      time.Time      `json:"timestamp,omitempty"`
	Harness        string         `json:"harness,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	TranscriptPath string         `json:"transcript_path,omitempty"`
	RepoRoot       string         `json:"repo_root,omitempty"`
	Branch         string         `json:"branch,omitempty"`
	Head           string         `json:"head,omitempty"`
	StepIndex      int            `json:"step_index,omitempty"`
	EntityID       string         `json:"entity_id,omitempty"`
	Reasoning      string         `json:"reasoning,omitempty"`
	Command        string         `json:"command,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
}

// UnmarshalJSON unmarshals JSON and normalizes defaults (such as defaulting Kind to KindCapture).
func (e *Entry) UnmarshalJSON(data []byte) error {
	type Alias Entry
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if e.Kind == "" {
		if e.TranscriptPath != "" || e.ConversationID != "" {
			e.Kind = KindCapture
		}
	}
	return nil
}

// NewCaptureEntry creates a spool entry for a finished agent session turn.
func NewCaptureEntry(harness, conversationID, transcriptPath, repoRoot, branch, head string) Entry {
	return Entry{
		Kind:           KindCapture,
		Timestamp:      time.Now().UTC(),
		Harness:        harness,
		ConversationID: conversationID,
		TranscriptPath: transcriptPath,
		RepoRoot:       repoRoot,
		Branch:         branch,
		Head:           head,
	}
}

// NewBriefEntry creates a spool entry requesting task brief generation.
func NewBriefEntry(harness, conversationID, repoRoot string, stepIndex int, reasoning string) Entry {
	return Entry{
		Kind:           KindBrief,
		Timestamp:      time.Now().UTC(),
		Harness:        harness,
		ConversationID: conversationID,
		RepoRoot:       repoRoot,
		StepIndex:      stepIndex,
		Reasoning:      reasoning,
	}
}

// NewDejavuEntry creates a spool entry requesting a tier-2 semantic déjà-vu check.
func NewDejavuEntry(harness, conversationID, entityID, reasoning string, stepIndex int) Entry {
	return Entry{
		Kind:           KindDejavu,
		Timestamp:      time.Now().UTC(),
		Harness:        harness,
		ConversationID: conversationID,
		EntityID:       entityID,
		Reasoning:      reasoning,
		StepIndex:      stepIndex,
	}
}

// NewRecheckEntry creates a spool entry requesting a re-verification after a git operation or dependency update.
func NewRecheckEntry(harness, conversationID, repoRoot, branch, head, command string, stepIndex int) Entry {
	return Entry{
		Kind:           KindRecheck,
		Timestamp:      time.Now().UTC(),
		Harness:        harness,
		ConversationID: conversationID,
		RepoRoot:       repoRoot,
		Branch:         branch,
		Head:           head,
		Command:        command,
		StepIndex:      stepIndex,
	}
}
