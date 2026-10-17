/*
ranker.go scores, orders, and token-budgets candidate dossier entries for push
delivery based on anchor specificity, entry type priority, verification status,
evidence count, and FTS5 BM25 relevance.
*/

package delivery

import (
	"sort"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
)

// RankedEntry pairs a dossier entry with its computed delivery score and match metadata.
type RankedEntry struct {
	// Entry is the underlying dossier entry.
	Entry dossiers.Entry
	// Score is the composite relevance score used for ordering and threshold filtering.
	Score int
	// AnchorMatch indicates how the entry matched target paths ("exact", "directory", "scope", or "").
	AnchorMatch string
	// FTSScore is the raw FTS5 BM25 relevance score when matched via text search.
	FTSScore float64
}

// DefaultMinRelevanceScore is the default minimum score required for an entry to be injected.
// Entries with no anchor match and low type score (e.g. score < 50) are filtered out.
const DefaultMinRelevanceScore = 50

// typeOrderRanking assigns base priority points by entry type so actionable
// invariants, contracts, conventions, and gotchas outrank historical diary entries.
var typeOrderRanking = map[dossiers.EntryType]int{
	dossiers.EntryTypeInvariant:         80,
	dossiers.EntryTypeContract:          80,
	dossiers.EntryTypeConvention:        80,
	dossiers.EntryTypeGotcha:            70,
	dossiers.EntryTypeExternalBehaviour: 70,
	dossiers.EntryTypeFailureSignature:  60,
	dossiers.EntryTypeRuledOut:          50,
	dossiers.EntryTypeDecision:          30,
	dossiers.EntryTypeProcedure:         20,
	dossiers.EntryTypeRecipe:            20,
	dossiers.EntryTypeInterface:         10,
	dossiers.EntryTypeOpenIssue:         5,
}

// RankEntries computes composite delivery scores for entries against targetAnchors
// and optional FTS5 BM25 scores, returning entries sorted descending by score.
func RankEntries(entries []dossiers.Entry, targetAnchors []string, ftsScores ...map[string]float64) []RankedEntry {
	// --- Step 1: Index target anchors and optional FTS score map ---
	var scores map[string]float64
	if len(ftsScores) > 0 {
		scores = ftsScores[0]
	}
	var ranked []RankedEntry

	targetSet := make(map[string]bool)
	for _, a := range targetAnchors {
		targetSet[a] = true
	}

	// --- Step 2: Score each entry across anchor, type, verification, evidence, and FTS signals ---
	for _, e := range entries {
		score := 0
		anchorMatch := ""

		// 1. Anchor match
		if len(e.Anchors) == 0 {
			score += 100
			anchorMatch = "scope"
		} else {
			matchedExact := false
			for _, a := range e.Anchors {
				if targetSet[a] {
					matchedExact = true
					break
				}
			}
			if matchedExact {
				score += 1000
				anchorMatch = "exact"
			} else {
				matchedDir := false
				for _, ta := range targetAnchors {
					for _, a := range e.Anchors {
						if len(ta) > len(a) && ta[:len(a)] == a && (ta[len(a)] == '/' || a[len(a)-1] == '/') {
							matchedDir = true
							break
						}
					}
					if matchedDir {
						break
					}
				}
				if matchedDir {
					score += 500
					anchorMatch = "directory"
				}
			}
		}

		// 2. Entry type
		score += typeOrderRanking[e.Type]

		// 3. Verification status
		if e.Verified != "" {
			score += 200
		}

		// 4. Evidence count
		evScore := len(e.Evidence) * 10
		if evScore > 50 {
			evScore = 50
		}
		score += evScore

		// 5. FTS BM25 relevance score
		var ftsScore float64
		if scores != nil {
			ftsScore = scores[e.ID]
		}
		if ftsScore > 0 {
			score += int(ftsScore * 100)
		}

		ranked = append(ranked, RankedEntry{
			Entry:       e,
			Score:       score,
			AnchorMatch: anchorMatch,
			FTSScore:    ftsScore,
		})
	}

	// --- Step 3: Sort entries descending by composite score ---
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}

// RankEntriesWithScores is an alias for RankEntries with explicit FTS scores.
func RankEntriesWithScores(entries []dossiers.Entry, targetAnchors []string, scores map[string]float64) []RankedEntry {
	return RankEntries(entries, targetAnchors, scores)
}

// SelectWithinBudget filters ranked entries by minimum relevance score and
// greedily selects entries whose estimated token cost fits within tokenBudget.
func SelectWithinBudget(ranked []RankedEntry, tokenBudget int, minScore ...int) []RankedEntry {
	// --- Step 1: Resolve relevance score threshold ---
	threshold := DefaultMinRelevanceScore
	if len(minScore) > 0 {
		threshold = minScore[0]
	}
	var selected []RankedEntry
	usedTokens := 0

	// --- Step 2: Accumulate entries that meet threshold and fit remaining token budget ---
	for _, r := range ranked {
		if r.Score < threshold {
			continue
		}
		tokens := len(r.Entry.Text) / 4
		if tokens == 0 {
			tokens = 1
		}
		if usedTokens+tokens <= tokenBudget {
			selected = append(selected, r)
			usedTokens += tokens
		}
	}

	return selected
}
