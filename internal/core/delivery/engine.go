/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/delivery/engine.go
 * Description: Implements the push-delivery selection engine across task-start,
 *              first-entry (dossier scope), first-touch (file/symbol anchor),
 *              command-error, and pre-edit gate moments.
 *
 * Exported Types / Functions:
 * - Engine:             Selects, verifies, ranks, and deduplicates dossier entries for push delivery.
 *   - NewEngine():        Constructs a push-delivery Engine backed by an Index and workspace root.
 *   - InjectedSet():      Returns the set of entry IDs marked as injected in the current session.
 *   - IsInjected():       Reports whether an entry ID has already been injected.
 *   - MarkInjected():     Marks an entry ID as injected in the current session.
 *   - ResetInjected():    Clears the per-session injected entry set.
 *   - TaskStartEntries(): Selects push-delivery entries for MomentTaskStart from prompt terms and git diffs.
 *   - DossierEntries():   Selects push-delivery entries for MomentFirstEntry when entering a module scope.
 *   - AnchorEntries():    Selects push-delivery entries for MomentFirstTouch when touching a file/symbol anchor.
 *   - ErrorEntries():     Selects push-delivery entries for MomentError when a command fails.
 *   - GateEntries():      Selects unacknowledged `gate: edit` entries anchored to a target file.
 * - EngineOption:       Functional option for configuring an Engine instance.
 * - WithBudgets():      Overrides per-moment token budgets on an Engine.
 * - WithScopeResolver(): Sets the module scope resolver on an Engine.
 * - WithMinScore():     Overrides the minimum relevance score threshold on an Engine.
 * - WithInjectedSet():  Shares an existing per-conversation injected-entry set with an Engine.
 * - IsStopWord():       Reports whether a token is a common stopword excluded from FTS5 prompt queries.
 */

package delivery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

// Engine selects, verifies, ranks, and deduplicates dossier entries for push delivery.
type Engine struct {
	// index is the SQLite FTS5/anchor index backing entry queries.
	index *index.Index
	// projector is the optional dossier projector.
	projector *dossiers.Projector
	// scopeResolver resolves file paths to dossier scopes.
	scopeResolver *config.ScopeResolver
	// workspaceRoot is the absolute path to the repository root used for anchor verification.
	workspaceRoot string
	// budgets maps each delivery Moment to its maximum token budget.
	budgets map[Moment]int
	// minScore is the minimum composite score required for push delivery.
	minScore int
	// injectedSet tracks entry IDs already injected in the active conversation.
	injectedSet map[string]bool
}

// EngineOption configures an Engine instance.
type EngineOption func(*Engine)

// WithBudgets overrides the per-moment token budgets on an Engine.
func WithBudgets(budgets map[Moment]int) EngineOption {
	return func(e *Engine) {
		e.budgets = budgets
	}
}

// WithScopeResolver sets the scope resolver on an Engine.
func WithScopeResolver(resolver *config.ScopeResolver) EngineOption {
	return func(e *Engine) {
		e.scopeResolver = resolver
	}
}

// WithMinScore overrides the minimum relevance score threshold on an Engine.
func WithMinScore(minScore int) EngineOption {
	return func(e *Engine) {
		e.minScore = minScore
	}
}

// WithInjectedSet shares an existing per-conversation injected-entry set with the Engine.
func WithInjectedSet(set map[string]bool) EngineOption {
	return func(e *Engine) {
		e.injectedSet = set
	}
}

// NewEngine constructs a push-delivery Engine backed by idx and rooted at workspaceRoot.
func NewEngine(idx *index.Index, workspaceRoot string, opts ...EngineOption) *Engine {
	// --- Step 1: Initialize Engine with default budgets and relevance threshold ---
	e := &Engine{
		index:         idx,
		workspaceRoot: workspaceRoot,
		budgets:       DefaultBudgets(),
		minScore:      DefaultMinRelevanceScore,
		injectedSet:   make(map[string]bool),
	}

	// --- Step 2: Apply functional options and ensure non-nil injectedSet ---
	for _, opt := range opts {
		opt(e)
	}
	if e.injectedSet == nil {
		e.injectedSet = make(map[string]bool)
	}
	return e
}

// InjectedSet returns the set of entry IDs marked as injected in this session.
func (e *Engine) InjectedSet() map[string]bool {
	return e.injectedSet
}

