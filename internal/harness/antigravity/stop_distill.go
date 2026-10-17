package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/scrub"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
	"gopkg.in/yaml.v3"
)

const (
	// TurnNoteCooldown is the minimum interval between non-commit/non-PR turn notes in a single conversation.
	TurnNoteCooldown = 30 * time.Minute
	// TurnNoteDailyCap is the maximum number of turn-note inbox notes harvested per repository per UTC day.
	TurnNoteDailyCap = 20
	// MaxNoteItems is the maximum number of items accepted from a single repo-memory-note block.
	MaxNoteItems = 6
)

var (
	goalCommandRe      = regexp.MustCompile(`(?im)(?:^|\s)/goal(?:\s|$)`)
	repoMemoryBlockRe  = regexp.MustCompile("(?is)```repo-memory-note[ \\t]*\\r?\\n(.*?)\\r?\\n?```")
	yamlBlockRe        = regexp.MustCompile("(?is)```(?:yaml|yml)[ \\t]*\\r?\\n(.*?)\\r?\\n?```")
	awsAccessKeyRe     = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	githubTokenRe      = regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,255}\b|\bgithub_pat_[A-Za-z0-9_]{22,255}\b`)
	bearerOrTokenRe    = regexp.MustCompile(`(?i)\b(bearer|token)\s+[A-Za-z0-9\-._~+/]{20,}=*`)
	privateKeyBlockRe  = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	privateKeyHeaderRe = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
)

// MilestoneResult describes the outcome of scanning uncovered transcript steps for a milestone.
type MilestoneResult struct {
	Matched      bool
	IsCommitOrPR bool
	Reason       string
}

func evaluateStopDistillation(
	ctx context.Context,
	payload *StopPayload,
	res *StopResult,
	dataDir, repoID, repoRoot string,
	repoCfg config.RepoConfig,
	transcriptPath string,
) {
	if payload == nil || res == nil || payload.ConversationID == "" || transcriptPath == "" {
		return
	}

	if payload.ExecutionNum >= 1 {
		harvestTurnNote(ctx, payload, res, dataDir, repoID, repoRoot, transcriptPath)
		return
	}

	evaluateMilestoneGate(ctx, payload, res, dataDir, repoID, repoRoot, repoCfg, transcriptPath)
}

func evaluateMilestoneGate(
	ctx context.Context,
	payload *StopPayload,
	res *StopResult,
	dataDir, repoID, _ string,
	repoCfg config.RepoConfig,
	transcriptPath string,
) {
	// 1. Check turn_notes config mode ("on" required for primary Stop continuation).
	if repoCfg.TurnNotesMode() != "on" {
		return
	}

	convID := payload.ConversationID

	// 2. Subagent exclusion.
	if strings.TrimSpace(res.ParentConversationID) != "" || strings.TrimSpace(payload.ParentConversationID) != "" {
		return
	}
	if isSub, _, err := state.IsSubagent(ctx, dataDir, repoID, convID); err == nil && isSub {
		return
	}

	// 3. Normal turn completion check: NO_TOOL_CALL + fullyIdle.
	if !strings.EqualFold(strings.TrimSpace(payload.TerminationReason), "NO_TOOL_CALL") || !payload.FullyIdle {
		return
	}

	// 4. Parse transcript.
	steps, err := ParseTranscript(transcriptDirFromPath(transcriptPath))
	if err != nil || len(steps) == 0 {
		return
	}

	// 5. Load mark and clamp on rewind.
	mark, err := state.LoadMark(dataDir, repoID, convID)
	if err != nil || mark == nil {
		return
	}

	latestStepIdx, _, stepAtMarkTimestamp := inspectStepsMetadata(steps, mark.StepIndex)
	effectiveStepCount := len(steps)
	if latestStepIdx+1 > effectiveStepCount {
		effectiveStepCount = latestStepIdx + 1
	}
	if mark.ClampIfRewound(effectiveStepCount, stepAtMarkTimestamp) {
		_ = state.SaveMark(dataDir, repoID, mark)
	}

	// 6. Role opt-out and autonomous /goal exclusion.
	if hasActiveGoalOrRoleOptOut(steps, repoCfg.TurnNotesRoleOptOut) {
		return
	}

	// 7. Milestone detection strictly after the mark.
	isInitialMark := mark.StepIndex == 0 && mark.NoteCount == 0 && mark.LastNoteAt.IsZero()
	milestone := DetectMilestones(steps, mark.StepIndex, isInitialMark)
	if !milestone.Matched {
		return
	}

	// 8. Cooldown (30m, waived for commits and PRs) & Daily Cap (20 turn-note notes/day).
	if !milestone.IsCommitOrPR && !mark.LastNoteAt.IsZero() && time.Since(mark.LastNoteAt) < TurnNoteCooldown {
		return
	}
	if dailyCount, err := state.CountTurnNotesToday(dataDir, repoID, time.Now().UTC()); err == nil && dailyCount >= TurnNoteDailyCap {
		return
	}

	// 9. Issue continuation request.
	mark.PendingRequestStep = latestStepIdx
	mark.PendingContinuation = true
	_ = state.SaveMark(dataDir, repoID, mark)

	hasCheckpoint := hasCheckpointAfterStep(steps, mark.StepIndex, isInitialMark)
	prompt := BuildTurnNoteRequestPrompt(mark.NoteTitles, hasCheckpoint)
	res.Decision = "continue"
	res.ContinueReason = prompt
}

func harvestTurnNote(
	_ context.Context,
	payload *StopPayload,
	res *StopResult,
	dataDir, repoID, repoRoot string,
	transcriptPath string,
) {
	// Loop protection: res.Decision is always empty on executionNum >= 1.
	res.Decision = ""
	res.ContinueReason = ""

	convID := payload.ConversationID
	mark, err := state.LoadMark(dataDir, repoID, convID)
	if err != nil || mark == nil {
		mark = &state.Mark{ConversationID: convID, NoteTitles: []string{}}
	}
	if !mark.PendingContinuation && mark.PendingRequestStep == 0 {
		return
	}
	startStep := mark.StepIndex
	pendingStep := mark.PendingRequestStep

	steps, err := ParseTranscript(transcriptDirFromPath(transcriptPath))
	var byteOffset int64
	if fi, statErr := os.Stat(transcriptPath); statErr == nil {
		byteOffset = fi.Size()
	}

	if err != nil || len(steps) == 0 {
		advanceMarkAndClearPending(mark, mark.StepIndex, byteOffset, mark.StepTimestamp)
		_ = state.SaveMark(dataDir, repoID, mark)
		return
	}

	latestStepIdx, latestTimestamp, _ := inspectStepsMetadata(steps, mark.StepIndex)
	replyStep := findHarvestReplyStep(steps, pendingStep)

	// Always advance mark and clear PendingRequestStep before returning.
	advanceMarkAndClearPending(mark, latestStepIdx, byteOffset, latestTimestamp)

	if replyStep == nil {
		_ = state.SaveMark(dataDir, repoID, mark)
		return
	}

	// Reject replies that made tool calls (loop protection & contract enforcement).
	if hasNonEmptyToolCalls(replyStep.ToolCalls) {
		_ = state.SaveMark(dataDir, repoID, mark)
		return
	}

	items, isNone, parseErr := ParseRepoMemoryNoteBlock(replyStep.Content)
	if parseErr != nil || isNone {
		_ = state.SaveMark(dataDir, repoID, mark)
		return
	}

	validItems := LintNoteItems(items)
	if len(validItems) == 0 {
		_ = state.SaveMark(dataDir, repoID, mark)
		return
	}

	allowedIDs := map[string]bool{}
	if strings.TrimSpace(convID) != "" {
		allowedIDs[strings.TrimSpace(convID)] = true
	}
	if strings.TrimSpace(res.Head) != "" {
		allowedIDs[strings.TrimSpace(res.Head)] = true
	}
	scrubCtx := scrub.Context{
		Boundary:   "note",
		RepoID:     repoID,
		RepoRoot:   repoRoot,
		DataDir:    dataDir,
		AllowedIDs: allowedIDs,
	}

	for i := range validItems {
		var scrubErr error
		if validItems[i].Title, scrubErr = scrubNoteField(validItems[i].Title, scrubCtx); scrubErr != nil {
			_, _ = scrub.QuarantinePayload(dataDir, repoID, "note", scrubErr.Error(), []byte(replyStep.Content))
			_ = state.SaveMark(dataDir, repoID, mark)
			return
		}
		if validItems[i].Why, scrubErr = scrubNoteField(validItems[i].Why, scrubCtx); scrubErr != nil {
			_, _ = scrub.QuarantinePayload(dataDir, repoID, "note", scrubErr.Error(), []byte(replyStep.Content))
			_ = state.SaveMark(dataDir, repoID, mark)
			return
		}
		if validItems[i].Do, scrubErr = scrubNoteField(validItems[i].Do, scrubCtx); scrubErr != nil {
			_, _ = scrub.QuarantinePayload(dataDir, repoID, "note", scrubErr.Error(), []byte(replyStep.Content))
			_ = state.SaveMark(dataDir, repoID, mark)
			return
		}
		if validItems[i].Evidence, scrubErr = scrubNoteField(validItems[i].Evidence, scrubCtx); scrubErr != nil {
			_, _ = scrub.QuarantinePayload(dataDir, repoID, "note", scrubErr.Error(), []byte(replyStep.Content))
			_ = state.SaveMark(dataDir, repoID, mark)
			return
		}
		validItems[i].AnchorChecks = inbox.CheckAnchors(repoRoot, validItems[i].Anchors)
	}

	var exposures []delivery.ExposureRecord
	if ledger, lErr := LoadLedger(dataDir, repoID, convID); lErr == nil && ledger != nil {
		exposures = ledger.GetExposureList()
	}

	store := inbox.NewStore(dataDir, repoID)
	note := &inbox.Note{
		Source:         inbox.SourceTurnNote,
		ConversationID: convID,
		StepsCovered:   [2]int{startStep, latestStepIdx},
		Model:          payload.ModelName,
		Branch:         res.Branch,
		HeadCommit:     res.Head,
		ExposureList:   exposures,
		Items:          validItems,
	}
	noteID, writeErr := store.Write(note)
	if writeErr == nil {
		res.NoteID = noteID
		mark.LastNoteAt = time.Now().UTC()
		mark.NoteCount++
		mark.NotesToday++
		for _, it := range validItems {
			if strings.TrimSpace(it.Title) != "" {
				mark.NoteTitles = append(mark.NoteTitles, strings.TrimSpace(it.Title))
			}
		}
	}

	_ = state.SaveMark(dataDir, repoID, mark)
}

func advanceMarkAndClearPending(mark *state.Mark, latestStepIdx int, byteOffset int64, latestTimestamp string) {
	if mark == nil {
		return
	}
	if latestStepIdx > mark.StepIndex {
		mark.StepIndex = latestStepIdx
	}
	if byteOffset > 0 {
		mark.ByteOffset = byteOffset
	}
	if latestTimestamp != "" {
		mark.StepTimestamp = latestTimestamp
	}
	mark.PendingRequestStep = 0
	mark.PendingContinuation = false
}

func transcriptDirFromPath(transcriptPath string) string {
	if fi, err := os.Stat(transcriptPath); err == nil && fi.IsDir() {
		return transcriptPath
	}
	return filepath.Dir(transcriptPath)
}

func inspectStepsMetadata(steps []Step, markStepIdx int) (latestStepIdx int, latestTimestamp string, stepAtMarkTimestamp string) {
	for _, s := range steps {
		if s.IsCheckpoint || s.IsGap {
			continue
		}
		if s.StepIndex == markStepIdx && s.CreatedAt != "" {
			stepAtMarkTimestamp = s.CreatedAt
		}
		if s.StepIndex >= latestStepIdx {
			latestStepIdx = s.StepIndex
			if s.CreatedAt != "" {
				latestTimestamp = s.CreatedAt
			}
		}
	}
	return latestStepIdx, latestTimestamp, stepAtMarkTimestamp
}

func hasCheckpointAfterStep(steps []Step, afterStepIdx int, isInitialMark bool) bool {
	lastRealStepIdx := -1
	for _, s := range steps {
		if s.IsGap {
			continue
		}
		if s.IsCheckpoint || strings.EqualFold(s.Type, "CHECKPOINT") {
			chkStep := s.StepIndex
			if chkStep == 0 && lastRealStepIdx >= 0 {
				chkStep = lastRealStepIdx + 1
			}
			if isInitialMark || chkStep > afterStepIdx {
				return true
			}
			continue
		}
		lastRealStepIdx = s.StepIndex
	}
	return false
}

func hasActiveGoalOrRoleOptOut(steps []Step, optOutRoles []string) bool {
	var normalizedRoles []string
	for _, r := range optOutRoles {
		trimmed := strings.ToLower(strings.TrimSpace(r))
		if trimmed != "" {
			normalizedRoles = append(normalizedRoles, trimmed)
		}
	}

	for _, s := range steps {
		if s.IsCheckpoint || s.IsGap {
			continue
		}
		isUserOrSystem := strings.EqualFold(s.Type, "USER_INPUT") ||
			strings.EqualFold(s.Type, "SYSTEM") ||
			strings.EqualFold(s.Source, "USER_EXPLICIT") ||
			strings.EqualFold(s.Source, "SYSTEM")
		if !isUserOrSystem {
			continue
		}
		content := strings.TrimSpace(s.Content)
		if content == "" {
			continue
		}
		if goalCommandRe.MatchString(content) {
			return true
		}
		if len(normalizedRoles) > 0 {
			lower := strings.ToLower(content)
			for _, role := range normalizedRoles {
				if strings.Contains(lower, role) {
					return true
				}
			}
		}
	}
	return false
}

type parsedToolCall struct {
	Name    string
	Args    map[string]any
	RawArgs string
}

func extractStepToolCalls(raw json.RawMessage) []parsedToolCall {
	if len(raw) == 0 {
		return nil
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		var single map[string]any
		if errSingle := json.Unmarshal(raw, &single); errSingle == nil && len(single) > 0 {
			list = []map[string]any{single}
		} else {
			return nil
		}
	}

	var out []parsedToolCall
	for _, item := range list {
		var tc parsedToolCall
		if name, ok := item["name"].(string); ok {
			tc.Name = strings.TrimSpace(name)
		} else if name, ok := item["tool"].(string); ok {
			tc.Name = strings.TrimSpace(name)
		}
		if argsObj, ok := item["args"].(map[string]any); ok {
			tc.Args = argsObj
			if b, err := json.Marshal(argsObj); err == nil {
				tc.RawArgs = string(b)
			}
		} else if argsStr, ok := item["args"].(string); ok && argsStr != "" {
			tc.RawArgs = argsStr
			_ = json.Unmarshal([]byte(argsStr), &tc.Args)
		} else if argsObj, ok := item["arguments"].(map[string]any); ok {
			tc.Args = argsObj
			if b, err := json.Marshal(argsObj); err == nil {
				tc.RawArgs = string(b)
			}
		} else if argsStr, ok := item["arguments"].(string); ok && argsStr != "" {
			tc.RawArgs = argsStr
			_ = json.Unmarshal([]byte(argsStr), &tc.Args)
		}

		if fn, ok := item["function"].(map[string]any); ok {
			if tc.Name == "" {
				if fnName, ok := fn["name"].(string); ok {
					tc.Name = strings.TrimSpace(fnName)
				}
			}
			if tc.Args == nil {
				if argsObj, ok := fn["arguments"].(map[string]any); ok {
					tc.Args = argsObj
					if b, err := json.Marshal(argsObj); err == nil {
						tc.RawArgs = string(b)
					}
				} else if argsStr, ok := fn["arguments"].(string); ok && argsStr != "" {
					tc.RawArgs = argsStr
					_ = json.Unmarshal([]byte(argsStr), &tc.Args)
				}
			}
		}

		if tc.Name != "" {
			out = append(out, tc)
		}
	}
	return out
}

func isCommandToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "run_command", "bash", "execute_command", "shell", "terminal":
		return true
	default:
		return false
	}
}

func isEditWriteToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "write_to_file", "replace_file_content", "edit_file", "multi_replace_file_content", "write_file":
		return true
	default:
		return false
	}
}

func extractCommandString(tc parsedToolCall) string {
	if tc.Args == nil {
		return ""
	}
	for _, k := range []string{"CommandLine", "commandLine", "command_line", "command", "cmd", "script"} {
		if v, ok := tc.Args[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func extractTargetPathFromArgs(tc parsedToolCall) string {
	if tc.Args == nil {
		return ""
	}
	for _, k := range []string{"TargetFile", "targetFile", "target_file", "FilePath", "filePath", "file_path", "Path", "path"} {
		if v, ok := tc.Args[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func stepCommandFailed(steps []Step, idx int) bool {
	s := steps[idx]
	if strings.EqualFold(s.Status, "ERROR") || strings.EqualFold(s.Status, "FAILED") {
		return true
	}
	if hasFailureOutputMarker(s.Content) {
		return true
	}
	for j := idx + 1; j < len(steps); j++ {
		next := steps[j]
		if next.IsCheckpoint || next.IsGap {
			continue
		}
		if strings.EqualFold(next.Type, "PLANNER_RESPONSE") || strings.EqualFold(next.Source, "USER_EXPLICIT") {
			break
		}
		if strings.EqualFold(next.Status, "ERROR") || strings.EqualFold(next.Status, "FAILED") {
			return true
		}
		if hasFailureOutputMarker(next.Content) {
			return true
		}
		break
	}
	return false
}

func hasFailureOutputMarker(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	lower := strings.ToLower(text)
	for _, marker := range []string{
		"exit code 1",
		"exit code 2",
		"exited with code 1",
		"exited with code 2",
		"exit status 1",
		"exit status 2",
		"--- fail:",
		"fail\t",
		"build failed",
		"error:",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	if strings.Contains(text, "FAIL") {
		return true
	}
	return false
}

func normalizeCommandPrefix(cmd string) string {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return ""
	}
	// If chained with &&, inspect the last or primary command segment that is not cd/export.
	segments := strings.Split(trimmed, "&&")
	for _, seg := range segments {
		fields := strings.Fields(strings.TrimSpace(seg))
		for len(fields) > 0 && strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "-") {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		first := strings.ToLower(filepath.Base(fields[0]))
		if first == "cd" || first == "export" {
			continue
		}
		if len(fields) >= 2 {
			second := strings.ToLower(fields[1])
			switch first {
			case "go", "pio", "npm", "pnpm", "yarn", "cargo", "git", "gh", "uv", "poetry", "dotnet":
				if first == "npm" && second == "run" && len(fields) >= 3 {
					return "npm run " + strings.ToLower(fields[2])
				}
				return first + " " + second
			}
		}
		return first
	}
	return ""
}

func isBuildOrTestCommand(cmd string) bool {
	lower := strings.ToLower(cmd)
	for _, pat := range []string{
		"go test",
		"go build",
		"go vet",
		"pytest",
		"npm test",
		"npm run build",
		"cargo test",
		"cargo build",
		"pio test",
		"pio run",
		"make",
		"ctest",
	} {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// DetectMilestones scans transcript steps to determine whether any of the 5 v2.2 milestones
// occurred in the uncovered window strictly after afterStepIdx (or >= 0 when isInitialMark is true).
func DetectMilestones(steps []Step, afterStepIdx int, isInitialMark bool) MilestoneResult {
	var res MilestoneResult

	failedCmdStepByPrefix := make(map[string]int)
	editedFilesInWindow := make(map[string]bool)
	secondEditStepIdx := -1
	hasBuildOrTestAfterSecondEdit := false

	for i, s := range steps {
		if s.IsCheckpoint || s.IsGap {
			continue
		}
		inWindow := s.StepIndex > afterStepIdx
		if isInitialMark {
			inWindow = s.StepIndex >= 0
		}

		calls := extractStepToolCalls(s.ToolCalls)
		failed := stepCommandFailed(steps, i)

		for _, tc := range calls {
			if isCommandToolName(tc.Name) {
				cmdStr := extractCommandString(tc)
				if cmdStr == "" {
					continue
				}
				prefix := normalizeCommandPrefix(cmdStr)

				if failed {
					if prefix != "" {
						if _, exists := failedCmdStepByPrefix[prefix]; !exists {
							failedCmdStepByPrefix[prefix] = s.StepIndex
						}
					}
					continue
				}

				// Command succeeded.
				if inWindow {
					if strings.Contains(cmdStr, "git commit") {
						res.Matched = true
						res.IsCommitOrPR = true
						if res.Reason == "" {
							res.Reason = "git commit"
						}
					}
					if strings.Contains(cmdStr, "gh pr create") {
						res.Matched = true
						res.IsCommitOrPR = true
						if res.Reason == "" {
							res.Reason = "gh pr create"
						}
					}
					if prefix != "" {
						if failStep, ok := failedCmdStepByPrefix[prefix]; ok && (s.StepIndex-failStep) >= 15 {
							res.Matched = true
							if res.Reason == "" {
								res.Reason = "error resolved after >= 15 steps"
							}
						}
					}
					if secondEditStepIdx >= 0 && s.StepIndex >= secondEditStepIdx && isBuildOrTestCommand(cmdStr) {
						hasBuildOrTestAfterSecondEdit = true
					}
				}
			} else if isEditWriteToolName(tc.Name) && inWindow && !failed {
				target := extractTargetPathFromArgs(tc)
				if target != "" {
					cleanTarget := filepath.ToSlash(filepath.Clean(target))
					if strings.HasSuffix(cleanTarget, "WORKORDER.md") && strings.Contains(tc.RawArgs, "## Completion Report") {
						res.Matched = true
						if res.Reason == "" {
							res.Reason = "work order completion report"
						}
					}
					if !editedFilesInWindow[cleanTarget] {
						editedFilesInWindow[cleanTarget] = true
						if len(editedFilesInWindow) == 2 && secondEditStepIdx < 0 {
							secondEditStepIdx = s.StepIndex
						}
					}
				}
			}
		}
	}

	if hasBuildOrTestAfterSecondEdit {
		res.Matched = true
		if res.Reason == "" {
			res.Reason = "edits to >= 2 files followed by build or test"
		}
	}

	return res
}

// BuildTurnNoteRequestPrompt returns the canonical Stage 1 Stop continuation prompt.
// The first line is kept to <= 29 characters ("repo-memory: saving turn note") so
// that when the host prepends "Stop hook blocked termination: " (31 chars), the
// 60-character collapsible SystemMessageStep header displays cleanly without truncation.
func BuildTurnNoteRequestPrompt(noteTitles []string, hasCheckpointSinceMark bool) string {
	var b strings.Builder
	b.WriteString("repo-memory: saving turn note\n\n")
	b.WriteString("Before you stop, write a memory note for this repository. Cover only what happened since your last memory note, or since the start of the conversation if there is none. Don't continue the task, don't call tools, and don't repeat your previous answer.\n\n")
	b.WriteString("Record only what a capable engineer new to this code would get wrong without being told:\n")
	b.WriteString("- decisions, with why and the alternatives rejected;\n")
	b.WriteString("- approaches that failed, and why;\n")
	b.WriteString("- error → cause → fix;\n")
	b.WriteString("- rules that must hold;\n")
	b.WriteString("- how external tools or systems behaved;\n")
	b.WriteString("- procedures you verified;\n")
	b.WriteString("- open issues, with what would close them.\n\n")
	b.WriteString("Leave out anything the code or docs already say, progress narration, plans you didn't carry out, and anything that only repeats repo-memory notes shown to you in this conversation.\n\n")
	b.WriteString("Reply with one `repo-memory-note` block in the format below, or the single word NONE.\n\n")
	b.WriteString("```repo-memory-note\n")
	b.WriteString("- type: ruled-out\n")
	b.WriteString("  title: Relative hook command paths in hooks.json\n")
	b.WriteString("  why: Antigravity runs hook commands from the directory containing hooks.json, not the repo root. A wrong path exits non-zero, which PreToolUse treats as deny, so every tool call was blocked.\n")
	b.WriteString("  do: Generate absolute paths at install time; never hand-write relative hook paths.\n")
	b.WriteString("  anchors: [packaging/antigravity/hooks.json, cmd/repo-memory/cmd_install.go]\n")
	b.WriteString("  evidence: probe hook denied all tool calls until the path was made absolute\n")
	b.WriteString("```\n\n")
	b.WriteString("Allowed types: decision, ruled-out, failure-signature, invariant, contract, convention, gotcha, external-behaviour, procedure, recipe, open-issue.\n")
	b.WriteString("Required fields: every type needs title and why (except procedure and recipe); do is required for ruled-out, failure-signature, invariant, procedure, and recipe; open-issue may carry closes_when.")

	if hasCheckpointSinceMark && len(noteTitles) > 0 {
		b.WriteString("\n\nEarlier note titles in this conversation (do not repeat):\n")
		for _, title := range noteTitles {
			trimmed := strings.TrimSpace(title)
			if trimmed != "" {
				b.WriteString("- ")
				b.WriteString(trimmed)
				b.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func findHarvestReplyStep(steps []Step, pendingRequestStep int) *Step {
	if pendingRequestStep > 0 {
		for i := len(steps) - 1; i >= 0; i-- {
			s := steps[i]
			if s.IsCheckpoint || s.IsGap || s.StepIndex <= pendingRequestStep {
				continue
			}
			if strings.EqualFold(s.Type, "PLANNER_RESPONSE") || strings.EqualFold(s.Source, "MODEL") {
				return &steps[i]
			}
		}
	}
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		if s.IsCheckpoint || s.IsGap {
			continue
		}
		if strings.EqualFold(s.Type, "PLANNER_RESPONSE") || strings.EqualFold(s.Source, "MODEL") {
			return &steps[i]
		}
	}
	for i := len(steps) - 1; i >= 0; i-- {
		if !steps[i].IsCheckpoint && !steps[i].IsGap {
			return &steps[i]
		}
	}
	return nil
}

func hasNonEmptyToolCalls(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" || trimmed == "{}" {
		return false
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return len(arr) > 0
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		return len(obj) > 0
	}
	return true
}

// ParseRepoMemoryNoteBlock extracts and parses a `repo-memory-note` YAML block (or "NONE") from model text.
func ParseRepoMemoryNoteBlock(text string) (items []inbox.NoteItem, isNone bool, err error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, false, errors.New("empty reply text")
	}
	if strings.EqualFold(trimmed, "NONE") || strings.EqualFold( strings.Trim(trimmed, "`* \t\r\n."), "NONE") {
		return nil, true, nil
	}

	var blockContent string
	if m := repoMemoryBlockRe.FindStringSubmatch(trimmed); len(m) >= 2 {
		blockContent = strings.TrimSpace(m[1])
	} else if m := yamlBlockRe.FindStringSubmatch(trimmed); len(m) >= 2 {
		blockContent = strings.TrimSpace(m[1])
	} else {
		return nil, false, errors.New("no repo-memory-note fenced block found")
	}

	if strings.EqualFold(blockContent, "NONE") {
		return nil, true, nil
	}
	if blockContent == "" || blockContent == "[]" {
		return nil, true, nil
	}

	var parsed []inbox.NoteItem
	if err := yaml.Unmarshal([]byte(blockContent), &parsed); err != nil {
		var single inbox.NoteItem
		if singleErr := yaml.Unmarshal([]byte(blockContent), &single); singleErr == nil && single.Type != "" && single.Title != "" {
			parsed = []inbox.NoteItem{single}
		} else {
			return nil, false, err
		}
	}

	return parsed, false, nil
}

