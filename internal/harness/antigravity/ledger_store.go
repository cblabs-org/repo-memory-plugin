/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/ledger_store.go
 * Description: Provides atomic, cross-process-locked JSON persistence for
 *              per-conversation delivery ledgers under <data-dir>/<repo-id>/ledgers/.
 *
 * Exported Types / Functions:
 * - ModifyLedger(): Atomically loads, mutates, and saves a conversation ledger under an exclusive lock.
 * - SaveLedger():   Atomically writes a conversation ledger to disk via temp-file rename.
 * - LoadLedger():   Loads a conversation ledger from disk, initializing an empty ledger if missing.
 */

package antigravity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
)

var (
	// ledgerStoreMutex serializes in-process concurrent access to ledger files.
	ledgerStoreMutex sync.Mutex
	// safeIDPattern matches UUIDs, hex strings, and other path-safe identifiers.
	safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
)

// sanitizeConversationID ensures the conversation ID is safe for use in a filesystem path.
// Empty, path-traversal, or non-alphanumeric IDs are hashed to a safe hex string.
func sanitizeConversationID(id string) string {
	if id == "" {
		return ""
	}
	if safeIDPattern.MatchString(id) && len(id) <= 255 {
		return id
	}
	// Hash unsafe IDs to prevent path traversal and other filesystem issues.
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:16])
}

// getLedgerPath returns the JSON file path for conversationID's ledger.
func getLedgerPath(dataDir, repoID, conversationID string) string {
	safeID := sanitizeConversationID(conversationID)
	return filepath.Join(dataDir, repoID, "ledgers", safeID+".json")
}

// flockPath returns the path for the cross-process lock file for this ledger.
func flockPath(dataDir, repoID, conversationID string) string {
	safeID := sanitizeConversationID(conversationID)
	return filepath.Join(dataDir, repoID, "ledgers", "."+safeID+".lock")
}

// acquireLock opens a lock file and acquires an exclusive cross-process lock.
// Returns the lock file (caller must close it after releasing) and any error.
func acquireLock(dataDir, repoID, conversationID string) (*os.File, error) {
	// --- Step 1: Ensure ledger directory exists and open the lock file ---
	dir := filepath.Join(dataDir, repoID, "ledgers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating ledger directory: %w", err)
	}

	lockFile := flockPath(dataDir, repoID, conversationID)
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	// --- Step 2: Acquire an OS-level exclusive lock on the file descriptor ---
	if err := fileLock(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}
	return f, nil
}

// ModifyLedger atomically loads, modifies, and saves a ledger under both
// an in-process mutex and a cross-process file lock, preventing lost updates
// when PreInvocation, PreToolUse, and PostToolUse overlap.
func ModifyLedger(dataDir, repoID, conversationID string, fn func(l *delivery.Ledger)) error {
	// --- Step 1: Acquire in-process mutex and cross-process file lock ---
	if conversationID == "" {
		return nil // skip persistence for empty IDs
	}

	ledgerStoreMutex.Lock()
	defer ledgerStoreMutex.Unlock()

	lock, err := acquireLock(dataDir, repoID, conversationID)
	if err != nil {
		return err
	}
	defer func() {
		fileUnlock(lock)
		lock.Close()
	}()

	// --- Step 2: Load current ledger state, apply fn, and save atomically ---
	ledger, err := loadLedgerLocked(dataDir, repoID, conversationID)
	if err != nil {
		return err
	}

	fn(ledger)

	return saveLedgerLocked(dataDir, repoID, ledger)
}

// SaveLedger atomically persists the ledger to disk using write-to-temp + rename.
// For cross-process safety, prefer ModifyLedger when doing load+modify+save.
func SaveLedger(dataDir, repoID string, ledger *delivery.Ledger) error {
	if ledger.ConversationID == "" {
		return nil // no-op when ledger has no bound conversation UUID
	}

	ledgerStoreMutex.Lock()
	defer ledgerStoreMutex.Unlock()

	return saveLedgerLocked(dataDir, repoID, ledger)
}

// saveLedgerLocked marshals ledger to JSON and atomically writes it via a temporary file rename.
// The caller must hold ledgerStoreMutex.
func saveLedgerLocked(dataDir, repoID string, ledger *delivery.Ledger) error {
	// --- Step 1: Ensure target directory exists and marshal ledger JSON ---
	path := getLedgerPath(dataDir, repoID, ledger.ConversationID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating ledger directory: %w", err)
	}

	if ledger.InjectedSet == nil {
		ledger.InjectedSet = make(map[string]bool)
	}

	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling ledger: %w", err)
	}

	// --- Step 2: Write to a temporary file in the same directory and rename atomically ---
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

// LoadLedger loads the persisted delivery.Ledger for conversationID, returning a newly
// initialized Ledger if the file does not exist or conversationID is empty.
func LoadLedger(dataDir, repoID, conversationID string) (*delivery.Ledger, error) {
	if conversationID == "" {
		return delivery.NewLedger(conversationID), nil
	}

	ledgerStoreMutex.Lock()
	defer ledgerStoreMutex.Unlock()

	return loadLedgerLocked(dataDir, repoID, conversationID)
}

// loadLedgerLocked reads and unmarshals the ledger file for conversationID while holding ledgerStoreMutex.
func loadLedgerLocked(dataDir, repoID, conversationID string) (*delivery.Ledger, error) {
	// --- Step 1: Read ledger file or return a fresh Ledger if not found ---
	path := getLedgerPath(dataDir, repoID, conversationID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return delivery.NewLedger(conversationID), nil
		}
		return nil, err
	}

	// --- Step 2: Unmarshal JSON, falling back to a fresh Ledger if corrupted ---
	ledger := delivery.Ledger{
		LastHandledCheckpointStep: -1,
	}
	if err := json.Unmarshal(data, &ledger); err != nil {
		// Corrupted file (e.g. truncated) — start fresh rather than failing.
		return delivery.NewLedger(conversationID), nil
	}

	// --- Step 3: Initialize nil maps and reconstruct InjectedSet from post-checkpoint injections ---
	if ledger.SeenDossiers == nil {
		ledger.SeenDossiers = make(map[string]bool)
	}
	if ledger.SeenAnchors == nil {
		ledger.SeenAnchors = make(map[string]bool)
	}
	if ledger.AcknowledgedGates == nil {
		ledger.AcknowledgedGates = make(map[string]bool)
	}
	if ledger.PendingGates == nil {
		ledger.PendingGates = make(map[string][]string)
	}
	if ledger.InjectedSet == nil {
		ledger.InjectedSet = make(map[string]bool)
		for _, inj := range ledger.Injections {
			if inj.StepIndex > ledger.LastHandledCheckpointStep && !strings.HasPrefix(inj.EntryID, "about-to-edit:") {
				ledger.InjectedSet[inj.EntryID] = true
			}
		}
	}

	return &ledger, nil
}