// IsInjected reports whether entry id has already been injected in this session.
func (e *Engine) IsInjected(id string) bool {
	return e.injectedSet != nil && e.injectedSet[id]
}

// MarkInjected records entry id as injected in this session.
func (e *Engine) MarkInjected(id string) {
	if e.injectedSet == nil {
		e.injectedSet = make(map[string]bool)
	}
	e.injectedSet[id] = true
}

// ResetInjected clears the per-session injected entry set.
func (e *Engine) ResetInjected() {
	e.injectedSet = make(map[string]bool)
}

// filterUninjected returns only entries that have not yet been injected in this session.
func (e *Engine) filterUninjected(entries []dossiers.Entry) []dossiers.Entry {
	var uninjected []dossiers.Entry
	for _, ent := range entries {
		if !e.IsInjected(ent.ID) {
			uninjected = append(uninjected, ent)
		}
	}
	return uninjected
}

// selectAndRecord selects entries within moment's token budget and marks them injected.
func (e *Engine) selectAndRecord(ranked []RankedEntry, moment Moment) []RankedEntry {
	selected := SelectWithinBudget(ranked, e.budgets[moment], e.minScore)
	for _, s := range selected {
		e.MarkInjected(s.Entry.ID)
	}
	return selected
}

// filterAndVerify removes retired entries, pull-only open issues, and entries whose anchors all fail verification.
func (e *Engine) filterAndVerify(entries []dossiers.Entry) []dossiers.Entry {
	// --- Step 1: Filter retired and pull-only open-issue entries ---
	var valid []dossiers.Entry
	for _, ent := range entries {
		if ent.Retired {
			continue
		}
		if ent.Type == dossiers.EntryTypeOpenIssue && strings.TrimSpace(ent.ClosesWhen) == "" {
			continue
		}
		// --- Step 2: Verify file/symbol anchors against the workspace ---
		if len(ent.Anchors) > 0 {
			verified, _ := VerifyAnchors(ent.Anchors, e.workspaceRoot)
			if len(verified) == 0 {
				continue // skip if all suspect
			}
			ent.Anchors = verified
		}
		valid = append(valid, ent)
	}
	return valid
}

// stopWords contains common English words and generic markup/URL tokens that match nearly everything in FTS5.
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true,
	"not": true, "you": true, "all": true, "can": true, "her": true,
	"was": true, "one": true, "our": true, "out": true, "has": true,
	"his": true, "how": true, "its": true, "may": true, "new": true,
	"now": true, "old": true, "see": true, "way": true, "who": true,
	"did": true, "get": true, "let": true, "say": true, "she": true,
	"too": true, "use": true, "fix": true, "any": true, "had": true,
	"that": true, "this": true, "with": true, "have": true, "from": true,
	"they": true, "been": true, "will": true, "what": true, "when": true,
	"your": true, "each": true, "make": true, "like": true, "just": true,
	"into": true, "them": true, "than": true, "then": true, "some": true,
	"also": true, "made": true, "after": true, "should": true, "would": true,
	"could": true, "about": true, "which": true, "their": true, "there": true,
	"write": true, "build": true, "using": true, "once": true, "per": true,
	"between": true, "need": true, "added": true, "show": true, "top": true,
	"level": true, "hour": true, "sec": true, "mins": true, "http": true,
	"https": true, "com": true, "org": true, "net": true, "www": true,
	"io": true, "dev": true, "json": true, "xml": true, "html": true,
	"public": true, "user_request": true,
}

// IsStopWord reports whether word (case-insensitive) is a common English or
// markup/URL stopword that should be excluded from FTS5 prompt queries.
func IsStopWord(word string) bool {
	return stopWords[strings.ToLower(word)]
}

// sanitizeFTSQuery converts raw prompt terms into a safe FTS5 query.
// Terms are split on non-word characters (so "auth.go" → "auth", "go"),
// stop words are filtered, and remaining terms are quoted and joined with OR.
func sanitizeFTSQuery(terms []string) string {
	// --- Step 1: Split terms on non-word characters and filter stopwords ---
	seen := make(map[string]bool)
	var clean []string
	for _, term := range terms {
		// Split each term on non-word characters to handle "auth.go", "it's", etc.
		parts := strings.FieldsFunc(term, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
		})
		for _, part := range parts {
			lower := strings.ToLower(part)
			if len(lower) < 3 || stopWords[lower] || seen[lower] {
				continue
			}
			seen[lower] = true
			escaped := strings.ReplaceAll(part, `"`, `""`)
			clean = append(clean, `"`+escaped+`"`)
		}
	}
	// --- Step 2: Join quoted terms with OR ---
	if len(clean) == 0 {
		return ""
	}
	return strings.Join(clean, " OR ")
}

