/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/consolidate/route.go
 * Description: Implements deterministic Five-Tier State Scope Hierarchy
 *              classification, dossier scope routing, exact-duplicate merging,
 *              and broken file/symbol anchor auditing for Stage 2 consolidation.
 *
 * Exported Types / Functions:
 * - ScopeTier:                Represents a tier in the Five-Tier State Scope Hierarchy.
 * - TierTemp:                 Ephemeral temporary scope tier ("temp").
 * - TierSession:              Single-session scope tier ("session").
 * - TierUser:                 Workstation-local personal user scope tier ("user").
 * - TierBranch:               Gitignored feature-branch scope tier ("branch").
 * - TierRepo:                 Shared repository-wide scope tier ("repo").
 * - RoutedItem:               Pairs a candidate NoteItem with its resolved ScopeTier, scope, and kind.
 * - DedupResult:              Holds the outcome of deterministic exact-duplicate merging.
 * - BrokenAnchorReport:       Describes an active dossier entry with one or more missing anchors.
 * - StripTierPrefix():        Extracts a known tier prefix and trimmed scope remainder.
 * - IsDefaultBranch():        Reports whether a branch name is empty or a default branch.
 * - ClassifyTier():           Classifies a candidate note item into a ScopeTier.
 * - RouteItem():              Classifies an item's tier and routes it to a target dossier scope and kind.
 * - RouteItems():             Routes a slice of NoteItems using RouteItem.
 * - CanonicalEntryTypeGroup(): Maps alias-equivalent entry types to their canonical counterpart.
 * - MergeExactDuplicates():   Merges duplicate candidate items into OpAmend ops and coalesces batch duplicates.
 * - CheckAnchor():            Reports whether a file and optional symbol anchor exist in repoRoot.
 * - CheckDossierAnchors():    Inspects active dossier entries and reports broken file or symbol anchors.
 */

// Package consolidate implements the Stage 2 consolidation pipeline: echo
// filtering, corroboration gating, tier and scope routing, exact-duplicate
// merging, LLM/deterministic op synthesis, op validation and scrubbing, and
// multi-tier dossier projection.
package consolidate

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// ScopeTier represents a tier in the Five-Tier State Scope Hierarchy (VD17, SPEC §5.5).
type ScopeTier string

const (
	// TierTemp designates ephemeral scratch state that is never persisted to dossiers.
	TierTemp ScopeTier = "temp"
	// TierSession designates single-conversation state bounded to an active session.
	TierSession ScopeTier = "session"
	// TierUser designates developer-local workstation state stored under <dataDir>/<repoID>/user-dossiers.
	TierUser ScopeTier = "user"
	// TierBranch designates unmerged feature-branch state stored under .memory/local/{ops,dossiers}/<branch-slug>.
	TierBranch ScopeTier = "branch"
	// TierRepo designates shared repository-wide state stored under .memory/{ops,dossiers}.
	TierRepo ScopeTier = "repo"
)

// StripTierPrefix inspects rawScope case-insensitively for a known tier prefix
// ("temp:", "session:", "user:", "branch:", or "repo:"). If present, it returns
// the matched ScopeTier and the trimmed remainder; otherwise ("", trimmed rawScope).
func StripTierPrefix(rawScope string) (ScopeTier, string) {
	// --- Step 1: Trim input and handle empty strings ---
	trimmed := strings.TrimSpace(rawScope)
	if trimmed == "" {
		return "", ""
	}
	lower := strings.ToLower(trimmed)
	prefixes := []struct {
		prefix string
		tier   ScopeTier
	}{
		{"temp:", TierTemp},
		{"session:", TierSession},
		{"user:", TierUser},
		{"branch:", TierBranch},
		{"repo:", TierRepo},
	}

	// --- Step 2: Match known tier prefixes case-insensitively ---
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p.prefix) {
			return p.tier, strings.TrimSpace(trimmed[len(p.prefix):])
		}
	}
	return "", trimmed
}

