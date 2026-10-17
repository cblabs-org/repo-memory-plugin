/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/lock_windows.go
 * Description: Provides Windows stub implementations for ledger file locking,
 *              relying on in-process mutex synchronization and atomic renames.
 *
 * Exported Types / Functions:
 * - None (package-private OS lock helpers).
 */

//go:build windows

package antigravity

import "os"

// fileLock is a no-op on Windows. Cross-process ledger safety relies on
// the in-process mutex and atomic writes.
// TODO: Implement using windows.LockFileEx from golang.org/x/sys/windows
// for true cross-process safety. ModifyLedger's doc comment promises
// protection across processes, but on Windows that currently only covers
// goroutines within one process.
func fileLock(f *os.File) error {
	return nil
}

// fileUnlock is a no-op on Windows.
func fileUnlock(f *os.File) error {
	return nil
}
