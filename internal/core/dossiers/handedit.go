package dossiers

import (
	"regexp"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

var entryHeaderRe = regexp.MustCompile(`^### \[([a-zA-Z0-9-]+)\]\s*(.*)$`)

type parsedEntry struct {
	id   string
	text string
}

func parseMarkdownEntries(md string) map[string]parsedEntry {
	lines := strings.Split(md, "\n")
	entries := make(map[string]parsedEntry)

	var currentID string
	var currentText strings.Builder

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
			if !strings.HasPrefix(line, "`") || !strings.HasSuffix(line, "`") || !strings.Contains(line, " · ") {
				currentText.WriteString(line + "\n")
			}
		}
	}

	if currentID != "" {
		entries[currentID] = parsedEntry{
			id:   currentID,
			text: strings.TrimSpace(currentText.String()),
		}
	}

	return entries
}

func DetectHandEdits(rendered string, onDisk string, dossierScope string) ([]ops.Op, error) {
	if rendered == onDisk {
		return nil, nil
	}

	renderedEntries := parseMarkdownEntries(rendered)
	onDiskEntries := parseMarkdownEntries(onDisk)

	var newOps []ops.Op
	now := time.Now().UTC()

	for id, onDiskEntry := range onDiskEntries {
		renderedEntry, ok := renderedEntries[id]
		if !ok {
			entryType := string(EntryTypeInvariant)
			for et, prefix := range EntryTypePrefixes {
				if strings.HasPrefix(id, prefix) {
					entryType = string(et)
					break
				}
			}

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
