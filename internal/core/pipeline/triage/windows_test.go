package triage_test

import (
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestCalculateWindows(t *testing.T) {
	cfg := triage.DefaultWindowsConfig()
	triageCfg := triage.DefaultConfig()

	// Failure followed by success
	ep1 := episode.NewEpisode(episode.Header{EpisodeID: "ep1"},
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 0
		harness.Step{Kind: harness.StepKindCommand, Outcome: "failure", Command: "test"}, // 1 (Failure)
		harness.Step{Kind: harness.StepKindEdit, Action: "edit"}, // 2
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "test"}, // 3 (Success)
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 4
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 5
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 6
	)

	windows := triage.CalculateWindows(ep1, cfg, triageCfg)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0].Start != 1 || windows[0].End != 5 { // 1 to 3+2
		t.Errorf("expected window [1, 5], got [%d, %d]", windows[0].Start, windows[0].End)
	}
	
	// Reasoning span
	ep2 := episode.NewEpisode(episode.Header{EpisodeID: "ep2"},
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 0
		harness.Step{Kind: harness.StepKindCommand, Text: "I decided to use this because it's better"}, // 1 (Decision)
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 2
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 3
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "dummy"}, // 4
	)
	
	windows2 := triage.CalculateWindows(ep2, cfg, triageCfg)
	if len(windows2) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows2))
	}
	if windows2[0].Start != 1 || windows2[0].End != 3 { // 1 to 1+2
		t.Errorf("expected window [1, 3], got [%d, %d]", windows2[0].Start, windows2[0].End)
	}
}
