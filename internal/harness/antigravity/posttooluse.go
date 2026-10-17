/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/posttooluse.go
 * Description: Implements the Antigravity PostToolUse hook handler, recording
 *              subagent lineage, command recipe outcomes, pending gate acknowledgments
 *              after successful edits, and MomentError ephemeral entry injections.
 *
 * Exported Types / Functions:
 * - PostToolUsePayload: JSON payload delivered to the PostToolUse hook.
 *   - Normalize():      Populates canonical fields from snake_case and error fallbacks.
 * - HandlePostToolUse(): Processes a PostToolUse hook event and returns InjectStepsResponse JSON.
 */

package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
)

var (
	// subagentUUIDPattern matches standard 36-character UUID strings in subagent tool outputs.
	subagentUUIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	// exitCodePattern matches non-zero exit code diagnostics in command tool output text.
	exitCodePattern = regexp.MustCompile(`(?i)(?:exited\s+with\s+code|exit\s+code|exit\s+status|exit_code"?\s*[:=]|exitCode"?\s*[:=])\s*([0-9]+)`)
)

// PostToolUsePayload represents the JSON payload delivered to the Antigravity PostToolUse hook.
type PostToolUsePayload struct {
	ConversationID      string `json:"conversationId"`      // Primary camelCase session UUID supplied by the PostToolUse hook.
	ConversationIDSnake string `json:"conversation_id"`     // Snake_case conversation UUID fallback.
	InvocationNum       int    `json:"invocationNum"`       // 0-based model invocation index within the turn.
	ToolName            string `json:"toolName"`            // Identifier of the tool that just finished executing (e.g. "run_command", "edit_file").
	ToolCall            struct {
		Name string          `json:"name"` // Nested tool call name fallback.
		Args json.RawMessage `json:"args"` // Raw JSON tool arguments.
	} `json:"toolCall"` // Nested tool call descriptor.
	ToolResult struct {
		Status string `json:"status"` // Tool execution status (e.g., "success", "error").
		Output string `json:"output"` // Tool execution output text.
	} `json:"toolResult"` // Nested tool execution result.
	Error               string   `json:"error"`           // Top-level error string when tool execution failed.
	StepIdx             int      `json:"stepIdx"`         // Transcript step index of the tool execution.
	TranscriptPath      string   `json:"transcriptPath"`  // CamelCase transcript file path.
	TranscriptPathSnake string   `json:"transcript_path"` // Snake_case transcript file path fallback.
	WorkspacePaths      []string `json:"workspacePaths"`  // CamelCase workspace root paths.
	WorkspacePathsSnake []string `json:"workspace_paths"` // Snake_case workspace root paths fallback.
}

// Normalize populates canonical fields from snake_case and top-level error fallbacks.
func (p *PostToolUsePayload) Normalize() {
	// --- Step 1: Coalesce snake_case and nested toolCall/error fields ---
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
	if p.Error != "" && p.ToolResult.Status == "" {
		p.ToolResult.Status = "error"
		if p.ToolResult.Output == "" {
			p.ToolResult.Output = p.Error
		}
	}
}

// isCommandTool reports whether name identifies a shell command execution tool.
func isCommandTool(name string) bool {
	return strings.EqualFold(name, "run_command") || strings.EqualFold(name, "bash")
}

