package seed

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func generateDeterministicID(path string, section string, index int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", path, section, index)))
	return fmt.Sprintf("seed-%x", hash[:4])
}

func ReportToOps(report *CompletionReport, resolver *config.ScopeResolver, path string, existingIDs map[string]bool) []ops.Op {
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
	
	now := time.Now()

	for i, dec := range report.Decisions {
		entryID := generateDeterministicID(path, "dec", i)
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

	for i, de := range report.DeadEnds {
		entryID := generateDeterministicID(path, "rul", i)
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

	for i, finding := range report.Findings {
		entryID := generateDeterministicID(path, "issue", i)
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
