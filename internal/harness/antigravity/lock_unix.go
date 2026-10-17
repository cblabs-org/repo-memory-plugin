/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/lock_unix.go
 * Description: Provides POSIX flock-based exclusive file locking for cross-process
 *              conversation ledger synchronization on Unix platforms.
 *
 * Exported Types / Functions:
 * - None (package-private OS lock helpers).
 */

//go:build !windows

package antigravity

import (
	"os"
	"syscall"
)

// fileLock acquires an exclusive lock on the given file using flock.
func fileLock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// fileUnlock releases the lock on the given file.
func fileUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
