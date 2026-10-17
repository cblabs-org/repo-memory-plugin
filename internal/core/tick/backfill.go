/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/backfill.go
 * Description: Historical backfill execution across transcripts, Principal Architect
 *              workspace artifacts, markdown documentation seeds, and git revert commits.
 *
 * Exported Types / Functions:
 * - QuotaSnapshot:                 Alias for telemetry.QuotaStatus used in backfill quota checks.
 * - BackfillOptions:               Configuration parameters for a historical backfill run.
 * - BackfillResult:                Summary metrics and created artifact IDs from a backfill run.
 * - RunBackfill():                 Convenience entry point that constructs a PipelineExecutor and runs backfill.
 * - PipelineExecutor.RunBackfill(): Executes historical backfill on an initialized PipelineExecutor.
 */

package tick

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// QuotaSnapshot aliases telemetry.QuotaStatus for backfill quota checks.
type QuotaSnapshot = telemetry.QuotaStatus

// BackfillOptions configures a historical backfill execution.
type BackfillOptions struct {
	RepoRoot           string                                        // Absolute path to the repository root.
	DataDir            string                                        // Base memory data directory path.
	RepoID             string                                        // Registry slug identifying the repository under <data-dir>/<repo-id>.
	DryRun             bool                                          // When true, computes backfill actions without writing to disk.
	Force              bool                                          // When true, bypasses spend guard interval and quota floor checks.
	IncludeTranscripts bool                                          // Whether to backfill transcripts and Principal Architect files.
	IncludeDocs        bool                                          // Whether to seed markdown documentation files.
	IncludeGit         bool                                          // Whether to scan git history for revert commits.
	MaxItems           int                                           // Optional upper bound on total items processed in this run.
	QuotaChecker       func(context.Context) (*QuotaSnapshot, error) // Optional quota probe callback.
}

// BackfillResult reports the outcome of a historical backfill run.
type BackfillResult struct {
	RepoID            string   `json:"repo_id"`                      // Registry slug of the repository that was backfilled.
	DryRun            bool     `json:"dry_run"`                      // Whether the backfill executed in dry-run mode.
	SpendGuardAllowed bool     `json:"spend_guard_allowed"`          // Whether spend guards permitted execution.
	SpendGuardReason  string   `json:"spend_guard_reason,omitempty"` // Reason spend guard blocked execution, if any.
	EffectiveMaxItems int      `json:"effective_max_items"`          // Effective item cap applied to the backfill run.
	ItemsProcessed    int      `json:"items_processed"`              // Total items processed across all sources.
	CapturesRequeued  int      `json:"captures_requeued"`            // Number of historical capture jobs requeued.
	PAFilesImported   int      `json:"pa_files_imported"`            // Number of Principal Architect files imported as episodes.
	EpisodesQueued    int      `json:"episodes_queued"`              // Number of uncovered flat episodes queued.
	DocsSeeded        int      `json:"docs_seeded"`                  // Number of markdown documentation files seeded.
	DocEntriesMutated int      `json:"doc_entries_mutated"`          // Total doc-seed entries added, amended, or retired.
	GitRevertsScanned int      `json:"git_reverts_scanned"`          // Number of git revert commits inspected.
	GitNotesCreated   int      `json:"git_notes_created"`            // Number of inbox notes created from git reverts.
	CreatedNoteIDs    []string `json:"created_note_ids,omitempty"`   // IDs of git-revert notes created during the run.
	ImportedPAFiles   []string `json:"imported_pa_files,omitempty"`  // Repository-relative paths of imported PA files.
}

// backfillPAState tracks previously imported Principal Architect files and queued flat episodes.
type backfillPAState struct {
	ImportedFiles  map[string]string `json:"imported_files"`  // Map of repo-relative PA file path to deterministic session ID.
	QueuedEpisodes map[string]bool   `json:"queued_episodes"` // Set of flat episode IDs already queued for backfill.
}

