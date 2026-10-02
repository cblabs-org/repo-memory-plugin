package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

// DefaultHookBudget is the maximum allowed runtime for the Stop hook (SPEC §6.2).
const DefaultHookBudget = 200 * time.Millisecond

// StopPayload represents the JSON payload received from Google Antigravity on a Stop event.
type StopPayload struct {
	ArtifactDirectoryPath string   `json:"artifactDirectoryPath,omitempty"`
	ConversationID        string   `json:"conversationId,omitempty"`
	ConversationIDSnake   string   `json:"conversation_id,omitempty"`
	Error                 string   `json:"error,omitempty"`
	ExecutionNum          int      `json:"executionNum,omitempty"`
	FullyIdle             bool     `json:"fullyIdle,omitempty"`
	ModelName             string   `json:"modelName,omitempty"`
	TerminationReason     string   `json:"terminationReason,omitempty"`
	TranscriptPath        string   `json:"transcriptPath,omitempty"`
	TranscriptPathSnake   string   `json:"transcript_path,omitempty"`
	WorkspacePaths        []string `json:"workspacePaths,omitempty"`
	WorkspacePathsSnake   []string `json:"workspace_paths,omitempty"`
	RepoID                string   `json:"repo_id,omitempty"`
	RepoIDAlt             string   `json:"repoId,omitempty"`
}

// Normalize normalizes fields across camelCase and snake_case variations.
func (p *StopPayload) Normalize() {
	if p.ConversationID == "" && p.ConversationIDSnake != "" {
		p.ConversationID = p.ConversationIDSnake
	}
	if p.TranscriptPath == "" && p.TranscriptPathSnake != "" {
		p.TranscriptPath = p.TranscriptPathSnake
	}
	if len(p.WorkspacePaths) == 0 && len(p.WorkspacePathsSnake) > 0 {
		p.WorkspacePaths = p.WorkspacePathsSnake
	}
	if p.RepoID == "" && p.RepoIDAlt != "" {
		p.RepoID = p.RepoIDAlt
	}

	if p.TranscriptPath != "" {
		p.TranscriptPath = expandHome(p.TranscriptPath)
	}
	for i, w := range p.WorkspacePaths {
		p.WorkspacePaths[i] = expandHome(w)
	}

	if p.ConversationID == "" {
		if p.ArtifactDirectoryPath != "" {
			p.ConversationID = filepath.Base(filepath.Clean(p.ArtifactDirectoryPath))
		} else if p.TranscriptPath != "" {
			p.ConversationID = extractConversationID(p.TranscriptPath)
		}
	}
}

