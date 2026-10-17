/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/index/populate.go
 * Description: Populates the episodes FTS5 table by reading and aggregating
 *               persisted episode files from an episode.Store.
 *
 * Exported Types / Functions:
 * - Index:
 *   - PopulateFromStore(): Reads all episodes from an episode.Store and indexes them.
 */

package index

import (
	"context"
	"fmt"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
)

// PopulateFromStore reads all episodes from the given store and indexes them.
func (idx *Index) PopulateFromStore(ctx context.Context, store *episode.Store) error {
	// --- Step 1: Enumerate all episode file paths in the store ---
	paths, err := store.ListPaths()
	if err != nil {
		return fmt.Errorf("listing episode paths: %w", err)
	}

	// --- Step 2: Read each episode and aggregate its steps into searchable fields ---
	for _, path := range paths {
		ep, err := store.ReadFromPath(path)
		if err != nil {
			return fmt.Errorf("reading episode from %s: %w", path, err)
		}

		var contentBuilder strings.Builder
		var filesTouched []string
		var outcomes []string

		for _, step := range ep.Steps {
			if step.Text != "" {
				contentBuilder.WriteString(step.Text)
				contentBuilder.WriteString("\n")
			}
			if step.Command != "" {
				contentBuilder.WriteString(step.Command)
				contentBuilder.WriteString("\n")
			}
			if step.File != "" {
				filesTouched = append(filesTouched, step.File)
			}
			if step.Outcome != "" {
				outcomes = append(outcomes, step.Outcome)
			}
		}

		// --- Step 3: Build and insert the FTS episode record ---
		record := EpisodeRecord{
			EpisodeID:    ep.Header.EpisodeID,
			SessionID:    ep.Header.SessionID,
			Content:      contentBuilder.String(),
			FilesTouched: strings.Join(unique(filesTouched), " "),
			Outcomes:     strings.Join(outcomes, "\n"),
			Date:         ep.Header.Started,
		}
		if record.Date.IsZero() {
			record.Date = ep.Header.Ended
		}

		if err := idx.InsertEpisode(ctx, record); err != nil {
			return fmt.Errorf("inserting episode %s: %w", ep.Header.EpisodeID, err)
		}
	}

	return nil
}

// unique returns a deduplicated slice of non-empty strings preserving first-seen order.
func unique(strs []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range strs {
		if !seen[s] && s != "" {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
