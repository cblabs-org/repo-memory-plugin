/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/preinvocation.go
 * Description: Implements the Antigravity PreInvocation hook handler, performing
 *              prompt-term sanitization, rewind and compaction detection,
 *              task-start/first-touch/first-entry/about-to-edit push selection,
 *              single-block SystemMessage coalescing, and cross-process ledger persistence.
 *
 * Exported Types / Functions:
 * - PreInvocationPayload: JSON payload delivered to the PreInvocation hook.
 *   - Normalize():        Populates canonical fields from snake_case fallbacks.
 * - ExtractPromptTerms(): Sanitizes and tokenizes user prompt text into FTS query terms.
 * - HandlePreInvocation(): Processes a PreInvocation hook event and returns InjectStepsResponse JSON.
 */

package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

// editIntentPattern matches natural-language edit intentions in planner thinking/responses.
var editIntentPattern = regexp.MustCompile(`(?i)\b(?:edit|modify|change|update|write\s+to|replace)\s+(?:the\s+file\s+|file\s+)?[` + "`" + `"'` + `]?([a-zA-Z0-9_\-./\\]+)`)

var (
	// xmlTagPattern matches XML/HTML wrapper tags such as <USER_REQUEST>.
	xmlTagPattern = regexp.MustCompile(`</?[A-Za-z_][A-Za-z0-9_-]*>`)
	// embeddedURLPattern matches embedded http/https URLs in prompts.
	embeddedURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)
	// compoundPrefixModifierPattern matches common hyphenated compound prefixes.
	compoundPrefixModifierPattern = regexp.MustCompile(`(?i)\b(?:on-device|in-memory|built-in|real-time|end-to-end|top-level)\b`)
	// compoundSuffixModifierPattern matches common hyphenated compound suffixes.
	compoundSuffixModifierPattern = regexp.MustCompile(`(?i)-(?:compatible|based|like|ready|style|aware|safe)\b`)
	// urlBoilerplateSegments lists generic URL path/subdomain segments stripped from prompt URLs.
	urlBoilerplateSegments = map[string]bool{
		"api": true, "public": true, "simple": true, "latest": true, "pro": true,
		"v1": true, "v2": true, "v3": true, "v4": true, "v5": true,
		"v6": true, "v7": true, "v8": true, "v9": true,
	}
)

// sanitizePromptURLs strips URL query strings, fragments, schemes, and generic
// API endpoint path/subdomain segments from embedded http/https URLs while
// preserving domain/resource nouns (e.g., "coingecko", "price").
func sanitizePromptURLs(s string) string {
	// --- Step 1: Replace each embedded URL with its non-boilerplate path/domain tokens ---
	return embeddedURLPattern.ReplaceAllStringFunc(s, func(rawURL string) string {
		u := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
		if idx := strings.IndexAny(u, "?#"); idx >= 0 {
			u = u[:idx]
		}
		parts := strings.FieldsFunc(u, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
		})
		var kept []string
		for _, p := range parts {
			lower := strings.ToLower(p)
			if urlBoilerplateSegments[lower] || delivery.IsStopWord(lower) {
				continue
			}
			kept = append(kept, p)
		}
		return strings.Join(kept, " ")
	})
}

