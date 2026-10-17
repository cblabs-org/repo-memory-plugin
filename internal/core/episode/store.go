/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/episode/store.go
 * Description: Defines the Episode and Header schemas and provides zstd-compressed,
 *              atomic JSONL persistence and retrieval for conversation episodes.
 *
 * Exported Types / Functions:
 * - DefaultCompactorVersion: Default schema version recorded for transcript compaction.
 * - DefaultPipelineVersion:  Default version recorded for the ingestion pipeline.
 * - HarnessAntigravity:      Canonical harness identifier for Antigravity sessions.
 * - HarnessClaudeCode:       Canonical harness identifier for Claude Code sessions.
 * - StartedByHuman:          Origin marker indicating an interactive human-initiated session.
 * - StartedByAutomation:     Origin marker indicating an automated background session.
 * - ErrEpisodeNotFound:      Sentinel error returned when a requested episode does not exist.
 * - ErrInvalidSessionID:     Sentinel error returned when a session ID contains path traversal characters.
 * - ErrNilEpisode:           Sentinel error returned when a nil Episode or Header is passed.
 * - ErrEmptySessionID:       Sentinel error returned when a session ID is empty.
 * - Step:                    Type alias to harness.Step for caller convenience.
 * - StepKind:                Type alias to harness.StepKind for caller convenience.
 * - StepGap:                 Inclusive [start, end] interval of unwritten step indices.
 *
 *   Own Public API:
 *   - Start():               Returns the inclusive start step index of the gap.
 *   - End():                 Returns the inclusive end step index of the gap.
 *
 *   fmt.Stringer:
 *   - String():              Formats the step gap as "[i]" or "[start-end]". [fmt.Stringer]
 *
 * - Stats:                   Raw and kept character footprint counts for an episode.
 * - Header:                  Metadata header line (line 1) of a compacted episode file.
 *
 *   Own Public API:
 *   - SetCheckpoint():       Records a named checkpoint section on the header.
 *   - Checkpoint():          Returns a recorded checkpoint section by name.
 *   - RecordGap():           Appends a [start, end] step gap interval.
 *   - RecordGaps():          Detects and records missing step index intervals from a step slice.
 *   - Gaps():                Returns recorded step gaps as typed StepGap values.
 *   - HasGaps():             Reports whether any step gaps are recorded.
 *   - TotalMissingSteps():   Returns the total count of missing steps across all gaps.
 *   - RecordCommit():        Appends a unique commit hash to Commits.
 *   - RecordCommits():       Appends multiple commit hashes to Commits, ignoring duplicates.
 *   - HasCommits():          Reports whether any commits are recorded on the header.
 *   - HasCommit():           Reports whether a specific commit hash is recorded on the header.
 *   - SetParentEpisode():    Sets or clears the parent episode ID pointer.
 *   - GetParentEpisode():    Returns the parent episode ID or empty string if unset.
 *
 *   json.Unmarshaler:
 *   - UnmarshalJSON():       Unmarshals Header JSON with flexible timestamp and commit formats. [json.Unmarshaler]
 *
 * - NewEpisodeID():          Generates a new ULID-based episode ID with the "ep-" prefix.
 * - NewEpisodeIDWithTime():  Generates a ULID-based episode ID seeded with a specific timestamp.
 * - ParseHeader():           Unmarshals a Header from JSON bytes.
 * - ParseHeaderLine():       Unmarshals a Header from a single JSON line string.
 * - DetectGaps():            Computes missing step index intervals across a slice of steps.
 * - Episode:                 Complete compacted conversation episode (Header plus Steps).
 *   - NewEpisode():          Constructs an Episode and populates initial step gaps.
 *   - AddStep():             Appends a step to the episode.
 *   - RecordGaps():          Detects and records missing step intervals from the episode's steps.
 *   - RecordCommit():        Appends a commit hash to the episode header.
 *   - RecordCommits():       Appends multiple commit hashes to the episode header.
 *   - HasCommits():          Reports whether any commits are recorded on the episode.
 *   - HasCommit():           Reports whether a specific commit hash is recorded on the episode.
 *   - ComputeKeptChars():    Sums character counts across all retained episode steps.
 *   - UpdateStats():         Updates raw and kept character counts on the episode header.
 * - Store:                   Manages zstd-compressed episode files on disk by year and session ID.
 *   - NewStore():            Constructs a Store for a data directory and repository ID.
 *   - DefaultStore():        Constructs a Store using the platform default data directory.
 *   - DataDir():             Returns the root data directory path.
 *   - RepoID():              Returns the repository ID bound to the store.
 *   - EpisodesDir():         Returns the base episodes directory path for the repository.
 *   - YearDir():             Returns the year-partitioned episodes subdirectory path.
 *   - Path():                Returns the file path for a session ID and year.
 *   - EpisodePath():         Returns the canonical file path for an Episode.
 *   - Write():               Atomically writes a zstd-compressed episode to disk.
 *   - WriteEpisode():        Atomically writes an episode and returns its destination file path.
 *   - Read():                Reads and decompresses an episode by session ID (and optional year).
 *   - ReadHeader():          Reads only the first-line Header for a session ID without decoding steps.
 *   - ReadFromPath():        Reads and decompresses a full Episode from an explicit path.
 *   - ReadHeaderFromPath():  Reads only the Header from an explicit episode file path.
 *   - FindPath():            Locates an episode file across year subdirectories by session ID.
 *   - Exists():              Reports whether an episode file exists for the given session ID.
 *   - List():                Returns sorted session IDs of all stored episodes.
 *   - ListPaths():           Returns sorted file paths of all stored episodes.
 *   - ListHeaders():         Returns Headers of all stored episodes in path order.
 *   - ListEpisodes():        Returns all stored Episodes in deterministic path order.
 */