var allowedTurnNoteTypes = map[dossiers.EntryType]bool{
	dossiers.EntryTypeDecision:          true,
	dossiers.EntryTypeRuledOut:          true,
	dossiers.EntryTypeFailureSignature:  true,
	dossiers.EntryTypeInvariant:         true,
	dossiers.EntryTypeContract:          true,
	dossiers.EntryTypeConvention:        true,
	dossiers.EntryTypeGotcha:            true,
	dossiers.EntryTypeExternalBehaviour: true,
	dossiers.EntryTypeProcedure:         true,
	dossiers.EntryTypeRecipe:            true,
	dossiers.EntryTypeOpenIssue:         true,
}

// LintNoteItems filters and normalizes candidate NoteItems according to the v2.2 turn-note schema rules.
func LintNoteItems(items []inbox.NoteItem) []inbox.NoteItem {
	var valid []inbox.NoteItem
	for _, it := range items {
		normType := dossiers.EntryType(strings.ToLower(strings.TrimSpace(string(it.Type))))
		if !allowedTurnNoteTypes[normType] {
			continue
		}
		it.Type = normType
		it.Title = strings.TrimSpace(it.Title)
		it.Why = strings.TrimSpace(it.Why)
		it.Do = strings.TrimSpace(it.Do)
		it.Evidence = strings.TrimSpace(it.Evidence)

		if it.Title == "" {
			continue
		}

		// Why is required for all types except procedure and recipe.
		if normType != dossiers.EntryTypeProcedure && normType != dossiers.EntryTypeRecipe {
			if it.Why == "" {
				continue
			}
		}

		// Do is required for ruled-out, failure-signature, invariant, procedure, and recipe.
		switch normType {
		case dossiers.EntryTypeRuledOut,
			dossiers.EntryTypeFailureSignature,
			dossiers.EntryTypeInvariant,
			dossiers.EntryTypeProcedure,
			dossiers.EntryTypeRecipe:
			if it.Do == "" {
				continue
			}
		}

		valid = append(valid, it)
		if len(valid) >= MaxNoteItems {
			break
		}
	}
	return valid
}

func scrubNoteField(s string, scrubCtx scrub.Context) (string, error) {
	res, err := scrub.Scrub(s, scrubCtx)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

func scrubNoteText(s string) string {
	if s == "" {
		return ""
	}
	if res, err := scrub.Scrub(s, scrub.Context{Boundary: "note"}); err == nil {
		return res.Text
	}
	out := privateKeyBlockRe.ReplaceAllString(s, "«secret:private_key»")
	out = privateKeyHeaderRe.ReplaceAllString(out, "«secret:private_key»")
	out = awsAccessKeyRe.ReplaceAllString(out, "«secret:aws»")
	out = githubTokenRe.ReplaceAllString(out, "«secret:github»")
	out = bearerOrTokenRe.ReplaceAllString(out, "$1 «secret:token»")
	return out
}
