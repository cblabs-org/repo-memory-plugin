/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/extractor/extractor.go
 * Description: Coordinates static, deterministic (no-LLM) entity extraction
 *              across a repository tree, combining rule declarations, config
 *              keys, code symbols, and file paths.
 *
 * Exported Types / Functions:
 * - Extractor: Deterministic repository entity extractor.
 *   - New():   Constructs an Extractor rooted at the given repository path.
 *   - Run():   Walks the repository and returns all discovered entities.
 */

// Package extractor provides deterministic, no-LLM extraction of repository
// entities from source trees, configuration files, and rule manifests.
package extractor

import (
	"io/fs"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// Extractor is the main deterministic (no-LLM) entity extractor.
type Extractor struct {
	root string // Absolute or relative filesystem path to the repository root.
}

// New creates a new Extractor rooted at root.
func New(root string) *Extractor {
	return &Extractor{root: root}
}

// Run executes the extraction process over the repository and returns all discovered entities.
func (e *Extractor) Run() ([]entities.Entity, error) {
	var all []entities.Entity

	// --- Step 1: Extract explicit rule entities from config.yaml if present ---
	ruleEntities, err := ExtractFromRules(filepath.Join(e.root, "config.yaml"))
	if err == nil && ruleEntities != nil {
		all = append(all, ruleEntities...)
	}

	// --- Step 2: Walk non-ignored repository files and extract config, symbol, and file entities ---
	err = Walk(e.root, func(path string, d fs.DirEntry) error {
		if d.IsDir() {
			return nil
		}

		// Config entities
		cfgEntities, err := ParseConfig(path)
		if err == nil && cfgEntities != nil {
			all = append(all, cfgEntities...)
		}

		// Symbol entities
		symEntities, err := ExtractSymbols(path)
		if err == nil && symEntities != nil {
			all = append(all, symEntities...)
		}

		// File entity for the visited path relative to the repository root
		rel, _ := filepath.Rel(e.root, path)
		all = append(all, entities.Entity{
			ID:   "file:" + rel,
			Kind: entities.KindFile,
			Name: rel,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return all, nil
}
