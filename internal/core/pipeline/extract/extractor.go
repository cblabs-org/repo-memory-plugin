package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// Gap represents things the episode relied on that match neither a claim nor an entity.
type Gap struct {
	Description string `json:"description"`
}

// ExtractionResult is the struct expected from the LLM.
type ExtractionResult struct {
	Candidates []Candidate `json:"candidates"`
	Gaps       []Gap       `json:"gaps"`
}

// Candidate is a proposed claim.
type Candidate struct {
	EpisodeID    string           `json:"episode_id"`
	Type         claims.ClaimType `json:"type"`
	Statement    string           `json:"statement"`
	RelevantWhen string           `json:"relevant_when,omitempty"`
	Apply        string           `json:"apply,omitempty"`
	Anchors      []claims.Anchor  `json:"anchors,omitempty"`
	Conditions   []claims.Condition `json:"conditions,omitempty"`
	ClosesWhen   *claims.ClosesWhen `json:"closes_when,omitempty"`
	Evidence     []claims.Evidence `json:"evidence,omitempty"`
	ProposedStatus claims.ClaimStatus `json:"proposed_status"`
	Rationale    string           `json:"rationale"`
	EchoOf       string           `json:"echo_of,omitempty"`
	Match        *claims.Match    `json:"match,omitempty"`
	Provenance   claims.Provenance `json:"provenance,omitempty"`
}

// Extractor orchestrates the extraction stage.
type Extractor struct {
	Housekeeper harness.Housekeeper
	TokenCap    int
}

// NewExtractor creates a new Extractor.
func NewExtractor(hk harness.Housekeeper, tokenCap int) *Extractor {
	return &Extractor{
		Housekeeper: hk,
		TokenCap:    tokenCap,
	}
}

// Extract processes episodes and extracts candidates.
func (e *Extractor) Extract(ctx context.Context, episodes []*episode.Episode, triageResults map[string]triage.TriageResult) ([]ExtractionResult, error) {
	packer := NewPacker(e.TokenCap)
	batches := packer.Pack(episodes, triageResults)
	
	var allResults []ExtractionResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	for _, batch := range batches {
		wg.Add(1)
		go func(b []EpisodePayload) {
			defer wg.Done()
			
			input := ExtractInput{Episodes: b}
			
			res, err := e.Housekeeper.Run(ctx, harness.StageExtract, input)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("extraction error: %w", err)
				}
				mu.Unlock()
				return
			}
			
			var output ExtractionResult
			if err := json.Unmarshal([]byte(res.Payload), &output); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to parse extraction output: %w", err)
				}
				mu.Unlock()
				return
			}
			
			output = e.enforceRules(output, b)
			
			mu.Lock()
			allResults = append(allResults, output)
			mu.Unlock()
		}(batch)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return allResults, nil
}

func (e *Extractor) enforceRules(res ExtractionResult, batch []EpisodePayload) ExtractionResult {
	var validCandidates []Candidate
	
	for _, c := range res.Candidates {
		// P13: Check evidence validness
		var validEvidence []claims.Evidence
		for _, ev := range c.Evidence {
			if isValidEvidence(ev, batch) {
				validEvidence = append(validEvidence, ev)
			}
		}
		c.Evidence = validEvidence
		
		validCandidates = append(validCandidates, c)
	}
	
	// Max 20 candidates per episode
	counts := make(map[string]int)
	var limited []Candidate
	for _, c := range validCandidates {
		if counts[c.EpisodeID] < 20 {
			limited = append(limited, c)
			counts[c.EpisodeID]++
		}
	}
	
	res.Candidates = limited
	return res
}

func isValidEvidence(ev claims.Evidence, batch []EpisodePayload) bool {
	if ev.Kind != "human" && ev.Kind != "tool_outcome" {
		return false
	}
	return true
}