// gitRevertNoteDisk represents the on-disk JSON schema for a git-revert backfill note.
type gitRevertNoteDisk struct {
	ID         string           `json:"id"`                    // SHA-derived note ID ("note-git-revert-<short-sha>") for idempotency.
	Kind       string           `json:"kind"`                  // Entry kind (e.g. "gotcha").
	Scope      string           `json:"scope"`                 // Dossier scope slug derived from the reverted commit's touched files.
	Statement  string           `json:"statement"`             // Concise revert summary statement.
	Anchors    []string         `json:"anchors,omitempty"`     // Files touched by the revert commit.
	Source     inbox.NoteSource `json:"source"`                // Note source identifier ("backfill-git").
	HeadCommit string           `json:"head_commit,omitempty"` // Git commit SHA of the revert.
	CreatedAt  string           `json:"created_at"`            // Commit timestamp formatted in RFC3339 UTC.
	DedupKey   string           `json:"dedup_key,omitempty"`   // Deduplication key matching ID.
	Items      []inbox.NoteItem `json:"items,omitempty"`       // Structured inbox note items.
}

// RunBackfill executes historical backfill across transcripts/PA artifacts, docs, and git reverts.
func RunBackfill(ctx context.Context, opts BackfillOptions) (*BackfillResult, error) {
	epStore := episode.NewStore(opts.DataDir, opts.RepoID)
	executor := NewPipelineExecutor(opts.DataDir, opts.RepoRoot, opts.RepoID, nil, epStore)
	if opts.QuotaChecker != nil {
		executor.QuotaChecker = opts.QuotaChecker
	}
	return executor.RunBackfill(ctx, opts)
}