package episode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/klauspost/compress/zstd"
	"github.com/oklog/ulid/v2"
)

// Default version and harness constants.
const (
	DefaultCompactorVersion = "1.0.0"       // Default version string for the transcript compactor.
	DefaultPipelineVersion  = "0.1.0"       // Default version string for the ingestion pipeline.
	HarnessAntigravity      = "antigravity" // Identifier for the Antigravity IDE harness.
	HarnessClaudeCode       = "claude-code" // Identifier for the Claude Code CLI harness.
	StartedByHuman          = "human"       // Indicates the session was initiated interactively by a user.
	StartedByAutomation     = "automation"  // Indicates the session was initiated by an automated runner.
)

// Common episode store errors.
var (
	ErrEpisodeNotFound  = errors.New("episode not found")         // Returned when no episode file matches the session ID or path.
	ErrInvalidSessionID = errors.New("invalid session id")        // Returned when a session ID contains path separators or "..".
	ErrNilEpisode       = errors.New("episode cannot be nil")     // Returned when a nil Episode or Header pointer is supplied.
	ErrEmptySessionID   = errors.New("session_id cannot be empty") // Returned when a required session ID is empty.
)

// Step is an alias to harness.Step for caller convenience.
type Step = harness.Step

// StepKind is an alias to harness.StepKind for caller convenience.
type StepKind = harness.StepKind

// StepGap represents an unwritten internal step gap [start, end] inclusive.
type StepGap [2]int

// Start returns the starting step index of the gap.
func (g StepGap) Start() int { return g[0] }

// End returns the ending step index of the gap.
func (g StepGap) End() int { return g[1] }

// String returns a human-readable representation of the step gap.
func (g StepGap) String() string {
	if g[0] == g[1] {
		return fmt.Sprintf("[%d]", g[0])
	}
	return fmt.Sprintf("[%d-%d]", g[0], g[1])
}

// Stats captures character footprint statistics for the episode.
type Stats struct {
	RawChars  int `json:"raw_chars"`  // Total character count in the uncompacted raw transcript.
	KeptChars int `json:"kept_chars"` // Total character count retained across compacted steps.
}