// IsDefaultBranch reports whether branch is empty or refers to a default branch ("main", "master", "HEAD").
func IsDefaultBranch(branch string) bool {
	b := strings.TrimSpace(branch)
	return b == "" ||
		strings.EqualFold(b, "main") ||
		strings.EqualFold(b, "master") ||
		strings.EqualFold(b, "HEAD")
}

// ClassifyTier classifies a candidate note item into the Five-Tier State Scope Hierarchy
// (SPEC §5.5, design-v2 §6 step 7).
func ClassifyTier(note *inbox.Note, item inbox.NoteItem, repoRoot string, isBranchMerged func(branch string) bool) ScopeTier {
	// --- Step 1: Honour explicit tier prefix on item.Scope ---
	prefixTier, _ := StripTierPrefix(item.Scope)
	switch prefixTier {
	case TierUser, TierBranch, TierRepo, TierSession, TierTemp:
		return prefixTier
	}

	// --- Step 2: Check personal/workstation scope heuristic (TierUser) ---
	trimmedScope := strings.TrimSpace(item.Scope)
	lowerScope := strings.ToLower(trimmedScope)
	if strings.HasPrefix(lowerScope, "user-dossiers") || strings.EqualFold(trimmedScope, "user") {
		return TierUser
	}

	// --- Step 3: Check unmerged feature-branch heuristic (TierBranch) when all file anchors are missing on default branch ---
	if note != nil && !IsDefaultBranch(note.Branch) {
		merged := isBranchMerged != nil && isBranchMerged(note.Branch)
		if !merged {
			fileAnchors := make([]string, 0, len(item.Anchors))
			for _, a := range item.Anchors {
				if StripAnchorSymbol(a) != "" {
					fileAnchors = append(fileAnchors, a)
				}
			}
			if len(fileAnchors) > 0 {
				if repoRoot != "" {
					checks := inbox.CheckAnchors(repoRoot, fileAnchors)
					allMissing := len(checks) > 0
					for _, c := range checks {
						if c.Exists {
							allMissing = false
							break
						}
					}
					if allMissing {
						return TierBranch
					}
				} else if hasBranchOnlyAnchorChecks(note, item, fileAnchors) {
					return TierBranch
				}
			}
		}
	}

	// --- Step 4: Default to shared repository tier (TierRepo) ---
	return TierRepo
}

// hasBranchOnlyAnchorChecks reports whether pre-recorded anchor checks on item or note
// confirm that all fileAnchors were missing from the working tree when repoRoot is unavailable.
func hasBranchOnlyAnchorChecks(note *inbox.Note, item inbox.NoteItem, fileAnchors []string) bool {
	// --- Step 1: Collect pre-recorded AnchorCheck entries from item and note ---
	checks := make([]inbox.AnchorCheck, 0, len(item.AnchorChecks))
	checks = append(checks, item.AnchorChecks...)
	if note != nil {
		checks = append(checks, note.AnchorChecks...)
	}
	if len(checks) == 0 {
		return false
	}

	// --- Step 2: Verify that at least one file anchor matched a check and none existed ---
	matchedAny := false
	for _, fa := range fileAnchors {
		normFA := StripAnchorSymbol(fa)
		for _, ac := range checks {
			normAC := StripAnchorSymbol(ac.Anchor)
			if strings.EqualFold(strings.TrimSpace(ac.Anchor), strings.TrimSpace(fa)) || (normFA != "" && normAC == normFA) {
				matchedAny = true
				if ac.Exists {
					return false
				}
			}
		}
	}
	return matchedAny
}

// RoutedItem pairs a candidate NoteItem with its resolved ScopeTier, target dossier scope, and kind.
type RoutedItem struct {
	NoteID         string               `json:"note_id"`                   // Originating inbox note ID.
	NoteSource     inbox.NoteSource     `json:"note_source"`               // Stage 1 source pathway of the parent note.
	ConversationID string               `json:"conversation_id,omitempty"` // Originating conversation or session ID.
	Branch         string               `json:"branch,omitempty"`          // Git branch recorded on the parent note.
	HeadCommit     string               `json:"head_commit,omitempty"`     // Git HEAD commit SHA recorded on the parent note.
	CreatedAt      time.Time            `json:"created_at"`                // UTC creation timestamp of the parent note.
	Tier           ScopeTier            `json:"tier"`                      // Resolved Five-Tier State Scope Hierarchy tier.
	DossierScope   string               `json:"dossier_scope"`             // Target dossier scope path (e.g. "core/index" or "general").
	DossierKind    dossiers.DossierKind `json:"dossier_kind"`              // Target dossier kind (module, topic, external, or procedure).
	DerivedNew     bool                 `json:"derived_new,omitempty"`     // True when DossierScope was newly derived from anchor directory structure.
	Item           inbox.NoteItem       `json:"item"`                      // Candidate knowledge item being routed.
}

