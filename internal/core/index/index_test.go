/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package index_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestIndexSearch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	now := time.Now()

	err = idx.InsertEpisode(ctx, index.EpisodeRecord{
		EpisodeID:    "ep1",
		SessionID:    "sess1",
		Content:      "this is some test content about banana",
		FilesTouched: "main.go",
		Outcomes:     "success",
		Date:         now,
	})
	if err != nil {
		t.Fatalf("InsertEpisode: %v", err)
	}

	results, err := idx.SearchEpisodes(ctx, "banana")
	if err != nil {
		t.Fatalf("SearchEpisodes: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].EpisodeID != "ep1" {
		t.Errorf("expected ep1, got %s", results[0].EpisodeID)
	}
}

func TestPopulateFromStore(t *testing.T) {
	ctx := context.Background()
	storeDir := t.TempDir()
	store := episode.NewStore(storeDir, "repo-1")

	ep := episode.NewEpisode(episode.Header{
		EpisodeID: "ep-populate",
		SessionID: "sess-populate",
		Started:   time.Now(),
	}, harness.Step{
		Text: "apple orange",
		File: "apple.go",
	})
	err := store.Write(ep)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	idx, err := index.NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	defer idx.Close()

	if err := idx.PopulateFromStore(ctx, store); err != nil {
		t.Fatalf("PopulateFromStore: %v", err)
	}

	results, err := idx.SearchEpisodes(ctx, "orange")
	if err != nil {
		t.Fatalf("SearchEpisodes: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].EpisodeID != "ep-populate" {
		t.Errorf("expected ep-populate, got %s", results[0].EpisodeID)
	}
	if results[0].FilesTouched != "apple.go" {
		t.Errorf("expected apple.go, got %s", results[0].FilesTouched)
	}
}
