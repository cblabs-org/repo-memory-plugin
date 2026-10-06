package ops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MigrateV1Events moves the v1 events directory to the archive directory if it exists.
// If eventsDir does not exist, it is a no-op and returns nil.
func MigrateV1Events(eventsDir, archiveDir string) error {
	info, err := os.Stat(eventsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat events dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("events path %s is not a directory", eventsDir)
	}

	cleanEvents := filepath.Clean(eventsDir)
	cleanArchive := filepath.Clean(archiveDir)
	if cleanEvents == cleanArchive {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cleanArchive), 0o755); err != nil {
		return fmt.Errorf("create archive parent dir: %w", err)
	}

	// Try direct atomic rename first
	if err := os.Rename(cleanEvents, cleanArchive); err == nil {
		return nil
	}

	// If destination already exists or rename fails across devices, move contents recursively
	if err := moveDirContents(cleanEvents, cleanArchive); err != nil {
		return fmt.Errorf("migrate events from %s to %s: %w", cleanEvents, cleanArchive, err)
	}

	return os.RemoveAll(cleanEvents)
}

func moveDirContents(srcDir, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		if entry.IsDir() {
			if err := moveDirContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := os.Rename(srcPath, dstPath); err != nil {
				if err := copyFile(srcPath, dstPath); err != nil {
					return err
				}
				if err := os.Remove(srcPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
