/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package episode_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestHeaderFieldsAndJSON(t *testing.T) {
	parent := "ep-01ARZ3NDEKTSV4RRFFQ69G5FA1"
	started := time.Date(2026, 9, 22, 19, 4, 11, 0, time.UTC)
	ended := time.Date(2026, 9, 22, 19, 30, 0, 0, time.UTC)

	header := episode.Header{
		EpisodeID:        episode.NewEpisodeID(),
		Harness:          episode.HarnessAntigravity,
		SessionID:        "session-xyz-123",
		ParentEpisode:    &parent,
		RepoID:           "repo-sha256-abc",
		Branch:           "ble/wo6-dis-service",
		Started:          started,
		Ended:            ended,
		HeadStart:        "sha-start",
		HeadEnd:          "sha-end",
		Commits:          []string{"commit-1", "commit-2"},
		StepGaps:         [][2]int{{812, 820}},
		Model:            "gemini-2.5-pro",
		HarnessVersion:   "2.1.284",
		StartedBy:        episode.StartedByHuman,
		CompactorVersion: episode.DefaultCompactorVersion,
		PipelineVersion:  episode.DefaultPipelineVersion,
		Stats: episode.Stats{
			RawChars:  12500,
			KeptChars: 3100,
		},
	}

	data, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("failed to marshal header: %v", err)
	}

	var parsed episode.Header
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal header: %v", err)
	}

	if parsed.Harness != episode.HarnessAntigravity {
		t.Errorf("expected harness %s, got %s", episode.HarnessAntigravity, parsed.Harness)
	}
	if parsed.HarnessVersion != "2.1.284" {
		t.Errorf("expected harness_version '2.1.284', got %s", parsed.HarnessVersion)
	}
	if parsed.StartedBy != episode.StartedByHuman {
		t.Errorf("expected started_by %s, got %s", episode.StartedByHuman, parsed.StartedBy)
	}
	if parsed.PipelineVersion != episode.DefaultPipelineVersion {
		t.Errorf("expected pipeline_version %s, got %s", episode.DefaultPipelineVersion, parsed.PipelineVersion)
	}
	if parsed.GetParentEpisode() != parent {
		t.Errorf("expected parent %s, got %s", parent, parsed.GetParentEpisode())
	}
	if !parsed.Started.Equal(started) {
		t.Errorf("expected started %v, got %v", started, parsed.Started)
	}
	if len(parsed.StepGaps) != 1 || parsed.StepGaps[0] != [2]int{812, 820} {
		t.Errorf("unexpected step gaps: %v", parsed.StepGaps)
	}
	if parsed.Stats.RawChars != 12500 || parsed.Stats.KeptChars != 3100 {
		t.Errorf("unexpected stats: %+v", parsed.Stats)
	}
}

func TestHeaderNullParentEpisode(t *testing.T) {
	header := episode.Header{
		SessionID: "sess-null-parent",
	}

	data, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// parent_episode should marshal as null
	if !strings.Contains(string(data), `"parent_episode":null`) {
		t.Errorf("expected parent_episode:null in json: %s", string(data))
	}

	var parsed episode.Header
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if parsed.ParentEpisode != nil {
		t.Errorf("expected nil parent_episode, got %v", *parsed.ParentEpisode)
	}
}

func TestHeaderFlexibleTimestamps(t *testing.T) {
	rawJSON := `{
		"episode_id": "ep-1",
		"session_id": "sess-time",
		"started": "2026-09-22T19:04:11Z",
		"ended": ""
	}`

	var header episode.Header
	if err := json.Unmarshal([]byte(rawJSON), &header); err != nil {
		t.Fatalf("failed to unmarshal with empty ended string: %v", err)
	}

	if header.Started.Year() != 2026 {
		t.Errorf("expected year 2026, got %d", header.Started.Year())
	}
	if !header.Ended.IsZero() {
		t.Errorf("expected zero ended time, got %v", header.Ended)
	}
}