// Header represents the metadata header line of an episode file (§5.1).
type Header struct {
	EpisodeID        string            `json:"episode_id"`                   // Unique ULID-based identifier (or session ID fallback) for the episode.
	Harness          string            `json:"harness"`                      // Source agent harness name (e.g., "antigravity" or "claude-code").
	SessionID        string            `json:"session_id"`                   // Harness conversation or session identifier.
	ParentEpisode    *string           `json:"parent_episode"`               // Optional parent episode ID for subagent trajectories.
	RepoID           string            `json:"repo_id"`                      // Stable registry slug derived from remote origin URL or root path hash.
	Branch           string            `json:"branch,omitempty"`             // Active git branch during the session.
	Started          time.Time         `json:"started,omitempty"`            // UTC timestamp of the first step in the episode.
	Ended            time.Time         `json:"ended,omitempty"`              // UTC timestamp of the final step in the episode.
	HeadStart        string            `json:"head_start,omitempty"`         // Git HEAD commit SHA at session start.
	HeadEnd          string            `json:"head_end,omitempty"`           // Git HEAD commit SHA at session end.
	Commits          []string          `json:"commits,omitempty"`            // Chronological commit SHAs linked to the episode.
	StepGaps         [][2]int          `json:"step_gaps,omitempty"`          // Inclusive [start, end] intervals of missing step indices.
	Model            string            `json:"model,omitempty"`              // Primary LLM model identifier used during the session.
	HarnessVersion   string            `json:"harness_version,omitempty"`    // Version string of the originating agent harness.
	StartedBy        string            `json:"started_by,omitempty"`         // Session initiator category ("human" or "automation").
	CompactorVersion string            `json:"compactor_version,omitempty"`  // Version of the transcript compactor that produced the episode.
	PipelineVersion  string            `json:"pipeline_version,omitempty"`   // Version of the ingestion pipeline that processed the episode.
	Stats            Stats             `json:"stats,omitempty"`              // Character footprint statistics before and after compaction.
	Source           string            `json:"source,omitempty"`             // Original filesystem path of the source transcript.
	Status           string            `json:"status,omitempty"`             // Processing lifecycle status (e.g., "captured", "parsed").
	StepCount        int               `json:"step_count,omitempty"`         // Total count of steps recorded in the episode.
	Checkpoints      map[string]string `json:"checkpoints,omitempty"`        // Extracted checkpoint sections keyed by canonical title.
	Exposures        []string          `json:"exposures,omitempty"`          // Memory entry IDs exposed to the agent during the session.
	Metadata         map[string]any    `json:"metadata,omitempty"`           // Auxiliary key-value metadata for flat JSON and summary adapters.
}

// SetCheckpoint records a parsed checkpoint section on the header.
func (h *Header) SetCheckpoint(section, content string) {
	if h.Checkpoints == nil {
		h.Checkpoints = make(map[string]string)
	}
	h.Checkpoints[section] = content
}

// Checkpoint returns a parsed checkpoint section by name.
func (h *Header) Checkpoint(section string) string {
	if h.Checkpoints == nil {
		return ""
	}
	return h.Checkpoints[section]
}

// NewEpisodeID creates a new ULID-based episode ID with the "ep-" prefix.
func NewEpisodeID() string {
	return "ep-" + ulid.Make().String()
}

// NewEpisodeIDWithTime creates a new ULID-based episode ID using the specified timestamp.
func NewEpisodeIDWithTime(t time.Time) string {
	return "ep-" + ulid.MustNew(ulid.Timestamp(t), ulid.DefaultEntropy()).String()
}

// RecordGap records a step gap interval [start, end] into the header.
func (h *Header) RecordGap(start, end int) {
	if start <= end {
		h.StepGaps = append(h.StepGaps, [2]int{start, end})
	}
}

// RecordGaps inspects steps, detects missing step index intervals, and appends them to StepGaps.
func (h *Header) RecordGaps(steps []harness.Step) {
	// --- Step 1: Detect contiguous missing step intervals ---
	gaps := DetectGaps(steps)
	if len(gaps) == 0 {
		return
	}

	// --- Step 2: Deduplicate against existing step gaps and append new intervals ---
	existing := make(map[[2]int]bool, len(h.StepGaps))
	for _, g := range h.StepGaps {
		existing[g] = true
	}
	for _, g := range gaps {
		if !existing[g] {
			h.StepGaps = append(h.StepGaps, g)
			existing[g] = true
		}
	}
}

// Gaps returns the header's step gaps as typed StepGap values.
func (h *Header) Gaps() []StepGap {
	res := make([]StepGap, len(h.StepGaps))
	for i, g := range h.StepGaps {
		res[i] = StepGap(g)
	}
	return res
}

// HasGaps returns true if any step gaps are recorded.
func (h *Header) HasGaps() bool {
	return len(h.StepGaps) > 0
}

// TotalMissingSteps returns the total count of missing step indices across all gaps.
func (h *Header) TotalMissingSteps() int {
	total := 0
	for _, g := range h.StepGaps {
		if g[1] >= g[0] {
			total += (g[1] - g[0] + 1)
		}
	}
	return total
}

