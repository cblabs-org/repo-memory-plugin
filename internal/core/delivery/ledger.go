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
	ConversationID           string
	Injections               []InjectionRecord
	SeenDossiers             map[string]bool
	SeenAnchors              map[string]bool
	AcknowledgedGates        map[string]bool
	ExposureList             []ExposureRecord
	LastHandledCheckpointStep int `json:"last_handled_checkpoint_step,omitempty"`
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
	}
}

func (l *Ledger) RecordInjection(rec InjectionRecord) {
	l.Injections = append(l.Injections, rec)
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

func (l *Ledger) RearmDedup() {
	l.SeenDossiers = make(map[string]bool)
	l.SeenAnchors = make(map[string]bool)
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
