package antigravity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
)

var (
	ledgerStoreMutex sync.Mutex
	// safeIDPattern matches UUIDs, hex strings, and other path-safe identifiers.
	safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
)

// sanitizeConversationID ensures the conversation ID is safe for use in a filesystem path.
// Empty, path-traversal, or non-alphanumeric IDs are hashed to a safe hex string.
func sanitizeConversationID(id string) string {
	if id == "" {
		return "unknown"
	}
	if safeIDPattern.MatchString(id) && len(id) <= 255 {
		return id
	}
	// Hash unsafe IDs to prevent path traversal and other filesystem issues.
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:16])
}

func getLedgerPath(dataDir, repoID, conversationID string) string {
	safeID := sanitizeConversationID(conversationID)
	return filepath.Join(dataDir, repoID, "ledgers", safeID+".json")
}

// SaveLedger atomically persists the ledger to disk using write-to-temp + rename.
func SaveLedger(dataDir, repoID string, ledger *delivery.Ledger) error {
	ledgerStoreMutex.Lock()
	defer ledgerStoreMutex.Unlock()

	path := getLedgerPath(dataDir, repoID, ledger.ConversationID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating ledger directory: %w", err)
	}

	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling ledger: %w", err)
	}

	// Atomic write: write to a temp file in the same directory, then rename.
	// This prevents readers from seeing a truncated file.
	tmp, err := os.CreateTemp(dir, ".ledger-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for ledger: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing ledger temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing ledger temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming ledger file: %w", err)
	}

	return nil
}

func LoadLedger(dataDir, repoID, conversationID string) (*delivery.Ledger, error) {
	ledgerStoreMutex.Lock()
	defer ledgerStoreMutex.Unlock()

	path := getLedgerPath(dataDir, repoID, conversationID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return delivery.NewLedger(conversationID), nil
		}
		return nil, err
	}

	var ledger delivery.Ledger
	if err := json.Unmarshal(data, &ledger); err != nil {
		// Corrupted file (e.g. truncated) — start fresh rather than failing.
		return delivery.NewLedger(conversationID), nil
	}
	
	// Ensure maps are initialized if they were nil in json
	if ledger.SeenDossiers == nil {
		ledger.SeenDossiers = make(map[string]bool)
	}
	if ledger.SeenAnchors == nil {
		ledger.SeenAnchors = make(map[string]bool)
	}
	if ledger.AcknowledgedGates == nil {
		ledger.AcknowledgedGates = make(map[string]bool)
	}

	return &ledger, nil
}
