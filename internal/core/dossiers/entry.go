package dossiers

import (
	"time"
)

type CognitiveKind string

const (
	CognitiveKindSemantic   CognitiveKind = "semantic"
	CognitiveKindProcedural CognitiveKind = "procedural"
	CognitiveKindEpisodic   CognitiveKind = "episodic"
)

func (k CognitiveKind) Valid() bool {
	switch k {
	case CognitiveKindSemantic, CognitiveKindProcedural, CognitiveKindEpisodic:
		return true
	default:
		return false
	}
}

type EntryType string

const (
	EntryTypeInvariant         EntryType = "invariant"
	EntryTypeContract          EntryType = "contract"
	EntryTypeConvention        EntryType = "convention"
	EntryTypeRuledOut          EntryType = "ruled-out"
	EntryTypeFailureSignature  EntryType = "failure-signature"
	EntryTypeGotcha            EntryType = "gotcha"
	EntryTypeExternalBehaviour EntryType = "external-behaviour"
	EntryTypeDecision          EntryType = "decision"
	EntryTypeProcedure         EntryType = "procedure"
	EntryTypeRecipe            EntryType = "recipe"
	EntryTypeInterface         EntryType = "interface"
	EntryTypeOpenIssue         EntryType = "open-issue"
)

var EntryTypePrefixes = map[EntryType]string{
	EntryTypeInvariant:         "inv-",
	EntryTypeContract:          "con-",
	EntryTypeConvention:        "con-",
	EntryTypeRuledOut:          "rul-",
	EntryTypeFailureSignature:  "fail-",
	EntryTypeGotcha:            "got-",
	EntryTypeExternalBehaviour: "got-",
	EntryTypeDecision:          "dec-",
	EntryTypeProcedure:         "proc-",
	EntryTypeRecipe:            "rec-",
	EntryTypeInterface:         "iface-",
	EntryTypeOpenIssue:         "issue-",
}

func DeriveCognitiveKind(t EntryType) CognitiveKind {
	switch t {
	case EntryTypeRecipe, EntryTypeProcedure, EntryTypeFailureSignature:
		return CognitiveKindProcedural
	case EntryTypeDecision, EntryTypeRuledOut:
		return CognitiveKindEpisodic
	case EntryTypeInvariant, EntryTypeContract, EntryTypeConvention, EntryTypeInterface,
		EntryTypeGotcha, EntryTypeExternalBehaviour, EntryTypeOpenIssue:
		return CognitiveKindSemantic
	default:
		return CognitiveKindSemantic
	}
}

type Entry struct {
	ID               string        `json:"id"`
	Type             EntryType     `json:"type"`
	Kind             CognitiveKind `json:"kind,omitempty"`
	DossierScope     string        `json:"dossier_scope"`
	Text             string        `json:"text"`
	Anchors          []string      `json:"anchors,omitempty"`
	Evidence         []string      `json:"evidence,omitempty"`
	Verified         string        `json:"verified,omitempty"`
	ClosesWhen       string        `json:"closes_when,omitempty"`
	Source           string        `json:"source"`
	SourceDocPath    string        `json:"source_doc_path,omitempty"`
	SourceDocBlobSHA string        `json:"source_doc_blob_sha,omitempty"`
	SuccessCount     int           `json:"success_count,omitempty"`
	FailureCount     int           `json:"failure_count,omitempty"`
	LastSucceededAt  string        `json:"last_succeeded_at,omitempty"`
	StaleSuspect     bool          `json:"stale_suspect,omitempty"`
	RenderAsSkill    string        `json:"render_as_skill,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	Retired          bool          `json:"retired"`
	RetiredReason    string        `json:"retired_reason,omitempty"`
	SupersededBy     string        `json:"superseded_by,omitempty"`
}

func (e Entry) EffectiveKind() CognitiveKind {
	if e.Kind.Valid() {
		return e.Kind
	}
	return DeriveCognitiveKind(e.Type)
}

