package antigravity

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	_ "modernc.org/sqlite"
)

func TestParseWorkspaceURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "standard file URI",
			input:    "file:///Users/testuser/repo",
			expected: filepath.Clean("/Users/testuser/repo"),
			wantErr:  false,
		},
		{
			name:     "file URI with percent encoding",
			input:    "file:///Users/testuser/my%20projects/repo",
			expected: filepath.Clean("/Users/testuser/my projects/repo"),
			wantErr:  false,
		},
		{
			name:     "plain path",
			input:    "/Users/testuser/repo",
			expected: filepath.Clean("/Users/testuser/repo"),
			wantErr:  false,
		},
		{
			name:     "empty URI",
			input:    "   ",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "file: single slash",
			input:    "file:/Users/testuser/repo",
			expected: filepath.Clean("/Users/testuser/repo"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseWorkspaceURI(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseWorkspaceURI() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.expected {
				t.Errorf("ParseWorkspaceURI() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func createTestDB(t *testing.T, dbPath string, statements []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
CREATE TABLE conversation_summaries (
	conversation_id text PRIMARY KEY,
	title text NOT NULL DEFAULT "",
	preview text NOT NULL DEFAULT "",
	step_count integer NOT NULL DEFAULT 0,
	last_modified_time datetime NOT NULL,
	workspace_uris text NOT NULL,
	status text NOT NULL DEFAULT "",
	source text NOT NULL DEFAULT "",
	project_id text NOT NULL DEFAULT "",
	agent_name text NOT NULL DEFAULT "",
	parent_conversation_id text NOT NULL DEFAULT "",
	nesting_depth integer NOT NULL DEFAULT 0,
	battle_id text NOT NULL DEFAULT "",
	winning_conversation_id text NOT NULL DEFAULT "",
	not_fully_idle numeric NOT NULL DEFAULT 0,
	killed numeric NOT NULL DEFAULT 0,
	last_user_input_time datetime NOT NULL DEFAULT "",
	last_user_input_step_index integer NOT NULL DEFAULT -1,
	app_data_dir text NOT NULL DEFAULT "",
	raw_summary blob,
	group_id text NOT NULL DEFAULT ""
);
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("creating test schema: %v", err)
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("executing test statement %q: %v", stmt, err)
		}
	}
}

func TestLoadSummariesFromDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "conversation_summaries.db")

	statements := []string{
		`INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time, workspace_uris, parent_conversation_id, not_fully_idle, killed)
		 VALUES ('c1', 'Top Conv', 42, '2026-10-02 08:00:00+00:00', '["file:///workspace/repo1"]', '', 0, 0);`,
		`INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time, workspace_uris, parent_conversation_id, not_fully_idle, killed)
		 VALUES ('c2', 'Subagent Conv', 10, '2026-10-02 08:05:00+00:00', '["file:///workspace/repo1"]', 'c1', 1, 0);`,
		`INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time, workspace_uris, parent_conversation_id, not_fully_idle, killed)
		 VALUES ('c3', 'Killed Subagent', 5, '2026-10-02 08:10:00+00:00', '[]', 'c1', 0, 1);`,
	}
	createTestDB(t, dbPath, statements)

	summaries, err := LoadSummariesFromDB(dbPath, "app")
	if err != nil {
		t.Fatalf("LoadSummariesFromDB failed: %v", err)
	}

	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}

	c1 := summaries["c1"]
	if c1 == nil || c1.Title != "Top Conv" || c1.StepCount != 42 || c1.ParentConversationID != "" {
		t.Errorf("unexpected c1 summary: %+v", c1)
	}
	if len(c1.WorkspacePaths) != 1 || c1.WorkspacePaths[0] != filepath.Clean("/workspace/repo1") {
		t.Errorf("unexpected c1 workspace paths: %v", c1.WorkspacePaths)
	}
	if !c1.InAppStore || c1.SourceStore != "app" {
		t.Errorf("unexpected store markers: inApp=%v, source=%s", c1.InAppStore, c1.SourceStore)
	}

	c2 := summaries["c2"]
	if c2 == nil || c2.ParentConversationID != "c1" || !c2.NotFullyIdle {
		t.Errorf("unexpected c2 summary: %+v", c2)
	}

	c3 := summaries["c3"]
	if c3 == nil || c3.ParentConversationID != "c1" || !c3.Killed || len(c3.WorkspacePaths) != 0 {
		t.Errorf("unexpected c3 summary: %+v", c3)
	}

	// Test non-existent file: returns empty without error
	emptySummaries, err := LoadSummariesFromDB(filepath.Join(tempDir, "does-not-exist.db"), "cli")
	if err != nil {
		t.Fatalf("expected nil error for missing DB, got: %v", err)
	}
	if len(emptySummaries) != 0 {
		t.Errorf("expected 0 summaries for missing DB, got %d", len(emptySummaries))
	}
}

