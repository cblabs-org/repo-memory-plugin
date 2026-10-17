/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/datadir.go
 * Description: Platform-aware resolution of the user data directory for
 *              repo-memory across macOS, Linux/BSD (XDG), and Windows, with
 *              environment variable override support.
 *
 * Exported Types / Functions:
 * - DataDir(): Returns the platform-appropriate user data directory for repo-memory.
 */

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

// dataDirForOS resolves the user data directory for the specified OS using the
// injected environment and home-directory lookup functions.
func dataDirForOS(goos string, getenv func(string) string, userHomeDir func() (string, error)) string {
	// --- Step 1: Check explicit environment override ---
	if override := getenv("REPO_MEMORY_DATA_DIR"); override != "" {
		return override
	}

	// --- Step 2: Resolve user home directory ---
	home, _ := userHomeDir()

	// --- Step 3: Select platform-specific application data path ---
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
