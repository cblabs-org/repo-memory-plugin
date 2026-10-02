package episode_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

type mockGitRunner struct {
	responses map[string]string
	errs      map[string]error
	calls     [][]string
}

func newMockGitRunner() *mockGitRunner {
	return &mockGitRunner{
		responses: make(map[string]string),
		errs:      make(map[string]error),
	}
}

func (m *mockGitRunner) RunGit(repoRoot string, args ...string) (string, error) {
	m.calls = append(m.calls, args)
	key := strings.Join(args, " ")
	for k, resp := range m.responses {
		if strings.HasPrefix(key, k) || strings.Contains(key, k) {
			if err := m.errs[k]; err != nil {
				return resp, err
			}
			return resp, nil
		}
	}
	return "", nil
}

func TestExtractTranscriptCommits(t *testing.T) {
	exitZero := 0
	exitOne := 1

	steps := []harness.Step{
		{Index: 0, Kind: harness.StepKindUser, Text: "Please make changes"},
		{
			Index:    1,
			Kind:     harness.StepKindCommand,
			Command:  `git commit -m "feat: first feature"`,
			ExitCode: &exitZero,
			Excerpt:  "[feature-branch 4a076ea] feat: first feature\n 1 file changed",
		},
		{
			Index:    2,
			Kind:     harness.StepKindCommand,
			Command:  `git status`,
			ExitCode: &exitZero,
			Excerpt:  "nothing to commit, working tree clean",
		},
		{
			Index:    3,
			Kind:     harness.StepKindCommand,
			Command:  `git commit -am "fix: typo"`,
			ExitCode: &exitOne,
			Excerpt:  "error: could not commit",
		},
		{
			Index:  4,
			Kind:   harness.StepKindEdit,
			File:   "main.go",
			Commit: "bf11a76bfb254d3943289660cdb6fdfb7c4cd822",
		},
	}

	tcs := episode.ExtractTranscriptCommits(steps)
	if len(tcs) != 3 {
		t.Fatalf("expected 3 extracted commit commands, got %d", len(tcs))
	}

	// First commit command
	if tcs[0].StepIndex != 1 || tcs[0].Message != "feat: first feature" {
		t.Errorf("unexpected tc 0: %+v", tcs[0])
	}
	if len(tcs[0].Hashes) == 0 || tcs[0].Hashes[0] != "4a076ea" {
		t.Errorf("expected hash '4a076ea' extracted from excerpt, got %v", tcs[0].Hashes)
	}

	// Second commit command (failed)
	if tcs[1].StepIndex != 3 || tcs[1].Message != "fix: typo" || *tcs[1].ExitCode != 1 {
		t.Errorf("unexpected tc 1: %+v", tcs[1])
	}

	// Third step with explicit commit
	if tcs[2].StepIndex != 4 || len(tcs[2].Hashes) == 0 || tcs[2].Hashes[0] != "bf11a76bfb254d3943289660cdb6fdfb7c4cd822" {
		t.Errorf("unexpected tc 2: %+v", tcs[2])
	}
}

func TestCommitLinkerLogMatch(t *testing.T) {
	mock := newMockGitRunner()
	sessionStart := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	sessionEnd := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	commitTime := time.Date(2026, 10, 2, 7, 30, 0, 0, time.UTC)

	fullSHA := "4a076eac53b1dadb00dba2d148a51d4ce8dd9bc2"
	shortSHA := "4a076ea"
	subject := "feat(episode): implement episode store and header structures"

	// Mock git log response: %H\x1f%h\x1f%cI\x1f%s
	logLine := fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s", fullSHA, shortSHA, commitTime.Format(time.RFC3339), subject)
	mock.responses["log"] = logLine
	mock.responses["diff-tree"] = "internal/core/episode/store.go\ninternal/core/episode/store_test.go"

	exitZero := 0
	steps := []harness.Step{
		{
			Index:    1,
			Kind:     harness.StepKindCommand,
			Command:  fmt.Sprintf(`git commit -m "%s"`, subject),
			ExitCode: &exitZero,
			Excerpt:  fmt.Sprintf("[%s] %s\n 2 files changed", shortSHA, subject),
		},
		{
			Index: 2,
			Kind:  harness.StepKindEdit,
			File:  "internal/core/episode/store.go",
		},
	}

	ep := episode.NewEpisode(
		episode.Header{
			SessionID: "sess-commit-link",
			Branch:    "pr-02a-transcript-episode/wo4-commit-linkage",
			Started:   sessionStart,
			Ended:     sessionEnd,
		},
		steps...,
	)

	linker := episode.NewCommitLinker(episode.WithGitRunner(mock))
	err := linker.LinkEpisode("/fake/repo", ep)
	if err != nil {
		t.Fatalf("LinkEpisode failed: %v", err)
	}

	if len(ep.Header.Commits) != 1 || ep.Header.Commits[0] != fullSHA {
		t.Errorf("expected header commits [%s], got %v", fullSHA, ep.Header.Commits)
	}
	if ep.Header.HeadEnd != fullSHA {
		t.Errorf("expected HeadEnd %s, got %s", fullSHA, ep.Header.HeadEnd)
	}

	// Check edit step was resolved to commit
	if ep.Steps[1].Commit != fullSHA {
		t.Errorf("expected edit step commit to be resolved to %s, got %s", fullSHA, ep.Steps[1].Commit)
	}
}

