/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/gardening.go
 * Description: Deterministic dossier gardening (SPEC §6.3.6). Cleans up broken
 *              anchors, merges near-duplicate entries within each dossier scope,
 *              splits oversized dossiers exceeding token caps, reconciles doc-seed
 *              freshness, and rebuilds the multi-tier SQLite search index.
 *
 * Exported Types / Functions:
 * - FreshnessResult:                  Alias for seed.DocSeedResult returned by CheckDocFreshness.
 * - GardeningOptions:                 Configuration parameters for a dossier gardening pass.
 * - GardeningResult:                  Summary of operations emitted and applied by RunGardening.
 * - PipelineExecutor.SetIndex():      Attaches a SQLite search index handle to the executor.
 * - PipelineExecutor.RunGardening():  Executes the 4-step deterministic dossier gardening pass.
 */

package tick

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

// FreshnessResult is an alias for seed.DocSeedResult returned by CheckDocFreshness.
type FreshnessResult = seed.DocSeedResult

// GardeningOptions configures a dossier gardening pass.
type GardeningOptions struct {
	DryRun           bool `json:"dry_run"`                      // When true, computes gardening ops without mutating disk or index.
	MaxDossierTokens int  `json:"max_dossier_tokens,omitempty"` // Token cap per dossier before splitting (defaults to 3000).
}

// GardeningResult summarizes all operations generated and applied by RunGardening.
type GardeningResult struct {
	RetiredBrokenAnchors []string         `json:"retired_broken_anchors,omitempty"` // Entry IDs retired because all anchors were missing.
	PrunedAnchorEntries  []string         `json:"pruned_anchor_entries,omitempty"`  // Entry IDs amended to prune partially missing anchors.
	MergedDuplicates     []string         `json:"merged_duplicates,omitempty"`      // Duplicate entry IDs merged and retired.
	SplitDossiers        []string         `json:"split_dossiers,omitempty"`         // Dossier scopes split due to exceeding MaxDossierTokens.
	OpsEmitted           int              `json:"ops_emitted"`                      // Total gardening operations generated.
	Ops                  []ops.Op         `json:"ops,omitempty"`                    // Ordered slice of gardening operations emitted.
	Freshness            *FreshnessResult `json:"freshness,omitempty"`              // Doc-freshness reconciliation result (nil in dry-run mode).
}

// SetIndex sets the SQLite index instance on the PipelineExecutor.
func (p *PipelineExecutor) SetIndex(idx *index.Index) {
	p.idx = idx
}

