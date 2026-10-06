package seed

import (
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func ReportToOps(report *CompletionReport, resolver *config.ScopeResolver) []ops.Op {
	var result []ops.Op

	if report == nil {
		return result
	}

	var anchors []string
	for _, c := range report.Changes {
		anchors = append(anchors, c.Path)
	}

	dossierScope := resolver.ResolveScope(anchors, nil)
	now := time.Now()

	for _, dec := range report.Decisions {
		result = append(result, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      ops.NewEntryID("dec"),
			DossierScope: dossierScope,
			Timestamp:    now,
			Source:       ops.SourceSeed,
			Fields: map[string]any{
				"type":    string(dossiers.EntryTypeDecision),
				"text":    dec,
				"anchors": anchors,
			},
		})
	}

	for _, de := range report.DeadEnds {
		result = append(result, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      ops.NewEntryID("rul"),
			DossierScope: dossierScope,
			Timestamp:    now,
			Source:       ops.SourceSeed,
			Fields: map[string]any{
				"type":    string(dossiers.EntryTypeRuledOut),
				"text":    de,
				"anchors": anchors,
			},
		})
	}

	for _, finding := range report.Findings {
		result = append(result, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      ops.NewEntryID("issue"),
			DossierScope: dossierScope,
			Timestamp:    now,
			Source:       ops.SourceSeed,
			Fields: map[string]any{
				"type":    string(dossiers.EntryTypeOpenIssue),
				"text":    finding,
				"anchors": anchors,
			},
		})
	}

	return result
}