// RunBackfill executes historical backfill on an initialized PipelineExecutor.
func (p *PipelineExecutor) RunBackfill(ctx context.Context, opts BackfillOptions) (*BackfillResult, error) {
	// --- Step 1: Normalize options and check A5 spend guards ---
	if opts.RepoRoot == "" {
		opts.RepoRoot = p.repoRoot
	}
	if opts.DataDir == "" {
		opts.DataDir = p.dataDir
	}
	if opts.RepoID == "" {
		opts.RepoID = p.repoID
	}
	if opts.QuotaChecker != nil {
		p.QuotaChecker = opts.QuotaChecker
	}

	if !opts.IncludeTranscripts && !opts.IncludeDocs && !opts.IncludeGit {
		opts.IncludeTranscripts = true
		opts.IncludeDocs = true
		opts.IncludeGit = true
	}

	allowed, reason, jobMaxItems := p.CheckSpendGuard("backfill", opts.Force)
	if jobMaxItems <= 0 {
		jobMaxItems = 25
	}
	effectiveMaxItems := jobMaxItems
	if opts.MaxItems > 0 {
		effectiveMaxItems = opts.MaxItems
	}

	res := &BackfillResult{
		RepoID:            opts.RepoID,
		DryRun:            opts.DryRun,
		SpendGuardAllowed: allowed,
		SpendGuardReason:  reason,
		EffectiveMaxItems: effectiveMaxItems,
	}

	if !allowed && !opts.Force && !opts.DryRun {
		return res, nil
	}

	remaining := effectiveMaxItems

	// --- Step 2: Backfill transcripts, capture jobs, and Principal Architect artifacts ---
	if opts.IncludeTranscripts {
		capturesRequeued := p.requeueCapturesForBackfill(ctx, opts)
		res.CapturesRequeued = capturesRequeued

		paState := loadBackfillPAState(opts.DataDir, opts.RepoID)
		stateDirty := false

		if opts.RepoRoot != "" && remaining > 0 {
			paDirs := []string{
				filepath.Join(opts.RepoRoot, ".agents", "plugins", "principal-architect-workspace"),
				filepath.Join(opts.RepoRoot, ".agents", "skills", "principal-architect-workspace"),
			}
			var candidateFiles []string
			for _, paDir := range paDirs {
				if info, err := os.Stat(paDir); err == nil && info.IsDir() {
					_ = filepath.WalkDir(paDir, func(path string, d fs.DirEntry, walkErr error) error {
						if walkErr != nil || d.IsDir() {
							return nil
						}
						base := strings.ToLower(d.Name())
						if base == "project.md" || base == "workorder.md" || base == "context_bridge.md" ||
							base == "memory.md" || base == "module_index.md" || base == "project_index.md" {
							candidateFiles = append(candidateFiles, path)
						}
						return nil
					})
				}
			}
			if len(candidateFiles) > 0 {
				sort.Strings(candidateFiles)

				epStore := p.epStore
				if epStore == nil && opts.DataDir != "" {
					epStore = episode.NewStore(opts.DataDir, opts.RepoID)
				}

				for _, fullPath := range candidateFiles {
					if remaining <= 0 {
						break
					}
					relPath, err := filepath.Rel(opts.RepoRoot, fullPath)
					if err != nil {
						relPath = fullPath
					}
					relSlash := filepath.ToSlash(relPath)

					sessionID := deterministicPASessionID(relSlash)
					if paState.ImportedFiles[relSlash] != "" {
						continue
					}
					if epStore != nil && epStore.Exists(sessionID) {
						continue
					}
					if flatEpisodeExists(opts.DataDir, opts.RepoID, sessionID) {
						continue
					}

					rawBytes, err := os.ReadFile(fullPath)
					if err != nil || len(strings.TrimSpace(string(rawBytes))) == 0 {
						continue
					}

					if !opts.DryRun && epStore != nil {
						now := time.Now().UTC()
						if fi, statErr := os.Stat(fullPath); statErr == nil {
							now = fi.ModTime().UTC()
						}
						header := episode.Header{
							EpisodeID: episode.NewEpisodeIDWithTime(now),
							Harness:   "import",
							SessionID: sessionID,
							RepoID:    opts.RepoID,
							Started:   now,
							Ended:     now,
							Source:    "pa-backfill",
						}
						step := harness.Step{
							Index: 1,
							Kind:  harness.StepKindNote,
							Text:  string(rawBytes),
							File:  relSlash,
						}
						ep := episode.NewEpisode(header, step)
						if err := epStore.Write(ep); err == nil {
							paState.ImportedFiles[relSlash] = sessionID
							stateDirty = true
						}
					}

					res.PAFilesImported++
					res.ItemsProcessed++
					res.ImportedPAFiles = append(res.ImportedPAFiles, relSlash)
					remaining--
				}
			}
		}

		if remaining > 0 && opts.DataDir != "" {
			queued := scanUncoveredFlatEpisodes(opts.DataDir, opts.RepoID, remaining, opts.DryRun, paState)
			if queued > 0 {
				res.EpisodesQueued += queued
				res.ItemsProcessed += queued
				remaining -= queued
				if remaining < 0 {
					remaining = 0
				}
				if !opts.DryRun {
					stateDirty = true
				}
			}
		}

		if !opts.DryRun && stateDirty {
			saveBackfillPAState(opts.DataDir, opts.RepoID, paState)
		}
	}

	// --- Step 3: Seed repository documentation files ---
	if opts.IncludeDocs && remaining > 0 && opts.RepoRoot != "" {
		repoCfg, err := config.LoadRepoConfig(opts.RepoRoot)
		if err != nil {
			repoCfg = config.DefaultRepoConfig()
		}
		scopeCfg, err := config.LoadScopeConfig(filepath.Join(opts.RepoRoot, ".memory", "config.yaml"))
		if err != nil || scopeCfg == nil {
			scopeCfg = &config.ScopeConfig{}
		}
		opsDir := filepath.Join(opts.RepoRoot, ".memory", "ops")
		opsReader := ops.NewReader(opsDir)
		var opsWriter *ops.Writer
		if !opts.DryRun {
			opsWriter = ops.NewWriter(opsDir)
		}

		docRes, err := seed.SeedDocs(ctx, seed.DocSeedOptions{
			RepoRoot: opts.RepoRoot,
			OpsDir:   opsDir,
			RepoCfg:  repoCfg,
			DryRun:   opts.DryRun,
		}, scopeCfg, opsReader, opsWriter)
		if err == nil && docRes != nil {
			res.DocsSeeded = docRes.FilesSeeded
			res.DocEntriesMutated = docRes.EntriesAdded + docRes.EntriesAmend + docRes.EntriesRetire
			if !opts.DryRun && res.DocEntriesMutated > 0 {
				if dossiersMap, projErr := dossiers.NewProjector(opsReader).Project(ctx); projErr == nil {
					_ = dossiers.NewWriter(filepath.Join(opts.RepoRoot, ".memory")).WriteAll(dossiersMap)
				}
			}
			res.ItemsProcessed += docRes.FilesSeeded
			remaining -= docRes.FilesSeeded
			if remaining < 0 {
				remaining = 0
			}
		}
	}

	// --- Step 4: Scan git log for revert commits and emit inbox notes ---
	if opts.IncludeGit && remaining > 0 && opts.RepoRoot != "" {
		cmd := exec.CommandContext(ctx, "git", "-C", opts.RepoRoot, "log", "--grep=Revert", "--grep=revert", "-i", "--format=%H%x09%s", "-n", "50")
		if out, err := cmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				if remaining <= 0 {
					break
				}
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, "\t", 2)
				if len(parts) < 2 {
					continue
				}
				sha := strings.TrimSpace(parts[0])
				subject := strings.TrimSpace(parts[1])
				if !isRevertSubject(subject) {
					continue
				}

				res.GitRevertsScanned++
				short12 := sha
				if len(short12) > 12 {
					short12 = short12[:12]
				}
				noteID := fmt.Sprintf("note-git-revert-%s", short12)
				if gitRevertNoteExists(opts.DataDir, opts.RepoID, noteID) {
					continue
				}

				touchedFiles := gitCommitTouchedFiles(ctx, opts.RepoRoot, sha)
				scope := inferScopeFromTouchedFiles(touchedFiles)
				short7 := sha
				if len(short7) > 7 {
					short7 = short7[:7]
				}
				statement := fmt.Sprintf("Reverted commit %s: %s", short7, subject)
				nowStr := time.Now().UTC().Format(time.RFC3339)

				notePayload := gitRevertNoteDisk{
					ID:         noteID,
					Kind:       "gotcha",
					Scope:      scope,
					Statement:  statement,
					Anchors:    touchedFiles,
					Source:     "backfill-git",
					HeadCommit: sha,
					CreatedAt:  nowStr,
					DedupKey:   noteID,
					Items: []inbox.NoteItem{
						{
							Type:         dossiers.EntryTypeGotcha,
							Title:        statement,
							Why:          statement,
							Do:           statement,
							Anchors:      touchedFiles,
							AnchorChecks: inbox.CheckAnchors(opts.RepoRoot, touchedFiles),
							Scope:        scope,
						},
					},
				}

				if !opts.DryRun {
					if err := writeGitRevertNote(opts.DataDir, opts.RepoID, noteID, notePayload); err != nil {
						continue
					}
				}

				res.GitNotesCreated++
				res.ItemsProcessed++
				res.CreatedNoteIDs = append(res.CreatedNoteIDs, noteID)
				remaining--
			}
		}
	}

	// --- Step 5: Record backfill spend run ---
	if !opts.DryRun && res.ItemsProcessed > 0 {
		p.RecordSpendRun("backfill", res.ItemsProcessed*100, res.ItemsProcessed*100)
	}

	return res, nil
}

