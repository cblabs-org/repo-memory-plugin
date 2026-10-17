/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/extractor/symbol.go
 * Description: Extracts function and prototype symbol entities from C and C++
 *              source/header files using ctags when available or a brace-aware
 *              regex fallback scanner.
 *
 * Exported Types / Functions:
 * - ExtractSymbols(): Extracts KindSymbol entities from C/C++ source and header files.
 */

package extractor

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// funcRegex matches top-level C/C++ function declarations and definitions.
var funcRegex = regexp.MustCompile(`^[\w\s\*&]+\s+(\w+)\s*\([^)]*\)\s*\{?$`)

// ExtractSymbols extracts function and prototype symbols from C/C++ files at path.
// Non-C/C++ file extensions return nil, nil.
func ExtractSymbols(path string) ([]entities.Entity, error) {
	// --- Step 1: Filter by C/C++ source and header extensions ---
	ext := filepath.Ext(path)
	if ext != ".c" && ext != ".cpp" && ext != ".h" && ext != ".hpp" {
		return nil, nil
	}

	// --- Step 2: Attempt extraction via universal/exuberant ctags ---
	if ctagsPath, err := exec.LookPath("ctags"); err == nil {
		cmd := exec.Command(ctagsPath, "-x", "--c-kinds=fp", path)
		out, err := cmd.Output()
		if err == nil {
			var ext []entities.Entity
			scanner := bufio.NewScanner(bytes.NewReader(out))
			for scanner.Scan() {
				parts := strings.Fields(scanner.Text())
				if len(parts) > 0 {
					ext = append(ext, entities.Entity{
						ID:   "symbol:" + parts[0],
						Kind: entities.KindSymbol,
						Name: parts[0],
					})
				}
			}
			return ext, nil
		}
	}

	// --- Step 3: Fall back to the built-in brace-aware scanner ---
	return fallbackCScanner(path)
}

// fallbackCScanner scans a C/C++ file line by line, tracking brace depth to identify
// top-level function definitions and prototypes when ctags is unavailable.
func fallbackCScanner(path string) ([]entities.Entity, error) {
	// --- Step 1: Open the source file ---
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// --- Step 2: Track brace nesting depth and match top-level function signatures ---
	var extracted []entities.Entity
	scanner := bufio.NewScanner(f)
	braceLevel := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Adjust brace nesting depth across the current line.
		braceLevel += strings.Count(line, "{")
		braceLevel -= strings.Count(line, "}")

		if braceLevel == 0 || (braceLevel == 1 && strings.HasSuffix(line, "{")) {
			// Match candidate function signatures at top-level scope.
			if matches := funcRegex.FindStringSubmatch(line); len(matches) > 1 {
				name := matches[1]
				if name != "if" && name != "for" && name != "while" && name != "switch" {
					extracted = append(extracted, entities.Entity{
						ID:   "symbol:" + name,
						Kind: entities.KindSymbol,
						Name: name,
					})
				}
			}
		}
	}
	return extracted, nil
}