// RouteItem deterministically classifies an item's tier and routes it to a target dossier scope and kind
// (SPEC §6.5 step 2, design-v2 §6 step 2).
func RouteItem(note *inbox.Note, item inbox.NoteItem, resolver *config.ScopeResolver, repoRoot string, isBranchMerged func(branch string) bool) RoutedItem {
	// --- Step 1: Classify the item's state tier and normalize any explicit scope ---
	tier := ClassifyTier(note, item, repoRoot, isBranchMerged)

	_, explicitScope := StripTierPrefix(item.Scope)
	explicitScope = strings.ReplaceAll(explicitScope, "\\", "/")
	explicitScope = strings.Trim(strings.TrimSpace(explicitScope), "/")
	if strings.EqualFold(explicitScope, "user") {
		explicitScope = ""
	}
	if strings.HasPrefix(strings.ToLower(explicitScope), "user-dossiers/") {
		explicitScope = strings.Trim(explicitScope[len("user-dossiers/"):], "/")
	} else if strings.EqualFold(explicitScope, "user-dossiers") {
		explicitScope = ""
	}

	var (
		dossierScope string
		dossierKind  dossiers.DossierKind
		derivedNew   bool
	)

	// --- Step 2: Case A — Resolve explicit scope on item.Scope ---
	if explicitScope != "" {
		dossierScope = explicitScope
		if resolver != nil {
			if resolved := resolver.ResolveScope([]string{explicitScope}, nil); resolved != "" {
				dossierScope = resolved
			} else if k := resolver.ScopeKind(explicitScope); k != "" {
				// ScopeKind resolves aliases internally; check if an alias mapped explicitScope.
				if resolvedKw := resolver.ResolveScope(nil, []string{explicitScope}); resolvedKw != "" {
					dossierScope = resolvedKw
				}
			}
			dossierKind = resolver.ScopeKind(dossierScope)
		}
		if dossierKind == "" {
			dossierKind = inferDossierKindFromScope(dossierScope)
		}
	}

	// --- Step 3: Case B — Route anchored items via ScopeResolver or module derivation ---
	if dossierScope == "" && len(item.Anchors) > 0 {
		if resolver != nil {
			if matched := resolver.ResolveScope(item.Anchors, nil); matched != "" {
				dossierScope = matched
				dossierKind = resolver.ScopeKind(matched)
				if dossierKind == "" {
					dossierKind = dossiers.DossierKindModule
				}
			}
		}
		if dossierScope == "" {
			fileAnchors := make([]string, 0, len(item.Anchors))
			for _, a := range item.Anchors {
				if StripAnchorSymbol(a) != "" {
					fileAnchors = append(fileAnchors, a)
				}
			}
			if len(fileAnchors) > 0 {
				derived := config.DeriveModuleScopes(fileAnchors)
				if len(derived) > 0 {
					dossierScope = derived[0].Name
					dossierKind = dossiers.DossierKindModule
					derivedNew = true
				}
			}
		}
	}

	// --- Step 4: Case C — Route unanchored items by keyword matching or cognitive type fallback ---
	if dossierScope == "" {
		keywords := extractRoutingKeywords(item)
		if resolver != nil && len(keywords) > 0 {
			if matched := resolver.ResolveScope(nil, keywords); matched != "" {
				dossierScope = matched
				dossierKind = resolver.ScopeKind(matched)
			}
		}
		if dossierScope == "" {
			switch item.Type {
			case dossiers.EntryTypeRecipe, dossiers.EntryTypeProcedure:
				dossierScope = "procedures/general"
				dossierKind = dossiers.DossierKindProcedure
			case dossiers.EntryTypeExternalBehaviour, dossiers.EntryTypeGotcha:
				dossierScope = "external/general"
				dossierKind = dossiers.DossierKindExternal
			default:
				dossierScope = "general"
				dossierKind = dossiers.DossierKindTopic
			}
		}
		if dossierKind == "" {
			dossierKind = inferDossierKindFromScope(dossierScope)
		}
	}

	// --- Step 5: Populate RoutedItem metadata from the parent note ---
	item.Scope = dossierScope

	routed := RoutedItem{
		Tier:         tier,
		DossierScope: dossierScope,
		DossierKind:  dossierKind,
		DerivedNew:   derivedNew,
		Item:         item,
	}
	if note != nil {
		routed.NoteID = note.ID
		routed.NoteSource = note.Source
		routed.ConversationID = note.ConversationID
		routed.Branch = note.Branch
		routed.HeadCommit = note.HeadCommit
		routed.CreatedAt = note.CreatedAt
	}
	return routed
}