// maxFTSResults caps FTS search results to preserve bm25 ranking relevance.
const maxFTSResults = 20

// TaskStartEntries selects push-delivery entries for MomentTaskStart by combining
// FTS5 prompt-term matches with git diff anchor matches.
func (e *Engine) TaskStartEntries(ctx context.Context, promptTerms []string, diffPaths []string) ([]RankedEntry, error) {
	// --- Step 1: Query FTS5 index by sanitized prompt terms ---
	entryMap := make(map[string]dossiers.Entry)
	ftsScores := make(map[string]float64)

	if len(promptTerms) > 0 {
		query := sanitizeFTSQuery(promptTerms)
		if query != "" {
			results, err := e.index.SearchEntriesWithScore(ctx, query)
			if err != nil {
				return nil, fmt.Errorf("FTS search: %w", err)
			}
			// Cap to top N by bm25 to keep only the most relevant.
			if len(results) > maxFTSResults {
				results = results[:maxFTSResults]
			}
			for _, res := range results {
				entryMap[res.Entry.ID] = res.Entry
				ftsScores[res.Entry.ID] = res.Score
			}
		}
	}

	// --- Step 2: Query index by recent git diff file paths ---
	for _, path := range diffPaths {
		ids, err := e.index.LookupByAnchor(ctx, path)
		if err != nil {
			continue // don't let one bad path kill the whole lookup
		}
		for _, id := range ids {
			if _, exists := entryMap[id]; !exists {
				ent, err := e.index.GetEntry(ctx, id)
				if err == nil && ent != nil {
					entryMap[id] = *ent
				}
			}
		}
	}

	// --- Step 3: Filter, verify, rank, and budget candidate entries ---
	var entries []dossiers.Entry
	for _, ent := range entryMap {
		entries = append(entries, ent)
	}

	validEntries := e.filterUninjected(e.filterAndVerify(entries))
	ranked := RankEntries(validEntries, diffPaths, ftsScores)
	return e.selectAndRecord(ranked, MomentTaskStart), nil
}

// isPushEligibleForScopeOrTouch reports whether an entry type is eligible for
// automatic mid-turn push delivery on MomentFirstEntry or MomentFirstTouch.
// Historical `decision` entries are reserved for TaskStart FTS/diff queries and
// `memory_why` pull lookups so mid-turn file reads do not flood context with
// implementation diary rationale.
func isPushEligibleForScopeOrTouch(t dossiers.EntryType) bool {
	switch t {
	case dossiers.EntryTypeInvariant,
		dossiers.EntryTypeContract,
		dossiers.EntryTypeConvention,
		dossiers.EntryTypeGotcha,
		dossiers.EntryTypeExternalBehaviour,
		dossiers.EntryTypeFailureSignature,
		dossiers.EntryTypeRuledOut,
		dossiers.EntryTypeRecipe,
		dossiers.EntryTypeProcedure,
		dossiers.EntryTypeOpenIssue:
		return true
	default:
		return false
	}
}

// DossierEntries selects push-delivery entries for MomentFirstEntry when an
// agent first enters a module scope.
func (e *Engine) DossierEntries(ctx context.Context, scope string) ([]RankedEntry, error) {
	// --- Step 1: List all entries in the target scope ---
	entries, err := e.index.ListEntriesByScope(ctx, scope)
	if err != nil {
		return nil, err
	}

	// --- Step 2: Filter to actionable semantic, procedural, and negative-knowledge types ---
	var valid []dossiers.Entry
	for _, ent := range entries {
		if isPushEligibleForScopeOrTouch(ent.Type) && ent.Type != dossiers.EntryTypeOpenIssue {
			valid = append(valid, ent)
		}
	}

	// --- Step 3: Verify anchors, deduplicate, rank, and select within MomentFirstEntry budget ---
	validEntries := e.filterUninjected(e.filterAndVerify(valid))
	ranked := RankEntries(validEntries, nil)
	return e.selectAndRecord(ranked, MomentFirstEntry), nil
}

