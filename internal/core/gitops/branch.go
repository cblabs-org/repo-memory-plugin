package gitops

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// GetCurrentBranch returns the active branch name for the repository at repoRoot.
// Returns an empty string if in detached HEAD state or unable to determine branch.
func GetCurrentBranch(repoRoot string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return GetCurrentBranchWithContext(ctx, repoRoot)
}

// GetCurrentBranchWithContext returns the active branch name using the provided context.
func GetCurrentBranchWithContext(ctx context.Context, repoRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "branch", "--show-current")
	out, err := cmd.Output()
	if err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" {
			return b, nil
		}
	}

	// Fallback: rev-parse --abbrev-ref HEAD
	cmd = exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	out, err = cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		if ref != "HEAD" && ref != "" {
			return ref, nil
		}
	}

	return "", nil
}

// GetHeadCommit returns the current HEAD commit hash for the repository at repoRoot.
func GetHeadCommit(repoRoot string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return GetHeadCommitWithContext(ctx, repoRoot)
}

// GetHeadCommitWithContext returns the HEAD commit hash using the provided context.
func GetHeadCommitWithContext(ctx context.Context, repoRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
