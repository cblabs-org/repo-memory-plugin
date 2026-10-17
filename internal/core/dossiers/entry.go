/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/entry.go
 * Description: Dossier entry domain types, cognitive classifications, entry type
 *              ID prefixes, and cognitive kind derivation helpers.
 *
 * Exported Types / Functions:
 * - CognitiveKind:              High-level cognitive classification of a memory entry.
 *   - Valid():                  Reports whether the CognitiveKind is a recognized constant.
 * - CognitiveKindSemantic:      Semantic knowledge classification ("semantic").
 * - CognitiveKindProcedural:    Procedural knowledge classification ("procedural").
 * - CognitiveKindEpisodic:      Episodic knowledge classification ("episodic").
 * - EntryType:                  Specific structural category of a dossier entry.
 * - EntryTypeInvariant:         Invariant entry type ("invariant").
 * - EntryTypeContract:          Contract entry type ("contract").
 * - EntryTypeConvention:        Convention entry type ("convention").
 * - EntryTypeRuledOut:          Ruled-out approach entry type ("ruled-out").
 * - EntryTypeFailureSignature:  Failure signature entry type ("failure-signature").
 * - EntryTypeGotcha:            Gotcha entry type ("gotcha").
 * - EntryTypeExternalBehaviour: External behaviour entry type ("external-behaviour").
 * - EntryTypeDecision:          Architectural decision entry type ("decision").
 * - EntryTypeProcedure:         Procedure entry type ("procedure").
 * - EntryTypeRecipe:            Recipe entry type ("recipe").
 * - EntryTypeInterface:         Interface/dependency entry type ("interface").
 * - EntryTypeOpenIssue:         Open issue entry type ("open-issue").
 * - EntryTypePrefixes:          Canonical ID prefix mapping for each EntryType.
 * - DeriveCognitiveKind():      Returns the default CognitiveKind corresponding to an EntryType.
 * - Entry:                      Projected dossier entry with provenance, anchors, and lifecycle state.
 *   - EffectiveKind():          Returns the entry's explicit CognitiveKind or derives it from Type.
 */

package dossiers

import (
	"time"
)

// CognitiveKind represents the cognitive memory tier of a dossier entry.
type CognitiveKind string

const (
	// CognitiveKindSemantic represents durable invariants, contracts, interfaces, and gotchas.
	CognitiveKindSemantic CognitiveKind = "semantic"
	// CognitiveKindProcedural represents executable recipes, procedures, and failure signatures.
	CognitiveKindProcedural CognitiveKind = "procedural"
	// CognitiveKindEpisodic represents historical decisions and ruled-out approaches.
	CognitiveKindEpisodic CognitiveKind = "episodic"
)

// Valid reports whether k is one of the defined CognitiveKind constants.
func (k CognitiveKind) Valid() bool {
	switch k {
	case CognitiveKindSemantic, CognitiveKindProcedural, CognitiveKindEpisodic:
		return true
	default:
		return false
	}
}

// EntryType identifies the specific structural category of a dossier entry.
type EntryType string

const (
	// EntryTypeInvariant denotes a hard system or module invariant.
	EntryTypeInvariant EntryType = "invariant"
	// EntryTypeContract denotes an API or cross-component contract.
	EntryTypeContract EntryType = "contract"
	// EntryTypeConvention denotes a codebase convention or pattern.
	EntryTypeConvention EntryType = "convention"
	// EntryTypeRuledOut denotes a rejected or ruled-out approach.
	EntryTypeRuledOut EntryType = "ruled-out"
	// EntryTypeFailureSignature denotes a recurring error or failure signature and its fix.
	EntryTypeFailureSignature EntryType = "failure-signature"
	// EntryTypeGotcha denotes a non-obvious trap or footgun.
	EntryTypeGotcha EntryType = "gotcha"
	// EntryTypeExternalBehaviour denotes an external dependency quirk or behaviour.
	EntryTypeExternalBehaviour EntryType = "external-behaviour"
	// EntryTypeDecision denotes a recorded architectural or design decision.
	EntryTypeDecision EntryType = "decision"
	// EntryTypeProcedure denotes a multi-step operational procedure.
	EntryTypeProcedure EntryType = "procedure"
	// EntryTypeRecipe denotes a reusable implementation or debugging recipe.
	EntryTypeRecipe EntryType = "recipe"
	// EntryTypeInterface denotes a key module interface or dependency boundary.
	EntryTypeInterface EntryType = "interface"
	// EntryTypeOpenIssue denotes an unresolved issue or technical debt item.
	EntryTypeOpenIssue EntryType = "open-issue"
)

// EntryTypePrefixes maps each EntryType to its canonical entry ID prefix.
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

// DeriveCognitiveKind returns the default CognitiveKind for the given EntryType.
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

// Entry represents a single projected dossier entry, including its text,
// anchors, provenance, verification state, and retirement metadata.
type Entry struct {
	ID               string        `json:"id"`                           // Unique entry identifier (e.g. "inv-01abc").
	Type             EntryType     `json:"type"`                         // Structural category of the entry.
	Kind             CognitiveKind `json:"kind,omitempty"`               // Explicit or derived cognitive memory tier.
	DossierScope     string        `json:"dossier_scope"`                // Slash-separated scope slug of the owning dossier in .memory/dossiers/.
	Text             string        `json:"text"`                         // Markdown prose body of the entry.
	Anchors          []string      `json:"anchors,omitempty"`            // File paths, globs, or symbols anchored to this entry.
	Evidence         []string      `json:"evidence,omitempty"`           // Supporting evidence references (commits, tests, episodes).
	Verified         string        `json:"verified,omitempty"`           // Verification commit SHA or date stamp.
	ClosesWhen       string        `json:"closes_when,omitempty"`        // Condition under which an open-issue entry can be retired.
	Source           string        `json:"source"`                       // Origin of the entry (e.g. "distill", "human", "doc-seed").
	SourceDocPath    string        `json:"source_doc_path,omitempty"`    // Repository-relative path of the seeded source document.
	SourceDocBlobSHA string        `json:"source_doc_blob_sha,omitempty"` // Git blob SHA of the seeded source document.
	SuccessCount     int           `json:"success_count,omitempty"`      // Number of times this procedural entry contributed to a successful outcome.
	FailureCount     int           `json:"failure_count,omitempty"`      // Number of times this procedural entry was associated with a failure.
	LastSucceededAt  string        `json:"last_succeeded_at,omitempty"`  // RFC3339 timestamp of the last recorded success.
	StaleSuspect     bool          `json:"stale_suspect,omitempty"`      // Whether anchor drift has flagged this entry as potentially stale.
	RenderAsSkill    string        `json:"render_as_skill,omitempty"`    // Target skill slug when graduated into .agents/skills/.
	CreatedAt        time.Time     `json:"created_at"`                   // Timestamp when the entry was first added.
	UpdatedAt        time.Time     `json:"updated_at"`                   // Timestamp of the most recent operation touching this entry.
	Retired          bool          `json:"retired"`                      // Whether the entry has been retired or superseded into history.
	RetiredReason    string        `json:"retired_reason,omitempty"`     // Reason for retirement (e.g. "superseded", "obsolete").
	SupersededBy     string        `json:"superseded_by,omitempty"`      // Replacement entry ID when retired via OpSupersede.
}

// EffectiveKind returns the entry's explicit Kind if valid, or derives it from Type.
func (e Entry) EffectiveKind() CognitiveKind {
	if e.Kind.Valid() {
		return e.Kind
	}
	return DeriveCognitiveKind(e.Type)
}
