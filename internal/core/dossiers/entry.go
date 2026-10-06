package dossiers

import (
	"time"
)

type EntryType string

const (
	EntryTypeInvariant        EntryType = "invariant"
	EntryTypeRuledOut         EntryType = "ruled-out"
	EntryTypeFailureSignature EntryType = "failure-signature"
	EntryTypeGotcha           EntryType = "gotcha"
	EntryTypeDecision         EntryType = "decision"
	EntryTypeProcedure        EntryType = "procedure"
	EntryTypeInterface        EntryType = "interface"
	EntryTypeOpenIssue        EntryType = "open-issue"
)

var EntryTypePrefixes = map[EntryType]string{
	EntryTypeInvariant:        "inv-",
	EntryTypeRuledOut:         "rul-",
	EntryTypeFailureSignature: "fail-",
	EntryTypeGotcha:           "got-",
	EntryTypeDecision:         "dec-",
	EntryTypeProcedure:        "proc-",
	EntryTypeInterface:        "iface-",
	EntryTypeOpenIssue:        "issue-",
}

type Entry struct {
	ID            string    `json:"id"`
	Type          EntryType `json:"type"`
	DossierScope  string    `json:"dossier_scope"`
	Text          string    `json:"text"`
	Anchors       []string  `json:"anchors,omitempty"`
	Evidence      []string  `json:"evidence,omitempty"`
	Verified      string    `json:"verified,omitempty"`
	ClosesWhen    string    `json:"closes_when,omitempty"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Retired       bool      `json:"retired"`
	RetiredReason string    `json:"retired_reason,omitempty"`
	SupersededBy  string    `json:"superseded_by,omitempty"`
}
