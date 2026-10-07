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

// sanitizeFTSQuery converts raw prompt terms into a safe FTS5 query.
// Each term is stripped of non-alphanumeric characters, quoted, and joined with OR.
func sanitizeFTSQuery(terms []string) string {
	var clean []string
	for _, term := range terms {
		// Strip non-word characters
		stripped := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				return r
			}
			return -1
		}, term)
		if len(stripped) < 3 {
			continue
		}
		// Double any embedded quotes and wrap in quotes for FTS5
		escaped := strings.ReplaceAll(stripped, `"`, `""`)
		clean = append(clean, `"`+escaped+`"`)
	}
	if len(clean) == 0 {
		return ""
	}
	return strings.Join(clean, " OR ")
}

func (e *Engine) TaskStartEntries(ctx context.Context, promptTerms []string, diffPaths []string) ([]RankedEntry, error) {
	entryMap := make(map[string]dossiers.Entry)

	// Query by prompt terms — search errors do NOT discard diff-path results.
	if len(promptTerms) > 0 {
		query := sanitizeFTSQuery(promptTerms)
		if query != "" {
			results, err := e.index.SearchEntries(ctx, query)
			if err == nil {
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

// isGateEntry detects whether an entry is marked as a gate.
// Uses case-insensitive matching for the structured "gate:" tag.
func isGateEntry(ent *dossiers.Entry) bool {
	gatePattern := regexp.MustCompile(`(?i)\bgate:\s*edit\b`)
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
	// Fall back to prefix match if exact yields nothing
	if len(ids) == 0 {
		ids, err = e.index.LookupByAnchor(ctx, targetFile)
		if err != nil {
			return nil, err
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
