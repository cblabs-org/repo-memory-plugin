/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/pretooluse.go
 * Description: Implements the Antigravity PreToolUse hook handler, enforcing
 *              fail-open allow defaults for non-enrolled workspaces and returning
 *              force_ask decisions when an edit targets a file with unacknowledged
 *              gated invariant/ruled-out entries.
 *
 * Exported Types / Functions:
 * - PreToolUsePayload:  JSON payload delivered to the PreToolUse hook.
 *   - Normalize():      Populates canonical fields from snake_case and toolCall fallbacks.
 * - PreToolUseResponse: JSON decision response returned to Antigravity for PreToolUse.
 * - HandlePreToolUse(): Evaluates file-edit gates and returns a PreToolUseResponse JSON payload.
 */

package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

// allowResponse is the fail-open default JSON response returned by PreToolUse when no gate fires.
var allowResponse = []byte(`{"decision":"allow"}`)

// PreToolUsePayload represents the JSON payload delivered to the Antigravity PreToolUse hook.
type PreToolUsePayload struct {
	ConversationID      string `json:"conversationId"`      // Primary camelCase session UUID supplied by the PreToolUse hook.
	ConversationIDSnake string `json:"conversation_id"`     // Snake_case conversation UUID fallback.
	InvocationNum       int    `json:"invocationNum"`       // 0-based model invocation index within the turn.
	ToolName            string `json:"toolName"`            // Identifier of the tool about to execute (e.g. "replace_file_content", "write_to_file").
	ToolCall            struct {
		Name string          `json:"name"` // Nested tool call name fallback.
		Args json.RawMessage `json:"args"` // Raw JSON tool arguments.
	} `json:"toolCall"` // Nested tool call descriptor.
	TranscriptPath      string   `json:"transcriptPath"`  // CamelCase transcript file path.
	TranscriptPathSnake string   `json:"transcript_path"` // Snake_case transcript file path fallback.
	WorkspacePaths      []string `json:"workspacePaths"`  // CamelCase workspace root paths.
	WorkspacePathsSnake []string `json:"workspace_paths"` // Snake_case workspace root paths fallback.
}

// Normalize populates canonical fields from snake_case and nested toolCall fallbacks.
func (p *PreToolUsePayload) Normalize() {
	if p.ConversationID == "" {
		p.ConversationID = p.ConversationIDSnake
	}
	if p.TranscriptPath == "" {
		p.TranscriptPath = p.TranscriptPathSnake
	}
	if len(p.WorkspacePaths) == 0 {
		p.WorkspacePaths = p.WorkspacePathsSnake
	}
	if p.ToolName == "" && p.ToolCall.Name != "" {
		p.ToolName = p.ToolCall.Name
	}
}

// PreToolUseResponse represents the JSON decision response returned by the PreToolUse hook.
type PreToolUseResponse struct {
	Decision string `json:"decision"`         // Gate decision ("allow", "deny", "ask", or "force_ask").
	Reason   string `json:"reason,omitempty"` // Human-readable gate explanation when Decision is not "allow".
}

// toolArgs captures candidate target-file field names across edit tool argument schemas.
type toolArgs struct {
	TargetFile      string `json:"TargetFile"`   // Destination path from replace_file_content / write_to_file PascalCase schema.
	TargetFileCamel string `json:"targetFile"`   // Destination path from camelCase edit tool argument schema.
	TargetFileSnake string `json:"target_file"`  // Destination path from snake_case edit tool argument schema.
	FilePath        string `json:"filePath"`     // Destination path from generic camelCase file-tool schema.
	FilePathSnake   string `json:"file_path"`    // Destination path from generic snake_case file-tool schema.
	PathPascal      string `json:"Path"`         // Destination path from edit_file / write_file PascalCase schema.
	Path            string `json:"path"`         // Destination path from edit_file / write_file lowercase schema.
	NotebookPascal  string `json:"NotebookPath"` // Destination path from notebook_edit PascalCase schema.
	NotebookCamel   string `json:"notebookPath"` // Destination path from notebook_edit camelCase schema.
}

// isEditTool reports whether name is a file-modifying tool subject to PreToolUse gates.
func isEditTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "replace_file_content", "write_to_file", "edit_file", "write_file", "multi_replace_file_content", "sed_file", "notebook_edit":
		return true
	default:
		return false
	}
}

