/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/lock_windows.go
 * Description: Non-blocking exclusive file locking using Win32 LockFileEx and
 *              UnlockFileEx APIs on Windows to prevent overlapping tick runs.
 *
 * Exported Types / Functions:
 * - Lock: Advisory file lock handle for preventing overlapping tick runs.
 *   - TryLock(): Attempts to acquire a non-blocking exclusive file lock.
 *   - Unlock():  Releases the file lock and closes the file handle.
 */

//go:build windows

package tick

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")  // Lazy handle to Windows kernel32.dll.
	procLockFileEx   = modkernel32.NewProc("LockFileEx")   // Win32 LockFileEx procedure entry point.
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx") // Win32 UnlockFileEx procedure entry point.
)

const (
	lockFileExclusiveLock   = 2 // Win32 LOCKFILE_EXCLUSIVE_LOCK flag requesting an exclusive byte-range lock.
	lockFileFailImmediately = 1 // Win32 LOCKFILE_FAIL_IMMEDIATELY flag returning immediately if locked.
)

// Lock holds an active OS-level file lock on the tick lockfile.
type Lock struct {
	file *os.File // Underlying open file handle holding the Win32 byte-range lock.
}

// TryLock attempts to acquire a non-blocking exclusive file lock at path.
// The caller must call Unlock on the returned Lock when finished. It returns
// ErrLocked (discoverable via errors.Is) if another process already holds the lock.
func TryLock(path string) (*Lock, error) {
	// --- Step 1: Open or create the lockfile on disk ---
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	// --- Step 2: Acquire a non-blocking exclusive byte-range lock via LockFileEx ---
	var overlapped syscall.Overlapped
	ret, _, err := procLockFileEx.Call(
		uintptr(file.Fd()),
		uintptr(lockFileExclusiveLock|lockFileFailImmediately),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret == 0 {
		file.Close()
		return nil, ErrLocked
	}

	return &Lock{file: file}, nil
}

// Unlock releases the exclusive Win32 byte-range lock and closes the underlying file handle.
func (l *Lock) Unlock() error {
	// --- Step 1: Ensure the underlying file handle closes on return ---
	defer l.file.Close()

	// --- Step 2: Release the locked byte range via UnlockFileEx ---
	var overlapped syscall.Overlapped
	ret, _, err := procUnlockFileEx.Call(
		uintptr(l.file.Fd()),
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ret == 0 {
		return fmt.Errorf("UnlockFileEx: %w", err)
	}
	return nil
}
