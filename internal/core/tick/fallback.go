/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/fallback.go
 * Description: Stage 1c Sidecar Fallback distillation (SPEC §6.3.3). Scans idle or
 *              subagent episodes with uncovered milestone steps past the conversation
 *              mark, scrubs and distills them via the Housekeeper harness, validates
 *              the output, writes sidecar inbox notes, and advances conversation marks.
 *
 * Exported Types / Functions:
 * - DefaultFallbackIdleThreshold:           Default idle threshold (2h) before fallback processes a conversation.
 * - MinFallbackQuotaFraction:               Minimum remaining model quota fraction required for fallback.
 * - PipelineExecutor.RunSidecarFallback():  Executes Stage 1c sidecar fallback across eligible episodes.
 */

package tick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/prepare"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/scrub"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

const (
	// DefaultFallbackIdleThreshold is the idle duration before Stage 1c Sidecar Fallback
	// processes a non-subagent conversation when turn_notes is "on".
	DefaultFallbackIdleThreshold = 2 * time.Hour
	// MinFallbackQuotaFraction is the minimum quota remaining fraction required to run sidecar fallback.
	MinFallbackQuotaFraction = 0.25
)

// now returns the current UTC timestamp using p.Now if configured or time.Now().
func (p *PipelineExecutor) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

// fallbackIdleThreshold returns p.FallbackIdleThreshold when positive, or DefaultFallbackIdleThreshold.
func (p *PipelineExecutor) fallbackIdleThreshold() time.Duration {
	if p.FallbackIdleThreshold > 0 {
		return p.FallbackIdleThreshold
	}
	return DefaultFallbackIdleThreshold
}

// checkQuotaHeadroom verifies that remaining model quota meets the configured spend guard quota floor.
func (p *PipelineExecutor) checkQuotaHeadroom(ctx context.Context) bool {
	return p.checkQuotaWithFloor(ctx, p.effectiveSpendConfig().QuotaFloor)
}

