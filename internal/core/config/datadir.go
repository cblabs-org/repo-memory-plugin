package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataDir returns the platform-appropriate user data directory for repo-memory:
// - macOS: ~/Library/Application Support/repo-memory
// - Linux: ~/.local/share/repo-memory (or $XDG_DATA_HOME/repo-memory)
// - Windows: %LOCALAPPDATA%\repo-memory (or AppData\Local\repo-memory)
//
// If REPO_MEMORY_DATA_DIR is set in the environment, it takes precedence.
func DataDir() string {
	return dataDirForOS(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

func dataDirForOS(goos string, getenv func(string) string, userHomeDir func() (string, error)) string {
	if override := getenv("REPO_MEMORY_DATA_DIR"); override != "" {
		return override
	}

	home, _ := userHomeDir()

	switch goos {
	case "darwin":
		if home != "" {
			return filepath.Join(home, "Library", "Application Support", "repo-memory")
		}
		return filepath.Join("Library", "Application Support", "repo-memory")

	case "windows":
		if localAppData := getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "repo-memory")
		}
		if home != "" {
			return filepath.Join(home, "AppData", "Local", "repo-memory")
		}
		return filepath.Join("AppData", "Local", "repo-memory")

	default: // linux, freebsd, openbsd, etc.
		if xdg := getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "repo-memory")
		}
		if home != "" {
			return filepath.Join(home, ".local", "share", "repo-memory")
		}
		return filepath.Join(".local", "share", "repo-memory")
	}
}
