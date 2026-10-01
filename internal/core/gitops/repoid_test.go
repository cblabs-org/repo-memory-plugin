package gitops

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ssh scp style with git extension",
			input:    "git@github.com:cblabs-org/repo-memory-plugin.git",
			expected: "https://github.com/cblabs-org/repo-memory-plugin",
		},
		{
			name:     "https with git extension and trailing slash",
			input:    "https://github.com/cblabs-org/repo-memory-plugin.git/",
			expected: "https://github.com/cblabs-org/repo-memory-plugin",
		},
		{
			name:     "ssh url scheme",
			input:    "ssh://git@github.com/cblabs-org/repo-memory-plugin.git",
			expected: "https://github.com/cblabs-org/repo-memory-plugin",
		},
		{
			name:     "uppercase host",
			input:    "https://GITHUB.COM/cblabs-org/repo-memory-plugin.git",
			expected: "https://github.com/cblabs-org/repo-memory-plugin",
		},
		{
			name:     "gitlab subgroup",
			input:    "git@gitlab.com:group/subgroup/project.git",
			expected: "https://gitlab.com/group/subgroup/project",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CanonicalURL(tc.input)
			if got != tc.expected {
				t.Errorf("CanonicalURL(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestComputeRepoID(t *testing.T) {
	rootCommit := "abc1234567890abcdef1234567890abcdef1234"
	remoteURL := "https://github.com/cblabs-org/repo-memory-plugin"

	expectedSum := sha256.Sum256([]byte(rootCommit + ":" + remoteURL))
	expected := hex.EncodeToString(expectedSum[:])

	got := ComputeRepoID(rootCommit, remoteURL)
	if got != expected {
		t.Errorf("ComputeRepoID() = %q; want %q", got, expected)
	}
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git command failed (%v): %s", err, string(out))
	}
}

func TestGenerateRepoID_WithRemote(t *testing.T) {
	tmp := t.TempDir()

	runGitCmd(t, tmp, "init")
	runGitCmd(t, tmp, "config", "user.name", "Test User")
	runGitCmd(t, tmp, "config", "user.email", "test@example.com")
	runGitCmd(t, tmp, "commit", "--allow-empty", "-m", "initial root commit")
	runGitCmd(t, tmp, "remote", "add", "origin", "git@github.com:cblabs-org/repo-memory-plugin.git")

	rootCommit, err := GetRootCommit(tmp)
	if err != nil {
		t.Fatalf("GetRootCommit failed: %v", err)
	}
	if len(rootCommit) != 40 {
		t.Errorf("expected 40-char commit hash, got %q", rootCommit)
	}

	repoID, err := GenerateRepoID(tmp)
	if err != nil {
		t.Fatalf("GenerateRepoID failed: %v", err)
	}

	expectedID := ComputeRepoID(rootCommit, "https://github.com/cblabs-org/repo-memory-plugin")
	if repoID != expectedID {
		t.Errorf("GenerateRepoID() = %q; want %q", repoID, expectedID)
	}

	// Make another commit, ensure root commit remains identical and repoID remains stable
	runGitCmd(t, tmp, "commit", "--allow-empty", "-m", "second commit")
	repoID2, err := GenerateRepoID(tmp)
	if err != nil {
		t.Fatalf("second GenerateRepoID failed: %v", err)
	}
	if repoID2 != repoID {
		t.Errorf("RepoID changed after second commit: got %q, want %q", repoID2, repoID)
	}
}

func TestGenerateRepoID_NoRemote(t *testing.T) {
	tmp := t.TempDir()

	runGitCmd(t, tmp, "init")
	runGitCmd(t, tmp, "config", "user.name", "Test User")
	runGitCmd(t, tmp, "config", "user.email", "test@example.com")
	runGitCmd(t, tmp, "commit", "--allow-empty", "-m", "initial root commit")

	rootCommit, err := GetRootCommit(tmp)
	if err != nil {
		t.Fatalf("GetRootCommit failed: %v", err)
	}

	realTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		realTmp = tmp
	}

	repoID, err := GenerateRepoID(tmp)
	if err != nil {
		t.Fatalf("GenerateRepoID failed: %v", err)
	}

	expectedID := ComputeRepoID(rootCommit, realTmp)
	if repoID != expectedID {
		t.Errorf("GenerateRepoID without remote = %q; want %q", repoID, expectedID)
	}
}

func TestFindRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	runGitCmd(t, tmp, "init")

	subDir := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	root, err := FindRepoRoot(subDir)
	if err != nil {
		t.Fatalf("FindRepoRoot failed: %v", err)
	}

	realTmp, _ := filepath.EvalSymlinks(tmp)
	realRoot, _ := filepath.EvalSymlinks(root)
	if realRoot != realTmp {
		t.Errorf("FindRepoRoot(%q) = %q; want %q", subDir, realRoot, realTmp)
	}
}
