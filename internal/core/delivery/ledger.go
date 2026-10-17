/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/delivery/ledger.go
 * Description: Tracks per-conversation memory injections, MCP pull exposures,
 *              visited dossier scopes and file anchors, and pre-edit gate
 *              acknowledgements across hook invocations.
 *
 * Exported Types / Functions:
 * - InjectionRecord: Tracks a single push-delivered entry and its transcript step.
 * - ExposureRecord:  Tracks an entry exposed to the agent via push or pull.
 * - Ledger:          Per-conversation state ledger for deduplication, gates, and checkpoints.
 *   - NewLedger():               Creates an initialized Ledger for a conversation ID.
 *   - RecordInjection():         Appends a push-delivery record and updates exposure/dedup state.
 *   - RecordPull():              Appends an exposure record for an explicit MCP pull lookup.
 *   - HasSeenDossier():          Reports whether a module dossier scope has been entered.
 *   - MarkDossierSeen():         Marks a module dossier scope as visited.
 *   - HasSeenAnchor():           Reports whether a file or symbol anchor has been touched.
 *   - MarkAnchorSeen():          Marks a file or symbol anchor as touched.
 *   - IsGateAcknowledged():      Reports whether an edit-gate entry has been acknowledged.
 *   - AcknowledgeGate():         Marks a single edit-gate entry as acknowledged.
 *   - SetPendingGates():         Records pending edit-gate entry IDs for a target file.
 *   - HasPendingGates():         Reports whether a target file has unacknowledged pending gates.
 *   - AcknowledgePendingGates(): Acknowledges and clears all pending gates for a target file.
 *   - RearmDedup():              Clears seen scopes, anchors, and injected IDs after a checkpoint.
 *   - HasInjected():             Reports whether an entry ID has already been injected.
 *   - MarkInjected():            Marks an entry ID as injected in the current session.
 *   - GetExposureList():         Returns all recorded push and pull exposures.
 *   - FindInjectionAfterStep():  Returns all injection records at or after a transcript step index.
 */

package delivery

import "time"

// InjectionRecord records a single push-delivery event within a conversation session.
type InjectionRecord struct {
	EntryID     string    // Unique identifier of the injected dossier entry.
	Channel     Channel   // Delivery channel used for the injection.
	Moment      Moment    // Lifecycle moment that triggered the injection.
	StepIndex   int       // Transcript step index at which the entry was injected.
	ContentHash string    // Hex-encoded SHA-256 hash of the formatted entry content.
	Tag         string    // Optional tag or category attached to the injection.
	Timestamp   time.Time // UTC timestamp when the injection occurred.
}

// ExposureRecord records an entry exposed to the agent via push injection or MCP pull.
type ExposureRecord struct {
	EntryID   string    // Unique identifier of the exposed dossier entry.
	Channel   Channel   // Delivery channel ("pull" for MCP tool lookups).
	Moment    Moment    // Lifecycle moment for push exposures, or empty for pulls.
	Timestamp time.Time // Timestamp when the exposure was recorded.
}

// Ledger maintains per-conversation deduplication state, gate acknowledgements,
// transcript checkpoint cursors, and exposure history across hook invocations.
type Ledger struct {
	ConversationID            string              // Unique identifier of the tracked agent conversation.
	Injections                []InjectionRecord   // Chronological history of push injections in the session.
	SeenDossiers              map[string]bool     // Set of module dossier scopes already entered.
	SeenAnchors               map[string]bool     // Set of file or symbol anchors already touched.
	AcknowledgedGates         map[string]bool     // Set of edit-gate entry IDs already acknowledged.
	ExposureList              []ExposureRecord    // Chronological list of all push and pull exposures.
	LastHandledCheckpointStep int                 `json:"last_handled_checkpoint_step"`      // Step index of the most recently handled context compaction checkpoint.
	PendingGates              map[string][]string `json:"pending_gates,omitempty"`           // Map of file path to pending gate entry IDs awaiting re-issued edit approval.
	LastProcessedStepIndex    int                 `json:"last_processed_step_index,omitempty"` // Highest transcript step index processed by hooks.
	LastKnownStepCount        int                 `json:"last_known_step_count,omitempty"`   // Total transcript step count observed on the previous turn.
	InjectedSet               map[string]bool     `json:"injected_set"`                      // Set of entry IDs injected since the start of the session or last checkpoint.
}

// NewLedger constructs an initialized Ledger for conversationID.
func NewLedger(conversationID string) *Ledger {
	return &Ledger{
		ConversationID:            conversationID,
		Injections:                make([]InjectionRecord, 0),
		SeenDossiers:              make(map[string]bool),
		SeenAnchors:               make(map[string]bool),
		AcknowledgedGates:         make(map[string]bool),
		ExposureList:              make([]ExposureRecord, 0),
		LastHandledCheckpointStep: -1,
		PendingGates:              make(map[string][]string),
		InjectedSet:               make(map[string]bool),
	}
}

