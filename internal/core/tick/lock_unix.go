//go:build unix || darwin || linux

package tick

import (
	"fmt"
	"os"
	"syscall"
)

type Lock struct {
	file *os.File
}

func TryLock(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

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

func (l *Lock) Unlock() error {
	defer l.file.Close()
	return syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
}