func TestGapRecording(t *testing.T) {
	header := episode.Header{}

	// 1. Manual gap recording
	header.RecordGap(812, 820)
	if !header.HasGaps() {
		t.Fatal("expected HasGaps to be true")
	}
	if header.TotalMissingSteps() != 9 {
		t.Errorf("expected 9 missing steps (812-820), got %d", header.TotalMissingSteps())
	}
	gaps := header.Gaps()
	if len(gaps) != 1 || gaps[0].Start() != 812 || gaps[0].End() != 820 {
		t.Errorf("unexpected gap: %v", gaps)
	}
	if gaps[0].String() != "[812-820]" {
		t.Errorf("unexpected gap string: %s", gaps[0].String())
	}

	// 2. Automatic gap detection from steps
	steps := []harness.Step{
		{Index: 0, Kind: harness.StepKindUser, Text: "hi"},
		{Index: 1, Kind: harness.StepKindAgent, Text: "hello"},
		{Index: 4, Kind: harness.StepKindCommand, Command: "ls"}, // missing 2, 3 -> gap [2, 3]
		{Index: 5, Kind: harness.StepKindRead, File: "a.txt"},
		{Index: 8, Kind: harness.StepKindEdit, File: "b.txt"}, // missing 6, 7 -> gap [6, 7]
	}

	header2 := episode.Header{}
	header2.RecordGaps(steps)
	if len(header2.StepGaps) != 2 {
		t.Fatalf("expected 2 detected gaps, got %d: %v", len(header2.StepGaps), header2.StepGaps)
	}
	if header2.StepGaps[0] != [2]int{2, 3} || header2.StepGaps[1] != [2]int{6, 7} {
		t.Errorf("unexpected detected gaps: %v", header2.StepGaps)
	}
	if header2.TotalMissingSteps() != 4 {
		t.Errorf("expected 4 total missing steps, got %d", header2.TotalMissingSteps())
	}

	// Idempotency: re-running should not duplicate
	header2.RecordGaps(steps)
	if len(header2.StepGaps) != 2 {
		t.Errorf("expected still 2 gaps after duplicate call, got %d", len(header2.StepGaps))
	}

	// Single missing step: 68 -> 70 (missing 69)
	header3 := episode.Header{}
	header3.RecordGaps([]harness.Step{
		{Index: 68, Kind: harness.StepKindAgent},
		{Index: 70, Kind: harness.StepKindAgent},
	})
	if len(header3.StepGaps) != 1 || header3.StepGaps[0] != [2]int{69, 69} {
		t.Errorf("expected single gap [69, 69], got %v", header3.StepGaps)
	}
	if header3.Gaps()[0].String() != "[69]" {
		t.Errorf("expected string '[69]', got %s", header3.Gaps()[0].String())
	}
}

func TestStoreStoragePath(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "test-repo-123"
	store := episode.NewStore(dataDir, repoID)

	sessionID := "conv-456"
	expectedPath := filepath.Join(dataDir, repoID, "episodes", "2026", sessionID+".jsonl.zst")

	// Direct path
	p := store.Path(sessionID, 2026)
	if p != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, p)
	}

	// Path from episode with Started time
	ep := &episode.Episode{
		Header: episode.Header{
			SessionID: sessionID,
			Started:   time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		},
	}
	if epPath := store.EpisodePath(ep); epPath != expectedPath {
		t.Errorf("expected episode path %s, got %s", expectedPath, epPath)
	}
}

