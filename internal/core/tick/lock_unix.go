/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/lock_unix.go
 * Description: Non-blocking exclusive advisory file locking using POSIX flock(2)
 *              for Unix, macOS, and Linux platforms to prevent overlapping tick runs.
 *
 * Exported Types / Functions:
 * - Lock: Advisory file lock handle for preventing overlapping tick runs.
 *   - TryLock(): Attempts to acquire a non-blocking exclusive advisory file lock.
 *   - Unlock():  Releases the advisory file lock and closes the file descriptor.
 */

//go:build unix || darwin || linux

package tick

import (
	"fmt"
	"os"
	"syscall"
)

// Lock holds an active OS-level advisory file lock on the tick lockfile.
type Lock struct {
	file *os.File // Underlying open file descriptor holding the flock lock.
}

// TryLock attempts to acquire a non-blocking exclusive advisory file lock at path.
// The caller must call Unlock on the returned Lock when finished. It returns
// ErrLocked (discoverable via errors.Is) if another process already holds the lock.
func TryLock(path string) (*Lock, error) {
	// --- Step 1: Open or create the lockfile on disk ---
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	// --- Step 2: Acquire a non-blocking exclusive flock on the descriptor ---
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("flock: %w", err)
	}

	return &Lock{file: file}, nil
}

// Unlock releases the exclusive advisory lock and closes the underlying file handle.
func (l *Lock) Unlock() error {
	defer l.file.Close()
	return syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
}