// RouteItems routes a slice of NoteItems from note using RouteItem.
func RouteItems(note *inbox.Note, items []inbox.NoteItem, resolver *config.ScopeResolver, repoRoot string, isBranchMerged func(branch string) bool) []RoutedItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]RoutedItem, 0, len(items))
	for _, item := range items {
		out = append(out, RouteItem(note, item, resolver, repoRoot, isBranchMerged))
	}
	return out
}

// inferDossierKindFromScope infers a DossierKind from conventional scope path prefixes.
func inferDossierKindFromScope(scope string) dossiers.DossierKind {
	lower := strings.ToLower(strings.TrimSpace(scope))
	switch {
	case strings.HasPrefix(lower, "topics/") || lower == "general":
		return dossiers.DossierKindTopic
	case strings.HasPrefix(lower, "external/") || strings.HasPrefix(lower, "externals/"):
		return dossiers.DossierKindExternal
	case strings.HasPrefix(lower, "procedures/") || strings.HasPrefix(lower, "procedure/"):
		return dossiers.DossierKindProcedure
	default:
		return dossiers.DossierKindModule
	}
}

// routingStopwords contains common English function words excluded during unanchored keyword routing.
var routingStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true,
	"that": true, "this": true, "when": true, "only": true, "have": true,
	"should": true, "must": true, "always": true, "never": true, "into": true,
	"over": true, "under": true, "after": true, "before": true, "while": true,
	"where": true, "which": true, "their": true, "there": true, "these": true,
	"those": true, "would": true, "could": true, "about": true, "other": true,
	"using": true, "used": true, "uses": true, "make": true, "made": true,
	"does": true, "done": true, "will": true, "shall": true, "are": true,
	"was": true, "were": true, "been": true, "being": true, "has": true,
	"had": true, "not": true, "but": true, "can": true, "may": true,
	"out": true, "all": true, "any": true, "new": true, "old": true,
	"one": true, "two": true, "via": true, "per": true, "use": true,
	"run": true, "set": true, "get": true, "add": true, "some": true,
	"more": true, "than": true, "then": true, "also": true, "each": true,
	"every": true, "both": true, "such": true, "same": true, "very": true,
}