// RecordCommit appends a commit hash to the header's Commits list if not already present.
func (h *Header) RecordCommit(sha string) {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return
	}
	for _, c := range h.Commits {
		if strings.EqualFold(c, sha) {
			return
		}
	}
	h.Commits = append(h.Commits, sha)
}

// RecordCommits appends multiple commit hashes to the header's Commits list, ignoring duplicates.
func (h *Header) RecordCommits(commits []string) {
	for _, c := range commits {
		h.RecordCommit(c)
	}
}

// HasCommits returns true if any commits are recorded.
func (h *Header) HasCommits() bool {
	return len(h.Commits) > 0
}

// HasCommit returns true if the specific commit hash is recorded in the header.
func (h *Header) HasCommit(sha string) bool {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return false
	}
	for _, c := range h.Commits {
		if strings.EqualFold(c, sha) {
			return true
		}
	}
	return false
}

// SetParentEpisode sets or clears the parent episode ID.
func (h *Header) SetParentEpisode(id string) {
	if id == "" {
		h.ParentEpisode = nil
	} else {
		h.ParentEpisode = &id
	}
}

// GetParentEpisode returns the parent episode ID or empty string if none.
func (h *Header) GetParentEpisode() string {
	if h.ParentEpisode == nil {
		return ""
	}
	return *h.ParentEpisode
}

// UnmarshalJSON implements custom JSON unmarshaling for Header to handle flexible timestamp formats
// and flexible commit formats.
func (h *Header) UnmarshalJSON(data []byte) error {
	// --- Step 1: Unmarshal standard fields and capture flexible raw fields ---
	type Alias Header
	aux := &struct {
		Started any `json:"started"` // Raw started timestamp value (string or numeric epoch).
		Ended   any `json:"ended"`   // Raw ended timestamp value (string or numeric epoch).
		Commits any `json:"commits"` // Raw commits payload (string, string slice, or object slice).
		*Alias                   // Embedded alias to populate all remaining Header fields.
	}{
		Alias: (*Alias)(h),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// --- Step 2: Parse flexible start and end timestamps ---
	if aux.Started != nil {
		t, err := parseTimeFlexible(aux.Started)
		if err != nil {
			return fmt.Errorf("parsing started timestamp: %w", err)
		}
		h.Started = t
	}

	if aux.Ended != nil {
		t, err := parseTimeFlexible(aux.Ended)
		if err != nil {
			return fmt.Errorf("parsing ended timestamp: %w", err)
		}
		h.Ended = t
	}

	// --- Step 3: Parse flexible commit list representations ---
	if aux.Commits != nil {
		commits, err := parseCommitsFlexible(aux.Commits)
		if err != nil {
			return fmt.Errorf("parsing commits: %w", err)
		}
		h.Commits = commits
	}

	return nil
}

// parseTimeFlexible converts string timestamps or numeric Unix seconds/milliseconds into UTC time.Time.
func parseTimeFlexible(val any) (time.Time, error) {
	// --- Step 1: Handle nil and string timestamp representations ---
	if val == nil {
		return time.Time{}, nil
	}
	switch v := val.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "null" {
			return time.Time{}, nil
		}
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t.UTC(), nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC(), nil
		}
		for _, layout := range []string{
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.UTC(), nil
			}
		}
		return time.Time{}, fmt.Errorf("unrecognized time format %q", v)

	// --- Step 2: Handle numeric Unix epoch seconds or milliseconds ---
	case float64:
		if v == 0 {
			return time.Time{}, nil
		}
		if v > 1e11 {
			return time.UnixMilli(int64(v)).UTC(), nil
		}
		return time.Unix(int64(v), 0).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unexpected time type %T", val)
	}
}

// parseCommitsFlexible normalizes commit lists represented as a delimited string,
// a JSON array of strings/objects, or a Go string slice.
func parseCommitsFlexible(val any) ([]string, error) {
	// --- Step 1: Handle nil or comma/whitespace-delimited string formats ---
	if val == nil {
		return nil, nil
	}
	switch v := val.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "null" {
			return nil, nil
		}
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t' || r == '\n'
		})
		var result []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result, nil

	// --- Step 2: Handle heterogeneous JSON arrays of strings or commit objects ---
	case []any:
		var result []string
		for _, item := range v {
			switch it := item.(type) {
			case string:
				it = strings.TrimSpace(it)
				if it != "" {
					result = append(result, it)
				}
			case map[string]any:
				for _, key := range []string{"sha", "commit", "hash", "ref"} {
					if s, ok := it[key].(string); ok {
						s = strings.TrimSpace(s)
						if s != "" {
							result = append(result, s)
							break
						}
					}
				}
			}
		}
		return result, nil

	// --- Step 3: Handle pre-typed string slices ---
	case []string:
		var result []string
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				result = append(result, s)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unexpected commits type %T", val)
	}
}

