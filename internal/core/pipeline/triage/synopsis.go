package triage

import (
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// Synopsis represents the summarized content of an episode for extraction.
type Synopsis struct {
	FilesBySubsystem map[string][]string `json:"files_by_subsystem"`
	Commits          []string            `json:"commits"`
	Outcomes         []string            `json:"outcomes"`
	Exposures        []string            `json:"exposures"`
	HumanMessages    []string            `json:"human_messages"`
}

// GenerateSynopsis creates a synopsis for the given episode.
func GenerateSynopsis(ep *episode.Episode, subsystems []string) Synopsis {
	syn := Synopsis{
		FilesBySubsystem: make(map[string][]string),
		Commits:          ep.Header.Commits,
		Outcomes:         make([]string, 0),
		Exposures:        make([]string, 0),
		HumanMessages:    make([]string, 0),
	}

	touchedFiles := make(map[string]bool)

	for _, step := range ep.Steps {
		// Files touched
		if step.File != "" && (step.Kind == harness.StepKindEdit || step.Kind == harness.StepKindRead) {
			if !touchedFiles[step.File] {
				touchedFiles[step.File] = true
				subsystem := matchSubsystem(step.File, subsystems)
				syn.FilesBySubsystem[subsystem] = append(syn.FilesBySubsystem[subsystem], step.File)
			}
		}

		// Outcomes
		if step.Outcome != "" {
			syn.Outcomes = append(syn.Outcomes, step.Outcome)
		} else if isFailureOutcome(step) {
			syn.Outcomes = append(syn.Outcomes, "failure")
		} else if isSuccessOutcome(step) {
			syn.Outcomes = append(syn.Outcomes, "success")
		}

		// Exposures
		if step.Kind == harness.StepKindExposure {
			syn.Exposures = append(syn.Exposures, step.Excerpt)
		} else if step.Via != "" {
			syn.Exposures = append(syn.Exposures, step.Via)
		}

		// Human messages verbatim
		if step.Kind == harness.StepKindUser {
			syn.HumanMessages = append(syn.HumanMessages, step.Text)
		}
	}

	return syn
}

func matchSubsystem(file string, subsystems []string) string {
	for _, sub := range subsystems {
		if strings.Contains(file, sub) {
			return sub
		}
	}
	return "unknown"
}
