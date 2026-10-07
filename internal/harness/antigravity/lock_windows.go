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
