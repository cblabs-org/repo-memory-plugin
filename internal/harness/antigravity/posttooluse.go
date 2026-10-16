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
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
)

var subagentUUIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

type PostToolUsePayload struct {
	ConversationID      string   `json:"conversationId"`
	ConversationIDSnake string   `json:"conversation_id"`
	InvocationNum       int      `json:"invocationNum"`
	ToolName            string   `json:"toolName"`
	ToolCall            struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCall"`
	ToolResult struct {
		Status string `json:"status"`
		Output string `json:"output"`
	} `json:"toolResult"`
	StepIdx             int      `json:"stepIdx"`
	TranscriptPath      string   `json:"transcriptPath"`
	TranscriptPathSnake string   `json:"transcript_path"`
	WorkspacePaths      []string `json:"workspacePaths"`
	WorkspacePathsSnake []string `json:"workspace_paths"`
}

func (p *PostToolUsePayload) Normalize() {
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

func HandlePostToolUse(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error) {
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

	// Fast exit: only edit tools (gate ack), subagent invocations, and error results need post-processing.
	// Everything else (view_file, grep_search, list_dir, etc.) returns immediately
	// to avoid paying the repo lookup + enrollment check cost on every tool call.
	isError := strings.EqualFold(input.ToolResult.Status, "error")
	isEdit := isEditTool(input.ToolName)
	isSubagentInvoke := strings.EqualFold(input.ToolName, "invoke_subagent") || strings.EqualFold(input.ToolCall.Name, "invoke_subagent")
	if !isError && !isEdit && !isSubagentInvoke {
		return []byte("{}"), nil
	}

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

	// On successful edit tool completion, acknowledge any pending gates.
	// Require explicit "success" status — empty/cancelled/rejected should not approve.
	if strings.EqualFold(input.ToolResult.Status, "success") && isEdit {
		targetFile := extractTargetFile(input.ToolCall.Args)
		if targetFile != "" {
			targetClean := filepath.Clean(targetFile)
			relPath := targetClean
			if filepath.IsAbs(targetClean) {
				if rel, err := filepath.Rel(workspaceRoot, targetClean); err == nil && !strings.HasPrefix(rel, "..") {
					relPath = rel
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

	// Error injection path — only runs when the tool result is an error.
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
	if input.ToolName == "run_command" {
		var args struct {
			CommandLine  string `json:"CommandLine"`
			CmdLineLower string `json:"commandLine"`
			CmdSnake     string `json:"command_line"`
			Command      string `json:"command"`
		}
		if len(input.ToolCall.Args) > 0 {
			if err := json.Unmarshal(input.ToolCall.Args, &args); err == nil {
				cmdStr := args.CommandLine
				if cmdStr == "" {
					cmdStr = args.CmdLineLower
				}
				if cmdStr == "" {
					cmdStr = args.CmdSnake
				}
				if cmdStr == "" {
					cmdStr = args.Command
				}
				fields := strings.Fields(cmdStr)
				if len(fields) > 0 {
					commandPrefix = fields[0]
				}
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

	// Record these injections for future dedup under the cross-process lock.
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

func extractSubagentChildID(output, transcriptPath string, stepIdx int, parentConvID string) string {
	if id := extractConvIDFromText(output, parentConvID); id != "" {
		return id
	}
	if transcriptPath == "" {
		return ""
	}
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

func extractConvIDFromText(s, parentConvID string) string {
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
	matches := subagentUUIDPattern.FindAllString(trimmed, -1)
	for _, m := range matches {
		if !strings.EqualFold(m, parentConvID) {
			return m
		}
	}
	return ""
}

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
