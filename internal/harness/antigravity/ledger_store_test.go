/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package antigravity

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
)

func TestLedgerStore(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "testrepo"
	convID := "conv123"

	t.Run("Load non-existent returns fresh", func(t *testing.T) {
		ledger, err := LoadLedger(dataDir, repoID, convID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if ledger == nil {
			t.Fatalf("Expected ledger, got nil")
		}
		if ledger.ConversationID != convID {
			t.Errorf("Expected %s, got %s", convID, ledger.ConversationID)
		}
		if len(ledger.Injections) != 0 {
			t.Errorf("Expected 0 injections, got %d", len(ledger.Injections))
		}
	})

	t.Run("Save and Load round-trip", func(t *testing.T) {
		ledger := delivery.NewLedger(convID)
		ledger.RecordInjection(delivery.InjectionRecord{EntryID: "ent1"})
		ledger.MarkDossierSeen("scope1")
		ledger.MarkAnchorSeen("file.go")
		ledger.AcknowledgeGate("gate1")

		err := SaveLedger(dataDir, repoID, ledger)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		loaded, err := LoadLedger(dataDir, repoID, convID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if loaded.ConversationID != convID {
			t.Errorf("Expected %s, got %s", convID, loaded.ConversationID)
		}
		if len(loaded.Injections) != 1 {
			t.Errorf("Expected 1 injection, got %d", len(loaded.Injections))
		}
		if !loaded.HasSeenDossier("scope1") {
			t.Errorf("Expected dossier scope1")
		}
		if !loaded.HasSeenAnchor("file.go") {
			t.Errorf("Expected anchor file.go")
		}
		if !loaded.IsGateAcknowledged("gate1") {
			t.Errorf("Expected gate1")
		}
	})

	t.Run("Concurrent safe", func(t *testing.T) {
		ledger := delivery.NewLedger(convID)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = SaveLedger(dataDir, repoID, ledger)
				_, _ = LoadLedger(dataDir, repoID, convID)
			}()
		}
		wg.Wait()
		
		path := filepath.Join(dataDir, repoID, "ledgers", convID+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("Expected file to exist: %v", err)
		}
	})
}
