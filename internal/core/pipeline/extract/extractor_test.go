package extract_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/extract"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

func TestExtractor(t *testing.T) {
	hk := fake.NewFakeHousekeeper()
	
	// Prepare a dummy extraction output.
	out := extract.ExtractionResult{
		Candidates: []extract.Candidate{
			{
				EpisodeID: "ep-1",
				Type: "fact",
				Statement: "Testing candidate parsing",
				Evidence: []claims.Evidence{
					{Kind: "tool_outcome", Ref: "ls", Outcome: "success"},
				},
			},
			{
				EpisodeID: "ep-1",
				Type: "fact",
				Statement: "Agent reasoning (should be filtered or marked)",
				Evidence: []claims.Evidence{
					{Kind: "agent", Ref: "I think this is right"},
				},
			},
		},
		Gaps: []extract.Gap{
			{Description: "Missing info about board"},
		},
	}
	payload, _ := json.Marshal(out)
	hk.SetResult(harness.StageExtract, harness.Result{Payload: string(payload)})
	
	ext := extract.NewExtractor(hk, 60000)
	
	ep := episode.NewEpisode(episode.Header{EpisodeID: "ep-1", SessionID: "s-1"}, harness.Step{Kind: harness.StepKindUser, Text: "hi"})
	triageResults := map[string]triage.TriageResult{
		"ep-1": {Keep: true},
	}
	
	res, err := ext.Extract(context.Background(), []*episode.Episode{ep}, triageResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	
	// P13 check: candidate with 'agent' evidence should be filtered or fixed
	// Currently we expect 1 candidate to be valid, or 2 candidates but with evidence filtered.
	if len(res[0].Candidates) != 2 {
		t.Errorf("expected 2 candidates")
	}
	
	c1 := res[0].Candidates[0]
	if len(c1.Evidence) != 1 {
		t.Errorf("candidate 1 should have 1 evidence, got %d", len(c1.Evidence))
	}
	
	c2 := res[0].Candidates[1]
	if len(c2.Evidence) != 0 {
		t.Errorf("candidate 2 should have 0 evidence due to P13, got %d", len(c2.Evidence))
	}
	
	if len(res[0].Gaps) != 1 {
		t.Errorf("expected 1 gap")
	}
}

func TestPacker(t *testing.T) {
	p := extract.NewPacker(10) // tiny token cap
	
	ep1 := episode.NewEpisode(episode.Header{EpisodeID: "ep-1", SessionID: "s-1"}, harness.Step{Kind: harness.StepKindUser, Text: "hello world, this is a long text to exceed the tiny cap"})
	ep2 := episode.NewEpisode(episode.Header{EpisodeID: "ep-2", SessionID: "s-1", ParentEpisode: ptr("ep-1")}, harness.Step{Kind: harness.StepKindUser, Text: "subagent"})
	
	triageResults := map[string]triage.TriageResult{
		"ep-1": {Keep: true, Windows: []triage.Window{{Start:0, End:0}}},
		"ep-2": {Keep: true, Windows: []triage.Window{{Start:0, End:0}}},
	}
	
	batches := p.Pack([]*episode.Episode{ep2, ep1}, triageResults)
	
	// ep1 and ep2 should be grouped by SessionID, sorted so ep1 (parent) is first.
	// But because token cap is 10, they should be split into 2 batches.
	if len(batches) < 2 {
		t.Errorf("expected multiple batches, got %d", len(batches))
	}
	
	if len(batches[0]) > 0 && batches[0][0].ID != "ep-1" {
		t.Errorf("expected ep-1 to be first in the first batch")
	}
}

func ptr(s string) *string {
	return &s
}
