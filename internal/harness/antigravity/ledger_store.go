package antigravity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
)

var ledgerStoreMutex sync.Mutex

func getLedgerPath(dataDir, repoID, conversationID string) string {
	return filepath.Join(dataDir, repoID, "ledgers", conversationID+".json")
}

func SaveLedger(dataDir, repoID string, ledger *delivery.Ledger) error {
	ledgerStoreMutex.Lock()
	defer ledgerStoreMutex.Unlock()

	path := getLedgerPath(dataDir, repoID, ledger.ConversationID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
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
		return nil, err
	}
	
	// ensure maps are initialized if they were nil in json
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