// RunSidecarFallback executes Stage 1c Sidecar Fallback (SPEC.md §6.3.3 / design-v2 §5.4).
// It scans episodes with uncovered milestone steps past the conversation mark that are eligible
// for fallback (idle >= 2h, subagent, or turn_notes == "fallback-only"), prepares and scrubs
// their session records, invokes Housekeeper (StageDistill / repo-memory-distiller), validates
// the output, writes source: "sidecar" inbox notes, and advances the conversation mark.
func (p *PipelineExecutor) RunSidecarFallback(ctx context.Context) (int, error) {
	// --- Step 1: Validate housekeeper, turn_notes mode, and episode store ---
	if ctx == nil {
		ctx = context.Background()
	}
	if p.hk == nil {
		return 0, nil
	}

	if p.cfg.TurnNotes == "" && p.repoRoot != "" {
		if loadedCfg, err := config.LoadRepoConfig(p.repoRoot); err == nil && loadedCfg.TurnNotes != "" {
			p.cfg.TurnNotes = loadedCfg.TurnNotes
		}
	}
	mode := p.cfg.TurnNotesMode()
	if mode == "off" {
		return 0, nil
	}

	if p.epStore == nil && p.dataDir != "" && p.repoID != "" {
		p.epStore = episode.NewStore(p.dataDir, p.repoID)
	}
	if p.epStore == nil {
		return 0, nil
	}

	episodes, err := p.epStore.ListEpisodes()
	if err != nil {
		return 0, err
	}
	if len(episodes) == 0 {
		return 0, nil
	}

	// --- Step 2: Open optional subagent store and evaluate A5 spend guards ---
	var subStore *state.SubagentStore
	if p.dataDir != "" && p.repoID != "" {
		dbPath := state.SubagentDBPath(p.dataDir, p.repoID)
		if _, statErr := os.Stat(dbPath); statErr == nil {
			if s, openErr := state.OpenSubagentStore(p.dataDir, p.repoID); openErr == nil {
				subStore = s
				defer subStore.Close()
			}
		}
	}

	notesWritten := 0
	distilledCount := 0
	totalEstimatedTokens := 0
	totalActualTokens := 0
	idleThreshold := p.fallbackIdleThreshold()
	now := p.now()

	allowed, _, maxItems := p.CheckSpendGuard("fallback", false)
	if !allowed {
		return 0, nil
	}

	// --- Step 3: Iterate over episodes and filter by mark, eligibility, and milestone gates ---
	for _, ep := range episodes {
		if err := ctx.Err(); err != nil {
			if distilledCount > 0 {
				p.RecordSpendRun("fallback", totalEstimatedTokens, totalActualTokens)
			}
			return notesWritten, err
		}
		if maxItems > 0 && distilledCount >= maxItems {
			break
		}
		if ep == nil {
			continue
		}

		convID := strings.TrimSpace(ep.Header.SessionID)
		if convID == "" {
			convID = strings.TrimSpace(ep.Header.EpisodeID)
		}
		if convID == "" {
			continue
		}

		mark, err := state.LoadMark(p.dataDir, p.repoID, convID)
		if err != nil || mark == nil {
			mark = &state.Mark{ConversationID: convID, NoteTitles: []string{}}
		}

		effectiveStepCount := len(ep.Steps)
		for _, s := range ep.Steps {
			if s.Index+1 > effectiveStepCount {
				effectiveStepCount = s.Index + 1
			}
		}
		if mark.ClampIfRewound(effectiveStepCount, "") {
			_ = state.SaveMark(p.dataDir, p.repoID, mark)
		}

		isInitialMark := mark.StepIndex == 0 && mark.UpdatedAt.IsZero() && mark.NoteCount == 0 && mark.LastNoteAt.IsZero()
		uncovered := stepsAfterMark(ep.Steps, mark.StepIndex, isInitialMark)
		if len(uncovered) == 0 {
			continue
		}

		firstStep, lastStep := stepBounds(uncovered, mark.StepIndex)

		isSubagent := ep.Header.GetParentEpisode() != ""
		if !isSubagent && subStore != nil {
			if ok, _, subErr := subStore.IsSubagent(ctx, convID); subErr == nil && ok {
				isSubagent = true
			}
		}

		if !isEligibleForFallback(mode, isSubagent, mark, ep.Header.Ended, now, idleThreshold) {
			continue
		}

		if !hasUncoveredMilestone(uncovered, ep.Steps, ep.Header, isInitialMark) {
			mark.StepIndex = lastStep
			mark.PendingContinuation = false
			mark.PendingRequestStep = 0
			_ = state.SaveMark(p.dataDir, p.repoID, mark)
			continue
		}

		if !p.checkQuotaHeadroom(ctx) {
			break
		}
		if err := p.checkBudget(nil); err != nil {
			break
		}

		// --- Step 4: Prepare session container, scrub secrets, and invoke Housekeeper distiller ---
		artifacts := loadConversationArtifactsMap(p.dataDir, p.repoID, convID)
		prepared, err := prepare.PrepareSession(convID, ep.Header.Checkpoints, uncovered, artifacts)
		if err != nil {
			continue
		}

		allowedIDs := buildAllowedIDs(convID, ep)
		scrubCtx := scrub.Context{
			Boundary:   "sidecar",
			RepoID:     p.repoID,
			RepoRoot:   p.repoRoot,
			DataDir:    p.dataDir,
			AllowedIDs: allowedIDs,
		}

		scrubbedContainer, err := scrub.Scrub(prepared.FormattedContainer, scrubCtx)
		if err != nil {
			_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar", err.Error(), []byte(prepared.FormattedContainer))
			continue
		}

		guardSt, _ := LoadSpendGuardState(p.dataDir, p.repoID)
		estTokens := EstimateTokensCalibrated(scrubbedContainer.Text, guardSt)

		res, err := p.hk.Run(ctx, harness.StageDistill, scrubbedContainer.Text)
		cost := jobs.FromUsage(res.Usage)
		_ = p.checkBudget(&cost)
		if err != nil {
			if errors.Is(err, harness.ErrQuotaExhausted) {
				break
			}
			continue
		}
		distilledCount++
		totalEstimatedTokens += estTokens
		totalActualTokens += res.Usage.InputTokens + res.Usage.OutputTokens

		// --- Step 5: Validate distiller output, scrub fields, write inbox note, and advance mark ---
		items, valErr := prepare.ValidateDistillerOutput(res.Payload, prepared)
		if valErr != nil {
			_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", valErr.Error(), []byte(res.Payload))
			mark.StepIndex = lastStep
			mark.PendingContinuation = false
			mark.PendingRequestStep = 0
			_ = state.SaveMark(p.dataDir, p.repoID, mark)
			continue
		}

		lastStepTS := extractLastStepTimestamp(uncovered, ep.Header.Ended, mark.StepTimestamp)
		written := 0

		if len(items) > 0 {
			scrubFailed := false
			var allAnchors []string
			seenAnchor := make(map[string]bool)

			for i := range items {
				var sErr error
				if items[i].Title, sErr = scrubFieldText(items[i].Title, scrubCtx); sErr != nil {
					scrubFailed = true
					_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", sErr.Error(), []byte(res.Payload))
					break
				}
				if items[i].Why, sErr = scrubFieldText(items[i].Why, scrubCtx); sErr != nil {
					scrubFailed = true
					_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", sErr.Error(), []byte(res.Payload))
					break
				}
				if items[i].Do, sErr = scrubFieldText(items[i].Do, scrubCtx); sErr != nil {
					scrubFailed = true
					_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", sErr.Error(), []byte(res.Payload))
					break
				}
				if items[i].Evidence, sErr = scrubFieldText(items[i].Evidence, scrubCtx); sErr != nil {
					scrubFailed = true
					_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", sErr.Error(), []byte(res.Payload))
					break
				}
				if items[i].ClosesWhen, sErr = scrubFieldText(items[i].ClosesWhen, scrubCtx); sErr != nil {
					scrubFailed = true
					_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", sErr.Error(), []byte(res.Payload))
					break
				}
				if items[i].ErrorPattern, sErr = scrubFieldText(items[i].ErrorPattern, scrubCtx); sErr != nil {
					scrubFailed = true
					_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", sErr.Error(), []byte(res.Payload))
					break
				}

				items[i].AnchorChecks = inbox.CheckAnchors(p.repoRoot, items[i].Anchors)
				for _, a := range items[i].Anchors {
					trimmed := strings.TrimSpace(a)
					if trimmed != "" && !seenAnchor[trimmed] {
						seenAnchor[trimmed] = true
						allAnchors = append(allAnchors, trimmed)
					}
				}
			}

			if scrubFailed {
				mark.StepIndex = lastStep
				mark.StepTimestamp = lastStepTS
				mark.PendingContinuation = false
				mark.PendingRequestStep = 0
				mark.LastFallbackAt = now
				_ = state.SaveMark(p.dataDir, p.repoID, mark)
				continue
			}

			allAnchorChecks := inbox.CheckAnchors(p.repoRoot, allAnchors)
			exposures := collectEpisodeExposures(p.dataDir, p.repoID, convID, ep)

			note := &inbox.Note{
				Source:         inbox.SourceSidecar,
				ConversationID: convID,
				StepsCovered:   [2]int{firstStep, lastStep},
				Model:          ep.Header.Model,
				HarnessVersion: ep.Header.HarnessVersion,
				Branch:         ep.Header.Branch,
				HeadCommit:     firstNonEmptyStr(ep.Header.HeadEnd, ep.Header.HeadStart),
				CreatedAt:      now,
				ExposureList:   exposures,
				AnchorChecks:   allAnchorChecks,
				Items:          items,
			}
			if _, writeErr := inbox.NewStore(p.dataDir, p.repoID).Write(note); writeErr == nil {
				written++
				notesWritten++
				for _, it := range items {
					if t := strings.TrimSpace(it.Title); t != "" {
						mark.NoteTitles = append(mark.NoteTitles, t)
					}
				}
			}
		}

		mark.StepIndex = lastStep
		mark.StepTimestamp = lastStepTS
		mark.PendingContinuation = false
		mark.PendingRequestStep = 0
		mark.LastFallbackAt = now
		if written > 0 {
			mark.LastNoteAt = now
			mark.NoteCount++
			mark.NotesToday++
		}
		_ = state.SaveMark(p.dataDir, p.repoID, mark)
	}

	// --- Step 6: Record spend guard telemetry if any episodes were distilled ---
	if distilledCount > 0 {
		p.RecordSpendRun("fallback", totalEstimatedTokens, totalActualTokens)
	}

	return notesWritten, nil
}

