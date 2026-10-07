package delivery

import (
	"context"
	"strings"

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
			ent.Anchors = verified // Keep only verified anchors? "whose anchors are all suspect (verified none)". Maybe we don't mutate, just check.
		}
		valid = append(valid, ent)
	}
	return valid
}

func (e *Engine) TaskStartEntries(ctx context.Context, promptTerms []string, diffPaths []string) ([]RankedEntry, error) {
	entryMap := make(map[string]dossiers.Entry)

	// query by prompt terms
	if len(promptTerms) > 0 {
		query := strings.Join(promptTerms, " ")
		results, err := e.index.SearchEntries(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, ent := range results {
			entryMap[ent.ID] = ent
		}
	}

	// query by diff paths
	for _, path := range diffPaths {
		ids, err := e.index.LookupByAnchor(ctx, path)
		if err != nil {
			return nil, err
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

func (e *Engine) ErrorEntries(ctx context.Context, commandPrefix string, output string) ([]RankedEntry, error) {
	results, err := e.index.LookupErrorPatterns(ctx, commandPrefix)
	if err != nil {
		return nil, err
	}
	
	var entries []dossiers.Entry
	for _, res := range results {
		ent, err := e.index.GetEntry(ctx, res.EntryID)
		if err == nil && ent != nil {
			entries = append(entries, *ent)
		}
	}
	
	validEntries := e.filterAndVerify(entries)
	ranked := RankEntries(validEntries, nil)
	return SelectWithinBudget(ranked, e.budgets[MomentError]), nil
}

func (e *Engine) GateEntries(ctx context.Context, targetFile string, ledger *Ledger) ([]RankedEntry, error) {
	ids, err := e.index.LookupByAnchor(ctx, targetFile)
	if err != nil {
		return nil, err
	}
	
	var entries []dossiers.Entry
	for _, id := range ids {
		if ledger != nil && ledger.IsGateAcknowledged(id) {
			continue
		}
		
		ent, err := e.index.GetEntry(ctx, id)
		if err == nil && ent != nil {
			if strings.Contains(ent.Text, "gate: edit") || strings.Contains(ent.ClosesWhen, "gate: edit") || strings.Contains(ent.Verified, "gate: edit") {
				entries = append(entries, *ent)
			}
		}
	}
	
	validEntries := e.filterAndVerify(entries)
	ranked := RankEntries(validEntries, []string{targetFile})
	return SelectWithinBudget(ranked, e.budgets[MomentGate]), nil
}
