package extract

import (
	"encoding/json"
	"sort"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// EpisodePayload contains the data for one episode to be extracted.
type EpisodePayload struct {
	ID       string          `json:"id"`
	SessionID string         `json:"session_id"`
	Parent   string          `json:"parent,omitempty"`
	Synopsis triage.Synopsis `json:"synopsis"`
	Windows  []triage.Window `json:"windows"`
	Entities []string        `json:"entities"`
	Steps    []harness.Step  `json:"steps"` // The actual steps within the windows
}

type ExtractInput struct {
	Instructions string           `json:"instructions"`
	Episodes     []EpisodePayload `json:"episodes"`
}

// Packer groups episodes up to a token limit.
type Packer struct {
	TokenCap int
}

// NewPacker creates a new Packer.
func NewPacker(tokenCap int) *Packer {
	return &Packer{TokenCap: tokenCap}
}

// Pack groups episodes based on the token cap.
func (p *Packer) Pack(episodes []*episode.Episode, triageResults map[string]triage.TriageResult) [][]EpisodePayload {
	// Filter to only kept episodes
	var kept []*episode.Episode
	for _, ep := range episodes {
		if res, ok := triageResults[ep.Header.EpisodeID]; ok && res.Keep {
			kept = append(kept, ep)
		}
	}

	// Group by SessionID
	sessions := make(map[string][]*episode.Episode)
	for _, ep := range kept {
		sessions[ep.Header.SessionID] = append(sessions[ep.Header.SessionID], ep)
	}

	// Ensure parent is first within each session
	for _, session := range sessions {
		sort.SliceStable(session, func(i, j int) bool {
			// Parent has nil ParentEpisode
			return session[i].Header.ParentEpisode == nil && session[j].Header.ParentEpisode != nil
		})
	}

	var payloads []EpisodePayload
	for _, session := range sessions {
		for _, ep := range session {
			res := triageResults[ep.Header.EpisodeID]
			
			// Filter steps to only those in windows
			var windowSteps []harness.Step
			for _, w := range res.Windows {
				if w.Start >= 0 && w.End < len(ep.Steps) {
					windowSteps = append(windowSteps, ep.Steps[w.Start:w.End+1]...)
				}
			}
			
			payloads = append(payloads, EpisodePayload{
				ID:        ep.Header.EpisodeID,
				SessionID: ep.Header.SessionID,
				Parent:    getParentStr(ep),
				Synopsis:  res.Synopsis,
				Windows:   res.Windows,
				Entities:  []string{}, // TODO: where to get entities?
				Steps:     windowSteps,
			})
		}
	}

	var batches [][]EpisodePayload
	var currentBatch []EpisodePayload
	var currentTokens int

	for _, payload := range payloads {
		// Estimate tokens
		b, _ := json.Marshal(payload)
		tokens := len(b) / 4
		
		if currentTokens+tokens > p.TokenCap && len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
			currentBatch = nil
			currentTokens = 0
		}
		
		currentBatch = append(currentBatch, payload)
		currentTokens += tokens
	}
	
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches
}

func getParentStr(ep *episode.Episode) string {
	if ep.Header.ParentEpisode != nil {
		return *ep.Header.ParentEpisode
	}
	return ""
}