// extractRoutingKeywords extracts deduplicated command phrases and non-stopword tokens
// from item.Commands, item.Title, item.Why, and item.Do for ScopeResolver matching.
func extractRoutingKeywords(item inbox.NoteItem) []string {
	// --- Step 1: Initialize deduplicating keyword collector ---
	seen := make(map[string]bool)
	var out []string
	addKw := func(kw string) {
		k := strings.ToLower(strings.TrimSpace(kw))
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, k)
	}

	// --- Step 2: Preserve full command phrases first so multi-word commands match with full weight ---
	for _, cmd := range item.Commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed != "" {
			addKw(trimmed)
		}
	}

	// --- Step 3: Extract punctuation-trimmed tokens and normalized sub-words from Title, Why, Do, and Commands ---
	sources := []string{item.Title, item.Why, item.Do}
	sources = append(sources, item.Commands...)

	for _, src := range sources {
		if strings.TrimSpace(src) == "" {
			continue
		}
		// Extract punctuation-trimmed tokens (preserving internal _ and - for tool names like mcp_antigravity or pre-tool).
		for _, rawTok := range strings.Fields(src) {
			tok := strings.Trim(strings.ToLower(rawTok), ".,:;!?'\"`()[]{}")
			if len(tok) >= 3 && !routingStopwords[tok] {
				addKw(tok)
			}
		}
		// Also extract normalized sub-words.
		for _, tok := range strings.Fields(NormalizeTitle(src)) {
			if len(tok) >= 3 && !routingStopwords[tok] {
				addKw(tok)
			}
		}
	}

	return out
}

// DedupResult holds the outcome of deterministic exact-duplicate merging (MergeExactDuplicates).
type DedupResult struct {
	Remaining []RoutedItem // Candidate items that did not match any existing active entry and were deduplicated within the batch.
	AmendOps  []ops.Op     // Deterministic OpAmend operations merging duplicate candidate evidence into existing active entries.
}

// CanonicalEntryTypeGroup maps alias-equivalent entry types to their canonical counterpart
// so exact duplicates across aliases merge cleanly.
func CanonicalEntryTypeGroup(t dossiers.EntryType) dossiers.EntryType {
	switch t {
	case dossiers.EntryTypeConvention:
		return dossiers.EntryTypeContract
	case dossiers.EntryTypeProcedure:
		return dossiers.EntryTypeRecipe
	case dossiers.EntryTypeExternalBehaviour:
		return dossiers.EntryTypeGotcha
	default:
		return t
	}
}

