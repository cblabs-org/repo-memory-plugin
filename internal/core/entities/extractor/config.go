/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/extractor/config.go
 * Description: Heuristic parser that extracts configuration and dependency
 *              entities from SDK configs, INI files, lockfiles, and go.mod.
 *
 * Exported Types / Functions:
 * - ParseConfig(): Extracts KindConfig entities from supported configuration files.
 */

package extractor

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// ParseConfig parses config files and lockfiles at path for config entities.
// It returns nil, nil when path is not a recognized configuration file name.
func ParseConfig(path string) ([]entities.Entity, error) {
	// --- Step 1: Filter by supported configuration filenames ---
	var extracted []entities.Entity
	base := filepath.Base(path)

	isConfig := false
	if base == "sdkconfig" || base == "Kconfig" || base == "platformio.ini" || strings.HasSuffix(base, ".lock") || base == "go.mod" {
		isConfig = true
	}
	if !isConfig {
		return nil, nil
	}

	// --- Step 2: Open configuration file for line-by-line scanning ---
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// --- Step 3: Extract key-value assignments and Go module requirements ---
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// For sdkconfig/Kconfig/platformio: usually KEY=VALUE or KEY = VALUE.
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			if key != "" {
				extracted = append(extracted, entities.Entity{
					ID:   "config:" + key,
					Kind: entities.KindConfig,
					Name: key,
				})
			}
		} else if strings.HasPrefix(line, "require ") { // Single-line go.mod require directive.
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				extracted = append(extracted, entities.Entity{
					ID:   "config:" + parts[1],
					Kind: entities.KindConfig,
					Name: parts[1],
				})
			}
		}
	}
	return extracted, nil
}