// RunGardening executes the 4-step deterministic dossier gardening pass:
//  1. Broken-anchor cleanup (retiring non-episodic entries with all anchors missing; pruning partial missing anchors).
//  2. Near-duplicate merging within each dossier (identical normalized title or token Jaccard >= 0.75 with overlapping anchors).
//  3. Dossier cap split (> MaxDossierTokens, defaulting to cfg.Budgets.DossierCapTokens or 3000).
//  4. Write & re-project (when !opts.DryRun), doc freshness check, and multi-tier index rebuild.
func (p *PipelineExecutor) RunGardening(ctx context.Context, opts GardeningOptions) (*GardeningResult, error) {
	// --- Step 0: Resolve configuration defaults and project existing dossiers ---
	if ctx == nil {
		ctx = context.Background()
	}

	res := &GardeningResult{}
	if strings.TrimSpace(p.repoRoot) == "" {
		return res, nil
	}

	maxTokens := opts.MaxDossierTokens
	if maxTokens <= 0 {
		maxTokens = p.cfg.Budgets.DossierCapTokens
	}
	if maxTokens <= 0 {
		maxTokens = 3000
	}

	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}

	opsDir := filepath.Join(p.repoRoot, ".memory", "ops")
	projected := make(map[string]*dossiers.Dossier)
	if info, err := os.Stat(opsDir); err == nil && info.IsDir() {
		var projErr error
		projected, projErr = dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
		if projErr != nil {
			return nil, fmt.Errorf("projecting dossiers for gardening: %w", projErr)
		}
	}

	scopes := make([]string, 0, len(projected))
	for s := range projected {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)

	retiredIDs := make(map[string]bool)
	prunedAnchorsByID := make(map[string][]string)

	// --- Step 1: Broken-anchor cleanup across all active entries ---
	for _, scope := range scopes {
		d := projected[scope]
		if d == nil || len(d.Entries) == 0 {
			continue
		}
		targetScope := strings.TrimSpace(d.Scope)
		if targetScope == "" {
			targetScope = scope
		}

		reports := consolidate.CheckDossierAnchors(p.repoRoot, d)
		reportByID := make(map[string]consolidate.BrokenAnchorReport, len(reports))
		for _, r := range reports {
			reportByID[r.EntryID] = r
		}

		for _, e := range d.Entries {
			if e.Retired || e.SupersededBy != "" {
				continue
			}
			rep, ok := reportByID[e.ID]
			if !ok {
				continue
			}
			if rep.AllMissing && e.EffectiveKind() != dossiers.CognitiveKindEpisodic {
				reason := "gardening: all anchors missing (" + strings.Join(rep.MissingAnchors, ", ") + ")"
				res.Ops = append(res.Ops, ops.Op{
					Type:         ops.OpRetire,
					EntryID:      e.ID,
					DossierScope: targetScope,
					Timestamp:    now,
					Source:       ops.SourceGardening,
					Fields: map[string]any{
						"reason": reason,
					},
				})
				res.RetiredBrokenAnchors = append(res.RetiredBrokenAnchors, e.ID)
				retiredIDs[e.ID] = true
			} else if len(rep.ValidAnchors) > 0 && len(rep.MissingAnchors) > 0 {
				validCopy := append([]string(nil), rep.ValidAnchors...)
				res.Ops = append(res.Ops, ops.Op{
					Type:         ops.OpAmend,
					EntryID:      e.ID,
					DossierScope: targetScope,
					Timestamp:    now,
					Source:       ops.SourceGardening,
					Fields: map[string]any{
						"anchors": validCopy,
					},
				})
				res.PrunedAnchorEntries = append(res.PrunedAnchorEntries, e.ID)
				prunedAnchorsByID[e.ID] = validCopy
			}
		}
	}

	// --- Step 2: Merge near-duplicate entries within each dossier ---
	survivingByScope := make(map[string][]dossiers.Entry, len(scopes))
	for _, scope := range scopes {
		d := projected[scope]
		if d == nil || len(d.Entries) == 0 {
			continue
		}
		targetScope := strings.TrimSpace(d.Scope)
		if targetScope == "" {
			targetScope = scope
		}

		var active []dossiers.Entry
		for _, e := range d.Entries {
			if e.Retired || e.SupersededBy != "" || retiredIDs[e.ID] {
				continue
			}
			if pruned, ok := prunedAnchorsByID[e.ID]; ok {
				e.Anchors = append([]string(nil), pruned...)
			}
			active = append(active, e)
		}

		// Order by CreatedAt (earlier first), then ID for canonical survivor selection.
		sort.SliceStable(active, func(i, j int) bool {
			ti, tj := active[i].CreatedAt, active[j].CreatedAt
			if !ti.Equal(tj) && !ti.IsZero() && !tj.IsZero() {
				return ti.Before(tj)
			}
			return active[i].ID < active[j].ID
		})

		for i := 0; i < len(active); i++ {
			if retiredIDs[active[i].ID] {
				continue
			}
			survivor := &active[i]
			mergedAny := false

			for j := i + 1; j < len(active); j++ {
				cand := active[j]
				if retiredIDs[cand.ID] {
					continue
				}
				if !areEntriesNearDuplicates(*survivor, cand) {
					continue
				}

				survivor.Anchors = mergeUniqueStrings(survivor.Anchors, cand.Anchors)
				survivor.Evidence = mergeUniqueStrings(survivor.Evidence, cand.Evidence)
				if strings.TrimSpace(cand.Verified) != "" {
					if strings.TrimSpace(survivor.Verified) == "" || cand.Verified > survivor.Verified {
						survivor.Verified = cand.Verified
					}
				}
				survivor.SuccessCount += cand.SuccessCount
				mergedAny = true

				retiredIDs[cand.ID] = true
				res.MergedDuplicates = append(res.MergedDuplicates, cand.ID)
				res.Ops = append(res.Ops, ops.Op{
					Type:         ops.OpRetire,
					EntryID:      cand.ID,
					DossierScope: targetScope,
					Timestamp:    now,
					Source:       ops.SourceGardening,
					Fields: map[string]any{
						"reason": fmt.Sprintf("gardening: merged near-duplicate into %s", survivor.ID),
					},
				})
			}

			if mergedAny {
				amendFields := map[string]any{
					"anchors":       append([]string(nil), survivor.Anchors...),
					"evidence":      append([]string(nil), survivor.Evidence...),
					"success_count": survivor.SuccessCount,
				}
				if strings.TrimSpace(survivor.Verified) != "" {
					amendFields["verified"] = survivor.Verified
				}
				res.Ops = append(res.Ops, ops.Op{
					Type:         ops.OpAmend,
					EntryID:      survivor.ID,
					DossierScope: targetScope,
					Timestamp:    now,
					Source:       ops.SourceGardening,
					Fields:       amendFields,
				})
			}
		}

		var survivors []dossiers.Entry
		for _, e := range active {
			if !retiredIDs[e.ID] {
				survivors = append(survivors, e)
			}
		}
		survivingByScope[scope] = survivors
	}

	// --- Step 3: Split oversized dossiers exceeding maxTokens ---
	var splitAliases [][2]string
	for _, scope := range scopes {
		d := projected[scope]
		if d == nil {
			continue
		}
		survivors := survivingByScope[scope]
		if len(survivors) < 2 {
			continue
		}
		targetScope := strings.TrimSpace(d.Scope)
		if targetScope == "" {
			targetScope = scope
		}

		simDossier := *d
		simDossier.Scope = targetScope
		simDossier.Entries = append([]dossiers.Entry(nil), survivors...)
		rendered := dossiers.RenderDossier(&simDossier)
		estTokens := len(rendered) / 4
		if estTokens <= maxTokens {
			continue
		}

		newScopes, moves := partitionDossierEntries(targetScope, survivors)
		if len(newScopes) < 2 || len(moves) == 0 {
			continue
		}

		res.Ops = append(res.Ops, ops.Op{
			Type:         ops.OpSplit,
			DossierScope: targetScope,
			Timestamp:    now,
			Source:       ops.SourceGardening,
			Fields: map[string]any{
				"new_dossiers": newScopes,
				"entry_moves":  moves,
				"split_from":   targetScope,
			},
		})
		res.SplitDossiers = append(res.SplitDossiers, targetScope)
		for _, ns := range newScopes {
			splitAliases = append(splitAliases, [2]string{targetScope, ns})
		}
	}

	res.OpsEmitted = len(res.Ops)

	// --- Step 4: Record ops, check doc freshness, and rebuild multi-tier index ---
	if !opts.DryRun {
		if len(res.Ops) > 0 {
			if _, err := consolidate.RecordRepoTier(ctx, p.repoRoot, res.Ops, nil, nil, now); err != nil {
				return nil, fmt.Errorf("recording gardening ops: %w", err)
			}
		}

		freshRes, err := p.CheckDocFreshness(ctx)
		if err != nil {
			return nil, fmt.Errorf("checking doc freshness during gardening: %w", err)
		}
		res.Freshness = freshRes

		idx := p.idx
		var closeIdx func()
		if idx == nil && strings.TrimSpace(p.dataDir) != "" && strings.TrimSpace(p.repoID) != "" {
			idxPath := index.IndexPath(p.dataDir, p.repoID)
			if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err == nil {
				if opened, openErr := index.NewIndex(idxPath); openErr == nil {
					idx = opened
					closeIdx = func() { _ = opened.Close() }
				}
			}
		}
		if closeIdx != nil {
			defer closeIdx()
		}

		if idx != nil {
			currentBranch, _ := gitops.GetCurrentBranchWithContext(ctx, p.repoRoot)
			if err := consolidate.RebuildMultiTierIndex(ctx, idx, p.dataDir, p.repoID, p.repoRoot, currentBranch); err != nil {
				return nil, fmt.Errorf("rebuilding multi-tier index after gardening: %w", err)
			}
			for _, pair := range splitAliases {
				if err := idx.UpsertAlias(ctx, pair[0], pair[1]); err != nil {
					return nil, fmt.Errorf("recording split alias %s -> %s: %w", pair[0], pair[1], err)
				}
			}
		}
	}

	return res, nil
}

