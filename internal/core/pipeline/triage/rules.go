package triage

import (
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

type Config struct {
	MemoryArtefacts []string
	Subsystems      []string // Assuming list of globs or similar
	MinSteps        int
}

// DefaultConfig returns the default triage configuration.
func DefaultConfig() Config {
	return Config{
		MemoryArtefacts: []string{"MEMORY.md", "PROJECT.md", "WORKORDER.md"},
		MinSteps:        5,
	}
}

// EvaluateRules evaluates an episode against the deterministic triage rules.
// It returns true if the episode should be kept, false if it should be skipped,
// and reasons for the decision.
func EvaluateRules(ep *episode.Episode, cfg Config) (keep bool, skip bool, reasons []string) {
	hasFailure := false
	hasCommit := ep.HasCommits()
	hasDecisionSignal := false

	failuresByTarget := make(map[string]bool)

	for _, step := range ep.Steps {
		// 1. Failure followed by success on the same target
		if isFailureOutcome(step) {
			hasFailure = true
			target := getTarget(step)
			if target != "" {
				failuresByTarget[target] = true
			}
		} else if isSuccessOutcome(step) {
			target := getTarget(step)
			if target != "" && failuresByTarget[target] {
				keep = true
				reasons = append(reasons, "failure outcome followed later by a success on the same target")
			}
		}

		// 2. Revert or abandoned edit
		if step.Action == "revert" || step.Action == "abandon" {
			keep = true
			reasons = append(reasons, "revert or abandoned edit")
		}

		// 3. User message containing a decision, preference, correction, or approval signal
		if step.Kind == harness.StepKindUser && containsDecisionSignal(step.Text) {
			hasDecisionSignal = true
			keep = true
			reasons = append(reasons, "user message containing a decision, preference, correction or approval signal")
		}

		// 4. New file under a subsystem glob
		if step.Kind == harness.StepKindEdit && step.Action == "create" && matchesSubsystem(step.File, cfg.Subsystems) {
			keep = true
			reasons = append(reasons, "new file under a subsystem glob")
		}

		// 5. Dependency or config change
		if isDependencyOrConfigChange(step) {
			keep = true
			reasons = append(reasons, "dependency or config change")
		}

		// 6. Reasoning spans above a length threshold discussing alternatives
		if isReasoningSpan(step) {
			keep = true
			reasons = append(reasons, "reasoning span discussing alternatives")
		}

		// 7. Device or soak outcome
		if step.Kind == harness.StepKindDevice || step.Action == "soak" {
			keep = true
			reasons = append(reasons, "device or soak outcome")
		}

		// 8. Write to a memory artefact, or memory.note call
		if step.Kind == harness.StepKindNote || (step.Kind == harness.StepKindEdit && isMemoryArtefact(step.File, cfg.MemoryArtefacts)) {
			keep = true
			reasons = append(reasons, "write to a memory artefact or memory.note call")
		}
	}

	// Skip if all hold: no failures, no commits, fewer than K steps, and no decision signals.
	if !keep && !hasFailure && !hasCommit && len(ep.Steps) < cfg.MinSteps && !hasDecisionSignal {
		skip = true
		reasons = append(reasons, "skip-all condition met: no failures, no commits, short episode, no decisions")
	}

	return keep, skip, reasons
}

func isFailureOutcome(s harness.Step) bool {
	return s.Outcome == "failure" || s.Outcome == "error" || (s.ExitCode != nil && *s.ExitCode != 0) || s.Failed > 0
}

func isSuccessOutcome(s harness.Step) bool {
	return s.Outcome == "success" || (s.ExitCode != nil && *s.ExitCode == 0) || s.Passed > 0
}

func getTarget(s harness.Step) string {
	if s.File != "" {
		return s.File
	}
	if s.Suite != "" {
		return s.Suite
	}
	if s.Command != "" {
		// Just use the first word of the command as a rough target
		return strings.Fields(s.Command)[0]
	}
	return ""
}

func containsDecisionSignal(text string) bool {
	lower := strings.ToLower(text)
	signals := []string{"decide", "prefer", "correct", "approve", "looks good", "lgtm", "instead", "rather than"}
	for _, sig := range signals {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}

func matchesSubsystem(file string, subsystems []string) bool {
	// Simple matching for now
	for _, sub := range subsystems {
		if strings.Contains(file, sub) {
			return true
		}
	}
	return false
}

func isDependencyOrConfigChange(s harness.Step) bool {
	if s.Kind == harness.StepKindEdit {
		lower := strings.ToLower(s.File)
		return strings.Contains(lower, "go.mod") || strings.Contains(lower, "package.json") ||
			strings.Contains(lower, "requirements.txt") || strings.Contains(lower, "config.yaml")
	}
	return false
}

func isReasoningSpan(s harness.Step) bool {
	// Crude check: long thinking block containing "alternative", "trade-off", etc.
	if len(s.Text) > 200 {
		lower := strings.ToLower(s.Text)
		return strings.Contains(lower, "alternative") || strings.Contains(lower, "trade-off") || strings.Contains(lower, "instead")
	}
	return false
}

func isMemoryArtefact(file string, artefacts []string) bool {
	for _, a := range artefacts {
		if strings.HasSuffix(file, a) {
			return true
		}
	}
	return false
}
