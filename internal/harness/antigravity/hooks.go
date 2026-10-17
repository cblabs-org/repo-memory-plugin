/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/hooks.go
 * Description: Implements the Antigravity Stop hook pipeline, including budget
 *              enforcement, kill-switch and enrollment checks, capture spool
 *              creation, incremental transcript queuing, artifact copying, and
 *              Stage 1 warm-cache Stop distillation.
 *
 * Exported Types / Functions:
 * - DefaultHookBudget:          Maximum allowed runtime for the Stop hook (200ms).
 * - StopPayload:                JSON payload received on an Antigravity Stop hook event.
 *   - Normalize():              Normalizes camelCase and snake_case fields and expands home paths.
 * - StopResult:                 Outcome summary of processing a Stop hook invocation.
 * - HookOptions:                Configuration overrides for hook execution.
 * - HookOption:                 Functional option for configuring HookOptions.
 * - WithHookAppDBPath():        Sets an explicit app conversation_summaries.db path.
 * - WithHookCLIDBPath():        Sets an explicit CLI conversation_summaries.db path.
 * - WithParentConversationID(): Sets an explicit parent conversation ID override.
 * - WithDataDir():              Sets an explicit repo-memory data directory override.
 * - WithWorkspace():            Sets an explicit workspace directory override.
 * - WithTimeout():              Sets a custom execution budget duration.
 * - HandleStop():               Parses raw JSON and executes the Stop hook, returning JSON response bytes.
 * - HandleStopEvent():          Executes the complete Stop hook pipeline on a normalized StopPayload.
 */

package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

// DefaultHookBudget is the maximum allowed runtime for the Stop hook (SPEC §6.2).
const DefaultHookBudget = 200 * time.Millisecond

// StopPayload represents the JSON payload received from Google Antigravity on a Stop event.
type StopPayload struct {
	ArtifactDirectoryPath      string   `json:"artifactDirectoryPath,omitempty"`      // CamelCase path to the session artifact directory.
	ArtifactDirectoryPathSnake string   `json:"artifact_directory_path,omitempty"`    // Snake_case fallback for the artifact directory path.
	ConversationID             string   `json:"conversationId,omitempty"`             // Primary camelCase session UUID supplied by the Antigravity Stop hook.
	ConversationIDSnake        string   `json:"conversation_id,omitempty"`            // Snake_case fallback for the conversation UUID.
	ParentConversationID       string   `json:"parentConversationId,omitempty"`       // CamelCase parent conversation UUID for subagents.
	ParentConversationIDSnake  string   `json:"parent_conversation_id,omitempty"`     // Snake_case fallback for the parent conversation UUID.
	Error                      string   `json:"error,omitempty"`                      // Error string reported on abnormal termination.
	ExecutionNum               int      `json:"executionNum,omitempty"`               // 0-based Stop hook continuation count within the turn.
	FullyIdle                  bool     `json:"fullyIdle,omitempty"`                  // True when the session has no background tasks running.
	ModelName                  string   `json:"modelName,omitempty"`                  // Model identifier active during the turn.
	TerminationReason          string   `json:"terminationReason,omitempty"`          // Reason the turn ended (e.g., "NO_TOOL_CALL").
	TranscriptPath             string   `json:"transcriptPath,omitempty"`             // CamelCase path to the conversation transcript.
	TranscriptPathSnake        string   `json:"transcript_path,omitempty"`            // Snake_case fallback for the transcript path.
	WorkspacePaths             []string `json:"workspacePaths,omitempty"`             // CamelCase list of workspace root paths.
	WorkspacePathsSnake        []string `json:"workspace_paths,omitempty"`            // Snake_case fallback for workspace root paths.
	RepoID                     string   `json:"repo_id,omitempty"`                    // Explicit snake_case repository ID override.
	RepoIDAlt                  string   `json:"repoId,omitempty"`                     // CamelCase fallback for the repository ID override.
}

