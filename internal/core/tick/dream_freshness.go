/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/dream_freshness.go
 * Description: Deterministic "Dreaming" document-freshness reconciliation (SPEC §6.3.5).
 *              Compares active doc-seed entries against current git blob SHAs on disk,
 *              amending modified entries, retiring deleted docs or bullets, re-projecting
 *              dossiers, and rebuilding the SQLite search index.
 *
 * Exported Types / Functions:
 * - PipelineExecutor.CheckDocFreshness(): Reconciles active doc-seed entries against disk and updates dossiers/index.
 */

package tick

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

// CheckDocFreshness performs deterministic "Dreaming" doc-freshness reconciliation (SPEC §6.3.5).
// It compares active `source: "doc-seed"` entries against the current git blob SHA of their
// source markdown files on disk, amending modified entries, retiring entries from deleted docs
// or removed bullets, re-projecting `.memory/dossiers/` and `.memory/ATLAS.md`, and rebuilding `index.db`.
func (p *PipelineExecutor) CheckDocFreshness(ctx context.Context) (*seed.DocSeedResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// --- Step 1: Drain queued dream-freshness and seed-docs jobs from the store ---
	var acquiredJobs []*jobs.Job
	if p.store != nil {
		for {
			job, err := p.store.AcquireNext(ctx, 5*time.Minute, jobs.KindDreamFreshness, jobs.KindSeedDocs)
			if err != nil {
				if errors.Is(err, jobs.ErrNoJobAvailable) {
					break
				}
				return nil, err
			}
			if job == nil {
				break
			}
			acquiredJobs = append(acquiredJobs, job)
		}
	}

	finishJobs := func(runErr error) {
		if p.store == nil {
			return
		}
		for _, job := range acquiredJobs {
			if runErr != nil {
				_ = p.store.Fail(ctx, job.ID, runErr.Error(), 5*time.Minute)
			} else {
				_ = p.store.Complete(ctx, job.ID, nil)
			}
		}
	}

	if p.repoRoot == "" {
		finishJobs(nil)
		return &seed.DocSeedResult{}, nil
	}

	// --- Step 2: Load repository and scope configurations ---
	opsDir := filepath.Join(p.repoRoot, ".memory", "ops")
	configPath := filepath.Join(p.repoRoot, ".memory", "config.yaml")

	repoCfg, err := config.LoadRepoConfig(p.repoRoot)
	if err != nil && !os.IsNotExist(err) {
		finishJobs(err)
		return nil, fmt.Errorf("loading repo config: %w", err)
	}
	if len(p.cfg.DocSeeds) > 0 && len(repoCfg.DocSeeds) == 0 {
		repoCfg.DocSeeds = p.cfg.DocSeeds
	}

	scopeCfg, err := config.LoadScopeConfig(configPath)
	if err != nil && !os.IsNotExist(err) {
		finishJobs(err)
		return nil, fmt.Errorf("loading scope config: %w", err)
	}
	if scopeCfg == nil {
		scopeCfg = &config.ScopeConfig{}
	}

	// --- Step 3: Check whether any active doc-seed entries exist ---
	opsReader := ops.NewReader(opsDir)
	hasActiveDocSeedEntries := false
	if fi, statErr := os.Stat(opsDir); statErr == nil && fi.IsDir() {
		proj := dossiers.NewProjector(opsReader)
		projected, projErr := proj.Project(ctx)
		if projErr != nil {
			finishJobs(projErr)
			return nil, fmt.Errorf("projecting existing ops: %w", projErr)
		}
		for _, d := range projected {
			for _, e := range d.Entries {
				if !e.Retired && e.Source == ops.SourceDocSeed {
					hasActiveDocSeedEntries = true
					break
				}
			}
			if hasActiveDocSeedEntries {
				break
			}
		}
	}

	// Fast no-op for unseeded workspaces without doc_seeds config or explicit dream/seed-docs jobs.
	if !hasActiveDocSeedEntries && len(repoCfg.DocSeeds) == 0 && len(acquiredJobs) == 0 {
		return &seed.DocSeedResult{}, nil
	}

	// --- Step 4: Execute deterministic doc-seed reconciliation ---
	res, err := seed.SeedDocs(
		ctx,
		seed.DocSeedOptions{
			RepoRoot: p.repoRoot,
			OpsDir:   opsDir,
			RepoCfg:  repoCfg,
		},
		scopeCfg,
		opsReader,
		ops.NewWriter(opsDir),
	)
	if err != nil {
		finishJobs(err)
		return nil, fmt.Errorf("checking doc freshness: %w", err)
	}

	// --- Step 5: Re-project dossiers and rebuild SQLite index when entries mutated ---
	if res.EntriesAdded+res.EntriesAmend+res.EntriesRetire > 0 {
		proj := dossiers.NewProjector(opsReader)
		dossiersMap, projErr := proj.Project(ctx)
		if projErr != nil {
			finishJobs(projErr)
			return nil, fmt.Errorf("projecting dossiers after doc freshness check: %w", projErr)
		}
		writer := dossiers.NewWriter(filepath.Join(p.repoRoot, ".memory"))
		if writeErr := writer.WriteAll(dossiersMap); writeErr != nil {
			finishJobs(writeErr)
			return nil, fmt.Errorf("writing projected dossiers after doc freshness check: %w", writeErr)
		}

		if p.idx != nil {
			if rebuildErr := p.idx.RebuildFromOpLog(ctx, opsReader); rebuildErr != nil {
				finishJobs(rebuildErr)
				return nil, fmt.Errorf("rebuilding index after doc freshness check: %w", rebuildErr)
			}
		} else if p.dataDir != "" && p.repoID != "" {
			idxPath := index.IndexPath(p.dataDir, p.repoID)
			if mkdirErr := os.MkdirAll(filepath.Dir(idxPath), 0755); mkdirErr != nil {
				finishJobs(mkdirErr)
				return nil, fmt.Errorf("creating index directory: %w", mkdirErr)
			}
			idx, idxErr := index.NewIndex(idxPath)
			if idxErr != nil {
				finishJobs(idxErr)
				return nil, fmt.Errorf("opening index after doc freshness check: %w", idxErr)
			}
			defer idx.Close()
			if rebuildErr := idx.RebuildFromOpLog(ctx, opsReader); rebuildErr != nil {
				finishJobs(rebuildErr)
				return nil, fmt.Errorf("rebuilding index after doc freshness check: %w", rebuildErr)
			}
		}
	}

	finishJobs(nil)
	return res, nil
}

// checkDocFreshness runs CheckDocFreshness and discards the result payload.
func (p *PipelineExecutor) checkDocFreshness(ctx context.Context) error {
	_, err := p.CheckDocFreshness(ctx)
	return err
}