// requeueCapturesForBackfill requeues (or counts in dry-run mode) completed/failed capture jobs across candidate job stores.
func (p *PipelineExecutor) requeueCapturesForBackfill(ctx context.Context, opts BackfillOptions) int {
	// --- Step 1: Use the executor's attached store if present ---
	if p.store != nil {
		return requeueOrCountStore(ctx, p.store, opts.DryRun)
	}
	if opts.DataDir == "" {
		return 0
	}

	// --- Step 2: Build candidate SQLite job store paths ---
	var candidates []string
	if opts.RepoID != "" {
		candidates = append(candidates,
			filepath.Join(opts.DataDir, opts.RepoID, "jobs.sqlite"),
			filepath.Join(opts.DataDir, "repos", opts.RepoID, "jobs.sqlite"),
		)
	} else {
		candidates = append(candidates, filepath.Join(opts.DataDir, "jobs.sqlite"))
	}

	// --- Step 3: Open each existing candidate store and requeue capture jobs ---
	total := 0
	seen := make(map[string]bool)
	for _, dbPath := range candidates {
		if seen[dbPath] {
			continue
		}
		seen[dbPath] = true
		if _, err := os.Stat(dbPath); err != nil {
			continue
		}
		store, err := jobs.Open(dbPath)
		if err != nil {
			continue
		}
		total += requeueOrCountStore(ctx, store, opts.DryRun)
		_ = store.Close()
	}
	return total
}