// MergeExactDuplicates deterministically merges candidate items that duplicate existing active
// dossier entries into OpAmend operations, and coalesces duplicate candidate items within the
// remaining batch (SPEC §6.5 step 3, design-v2 §6 step 3).
func MergeExactDuplicates(routed []RoutedItem, dossiersMap map[string]*dossiers.Dossier, now time.Time) DedupResult {
	// --- Step 1: Normalize timestamp and handle empty input ---
	var res DedupResult
	if len(routed) == 0 {
		return res
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	// Track in-flight OpAmend index by EntryID so multiple candidates matching the same entry coalesce.
	amendIdxByEntryID := make(map[string]int)
	var unmatched []RoutedItem

	// --- Step 2: Deduplicate candidate items against existing active entries in dossiersMap ---
	for _, ri := range routed {
		var matchedEntry *dossiers.Entry
		if dossiersMap != nil && (ri.Tier == "" || ri.Tier == TierRepo) {
			if d := dossiersMap[ri.DossierScope]; d != nil {
				normCandTitle := NormalizeTitle(ri.Item.Title)
				candGroup := CanonicalEntryTypeGroup(ri.Item.Type)
				if normCandTitle != "" {
					for i := range d.Entries {
						e := &d.Entries[i]
						if e.Retired || e.SupersededBy != "" {
							continue
						}
						if CanonicalEntryTypeGroup(e.Type) != candGroup {
							continue
						}
						if NormalizeTitle(ExtractEntryTitle(e.Text)) != normCandTitle {
							continue
						}
						if !AnchorsOverlap(ri.Item.Anchors, e.Anchors) {
							continue
						}
						matchedEntry = e
						break
					}
				}
			}
		}

		if matchedEntry == nil {
			unmatched = append(unmatched, ri)
			continue
		}

		entryID := matchedEntry.ID
		if idx, exists := amendIdxByEntryID[entryID]; exists {
			existingOp := &res.AmendOps[idx]
			prevEvidence, _ := existingOp.Fields["evidence"].([]string)
			prevAnchors, _ := existingOp.Fields["anchors"].([]string)
			existingOp.Fields["evidence"] = mergeEvidenceSlice(prevEvidence, ri.NoteID, ri.Item.Evidence)
			existingOp.Fields["anchors"] = combineAnchors(prevAnchors, ri.Item.Anchors)
			if _, hasVerified := existingOp.Fields["verified"]; !hasVerified {
				if !IsVerifiedStamp(matchedEntry.Verified) && IsVerifiedStamp(ri.Item.Verified) {
					existingOp.Fields["verified"] = ri.Item.Verified
				}
			}
			if existingOp.SessionID == "" && ri.ConversationID != "" {
				existingOp.SessionID = ri.ConversationID
			}
		} else {
			mergedEvidence := mergeEvidenceSlice(matchedEntry.Evidence, ri.NoteID, ri.Item.Evidence)
			mergedAnchors := combineAnchors(matchedEntry.Anchors, ri.Item.Anchors)
			fields := map[string]any{
				"evidence": mergedEvidence,
				"anchors":  mergedAnchors,
			}
			if !IsVerifiedStamp(matchedEntry.Verified) && IsVerifiedStamp(ri.Item.Verified) {
				fields["verified"] = ri.Item.Verified
			}
			op := ops.Op{
				Type:         ops.OpAmend,
				EntryID:      entryID,
				DossierScope: ri.DossierScope,
				Timestamp:    now,
				Source:       ops.SourceConsolidation,
				SessionID:    ri.ConversationID,
				Fields:       fields,
			}
			amendIdxByEntryID[entryID] = len(res.AmendOps)
			res.AmendOps = append(res.AmendOps, op)
		}
	}

	// --- Step 3: Coalesce exact duplicates within the remaining candidate batch ---
	for _, cand := range unmatched {
		normCandTitle := NormalizeTitle(cand.Item.Title)
		candGroup := CanonicalEntryTypeGroup(cand.Item.Type)
		mergedIntoExisting := false

		if normCandTitle != "" {
			for i := range res.Remaining {
				existing := &res.Remaining[i]
				if existing.Tier != cand.Tier || existing.DossierScope != cand.DossierScope {
					continue
				}
				if CanonicalEntryTypeGroup(existing.Item.Type) != candGroup {
					continue
				}
				if NormalizeTitle(existing.Item.Title) != normCandTitle {
					continue
				}
				if !AnchorsOverlap(existing.Item.Anchors, cand.Item.Anchors) {
					continue
				}

				// Merge cand into existing.
				evSlice := mergeEvidenceSlice(nil, existing.NoteID, existing.Item.Evidence)
				evSlice = mergeEvidenceSlice(evSlice, cand.NoteID, cand.Item.Evidence)
				existing.Item.Evidence = strings.Join(evSlice, ", ")
				existing.Item.Anchors = combineAnchors(existing.Item.Anchors, cand.Item.Anchors)
				if !IsVerifiedStamp(existing.Item.Verified) && IsVerifiedStamp(cand.Item.Verified) {
					existing.Item.Verified = cand.Item.Verified
				}
				if existing.Item.Why == "" && cand.Item.Why != "" {
					existing.Item.Why = cand.Item.Why
				}
				if existing.Item.Do == "" && cand.Item.Do != "" {
					existing.Item.Do = cand.Item.Do
				}
				mergedIntoExisting = true
				break
			}
		}

		if !mergedIntoExisting {
			res.Remaining = append(res.Remaining, cand)
		}
	}

	return res
}

// mergeEvidenceSlice combines base evidence entries, noteID, and comma-separated itemEvidence into a deduplicated slice.
func mergeEvidenceSlice(base []string, noteID string, itemEvidence string) []string {
	// --- Step 1: Define comma-splitting deduplicated evidence appender ---
	seen := make(map[string]bool)
	var out []string
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}

	// --- Step 2: Merge existing base evidence, noteID, and itemEvidence ---
	for _, b := range base {
		add(b)
	}
	if noteID != "" {
		add(noteID)
	}
	if itemEvidence != "" && itemEvidence != noteID {
		add(itemEvidence)
	}
	return out
}

