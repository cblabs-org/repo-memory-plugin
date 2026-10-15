package seed

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func generateDeterministicID(relPath string, section string, bulletText string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", relPath, section, bulletText)))
	return fmt.Sprintf("seed-%x", hash[:4])
}

// IsJunkText returns true for placeholder entries that PA agents produce
// when they haven't filled in a section. These waste injection budget.
func IsJunkText(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	switch lower {
	case "none.", "none", "(pending)", "n/a", "pending", "tbd", "todo":
		return true
	}
	if strings.HasPrefix(lower, "none encountered") || strings.HasPrefix(lower, "none identified") ||
		strings.HasPrefix(lower, "no issues") || strings.HasPrefix(lower, "nothing to") {
		return true
	}
	return false
}

func isJunkText(text string) bool {
	return IsJunkText(text)
}

func ReportToOps(report *CompletionReport, resolver *config.ScopeResolver, path string, commitTime time.Time, existingIDs map[string]bool) []ops.Op {
	var result []ops.Op

	if report == nil {
		return result
	}

	var anchors []string
	for _, c := range report.Changes {
		anchors = append(anchors, c.Path)
	}

	dossierScope := resolver.ResolveScope(anchors, nil)
	if dossierScope == "" {
		dossierScope = report.ProjectName
	}
	
	now := commitTime
	if now.IsZero() {
		now = time.Now()
	}

	for _, dec := range report.Decisions {
		if isJunkText(dec) {
			continue
		}
		entryID := generateDeterministicID(path, "dec", dec)
		if existingIDs != nil && existingIDs[entryID] {
			continue
		}
		result = append(result, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      entryID,
			DossierScope: dossierScope,
			Timestamp:    now,
			Source:       ops.SourceSeed,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeDecision),
				"text":       dec,
				"anchors":    anchors,
			},
		})
	}

	for _, de := range report.DeadEnds {
		if isJunkText(de) {
			continue
		}
		entryID := generateDeterministicID(path, "rul", de)
		if existingIDs != nil && existingIDs[entryID] {
			continue
		}
		result = append(result, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      entryID,
			DossierScope: dossierScope,
			Timestamp:    now,
			Source:       ops.SourceSeed,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeRuledOut),
				"text":       de,
				"anchors":    anchors,
			},
		})
	}

	for _, finding := range report.Findings {
		if isJunkText(finding) {
			continue
		}
		entryID := generateDeterministicID(path, "issue", finding)
		if existingIDs != nil && existingIDs[entryID] {
			continue
		}
		result = append(result, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      entryID,
			DossierScope: dossierScope,
			Timestamp:    now,
			Source:       ops.SourceSeed,
			Fields: map[string]any{
				"entry_type": string(dossiers.EntryTypeOpenIssue),
				"text":       finding,
				"anchors":    anchors,
			},
		})
	}

	return result
}
