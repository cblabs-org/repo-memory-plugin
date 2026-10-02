package triage

import (
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// Window represents a range of steps [Start, End] inclusive.
type Window struct {
	Start int
	End   int
}

// WindowsConfig holds configurable values for salience window generation.
type WindowsConfig struct {
	DecisionMarkers []string
}

// DefaultWindowsConfig returns default configuration for windows.
func DefaultWindowsConfig() WindowsConfig {
	return WindowsConfig{
		DecisionMarkers: []string{"trade-off", "instead of", "rather than", "decided to", "because", "invariant", "constraint", "must not", "do not use", "ADR"},
	}
}

// CalculateWindows calculates salience windows for an episode.
func CalculateWindows(ep *episode.Episode, cfg WindowsConfig, triageCfg Config) []Window {
	var windows []Window
	decisionCount := 0

	// We need to match failures to their eventual fixes/abandonments
	// target -> index of first failure
	failures := make(map[string]int)

	for i, step := range ep.Steps {
		// Failure and its eventual fix or abandonment
		if isFailureOutcome(step) {
			target := getTarget(step)
			if target != "" {
				if _, exists := failures[target]; !exists {
					failures[target] = i // first failure
				}
			}
		} else if isSuccessOutcome(step) || step.Action == "abandon" {
			target := getTarget(step)
			if target != "" {
				if startIdx, exists := failures[target]; exists {
					// From the first failure to 2 steps after the success or abandonment
					windows = append(windows, Window{Start: startIdx, End: i + 2})
					delete(failures, target) // resolved
				}
			}
		}

		// Revert or abandoned edit
		if step.Action == "revert" || step.Action == "abandon" {
			windows = append(windows, Window{Start: i - 5, End: i + 5})
		}

		// Human message with a decision, preference, correction or approval signal
		if step.Kind == harness.StepKindUser {
			if containsDecisionSignal(step.Text) {
				windows = append(windows, Window{Start: i - 3, End: i + 3})
			} else {
				// human messages themselves are always included
				windows = append(windows, Window{Start: i, End: i})
			}
		}

		// Write to a memory artefact, or memory.note call
		if step.Kind == harness.StepKindNote || (step.Kind == harness.StepKindEdit && isMemoryArtefact(step.File, triageCfg.MemoryArtefacts)) {
			// The note step, plus the steps back to the start of the failure, discussion or decision it records (at most 20)
			startIdx := i - 20
			if startIdx < 0 {
				startIdx = 0
			}
			windows = append(windows, Window{Start: startIdx, End: i})
		}

		// Reasoning span that weighs alternatives, or human message that sets constraint/trade-off
		if decisionCount < 2 {
			if hasDecisionMarker(step.Text, cfg.DecisionMarkers) {
				windows = append(windows, Window{Start: i, End: i + 2})
				decisionCount++
			}
		}

		// Device, flash or soak outcome
		if step.Kind == harness.StepKindDevice || step.Action == "soak" || step.Action == "flash" {
			windows = append(windows, Window{Start: i - 5, End: i + 5})
		}
	}

	return mergeWindows(windows, len(ep.Steps))
}

func hasDecisionMarker(text string, markers []string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// mergeWindows merges overlapping windows and clamps them to the episode boundaries.
func mergeWindows(windows []Window, totalSteps int) []Window {
	if len(windows) == 0 {
		return nil
	}
	
	// Clamp to 0..totalSteps-1
	for i := range windows {
		if windows[i].Start < 0 {
			windows[i].Start = 0
		}
		if windows[i].End >= totalSteps {
			windows[i].End = totalSteps - 1
		}
	}
	
	// Sort by start index
	for i := 0; i < len(windows)-1; i++ {
		for j := i + 1; j < len(windows); j++ {
			if windows[i].Start > windows[j].Start {
				windows[i], windows[j] = windows[j], windows[i]
			}
		}
	}
	
	var merged []Window
	curr := windows[0]
	
	for i := 1; i < len(windows); i++ {
		w := windows[i]
		if w.Start <= curr.End+1 {
			// Overlapping or contiguous
			if w.End > curr.End {
				curr.End = w.End
			}
		} else {
			merged = append(merged, curr)
			curr = w
		}
	}
	merged = append(merged, curr)
	
	return merged
}
