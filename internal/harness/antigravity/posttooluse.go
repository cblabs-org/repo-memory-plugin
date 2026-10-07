package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

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

	if !strings.EqualFold(input.ToolResult.Status, "error") {
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
		cmd := exec.Command("git", "rev-parse", "--show-toplevel")
		cmd.Dir = workspaceRoot
		out, err := cmd.Output()
		if err == nil {
			repoID = filepath.Base(strings.TrimSpace(string(out)))
		}
	}
	if repoID == "" {
		return []byte("{}"), nil
	}

	idxPath := filepath.Join(dataDir, repoID, "index.db")
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

	// Record these injections for future dedup.
	for _, ent := range fresh {
		ledger.RecordInjection(delivery.InjectionRecord{
			EntryID:   ent.Entry.ID,
			Channel:   delivery.ChannelEphemeral,
			Moment:    delivery.MomentError,
			StepIndex: input.StepIdx,
		})
	}
	_ = SaveLedger(dataDir, repoID, ledger)

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
