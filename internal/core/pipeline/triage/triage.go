package triage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// FallbackInput represents the input to the small-model fallback prompt.
type FallbackInput struct {
	Episodes []EpisodeSynopsis `json:"episodes"`
}

// EpisodeSynopsis is the data passed to the model for a single episode.
type EpisodeSynopsis struct {
	ID       string   `json:"id"`
	Synopsis Synopsis `json:"synopsis"`
}

// FallbackDecision represents the model's decision for a single episode.
type FallbackDecision struct {
	ID     string   `json:"id"`
	Keep   bool     `json:"keep"`
	Reason string   `json:"reason"`
}

// FallbackOutput represents the expected JSON output from the small-model fallback.
type FallbackOutput struct {
	Decisions []FallbackDecision `json:"decisions"`
}

// Triager orchestrates the triage stage.
type Triager struct {
	Housekeeper harness.Housekeeper
	Config      Config
	WinConfig   WindowsConfig
}

// NewTriager creates a new Triager.
func NewTriager(hk harness.Housekeeper, cfg Config, winCfg WindowsConfig) *Triager {
	return &Triager{
		Housekeeper: hk,
		Config:      cfg,
		WinConfig:   winCfg,
	}
}

// TriageResult holds the outcome of triage for an episode.
type TriageResult struct {
	Keep     bool
	Reasons  []string
	Windows  []Window
	Synopsis Synopsis
}

// Process runs the triage stage on a set of episodes.
func (t *Triager) Process(ctx context.Context, episodes []*episode.Episode) (map[string]TriageResult, error) {
	results := make(map[string]TriageResult)
	var ambiguous []*episode.Episode
	
	// 1. Rule-based triage
	for _, ep := range episodes {
		keep, skip, reasons := EvaluateRules(ep, t.Config)
		
		synopsis := GenerateSynopsis(ep, t.Config.Subsystems)
		windows := CalculateWindows(ep, t.WinConfig, t.Config)

		if keep {
			results[ep.Header.EpisodeID] = TriageResult{
				Keep:     true,
				Reasons:  reasons,
				Windows:  windows,
				Synopsis: synopsis,
			}
		} else if skip {
			results[ep.Header.EpisodeID] = TriageResult{
				Keep:     false,
				Reasons:  reasons,
				Synopsis: synopsis,
			}
		} else {
			// Ambiguous, defer to LLM
			ambiguous = append(ambiguous, ep)
		}
	}

	// 2. Small-model fallback for ambiguous episodes
	if len(ambiguous) > 0 {
		// Batch up to 20 episodes as per SPEC
		const batchSize = 20
		for i := 0; i < len(ambiguous); i += batchSize {
			end := i + batchSize
			if end > len(ambiguous) {
				end = len(ambiguous)
			}
			batch := ambiguous[i:end]
			
			input := FallbackInput{}
			for _, ep := range batch {
				input.Episodes = append(input.Episodes, EpisodeSynopsis{
					ID:       ep.Header.EpisodeID,
					Synopsis: GenerateSynopsis(ep, t.Config.Subsystems),
				})
			}
			
			res, err := t.Housekeeper.Run(ctx, harness.StageTriage, input)
			if err != nil {
				return nil, fmt.Errorf("fallback model error: %w", err)
			}
			
			var output FallbackOutput
			if err := json.Unmarshal([]byte(res.Payload), &output); err != nil {
				// If unmarshal fails, we might just skip them to be safe, or log it
				return nil, fmt.Errorf("failed to parse fallback output: %w", err)
			}
			
			for _, dec := range output.Decisions {
				// Find the matching episode
				for _, ep := range batch {
					if ep.Header.EpisodeID == dec.ID {
						windows := CalculateWindows(ep, t.WinConfig, t.Config)
						synopsis := GenerateSynopsis(ep, t.Config.Subsystems)
						results[ep.Header.EpisodeID] = TriageResult{
							Keep:     dec.Keep,
							Reasons:  []string{"fallback model: " + dec.Reason},
							Windows:  windows,
							Synopsis: synopsis,
						}
						break
					}
				}
			}
		}
	}

	return results, nil
}
