package triage_test

import (
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestGenerateSynopsis(t *testing.T) {
	ep := episode.NewEpisode(episode.Header{EpisodeID: "ep1", Commits: []string{"abc1234"}},
		harness.Step{Kind: harness.StepKindEdit, File: "src/main.go"},
		harness.Step{Kind: harness.StepKindCommand, Outcome: "success"},
		harness.Step{Kind: harness.StepKindUser, Text: "Looks good to me."},
		harness.Step{Kind: harness.StepKindExposure, Excerpt: "memory snippet"},
	)

	subsystems := []string{"src"}
	syn := triage.GenerateSynopsis(ep, subsystems)

	if len(syn.Commits) != 1 || syn.Commits[0] != "abc1234" {
		t.Errorf("expected commit abc1234")
	}
	if len(syn.FilesBySubsystem["src"]) != 1 || syn.FilesBySubsystem["src"][0] != "src/main.go" {
		t.Errorf("expected src/main.go in src subsystem")
	}
	if len(syn.Outcomes) != 1 || syn.Outcomes[0] != "success" {
		t.Errorf("expected outcome success")
	}
	if len(syn.HumanMessages) != 1 || syn.HumanMessages[0] != "Looks good to me." {
		t.Errorf("expected human message")
	}
	if len(syn.Exposures) != 1 || syn.Exposures[0] != "memory snippet" {
		t.Errorf("expected exposure")
	}
}