// ParseHeader parses an episode header from JSON bytes.
func ParseHeader(data []byte) (*Header, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("empty header data")
	}
	var header Header
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("parsing episode header: %w", err)
	}
	return &header, nil
}

// ParseHeaderLine parses an episode header from a single line string.
func ParseHeaderLine(line string) (*Header, error) {
	return ParseHeader([]byte(line))
}

// DetectGaps inspects steps and returns any missing step index intervals.
func DetectGaps(steps []harness.Step) [][2]int {
	// --- Step 1: Extract and sort step indices ---
	if len(steps) <= 1 {
		return nil
	}
	indices := make([]int, len(steps))
	for i, s := range steps {
		indices[i] = s.Index
	}
	sort.Ints(indices)

	// --- Step 2: Identify non-contiguous index jumps ---
	var gaps [][2]int
	for i := 1; i < len(indices); i++ {
		prev := indices[i-1]
		curr := indices[i]
		if curr > prev+1 {
			gaps = append(gaps, [2]int{prev + 1, curr - 1})
		}
	}
	return gaps
}

// Episode represents a complete compacted conversation episode.
type Episode struct {
	Header Header         `json:"header"` // First-line JSONL metadata record containing provenance, git commits, and step gaps.
	Steps  []harness.Step `json:"steps"`  // Compacted chronological sequence of steps.
}

// NewEpisode constructs an episode and automatically detects step gaps.
func NewEpisode(header Header, steps ...harness.Step) *Episode {
	ep := &Episode{
		Header: header,
		Steps:  steps,
	}
	if len(ep.Header.StepGaps) == 0 && len(steps) > 1 {
		ep.Header.RecordGaps(steps)
	}
	return ep
}

// AddStep appends a step to the episode.
func (e *Episode) AddStep(step harness.Step) {
	e.Steps = append(e.Steps, step)
}

// RecordGaps detects and populates step gaps from the episode's steps.
func (e *Episode) RecordGaps() {
	e.Header.RecordGaps(e.Steps)
}

// RecordCommit appends a commit hash to the episode header.
func (e *Episode) RecordCommit(sha string) {
	e.Header.RecordCommit(sha)
}

// RecordCommits appends multiple commit hashes to the episode header.
func (e *Episode) RecordCommits(commits []string) {
	e.Header.RecordCommits(commits)
}

// HasCommits returns true if any commits are recorded in the episode header.
func (e *Episode) HasCommits() bool {
	return e.Header.HasCommits()
}

// HasCommit returns true if the specific commit hash is recorded in the episode header.
func (e *Episode) HasCommit(sha string) bool {
	return e.Header.HasCommit(sha)
}

// ComputeKeptChars calculates the total kept character count for episode steps.
func (e *Episode) ComputeKeptChars() int {
	total := 0
	for _, s := range e.Steps {
		total += len(s.Text) + len(s.Excerpt) + len(s.Command) + len(s.Outcome)
	}
	return total
}

// UpdateStats updates the episode's raw and kept character footprint statistics.
func (e *Episode) UpdateStats(rawChars int) {
	e.Header.Stats.RawChars = rawChars
	e.Header.Stats.KeptChars = e.ComputeKeptChars()
}

// Store handles persistence and retrieval of compacted episodes at:
// <data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst
type Store struct {
	dataDir string // Base data directory containing per-repository subdirectories.
	repoID  string // Target repository slug under <data-dir>/<repo-id>/episodes/.
}

// NewStore creates a new Episode Store. If dataDir is empty, the platform user data directory is used.
func NewStore(dataDir, repoID string) *Store {
	if dataDir == "" {
		dataDir = config.DataDir()
	}
	return &Store{
		dataDir: dataDir,
		repoID:  repoID,
	}
}

