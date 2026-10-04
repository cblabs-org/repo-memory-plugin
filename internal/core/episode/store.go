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

// Default version constants.
const (
	DefaultCompactorVersion = "1.0.0"
	DefaultPipelineVersion  = "0.1.0"

	HarnessAntigravity = "antigravity"
	HarnessClaudeCode  = "claude-code"

	StartedByHuman      = "human"
	StartedByAutomation = "automation"
)

// Common errors.
var (
	ErrEpisodeNotFound  = errors.New("episode not found")
	ErrInvalidSessionID = errors.New("invalid session id")
	ErrNilEpisode       = errors.New("episode cannot be nil")
	ErrEmptySessionID   = errors.New("session_id cannot be empty")
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
	RawChars  int `json:"raw_chars"`
	KeptChars int `json:"kept_chars"`
}

// Header represents the metadata header line of an episode file (§5.1).
type Header struct {
	EpisodeID        string            `json:"episode_id"`
	Harness          string            `json:"harness"`
	SessionID        string            `json:"session_id"`
	ParentEpisode    *string           `json:"parent_episode"`
	RepoID           string            `json:"repo_id"`
	Branch           string            `json:"branch,omitempty"`
	Started          time.Time         `json:"started,omitempty"`
	Ended            time.Time         `json:"ended,omitempty"`
	HeadStart        string            `json:"head_start,omitempty"`
	HeadEnd          string            `json:"head_end,omitempty"`
	Commits          []string          `json:"commits,omitempty"`
	StepGaps         [][2]int          `json:"step_gaps,omitempty"`
	Model            string            `json:"model,omitempty"`
	HarnessVersion   string            `json:"harness_version,omitempty"`
	StartedBy        string            `json:"started_by,omitempty"`
	CompactorVersion string            `json:"compactor_version,omitempty"`
	PipelineVersion  string            `json:"pipeline_version,omitempty"`
	Stats            Stats             `json:"stats,omitempty"`
	Source           string            `json:"source,omitempty"`
	Status           string            `json:"status,omitempty"`
	StepCount        int               `json:"step_count,omitempty"`
	Checkpoints      map[string]string `json:"checkpoints,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
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
	gaps := DetectGaps(steps)
	if len(gaps) == 0 {
		return
	}
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
	type Alias Header
	aux := &struct {
		Started any `json:"started"`
		Ended   any `json:"ended"`
		Commits any `json:"commits"`
		*Alias
	}{
		Alias: (*Alias)(h),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

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

	if aux.Commits != nil {
		commits, err := parseCommitsFlexible(aux.Commits)
		if err != nil {
			return fmt.Errorf("parsing commits: %w", err)
		}
		h.Commits = commits
	}

	return nil
}

func parseTimeFlexible(val any) (time.Time, error) {
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

func parseCommitsFlexible(val any) ([]string, error) {
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
	if len(steps) <= 1 {
		return nil
	}
	indices := make([]int, len(steps))
	for i, s := range steps {
		indices[i] = s.Index
	}
	sort.Ints(indices)

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
	Header Header         `json:"header"`
	Steps  []harness.Step `json:"steps"`
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
	dataDir string
	repoID  string
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

// Path returns the path for an episode file:
// <data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst
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
	if ep == nil {
		return "", ErrNilEpisode
	}
	if ep.Header.SessionID == "" {
		return "", ErrEmptySessionID
	}
	if err := validateSessionID(ep.Header.SessionID); err != nil {
		return "", err
	}

	// Fill defaults if unset
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

	enc, err := zstd.NewWriter(tmpFile, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return "", fmt.Errorf("creating zstd encoder: %w", err)
	}

	// Line 1: Header line
	headerBytes, err := json.Marshal(ep.Header)
	if err != nil {
		_ = enc.Close()
		return "", fmt.Errorf("marshaling episode header: %w", err)
	}
	if _, err := enc.Write(append(headerBytes, '\n')); err != nil {
		_ = enc.Close()
		return "", fmt.Errorf("writing episode header: %w", err)
	}

	// Step lines
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

	// Line 1: Header line
	headerBytes, err := readLineBytes(reader)
	if err != nil {
		return nil, fmt.Errorf("reading episode header from %s: %w", path, err)
	}
	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshaling episode header from %s: %w", path, err)
	}

	// Subsequent lines: Steps
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
	sort.Strings(paths)
	return paths, nil
}

// ListHeaders returns the Header of all stored episodes, sorted by path.
func (s *Store) ListHeaders(year ...int) ([]*Header, error) {
	paths, err := s.ListPaths(year...)
	if err != nil {
		return nil, err
	}
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

func readLineBytes(r *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, err := r.ReadBytes('\n')
		line = append(line, chunk...)
		if err != nil {
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

func validateSessionID(sessionID string) error {
	if sessionID == "" {
		return ErrEmptySessionID
	}
	if strings.ContainsAny(sessionID, "/\\") || sessionID == "." || sessionID == ".." || strings.Contains(sessionID, "..") {
		return fmt.Errorf("%w: %q", ErrInvalidSessionID, sessionID)
	}
	return nil
}
