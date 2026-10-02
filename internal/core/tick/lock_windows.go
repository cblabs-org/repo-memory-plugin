//go:build windows

package tick

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	LOCKFILE_EXCLUSIVE_LOCK   = 2
	LOCKFILE_FAIL_IMMEDIATELY = 1
)

type Lock struct {
	file *os.File
}

func TryLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	var overlapped syscall.Overlapped
	ret, _, err := procLockFileEx.Call(
		uintptr(file.Fd()),
		uintptr(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY),
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

func (l *Lock) Unlock() error {
	defer l.file.Close()
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