// AnchorEntries selects push-delivery entries for MomentFirstTouch when an
// agent first touches a file or symbol anchor.
func (e *Engine) AnchorEntries(ctx context.Context, anchor string) ([]RankedEntry, error) {
	// --- Step 1: Verify the touched anchor exists in the workspace ---
	if !VerifyAnchor(anchor, e.workspaceRoot) {
		return nil, nil
	}

	// --- Step 2: Look up entries matching the anchor and filter out diary-only decisions ---
	ids, err := e.index.LookupByAnchor(ctx, anchor)
	if err != nil {
		return nil, err
	}

	var entries []dossiers.Entry
	for _, id := range ids {
		ent, err := e.index.GetEntry(ctx, id)
		if err == nil && ent != nil && isPushEligibleForScopeOrTouch(ent.Type) {
			entries = append(entries, *ent)
		}
	}

	// --- Step 3: Verify anchors, deduplicate, rank, and select within MomentFirstTouch budget ---
	validEntries := e.filterUninjected(e.filterAndVerify(entries))
	ranked := RankEntries(validEntries, []string{anchor})
	return e.selectAndRecord(ranked, MomentFirstTouch), nil
}

// patternMatchesOutput returns true if the pattern matches the tool output.
// Tries regexp first; falls back to substring match.
func patternMatchesOutput(pattern, output string) bool {
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re.MatchString(output)
	}
	// Fallback: case-insensitive substring match
	return strings.Contains(strings.ToLower(output), strings.ToLower(pattern))
}

// ErrorEntries selects push-delivery entries for MomentError when a command fails.
func (e *Engine) ErrorEntries(ctx context.Context, commandPrefix string, output string) ([]RankedEntry, error) {
	// --- Step 1: Look up error patterns registered for commandPrefix ---
	results, err := e.index.LookupErrorPatterns(ctx, commandPrefix)
	if err != nil {
		return nil, err
	}

	// --- Step 2: Filter to patterns matching the actual command output ---
	var entries []dossiers.Entry
	for _, res := range results {
		if !patternMatchesOutput(res.Pattern, output) {
			continue
		}
		ent, err := e.index.GetEntry(ctx, res.EntryID)
		if err == nil && ent != nil {
			entries = append(entries, *ent)
		}
	}

	// --- Step 3: Verify, deduplicate, rank, and select within MomentError budget ---
	validEntries := e.filterUninjected(e.filterAndVerify(entries))
	ranked := RankEntries(validEntries, nil)
	return e.selectAndRecord(ranked, MomentError), nil
}

// gatePattern is compiled once at package level for efficiency.
var gatePattern = regexp.MustCompile(`(?i)\bgate:\s*edit\b`)

// isGateEntry detects whether an entry is marked as a gate.
// Uses case-insensitive matching for the structured "gate:" tag.
func isGateEntry(ent *dossiers.Entry) bool {
	return gatePattern.MatchString(ent.Text) ||
		gatePattern.MatchString(ent.ClosesWhen) ||
		gatePattern.MatchString(ent.Verified)
}

// GateEntries selects unacknowledged `gate: edit` entries anchored to targetFile.
func (e *Engine) GateEntries(ctx context.Context, targetFile string, ledger *Ledger) ([]RankedEntry, error) {
	// --- Step 1: Look up exact file or symbol anchors ---
	// Use exact anchor match for gates to avoid LIKE wildcards matching unrelated files.
	ids, err := e.index.LookupByAnchorExact(ctx, targetFile)
	if err != nil {
		return nil, err
	}
	// Fall back to symbol-style anchors (e.g. "file.go:FuncName") only.
	// LookupByAnchor uses LIKE 'target:%', so "file.go:" matches
	// "file.go:FuncName" but not "file.go.orig".
	if len(ids) == 0 {
		symbolIDs, err := e.index.LookupByAnchor(ctx, targetFile+":")
		if err == nil {
			ids = symbolIDs
		}
	}

	// --- Step 2: Filter to unacknowledged gate entries ---
	var entries []dossiers.Entry
	for _, id := range ids {
		if ledger != nil && ledger.IsGateAcknowledged(id) {
			continue
		}

		ent, err := e.index.GetEntry(ctx, id)
		if err == nil && ent != nil {
			if isGateEntry(ent) {
				entries = append(entries, *ent)
			}
		}
	}

	// --- Step 3: Verify, rank, and select within MomentGate budget ---
	validEntries := e.filterAndVerify(entries)
	ranked := RankEntries(validEntries, []string{targetFile})
	return SelectWithinBudget(ranked, e.budgets[MomentGate], e.minScore), nil
}
