package antigravity

import (
	"bytes"
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

type PreInvocationPayload struct {
	ConversationID      string   `json:"conversationId"`
	ConversationIDSnake string   `json:"conversation_id"`
	InvocationNum       int      `json:"invocationNum"`
	InitialNumSteps     int      `json:"initialNumSteps"`
	UserPrompt          string   `json:"userPrompt"`
	TranscriptPath      string   `json:"transcriptPath"`
	TranscriptPathSnake string   `json:"transcript_path"`
	WorkspacePaths      []string `json:"workspacePaths"`
	WorkspacePathsSnake []string `json:"workspace_paths"`
}

func (p *PreInvocationPayload) Normalize() {
	if p.ConversationID == "" {
		p.ConversationID = p.ConversationIDSnake
	}
	if p.TranscriptPath == "" {
		p.TranscriptPath = p.TranscriptPathSnake
	}
	if len(p.WorkspacePaths) == 0 {
		p.WorkspacePaths = p.WorkspacePathsSnake
	}
}

func HandlePreInvocation(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error) {
	var input PreInvocationPayload
	if err := json.Unmarshal(payload, &input); err != nil {
		return []byte("{}"), nil
	}
	input.Normalize()

	if input.InvocationNum > 0 {
		return []byte("{}"), nil
	}
	if os.Getenv("REPO_MEMORY_DISABLED") == "1" {
		return []byte("{}"), nil
	}
	if os.Getenv("REPO_MEMORY_HOUSEKEEPING") == "1" {
		return []byte("{}"), nil
	}

	workspaceRoot := ""
	if len(input.WorkspacePaths) > 0 {
		workspaceRoot = input.WorkspacePaths[0]
	} else {
		workspaceRoot, _ = os.Getwd()
	}
	if workspaceRoot == "" {
		return []byte("{}"), nil
	}

	if !config.IsEnrolledFast(workspaceRoot) {
		return []byte("{}"), nil
	}

	repoID := ""
	if entry, found, _ := config.FindRegistryEntry(config.DataDir(), workspaceRoot); found && entry != nil {
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

	idxPath := filepath.Join(config.DataDir(), repoID, "index.db")
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		return []byte("{}"), nil
	}
	defer idx.Close()

	engine := delivery.NewEngine(idx, workspaceRoot)
	ledger := delivery.NewLedger(input.ConversationID)

	var injectSteps []InjectStep

	// Task start moment
	if input.InitialNumSteps <= 1 {
		var promptTerms []string
		for _, term := range strings.Fields(input.UserPrompt) {
			if len(term) >= 3 {
				promptTerms = append(promptTerms, term)
			}
		}

		var diffPaths []string
		cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD~1")
		cmd.Dir = workspaceRoot
		if out, err := cmd.Output(); err == nil {
			lines := strings.Split(string(bytes.TrimSpace(out)), "\n")
			for _, line := range lines {
				if line != "" {
					diffPaths = append(diffPaths, filepath.Join(workspaceRoot, line))
				}
			}
		}

		entries, err := engine.TaskStartEntries(ctx, promptTerms, diffPaths)
		if err == nil && len(entries) > 0 {
			formatted := delivery.FormatEntries(entries)
			injectSteps = append(injectSteps, InjectStep{
				SystemMessage: &SystemMessageEntry{
					SystemMessage: formatted,
					Metadata:      map[string]string{"moment": "task-start"},
				},
			})
			for _, ent := range entries {
				ledger.RecordInjection(delivery.InjectionRecord{
					EntryID: ent.Entry.ID,
					Channel: "systemMessage",
					Moment:  delivery.MomentTaskStart,
				})
			}
		}
	}

	// First-entry and first-touch moments
	var touchedFiles []string
	if input.TranscriptPath != "" {
		steps, err := ParseTranscript(filepath.Dir(input.TranscriptPath))
		if err == nil {
			seenPaths := make(map[string]bool)
			for _, step := range steps {
				if len(step.ToolCalls) == 0 {
					continue
				}
				var calls []map[string]any
				if err := json.Unmarshal(step.ToolCalls, &calls); err != nil {
					continue
				}
				for _, call := range calls {
					fn, ok := call["function"].(map[string]any)
					if !ok {
						continue
					}
					argsStr, ok := fn["arguments"].(string)
					if !ok || argsStr == "" {
						continue
					}
					var args map[string]any
					if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
						continue
					}
					for _, v := range args {
						str, ok := v.(string)
						if !ok {
							continue
						}
						// If path is absolute and within workspace
						if strings.HasPrefix(str, workspaceRoot) {
							if !seenPaths[str] {
								seenPaths[str] = true
								touchedFiles = append(touchedFiles, str)
							}
						}
					}
				}
			}
		}
	}

	for _, path := range touchedFiles {
		// Unique anchor
		if !ledger.HasSeenAnchor(path) {
			entries, err := engine.AnchorEntries(ctx, path)
			if err == nil && len(entries) > 0 {
				formatted := delivery.FormatEntries(entries)
				injectSteps = append(injectSteps, InjectStep{
					SystemMessage: &SystemMessageEntry{
						SystemMessage: formatted,
						Metadata:      map[string]string{"moment": "first-touch", "anchor": path},
					},
				})
				for _, ent := range entries {
					ledger.RecordInjection(delivery.InjectionRecord{
						EntryID: ent.Entry.ID,
						Channel: "systemMessage",
						Moment:  delivery.MomentFirstTouch,
					})
				}
			}
			ledger.MarkAnchorSeen(path)
		}
	}

	// Scopes
	if len(touchedFiles) > 0 {
		moduleScopes := config.DeriveModuleScopes(touchedFiles)
		for _, mScope := range moduleScopes {
			scopeName := mScope.Name
			if !ledger.HasSeenDossier(scopeName) {
				entries, err := engine.DossierEntries(ctx, scopeName)
				if err == nil && len(entries) > 0 {
					formatted := delivery.FormatEntries(entries)
					injectSteps = append(injectSteps, InjectStep{
						SystemMessage: &SystemMessageEntry{
							SystemMessage: formatted,
							Metadata:      map[string]string{"moment": "first-entry", "scope": scopeName},
						},
					})
					for _, ent := range entries {
						ledger.RecordInjection(delivery.InjectionRecord{
							EntryID: ent.Entry.ID,
							Channel: "systemMessage",
							Moment:  delivery.MomentFirstEntry,
						})
					}
				}
				ledger.MarkDossierSeen(scopeName)
			}
		}
	}

	if len(injectSteps) == 0 {
		return []byte("{}"), nil
	}

	resp := InjectStepsResponse{
		InjectSteps: injectSteps,
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return []byte("{}"), nil
	}
	return out, nil
}