// extractPostToolCommand extracts the command string from rawArgs across supported casing variants.
func extractPostToolCommand(rawArgs json.RawMessage) string {
	// --- Step 1: Unmarshal candidate command argument fields ---
	if len(rawArgs) == 0 {
		return ""
	}
	var args struct {
		CommandLine  string `json:"CommandLine"`  // PascalCase CommandLine argument.
		CmdLineLower string `json:"commandLine"`  // CamelCase commandLine argument.
		CmdSnake     string `json:"command_line"` // Snake_case command_line argument.
		Command      string `json:"command"`      // Lowercase command argument.
		CmdCap       string `json:"Command"`      // PascalCase Command argument.
		Cmd          string `json:"cmd"`          // Shorthand cmd argument.
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return ""
	}
	// --- Step 2: Return the first non-empty command string ---
	for _, s := range []string{args.CommandLine, args.CmdLineLower, args.CmdSnake, args.Command, args.CmdCap, args.Cmd} {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// hasNonZeroExitCodeInOutput reports whether output contains a non-zero exit code marker.
func hasNonZeroExitCodeInOutput(output string) bool {
	matches := exitCodePattern.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		code := strings.TrimSpace(m[1])
		if code != "" && code != "0" {
			return true
		}
	}
	return false
}

// HandlePostToolUse processes an Antigravity PostToolUse hook event, recording subagent
// lineage, command recipe outcomes, and gate acknowledgments, and returning MomentError
// ephemeral step injections when a tool fails.
func HandlePostToolUse(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error) {
	// --- Step 1: Parse payload and evaluate fast-exit guards ---
	var input PostToolUsePayload
	if err := json.Unmarshal(payload, &input); err != nil {
		return []byte("{}"), nil
	}
	input.Normalize()

	if os.Getenv("REPO_MEMORY_DISABLED") == "1" {
		return []byte("{}"), nil
	}
	if os.Getenv("REPO_MEMORY_HOUSEKEEPING") == "1" {
		return []byte("{}"), nil
	}

	// Fast exit: only edit tools (gate ack), subagent invocations, command tools (recipe tracking),
	// and error results need post-processing.
	isError := strings.EqualFold(input.ToolResult.Status, "error") || strings.EqualFold(input.ToolResult.Status, "failed")
	isEdit := isEditTool(input.ToolName)
	isSubagentInvoke := strings.EqualFold(input.ToolName, "invoke_subagent") || strings.EqualFold(input.ToolCall.Name, "invoke_subagent")
	isCommand := isCommandTool(input.ToolName) || isCommandTool(input.ToolCall.Name)
	if !isError && !isEdit && !isSubagentInvoke && !isCommand {
		return []byte("{}"), nil
	}

	// --- Step 2: Resolve enrolled workspace root and repository ID ---
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
		return []byte("{}"), nil
	}

	if !config.IsEnrolledFast(workspaceRoot) {
		return []byte("{}"), nil
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
		return []byte("{}"), nil
	}

	// --- Step 3: Record subagent lineage and command execution outcomes ---
	if isSubagentInvoke {
		childConvID := extractSubagentChildID(input.ToolResult.Output, input.TranscriptPath, input.StepIdx, input.ConversationID)
		role := extractSubagentRole(input.ToolCall.Args)
		if childConvID != "" && input.ConversationID != "" && !strings.EqualFold(childConvID, input.ConversationID) {
			_ = state.RecordSubagent(ctx, dataDir, repoID, childConvID, input.ConversationID, role)
		}
		if !isError {
			return []byte("{}"), nil
		}
	}

	if isCommand {
		cmdStr := extractPostToolCommand(input.ToolCall.Args)
		if cmdStr != "" {
			nonZero := hasNonZeroExitCodeInOutput(input.ToolResult.Output)
			failed := isError || nonZero
			succeeded := !failed && (strings.EqualFold(input.ToolResult.Status, "success") ||
				strings.EqualFold(input.ToolResult.Status, "ok") ||
				strings.EqualFold(input.ToolResult.Status, "done") ||
				(input.ToolResult.Status == "" && input.Error == ""))
			if succeeded || failed {
				_, _ = skills.RecordCommandOutcome(ctx, dataDir, repoID, workspaceRoot, input.ConversationID, cmdStr, succeeded)
			}
			if nonZero {
				isError = true
			}
		}
	}

	// --- Step 4: Acknowledge pending edit gates on successful file modification ---
	// Require explicit "success" status — empty/cancelled/rejected should not approve.
	if strings.EqualFold(input.ToolResult.Status, "success") && isEdit {
		targetFile := extractTargetFile(input.ToolCall.Args)
		if targetFile != "" {
			targetClean := filepath.Clean(targetFile)
			relPath := filepath.ToSlash(targetClean)
			if filepath.IsAbs(targetClean) {
				if rel, err := filepath.Rel(workspaceRoot, targetClean); err == nil && !strings.HasPrefix(rel, "..") {
					relPath = filepath.ToSlash(rel)
				}
			}
			_ = ModifyLedger(dataDir, repoID, input.ConversationID, func(l *delivery.Ledger) {
				if l.HasPendingGates(relPath) {
					l.AcknowledgePendingGates(relPath)
				}
				if relPath != targetClean && l.HasPendingGates(targetClean) {
					l.AcknowledgePendingGates(targetClean)
				}
			})
		}
		return []byte("{}"), nil
	}

	// --- Step 5: Query MomentError entries on tool failure and deduplicate against the ledger ---
	if !isError {
		return []byte("{}"), nil
	}

	idxPath := index.IndexPath(dataDir, repoID)
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		return []byte("{}"), nil
	}
	defer idx.Close()

	commandPrefix := input.ToolName
	if isCommand {
		if cmdStr := extractPostToolCommand(input.ToolCall.Args); cmdStr != "" {
			fields := strings.Fields(cmdStr)
			if len(fields) > 0 {
				commandPrefix = fields[0]
			}
		}
	}
	if commandPrefix == "" {
		commandPrefix = input.ToolName
	}

	engine := delivery.NewEngine(idx, workspaceRoot)
	entries, err := engine.ErrorEntries(ctx, commandPrefix, input.ToolResult.Output)
	if err != nil || len(entries) == 0 {
		return []byte("{}"), nil
	}

	// Load the persisted ledger for dedup: don't re-inject the same error entries.
	ledger, loadErr := LoadLedger(dataDir, repoID, input.ConversationID)
	if loadErr != nil || ledger == nil {
		ledger = delivery.NewLedger(input.ConversationID)
	}

	// Filter out entries already injected in this conversation.
	var fresh []delivery.RankedEntry
	for _, ent := range entries {
		alreadyInjected := false
		for _, inj := range ledger.Injections {
			if inj.EntryID == ent.Entry.ID && inj.Moment == delivery.MomentError {
				alreadyInjected = true
				break
			}
		}
		if !alreadyInjected {
			fresh = append(fresh, ent)
		}
	}

	if len(fresh) == 0 {
		return []byte("{}"), nil
	}

	formatted := delivery.FormatEntries(fresh)
	if formatted == "" {
		return []byte("{}"), nil
	}

	// --- Step 6: Persist MomentError injections and return ephemeral step response ---
	_ = ModifyLedger(dataDir, repoID, input.ConversationID, func(l *delivery.Ledger) {
		for _, ent := range fresh {
			l.RecordInjection(delivery.InjectionRecord{
				EntryID:   ent.Entry.ID,
				Channel:   delivery.ChannelEphemeral,
				Moment:    delivery.MomentError,
				StepIndex: input.StepIdx,
			})
		}
	})

	resp := InjectStepsResponse{
		InjectSteps: []InjectStep{
			{
				EphemeralMessage: formatted,
			},
		},
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return []byte("{}"), nil
	}
	return out, nil
}

