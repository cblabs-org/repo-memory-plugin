package delivery

import "time"

type InjectionRecord struct {
	EntryID     string
	Channel     Channel
	Moment      Moment
	StepIndex   int
	ContentHash string
	Tag         string
	Timestamp   time.Time
}

type ExposureRecord struct {
	EntryID   string
	Channel   Channel
	Moment    Moment
	Timestamp time.Time
}

type Ledger struct {
	ConversationID            string
	Injections                []InjectionRecord
	SeenDossiers              map[string]bool
	SeenAnchors               map[string]bool
	AcknowledgedGates         map[string]bool
	ExposureList              []ExposureRecord
	LastHandledCheckpointStep int                 `json:"last_handled_checkpoint_step,omitempty"`
	PendingGates              map[string][]string `json:"pending_gates,omitempty"`
	LastProcessedStepIndex    int                 `json:"last_processed_step_index,omitempty"`
	InjectedSet               map[string]bool     `json:"injected_set,omitempty"`
}

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

func (l *Ledger) RecordPull(entryID string) {
	l.ExposureList = append(l.ExposureList, ExposureRecord{
		EntryID:   entryID,
		Channel:   "pull",
		Moment:    "",
		Timestamp: time.Now(),
	})
}

func (l *Ledger) HasSeenDossier(scope string) bool {
	return l.SeenDossiers[scope]
}

func (l *Ledger) MarkDossierSeen(scope string) {
	l.SeenDossiers[scope] = true
}

func (l *Ledger) HasSeenAnchor(anchor string) bool {
	return l.SeenAnchors[anchor]
}

func (l *Ledger) MarkAnchorSeen(anchor string) {
	l.SeenAnchors[anchor] = true
}

func (l *Ledger) IsGateAcknowledged(entryID string) bool {
	return l.AcknowledgedGates[entryID]
}

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

func (l *Ledger) RearmDedup() {
	l.SeenDossiers = make(map[string]bool)
	l.SeenAnchors = make(map[string]bool)
	l.InjectedSet = make(map[string]bool)
}

func (l *Ledger) HasInjected(entryID string) bool {
	return l.InjectedSet != nil && l.InjectedSet[entryID]
}

func (l *Ledger) MarkInjected(entryID string) {
	if l.InjectedSet == nil {
		l.InjectedSet = make(map[string]bool)
	}
	l.InjectedSet[entryID] = true
}

func (l *Ledger) GetExposureList() []ExposureRecord {
	return l.ExposureList
}

func (l *Ledger) FindInjectionAfterStep(stepIndex int) []InjectionRecord {
	var result []InjectionRecord
	for _, rec := range l.Injections {
		if rec.StepIndex >= stepIndex {
			result = append(result, rec)
		}
	}
	return result
}
