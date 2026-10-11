package delivery

import (
	"sort"
	
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
)

type RankedEntry struct {
	Entry       dossiers.Entry
	Score       int
	AnchorMatch string // "exact", "directory", "scope", or ""
	FTSScore    float64
}

// DefaultMinRelevanceScore is the default minimum score required for an entry to be injected.
// Entries with no anchor match and low type score (e.g. score < 50) are filtered out.
const DefaultMinRelevanceScore = 50

var typeOrderRanking = map[dossiers.EntryType]int{
	dossiers.EntryTypeInvariant:        80,
	dossiers.EntryTypeFailureSignature: 60,
	dossiers.EntryTypeRuledOut:         50,
	dossiers.EntryTypeGotcha:           40,
	dossiers.EntryTypeDecision:         30,
	dossiers.EntryTypeProcedure:        20,
	dossiers.EntryTypeInterface:        10,
	dossiers.EntryTypeOpenIssue:        5,
}

func RankEntries(entries []dossiers.Entry, targetAnchors []string, ftsScores ...map[string]float64) []RankedEntry {
	var scores map[string]float64
	if len(ftsScores) > 0 {
		scores = ftsScores[0]
	}
	var ranked []RankedEntry
	
	targetSet := make(map[string]bool)
	for _, a := range targetAnchors {
		targetSet[a] = true
	}

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

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}

func RankEntriesWithScores(entries []dossiers.Entry, targetAnchors []string, scores map[string]float64) []RankedEntry {
	return RankEntries(entries, targetAnchors, scores)
}

func SelectWithinBudget(ranked []RankedEntry, tokenBudget int, minScore ...int) []RankedEntry {
	threshold := DefaultMinRelevanceScore
	if len(minScore) > 0 {
		threshold = minScore[0]
	}
	var selected []RankedEntry
	usedTokens := 0
	
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
