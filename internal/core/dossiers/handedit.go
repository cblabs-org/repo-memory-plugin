/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/handedit.go
 * Description: Detection of human prose additions and amendments in on-disk
 *              Markdown dossiers, synthesizing OpAdd and OpAmend operations
 *              so manual edits are preserved in the operation log.
 *
 * Exported Types / Functions:
 * - InferEntryTypeFromID(): Infers the canonical EntryType from an entry ID prefix.
 * - DetectHandEdits():      Diffs rendered vs. on-disk dossier Markdown and returns human OpAdd/OpAmend ops.
 */

package dossiers

import (
	"regexp"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// entryHeaderRe matches a level-3 Markdown entry heading of the form `### [<id>] <title>`.
var entryHeaderRe = regexp.MustCompile(`^### \[([a-zA-Z0-9-]+)\]\s*(.*)$`)

// parsedEntry holds the entry ID and normalized body text extracted from a Markdown dossier.
type parsedEntry struct {
	id   string // Extracted bracketed entry identifier.
	text string // Trimmed title and multiline prose body (excluding the backtick metadata line).
}

// parseMarkdownEntries parses a rendered dossier Markdown document into a map
// of entry IDs to their extracted prose text.
func parseMarkdownEntries(md string) map[string]parsedEntry {
	// --- Step 1: Split Markdown content into lines and initialize accumulator state ---
	lines := strings.Split(md, "\n")
	entries := make(map[string]parsedEntry)

	var currentID string
	var currentText strings.Builder

	// --- Step 2: Scan lines for H3 entry headers and accumulate non-metadata body lines ---
	for _, line := range lines {
		if match := entryHeaderRe.FindStringSubmatch(line); match != nil {
			if currentID != "" {
				entries[currentID] = parsedEntry{
					id:   currentID,
					text: strings.TrimSpace(currentText.String()),
				}
			}
			currentID = match[1]
			currentText.Reset()
			currentText.WriteString(match[2] + "\n")
		} else if currentID != "" {
			// Skip the single-line backtick metadata row (`type · kind: ...`) when reconstructing prose.
			if !strings.HasPrefix(line, "`") || !strings.HasSuffix(line, "`") || !strings.Contains(line, " · ") {
				currentText.WriteString(line + "\n")
			}
		}
	}

	// --- Step 3: Flush the final entry if present ---
	if currentID != "" {
		entries[currentID] = parsedEntry{
			id:   currentID,
			text: strings.TrimSpace(currentText.String()),
		}
	}

	return entries
}

// InferEntryTypeFromID infers the EntryType corresponding to the prefix of id,
// defaulting to EntryTypeInvariant when no known prefix matches.
func InferEntryTypeFromID(id string) EntryType {
	// --- Step 1: Match id prefix against canonical and legacy entry type prefixes ---
	switch {
	case strings.HasPrefix(id, "inv-"):
		return EntryTypeInvariant
	case strings.HasPrefix(id, "con-"):
		return EntryTypeContract
	case strings.HasPrefix(id, "rul-"):
		return EntryTypeRuledOut
	case strings.HasPrefix(id, "fail-"), strings.HasPrefix(id, "err-"):
		return EntryTypeFailureSignature
	case strings.HasPrefix(id, "got-"):
		return EntryTypeGotcha
	case strings.HasPrefix(id, "dec-"):
		return EntryTypeDecision
	case strings.HasPrefix(id, "rec-"):
		return EntryTypeRecipe
	case strings.HasPrefix(id, "proc-"):
		return EntryTypeProcedure
	case strings.HasPrefix(id, "iface-"):
		return EntryTypeInterface
	case strings.HasPrefix(id, "issue-"), strings.HasPrefix(id, "iss-"):
		return EntryTypeOpenIssue
	case strings.HasPrefix(id, "docseed-"):
		return EntryTypeInvariant
	default:
		return EntryTypeInvariant
	}
}

// DetectHandEdits compares the expected rendered Markdown of a dossier against
// the onDisk Markdown and returns OpAdd and OpAmend operations for any human edits.
func DetectHandEdits(rendered string, onDisk string, dossierScope string) ([]ops.Op, error) {
	// --- Step 1: Short-circuit when rendered and on-disk content are identical ---
	if rendered == onDisk {
		return nil, nil
	}

	// --- Step 2: Parse both Markdown documents into entry maps ---
	renderedEntries := parseMarkdownEntries(rendered)
	onDiskEntries := parseMarkdownEntries(onDisk)

	var newOps []ops.Op
	now := time.Now().UTC()

	// --- Step 3: Emit OpAdd for new IDs and OpAmend for modified prose ---
	for id, onDiskEntry := range onDiskEntries {
		renderedEntry, ok := renderedEntries[id]
		if !ok {
			entryType := string(InferEntryTypeFromID(id))

			// Possibly a new entry added by hand, but ID must be valid. We assume it's valid.
			newOps = append(newOps, ops.Op{
				Type:         ops.OpAdd,
				EntryID:      id,
				DossierScope: dossierScope,
				Timestamp:    now,
				Source:       ops.SourceHuman,
				Fields: map[string]any{
					"entry_type": entryType,
					"text":       onDiskEntry.text,
				},
			})
		} else if renderedEntry.text != onDiskEntry.text {
			// Text changed
			newOps = append(newOps, ops.Op{
				Type:         ops.OpAmend,
				EntryID:      id,
				DossierScope: dossierScope,
				Timestamp:    now,
				Source:       ops.SourceHuman,
				Fields: map[string]any{
					"text": onDiskEntry.text,
				},
			})
		}
	}

	return newOps, nil
}