// BrokenAnchorReport describes an active dossier entry with one or more missing anchors in repoRoot.
type BrokenAnchorReport struct {
	EntryID        string             `json:"entry_id"`        // Unique identifier of the entry with broken anchors.
	DossierScope   string             `json:"dossier_scope"`   // Dossier scope containing the entry.
	EntryType      dossiers.EntryType `json:"entry_type"`      // Cognitive entry type of the affected entry.
	Title          string             `json:"title"`           // Extracted one-line title of the affected entry.
	MissingAnchors []string           `json:"missing_anchors"` // Anchors whose file path or symbol could not be found in repoRoot.
	ValidAnchors   []string           `json:"valid_anchors"`   // Anchors that still resolve validly in repoRoot.
	AllMissing     bool               `json:"all_missing"`     // True when every anchor on the entry is missing (len(ValidAnchors) == 0).
}

// CheckAnchor reports whether a file (and optional #symbol or :symbol suffix) anchor exists in repoRoot.
func CheckAnchor(repoRoot, rawAnchor string) bool {
	// --- Step 1: Validate inputs and extract the normalized file path ---
	trimmed := strings.TrimSpace(rawAnchor)
	if repoRoot == "" || trimmed == "" {
		return false
	}
	normPath := StripAnchorSymbol(trimmed)
	if normPath == "" {
		return false
	}

	// --- Step 2: Check file existence and verify optional symbol presence inside the file ---
	checks := inbox.CheckAnchors(repoRoot, []string{trimmed})
	exists := len(checks) > 0 && checks[0].Exists
	if exists {
		if sym := extractAnchorSymbol(trimmed); sym != "" {
			fullPath := filepath.Join(repoRoot, filepath.FromSlash(normPath))
			info, err := os.Stat(fullPath)
			if err != nil || !info.Mode().IsRegular() {
				return false
			}
			data, err := os.ReadFile(fullPath)
			if err != nil || !strings.Contains(string(data), sym) {
				return false
			}
		}
	}
	return exists
}

// CheckDossierAnchors inspects all active entries in d against repoRoot and returns a report
// for each entry that has at least one missing file or symbol anchor (SPEC §6.5 step 3, design-v2 §6 step 4).
func CheckDossierAnchors(repoRoot string, d *dossiers.Dossier) []BrokenAnchorReport {
	// --- Step 1: Return early when repoRoot or dossier is empty ---
	if repoRoot == "" || d == nil {
		return nil
	}

	var reports []BrokenAnchorReport
	for _, e := range d.Entries {
		if e.Retired || e.SupersededBy != "" || len(e.Anchors) == 0 {
			continue
		}

		// --- Step 2: Partition each active entry's anchors into valid and missing ---
		var missing []string
		var valid []string

		for _, rawAnchor := range e.Anchors {
			trimmed := strings.TrimSpace(rawAnchor)
			if trimmed == "" {
				continue
			}
			normPath := StripAnchorSymbol(trimmed)
			if normPath == "" {
				// Pure symbol anchor without a file path.
				valid = append(valid, trimmed)
				continue
			}

			checks := inbox.CheckAnchors(repoRoot, []string{trimmed})
			exists := len(checks) > 0 && checks[0].Exists
			if exists {
				if sym := extractAnchorSymbol(trimmed); sym != "" {
					fullPath := filepath.Join(repoRoot, filepath.FromSlash(normPath))
					info, err := os.Stat(fullPath)
					if err != nil || !info.Mode().IsRegular() {
						exists = false
					} else if data, err := os.ReadFile(fullPath); err != nil || !strings.Contains(string(data), sym) {
						exists = false
					}
				}
			}

			if exists {
				valid = append(valid, trimmed)
			} else {
				missing = append(missing, trimmed)
			}
		}

		// --- Step 3: Record a BrokenAnchorReport when any anchor is missing ---
		if len(missing) > 0 {
			scope := d.Scope
			if scope == "" {
				scope = e.DossierScope
			}
			reports = append(reports, BrokenAnchorReport{
				EntryID:        e.ID,
				DossierScope:   scope,
				EntryType:      e.Type,
				Title:          ExtractEntryTitle(e.Text),
				MissingAnchors: missing,
				ValidAnchors:   valid,
				AllMissing:     len(valid) == 0,
			})
		}
	}

	return reports
}
