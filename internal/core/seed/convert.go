/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/seed/convert.go
 * Description: Converts parsed Principal Architect CompletionReport structures into
 *              deterministic, deduplicated dossier op-log entries.
 *
 * Exported Types / Functions:
 * - IsJunkText():  Reports whether a bullet string is an empty/placeholder PA value.
 * - ReportToOps(): Converts a CompletionReport into OpAdd operations with deterministic IDs.
 */

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

// generateDeterministicID hashes relPath, section, and bulletText into a stable "seed-<hex>" entry ID.
func generateDeterministicID(relPath string, section string, bulletText string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", relPath, section, bulletText)))
	return fmt.Sprintf("seed-%x", hash[:4])
}

// IsJunkText returns true for placeholder entries that PA agents produce
// when they haven't filled in a section. These waste injection budget.
func IsJunkText(text string) bool {
	// --- Step 1: Normalize and check exact placeholder matches ---
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	switch lower {
	case "none.", "none", "(none)", "(none).", "(pending)", "(pending).",
		"n/a", "(n/a)", "pending", "tbd", "(tbd)", "todo", "(todo)", "-":
		return true
	}

	// --- Step 2: Check punctuation-stripped tokens and common placeholder prefixes ---
	stripped := strings.Trim(lower, "(). \t-")
	switch stripped {
	case "", "none", "n/a", "pending", "tbd", "todo":
		return true
	}
	if strings.HasPrefix(lower, "none encountered") || strings.HasPrefix(lower, "none identified") ||
		strings.HasPrefix(lower, "no issues") || strings.HasPrefix(lower, "nothing to") {
		return true
	}
	return false
}

// isJunkText wraps IsJunkText for internal callers in package seed.
func isJunkText(text string) bool {
	return IsJunkText(text)
}

// ReportToOps converts a parsed CompletionReport into deduplicated OpAdd operations
// scoped via resolver.
func ReportToOps(report *CompletionReport, resolver *config.ScopeResolver, reportPath string, commitTime time.Time, existingIDs map[string]bool) []ops.Op {
	var result []ops.Op

	if report == nil {
		return result
	}

	// --- Step 1: Collect deduplicated file anchors and resolve dossier scope ---
	var anchors []string
	seenAnchors := make(map[string]bool)
	for _, c := range report.Changes {
		if c.Path != "" && !seenAnchors[c.Path] {
			seenAnchors[c.Path] = true
			anchors = append(anchors, c.Path)
		}
	}

	dossierScope := resolver.ResolveScope(anchors, nil)
	if dossierScope == "" {
		dossierScope = report.ProjectName
	}

	now := commitTime
	if now.IsZero() {
		now = time.Now()
	}

	// --- Step 2: Emit decision, ruled-out, and open-issue operations ---
	for _, dec := range report.Decisions {
		if isJunkText(dec) {
			continue
		}
		entryID := generateDeterministicID(reportPath, "dec", dec)
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
		entryID := generateDeterministicID(reportPath, "rul", de)
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
		entryID := generateDeterministicID(reportPath, "issue", finding)
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