// requeueOrCountStore requeues completed, failed, or dead capture jobs in store, or counts them when dryRun is true.
func requeueOrCountStore(ctx context.Context, store *jobs.Store, dryRun bool) int {
	// --- Step 1: Count eligible capture jobs without mutating state when dryRun is enabled ---
	if dryRun {
		list, err := store.List(ctx, jobs.Filter{Kind: jobs.KindCapture})
		if err != nil {
			return 0
		}
		count := 0
		for _, j := range list {
			if j.State == jobs.StateDone || j.State == jobs.StateDead || j.State == jobs.StateFailed {
				count++
			}
		}
		return count
	}

	// --- Step 2: Requeue all capture jobs in store ---
	n, err := store.RequeueKind(ctx, jobs.KindCapture)
	if err != nil {
		return 0
	}
	return n
}

// nonAlnumRegex matches sequences of non-alphanumeric ASCII characters for slug sanitization.
var nonAlnumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// deterministicPASessionID derives a stable session ID from a repo-relative Principal Architect file path.
func deterministicPASessionID(relSlash string) string {
	h := sha256.Sum256([]byte(relSlash))
	shortHash := hex.EncodeToString(h[:])[:12]
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(relSlash), filepath.Ext(relSlash)))
	stem = strings.Trim(nonAlnumRegex.ReplaceAllString(stem, "-"), "-")
	if stem == "" {
		stem = "artifact"
	}
	return fmt.Sprintf("pa-%s-%s", stem, shortHash)
}

// backfillPAStatePath returns the path to the Principal Architect backfill state file.
func backfillPAStatePath(dataDir, repoID string) string {
	if repoID == "" {
		return filepath.Join(dataDir, "state", "backfill_pa.json")
	}
	return filepath.Join(dataDir, repoID, "state", "backfill_pa.json")
}

// loadBackfillPAState loads the persisted Principal Architect backfill state from disk.
func loadBackfillPAState(dataDir, repoID string) *backfillPAState {
	// --- Step 1: Initialize empty state maps ---
	st := &backfillPAState{
		ImportedFiles:  make(map[string]string),
		QueuedEpisodes: make(map[string]bool),
	}
	if dataDir == "" {
		return st
	}

	// --- Step 2: Read and unmarshal state file if present ---
	data, err := os.ReadFile(backfillPAStatePath(dataDir, repoID))
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, st)
	if st.ImportedFiles == nil {
		st.ImportedFiles = make(map[string]string)
	}
	if st.QueuedEpisodes == nil {
		st.QueuedEpisodes = make(map[string]bool)
	}
	return st
}