// DefaultStore returns an Episode Store using the default platform data directory.
func DefaultStore(repoID string) *Store {
	return NewStore(config.DataDir(), repoID)
}

// DataDir returns the root data directory path.
func (s *Store) DataDir() string {
	return s.dataDir
}

// RepoID returns the repo ID for this store.
func (s *Store) RepoID() string {
	return s.repoID
}

// EpisodesDir returns <data-dir>/<repo-id>/episodes.
func (s *Store) EpisodesDir() string {
	return filepath.Join(s.dataDir, s.repoID, "episodes")
}

// YearDir returns <data-dir>/<repo-id>/episodes/<yyyy>.
func (s *Store) YearDir(year int) string {
	return filepath.Join(s.EpisodesDir(), fmt.Sprintf("%04d", year))
}

// Path returns the filesystem path for a compressed episode archive given its sessionID and four-digit year.
func (s *Store) Path(sessionID string, year int) string {
	cleanID := filepath.Base(sessionID)
	return filepath.Join(s.YearDir(year), cleanID+".jsonl.zst")
}

// EpisodePath computes the canonical storage path for the given episode based on its start year.
func (s *Store) EpisodePath(ep *Episode) string {
	year := ep.Header.Started.Year()
	if ep.Header.Started.IsZero() {
		if !ep.Header.Ended.IsZero() {
			year = ep.Header.Ended.Year()
		} else {
			year = time.Now().UTC().Year()
		}
	}
	return s.Path(ep.Header.SessionID, year)
}

// Write writes an episode to disk, compressed with zstd, using atomic rename.
func (s *Store) Write(ep *Episode) error {
	_, err := s.WriteEpisode(ep)
	return err
}

// WriteEpisode writes an episode and returns its final destination path.
func (s *Store) WriteEpisode(ep *Episode) (string, error) {
	// --- Step 1: Validate episode and backfill default header metadata ---
	if ep == nil {
		return "", ErrNilEpisode
	}
	if ep.Header.SessionID == "" {
		return "", ErrEmptySessionID
	}
	if err := validateSessionID(ep.Header.SessionID); err != nil {
		return "", err
	}

	if ep.Header.EpisodeID == "" {
		ep.Header.EpisodeID = NewEpisodeID()
	}
	if ep.Header.RepoID == "" && s.repoID != "" {
		ep.Header.RepoID = s.repoID
	}
	if ep.Header.PipelineVersion == "" {
		ep.Header.PipelineVersion = DefaultPipelineVersion
	}
	if ep.Header.CompactorVersion == "" {
		ep.Header.CompactorVersion = DefaultCompactorVersion
	}
	if len(ep.Header.StepGaps) == 0 && len(ep.Steps) > 1 {
		ep.Header.RecordGaps(ep.Steps)
	}

	// --- Step 2: Create destination directory and temporary sibling file ---
	targetPath := s.EpisodePath(ep)
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, fmt.Sprintf(".%s-*.tmp", ep.Header.SessionID))
	if err != nil {
		return "", fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmpFile.Name()
	success := false
	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpName)
		}
	}()

	// --- Step 3: Stream zstd-compressed header and step JSONL lines ---
	enc, err := zstd.NewWriter(tmpFile, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return "", fmt.Errorf("creating zstd encoder: %w", err)
	}

	// Line 1: Header line.
	headerBytes, err := json.Marshal(ep.Header)
	if err != nil {
		_ = enc.Close()
		return "", fmt.Errorf("marshaling episode header: %w", err)
	}
	if _, err := enc.Write(append(headerBytes, '\n')); err != nil {
		_ = enc.Close()
		return "", fmt.Errorf("writing episode header: %w", err)
	}

	// Subsequent lines: Step lines.
	for i, step := range ep.Steps {
		stepBytes, err := json.Marshal(step)
		if err != nil {
			_ = enc.Close()
			return "", fmt.Errorf("marshaling step %d: %w", i, err)
		}
		if _, err := enc.Write(append(stepBytes, '\n')); err != nil {
			_ = enc.Close()
			return "", fmt.Errorf("writing step %d: %w", i, err)
		}
	}

	// --- Step 4: Flush encoder, close file handle, and atomically rename ---
	if err := enc.Close(); err != nil {
		return "", fmt.Errorf("closing zstd encoder: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		return "", fmt.Errorf("committing episode file to %s: %w", targetPath, err)
	}

	success = true
	return targetPath, nil
}