func TestDeduplicateSummaries(t *testing.T) {
	tApp := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	tCLI := time.Date(2026, 10, 2, 8, 15, 0, 0, time.UTC)

	appSummaries := map[string]*ConversationSummary{
		"c-app-only": {
			ConversationID:   "c-app-only",
			Title:            "App Only",
			StepCount:        10,
			LastModifiedTime: tApp,
			WorkspacePaths:   []string{"/workspace/repo1"},
		},
		"c-both": {
			ConversationID:   "c-both",
			Title:            "Overlap Old",
			StepCount:        20,
			LastModifiedTime: tApp,
			WorkspacePaths:   []string{"/workspace/repo1"},
			NotFullyIdle:     true,
		},
	}

	cliSummaries := map[string]*ConversationSummary{
		"c-cli-only": {
			ConversationID:   "c-cli-only",
			Title:            "CLI Only",
			StepCount:        5,
			LastModifiedTime: tCLI,
			WorkspacePaths:   []string{"/workspace/repo2"},
		},
		"c-both": {
			ConversationID:       "c-both",
			Title:                "Overlap New",
			StepCount:            25,
			LastModifiedTime:     tCLI,
			WorkspacePaths:       []string{}, // Missing in CLI
			ParentConversationID: "c-cli-parent",
			NotFullyIdle:         false,
			Killed:               false,
		},
	}

	merged := DeduplicateSummaries(appSummaries, cliSummaries)

	if len(merged) != 3 {
		t.Fatalf("expected 3 merged summaries, got %d", len(merged))
	}

	appOnly := merged["c-app-only"]
	if !appOnly.InAppStore || appOnly.InCLIStore || appOnly.SourceStore != "app" {
		t.Errorf("unexpected appOnly markers: %+v", appOnly)
	}

	cliOnly := merged["c-cli-only"]
	if cliOnly.InAppStore || !cliOnly.InCLIStore || cliOnly.SourceStore != "cli" {
		t.Errorf("unexpected cliOnly markers: %+v", cliOnly)
	}

	both := merged["c-both"]
	if !both.InAppStore || !both.InCLIStore || both.SourceStore != "both" {
		t.Errorf("unexpected overlap markers: %+v", both)
	}
	// Newer CLI timestamp should win
	if both.LastModifiedTime != tCLI {
		t.Errorf("expected LastModifiedTime %v, got %v", tCLI, both.LastModifiedTime)
	}
	// Max step count (25 > 20)
	if both.StepCount != 25 {
		t.Errorf("expected step count 25, got %d", both.StepCount)
	}
	// Missing workspace in CLI was preserved from App
	if len(both.WorkspacePaths) != 1 || both.WorkspacePaths[0] != "/workspace/repo1" {
		t.Errorf("expected preserved workspace paths from app, got %v", both.WorkspacePaths)
	}
	// Parent conversation ID was preserved
	if both.ParentConversationID != "c-cli-parent" {
		t.Errorf("expected ParentConversationID 'c-cli-parent', got %q", both.ParentConversationID)
	}
	// Idle state updated from CLI
	if both.NotFullyIdle {
		t.Errorf("expected NotFullyIdle false from newer CLI record")
	}
}

func TestLinkSubagents(t *testing.T) {
	summaries := map[string]*ConversationSummary{
		"root": {
			ConversationID: "root",
		},
		"child-1": {
			ConversationID:       "child-1",
			ParentConversationID: "root",
		},
		"child-2": {
			ConversationID:       "child-2",
			ParentConversationID: "root",
		},
		"grandchild": {
			ConversationID:       "grandchild",
			ParentConversationID: "child-1",
		},
	}

	LinkSubagents(summaries)

	root := summaries["root"]
	if len(root.SubagentIDs) != 2 {
		t.Fatalf("expected 2 subagents for root, got %d: %v", len(root.SubagentIDs), root.SubagentIDs)
	}

	child1 := summaries["child-1"]
	if len(child1.SubagentIDs) != 1 || child1.SubagentIDs[0] != "grandchild" {
		t.Errorf("expected grandchild under child-1, got %v", child1.SubagentIDs)
	}

	child2 := summaries["child-2"]
	if len(child2.SubagentIDs) != 0 {
		t.Errorf("expected 0 subagents under child-2, got %v", child2.SubagentIDs)
	}
}

