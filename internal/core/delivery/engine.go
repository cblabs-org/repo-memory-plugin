package delivery

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

type Engine struct {
	index         *index.Index
	projector     *dossiers.Projector
	scopeResolver *config.ScopeResolver
	workspaceRoot string
	budgets       map[Moment]int
}

type EngineOption func(*Engine)

func WithBudgets(budgets map[Moment]int) EngineOption {
	return func(e *Engine) {
		e.budgets = budgets
	}
}

func WithScopeResolver(resolver *config.ScopeResolver) EngineOption {
	return func(e *Engine) {
		e.scopeResolver = resolver
	}
}

func NewEngine(idx *index.Index, workspaceRoot string, opts ...EngineOption) *Engine {
	e := &Engine{
		index:         idx,
		workspaceRoot: workspaceRoot,
		budgets:       DefaultBudgets(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) filterAndVerify(entries []dossiers.Entry) []dossiers.Entry {
	var valid []dossiers.Entry
	for _, ent := range entries {
		if ent.Retired {
			continue
		}
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

// stopWords contains common English words that match nearly everything in FTS5.
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
}

// sanitizeFTSQuery converts raw prompt terms into a safe FTS5 query.
// Terms are split on non-word characters (so "auth.go" → "auth", "go"),
// stop words are filtered, and remaining terms are quoted and joined with OR.
func sanitizeFTSQuery(terms []string) string {
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
	if len(clean) == 0 {
		return ""
	}
	return strings.Join(clean, " OR ")
}

// maxFTSResults caps FTS search results to preserve bm25 ranking relevance.
const maxFTSResults = 20

func (e *Engine) TaskStartEntries(ctx context.Context, promptTerms []string, diffPaths []string) ([]RankedEntry, error) {
	entryMap := make(map[string]dossiers.Entry)

	// Query by prompt terms — search errors do NOT discard diff-path results.
	if len(promptTerms) > 0 {
		query := sanitizeFTSQuery(promptTerms)
		if query != "" {
			results, err := e.index.SearchEntries(ctx, query)
			if err == nil {
				// Cap to top N by bm25 to keep only the most relevant.
				if len(results) > maxFTSResults {
					results = results[:maxFTSResults]
				}
				for _, ent := range results {
					entryMap[ent.ID] = ent
				}
			}
			// On error, log/ignore — diff-path results still proceed.
		}
	}

	// Query by diff paths (these are repo-relative, matching anchor format).
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

	var entries []dossiers.Entry
	for _, ent := range entryMap {
		entries = append(entries, ent)
	}

	validEntries := e.filterAndVerify(entries)
	ranked := RankEntries(validEntries, diffPaths)
	return SelectWithinBudget(ranked, e.budgets[MomentTaskStart]), nil
}

func (e *Engine) DossierEntries(ctx context.Context, scope string) ([]RankedEntry, error) {
	entries, err := e.index.ListEntriesByScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	
	var valid []dossiers.Entry
	for _, ent := range entries {
		if ent.Type == dossiers.EntryTypeInvariant || ent.Type == dossiers.EntryTypeFailureSignature || ent.Type == dossiers.EntryTypeRuledOut {
			valid = append(valid, ent)
		}
	}
	
	validEntries := e.filterAndVerify(valid)
	ranked := RankEntries(validEntries, nil)
	return SelectWithinBudget(ranked, e.budgets[MomentFirstEntry]), nil
}

func (e *Engine) AnchorEntries(ctx context.Context, anchor string) ([]RankedEntry, error) {
	if !VerifyAnchor(anchor, e.workspaceRoot) {
		return nil, nil
	}
	
	ids, err := e.index.LookupByAnchor(ctx, anchor)
	if err != nil {
		return nil, err
	}
	
	var entries []dossiers.Entry
	for _, id := range ids {
		ent, err := e.index.GetEntry(ctx, id)
		if err == nil && ent != nil {
			entries = append(entries, *ent)
		}
	}
	
	validEntries := e.filterAndVerify(entries)
	ranked := RankEntries(validEntries, []string{anchor})
	return SelectWithinBudget(ranked, e.budgets[MomentFirstTouch]), nil
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

func (e *Engine) ErrorEntries(ctx context.Context, commandPrefix string, output string) ([]RankedEntry, error) {
	results, err := e.index.LookupErrorPatterns(ctx, commandPrefix)
	if err != nil {
		return nil, err
	}
	
	var entries []dossiers.Entry
	for _, res := range results {
		// Filter: pattern must match the actual output
		if !patternMatchesOutput(res.Pattern, output) {
			continue
		}
		ent, err := e.index.GetEntry(ctx, res.EntryID)
		if err == nil && ent != nil {
			entries = append(entries, *ent)
		}
	}
	
	validEntries := e.filterAndVerify(entries)
	ranked := RankEntries(validEntries, nil)
	return SelectWithinBudget(ranked, e.budgets[MomentError]), nil
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

func (e *Engine) GateEntries(ctx context.Context, targetFile string, ledger *Ledger) ([]RankedEntry, error) {
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
	
	validEntries := e.filterAndVerify(entries)
	ranked := RankEntries(validEntries, []string{targetFile})
	return SelectWithinBudget(ranked, e.budgets[MomentGate]), nil
}