// Read reads and decompresses an episode from disk by session ID.
// If year is omitted or 0, it searches across all year directories.
func (s *Store) Read(sessionID string, year ...int) (*Episode, error) {
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	if len(year) > 0 && year[0] > 0 {
		path := s.Path(sessionID, year[0])
		return s.ReadFromPath(path)
	}
	path, err := s.FindPath(sessionID)
	if err != nil {
		return nil, err
	}
	return s.ReadFromPath(path)
}

// ReadHeader reads only the metadata header from an episode file without reading all steps.
func (s *Store) ReadHeader(sessionID string, year ...int) (*Header, error) {
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	if len(year) > 0 && year[0] > 0 {
		path := s.Path(sessionID, year[0])
		return s.ReadHeaderFromPath(path)
	}
	path, err := s.FindPath(sessionID)
	if err != nil {
		return nil, err
	}
	return s.ReadHeaderFromPath(path)
}

// ReadFromPath reads and decompresses an episode from an explicit file path.
func (s *Store) ReadFromPath(path string) (*Episode, error) {
	// --- Step 1: Open file and initialize zstd reader ---
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrEpisodeNotFound, path)
		}
		return nil, err
	}
	defer f.Close()

	dec, err := zstd.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("creating zstd reader for %s: %w", path, err)
	}
	defer dec.Close()

	reader := bufio.NewReader(dec)

	// --- Step 2: Read and unmarshal line 1 as Header ---
	headerBytes, err := readLineBytes(reader)
	if err != nil {
		return nil, fmt.Errorf("reading episode header from %s: %w", path, err)
	}
	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshaling episode header from %s: %w", path, err)
	}

	// --- Step 3: Read and unmarshal subsequent lines as Steps ---
	var steps []harness.Step
	lineNum := 1
	for {
		stepBytes, err := readLineBytes(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("reading step at line %d from %s: %w", lineNum+1, path, err)
		}
		lineNum++
		if len(stepBytes) == 0 {
			continue
		}
		var step harness.Step
		if err := json.Unmarshal(stepBytes, &step); err != nil {
			return nil, fmt.Errorf("unmarshaling step at line %d from %s: %w", lineNum, path, err)
		}
		steps = append(steps, step)
	}

	return &Episode{
		Header: header,
		Steps:  steps,
	}, nil
}

// ReadHeaderFromPath reads only the Header from a given episode file.
func (s *Store) ReadHeaderFromPath(path string) (*Header, error) {
	// --- Step 1: Open compressed episode file and attach zstd decoder ---
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrEpisodeNotFound, path)
		}
		return nil, err
	}
	defer f.Close()

	dec, err := zstd.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("creating zstd reader for %s: %w", path, err)
	}
	defer dec.Close()

	// --- Step 2: Read and decode only the first JSONL line ---
	reader := bufio.NewReader(dec)
	headerBytes, err := readLineBytes(reader)
	if err != nil {
		return nil, fmt.Errorf("reading episode header from %s: %w", path, err)
	}
	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshaling episode header from %s: %w", path, err)
	}
	return &header, nil
}

// FindPath locates an episode file by session ID across year subdirectories.
func (s *Store) FindPath(sessionID string) (string, error) {
	// --- Step 1: Validate session ID and read year subdirectories ---
	if err := validateSessionID(sessionID); err != nil {
		return "", err
	}
	baseDir := s.EpisodesDir()
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrEpisodeNotFound, sessionID)
		}
		return "", err
	}

	// --- Step 2: Probe each year directory for a matching <session-id>.jsonl.zst file ---
	filename := sessionID + ".jsonl.zst"
	var found []string

	for _, entry := range entries {
		if entry.IsDir() {
			candidate := filepath.Join(baseDir, entry.Name(), filename)
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				found = append(found, candidate)
			}
		}
	}

	if len(found) == 0 {
		return "", fmt.Errorf("%w: %s", ErrEpisodeNotFound, sessionID)
	}

	// --- Step 3: Prefer the latest year directory if multiple matches exist ---
	if len(found) > 1 {
		sort.Sort(sort.Reverse(sort.StringSlice(found)))
	}

	return found[0], nil
}

