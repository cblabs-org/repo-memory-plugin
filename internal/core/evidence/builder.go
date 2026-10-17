/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/evidence/builder.go
 * Description: Assembles a normalized episode.Episode from a spool capture
 *              entry and a parsed transcript stream.
 *
 * Exported Types / Functions:
 * - BuildEpisode(): Constructs an episode.Episode from a spool.Entry and ParsedTranscript.
 */

// Package evidence parses raw agent transcripts, checkpoint summaries, and
// conversation artifacts into structured episode evidence.
package evidence

import (
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

// BuildEpisode creates an episode.Episode from a spool entry and parsed transcript.
func BuildEpisode(entry spool.Entry, parsed *ParsedTranscript, repoID string) *episode.Episode {
	// --- Step 1: Resolve session start and end timestamps with fallbacks ---
	started := parsed.Started
	if started.IsZero() {
		if !entry.Timestamp.IsZero() {
			started = entry.Timestamp
		} else {
			started = time.Now().UTC()
		}
	}

	ended := parsed.Ended
	if ended.IsZero() {
		if !entry.Timestamp.IsZero() {
			ended = entry.Timestamp
		} else {
			ended = started
		}
	}

	// --- Step 2: Populate episode header metadata and checkpoint status ---
	harnessName := entry.Harness
	if harnessName == "" {
		harnessName = "antigravity"
	}

	status := "captured"
	if len(parsed.Checkpoints) > 0 {
		status = "parsed"
	}

	header := episode.Header{
		EpisodeID:        entry.ConversationID,
		Harness:          harnessName,
		SessionID:        entry.ConversationID,
		RepoID:           repoID,
		Branch:           entry.Branch,
		Started:          started,
		Ended:            ended,
		HeadStart:        entry.Head,
		HeadEnd:          entry.Head,
		Source:           entry.TranscriptPath,
		Status:           status,
		StepCount:        len(parsed.Steps),
		Checkpoints:      parsed.Checkpoints,
		PipelineVersion:  episode.DefaultPipelineVersion,
		CompactorVersion: episode.DefaultCompactorVersion,
	}

	if entry.ParentConversationID != "" {
		header.SetParentEpisode(entry.ParentConversationID)
	}

	// --- Step 3: Construct episode, record initial HEAD commit, and compute stats ---
	ep := episode.NewEpisode(header, parsed.Steps...)

	if entry.Head != "" {
		ep.RecordCommit(entry.Head)
	}

	ep.RecordGaps()
	ep.UpdateStats(int(parsed.RawChars))

	return ep
}