func TestCommitLinkerReflogAmendedMatch(t *testing.T) {
	mock := newMockGitRunner()
	sessionStart := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	sessionEnd := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	time1 := time.Date(2026, 10, 2, 7, 10, 0, 0, time.UTC)
	time2 := time.Date(2026, 10, 2, 7, 20, 0, 0, time.UTC)

	sha1 := "1111111111111111111111111111111111111111"
	sha2 := "2222222222222222222222222222222222222222"

	// Mock git log (only shows latest commit sha2 after amend)
	mock.responses["log feature"] = fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s", sha2, sha2[:7], time2.Format(time.RFC3339), "commit two (amended)")

	// Mock git reflog (shows both initial commit and amended commit)
	reflogOutput := fmt.Sprintf("%s\x1f%s\x1f%s\x1fcommit (amend): commit two (amended)\n%s\x1f%s\x1f%s\x1fcommit: commit one (initial)",
		sha2, sha2[:7], time2.Format(time.RFC3339),
		sha1, sha1[:7], time1.Format(time.RFC3339),
	)
	mock.responses["log -g"] = reflogOutput

	exitZero := 0
	steps := []harness.Step{
		{
			Index:    1,
			Kind:     harness.StepKindCommand,
			Command:  `git commit -m "commit one (initial)"`,
			ExitCode: &exitZero,
			Excerpt:  fmt.Sprintf("[%s] commit one (initial)", sha1[:7]),
		},
		{
			Index:    2,
			Kind:     harness.StepKindCommand,
			Command:  `git commit --amend -m "commit two (amended)"`,
			ExitCode: &exitZero,
			Excerpt:  fmt.Sprintf("[%s] commit two (amended)", sha2[:7]),
		},
	}

	header := &episode.Header{
		SessionID: "sess-amend",
		Branch:    "feature",
		Started:   sessionStart,
		Ended:     sessionEnd,
	}

	linker := episode.NewCommitLinker(episode.WithGitRunner(mock))
	commits, err := linker.Link("/fake/repo", header, steps)
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 linked commits from reflog, got %d (%v)", len(commits), commits)
	}

	// Should be chronologically ordered: sha1 then sha2
	if commits[0] != sha1 || commits[1] != sha2 {
		t.Errorf("expected [%s, %s], got [%s, %s]", sha1, sha2, commits[0], commits[1])
	}
}