// StopResult records the outcome of processing a Stop hook invocation.
type StopResult struct {
	Skipped        bool              `json:"skipped"`
	Reason         string            `json:"reason,omitempty"`
	ConversationID string            `json:"conversation_id,omitempty"`
	RepoID         string            `json:"repo_id,omitempty"`
	RepoRoot       string            `json:"repo_root,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	Head           string            `json:"head,omitempty"`
	SpoolPath      string            `json:"spool_path,omitempty"`
	SpoolEntry     *spool.Entry      `json:"spool_entry,omitempty"`
	CopyResult     *spool.CopyResult `json:"copy_result,omitempty"`
	Duration       time.Duration     `json:"duration_ms"`
}

// HookOptions configures optional overrides for hook execution.
type HookOptions struct {
	DataDir           string
	WorkspaceOverride string
	Timeout           time.Duration
}

// HookOption is a functional configuration option.
type HookOption func(*HookOptions)

// WithDataDir specifies an explicit repo-memory data directory override.
func WithDataDir(dataDir string) HookOption {
	return func(o *HookOptions) {
		o.DataDir = dataDir
	}
}

// WithWorkspace specifies an explicit workspace directory override.
func WithWorkspace(ws string) HookOption {
	return func(o *HookOptions) {
		o.WorkspaceOverride = ws
	}
}

// WithTimeout configures a custom execution budget.
func WithTimeout(d time.Duration) HookOption {
	return func(o *HookOptions) {
		o.Timeout = d
	}
}

// HandleStop parses the raw JSON payload and handles the Stop event within the execution budget.
// It returns a standard JSON object string ("{}") on completion or no-op skip.
func HandleStop(ctx context.Context, rawPayload []byte, opts ...HookOption) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var payload StopPayload
	if len(rawPayload) > 0 && strings.TrimSpace(string(rawPayload)) != "" {
		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			return nil, fmt.Errorf("parsing Stop payload JSON: %w", err)
		}
	}
	payload.Normalize()

	_, err := HandleStopEvent(ctx, &payload, opts...)
	if err != nil {
		return nil, err
	}

	return []byte("{}"), nil
}

// HandleStopEvent executes the complete Stop hook pipeline:
// 1. Enforces execution budget (≤ 200ms)
// 2. Evaluates kill-switch checks (REPO_MEMORY_DISABLED, machine config, repo config)
// 3. Evaluates REPO_MEMORY_HOUSEKEEPING guard (skipping housekeeping runs)
// 4. Evaluates enrolment checks (verifying the workspace is enrolled in registry)
// 5. Creates and writes a durable capture spool entry
// 6. Performs incremental transcript copying into queue/ with cursor state tracking
func HandleStopEvent(ctx context.Context, payload *StopPayload, opts ...HookOption) (*StopResult, error) {
	start := time.Now()

	opt := HookOptions{
		Timeout: DefaultHookBudget,
	}
	for _, fn := range opts {
		fn(&opt)
	}

	// 1. Budget enforcement (≤ 200ms)
	budget := opt.Timeout
	if budget <= 0 {
		budget = DefaultHookBudget
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}

	// 2. Kill-switch check (Step A: REPO_MEMORY_DISABLED env var)
	if os.Getenv("REPO_MEMORY_DISABLED") == "1" || strings.EqualFold(os.Getenv("REPO_MEMORY_DISABLED"), "true") {
		return &StopResult{
			Skipped:  true,
			Reason:   "kill-switch active (REPO_MEMORY_DISABLED)",
			Duration: time.Since(start),
		}, nil
	}

	// 3. REPO_MEMORY_HOUSEKEEPING guard (Step B: skip housekeeping runs per SPEC §6.2 / E07)
	if os.Getenv("REPO_MEMORY_HOUSEKEEPING") == "1" || strings.EqualFold(os.Getenv("REPO_MEMORY_HOUSEKEEPING"), "true") {
		return &StopResult{
			Skipped:  true,
			Reason:   "housekeeping run skipped (REPO_MEMORY_HOUSEKEEPING)",
			Duration: time.Since(start),
		}, nil
	}

	dataDir := opt.DataDir
	if dataDir == "" {
		dataDir = config.DataDir()
	}

	// 2. Kill-switch check (Step C: machine configuration enabled)
	machCfg, err := config.LoadMachineConfig(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading machine config: %w", err)
	}
	if !machCfg.Enabled {
		return &StopResult{
			Skipped:  true,
			Reason:   "kill-switch active (machine config disabled)",
			Duration: time.Since(start),
		}, nil
	}

	// Determine candidate workspace
	workspace := opt.WorkspaceOverride
	if workspace == "" && len(payload.WorkspacePaths) > 0 && payload.WorkspacePaths[0] != "" {
		workspace = payload.WorkspacePaths[0]
	}
	if workspace == "" && payload.TranscriptPath != "" {
		if root, err := gitops.FindRepoRoot(filepath.Dir(payload.TranscriptPath)); err == nil {
			workspace = root
		}
	}
	if workspace == "" {
		if wd, err := os.Getwd(); err == nil {
			workspace = wd
		}
	}

	// Additional housekeeping path guard: directory matching runs/<repo-id>
	if isHousekeepingPath(workspace) {
		return &StopResult{
			Skipped:  true,
			Reason:   "housekeeping directory skipped (runs/ path)",
			Duration: time.Since(start),
		}, nil
	}

	// 4. Enrolment check (Step D: verify repository enrolment in registry.json)
	entry, found, err := config.FindRegistryEntry(dataDir, workspace)
	if err != nil {
		return nil, fmt.Errorf("querying registry: %w", err)
	}
	if !found || entry == nil {
		return &StopResult{
			Skipped:  true,
			Reason:   "workspace not enrolled in registry",
			Duration: time.Since(start),
		}, nil
	}

	// 2. Kill-switch check (Step E: repository config enabled)
	repoCfg, err := config.LoadRepoConfig(entry.RootPath)
	if err != nil {
		return nil, fmt.Errorf("loading repo config: %w", err)
	}
	if !repoCfg.IsEnabled() {
		return &StopResult{
			Skipped:  true,
			Reason:   "kill-switch active (repo config disabled)",
			Duration: time.Since(start),
		}, nil
	}

	repoID := entry.RepoID
	if repoID == "" {
		repoID, err = gitops.GenerateRepoID(entry.RootPath)
		if err != nil {
			return nil, fmt.Errorf("generating repo ID: %w", err)
		}
	}
	repoRoot := entry.RootPath

	// Ensure destination directory structure exists under <data-dir>/<repo-id>/
	if err := config.EnsureDataDirLayout(dataDir, repoID); err != nil {
		return nil, fmt.Errorf("ensuring data dir layout: %w", err)
	}

	// Query Git branch and HEAD commit (bounded by 50ms context timeout to preserve budget)
	gitCtx, gitCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	branch, _ := gitops.GetCurrentBranchWithContext(gitCtx, repoRoot)
	head, _ := gitops.GetHeadCommitWithContext(gitCtx, repoRoot)
	gitCancel()

	convID := payload.ConversationID
	if convID == "" {
		return &StopResult{
			Skipped:  true,
			Reason:   "conversation ID empty; no session to capture",
			RepoID:   repoID,
			RepoRoot: repoRoot,
			Duration: time.Since(start),
		}, nil
	}

	// Resolve transcript source path
	transcriptPath := payload.TranscriptPath
	if transcriptPath == "" {
		transcriptPath = findBrainTranscript(convID)
	}

	// 5. Spool entry creation (using packages from WO1)
	spoolDir := filepath.Join(dataDir, repoID, "spool")
	captureEntry := spool.NewCaptureEntry("antigravity", convID, transcriptPath, repoRoot, branch, head)
	spoolPath, err := spool.Write(spoolDir, captureEntry)
	if err != nil {
		return nil, fmt.Errorf("writing capture spool entry: %w", err)
	}

	// 6. Incremental copy (using packages from WO1)
	queueDir := filepath.Join(dataDir, repoID, "queue")
	stateDir := filepath.Join(dataDir, repoID, "state")

	var copyResult *spool.CopyResult
	if transcriptPath != "" {
		if _, err := os.Stat(transcriptPath); err == nil {
			copyResult, err = spool.CopyTranscriptIncremental(convID, transcriptPath, queueDir, stateDir)
			if err != nil {
				return nil, fmt.Errorf("copying transcript incrementally: %w", err)
			}
		}
	}

	duration := time.Since(start)
	return &StopResult{
		Skipped:        false,
		ConversationID: convID,
		RepoID:         repoID,
		RepoRoot:       repoRoot,
		Branch:         branch,
		Head:           head,
		SpoolPath:      spoolPath,
		SpoolEntry:     &captureEntry,
		CopyResult:     copyResult,
		Duration:       duration,
	}, nil
}

func isHousekeepingPath(path string) bool {
	if path == "" {
		return false
	}
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(filepath.Separator))
	for i, part := range parts {
		if part == "runs" && i+1 < len(parts) {
			return true
		}
	}
	return false
}

func extractConversationID(transcriptPath string) string {
	clean := filepath.Clean(transcriptPath)
	idx := strings.Index(clean, "/brain/")
	if idx == -1 {
		idx = strings.Index(clean, `\brain\`)
	}
	if idx != -1 {
		sub := clean[idx+len("/brain/"):]
		parts := strings.Split(sub, string(filepath.Separator))
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}
	return ""
}

func findBrainTranscript(convID string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(home, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs", "transcript_full.jsonl"),
		filepath.Join(home, ".gemini", "antigravity", "brain", convID, "transcript_full.jsonl"),
		filepath.Join(home, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs", "transcript.jsonl"),
		filepath.Join(home, ".gemini", "antigravity-cli", "brain", convID, ".system_generated", "logs", "transcript_full.jsonl"),
	}

	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	} else if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
