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
	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

const (
	// DefaultFallbackIdleThreshold is the idle duration before Stage 1c Sidecar Fallback
	// processes a non-subagent conversation when turn_notes is "on".
	DefaultFallbackIdleThreshold = 2 * time.Hour
	// MinFallbackQuotaFraction is the minimum quota remaining fraction required to run sidecar fallback.
	MinFallbackQuotaFraction = 0.25
)

func (p *PipelineExecutor) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC()
	}
	return time.Now().UTC()
}

func (p *PipelineExecutor) fallbackIdleThreshold() time.Duration {
	if p.FallbackIdleThreshold > 0 {
		return p.FallbackIdleThreshold
	}
	return DefaultFallbackIdleThreshold
}

func (p *PipelineExecutor) checkQuotaHeadroom(ctx context.Context) bool {
	checker := p.QuotaChecker
	if checker == nil {
		checker = telemetry.CheckQuota
	}

	qs, err := checker(ctx)
	if err != nil {
		if errors.Is(err, harness.ErrQuotaExhausted) {
			return false
		}
		// If agy is not installed or /usage is unavailable, treat quota as unknown/available.
		return true
	}
	if qs == nil {
		return true
	}

	var allBuckets []telemetry.QuotaBucket
	allBuckets = append(allBuckets, qs.Buckets...)
	for _, g := range qs.Groups {
		allBuckets = append(allBuckets, g.Buckets...)
	}
	if len(allBuckets) == 0 {
		return true
	}

	var flashBuckets []telemetry.QuotaBucket
	for _, b := range allBuckets {
		if strings.Contains(strings.ToLower(b.ID), "flash") || strings.Contains(strings.ToLower(b.Name), "flash") {
			flashBuckets = append(flashBuckets, b)
		}
	}

	targetBuckets := flashBuckets
	if len(targetBuckets) == 0 {
		targetBuckets = allBuckets
	}

	for _, b := range targetBuckets {
		if b.RemainingFraction < MinFallbackQuotaFraction {
			return false
		}
	}
	return true
}

// RunSidecarFallback executes Stage 1c Sidecar Fallback (SPEC.md §6.3.3 / design-v2 §5.4).
// It scans episodes with uncovered milestone steps past the conversation mark that are eligible
// for fallback (idle >= 2h, subagent, or turn_notes == "fallback-only"), prepares and scrubs
// their session records, invokes Housekeeper (StageDistill / repo-memory-distiller), validates
// the output, writes source: "sidecar" inbox notes, and advances the conversation mark.
func (p *PipelineExecutor) RunSidecarFallback(ctx context.Context) (int, error) {
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
	idleThreshold := p.fallbackIdleThreshold()
	now := p.now()

	for _, ep := range episodes {
		if err := ctx.Err(); err != nil {
			return notesWritten, err
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

		res, err := p.hk.Run(ctx, harness.StageDistill, scrubbedContainer.Text)
		cost := jobs.FromUsage(res.Usage)
		_ = p.checkBudget(&cost)
		if err != nil {
			if errors.Is(err, harness.ErrQuotaExhausted) {
				break
			}
			continue
		}

		items, valErr := prepare.ValidateDistillerOutput(res.Payload, prepared)
		if valErr != nil {
			_, _ = scrub.QuarantinePayload(p.dataDir, p.repoID, "sidecar-output", valErr.Error(), []byte(res.Payload))
			mark.StepIndex = lastStep
			mark.PendingContinuation = false
			mark.PendingRequestStep = 0
			_ = state.SaveMark(p.dataDir, p.repoID, mark)
			continue
		}

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
				mark.PendingContinuation = false
				mark.PendingRequestStep = 0
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
				notesWritten++
				for _, it := range items {
					if t := strings.TrimSpace(it.Title); t != "" {
						mark.NoteTitles = append(mark.NoteTitles, t)
					}
				}
			}
		}

		mark.StepIndex = lastStep
		mark.PendingContinuation = false
		mark.PendingRequestStep = 0
		mark.LastNoteAt = now
		mark.NoteCount++
		mark.NotesToday++
		_ = state.SaveMark(p.dataDir, p.repoID, mark)
	}

	return notesWritten, nil
}

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

func stepBounds(uncovered []harness.Step, defaultStart int) (firstStep, lastStep int) {
	if len(uncovered) == 0 {
		return defaultStart, defaultStart
	}
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

func isEligibleForFallback(
	mode string,
	isSubagent bool,
	mark *state.Mark,
	ended time.Time,
	now time.Time,
	idleThreshold time.Duration,
) bool {
	if mode == "fallback-only" || isSubagent {
		return true
	}
	if mark != nil && (mark.PendingContinuation || mark.PendingRequestStep > 0) {
		if !ended.IsZero() && now.Sub(ended) >= idleThreshold {
			return true
		}
		if ended.IsZero() && (mark.UpdatedAt.IsZero() || now.Sub(mark.UpdatedAt) >= idleThreshold) {
			return true
		}
	}
	if !ended.IsZero() && now.Sub(ended) >= idleThreshold {
		return true
	}
	return false
}

func hasUncoveredMilestone(uncovered []harness.Step, allSteps []harness.Step, hdr episode.Header, isInitialMark bool) bool {
	if isInitialMark && len(hdr.Commits) > 0 {
		for _, c := range hdr.Commits {
			if strings.TrimSpace(c) != "" {
				return true
			}
		}
	}

	distinctEdits := make(map[string]bool)
	hasTestOrBuild := false

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

	// 4. Error resolved after >= 15 steps.
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

func isEpisodeTestOrBuildStep(s harness.Step) bool {
	if s.Kind == harness.StepKindTest {
		return true
	}
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

func isEpisodeStepFailed(s harness.Step) bool {
	if s.Kind != harness.StepKindCommand && s.Kind != harness.StepKindTest {
		return false
	}
	if s.ExitCode != nil && *s.ExitCode != 0 {
		return true
	}
	if s.Failed > 0 {
		return true
	}
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

func isEpisodeStepSucceeded(s harness.Step) bool {
	if s.Kind != harness.StepKindCommand && s.Kind != harness.StepKindTest {
		return false
	}
	if isEpisodeStepFailed(s) {
		return false
	}
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

func loadConversationArtifactsMap(dataDir, repoID, convID string) map[string]string {
	if dataDir == "" || repoID == "" || convID == "" {
		return nil
	}
	convDir := filepath.Join(dataDir, repoID, "evidence", "artifacts", state.SanitizeConversationID(convID))
	info, err := os.Stat(convDir)
	if err != nil || !info.IsDir() {
		return nil
	}

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

func buildAllowedIDs(convID string, ep *episode.Episode) map[string]bool {
	allowed := map[string]bool{}
	add := func(id string) {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			allowed[trimmed] = true
		}
	}
	add(convID)
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

func collectEpisodeExposures(dataDir, repoID, convID string, ep *episode.Episode) []delivery.ExposureRecord {
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

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
