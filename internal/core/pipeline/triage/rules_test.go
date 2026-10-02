package triage_test

import (
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestEvaluateRules(t *testing.T) {
	cfg := triage.DefaultConfig()

	// 1. Skip all condition
	ep1 := episode.NewEpisode(episode.Header{EpisodeID: "ep1"},
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "ls"},
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "echo hi"},
	)
	keep, skip, _ := triage.EvaluateRules(ep1, cfg)
	if keep || !skip {
		t.Errorf("ep1 should be skipped")
	}

	// 2. Keep: failure followed by success
	ep2 := episode.NewEpisode(episode.Header{EpisodeID: "ep2"},
		harness.Step{Kind: harness.StepKindCommand, Outcome: "failure", Command: "make build"},
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success", Command: "make build"},
	)
	keep, skip, _ = triage.EvaluateRules(ep2, cfg)
	if !keep || skip {
		t.Errorf("ep2 should be kept (failure followed by success)")
	}

	// 3. Keep: user decision signal
	ep3 := episode.NewEpisode(episode.Header{EpisodeID: "ep3"},
		harness.Step{Kind: harness.StepKindUser, Text: "I prefer to use JSON instead of YAML"},
	)
	keep, skip, _ = triage.EvaluateRules(ep3, cfg)
	if !keep || skip {
		t.Errorf("ep3 should be kept (user decision signal)")
	}
	
	// 4. Keep: memory artefact write
	ep4 := episode.NewEpisode(episode.Header{EpisodeID: "ep4"},
		harness.Step{Kind: harness.StepKindEdit, File: "PROJECT.md", Action: "edit"},
	)
	keep, skip, _ = triage.EvaluateRules(ep4, cfg)
	if !keep || skip {
		t.Errorf("ep4 should be kept (memory artefact write)")
	}
}