// areEntriesNearDuplicates reports whether two dossier entries in the same canonical type group
// have identical normalized titles or overlapping anchors with token Jaccard similarity >= 0.75.
func areEntriesNearDuplicates(a, b dossiers.Entry) bool {
	// --- Step 1: Require matching canonical entry type group ---
	if consolidate.CanonicalEntryTypeGroup(a.Type) != consolidate.CanonicalEntryTypeGroup(b.Type) {
		return false
	}

	// --- Step 2: Check for exact normalized title match ---
	titleA := consolidate.NormalizeTitle(consolidate.ExtractEntryTitle(a.Text))
	titleB := consolidate.NormalizeTitle(consolidate.ExtractEntryTitle(b.Text))
	if titleA != "" && titleA == titleB {
		return true
	}

	// --- Step 3: Check anchor overlap and token Jaccard similarity ---
	if !consolidate.AnchorsOverlap(a.Anchors, b.Anchors) {
		return false
	}

	if tokenJaccard(a.Text, b.Text) >= 0.75 {
		return true
	}
	if titleA != "" && titleB != "" && tokenJaccard(titleA, titleB) >= 0.75 {
		return true
	}
	return false
}

// tokenJaccard computes the Jaccard similarity coefficient between the token sets of a and b.
func tokenJaccard(a, b string) float64 {
	// --- Step 1: Tokenize both strings ---
	setA := tokenizeForJaccard(a)
	setB := tokenizeForJaccard(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}

	// --- Step 2: Compute intersection over union ---
	inter := 0
	for k := range setA {
		if setB[k] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// tokenizeForJaccard splits text into lowercase alphanumeric tokens for Jaccard comparison.
func tokenizeForJaccard(text string) map[string]bool {
	tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		if len(tok) >= 2 || (len(tok) == 1 && unicode.IsDigit(rune(tok[0]))) {
			out[tok] = true
		}
	}
	return out
}

// mergeUniqueStrings combines two string slices while preserving order and eliminating duplicates.
func mergeUniqueStrings(a, b []string) []string {
	// --- Step 1: Collect unique non-empty strings from slice a ---
	seen := make(map[string]bool, len(a)+len(b))
	var out []string
	for _, s := range a {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}

	// --- Step 2: Append unique non-empty strings from slice b ---
	for _, s := range b {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}
	return out
}

// partitionDossierEntries partitions entries of an oversized dossier into at least two sub-scopes
// using anchor subdirectory prefixes, anchor file stems, entry types, or cognitive kinds.
func partitionDossierEntries(parentScope string, entries []dossiers.Entry) ([]string, map[string]string) {
	// --- Step 1: Partition by anchor sub-directory prefix ---
	dirMoves := make(map[string]string, len(entries))
	dirScopes := make(map[string]bool)
	for _, e := range entries {
		sub := deriveAnchorDirSubScope(parentScope, e.Anchors)
		target := parentScope + "/" + sub
		dirMoves[e.ID] = target
		dirScopes[target] = true
	}
	if len(dirScopes) >= 2 {
		return sortedKeys(dirScopes), dirMoves
	}

	// --- Step 2: Partition by anchor file stem if all anchors share the same directory ---
	fileMoves := make(map[string]string, len(entries))
	fileScopes := make(map[string]bool)
	for _, e := range entries {
		sub := deriveAnchorFileSubScope(e.Anchors)
		target := parentScope + "/" + sub
		fileMoves[e.ID] = target
		fileScopes[target] = true
	}
	if len(fileScopes) >= 2 {
		return sortedKeys(fileScopes), fileMoves
	}

	// --- Step 3: Partition by entry type ---
	typeMoves := make(map[string]string, len(entries))
	typeScopes := make(map[string]bool)
	for _, e := range entries {
		sub := consolidate.SanitizeBranchSlug(string(e.Type))
		if sub == "" || sub == "default" {
			sub = "general"
		}
		target := parentScope + "/" + sub
		typeMoves[e.ID] = target
		typeScopes[target] = true
	}
	if len(typeScopes) >= 2 {
		return sortedKeys(typeScopes), typeMoves
	}

	// --- Step 4: Partition by cognitive kind ---
	kindMoves := make(map[string]string, len(entries))
	kindScopes := make(map[string]bool)
	for _, e := range entries {
		sub := consolidate.SanitizeBranchSlug(string(e.EffectiveKind()))
		if sub == "" || sub == "default" {
			sub = "general"
		}
		target := parentScope + "/" + sub
		kindMoves[e.ID] = target
		kindScopes[target] = true
	}
	if len(kindScopes) >= 2 {
		return sortedKeys(kindScopes), kindMoves
	}

	return nil, nil
}

// deriveAnchorDirSubScope derives a child scope slug from the subdirectory or file stem of anchors relative to parentScope.
func deriveAnchorDirSubScope(parentScope string, anchors []string) string {
	// --- Step 1: Normalize parentScope and inspect each anchor path ---
	scopeNorm := strings.Trim(filepath.ToSlash(strings.TrimSpace(parentScope)), "/")
	for _, raw := range anchors {
		p := consolidate.StripAnchorSymbol(raw)
		p = strings.Trim(filepath.ToSlash(p), "/")
		if p == "" {
			continue
		}

		// --- Step 2: Extract first path segment relative to parentScope when prefixed ---
		if scopeNorm != "" {
			if strings.HasPrefix(p, scopeNorm+"/") {
				rem := strings.TrimPrefix(p, scopeNorm+"/")
				parts := strings.Split(rem, "/")
				if len(parts) > 1 {
					return consolidate.SanitizeBranchSlug(parts[0])
				}
				stem := strings.TrimSuffix(parts[0], filepath.Ext(parts[0]))
				return consolidate.SanitizeBranchSlug(stem)
			}
			if idx := strings.Index(p, "/"+scopeNorm+"/"); idx >= 0 {
				rem := p[idx+len("/"+scopeNorm+"/"):]
				parts := strings.Split(rem, "/")
				if len(parts) > 1 {
					return consolidate.SanitizeBranchSlug(parts[0])
				}
				stem := strings.TrimSuffix(parts[0], filepath.Ext(parts[0]))
				return consolidate.SanitizeBranchSlug(stem)
			}
		}

		// --- Step 3: Fall back to parent directory basename or file stem ---
		dir := filepath.ToSlash(filepath.Dir(p))
		if dir != "." && dir != "" {
			base := filepath.Base(dir)
			if !strings.EqualFold(base, filepath.Base(scopeNorm)) {
				return consolidate.SanitizeBranchSlug(base)
			}
			parts := strings.Split(dir, "/")
			if len(parts) > 0 {
				return consolidate.SanitizeBranchSlug(parts[0])
			}
		}
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if stem != "" {
			return consolidate.SanitizeBranchSlug(stem)
		}
	}
	return "general"
}

// deriveAnchorFileSubScope derives a child scope slug from the first non-empty anchor file stem.
func deriveAnchorFileSubScope(anchors []string) string {
	for _, raw := range anchors {
		p := consolidate.StripAnchorSymbol(raw)
		p = strings.Trim(filepath.ToSlash(p), "/")
		if p == "" {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if stem != "" {
			return consolidate.SanitizeBranchSlug(stem)
		}
	}
	return "general"
}

// sortedKeys returns the keys of map m in ascending lexicographical order.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