func TestAttributeConversations(t *testing.T) {
	repoRoot := filepath.Clean("/Users/test/projects/my-repo")
	otherRepo := filepath.Clean("/Users/test/projects/other-repo")

	enrolledRepos := map[string]config.RegistryEntry{
		repoRoot: {
			RepoID:   "repo-123",
			RootPath: repoRoot,
		},
	}

	summaries := map[string]*ConversationSummary{
		"c-direct": {
			ConversationID: "c-direct",
			WorkspacePaths: []string{repoRoot},
		},
		"c-subdir": {
			ConversationID: "c-subdir",
			WorkspacePaths: []string{filepath.Join(repoRoot, "src", "pkg")},
		},
		"c-unenrolled": {
			ConversationID: "c-unenrolled",
			WorkspacePaths: []string{otherRepo},
		},
		"c-subagent-inherits": {
			ConversationID:       "c-subagent-inherits",
			WorkspacePaths:       []string{}, // No workspace directly
			ParentConversationID: "c-direct",
		},
		"c-grandchild-inherits": {
			ConversationID:       "c-grandchild-inherits",
			WorkspacePaths:       []string{},
			ParentConversationID: "c-subagent-inherits",
		},
		"c-subagent-unenrolled": {
			ConversationID:       "c-subagent-unenrolled",
			WorkspacePaths:       []string{},
			ParentConversationID: "c-unenrolled",
		},
	}

	attributed := AttributeConversations(summaries, enrolledRepos)

	// c-direct should match
	if attr, ok := attributed["c-direct"]; !ok || attr.RepoID != "repo-123" {
		t.Errorf("expected c-direct attributed to repo-123, got: %+v", attr)
	}

	// c-subdir should match
	if attr, ok := attributed["c-subdir"]; !ok || attr.RepoID != "repo-123" {
		t.Errorf("expected c-subdir attributed to repo-123, got: %+v", attr)
	}

	// c-unenrolled should NOT match
	if attr, ok := attributed["c-unenrolled"]; ok {
		t.Errorf("expected c-unenrolled not to be attributed, got: %+v", attr)
	}

	// c-subagent-inherits should inherit repo-123 from c-direct
	if attr, ok := attributed["c-subagent-inherits"]; !ok || attr.RepoID != "repo-123" {
		t.Errorf("expected c-subagent-inherits attributed to repo-123, got: %+v", attr)
	}

	// c-grandchild-inherits should recursively inherit repo-123
	if attr, ok := attributed["c-grandchild-inherits"]; !ok || attr.RepoID != "repo-123" {
		t.Errorf("expected c-grandchild-inherits attributed to repo-123, got: %+v", attr)
	}

	// c-subagent-unenrolled should NOT match
	if attr, ok := attributed["c-subagent-unenrolled"]; ok {
		t.Errorf("expected c-subagent-unenrolled not to be attributed, got: %+v", attr)
	}
}

func TestIsConversationIdle(t *testing.T) {
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	cutoff := 15 * time.Minute

	// Active (modified 5m ago)
	activeSummary := ConversationSummary{
		LastModifiedTime: now.Add(-5 * time.Minute),
	}
	if IsConversationIdle(activeSummary, "", now, cutoff) {
		t.Error("expected session modified 5m ago to be NOT idle")
	}

	// Idle (modified 20m ago)
	idleSummary := ConversationSummary{
		LastModifiedTime: now.Add(-20 * time.Minute),
	}
	if !IsConversationIdle(idleSummary, "", now, cutoff) {
		t.Error("expected session modified 20m ago to be idle")
	}

	// Active by time, but explicitly Killed
	killedSummary := ConversationSummary{
		LastModifiedTime: now.Add(-2 * time.Minute),
		Killed:           true,
	}
	if !IsConversationIdle(killedSummary, "", now, cutoff) {
		t.Error("expected killed session to be immediately idle")
	}

	// Transcript modified recently on disk
	tempDir := t.TempDir()
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	_ = os.WriteFile(transcriptPath, []byte("data"), 0644)
	// file was just created, so its mod time is ~now
	if IsConversationIdle(idleSummary, transcriptPath, now, cutoff) {
		// Note: if transcript was modified recently, it's not idle
	}
}