// ExtractPromptTerms strips XML/HTML wrapper tags, sanitizes embedded URLs and
// generic hyphenated compound modifiers, splits on non-word characters, filters
// shared stopwords, and returns deduplicated terms of length >= 3.
func ExtractPromptTerms(prompt string) []string {
	// --- Step 1: Strip XML tags, URLs, and generic compound modifiers ---
	cleaned := xmlTagPattern.ReplaceAllString(prompt, " ")
	cleaned = sanitizePromptURLs(cleaned)
	cleaned = compoundPrefixModifierPattern.ReplaceAllString(cleaned, " ")
	cleaned = compoundSuffixModifierPattern.ReplaceAllString(cleaned, " ")

	// --- Step 2: Tokenize on non-word boundaries and deduplicate non-stopwords ---
	seen := make(map[string]bool)
	var terms []string
	parts := strings.FieldsFunc(cleaned, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	for _, part := range parts {
		lower := strings.ToLower(part)
		if len(lower) < 3 || delivery.IsStopWord(lower) || seen[lower] {
			continue
		}
		seen[lower] = true
		terms = append(terms, part)
	}
	return terms
}

// PreInvocationPayload represents the JSON payload delivered to the PreInvocation hook.
type PreInvocationPayload struct {
	// ConversationID is the camelCase conversation identifier.
	ConversationID string `json:"conversationId"`
	// ConversationIDSnake is the snake_case conversation identifier fallback.
	ConversationIDSnake string `json:"conversation_id"`
	// InvocationNum is the 0-based model invocation index within the current user turn.
	InvocationNum int `json:"invocationNum"`
	// InitialNumSteps is the number of transcript steps at the start of the turn.
	InitialNumSteps int `json:"initialNumSteps"`
	// UserPrompt is the user prompt text for the turn.
	UserPrompt string `json:"userPrompt"`
	// TranscriptPath is the camelCase path to the conversation transcript file.
	TranscriptPath string `json:"transcriptPath"`
	// TranscriptPathSnake is the snake_case transcript path fallback.
	TranscriptPathSnake string `json:"transcript_path"`
	// WorkspacePaths is the camelCase list of active workspace root paths.
	WorkspacePaths []string `json:"workspacePaths"`
	// WorkspacePathsSnake is the snake_case workspace paths fallback.
	WorkspacePathsSnake []string `json:"workspace_paths"`
}

// Normalize populates canonical fields from their snake_case fallbacks when empty.
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

// HandlePreInvocation processes an Antigravity PreInvocation hook event and returns
// the serialized InjectStepsResponse JSON.
func HandlePreInvocation(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error) {
	// --- Step 1: Parse payload and evaluate fast-exit guards ---
	var input PreInvocationPayload
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

	var hookOpts HookOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&hookOpts)
		}
	}

	// --- Step 2: Resolve enrolled workspace root, repoID, index, and conversation ledger ---
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

	idxPath := index.IndexPath(dataDir, repoID)
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		return []byte("{}"), nil
	}
	defer idx.Close()

	ledger, err := LoadLedger(dataDir, repoID, input.ConversationID)
	if err != nil || ledger == nil {
		ledger = delivery.NewLedger(input.ConversationID)
	}

	// Snapshot the injection count at load time. Only records added *after*
	// this point belong to this invocation and must be replayed under the lock.
	injectionBase := len(ledger.Injections)
	rearmed := false // set to true if compaction or rewind triggers RearmDedup
	rewound := false
	maxRemainingStepIdx := 0

	var systemMessageBlocks []string
	var ephemeralSteps []InjectStep

	// --- Step 3: Parse transcript delta, detect rewind, and extract touched workspace paths ---
	var touchedFiles []string
	var steps []Step
	if input.TranscriptPath != "" {
		steps, _ = ParseTranscript(filepath.Dir(input.TranscriptPath))
		if DetectRewind(steps, ledger.LastKnownStepCount) {
			ledger.RearmDedup()
			ledger.LastProcessedStepIndex = 0
			hasCheckpoint := false
			for _, st := range steps {
				if st.IsCheckpoint {
					hasCheckpoint = true
				}
				if st.StepIndex > maxRemainingStepIdx {
					maxRemainingStepIdx = st.StepIndex
				}
			}
			if !hasCheckpoint || ledger.LastHandledCheckpointStep > maxRemainingStepIdx {
				ledger.LastHandledCheckpointStep = -1
			}
			var surviving []delivery.InjectionRecord
			for _, inj := range ledger.Injections {
				if inj.StepIndex <= maxRemainingStepIdx {
					surviving = append(surviving, inj)
				}
			}
			ledger.Injections = surviving
			injectionBase = len(ledger.Injections)
			rearmed = true
			rewound = true
		}
		ledger.LastKnownStepCount = len(steps)
		if len(steps) > 0 {
			seenPaths := make(map[string]bool)
			// Only scan steps newer than our high-water mark to keep cost
			// proportional to new steps in long conversations.
			hwm := ledger.LastProcessedStepIndex
			for _, step := range steps {
				if step.StepIndex <= hwm && hwm > 0 {
					continue
				}
				if len(step.ToolCalls) == 0 {
					continue
				}
				var calls []map[string]any
				if err := json.Unmarshal(step.ToolCalls, &calls); err != nil {
					continue
				}
				for _, call := range calls {
					// Extract args from tool call — support both formats.
					var args map[string]any

					// Format 1 (Antigravity native): {"name": "...", "args": {...}}
					if argsObj, ok := call["args"].(map[string]any); ok {
						args = argsObj
					}
					// Format 2 (OpenAI): {"function": {"name": "...", "arguments": "{...}"}}
					if args == nil {
						if fn, ok := call["function"].(map[string]any); ok {
							if argsStr, ok := fn["arguments"].(string); ok && argsStr != "" {
								json.Unmarshal([]byte(argsStr), &args)
							}
						}
					}
					if args == nil {
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
			// Update high-water mark
			if last := steps[len(steps)-1]; last.StepIndex > hwm {
				ledger.LastProcessedStepIndex = last.StepIndex
			}
		}
	}

	// --- Step 4: Detect transcript compaction and build lost-entry re-injection ---
	if len(ledger.Injections) > 0 && len(steps) > 0 {
		res := DetectCompaction(steps, ledger)
		if res.Detected && res.CheckpointStepIdx > ledger.LastHandledCheckpointStep {
			var checkpointText string
			checkpointFound := false
			for _, step := range steps {
				if step.IsCheckpoint {
					checkpointFound = true
				} else if checkpointFound {
					if step.Type == "PLANNER_RESPONSE" || strings.Contains(step.Content, "[repo-memory") {
						checkpointText += step.Content
					}
				}
			}

			reinjStep, err := BuildReinjection(ctx, idx, res.LostInjections, checkpointText, 1000)
			if err == nil && reinjStep != nil && reinjStep.SystemMessage != nil && reinjStep.SystemMessage.SystemMessage != "" {
				systemMessageBlocks = append(systemMessageBlocks, reinjStep.SystemMessage.SystemMessage)
				ledger.RearmDedup()
				rearmed = true
			}
			// Mark this checkpoint as handled so we don't re-inject on future turns.
			ledger.LastHandledCheckpointStep = res.CheckpointStepIdx
		}
	}

	engine := delivery.NewEngine(idx, workspaceRoot, delivery.WithInjectedSet(ledger.InjectedSet))

	// --- Step 5: Select MomentTaskStart entries on the first invocation of a new conversation ---
	if input.InvocationNum == 0 && input.InitialNumSteps <= 1 {
		promptTerms := ExtractPromptTerms(input.UserPrompt)

		// Diff paths: git already returns repo-relative paths — keep them as-is.
		// Guard against single-commit repos where HEAD~1 would fail.
		var diffPaths []string
		cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD~1")
		cmd.Dir = workspaceRoot
		if out, err := cmd.Output(); err == nil {
			lines := strings.Split(string(bytes.TrimSpace(out)), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					diffPaths = append(diffPaths, line)
				}
			}
		}
		// err is intentionally ignored — single-commit repos just get no diff paths.

		entries, err := engine.TaskStartEntries(ctx, promptTerms, diffPaths)
		if err != nil {
			ephemeralSteps = append(ephemeralSteps, InjectStep{
				EphemeralMessage: fmt.Sprintf("[repo-memory] search error: %v", err),
			})
		} else if len(entries) > 0 {
			formatted := delivery.FormatEntries(entries)
			systemMessageBlocks = append(systemMessageBlocks, formatted)
			for _, ent := range entries {
				ledger.RecordInjection(delivery.InjectionRecord{
					EntryID:   ent.Entry.ID,
					Channel:   delivery.ChannelSystem,
					Moment:    delivery.MomentTaskStart,
					StepIndex: input.InitialNumSteps,
				})
			}
		}
	}

	// --- Step 6: Select MomentFirstTouch and MomentFirstEntry entries for newly touched files/scopes ---
	for _, path := range touchedFiles {
		anchor := path
		if rel, err := filepath.Rel(workspaceRoot, path); err == nil && !strings.HasPrefix(rel, "..") {
			anchor = filepath.ToSlash(rel)
		}

		// Unique anchor
		if !ledger.HasSeenAnchor(anchor) && !ledger.HasSeenAnchor(path) {
			entries, err := engine.AnchorEntries(ctx, anchor)
			if (err != nil || len(entries) == 0) && anchor != path {
				entries, err = engine.AnchorEntries(ctx, path)
			}
			if err == nil && len(entries) > 0 {
				formatted := delivery.FormatEntriesForMoment(entries, delivery.MomentFirstTouch, workspaceRoot)
				systemMessageBlocks = append(systemMessageBlocks, formatted)
				for _, ent := range entries {
					ledger.RecordInjection(delivery.InjectionRecord{
						EntryID:   ent.Entry.ID,
						Channel:   delivery.ChannelSystem,
						Moment:    delivery.MomentFirstTouch,
						StepIndex: input.InitialNumSteps,
					})
				}
			}
			ledger.MarkAnchorSeen(anchor)
			ledger.MarkAnchorSeen(path)
		}
	}

	// Scopes
	if len(touchedFiles) > 0 {
		var relTouchedFiles []string
		for _, path := range touchedFiles {
			rel := path
			if r, err := filepath.Rel(workspaceRoot, path); err == nil && !strings.HasPrefix(r, "..") {
				rel = filepath.ToSlash(r)
			}
			relTouchedFiles = append(relTouchedFiles, rel)
		}
		var candidateScopes []string
		seenScope := make(map[string]bool)
		addScope := func(s string) {
			if s != "" && !seenScope[s] {
				seenScope[s] = true
				candidateScopes = append(candidateScopes, s)
			}
		}
		for _, rel := range relTouchedFiles {
			for _, mScope := range config.DeriveModuleScopes([]string{rel, rel + ".1", rel + ".2"}) {
				addScope(mScope.Name)
			}
		}
		for _, mScope := range config.DeriveModuleScopes(relTouchedFiles) {
			addScope(mScope.Name)
		}
		for _, scopeName := range candidateScopes {
			if !ledger.HasSeenDossier(scopeName) {
				entries, err := engine.DossierEntries(ctx, scopeName)
				if err == nil && len(entries) > 0 {
					formatted := delivery.FormatEntriesForMoment(entries, delivery.MomentFirstEntry, workspaceRoot)
					systemMessageBlocks = append(systemMessageBlocks, formatted)
					for _, ent := range entries {
						ledger.RecordInjection(delivery.InjectionRecord{
							EntryID:   ent.Entry.ID,
							Channel:   delivery.ChannelSystem,
							Moment:    delivery.MomentFirstEntry,
							StepIndex: input.InitialNumSteps,
						})
					}
				}
				ledger.MarkDossierSeen(scopeName)
			}
		}
	}

	// --- Step 7: Evaluate MomentAboutToEdit ephemeral reminder ---
	if len(steps) > 0 {
		var latestPlanner *Step
		for i := len(steps) - 1; i >= 0; i-- {
			if strings.EqualFold(steps[i].Type, "PLANNER_RESPONSE") {
				latestPlanner = &steps[i]
				break
			}
		}

		if latestPlanner != nil {
			text := latestPlanner.Thinking + "\n" + latestPlanner.Content
			matches := editIntentPattern.FindAllStringSubmatch(text, -1)
			seenTarget := make(map[string]bool)
			for _, m := range matches {
				target := strings.Trim(m[1], "`'\" \t\r\n.,;:()")
				if target == "" || seenTarget[target] {
					continue
				}
				seenTarget[target] = true

				// Dedup: skip if we already injected about-to-edit for this target.
				dedupKey := "about-to-edit:" + target
				alreadyInjected := false
				for _, inj := range ledger.Injections {
					if inj.EntryID == dedupKey {
						alreadyInjected = true
						break
					}
				}
				if alreadyInjected {
					continue
				}

				if hasEntriesInContext(ctx, idx, ledger, workspaceRoot, target) {
					filename := filepath.Base(target)
					ephemeralSteps = append(ephemeralSteps, InjectStep{
						EphemeralMessage: fmt.Sprintf("[repo-memory] Check injected entries above before editing %s", filename),
					})
					ledger.RecordInjection(delivery.InjectionRecord{
						EntryID:   dedupKey,
						Channel:   delivery.ChannelEphemeral,
						Moment:    delivery.MomentAboutToEdit,
						StepIndex: input.InitialNumSteps,
					})
					break
				}
			}
		}
	}

	// --- Step 8: Coalesce all persistent SystemMessage blocks into a single InjectStep and persist ledger ---
	var injectSteps []InjectStep
	if coalesced := delivery.CoalesceContextBlocks(systemMessageBlocks); coalesced != "" {
		injectSteps = append(injectSteps, InjectStep{
			SystemMessage: &SystemMessageEntry{
				SystemMessage: coalesced,
			},
		})
	}
	injectSteps = append(injectSteps, ephemeralSteps...)

	// Persist ledger changes under the cross-process lock.
	// We replay only this invocation's new records (from injectionBase onwards)
	// into a freshly loaded ledger so concurrent writes are not lost.
	_ = ModifyLedger(dataDir, repoID, input.ConversationID, func(l *delivery.Ledger) {
		// If compaction or rewind cleared the seen markers, rearm before applying new ones.
		if rearmed {
			l.RearmDedup()
		}
		if rewound {
			var surviving []delivery.InjectionRecord
			for _, inj := range l.Injections {
				if inj.StepIndex <= maxRemainingStepIdx {
					surviving = append(surviving, inj)
				}
			}
			l.Injections = surviving
			l.LastProcessedStepIndex = ledger.LastProcessedStepIndex
			l.LastHandledCheckpointStep = ledger.LastHandledCheckpointStep
		}
		// Replay only this invocation's new injection records.
		for _, inj := range ledger.Injections[injectionBase:] {
			l.RecordInjection(inj)
		}
		// Copy seen markers and injected set added by this invocation.
		for k, v := range ledger.InjectedSet {
			if v {
				l.InjectedSet[k] = true
			}
		}
		for k, v := range ledger.SeenDossiers {
			if v {
				l.SeenDossiers[k] = true
			}
		}
		for k, v := range ledger.SeenAnchors {
			if v {
				l.SeenAnchors[k] = true
			}
		}
		// Copy checkpoint and high-water marks.
		if ledger.LastHandledCheckpointStep > l.LastHandledCheckpointStep {
			l.LastHandledCheckpointStep = ledger.LastHandledCheckpointStep
		}
		if ledger.LastProcessedStepIndex > l.LastProcessedStepIndex {
			l.LastProcessedStepIndex = ledger.LastProcessedStepIndex
		}
		l.LastKnownStepCount = ledger.LastKnownStepCount
	})

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

// matchAnchor reports whether anchor and target refer to the same file or path suffix.
func matchAnchor(anchor, target, workspaceRoot string) bool {
	// --- Step 1: Normalize and compare exact and workspace-joined paths ---
	cleanA := filepath.Clean(anchor)
	cleanT := filepath.Clean(target)
	if cleanA == cleanT {
		return true
	}
	if filepath.Join(workspaceRoot, cleanA) == filepath.Join(workspaceRoot, cleanT) {
		return true
	}
	// --- Step 2: Check path suffix and basename matches ---
	if strings.HasSuffix(cleanA, "/"+cleanT) || strings.HasSuffix(cleanT, "/"+cleanA) {
		return true
	}
	if !strings.Contains(cleanT, string(filepath.Separator)) && filepath.Base(cleanA) == cleanT {
		return true
	}
	if !strings.Contains(cleanA, string(filepath.Separator)) && filepath.Base(cleanT) == cleanA {
		return true
	}
	return false
}

// hasEntriesInContext reports whether the conversation ledger already holds
// injected entries or seen anchors matching targetFile.
func hasEntriesInContext(ctx context.Context, idx *index.Index, ledger *delivery.Ledger, workspaceRoot, targetFile string) bool {
	// --- Step 1: Check entries previously injected in this conversation ---
	for _, inj := range ledger.Injections {
		if inj.EntryID == "" || strings.HasPrefix(inj.EntryID, "about-to-edit:") {
			continue
		}
		ent, err := idx.GetEntry(ctx, inj.EntryID)
		if err == nil && ent != nil {
			for _, a := range ent.Anchors {
				if matchAnchor(a, targetFile, workspaceRoot) {
					return true
				}
			}
		}
	}

	// --- Step 2: Check seen anchors that have indexed entries ---
	for seenAnchor := range ledger.SeenAnchors {
		if matchAnchor(seenAnchor, targetFile, workspaceRoot) {
			ids, err := idx.LookupByAnchor(ctx, seenAnchor)
			if err == nil && len(ids) > 0 {
				return true
			}
		}
	}

	return false
}
