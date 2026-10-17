/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/marker.go
 * Description: Zero-byte filesystem marker (.memory/.enrolled) creation,
 *              removal, and sub-millisecond ancestor traversal for fast-path
 *              hook enrolment checks.
 *
 * Exported Types / Functions:
 * - EnrolledMarkerFile:     Filename of the zero-byte enrolment marker (".enrolled").
 * - EnrolledMarkerPath():   Returns the path to .memory/.enrolled for a repository root.
 * - CreateEnrolledMarker(): Creates the zero-byte .memory/.enrolled marker file.
 * - RemoveEnrolledMarker(): Removes the .memory/.enrolled marker file if present.
 * - IsEnrolledFast():       Checks whether a directory or any repo ancestor has .memory/.enrolled.
 */

package config

import (
	"os"
	"path/filepath"
)

// EnrolledMarkerFile is the filename of the zero-byte marker indicating repo enrolment.
const EnrolledMarkerFile = ".enrolled"

// EnrolledMarkerPath returns the path to the .enrolled marker file for a given repo root.
func EnrolledMarkerPath(repoRoot string) string {
	if repoRoot == "" {
		repoRoot = "."
	}
	return filepath.Join(repoRoot, ".memory", EnrolledMarkerFile)
}

// CreateEnrolledMarker creates the zero-byte .memory/.enrolled marker file in repoRoot.
// It creates the parent .memory directory if it does not already exist.
func CreateEnrolledMarker(repoRoot string) error {
	if err := EnsureRepoLayout(repoRoot); err != nil {
		return err
	}
	markerPath := EnrolledMarkerPath(repoRoot)
	f, err := os.OpenFile(markerPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// RemoveEnrolledMarker removes the .memory/.enrolled marker file in repoRoot.
// If the marker file does not exist, it returns nil.
func RemoveEnrolledMarker(repoRoot string) error {
	markerPath := EnrolledMarkerPath(repoRoot)
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsEnrolledFast checks whether the given directory (or any ancestor within its repository)
// contains the .memory/.enrolled marker file.
// It uses pure filesystem stats (O(1) filesystem stat, no SQLite or subprocess)
// to satisfy the <= 5 ms fast-path latency requirement.
func IsEnrolledFast(dir string) bool {
	// --- Step 1: Resolve starting directory to an absolute path ---
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir == "" {
		return false
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}

	// --- Step 2: Walk upward checking for .memory/.enrolled or a .git boundary ---
	curr := abs
	for {
		marker := filepath.Join(curr, ".memory", EnrolledMarkerFile)
		if fi, err := os.Stat(marker); err == nil && !fi.IsDir() {
			if entry, found, _ := FindRegistryEntry(DataDir(), curr); found && entry != nil {
				if entry.Enabled != nil && !*entry.Enabled {
					return false
				}
			}
			return true
		}
		// If we find a .git root and no .memory/.enrolled was present, this repository is not enrolled.
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			return false
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return false
}