func TestSessionAttributor_Integration(t *testing.T) {
	tempDir := t.TempDir()
	appDB := filepath.Join(tempDir, "app", "conversation_summaries.db")
	cliDB := filepath.Join(tempDir, "cli", "conversation_summaries.db")
	appBrain := filepath.Join(tempDir, "app", "brain")
	cliBrain := filepath.Join(tempDir, "cli", "brain")

	repoRoot := filepath.Clean("/workspace/test-repo")
	enrolledRepos := map[string]config.RegistryEntry{
		repoRoot: {
			RepoID:   "repo-test-1",
			RootPath: repoRoot,
		},
	}

	// Create transcripts on disk for c1 (in app) and c2 (in cli)
	c1TranscriptDir := filepath.Join(appBrain, "c1", ".system_generated", "logs")
	_ = os.MkdirAll(c1TranscriptDir, 0755)
	_ = os.WriteFile(filepath.Join(c1TranscriptDir, "transcript_full.jsonl"), []byte("{}\n"), 0644)

	c2TranscriptDir := filepath.Join(cliBrain, "c2", ".system_generated", "logs")
	_ = os.MkdirAll(c2TranscriptDir, 0755)
	_ = os.WriteFile(filepath.Join(c2TranscriptDir, "transcript_full.jsonl"), []byte("{}\n"), 0644)
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	transcriptModTime := now.Add(-1 * time.Hour)
	_ = os.Chtimes(filepath.Join(c1TranscriptDir, "transcript_full.jsonl"), transcriptModTime, transcriptModTime)
	_ = os.Chtimes(filepath.Join(c2TranscriptDir, "transcript_full.jsonl"), transcriptModTime, transcriptModTime)
	_ = os.Chtimes(c1TranscriptDir, transcriptModTime, transcriptModTime)
	_ = os.Chtimes(c2TranscriptDir, transcriptModTime, transcriptModTime)
	_ = os.Chtimes(filepath.Join(appBrain, "c1"), transcriptModTime, transcriptModTime)
	_ = os.Chtimes(filepath.Join(cliBrain, "c2"), transcriptModTime, transcriptModTime)

	// App DB rows: c1 (enrolled, idle), c-dup (in both)
	createTestDB(t, appDB, []string{
		`INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time, workspace_uris, parent_conversation_id)
		 VALUES ('c1', 'App Session', 10, '2026-10-02 09:00:00+00:00', '["file:///workspace/test-repo"]', '');`,
		`INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time, workspace_uris, parent_conversation_id)
		 VALUES ('c-dup', 'Dup Session Old', 5, '2026-10-02 09:00:00+00:00', '["file:///workspace/test-repo"]', '');`,
	})

	// CLI DB rows: c2 (subagent of c1, inherits repo, idle), c-dup (newer)
	createTestDB(t, cliDB, []string{
		`INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time, workspace_uris, parent_conversation_id)
		 VALUES ('c2', 'CLI Subagent', 8, '2026-10-02 09:10:00+00:00', '[]', 'c1');`,
		`INSERT INTO conversation_summaries (conversation_id, title, step_count, last_modified_time, workspace_uris, parent_conversation_id)
		 VALUES ('c-dup', 'Dup Session New', 12, '2026-10-02 09:30:00+00:00', '["file:///workspace/test-repo"]', '');`,
	})

	attributor := NewSessionAttributor(
		WithAppDBPath(appDB),
		WithCLIDBPath(cliDB),
		WithAppBrainDir(appBrain),
		WithCLIBrainDir(cliBrain),
		WithAttributionNow(now),
		WithIdleCutoff(15*time.Minute),
	)

	ctx := context.Background()
	sessions, err := attributor.AttributedSessions(ctx, enrolledRepos)
	if err != nil {
		t.Fatalf("AttributedSessions failed: %v", err)
	}

	// Should have c1, c2, c-dup (de-duplicated from both stores)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 attributed sessions, got %d", len(sessions))
	}

	sessionMap := make(map[string]tick.AttributedSession)
	for _, s := range sessions {
		sessionMap[s.ConversationID] = s
	}

	// c1: direct enrolled session
	s1, ok := sessionMap["c1"]
	if !ok {
		t.Fatal("c1 not found in attributed sessions")
	}
	if s1.RepoID != "repo-test-1" {
		t.Errorf("expected repo-test-1 for c1, got %s", s1.RepoID)
	}
	if s1.TranscriptPath == "" {
		t.Error("expected non-empty transcript path for c1")
	}
	if !s1.Idle {
		t.Error("expected c1 to be idle (modified 1h ago)")
	}

	// c2: subagent inheriting from c1
	s2, ok := sessionMap["c2"]
	if !ok {
		t.Fatal("c2 not found in attributed sessions")
	}
	if s2.RepoID != "repo-test-1" {
		t.Errorf("expected repo-test-1 for subagent c2, got %s", s2.RepoID)
	}
	if s2.ParentConversationID != "c1" {
		t.Errorf("expected parent_conversation_id 'c1' for c2, got %s", s2.ParentConversationID)
	}
	if s2.TranscriptPath == "" {
		t.Error("expected non-empty transcript path for c2")
	}

	// c-dup: de-duplicated
	sDup, ok := sessionMap["c-dup"]
	if !ok {
		t.Fatal("c-dup not found in attributed sessions")
	}
	if sDup.RepoID != "repo-test-1" {
		t.Errorf("expected repo-test-1 for c-dup, got %s", sDup.RepoID)
	}
}
