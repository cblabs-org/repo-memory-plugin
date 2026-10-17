/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/stop_distill.go
 * Description: Implements Stage 1 warm-cache Stop hook distillation: evaluates
 *              v2.2 milestone triggers on primary Stop invocations (executionNum == 0),
 *              constructs the continuation prompt, and harvests, lints, scrubs,
 *              and persists `repo-memory-note` YAML blocks on continuation turns
 *              (executionNum >= 1).
 *
 * Exported Types / Functions:
 * - TurnNoteCooldown:           Minimum interval between non-commit/non-PR turn notes (30m).
 * - TurnNoteDailyCap:           Maximum turn-note inbox notes harvested per repo per UTC day (20).
 * - MaxNoteItems:               Maximum items accepted from a single repo-memory-note block (6).
 * - MilestoneResult:            Summary of whether uncovered transcript steps matched a milestone.
 * - DetectMilestones():         Scans uncovered transcript steps for the 5 v2.2 milestones.
 * - BuildTurnNoteRequestPrompt(): Builds the Stage 1 Stop continuation prompt.
 * - ParseRepoMemoryNoteBlock(): Extracts and parses a repo-memory-note YAML block or NONE.
 * - LintNoteItems():            Filters and normalizes candidate NoteItems per v2.2 schema rules.
 */

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
	// goalCommandRe matches the /goal autonomous mode slash command in user or system steps.
	goalCommandRe = regexp.MustCompile(`(?im)(?:^|\s)/goal(?:\s|$)`)
	// repoMemoryBlockRe matches fenced ```repo-memory-note code blocks in model replies.
	repoMemoryBlockRe = regexp.MustCompile("(?is)```repo-memory-note[ \\t]*\\r?\\n(.*?)\\r?\\n?```")
	// yamlBlockRe matches fallback fenced ```yaml or ```yml code blocks in model replies.
	yamlBlockRe = regexp.MustCompile("(?is)```(?:yaml|yml)[ \\t]*\\r?\\n(.*?)\\r?\\n?```")
	// awsAccessKeyRe matches AWS IAM access key IDs starting with AKIA.
	awsAccessKeyRe = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	// githubTokenRe matches GitHub classic and fine-grained personal access tokens.
	githubTokenRe = regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,255}\b|\bgithub_pat_[A-Za-z0-9_]{22,255}\b`)
	// bearerOrTokenRe matches Bearer or Token authorization headers in note text.
	bearerOrTokenRe = regexp.MustCompile(`(?i)\b(bearer|token)\s+[A-Za-z0-9\-._~+/]{20,}=*`)
	// privateKeyBlockRe matches PEM-encoded private key blocks.
	privateKeyBlockRe = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	// privateKeyHeaderRe matches standalone PEM private key headers.
	privateKeyHeaderRe = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
)

// MilestoneResult describes the outcome of scanning uncovered transcript steps for a milestone.
type MilestoneResult struct {
	Matched      bool   // True when at least one v2.2 milestone was detected in the uncovered window.
	IsCommitOrPR bool   // True when the milestone was a git commit or gh pr create (waiving cooldown).
	Reason       string // Short human-readable description of the matched milestone.
}

// evaluateStopDistillation dispatches between primary milestone evaluation (executionNum == 0)
// and turn-note harvesting (executionNum >= 1).
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

// evaluateMilestoneGate evaluates whether the completed turn satisfies the Stage 1
// milestone, cooldown, and daily-cap criteria to request a warm-cache turn note.
func evaluateMilestoneGate(
	ctx context.Context,
	payload *StopPayload,
	res *StopResult,
	dataDir, repoID, _ string,
	repoCfg config.RepoConfig,
	transcriptPath string,
) {
	// --- Step 1: Check turn_notes config mode, subagent exclusions, and normal idle termination ---
	if repoCfg.TurnNotesMode() != "on" {
		return
	}

	convID := payload.ConversationID

	if strings.TrimSpace(res.ParentConversationID) != "" || strings.TrimSpace(payload.ParentConversationID) != "" {
		return
	}
	if isSub, _, err := state.IsSubagent(context.Background(), dataDir, repoID, convID); err != nil || isSub {
		return
	}

	if !strings.EqualFold(strings.TrimSpace(payload.TerminationReason), "NO_TOOL_CALL") || !payload.FullyIdle {
		return
	}

	// --- Step 2: Parse transcript, load distillation mark, and clamp on rewind ---
	steps, err := ParseTranscript(transcriptDirFromPath(transcriptPath))
	if err != nil || len(steps) == 0 {
		return
	}

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

	// --- Step 3: Evaluate role opt-out, /goal exclusion, and milestone triggers ---
	if hasActiveGoalOrRoleOptOut(steps, repoCfg.TurnNotesRoleOptOut) {
		return
	}

	isInitialMark := mark.StepIndex == 0 && mark.NoteCount == 0 && mark.LastNoteAt.IsZero()
	milestone := DetectMilestones(steps, mark.StepIndex, isInitialMark)
	if !milestone.Matched {
		return
	}

	// --- Step 4: Enforce 30m cooldown and daily cap, then issue continuation prompt ---
	if !milestone.IsCommitOrPR && !mark.LastNoteAt.IsZero() && time.Since(mark.LastNoteAt) < TurnNoteCooldown {
		return
	}
	if dailyCount, err := state.CountTurnNotesToday(dataDir, repoID, time.Now().UTC()); err == nil && dailyCount >= TurnNoteDailyCap {
		return
	}

	mark.PendingRequestStep = latestStepIdx
	mark.PendingContinuation = true
	_ = state.SaveMark(dataDir, repoID, mark)

	hasCheckpoint := hasCheckpointAfterStep(steps, mark.StepIndex, isInitialMark)
	prompt := BuildTurnNoteRequestPrompt(mark.NoteTitles, hasCheckpoint)
	res.Decision = "continue"
	res.ContinueReason = prompt
}

// harvestTurnNote extracts, lints, scrubs, and persists a turn note from the model's
// continuation reply on executionNum >= 1, advancing the distillation mark.
func harvestTurnNote(
	_ context.Context,
	payload *StopPayload,
	res *StopResult,
	dataDir, repoID, repoRoot string,
	transcriptPath string,
) {
	// --- Step 1: Enforce loop protection and load pending distillation mark ---
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

	// --- Step 2: Parse transcript, locate reply step, and advance mark ---
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

	// --- Step 3: Parse and lint repo-memory-note YAML items ---
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

	// --- Step 4: Scrub note fields for secrets/PII and check repository path anchors ---
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

	// --- Step 5: Write harvested note to the inbox store and persist updated mark ---
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

// advanceMarkAndClearPending updates the distillation mark high-water step/offset and clears pending continuation state.
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

// transcriptDirFromPath returns transcriptPath if it is a directory, or its parent directory if it is a file.
func transcriptDirFromPath(transcriptPath string) string {
	if fi, err := os.Stat(transcriptPath); err == nil && fi.IsDir() {
		return transcriptPath
	}
	return filepath.Dir(transcriptPath)
}

// inspectStepsMetadata scans non-checkpoint, non-gap steps to find the highest step index,
// its timestamp, and the timestamp at markStepIdx.
func inspectStepsMetadata(steps []Step, markStepIdx int) (latestStepIdx int, latestTimestamp string, stepAtMarkTimestamp string) {
	// --- Step 1: Scan real steps for mark timestamp and maximum step index ---
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

// hasCheckpointAfterStep reports whether a CHECKPOINT step occurred after afterStepIdx.
func hasCheckpointAfterStep(steps []Step, afterStepIdx int, isInitialMark bool) bool {
	// --- Step 1: Track effective step index across steps and detect post-mark checkpoints ---
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

// hasActiveGoalOrRoleOptOut reports whether any user or system step invokes /goal or matches an opted-out role.
func hasActiveGoalOrRoleOptOut(steps []Step, optOutRoles []string) bool {
	// --- Step 1: Normalize configured opt-out role substrings ---
	var normalizedRoles []string
	for _, r := range optOutRoles {
		trimmed := strings.ToLower(strings.TrimSpace(r))
		if trimmed != "" {
			normalizedRoles = append(normalizedRoles, trimmed)
		}
	}

	// --- Step 2: Scan user and system steps for /goal or opted-out role markers ---
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

// parsedToolCall represents a normalized tool call extracted from a transcript step.
type parsedToolCall struct {
	Name    string         // Tool name (e.g., "run_command", "edit_file").
	Args    map[string]any // Parsed JSON arguments map.
	RawArgs string         // Raw serialized JSON arguments string.
}

// extractStepToolCalls parses raw JSON tool_calls from a transcript step into normalized parsedToolCall records.
func extractStepToolCalls(raw json.RawMessage) []parsedToolCall {
	// --- Step 1: Unmarshal raw JSON as a list or single tool-call object ---
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

	// --- Step 2: Normalize tool name and arguments across Antigravity and OpenAI schemas ---
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

// isCommandToolName reports whether name identifies a shell/command execution tool.
func isCommandToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "run_command", "bash", "execute_command", "shell", "terminal":
		return true
	default:
		return false
	}
}

// isEditWriteToolName reports whether name identifies a file edit or write tool.
func isEditWriteToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "write_to_file", "replace_file_content", "edit_file", "multi_replace_file_content", "write_file":
		return true
	default:
		return false
	}
}

// extractCommandString extracts the command line string from a parsedToolCall's arguments.
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

// extractTargetPathFromArgs extracts the target file path from a parsedToolCall's arguments.
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

// stepCommandFailed reports whether the command tool call at steps[idx] failed based on step status or output markers.
func stepCommandFailed(steps []Step, idx int) bool {
	// --- Step 1: Check status and output markers on the tool call step itself ---
	s := steps[idx]
	if strings.EqualFold(s.Status, "ERROR") || strings.EqualFold(s.Status, "FAILED") {
		return true
	}
	if hasFailureOutputMarker(s.Content) {
		return true
	}
	// --- Step 2: Inspect the immediately following tool response step before the next planner/user turn ---
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

// hasFailureOutputMarker reports whether text contains common CLI or test failure indicators.
func hasFailureOutputMarker(text string) bool {
	// --- Step 1: Scan lowercased text for exit code and test/build failure substrings ---
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

// normalizeCommandPrefix extracts the canonical tool/subcommand prefix from a shell command string.
func normalizeCommandPrefix(cmd string) string {
	// --- Step 1: Split chained commands on && and skip leading env assignments or cd/export ---
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return ""
	}
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
		// --- Step 2: Include subcommand token for multi-verb CLI tools ---
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

// isBuildOrTestCommand reports whether cmd invokes a standard build or test runner.
func isBuildOrTestCommand(cmd string) bool {
	// --- Step 1: Match command text against known build and test runner invocations ---
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
	// --- Step 1: Initialize tracking state for failed commands and edited files ---
	var res MilestoneResult

	failedCmdStepByPrefix := make(map[string]int)
	editedFilesInWindow := make(map[string]bool)
	secondEditStepIdx := -1
	hasBuildOrTestAfterSecondEdit := false

	// --- Step 2: Scan transcript steps for command and edit milestones ---
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

	// --- Step 3: Check multi-file edit + build/test milestone ---
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
	// --- Step 1: Assemble the core Stage 1 turn-note instructions and example block ---
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

	// --- Step 2: Append earlier note titles for deduplication if a checkpoint occurred since the mark ---
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

// findHarvestReplyStep locates the latest model response step after pendingRequestStep.
func findHarvestReplyStep(steps []Step, pendingRequestStep int) *Step {
	// --- Step 1: Search backwards for a model response strictly after pendingRequestStep ---
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
	// --- Step 2: Fall back to the most recent model or non-marker step in the transcript ---
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

// hasNonEmptyToolCalls reports whether raw contains at least one tool call entry.
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
	// --- Step 1: Check for empty text or explicit NONE reply ---
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, false, errors.New("empty reply text")
	}
	if strings.EqualFold(trimmed, "NONE") || strings.EqualFold(strings.Trim(trimmed, "`* \t\r\n."), "NONE") {
		return nil, true, nil
	}

	// --- Step 2: Extract fenced repo-memory-note or yaml block content ---
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

	// --- Step 3: Unmarshal YAML list or single NoteItem object ---
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

// allowedTurnNoteTypes defines the set of entry types permitted in a Stage 1 turn note.
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
	// --- Step 1: Validate entry type, required fields, and cap at MaxNoteItems ---
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

// scrubNoteField runs the layered secret/PII scrubber on s under scrubCtx.
func scrubNoteField(s string, scrubCtx scrub.Context) (string, error) {
	res, err := scrub.Scrub(s, scrubCtx)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// scrubNoteText scrubs s using the note boundary scrubber, falling back to regex redaction on error.
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
