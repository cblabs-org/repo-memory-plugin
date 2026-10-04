package evidence

import (
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

// BuildEpisode creates an episode.Episode from a spool entry and parsed transcript.
func BuildEpisode(entry spool.Entry, parsed *ParsedTranscript, repoID string) *episode.Episode {
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

	ep := episode.NewEpisode(header, parsed.Steps...)

	if entry.Head != "" {
		ep.RecordCommit(entry.Head)
	}

	ep.RecordGaps()
	ep.UpdateStats(int(parsed.RawChars))

	return ep
}