// RecordInjection appends rec to the injection history, marks its EntryID as
// injected, and appends a corresponding ExposureRecord.
func (l *Ledger) RecordInjection(rec InjectionRecord) {
	l.Injections = append(l.Injections, rec)
	if l.InjectedSet == nil {
		l.InjectedSet = make(map[string]bool)
	}
	l.InjectedSet[rec.EntryID] = true
	l.ExposureList = append(l.ExposureList, ExposureRecord{
		EntryID:   rec.EntryID,
		Channel:   rec.Channel,
		Moment:    rec.Moment,
		Timestamp: rec.Timestamp,
	})
}

// RecordPull appends a pull-channel ExposureRecord for entryID at the current time.
func (l *Ledger) RecordPull(entryID string) {
	l.ExposureList = append(l.ExposureList, ExposureRecord{
		EntryID:   entryID,
		Channel:   "pull",
		Moment:    "",
		Timestamp: time.Now(),
	})
}

// HasSeenDossier reports whether scope has already been entered in the current deduplication window.
func (l *Ledger) HasSeenDossier(scope string) bool {
	return l.SeenDossiers[scope]
}

// MarkDossierSeen records that scope has been entered in the current deduplication window.
func (l *Ledger) MarkDossierSeen(scope string) {
	l.SeenDossiers[scope] = true
}

// HasSeenAnchor reports whether anchor has already been touched in the current deduplication window.
func (l *Ledger) HasSeenAnchor(anchor string) bool {
	return l.SeenAnchors[anchor]
}

// MarkAnchorSeen records that anchor has been touched in the current deduplication window.
func (l *Ledger) MarkAnchorSeen(anchor string) {
	l.SeenAnchors[anchor] = true
}

// IsGateAcknowledged reports whether the edit gate for entryID has been acknowledged.
func (l *Ledger) IsGateAcknowledged(entryID string) bool {
	return l.AcknowledgedGates[entryID]
}

// AcknowledgeGate marks the edit gate for entryID as acknowledged for the remainder of the conversation.
func (l *Ledger) AcknowledgeGate(entryID string) {
	l.AcknowledgedGates[entryID] = true
}

// SetPendingGates records that a force_ask was shown for the given file.
// The gate is not acknowledged yet — that happens when the edit is re-issued.
func (l *Ledger) SetPendingGates(file string, entryIDs []string) {
	if l.PendingGates == nil {
		l.PendingGates = make(map[string][]string)
	}
	l.PendingGates[file] = entryIDs
}

// HasPendingGates returns true if a force_ask was previously shown for this file.
func (l *Ledger) HasPendingGates(file string) bool {
	return len(l.PendingGates[file]) > 0
}

// AcknowledgePendingGates acknowledges all gates that were pending for a file
// (i.e., the user approved the edit by re-issuing it).
func (l *Ledger) AcknowledgePendingGates(file string) {
	for _, id := range l.PendingGates[file] {
		l.AcknowledgedGates[id] = true
	}
	delete(l.PendingGates, file)
}

// RearmDedup clears seen dossiers, seen anchors, and the injected-entry set so
// critical context can be re-delivered after a transcript compaction checkpoint.
func (l *Ledger) RearmDedup() {
	// --- Step 1: Clear and re-initialize seen dossier and anchor maps ---
	clear(l.SeenDossiers)
	if l.SeenDossiers == nil {
		l.SeenDossiers = make(map[string]bool)
	}
	clear(l.SeenAnchors)
	if l.SeenAnchors == nil {
		l.SeenAnchors = make(map[string]bool)
	}

	// --- Step 2: Clear and re-initialize the per-session injected entry set ---
	clear(l.InjectedSet)
	if l.InjectedSet == nil {
		l.InjectedSet = make(map[string]bool)
	}
}

// HasInjected reports whether entryID is currently marked as injected in this session.
func (l *Ledger) HasInjected(entryID string) bool {
	return l.InjectedSet != nil && l.InjectedSet[entryID]
}

// MarkInjected marks entryID as injected in the current session.
func (l *Ledger) MarkInjected(entryID string) {
	if l.InjectedSet == nil {
		l.InjectedSet = make(map[string]bool)
	}
	l.InjectedSet[entryID] = true
}

// GetExposureList returns the slice of all recorded push and pull exposures.
func (l *Ledger) GetExposureList() []ExposureRecord {
	return l.ExposureList
}

// FindInjectionAfterStep returns all recorded injections whose StepIndex is
// greater than or equal to stepIndex.
func (l *Ledger) FindInjectionAfterStep(stepIndex int) []InjectionRecord {
	var result []InjectionRecord
	for _, rec := range l.Injections {
		if rec.StepIndex >= stepIndex {
			result = append(result, rec)
		}
	}
	return result
}
