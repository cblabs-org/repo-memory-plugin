package gitops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitBranchAndHead(t *testing.T) {
	tempDir := t.TempDir()
	runGitCmd(t, tempDir, "init", "-b", "feature-test")

	// In an empty repo without commits
	branch, err := GetCurrentBranch(tempDir)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed on empty repo: %v", err)
	}
	if branch != "feature-test" {
		t.Errorf("expected branch feature-test, got %q", branch)
	}

	head, err := GetHeadCommit(tempDir)
	if err == nil && head != "" {
		t.Errorf("expected empty/error head on empty repo, got %q", head)
	}

	// Create a commit
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	runGitCmd(t, tempDir, "add", "test.txt")
	runGitCmd(t, tempDir, "commit", "-m", "initial commit")

	branch, err = GetCurrentBranch(tempDir)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "feature-test" {
		t.Errorf("expected branch feature-test, got %q", branch)
	}

	head, err = GetHeadCommit(tempDir)
	if err != nil {
		t.Fatalf("GetHeadCommit failed: %v", err)
	}
	if len(head) != 40 {
		t.Errorf("expected 40-char SHA head commit, got %q", head)
	}

	// Test detached HEAD
	runGitCmd(t, tempDir, "checkout", head)
	detachedBranch, err := GetCurrentBranch(tempDir)
	if err != nil {
		t.Fatalf("GetCurrentBranch on detached head failed: %v", err)
	}
	if detachedBranch != "" {
		t.Errorf("expected empty branch on detached HEAD, got %q", detachedBranch)
	}

	detachedHead, err := GetHeadCommit(tempDir)
	if err != nil {
		t.Fatalf("GetHeadCommit on detached head failed: %v", err)
	}
	if detachedHead != head {
		t.Errorf("expected head %q, got %q", head, detachedHead)
	}
}
