package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
)

func FormatEntry(entry dossiers.Entry) string {
	return fmt.Sprintf("[repo-memory %s] %s: %s", entry.ID, entry.Type, entry.Text)
}

func FormatEntries(entries []RankedEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("--- repo-memory context ---\n")
	for _, r := range entries {
		sb.WriteString(FormatEntry(r.Entry))
		sb.WriteString("\n")
	}
	sb.WriteString("---")
	return sb.String()
}

func FormatGateDeny(entry dossiers.Entry) string {
	return fmt.Sprintf("[REPO-MEMORY %s] %s. Re-issue the edit after checking this.", entry.ID, entry.Text)
}

func ContentHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