func TestCommitLinkerRejectsUnconfirmedCommits(t *testing.T) {
	mock := newMockGitRunner()
	sessionStart := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	sessionEnd := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	commitTime := time.Date(2026, 10, 2, 7, 30, 0, 0, time.UTC)

	fullSHA := "3333333333333333333333333333333333333333"
	mock.responses["log"] = fmt.Sprintf("%s\x1f%s\x1f%s\x1funrelated commit", fullSHA, fullSHA[:7], commitTime.Format(time.RFC3339))

	// Transcript has commands, but NO git commit command
	steps := []harness.Step{
		{Index: 0, Kind: harness.StepKindUser, Text: "read only session"},
		{Index: 1, Kind: harness.StepKindCommand, Command: "git status", Excerpt: "On branch main"},
	}

	header := &episode.Header{
		SessionID: "sess-no-commit-cmd",
		Branch:    "main",
		Started:   sessionStart,
		Ended:     sessionEnd,
	}

	linker := episode.NewCommitLinker(episode.WithGitRunner(mock))
	commits, err := linker.Link("/fake/repo", header, steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(commits) != 0 {
		t.Errorf("expected 0 commits confirmed, got %v", commits)
	}
	if header.HasCommits() {
		t.Error("header should not have commits recorded")
	}
}

func TestCommitLinkerRejectsFailedCommitCommands(t *testing.T) {
	mock := newMockGitRunner()
	sessionStart := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	sessionEnd := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	commitTime := time.Date(2026, 10, 2, 7, 30, 0, 0, time.UTC)

	fullSHA := "4444444444444444444444444444444444444444"
	mock.responses["log"] = fmt.Sprintf("%s\x1f%s\x1f%s\x1fcommit subject", fullSHA, fullSHA[:7], commitTime.Format(time.RFC3339))

	exitOne := 1
	steps := []harness.Step{
		{
			Index:    1,
			Kind:     harness.StepKindCommand,
			Command:  `git commit -m "commit subject"`,
			ExitCode: &exitOne,
			Excerpt:  "fatal: pathspec did not match any files",
		},
	}

	header := &episode.Header{
		SessionID: "sess-failed-cmd",
		Branch:    "main",
		Started:   sessionStart,
		Ended:     sessionEnd,
	}

	linker := episode.NewCommitLinker(episode.WithGitRunner(mock))
	commits, err := linker.Link("/fake/repo", header, steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(commits) != 0 {
		t.Errorf("expected 0 commits for failed command, got %v", commits)
	}
}

func TestCommitLinkerTimeWindowFiltering(t *testing.T) {
	mock := newMockGitRunner()
	sessionStart := time.Date(2026, 10, 2, 7, 0, 0, 0, time.UTC)
	sessionEnd := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)

	// Commit happened yesterday (outside session window)
	oldTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	oldSHA := "5555555555555555555555555555555555555555"

	mock.responses["log"] = fmt.Sprintf("%s\x1f%s\x1f%s\x1fold commit", oldSHA, oldSHA[:7], oldTime.Format(time.RFC3339))

	exitZero := 0
	steps := []harness.Step{
		{
			Index:    1,
			Kind:     harness.StepKindCommand,
			Command:  `git commit -m "old commit"`,
			ExitCode: &exitZero,
		},
	}

	header := &episode.Header{
		SessionID: "sess-outside-window",
		Branch:    "main",
		Started:   sessionStart,
		Ended:     sessionEnd,
	}

	linker := episode.NewCommitLinker(episode.WithGitRunner(mock), episode.WithTimeMargin(10*time.Second))
	commits, err := linker.Link("/fake/repo", header, steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(commits) != 0 {
		t.Errorf("expected 0 commits because commit is outside time window, got %v", commits)
	}
}

func TestCommitLinkerErrors(t *testing.T) {
	// Nil header
	linker := episode.NewCommitLinker()
	if _, err := linker.Link(".", nil, nil); err == nil {
		t.Error("expected error for nil header")
	}

	// Nil episode
	if err := linker.LinkEpisode(".", nil); err == nil {
		t.Error("expected error for nil episode")
	}

	// Non-git repo
	mock := newMockGitRunner()
	mock.errs["log"] = fmt.Errorf("fatal: not a git repository (or any of the parent directories)")
	mock.responses["log"] = "fatal: not a git repository"

	errLinker := episode.NewCommitLinker(episode.WithGitRunner(mock))
	h := &episode.Header{SessionID: "sess-not-git", Branch: "main"}
	_, err := errLinker.Link("/not/a/git/repo", h, nil)
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected not a git repository error, got %v", err)
	}

	// Empty repo (no commits yet)
	mockEmpty := newMockGitRunner()
	mockEmpty.errs["log"] = fmt.Errorf("fatal: your current branch 'main' does not have any commits yet")
	mockEmpty.responses["log"] = "fatal: your current branch 'main' does not have any commits yet"

	emptyLinker := episode.NewCommitLinker(episode.WithGitRunner(mockEmpty))
	hEmpty := &episode.Header{SessionID: "sess-empty", Branch: "main"}
	commits, err := emptyLinker.Link("/empty/repo", hEmpty, nil)
	if err != nil {
		t.Errorf("expected nil error for empty repo, got %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits for empty repo, got %v", commits)
	}
}

// Integration test using real git commands in a temporary repository
func TestCommitLinkerIntegrationWithRealGit(t *testing.T) {
	tmp := t.TempDir()

	runGit := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", tmp}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git command failed: git %s: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init")
	runGit("config", "user.email", "worker@cblabs.org")
	runGit("config", "user.name", "Worker Agent")
	runGit("config", "commit.gpgsign", "false")

	// Create branch
	branchName := "test/commit-linkage"
	runGit("checkout", "-b", branchName)

	sessionStart := time.Now().UTC().Add(-5 * time.Second)

	// File 1
	file1 := "service.go"
	if err := os.WriteFile(filepath.Join(tmp, file1), []byte("package service\n"), 0644); err != nil {
		t.Fatalf("writing file1: %v", err)
	}
	runGit("add", file1)
	msg1 := "feat(service): implement core service"
	commitOut1 := runGit("commit", "-m", msg1)
	sha1 := runGit("rev-parse", "HEAD")

	// File 2
	file2 := "service_test.go"
	if err := os.WriteFile(filepath.Join(tmp, file2), []byte("package service_test\n"), 0644); err != nil {
		t.Fatalf("writing file2: %v", err)
	}
	runGit("add", file2)
	msg2 := "test(service): add unit tests"
	commitOut2 := runGit("commit", "-m", msg2)
	sha2 := runGit("rev-parse", "HEAD")

	sessionEnd := time.Now().UTC().Add(5 * time.Second)

	exitZero := 0
	steps := []harness.Step{
		{
			Index:    0,
			Kind:     harness.StepKindCommand,
			Command:  fmt.Sprintf(`git commit -m "%s"`, msg1),
			ExitCode: &exitZero,
			Excerpt:  commitOut1,
		},
		{
			Index: 1,
			Kind:  harness.StepKindEdit,
			File:  file1,
		},
		{
			Index:    2,
			Kind:     harness.StepKindCommand,
			Command:  fmt.Sprintf(`git commit -m "%s"`, msg2),
			ExitCode: &exitZero,
			Excerpt:  commitOut2,
		},
		{
			Index: 3,
			Kind:  harness.StepKindEdit,
			File:  file2,
		},
	}

	ep := episode.NewEpisode(
		episode.Header{
			SessionID: "sess-real-git",
			Branch:    branchName,
			Started:   sessionStart,
			Ended:     sessionEnd,
		},
		steps...,
	)

	// Call LinkCommits
	if err := episode.LinkCommits(tmp, ep); err != nil {
		t.Fatalf("LinkCommits failed: %v", err)
	}

	if len(ep.Header.Commits) != 2 {
		t.Fatalf("expected 2 linked commits, got %d (%v)", len(ep.Header.Commits), ep.Header.Commits)
	}
	if ep.Header.Commits[0] != sha1 {
		t.Errorf("expected commit 0 to be %s, got %s", sha1, ep.Header.Commits[0])
	}
	if ep.Header.Commits[1] != sha2 {
		t.Errorf("expected commit 1 to be %s, got %s", sha2, ep.Header.Commits[1])
	}
	if ep.Header.HeadEnd != sha2 {
		t.Errorf("expected HeadEnd %s, got %s", sha2, ep.Header.HeadEnd)
	}

	// Verify edit steps were resolved to commits
	if ep.Steps[1].Commit != sha1 {
		t.Errorf("expected step 1 (file1) commit to be resolved to %s, got %s", sha1, ep.Steps[1].Commit)
	}
	if ep.Steps[3].Commit != sha2 {
		t.Errorf("expected step 3 (file2) commit to be resolved to %s, got %s", sha2, ep.Steps[3].Commit)
	}

	// Also verify LinkHeaderCommits helper
	headerCopy := ep.Header
	headerCopy.Commits = nil
	confirmed, err := episode.LinkHeaderCommits(tmp, &headerCopy, steps)
	if err != nil {
		t.Fatalf("LinkHeaderCommits failed: %v", err)
	}
	if len(confirmed) != 2 || confirmed[0] != sha1 || confirmed[1] != sha2 {
		t.Errorf("unexpected confirmed hashes from LinkHeaderCommits: %v", confirmed)
	}
}
