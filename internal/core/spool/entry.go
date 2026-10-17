/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/spool/entry.go
 * Description: Defines the Entry payload schema and EntryKind constants written
 *              by fast-path hooks for asynchronous background tick ingestion.
 *
 * Exported Types / Functions:
 * - EntryKind:   Category of work requested via the spool.
 * - KindCapture: Indicates a finished agent turn requiring transcript capture.
 * - KindBrief:   Indicates a request for task brief generation.
 * - KindDejavu:  Indicates a tier-2 semantic deja-vu check request.
 * - KindRecheck: Indicates a re-check requested following git or dependency changes.
 * - Entry:       Durable unit of work spooled by hooks for tick ingestion.
 *
 *   Lifecycle:
 *   - NewCaptureEntry(): Creates a spool entry for a finished agent session turn.
 *   - NewBriefEntry():   Creates a spool entry requesting task brief generation.
 *   - NewDejavuEntry():  Creates a spool entry requesting a tier-2 semantic deja-vu check.
 *   - NewRecheckEntry(): Creates a spool entry requesting re-verification after git changes.
 *
 *   json.Unmarshaler:
 *   - UnmarshalJSON():   Unmarshals JSON and defaults Kind to KindCapture when omitted.
 */

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
	ID                   string         `json:"id,omitempty"`                     // Unique spool entry identifier.
	Kind                 EntryKind      `json:"kind,omitempty"`                   // Category of background work requested.
	Timestamp            time.Time      `json:"timestamp,omitempty"`              // UTC creation timestamp of the spool event.
	Harness              string         `json:"harness,omitempty"`                // Originating agent harness identifier.
	ConversationID       string         `json:"conversation_id,omitempty"`        // Active conversation or trajectory UUID.
	ParentConversationID string         `json:"parent_conversation_id,omitempty"` // Parent conversation UUID for subagent sessions.
	SessionID            string         `json:"session_id,omitempty"`             // Optional harness session identifier.
	TranscriptPath       string         `json:"transcript_path,omitempty"`        // Filesystem path to the queued or raw transcript.
	RepoRoot             string         `json:"repo_root,omitempty"`              // Repository root directory path.
	Branch               string         `json:"branch,omitempty"`                 // Active git branch name at spool time.
	Head                 string         `json:"head,omitempty"`                   // Active git HEAD commit hash at spool time.
	StepIndex            int            `json:"step_index,omitempty"`             // Transcript step index associated with the event.
	EntityID             string         `json:"entity_id,omitempty"`              // Target entity identifier for deja-vu checks.
	Reasoning            string         `json:"reasoning,omitempty"`              // Agent reasoning excerpt attached to the request.
	Command              string         `json:"command,omitempty"`                // Executed shell or git command triggering the event.
	Details              map[string]any `json:"details,omitempty"`                // Additional unstructured event metadata.
}

// UnmarshalJSON unmarshals JSON and normalizes defaults (such as defaulting Kind to KindCapture).
func (e *Entry) UnmarshalJSON(data []byte) error {
	// --- Step 1: Unmarshal raw JSON fields via alias type ---
	type Alias Entry
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// --- Step 2: Backfill default KindCapture when transcript or conversation is set ---
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
