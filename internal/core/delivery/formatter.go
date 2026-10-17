/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/delivery/formatter.go
 * Description: Renders ranked repository memory entries, graduated skill
 *              pointers, and edit-gate denial messages for harness injection,
 *              and coalesces multiple formatted context blocks into a single
 *              wrapper block per invocation.
 *
 * Exported Types / Functions:
 * - FormatEntry():            Formats a single dossier entry as a tagged memory line.
 * - FormatEntryForMoment():   Formats an entry for a delivery moment, substituting skill pointers when graduated.
 * - FormatEntries():          Formats ranked entries into a wrapped Markdown context block.
 * - FormatEntriesForMoment(): Formats ranked entries for a specific delivery moment and workspace.
 * - CoalesceContextBlocks():  Merges and deduplicates multiple context blocks under one header.
 * - FormatGateDeny():         Formats an edit-gate denial message for PreToolUse enforcement.
 * - ContentHash():            Computes the hex-encoded SHA-256 digest of a rendered text string.
 */

package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
)

// FormatEntry formats a single dossier entry as a tagged repository memory line.
func FormatEntry(entry dossiers.Entry) string {
	return fmt.Sprintf("[repo-memory %s] %s: %s", entry.ID, entry.Type, entry.Text)
}

// FormatEntryForMoment formats a single dossier entry for a specific delivery
// moment, substituting a skill pointer line on first-touch/first-entry when
// the entry has graduated to an active workspace skill.
func FormatEntryForMoment(entry dossiers.Entry, moment Moment, workspaceRoot string) string {
	if moment == MomentFirstTouch || moment == MomentFirstEntry {
		if slug := skills.GraduatedSkillSlug(entry, workspaceRoot); slug != "" {
			title := skills.EntryTitleLine(entry)
			return fmt.Sprintf("[repo-memory %s] skill %s covers this procedure: %s", entry.ID, slug, title)
		}
	}
	return FormatEntry(entry)
}

// formatBulletLine formats a single rendered entry string as a Markdown list
// item, indenting any continuation lines so multi-line entries stay within the
// bullet item.
func formatBulletLine(entryText string) string {
	trimmed := strings.TrimSpace(entryText)
	trimmed = strings.TrimPrefix(trimmed, "- ")
	indented := strings.ReplaceAll(trimmed, "\n", "\n  ")
	return "- " + indented
}

// FormatEntries formats a slice of ranked entries into a wrapped context block.
func FormatEntries(entries []RankedEntry) string {
	return FormatEntriesForMoment(entries, "", "")
}

// FormatEntriesForMoment formats a slice of ranked entries for a specific
// delivery moment into a compact Markdown bullet block under
// `--- repo-memory context ---` (with a leading newline and no trailing `---`
// Setext heading underline so Markdown renderers do not promote the block to
// an `<h2>` heading).
func FormatEntriesForMoment(entries []RankedEntry, moment Moment, workspaceRoot string) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n--- repo-memory context ---\n\n")
	for i, r := range entries {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(formatBulletLine(FormatEntryForMoment(r.Entry, moment, workspaceRoot)))
	}
	return sb.String()
}

// CoalesceContextBlocks merges multiple formatted `--- repo-memory context ---`
// blocks into a single compact Markdown bullet block with one header,
// preserving entry order and deduplicating identical entries.
func CoalesceContextBlocks(blocks []string) string {
	// --- Step 1: Strip wrapper headers/footers and deduplicate entry items ---
	var bodyParts []string
	seen := make(map[string]bool)
	for _, b := range blocks {
		trimmed := strings.TrimSpace(b)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "--- repo-memory context ---")
		trimmed = strings.TrimSuffix(trimmed, "---")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed == "" {
			continue
		}
		// Split individual bullet items when a block contains multiple entries.
		for _, rawLine := range strings.Split(trimmed, "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" || line == "---" {
				continue
			}
			line = strings.TrimPrefix(line, "- ")
			if strings.HasPrefix(line, "[repo-memory ") {
				if seen[line] {
					continue
				}
				seen[line] = true
				bodyParts = append(bodyParts, formatBulletLine(line))
			} else if len(bodyParts) > 0 {
				// Continuation line of a multi-line entry (e.g. a recipe).
				bodyParts[len(bodyParts)-1] += "\n  " + line
			} else {
				if seen[line] {
					continue
				}
				seen[line] = true
				bodyParts = append(bodyParts, formatBulletLine(line))
			}
		}
	}
	if len(bodyParts) == 0 {
		return ""
	}

	// --- Step 2: Wrap the combined bullets under a single context header ---
	var sb strings.Builder
	sb.WriteString("\n--- repo-memory context ---\n\n")
	sb.WriteString(strings.Join(bodyParts, "\n"))
	return sb.String()
}

// FormatGateDeny formats an edit-gate denial message for PreToolUse enforcement.
func FormatGateDeny(entry dossiers.Entry) string {
	return fmt.Sprintf("[REPO-MEMORY %s] %s. Re-issue the edit after checking this.", entry.ID, entry.Text)
}

// ContentHash returns the hex-encoded SHA-256 digest of text.
func ContentHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