// Exists checks if an episode exists for the given session ID.
func (s *Store) Exists(sessionID string, year ...int) bool {
	if err := validateSessionID(sessionID); err != nil {
		return false
	}
	if len(year) > 0 && year[0] > 0 {
		path := s.Path(sessionID, year[0])
		fi, err := os.Stat(path)
		return err == nil && !fi.IsDir()
	}
	_, err := s.FindPath(sessionID)
	return err == nil
}

// List returns session IDs of all stored episodes, sorted.
func (s *Store) List(year ...int) ([]string, error) {
	paths, err := s.ListPaths(year...)
	if err != nil {
		return nil, err
	}
	sessions := make([]string, 0, len(paths))
	for _, p := range paths {
		base := filepath.Base(p)
		sessionID := strings.TrimSuffix(base, ".jsonl.zst")
		sessions = append(sessions, sessionID)
	}
	return sessions, nil
}

// ListPaths returns file paths to all stored episodes, sorted.
func (s *Store) ListPaths(year ...int) ([]string, error) {
	var paths []string

	// --- Step 1: Collect episode paths from a specific year directory or all years ---
	if len(year) > 0 && year[0] > 0 {
		yearDir := s.YearDir(year[0])
		entries, err := os.ReadDir(yearDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl.zst") {
				paths = append(paths, filepath.Join(yearDir, e.Name()))
			}
		}
	} else {
		baseDir := s.EpisodesDir()
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				yd := filepath.Join(baseDir, entry.Name())
				files, err := os.ReadDir(yd)
				if err != nil {
					continue
				}
				for _, f := range files {
					if !f.IsDir() && strings.HasSuffix(f.Name(), ".jsonl.zst") {
						paths = append(paths, filepath.Join(yd, f.Name()))
					}
				}
			}
		}
	}

	// --- Step 2: Sort collected paths deterministically ---
	sort.Strings(paths)
	return paths, nil
}

// ListHeaders returns the Header of all stored episodes, sorted by path.
func (s *Store) ListHeaders(year ...int) ([]*Header, error) {
	// --- Step 1: Enumerate sorted episode file paths ---
	paths, err := s.ListPaths(year...)
	if err != nil {
		return nil, err
	}

	// --- Step 2: Read the header line from each episode file ---
	headers := make([]*Header, 0, len(paths))
	for _, p := range paths {
		h, err := s.ReadHeaderFromPath(p)
		if err != nil {
			return nil, fmt.Errorf("reading header from %s: %w", p, err)
		}
		headers = append(headers, h)
	}
	return headers, nil
}

// ListEpisodes returns all stored episodes in deterministic path order.
func (s *Store) ListEpisodes(year ...int) ([]*Episode, error) {
	// --- Step 1: Enumerate sorted episode file paths ---
	paths, err := s.ListPaths(year...)
	if err != nil {
		return nil, err
	}

	// --- Step 2: Decompress and decode each full episode ---
	episodes := make([]*Episode, 0, len(paths))
	for _, p := range paths {
		ep, err := s.ReadFromPath(p)
		if err != nil {
			return nil, fmt.Errorf("reading episode from %s: %w", p, err)
		}
		episodes = append(episodes, ep)
	}
	return episodes, nil
}

// readLineBytes reads a single newline-delimited line from r without arbitrary length limits.
func readLineBytes(r *bufio.Reader) ([]byte, error) {
	// --- Step 1: Accumulate chunks until a newline delimiter or EOF is reached ---
	var line []byte
	for {
		chunk, err := r.ReadBytes('\n')
		line = append(line, chunk...)
		if err != nil {
			// --- Step 2: Return any trailing content before propagating io.EOF ---
			if errors.Is(err, io.EOF) {
				trimmed := bytes.TrimSpace(line)
				if len(trimmed) == 0 {
					return nil, io.EOF
				}
				return trimmed, nil
			}
			return nil, err
		}
		if len(chunk) > 0 && chunk[len(chunk)-1] == '\n' {
			return bytes.TrimSpace(line), nil
		}
	}
}

// validateSessionID ensures sessionID is non-empty and free of path separators or traversal segments.
func validateSessionID(sessionID string) error {
	if sessionID == "" {
		return ErrEmptySessionID
	}
	if strings.ContainsAny(sessionID, "/\\") || sessionID == "." || sessionID == ".." || strings.Contains(sessionID, "..") {
		return fmt.Errorf("%w: %q", ErrInvalidSessionID, sessionID)
	}
	return nil
}
