/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/subsystem.go
 * Description: Subsystem glob matching and unassigned file identification for
 *              repository configurations (.memory/config.yaml).
 *
 * Exported Types / Functions:
 * - RepoConfig.AssignSubsystem(): Returns the subsystem name matching a file path.
 * - RepoConfig.UnassignedFiles(): Returns files that do not match any configured subsystem.
 */

package config

import (
	"path/filepath"
	"sort"
	"strings"
)

// AssignSubsystem returns the name of the subsystem that the given file belongs to.
// It iterates through the subsystems and returns the first one where a glob matches the file path.
// Returns an empty string if no subsystem matches.
func (c RepoConfig) AssignSubsystem(filePath string) string {
	// --- Step 1: Sort subsystem names for deterministic evaluation order ---
	var names []string
	for name := range c.Subsystems {
		names = append(names, name)
	}
	sort.Strings(names)

	// --- Step 2: Match filePath against each subsystem's configured globs ---
	for _, name := range names {
		subsystem := c.Subsystems[name]
		for _, pattern := range subsystem.Globs {
			matched, err := filepath.Match(pattern, filePath)
			if err == nil && matched {
				return name
			}

			// Support ** for recursive directory matching
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				if strings.HasPrefix(filePath, prefix+"/") {
					return name
				}
			}
		}
	}
	return ""
}

// UnassignedFiles returns a list of files from the given list that do not match any subsystem.
func (c RepoConfig) UnassignedFiles(files []string) []string {
	var unassigned []string
	for _, f := range files {
		if c.AssignSubsystem(f) == "" {
			unassigned = append(unassigned, f)
		}
	}
	return unassigned
}