// stepsAfterMark returns the slice of steps occurring after markStepIndex (or all steps >= 0 when isInitialMark is true).
func stepsAfterMark(steps []harness.Step, markStepIndex int, isInitialMark bool) []harness.Step {
	var out []harness.Step
	for _, s := range steps {
		if isInitialMark {
			if s.Index >= 0 {
				out = append(out, s)
			}
		} else if s.Index > markStepIndex {
			out = append(out, s)
		}
	}
	return out
}

// stepBounds returns the minimum and maximum step indices in uncovered, or defaultStart when uncovered is empty.
func stepBounds(uncovered []harness.Step, defaultStart int) (firstStep, lastStep int) {
	// --- Step 1: Handle empty slice fallback ---
	if len(uncovered) == 0 {
		return defaultStart, defaultStart
	}

	// --- Step 2: Scan uncovered steps for minimum and maximum index ---
	firstStep = uncovered[0].Index
	lastStep = uncovered[0].Index
	for _, s := range uncovered[1:] {
		if s.Index < firstStep {
			firstStep = s.Index
		}
		if s.Index > lastStep {
			lastStep = s.Index
		}
	}
	return firstStep, lastStep
}

// extractLastStepTimestamp extracts the latest step timestamp from uncovered RawData, falling back to ended or currentTS.
func extractLastStepTimestamp(uncovered []harness.Step, ended time.Time, currentTS string) string {
	// --- Step 1: Search uncovered steps in reverse order for a raw timestamp field ---
	for i := len(uncovered) - 1; i >= 0; i-- {
		raw := uncovered[i].RawData
		if len(raw) == 0 {
			continue
		}
		for _, k := range []string{"created_at", "timestamp", "time"} {
			if v, ok := raw[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}

	// --- Step 2: Fall back to episode Ended timestamp or existing mark timestamp ---
	if !ended.IsZero() {
		return ended.UTC().Format(time.RFC3339)
	}
	return currentTS
}

// isEligibleForFallback reports whether an episode satisfies the mode, subagent, or idle-duration rules for Stage 1c fallback.
func isEligibleForFallback(
	mode string,
	isSubagent bool,
	mark *state.Mark,
	ended time.Time,
	now time.Time,
	idleThreshold time.Duration,
) bool {
	// --- Step 1: Immediately allow fallback-only mode or subagent sessions ---
	if mode == "fallback-only" || isSubagent {
		return true
	}

	// --- Step 2: Check pending continuation or request step against idleThreshold ---
	if mark != nil && (mark.PendingContinuation || mark.PendingRequestStep > 0) {
		if !ended.IsZero() && now.Sub(ended) >= idleThreshold {
			return true
		}
		if ended.IsZero() && (mark.UpdatedAt.IsZero() || now.Sub(mark.UpdatedAt) >= idleThreshold) {
			return true
		}
	}

	// --- Step 3: Check whether the episode ended at least idleThreshold ago ---
	if !ended.IsZero() && now.Sub(ended) >= idleThreshold {
		return true
	}
	return false
}

// hasUncoveredMilestone reports whether uncovered steps contain at least one milestone event
// (commit/push/PR, >= 2 file edits + build/test, WORKORDER/PROJECT update, or error resolved after >= 15 steps).
func hasUncoveredMilestone(uncovered []harness.Step, allSteps []harness.Step, hdr episode.Header, isInitialMark bool) bool {
	// --- Step 1: Check header commits on initial mark ---
	if isInitialMark && len(hdr.Commits) > 0 {
		for _, c := range hdr.Commits {
			if strings.TrimSpace(c) != "" {
				return true
			}
		}
	}

	distinctEdits := make(map[string]bool)
	hasTestOrBuild := false

	// --- Step 2: Scan uncovered steps for commits, edits, test/build runs, and PA updates ---
	for _, s := range uncovered {
		// 1 & 2. Commit, branch push, or PR creation/merge.
		if strings.TrimSpace(s.Commit) != "" {
			return true
		}
		cmdLower := strings.ToLower(s.Command + " " + s.Text + " " + s.Action)
		for _, kw := range []string{
			"git commit",
			"git merge",
			"git cherry-pick",
			"git push",
			"gh pr create",
			"gh pr merge",
		} {
			if strings.Contains(cmdLower, kw) {
				return true
			}
		}

		// 3. Track file edits and test/build steps.
		if s.Kind == harness.StepKindEdit {
			key := strings.TrimSpace(s.File)
			if key != "" {
				key = filepath.ToSlash(filepath.Clean(key))
			} else {
				key = fmt.Sprintf("edit-step-%d", s.Index)
			}
			distinctEdits[key] = true
		}
		if isEpisodeTestOrBuildStep(s) {
			hasTestOrBuild = true
		}

		// 5. Principal Architect WORKORDER.md or PROJECT.md edited/committed.
		fileLower := strings.ToLower(filepath.ToSlash(strings.TrimSpace(s.File)))
		if (s.Kind == harness.StepKindEdit || strings.TrimSpace(s.Commit) != "") &&
			(strings.HasSuffix(fileLower, "workorder.md") || strings.HasSuffix(fileLower, "project.md")) {
			return true
		}
		if strings.Contains(cmdLower, "workorder.md") && strings.Contains(cmdLower, "completion report") {
			return true
		}
	}

	if len(distinctEdits) >= 2 && hasTestOrBuild {
		return true
	}

	// --- Step 3: Check for error resolution spanning at least 15 steps ---
	firstFailStepIdx := -1
	for _, s := range allSteps {
		if isEpisodeStepFailed(s) {
			firstFailStepIdx = s.Index
			break
		}
	}
	if firstFailStepIdx >= 0 {
		for _, s := range uncovered {
			if isEpisodeStepSucceeded(s) && (s.Index-firstFailStepIdx) >= 15 {
				return true
			}
		}
	}

	return false
}

// isEpisodeTestOrBuildStep reports whether step s represents a test or build execution.
func isEpisodeTestOrBuildStep(s harness.Step) bool {
	// --- Step 1: Check explicit test step kind ---
	if s.Kind == harness.StepKindTest {
		return true
	}

	// --- Step 2: Match known build and test command patterns ---
	combined := strings.ToLower(s.Command + " " + s.Action + " " + s.Suite)
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
		if strings.Contains(combined, pat) {
			return true
		}
	}
	return false
}

// isEpisodeStepFailed reports whether command or test step s failed based on exit code, counts, outcome, or output markers.
func isEpisodeStepFailed(s harness.Step) bool {
	// --- Step 1: Check step kind, exit code, and failure count ---
	if s.Kind != harness.StepKindCommand && s.Kind != harness.StepKindTest {
		return false
	}
	if s.ExitCode != nil && *s.ExitCode != 0 {
		return true
	}
	if s.Failed > 0 {
		return true
	}

	// --- Step 2: Check normalized outcome label and failure text markers ---
	outcome := strings.ToLower(strings.TrimSpace(s.Outcome))
	if outcome == "error" || outcome == "failed" || outcome == "fail" {
		return true
	}
	lower := strings.ToLower(s.Excerpt + " " + s.Text)
	for _, marker := range []string{
		"exit code 1",
		"exit code 2",
		"exit status 1",
		"--- fail:",
		"build failed",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// isEpisodeStepSucceeded reports whether command or test step s succeeded.
func isEpisodeStepSucceeded(s harness.Step) bool {
	// --- Step 1: Reject non-command/test steps and steps that failed ---
	if s.Kind != harness.StepKindCommand && s.Kind != harness.StepKindTest {
		return false
	}
	if isEpisodeStepFailed(s) {
		return false
	}

	// --- Step 2: Check zero exit code, pass counts, or success outcome ---
	if s.ExitCode != nil && *s.ExitCode == 0 {
		return true
	}
	if s.Passed > 0 && s.Failed == 0 {
		return true
	}
	outcome := strings.ToLower(strings.TrimSpace(s.Outcome))
	if outcome == "success" || outcome == "passed" || outcome == "ok" || outcome == "pass" {
		return true
	}
	return isEpisodeTestOrBuildStep(s)
}

// loadConversationArtifactsMap reads markdown artifacts stored under the conversation's evidence directory.
func loadConversationArtifactsMap(dataDir, repoID, convID string) map[string]string {
	// --- Step 1: Validate inputs and check conversation artifact directory ---
	if dataDir == "" || repoID == "" || convID == "" {
		return nil
	}
	convDir := filepath.Join(dataDir, repoID, "evidence", "artifacts", state.SanitizeConversationID(convID))
	info, err := os.Stat(convDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	// --- Step 2: Walk directory for user-authored markdown files ---
	var mdFiles []string
	_ = filepath.WalkDir(convDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".system_generated" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") ||
			strings.HasSuffix(name, ".metadata.json") ||
			strings.HasSuffix(name, ".resolved") ||
			strings.HasSuffix(name, ".md.resolved") {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if len(mdFiles) == 0 {
		return nil
	}

	// --- Step 3: Read and key non-empty markdown contents by relative slash path ---
	sort.Strings(mdFiles)
	out := make(map[string]string, len(mdFiles))
	for _, fullPath := range mdFiles {
		rel, err := filepath.Rel(convDir, fullPath)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
			out[filepath.ToSlash(rel)] = trimmed
		}
	}
	return out
}

// buildAllowedIDs constructs the set of known session, episode, and commit identifiers exempt from high-entropy scrubbing.
func buildAllowedIDs(convID string, ep *episode.Episode) map[string]bool {
	// --- Step 1: Initialize allowlist helper and add conversation ID ---
	allowed := map[string]bool{}
	add := func(id string) {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			allowed[trimmed] = true
		}
	}
	add(convID)

	// --- Step 2: Add episode header and step commit identifiers ---
	if ep != nil {
		add(ep.Header.EpisodeID)
		add(ep.Header.SessionID)
		add(ep.Header.HeadStart)
		add(ep.Header.HeadEnd)
		for _, c := range ep.Header.Commits {
			add(c)
		}
		for _, s := range ep.Steps {
			add(s.Commit)
		}
	}
	return allowed
}

// scrubFieldText runs scrub.Scrub on a non-empty note item field string.
func scrubFieldText(val string, ctx scrub.Context) (string, error) {
	if val == "" {
		return "", nil
	}
	res, err := scrub.Scrub(val, ctx)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// collectEpisodeExposures aggregates deduplicated ExposureRecords from the episode header, steps, and delivery ledger.
func collectEpisodeExposures(dataDir, repoID, convID string, ep *episode.Episode) []delivery.ExposureRecord {
	// --- Step 1: Set up deduplicating accumulator ---
	var out []delivery.ExposureRecord
	seen := make(map[string]bool)

	addExp := func(rec delivery.ExposureRecord) {
		id := strings.TrimSpace(rec.EntryID)
		if id == "" {
			return
		}
		rec.EntryID = id
		key := fmt.Sprintf("%s|%s|%s", rec.EntryID, rec.Channel, rec.Moment)
		if !seen[key] {
			seen[key] = true
			out = append(out, rec)
		}
	}

	// --- Step 2: Collect exposures recorded on episode header and exposure steps ---
	if ep != nil {
		for _, id := range ep.Header.Exposures {
			addExp(delivery.ExposureRecord{EntryID: id})
		}
		for _, s := range ep.Steps {
			if s.Kind != harness.StepKindExposure {
				continue
			}
			ch := delivery.Channel(strings.TrimSpace(s.Via))
			mom := delivery.Moment(strings.TrimSpace(s.Trigger))
			if len(s.Claims) > 0 {
				for _, cid := range s.Claims {
					addExp(delivery.ExposureRecord{
						EntryID: cid,
						Channel: ch,
						Moment:  mom,
					})
				}
			} else if strings.TrimSpace(s.Action) != "" {
				addExp(delivery.ExposureRecord{
					EntryID: s.Action,
					Channel: ch,
					Moment:  mom,
				})
			} else if strings.TrimSpace(s.Text) != "" {
				addExp(delivery.ExposureRecord{
					EntryID: s.Text,
					Channel: ch,
					Moment:  mom,
				})
			}
		}
	}

	// --- Step 3: Merge exposures from on-disk conversation delivery ledger ---
	if dataDir != "" && repoID != "" && convID != "" {
		ledgerPath := filepath.Join(dataDir, repoID, "ledgers", state.SanitizeConversationID(convID)+".json")
		if data, err := os.ReadFile(ledgerPath); err == nil {
			var l delivery.Ledger
			if json.Unmarshal(data, &l) == nil {
				for _, rec := range l.ExposureList {
					addExp(rec)
				}
			}
		}
	}

	return out
}

// firstNonEmptyStr returns the first non-whitespace string from vals, or "".
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
