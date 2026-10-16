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
	TierTemp    ScopeTier = "temp"
	TierSession ScopeTier = "session"
	TierUser    ScopeTier = "user"
	TierBranch  ScopeTier = "branch"
	TierRepo    ScopeTier = "repo"
)

// StripTierPrefix inspects rawScope case-insensitively for a known tier prefix
// ("temp:", "session:", "user:", "branch:", or "repo:"). If present, it returns
// the matched ScopeTier and the trimmed remainder; otherwise ("", trimmed rawScope).
func StripTierPrefix(rawScope string) (ScopeTier, string) {
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
	// 1. Explicit prefix on item.Scope.
	prefixTier, _ := StripTierPrefix(item.Scope)
	switch prefixTier {
	case TierUser, TierBranch, TierRepo, TierSession, TierTemp:
		return prefixTier
	}

	// 2. Personal/Workstation heuristic (TierUser).
	trimmedScope := strings.TrimSpace(item.Scope)
	lowerScope := strings.ToLower(trimmedScope)
	if strings.HasPrefix(lowerScope, "user-dossiers") || strings.EqualFold(trimmedScope, "user") {
		return TierUser
	}

	// 3. Unmerged Branch heuristic (TierBranch).
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

	// 4. Default (TierRepo).
	return TierRepo
}

func hasBranchOnlyAnchorChecks(note *inbox.Note, item inbox.NoteItem, fileAnchors []string) bool {
	checks := make([]inbox.AnchorCheck, 0, len(item.AnchorChecks))
	checks = append(checks, item.AnchorChecks...)
	if note != nil {
		checks = append(checks, note.AnchorChecks...)
	}
	if len(checks) == 0 {
		return false
	}

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
	NoteID         string               `json:"note_id"`
	NoteSource     inbox.NoteSource     `json:"note_source"`
	ConversationID string               `json:"conversation_id,omitempty"`
	Branch         string               `json:"branch,omitempty"`
	HeadCommit     string               `json:"head_commit,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	Tier           ScopeTier            `json:"tier"`
	DossierScope   string               `json:"dossier_scope"`
	DossierKind    dossiers.DossierKind `json:"dossier_kind"`
	DerivedNew     bool                 `json:"derived_new,omitempty"`
	Item           inbox.NoteItem       `json:"item"`
}

// RouteItem deterministically classifies an item's tier and routes it to a target dossier scope and kind
// (SPEC §6.5 step 2, design-v2 §6 step 2).
func RouteItem(note *inbox.Note, item inbox.NoteItem, resolver *config.ScopeResolver, repoRoot string, isBranchMerged func(branch string) bool) RoutedItem {
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

	// Case A — Explicit scope on item.Scope.
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

	// Case B — Anchored item (len(item.Anchors) > 0).
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

	// Case C — Unanchored item (or still empty dossierScope).
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

func extractRoutingKeywords(item inbox.NoteItem) []string {
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

	// Preserve full command phrases first so multi-word commands (e.g. "git tag", "make release") match.
	for _, cmd := range item.Commands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed != "" {
			addKw(trimmed)
		}
	}

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
	Remaining []RoutedItem
	AmendOps  []ops.Op
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

	// 1. Deduplicate candidate items against existing active entries in dossiersMap.
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

	// 2. Coalesce exact duplicates within the remaining candidate batch.
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

func mergeEvidenceSlice(base []string, noteID string, itemEvidence string) []string {
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
	EntryID        string             `json:"entry_id"`
	DossierScope   string             `json:"dossier_scope"`
	EntryType      dossiers.EntryType `json:"entry_type"`
	Title          string             `json:"title"`
	MissingAnchors []string           `json:"missing_anchors"`
	ValidAnchors   []string           `json:"valid_anchors"`
	AllMissing     bool               `json:"all_missing"`
}

// CheckDossierAnchors inspects all active entries in d against repoRoot and returns a report
// for each entry that has at least one missing file or symbol anchor (SPEC §6.5 step 3, design-v2 §6 step 4).
func CheckDossierAnchors(repoRoot string, d *dossiers.Dossier) []BrokenAnchorReport {
	if repoRoot == "" || d == nil {
		return nil
	}

	var reports []BrokenAnchorReport
	for _, e := range d.Entries {
		if e.Retired || e.SupersededBy != "" || len(e.Anchors) == 0 {
			continue
		}

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
