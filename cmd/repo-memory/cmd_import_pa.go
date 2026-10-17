/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_import_pa.go
 * Description: Implements the `repo-memory import-pa` command and helper to import
 *              Principal Architect workspace artifacts (PROJECT.md, WORKORDER.md,
 *              context_bridge.md, MEMORY.md, MODULE_INDEX.md, PROJECT_INDEX.md) as
 *              episodes across both .agents/plugins/ and .agents/skills/ layouts.
 *
 * Exported Types / Functions:
 * - ImportPAWorkspace(): Walks PA workspace directories and imports artifacts into episode.Store.
 */

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/spf13/cobra"
)

var importPARepoDir string // Target repository root path for `repo-memory import-pa --repo`.

// importPACmd defines the `repo-memory import-pa` subcommand.
var importPACmd = &cobra.Command{
	Use:   "import-pa [dir]",
	Short: "Import Principal Architect files as episodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve repository root and repository ID ---
		repoRoot, err := gitops.FindRepoRoot(importPARepoDir)
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		repoID, err := gitops.GenerateRepoID(repoRoot)
		if err != nil {
			return fmt.Errorf("generating repo id: %w", err)
		}

		customDir := ""
		if len(args) > 0 {
			customDir = args[0]
		}

		// --- Step 2: Import PA workspace artifacts and print summary ---
		importedCount, err := ImportPAWorkspace(repoRoot, config.DataDir(), repoID, customDir, true)
		if err != nil {
			return err
		}
		fmt.Printf("Imported %d PA files.\n", importedCount)
		return nil
	},
}

func init() {
	importPACmd.Flags().StringVar(&importPARepoDir, "repo", ".", "Path to the target repository root")
}

// ImportPAWorkspace walks Principal Architect workspace directories under repoRoot
// (either customDir when provided, or both .agents/plugins/principal-architect-workspace
// and .agents/skills/principal-architect-workspace) and imports PA markdown artifacts
// as episodes in dataDir.
func ImportPAWorkspace(repoRoot, dataDir, repoID, customDir string, verbose bool) (int, error) {
	// --- Step 1: Open episode store, SQLite index, and resolve scan directories ---
	store := episode.NewStore(dataDir, repoID)
	dbPath := index.IndexPath(dataDir, repoID)
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)
	idx, _ := index.NewIndex(dbPath)
	if idx != nil {
		defer idx.Close()
	}

	targetFiles := map[string]bool{
		"PROJECT.md":        true,
		"WORKORDER.md":      true,
		"context_bridge.md": true,
		"MEMORY.md":         true,
		"MODULE_INDEX.md":   true,
		"PROJECT_INDEX.md":  true,
	}

	var dirs []string
	if customDir != "" {
		if filepath.IsAbs(customDir) {
			dirs = []string{customDir}
		} else {
			dirs = []string{filepath.Join(repoRoot, customDir)}
		}
	} else {
		for _, rel := range []string{
			filepath.Join(".agents", "plugins", "principal-architect-workspace"),
			filepath.Join(".agents", "skills", "principal-architect-workspace"),
		} {
			cand := filepath.Join(repoRoot, rel)
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				dirs = append(dirs, cand)
			}
		}
	}

	// --- Step 2: Walk PA directories and import matching markdown files as episodes ---
	importedCount := 0
	for _, scanRoot := range dirs {
		walkErr := filepath.Walk(scanRoot, func(filePath string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !targetFiles[info.Name()] {
				return nil
			}

			contentBytes, err := os.ReadFile(filePath)
			if err != nil {
				return nil
			}
			content := string(contentBytes)
			date := info.ModTime()

			relSlash := filePath
			if rel, relErr := filepath.Rel(repoRoot, filePath); relErr == nil {
				relSlash = filepath.ToSlash(rel)
			}
			sum := sha256.Sum256([]byte(relSlash))
			sessionID := fmt.Sprintf("pa-%s-%s", info.Name(), hex.EncodeToString(sum[:8]))
			if store.Exists(sessionID) {
				return nil
			}

			epID := episode.NewEpisodeIDWithTime(date)
			header := episode.Header{
				EpisodeID: epID,
				Harness:   "import",
				SessionID: sessionID,
				RepoID:    repoID,
				Started:   date,
				Ended:     date,
			}

			step := harness.Step{
				Index: 1,
				Kind:  harness.StepKindNote,
				Text:  content,
				File:  filePath,
			}

			ep := episode.NewEpisode(header, step)
			if err := store.Write(ep); err != nil {
				return fmt.Errorf("writing episode %s: %w", filePath, err)
			}
			if idx != nil {
				_ = idx.InsertEpisode(context.Background(), index.EpisodeRecord{
					EpisodeID:    epID,
					SessionID:    sessionID,
					Content:      content,
					FilesTouched: relSlash,
					Outcomes:     "pa-import",
					Date:         date,
				})
			}
			importedCount++
			if verbose {
				fmt.Printf("Imported %s as episode %s\n", filePath, epID)
			}
			return nil
		})
		if walkErr != nil {
			return importedCount, walkErr
		}
	}

	return importedCount, nil
}