// extractTargetFile extracts the target file path from raw JSON tool arguments.
func extractTargetFile(argsRaw json.RawMessage) string {
	// --- Step 1: Unmarshal direct JSON object or double-encoded JSON string ---
	if len(argsRaw) == 0 {
		return ""
	}
	var args toolArgs
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		var s string
		if errStr := json.Unmarshal(argsRaw, &s); errStr == nil {
			_ = json.Unmarshal([]byte(s), &args)
		}
	}
	// --- Step 2: Return the first non-empty file path field ---
	for _, p := range []string{
		args.TargetFile,
		args.TargetFileCamel,
		args.TargetFileSnake,
		args.FilePath,
		args.FilePathSnake,
		args.PathPascal,
		args.Path,
		args.NotebookPascal,
		args.NotebookCamel,
	} {
		if strings.TrimSpace(p) != "" {
			return strings.TrimSpace(p)
		}
	}
	return ""
}

// HandlePreToolUse evaluates PreToolUse edit gates for file-modifying tools and returns
// a JSON decision ("allow" or "force_ask").
//
// Note on about_to_edit limitation:
// The about_to_edit moment rarely fires because the model typically decides and edits
// in the same turn, so PreToolUse for the edit tool call is the first signal.
// By then the decision is already made and a "you're about to edit X" injection is too late.
// Note: a future approach might use PostToolUse on file-read as a proxy for "interested in this file."
func HandlePreToolUse(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error) {
	// --- Step 1: Check environment guards, parse payload, and filter non-edit tools ---
	if os.Getenv("REPO_MEMORY_DISABLED") == "1" {
		return allowResponse, nil
	}
	if os.Getenv("REPO_MEMORY_HOUSEKEEPING") == "1" {
		return allowResponse, nil
	}

	var input PreToolUsePayload
	if err := json.Unmarshal(payload, &input); err != nil {
		return allowResponse, nil
	}
	input.Normalize()

	if !isEditTool(input.ToolName) {
		return allowResponse, nil
	}

	// --- Step 2: Resolve enrolled workspace root, repository ID, and target file ---
	var hookOpts HookOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&hookOpts)
		}
	}

	workspaceRoot := hookOpts.WorkspaceOverride
	if workspaceRoot == "" {
		if len(input.WorkspacePaths) > 0 {
			workspaceRoot = input.WorkspacePaths[0]
		} else {
			workspaceRoot, _ = os.Getwd()
		}
	}
	if workspaceRoot == "" {
		return allowResponse, nil
	}

	if !config.IsEnrolledFast(workspaceRoot) {
		return allowResponse, nil
	}

	dataDir := hookOpts.DataDir
	if dataDir == "" {
		dataDir = config.DataDir()
	}

	repoID := ""
	if entry, found, _ := config.FindRegistryEntry(dataDir, workspaceRoot); found && entry != nil {
		repoID = entry.RepoID
	}
	if repoID == "" {
		if root, err := gitops.FindRepoRoot(workspaceRoot); err == nil {
			repoID, _ = gitops.GenerateRepoID(root)
		}
	}
	if repoID == "" {
		return allowResponse, nil
	}

	targetFile := extractTargetFile(input.ToolCall.Args)
	if targetFile == "" {
		return allowResponse, nil
	}

	// --- Step 3: Query unacknowledged gate entries for the target file ---
	idxPath := index.IndexPath(dataDir, repoID)
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		return allowResponse, nil
	}
	defer idx.Close()

	engine := delivery.NewEngine(idx, workspaceRoot)

	// Load the persisted ledger so gate acknowledgments survive across hook invocations.
	ledger, err := LoadLedger(dataDir, repoID, input.ConversationID)
	if err != nil || ledger == nil {
		ledger = delivery.NewLedger(input.ConversationID)
	}

	targetClean := filepath.Clean(targetFile)
	relPath := filepath.ToSlash(targetClean)
	if filepath.IsAbs(targetClean) {
		if rel, err := filepath.Rel(workspaceRoot, targetClean); err == nil && !strings.HasPrefix(rel, "..") {
			relPath = filepath.ToSlash(rel)
		}
	}

	entries, err := engine.GateEntries(ctx, relPath, ledger)
	if (err != nil || len(entries) == 0) && relPath != targetClean {
		entries, err = engine.GateEntries(ctx, targetClean, ledger)
	}
	if err != nil || len(entries) == 0 {
		return allowResponse, nil
	}

	// --- Step 4: Record pending gates in the ledger and return force_ask ---
	var gateIDs []string
	for _, ent := range entries {
		gateIDs = append(gateIDs, ent.Entry.ID)
	}
	_ = ModifyLedger(dataDir, repoID, input.ConversationID, func(l *delivery.Ledger) {
		l.SetPendingGates(relPath, gateIDs)
	})

	reason := delivery.FormatGateDeny(entries[0].Entry)
	resp := PreToolUseResponse{
		Decision: "force_ask",
		Reason:   reason,
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return allowResponse, nil
	}
	return out, nil
}