func TestStoreWriteAndReadZstd(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "test-repo-zstd"
	store := episode.NewStore(dataDir, repoID)

	parentEp := "ep-parent-999"
	started := time.Date(2026, 8, 15, 14, 30, 0, 0, time.UTC)
	ended := time.Date(2026, 8, 15, 14, 45, 0, 0, time.UTC)
	exitCode := 1

	ep := &episode.Episode{
		Header: episode.Header{
			Harness:          episode.HarnessAntigravity,
			SessionID:        "session-test-01",
			ParentEpisode:    &parentEp,
			Branch:           "main",
			Started:          started,
			Ended:            ended,
			Model:            "gemini-2.5-flash",
			HarnessVersion:   "1.0.0",
			StartedBy:        episode.StartedByAutomation,
			PipelineVersion:  episode.DefaultPipelineVersion,
			CompactorVersion: episode.DefaultCompactorVersion,
			Stats: episode.Stats{
				RawChars:  500,
				KeptChars: 120,
			},
		},
		Steps: []harness.Step{
			{Index: 0, Kind: harness.StepKindUser, Author: "automation", Text: "run tests"},
			{Index: 1, Kind: harness.StepKindAgent, Text: "running build"},
			{Index: 2, Kind: harness.StepKindCommand, Command: "pio run", ExitCode: &exitCode, Outcome: "build_fail", Excerpt: "compile error"},
			{Index: 4, Kind: harness.StepKindEdit, File: "main.cpp", Commit: "abc1234"}, // triggers gap [3, 3]
		},
	}

	filePath, err := store.WriteEpisode(ep)
	if err != nil {
		t.Fatalf("failed to write episode: %v", err)
	}

	// 1. Verify file exists at correct path
	expectedSubpath := filepath.Join(repoID, "episodes", "2026", "session-test-01.jsonl.zst")
	if !strings.HasSuffix(filePath, expectedSubpath) {
		t.Errorf("path does not end with %s, got: %s", expectedSubpath, filePath)
	}

	// 2. Verify file is zstd compressed (magic header 0x28, 0xb5, 0x2f, 0xfd)
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read raw file: %v", err)
	}
	zstdMagic := []byte{0x28, 0xb5, 0x2f, 0xfd}
	if len(rawBytes) < 4 || !bytes.Equal(rawBytes[:4], zstdMagic) {
		t.Fatalf("file does not start with zstd magic bytes: %v", rawBytes[:min(4, len(rawBytes))])
	}

	// 3. Read back episode
	readEp, err := store.Read("session-test-01", 2026)
	if err != nil {
		t.Fatalf("failed to read episode: %v", err)
	}

	// Verify Header
	if readEp.Header.SessionID != "session-test-01" {
		t.Errorf("session ID mismatch: got %s", readEp.Header.SessionID)
	}
	if readEp.Header.Harness != episode.HarnessAntigravity {
		t.Errorf("harness mismatch: got %s", readEp.Header.Harness)
	}
	if readEp.Header.HarnessVersion != "1.0.0" {
		t.Errorf("harness version mismatch: got %s", readEp.Header.HarnessVersion)
	}
	if readEp.Header.StartedBy != episode.StartedByAutomation {
		t.Errorf("started_by mismatch: got %s", readEp.Header.StartedBy)
	}
	if readEp.Header.PipelineVersion != episode.DefaultPipelineVersion {
		t.Errorf("pipeline_version mismatch: got %s", readEp.Header.PipelineVersion)
	}
	if readEp.Header.RepoID != repoID {
		t.Errorf("repoID mismatch: expected %s, got %s", repoID, readEp.Header.RepoID)
	}
	if !readEp.Header.Started.Equal(started) {
		t.Errorf("started time mismatch: %v vs %v", readEp.Header.Started, started)
	}
	// Verify auto-detected gap [3, 3] was recorded
	if len(readEp.Header.StepGaps) != 1 || readEp.Header.StepGaps[0] != [2]int{3, 3} {
		t.Errorf("expected auto gap [3, 3], got %v", readEp.Header.StepGaps)
	}

	// Verify Steps
	if len(readEp.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(readEp.Steps))
	}
	if readEp.Steps[0].Kind != harness.StepKindUser || readEp.Steps[0].Text != "run tests" {
		t.Errorf("step 0 mismatch: %+v", readEp.Steps[0])
	}
	if readEp.Steps[2].Command != "pio run" || *readEp.Steps[2].ExitCode != 1 {
		t.Errorf("step 2 mismatch: %+v", readEp.Steps[2])
	}

	// 4. Read back without year (searching across year dirs)
	searchedEp, err := store.Read("session-test-01")
	if err != nil {
		t.Fatalf("failed to read episode without specifying year: %v", err)
	}
	if searchedEp.Header.EpisodeID != readEp.Header.EpisodeID {
		t.Errorf("episode IDs do not match: %s vs %s", searchedEp.Header.EpisodeID, readEp.Header.EpisodeID)
	}

	// 5. Read only Header
	hdr, err := store.ReadHeader("session-test-01")
	if err != nil {
		t.Fatalf("failed to read header: %v", err)
	}
	if hdr.SessionID != "session-test-01" {
		t.Errorf("header session ID mismatch: %s", hdr.SessionID)
	}
}

