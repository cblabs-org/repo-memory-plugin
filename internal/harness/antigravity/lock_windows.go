//go:build windows

package antigravity

import "os"

// fileLock is a no-op on Windows. Cross-process ledger safety relies on
// the in-process mutex and atomic writes. A proper implementation using
// windows.LockFileEx can be added as a follow-up if needed.
func fileLock(f *os.File) error {
	return nil
}

// fileUnlock is a no-op on Windows.
func fileUnlock(f *os.File) error {
	return nil
}