// extractSubagentChildID extracts the spawned child conversation UUID from tool output or the transcript tail.
func extractSubagentChildID(output, transcriptPath string, stepIdx int, parentConvID string) string {
	// --- Step 1: Attempt direct extraction from the tool output string ---
	if id := extractConvIDFromText(output, parentConvID); id != "" {
		return id
	}
	if transcriptPath == "" {
		return ""
	}
	// --- Step 2: Fall back to scanning recent steps in the transcript ---
	baseDir := transcriptPath
	if fi, err := os.Stat(transcriptPath); err == nil && !fi.IsDir() {
		baseDir = filepath.Dir(transcriptPath)
	}
	steps, err := ParseTranscript(baseDir)
	if err != nil || len(steps) == 0 {
		if data, readErr := os.ReadFile(transcriptPath); readErr == nil {
			if id := extractConvIDFromText(string(data), parentConvID); id != "" {
				return id
			}
		}
		return ""
	}
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		if stepIdx > 0 && s.StepIndex < stepIdx {
			continue
		}
		if id := extractConvIDFromText(s.Content, parentConvID); id != "" {
			return id
		}
		if len(s.ToolCalls) > 0 {
			if id := extractConvIDFromText(string(s.ToolCalls), parentConvID); id != "" {
				return id
			}
		}
	}
	return ""
}

// extractConvIDFromText parses JSON fields or UUID substrings in s, returning the first ID distinct from parentConvID.
func extractConvIDFromText(s, parentConvID string) string {
	// --- Step 1: Check structured JSON keys for conversationId / childConversationId ---
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
		for _, k := range []string{"conversationId", "conversation_id", "childConversationId", "child_conversation_id"} {
			if v, ok := obj[k].(string); ok {
				v = strings.TrimSpace(v)
				if v != "" && !strings.EqualFold(v, parentConvID) {
					return v
				}
			}
		}
		if res, ok := obj["result"].(map[string]any); ok {
			for _, k := range []string{"conversationId", "conversation_id", "childConversationId", "child_conversation_id"} {
				if v, ok := res[k].(string); ok {
					v = strings.TrimSpace(v)
					if v != "" && !strings.EqualFold(v, parentConvID) {
						return v
					}
				}
			}
		}
	}
	// --- Step 2: Fall back to regex UUID matching ---
	matches := subagentUUIDPattern.FindAllString(trimmed, -1)
	for _, m := range matches {
		if !strings.EqualFold(m, parentConvID) {
			return m
		}
	}
	return ""
}

// extractSubagentRole extracts the Role or TypeName field from invoke_subagent tool arguments.
func extractSubagentRole(rawArgs json.RawMessage) string {
	if len(rawArgs) == 0 {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal(rawArgs, &obj); err != nil {
		return ""
	}
	for _, k := range []string{"Role", "role", "TypeName", "type_name"} {
		if v, ok := obj[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