func TestStoreListingAndExists(t *testing.T) {
	dataDir := t.TempDir()
	repoID := "repo-list-test"
	store := episode.NewStore(dataDir, repoID)

	ep2025 := &episode.Episode{
		Header: episode.Header{
			SessionID: "sess-2025",
			Started:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		Steps: []harness.Step{{Index: 0, Kind: harness.StepKindUser, Text: "hello"}},
	}
	ep2026A := &episode.Episode{
		Header: episode.Header{
			SessionID: "sess-2026-a",
			Started:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		Steps: []harness.Step{{Index: 0, Kind: harness.StepKindUser, Text: "hi"}},
	}
	ep2026B := &episode.Episode{
		Header: episode.Header{
			SessionID: "sess-2026-b",
			Started:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		},
		Steps: []harness.Step{{Index: 0, Kind: harness.StepKindUser, Text: "hey"}},
	}

	for _, ep := range []*episode.Episode{ep2025, ep2026A, ep2026B} {
		if err := store.Write(ep); err != nil {
			t.Fatalf("failed to write %s: %v", ep.Header.SessionID, err)
		}
	}

	// Exists checks
	if !store.Exists("sess-2025") {
		t.Error("expected sess-2025 to exist")
	}
	if !store.Exists("sess-2026-a", 2026) {
		t.Error("expected sess-2026-a to exist in 2026")
	}
	if store.Exists("sess-nonexistent") {
		t.Error("expected sess-nonexistent to not exist")
	}

	// List all
	allSessions, err := store.List()
	if err != nil {
		t.Fatalf("failed to list all: %v", err)
	}
	if len(allSessions) != 3 {
		t.Errorf("expected 3 sessions, got %d: %v", len(allSessions), allSessions)
	}

	// List year 2026
	sessions2026, err := store.List(2026)
	if err != nil {
		t.Fatalf("failed to list 2026: %v", err)
	}
	if len(sessions2026) != 2 {
		t.Errorf("expected 2 sessions in 2026, got %d: %v", len(sessions2026), sessions2026)
	}

	// ListHeaders
	headers, err := store.ListHeaders(2026)
	if err != nil {
		t.Fatalf("failed to list headers 2026: %v", err)
	}
	if len(headers) != 2 {
		t.Errorf("expected 2 headers, got %d", len(headers))
	}
}

func TestStoreValidationAndErrors(t *testing.T) {
	dataDir := t.TempDir()
	store := episode.NewStore(dataDir, "repo-val")

	// Nil episode
	if err := store.Write(nil); err != episode.ErrNilEpisode {
		t.Errorf("expected ErrNilEpisode, got %v", err)
	}

	// Empty session ID
	if err := store.Write(&episode.Episode{}); err != episode.ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID, got %v", err)
	}

	// Traversal attempt
	badEp := &episode.Episode{
		Header: episode.Header{SessionID: "../../etc/passwd"},
	}
	if err := store.Write(badEp); err == nil || !strings.Contains(err.Error(), "invalid session id") {
		t.Errorf("expected invalid session id error, got %v", err)
	}

	// Non-existent read
	if _, err := store.Read("does-not-exist"); err == nil {
		t.Error("expected error reading non-existent episode")
	}
}

func TestEpisodeStatsComputation(t *testing.T) {
	ep := episode.NewEpisode(
		episode.Header{SessionID: "sess-stats"},
		harness.Step{Index: 0, Kind: harness.StepKindUser, Text: "12345"},                 // 5
		harness.Step{Index: 1, Kind: harness.StepKindAgent, Text: "67890"},                // 5
		harness.Step{Index: 2, Kind: harness.StepKindCommand, Command: "ls", Excerpt: "e"}, // 2 + 1 = 3
	)

	kept := ep.ComputeKeptChars()
	if kept != 13 {
		t.Errorf("expected 13 kept chars, got %d", kept)
	}

	ep.UpdateStats(100)
	if ep.Header.Stats.RawChars != 100 || ep.Header.Stats.KeptChars != 13 {
		t.Errorf("unexpected stats: %+v", ep.Header.Stats)
	}
}

func TestHeaderParseAndFlexibleCommits(t *testing.T) {
	// 1. Array of strings
	rawArray := `{"session_id":"s1","commits":["c1","c2"]}`
	h1, err := episode.ParseHeader([]byte(rawArray))
	if err != nil {
		t.Fatalf("failed to parse array commits: %v", err)
	}
	if len(h1.Commits) != 2 || h1.Commits[0] != "c1" || h1.Commits[1] != "c2" {
		t.Errorf("unexpected commits: %v", h1.Commits)
	}

	// 2. Comma-separated string
	rawStr := `{"session_id":"s2","commits":"c1, c2, c3"}`
	h2, err := episode.ParseHeaderLine(rawStr)
	if err != nil {
		t.Fatalf("failed to parse comma-separated string commits: %v", err)
	}
	if len(h2.Commits) != 3 || h2.Commits[0] != "c1" || h2.Commits[2] != "c3" {
		t.Errorf("unexpected commits: %v", h2.Commits)
	}

	// 3. Array of objects
	rawObjects := `{"session_id":"s3","commits":[{"sha":"c1"},{"commit":"c2"},{"hash":"c3"}]}`
	h3, err := episode.ParseHeader([]byte(rawObjects))
	if err != nil {
		t.Fatalf("failed to parse object array commits: %v", err)
	}
	if len(h3.Commits) != 3 || h3.Commits[0] != "c1" || h3.Commits[1] != "c2" || h3.Commits[2] != "c3" {
		t.Errorf("unexpected commits: %v", h3.Commits)
	}

	// 4. Null commits
	rawNull := `{"session_id":"s4","commits":null}`
	h4, err := episode.ParseHeader([]byte(rawNull))
	if err != nil {
		t.Fatalf("failed to parse null commits: %v", err)
	}
	if len(h4.Commits) != 0 {
		t.Errorf("expected empty commits for null, got %v", h4.Commits)
	}

	// 5. Empty header error
	if _, err := episode.ParseHeader([]byte("")); err == nil {
		t.Error("expected error for empty header bytes")
	}
}

func TestHeaderAndEpisodeCommitHelpers(t *testing.T) {
	h := &episode.Header{SessionID: "sess-helpers"}
	if h.HasCommits() {
		t.Error("expected HasCommits false initially")
	}
	if h.HasCommit("sha1") {
		t.Error("expected HasCommit false initially")
	}

	h.RecordCommit("sha1")
	if !h.HasCommits() || !h.HasCommit("sha1") {
		t.Error("expected HasCommits and HasCommit true after RecordCommit")
	}

	// Duplicate RecordCommit should not duplicate
	h.RecordCommit("sha1")
	h.RecordCommit("SHA1") // case-insensitive check
	if len(h.Commits) != 1 {
		t.Errorf("expected 1 commit after duplicate RecordCommit, got %d", len(h.Commits))
	}

	// RecordCommits
	h.RecordCommits([]string{"sha2", "sha3", "sha1"})
	if len(h.Commits) != 3 {
		t.Errorf("expected 3 commits after RecordCommits, got %d (%v)", len(h.Commits), h.Commits)
	}

	// Episode helpers
	ep := episode.NewEpisode(*h)
	if !ep.HasCommits() || !ep.HasCommit("sha2") {
		t.Error("expected episode to have commits")
	}

	ep.RecordCommit("sha4")
	if !ep.HasCommit("sha4") {
		t.Error("expected episode to have sha4")
	}
	if !ep.Header.HasCommit("sha4") {
		t.Error("expected episode header to have sha4")
	}
}