// Normalize normalizes fields across camelCase and snake_case variations.
func (p *StopPayload) Normalize() {
	// --- Step 1: Coalesce snake_case fallbacks into canonical fields ---
	if p.ArtifactDirectoryPath == "" && p.ArtifactDirectoryPathSnake != "" {
		p.ArtifactDirectoryPath = p.ArtifactDirectoryPathSnake
	}
	if p.ArtifactDirectoryPath != "" {
		p.ArtifactDirectoryPath = expandHome(p.ArtifactDirectoryPath)
	}
	if p.ConversationID == "" && p.ConversationIDSnake != "" {
		p.ConversationID = p.ConversationIDSnake
	}
	if p.ParentConversationID == "" && p.ParentConversationIDSnake != "" {
		p.ParentConversationID = p.ParentConversationIDSnake
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

	// --- Step 2: Expand leading tilde (~) home prefixes on filesystem paths ---
	if p.TranscriptPath != "" {
		p.TranscriptPath = expandHome(p.TranscriptPath)
	}
	for i, w := range p.WorkspacePaths {
		p.WorkspacePaths[i] = expandHome(w)
	}

	// --- Step 3: Derive ConversationID from artifact or transcript path when omitted ---
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
	Skipped              bool              `json:"skipped"`                          // True when hook execution was bypassed by a guard.
	Reason               string            `json:"reason,omitempty"`                 // Explanation for why execution was skipped.
	ConversationID       string            `json:"conversation_id,omitempty"`        // Normalized session UUID after payload normalization and path fallback.
	ParentConversationID string            `json:"parent_conversation_id,omitempty"` // Resolved parent conversation UUID if subagent.
	RepoID               string            `json:"repo_id,omitempty"`                // Canonical 16-hex repository identifier resolved for the workspace.
	RepoRoot             string            `json:"repo_root,omitempty"`              // Canonical root path of the enrolled repository.
	Branch               string            `json:"branch,omitempty"`                 // Active git branch name at capture time.
	Head                 string            `json:"head,omitempty"`                   // Active git HEAD commit hash at capture time.
	SpoolPath            string            `json:"spool_path,omitempty"`             // Filesystem path to the written spool entry.
	SpoolEntry           *spool.Entry      `json:"spool_entry,omitempty"`            // Written capture spool entry metadata.
	CopyResult           *spool.CopyResult `json:"copy_result,omitempty"`            // Incremental transcript copy statistics.
	ArtifactsCopied      int               `json:"artifacts_copied,omitempty"`       // Count of artifact files copied to evidence storage.
	Decision             string            `json:"decision,omitempty"`               // Optional Stop hook decision (e.g., "continue").
	ContinueReason       string            `json:"continue_reason,omitempty"`        // Continuation prompt injected when Decision is "continue".
	NoteID               string            `json:"note_id,omitempty"`                // Inbox note ID when a turn note was harvested.
	Duration             time.Duration     `json:"duration_ms"`                      // Wall-clock execution duration of the Stop hook.
}

// HookOptions configures optional overrides for hook execution.
type HookOptions struct {
	DataDir              string        // Explicit repo-memory data directory override.
	WorkspaceOverride    string        // Explicit workspace root directory override.
	Timeout              time.Duration // Maximum execution budget for the hook call.
	AppDBPath            string        // Custom path to the app conversation_summaries.db.
	CLIDBPath            string        // Custom path to the CLI conversation_summaries.db.
	ParentConversationID string        // Explicit parent conversation UUID override.
}

// HookOption is a functional configuration option.
type HookOption func(*HookOptions)

// WithHookAppDBPath specifies an explicit conversation summaries DB path.
func WithHookAppDBPath(p string) HookOption {
	return func(o *HookOptions) {
		o.AppDBPath = p
	}
}

// WithHookCLIDBPath specifies an explicit CLI conversation summaries DB path.
func WithHookCLIDBPath(p string) HookOption {
	return func(o *HookOptions) {
		o.CLIDBPath = p
	}
}

// WithParentConversationID specifies an explicit parent conversation ID override.
func WithParentConversationID(id string) HookOption {
	return func(o *HookOptions) {
		o.ParentConversationID = id
	}
}

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
	// --- Step 1: Unmarshal and normalize the Stop hook payload ---
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

	// --- Step 2: Execute Stop pipeline and serialize continuation or empty response ---
	res, err := HandleStopEvent(ctx, &payload, opts...)
	if err != nil {
		return nil, err
	}
	if res != nil && res.Decision == "continue" && res.ContinueReason != "" {
		return json.Marshal(map[string]string{
			"decision": "continue",
			"reason":   res.ContinueReason,
		})
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

	// --- Step 1: Apply hook options and enforce the execution budget timeout ---
	opt := HookOptions{
		Timeout: DefaultHookBudget,
	}
	for _, fn := range opts {
		fn(&opt)
	}

	budget := opt.Timeout
	if budget <= 0 {
		budget = DefaultHookBudget
		if runtime.GOOS == "windows" {
			budget = 2 * time.Second
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}

	// --- Step 2: Evaluate environment kill-switch and housekeeping/internal guards ---
	if os.Getenv("REPO_MEMORY_DISABLED") == "1" || strings.EqualFold(os.Getenv("REPO_MEMORY_DISABLED"), "true") {
		return &StopResult{
			Skipped:  true,
			Reason:   "kill-switch active (REPO_MEMORY_DISABLED)",
			Duration: time.Since(start),
		}, nil
	}

	if os.Getenv("REPO_MEMORY_HOUSEKEEPING") == "1" || strings.EqualFold(os.Getenv("REPO_MEMORY_HOUSEKEEPING"), "true") {
		return &StopResult{
			Skipped:  true,
			Reason:   "housekeeping run skipped (REPO_MEMORY_HOUSEKEEPING)",
			Duration: time.Since(start),
		}, nil
	}
	if os.Getenv("REPO_MEMORY_INTERNAL") == "1" || strings.EqualFold(os.Getenv("REPO_MEMORY_INTERNAL"), "true") {
		return &StopResult{
			Skipped:  true,
			Reason:   "internal run skipped (REPO_MEMORY_INTERNAL)",
			Duration: time.Since(start),
		}, nil
	}

	dataDir := opt.DataDir
	if dataDir == "" {
		dataDir = config.DataDir()
	}

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

	// --- Step 3: Resolve candidate workspace and verify repository enrollment ---
	workspace := opt.WorkspaceOverride
	if workspace == "" && len(payload.WorkspacePaths) > 0 && payload.WorkspacePaths[0] != "" {
		workspace = payload.WorkspacePaths[0]
	}
	if workspace == "" && payload.TranscriptPath != "" {
		if root, err := gitops.FindRepoRoot(filepath.Dir(payload.TranscriptPath)); err == nil {
			workspace = root
		}
	}
	var convSummary *ConversationSummary
	if (workspace == "" || payload.ParentConversationID == "") && payload.ConversationID != "" && (opt.AppDBPath != "" || opt.CLIDBPath != "") {
		var attrOpts []AttributionOption
		if opt.AppDBPath != "" {
			attrOpts = append(attrOpts, WithAppDBPath(opt.AppDBPath))
		}
		if opt.CLIDBPath != "" {
			attrOpts = append(attrOpts, WithCLIDBPath(opt.CLIDBPath))
		}
		convSummary, _ = LookupConversationSummary(payload.ConversationID, attrOpts...)
		if workspace == "" && convSummary != nil && len(convSummary.WorkspacePaths) > 0 {
			workspace = convSummary.WorkspacePaths[0]
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

	// --- Step 4: Ensure data layout and capture git branch/HEAD metadata ---
	repoID := entry.RepoID
	if repoID == "" {
		repoID, err = gitops.GenerateRepoID(entry.RootPath)
		if err != nil {
			return nil, fmt.Errorf("generating repo ID: %w", err)
		}
	}
	repoRoot := entry.RootPath

	if err := config.EnsureDataDirLayout(dataDir, repoID); err != nil {
		return nil, fmt.Errorf("ensuring data dir layout: %w", err)
	}

	// Query Git branch and HEAD commit (bounded by context timeout to preserve budget)
	gitTimeout := 50 * time.Millisecond
	if runtime.GOOS == "windows" {
		gitTimeout = 1 * time.Second
	}
	gitCtx, gitCancel := context.WithTimeout(ctx, gitTimeout)
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

	// --- Step 5: Write capture spool entry and incrementally copy transcript to queue/ ---
	spoolDir := filepath.Join(dataDir, repoID, "spool")
	captureEntry := spool.NewCaptureEntry("antigravity", convID, transcriptPath, repoRoot, branch, head)
	parentConvID := opt.ParentConversationID
	if parentConvID == "" {
		parentConvID = payload.ParentConversationID
	}
	if parentConvID == "" && convSummary != nil {
		parentConvID = convSummary.ParentConversationID
	}
	captureEntry.ParentConversationID = parentConvID
	spoolPath, err := spool.Write(spoolDir, captureEntry)
	if err != nil {
		return nil, fmt.Errorf("writing capture spool entry: %w", err)
	}

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

	// --- Step 6: Copy session artifacts best-effort within a 100ms bound ---
	var artifactsCopied int
	if payload.ArtifactDirectoryPath != "" {
		artSrc := expandHome(payload.ArtifactDirectoryPath)
		artDst := evidence.ArtifactDir(dataDir, repoID, convID)

		ch := make(chan int, 1)
		go func() {
			count, err := evidence.CopyArtifacts(artSrc, artDst)
			if err != nil {
				fmt.Fprintf(os.Stderr, "copying artifacts for %s: %v\n", convID, err)
			}
			ch <- count
		}()

		select {
		case count := <-ch:
			artifactsCopied = count
		case <-time.After(100 * time.Millisecond):
			// Timeout, let it run in background (will be killed when process exits)
		}
	}

	result := &StopResult{
		Skipped:              false,
		ConversationID:       convID,
		ParentConversationID: parentConvID,
		RepoID:               repoID,
		RepoRoot:             repoRoot,
		Branch:               branch,
		Head:                 head,
		SpoolPath:            spoolPath,
		SpoolEntry:           &captureEntry,
		CopyResult:           copyResult,
		ArtifactsCopied:      artifactsCopied,
	}

	// --- Step 7: Evaluate Stage 1 warm-cache Stop distillation ---
	evaluateStopDistillation(ctx, payload, result, dataDir, repoID, repoRoot, repoCfg, transcriptPath)

	result.Duration = time.Since(start)
	return result, nil
}

// isHousekeepingPath reports whether path resides inside an isolated runs/<repo-id> directory.
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

// extractConversationID extracts the conversation UUID segment following /brain/ in transcriptPath.
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

// findBrainTranscript searches standard Antigravity app and CLI brain locations for convID's transcript file.
func findBrainTranscript(convID string) string {
	// --- Step 1: Resolve user home directory and candidate transcript paths ---
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

	// --- Step 2: Return the first candidate path that exists on disk ---
	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}

// expandHome expands a leading "~" or "~/" prefix in path to the current user's home directory.
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
