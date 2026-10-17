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
	RepoRoot           string
	DataDir            string
	RepoID             string
	DryRun             bool
	Force              bool
	IncludeTranscripts bool
	IncludeDocs        bool
	IncludeGit         bool
	MaxItems           int
	QuotaChecker       func(context.Context) (*QuotaSnapshot, error)
}

// BackfillResult reports the outcome of a historical backfill run.
type BackfillResult struct {
	RepoID             string   `json:"repo_id"`
	DryRun             bool     `json:"dry_run"`
	SpendGuardAllowed  bool     `json:"spend_guard_allowed"`
	SpendGuardReason   string   `json:"spend_guard_reason,omitempty"`
	EffectiveMaxItems  int      `json:"effective_max_items"`
	ItemsProcessed     int      `json:"items_processed"`
	CapturesRequeued   int      `json:"captures_requeued"`
	PAFilesImported    int      `json:"pa_files_imported"`
	EpisodesQueued     int      `json:"episodes_queued"`
	DocsSeeded         int      `json:"docs_seeded"`
	DocEntriesMutated  int      `json:"doc_entries_mutated"`
	GitRevertsScanned  int      `json:"git_reverts_scanned"`
	GitNotesCreated    int      `json:"git_notes_created"`
	CreatedNoteIDs     []string `json:"created_note_ids,omitempty"`
	ImportedPAFiles    []string `json:"imported_pa_files,omitempty"`
}

type backfillPAState struct {
	ImportedFiles  map[string]string `json:"imported_files"`
	QueuedEpisodes map[string]bool   `json:"queued_episodes"`
}

type gitRevertNoteDisk struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"`
	Scope      string           `json:"scope"`
	Statement  string           `json:"statement"`
	Anchors    []string         `json:"anchors,omitempty"`
	Source     inbox.NoteSource `json:"source"`
	HeadCommit string           `json:"head_commit,omitempty"`
	CreatedAt  string           `json:"created_at"`
	DedupKey   string           `json:"dedup_key,omitempty"`
	Items      []inbox.NoteItem `json:"items,omitempty"`
}

// RunBackfill executes historical backfill across transcripts/PA artifacts, docs, and git reverts.
func RunBackfill(ctx context.Context, opts BackfillOptions) (*BackfillResult, error) {
	epStore := episode.NewStore(opts.DataDir, opts.RepoID)
	exec := NewPipelineExecutor(opts.DataDir, opts.RepoRoot, opts.RepoID, nil, epStore)
	if opts.QuotaChecker != nil {
		exec.QuotaChecker = opts.QuotaChecker
	}
	return exec.RunBackfill(ctx, opts)
}

// RunBackfill executes historical backfill on an initialized PipelineExecutor.
func (p *PipelineExecutor) RunBackfill(ctx context.Context, opts BackfillOptions) (*BackfillResult, error) {
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

	// 1. Transcripts & Captures
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

	// 2. Docs
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

	// 3. Git Revert Commits
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

	if !opts.DryRun && res.ItemsProcessed > 0 {
		p.RecordSpendRun("backfill", res.ItemsProcessed*100, res.ItemsProcessed*100)
	}

	return res, nil
}

func (p *PipelineExecutor) requeueCapturesForBackfill(ctx context.Context, opts BackfillOptions) int {
	if p.store != nil {
		return requeueOrCountStore(ctx, p.store, opts.DryRun)
	}
	if opts.DataDir == "" {
		return 0
	}

	var candidates []string
	if opts.RepoID != "" {
		candidates = append(candidates,
			filepath.Join(opts.DataDir, opts.RepoID, "jobs.sqlite"),
			filepath.Join(opts.DataDir, "repos", opts.RepoID, "jobs.sqlite"),
		)
	} else {
		candidates = append(candidates, filepath.Join(opts.DataDir, "jobs.sqlite"))
	}

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

func requeueOrCountStore(ctx context.Context, store *jobs.Store, dryRun bool) int {
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
	n, err := store.RequeueKind(ctx, jobs.KindCapture)
	if err != nil {
		return 0
	}
	return n
}

var nonAlnumRegex = regexp.MustCompile(`[^a-z0-9]+`)

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

func backfillPAStatePath(dataDir, repoID string) string {
	if repoID == "" {
		return filepath.Join(dataDir, "state", "backfill_pa.json")
	}
	return filepath.Join(dataDir, repoID, "state", "backfill_pa.json")
}

func loadBackfillPAState(dataDir, repoID string) *backfillPAState {
	st := &backfillPAState{
		ImportedFiles:  make(map[string]string),
		QueuedEpisodes: make(map[string]bool),
	}
	if dataDir == "" {
		return st
	}
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

func scanUncoveredFlatEpisodes(dataDir, repoID string, remaining int, dryRun bool, paState *backfillPAState) int {
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
			ID        string   `json:"id"`
			SessionID string   `json:"session_id"`
			NoteIDs   []string `json:"note_ids"`
			Processed bool     `json:"processed"`
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

func isRevertSubject(subject string) bool {
	lower := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(lower, "revert \"") ||
		strings.HasPrefix(lower, "revert:") ||
		strings.HasPrefix(lower, "revert(")
}

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

func gitCommitTouchedFiles(ctx context.Context, repoRoot, sha string) []string {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
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

func writeGitRevertNote(dataDir, repoID, noteID string, payload gitRevertNoteDisk) error {
	if dataDir == "" {
		return fmt.Errorf("dataDir is required")
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	primaryDir := filepath.Join(dataDir, repoID, "evidence", "notes")
	if err := os.MkdirAll(primaryDir, 0o755); err != nil {
		return err
	}
	primaryPath := filepath.Join(primaryDir, noteID+".json")
	if err := os.WriteFile(primaryPath, data, 0o644); err != nil {
		return err
	}

	// Also mirror into <dataDir>/repos/<repoID>/evidence/notes if <dataDir>/repos exists.
	reposBase := filepath.Join(dataDir, "repos")
	if info, err := os.Stat(reposBase); err == nil && info.IsDir() && repoID != "" {
		altDir := filepath.Join(reposBase, repoID, "evidence", "notes")
		if err := os.MkdirAll(altDir, 0o755); err == nil {
			_ = os.WriteFile(filepath.Join(altDir, noteID+".json"), data, 0o644)
		}
	}
	return nil
}