// saveBackfillPAState writes the Principal Architect backfill state to disk.
func saveBackfillPAState(dataDir, repoID string, st *backfillPAState) {
	if dataDir == "" || st == nil {
		return
	}
	path := backfillPAStatePath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// flatEpisodeExists reports whether a flat JSON episode file exists for sessionID.
func flatEpisodeExists(dataDir, repoID, sessionID string) bool {
	if dataDir == "" {
		return false
	}
	paths := []string{
		filepath.Join(dataDir, "repos", repoID, "evidence", "episodes", sessionID+".json"),
		filepath.Join(dataDir, repoID, "evidence", "episodes", sessionID+".json"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// scanUncoveredFlatEpisodes scans flat JSON episode files that have not yet been processed and marks up to remaining as queued.
func scanUncoveredFlatEpisodes(dataDir, repoID string, remaining int, dryRun bool, paState *backfillPAState) int {
	// --- Step 1: Collect flat JSON episode files across candidate directories ---
	dirs := []string{
		filepath.Join(dataDir, "repos", repoID, "evidence", "episodes"),
		filepath.Join(dataDir, repoID, "evidence", "episodes"),
	}
	seenIDs := make(map[string]bool)
	var files []string
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			files = append(files, filepath.Join(d, e.Name()))
		}
	}
	sort.Strings(files)

	// --- Step 2: Inspect each episode file and queue uncovered sessions up to remaining ---
	queued := 0
	for _, f := range files {
		if remaining <= 0 {
			break
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var raw struct {
			ID        string   `json:"id"`         // Legacy episode identifier.
			SessionID string   `json:"session_id"` // Session identifier.
			NoteIDs   []string `json:"note_ids"`   // Linked inbox note identifiers.
			Processed bool     `json:"processed"`  // Whether the episode was already processed.
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		epID := raw.SessionID
		if epID == "" {
			epID = raw.ID
		}
		if epID == "" {
			epID = strings.TrimSuffix(filepath.Base(f), ".json")
		}
		if seenIDs[epID] || paState.QueuedEpisodes[epID] {
			continue
		}
		seenIDs[epID] = true
		if len(raw.NoteIDs) > 0 || raw.Processed {
			continue
		}
		if !dryRun {
			paState.QueuedEpisodes[epID] = true
		}
		queued++
		remaining--
	}
	return queued
}

// isRevertSubject reports whether a git commit subject line represents a revert commit.
func isRevertSubject(subject string) bool {
	lower := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(lower, "revert \"") ||
		strings.HasPrefix(lower, "revert:") ||
		strings.HasPrefix(lower, "revert(")
}

// gitRevertNoteExists reports whether a git-revert inbox note with noteID already exists on disk.
func gitRevertNoteExists(dataDir, repoID, noteID string) bool {
	if dataDir == "" {
		return false
	}
	paths := []string{
		filepath.Join(dataDir, repoID, "evidence", "notes", noteID+".json"),
		filepath.Join(dataDir, "repos", repoID, "evidence", "notes", noteID+".json"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// gitCommitTouchedFiles returns the sorted list of repository-relative file paths modified by commit sha.
func gitCommitTouchedFiles(ctx context.Context, repoRoot, sha string) []string {
	// --- Step 1: Query git diff-tree for files modified in sha ---
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	// --- Step 2: Normalize and sort touched file paths ---
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := filepath.ToSlash(strings.TrimSpace(line))
		if f != "" {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files
}

// inferScopeFromTouchedFiles derives a module scope string from the first touched file's parent directory.
func inferScopeFromTouchedFiles(touchedFiles []string) string {
	if len(touchedFiles) == 0 {
		return "repo"
	}
	dir := filepath.ToSlash(filepath.Dir(touchedFiles[0]))
	dir = strings.TrimPrefix(dir, "./")
	if dir == "." || dir == "" || dir == "/" {
		return "repo"
	}
	return dir
}

// writeGitRevertNote persists a gitRevertNoteDisk JSON payload to the repository's evidence notes directory.
func writeGitRevertNote(dataDir, repoID, noteID string, payload gitRevertNoteDisk) error {
	// --- Step 1: Validate dataDir and serialize payload ---
	if dataDir == "" {
		return fmt.Errorf("dataDir is required")
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	// --- Step 2: Write note to primary evidence/notes directory ---
	primaryDir := filepath.Join(dataDir, repoID, "evidence", "notes")
	if err := os.MkdirAll(primaryDir, 0o755); err != nil {
		return err
	}
	primaryPath := filepath.Join(primaryDir, noteID+".json")
	if err := os.WriteFile(primaryPath, data, 0o644); err != nil {
		return err
	}

	// --- Step 3: Mirror into <dataDir>/repos/<repoID>/evidence/notes if <dataDir>/repos exists ---
	reposBase := filepath.Join(dataDir, "repos")
	if info, err := os.Stat(reposBase); err == nil && info.IsDir() && repoID != "" {
		altDir := filepath.Join(reposBase, repoID, "evidence", "notes")
		if err := os.MkdirAll(altDir, 0o755); err == nil {
			_ = os.WriteFile(filepath.Join(altDir, noteID+".json"), data, 0o644)
		}
	}
	return nil
}
