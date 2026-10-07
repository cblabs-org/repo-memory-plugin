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
