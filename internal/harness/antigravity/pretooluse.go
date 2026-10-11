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

var allowResponse = []byte(`{"decision":"allow"}`)

type PreToolUsePayload struct {
	ConversationID      string   `json:"conversationId"`
	ConversationIDSnake string   `json:"conversation_id"`
	InvocationNum       int      `json:"invocationNum"`
	ToolName            string   `json:"toolName"`
	ToolCall            struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCall"`
	TranscriptPath      string   `json:"transcriptPath"`
	TranscriptPathSnake string   `json:"transcript_path"`
	WorkspacePaths      []string `json:"workspacePaths"`
	WorkspacePathsSnake []string `json:"workspace_paths"`
}

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

type PreToolUseResponse struct {
	Decision string `json:"decision"` // "allow", "deny", "ask", "force_ask"
	Reason   string `json:"reason,omitempty"`
}

type toolArgs struct {
	TargetFile      string `json:"TargetFile"`
	TargetFileCamel string `json:"targetFile"`
	TargetFileSnake string `json:"target_file"`
	FilePath        string `json:"filePath"`
	FilePathSnake   string `json:"file_path"`
	Path            string `json:"path"`
}

func isEditTool(name string) bool {
	return strings.EqualFold(name, "replace_file_content") || strings.EqualFold(name, "write_to_file")
}

func extractTargetFile(argsRaw json.RawMessage) string {
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
	for _, p := range []string{args.TargetFile, args.TargetFileCamel, args.TargetFileSnake, args.FilePath, args.FilePathSnake, args.Path} {
		if strings.TrimSpace(p) != "" {
			return strings.TrimSpace(p)
		}
	}
	return ""
}

// Note on about_to_edit limitation:
// The about_to_edit moment rarely fires because the model typically decides and edits
// in the same turn, so PreToolUse for the edit tool call is the first signal.
// By then the decision is already made and a "you're about to edit X" injection is too late.
// Note: a future approach might use PostToolUse on file-read as a proxy for "interested in this file."
func HandlePreToolUse(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error) {
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
	relPath := targetClean
	if filepath.IsAbs(targetClean) {
		if rel, err := filepath.Rel(workspaceRoot, targetClean); err == nil && !strings.HasPrefix(rel, "..") {
			relPath = rel
		}
	}

	entries, err := engine.GateEntries(ctx, relPath, ledger)
	if (err != nil || len(entries) == 0) && relPath != targetClean {
		entries, err = engine.GateEntries(ctx, targetClean, ledger)
	}
	if err != nil || len(entries) == 0 {
		return allowResponse, nil
	}

	// Record the gate entries as pending for this file. PostToolUse will
	// acknowledge them when the edit completes successfully (user approved).
	// Until then, every PreToolUse for this file returns force_ask.
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
