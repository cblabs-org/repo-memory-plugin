package gitops

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CanonicalURL normalizes a Git remote URL into a canonical form:
// - Strips trailing .git and trailing slashes
// - Converts SSH/scp-like syntax (git@host:owner/repo) to https://host/owner/repo
// - Normalizes schemes (ssh://, git://) to https://
// - Lowercases the host component
func CanonicalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.TrimSuffix(raw, "/")
	raw = strings.TrimSuffix(raw, ".git")

	// Match scp style: [user@]host:path
	if !strings.Contains(raw, "://") && strings.Contains(raw, ":") {
		parts := strings.SplitN(raw, ":", 2)
		hostPart := parts[0]
		pathPart := parts[1]
		if atIdx := strings.Index(hostPart, "@"); atIdx != -1 {
			hostPart = hostPart[atIdx+1:]
		}
		hostPart = strings.ToLower(hostPart)
		pathPart = strings.TrimPrefix(pathPart, "/")
		pathPart = strings.TrimSuffix(pathPart, "/")
		pathPart = strings.TrimSuffix(pathPart, ".git")
		return "https://" + hostPart + "/" + pathPart
	}

	// Match URL scheme: scheme://[user@]host[:port]/path
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		host := strings.ToLower(u.Hostname())
		path := strings.TrimPrefix(u.Path, "/")
		path = strings.TrimSuffix(path, "/")
		path = strings.TrimSuffix(path, ".git")
		return "https://" + host + "/" + path
	}

	return raw
}

// ComputeRepoID hashes the root commit and remote URL/worktree real path using SHA-256:
// sha256(root commit + ":" + canonical origin URL or real path)
func ComputeRepoID(rootCommit, remoteOrPath string) string {
	sum := sha256.Sum256([]byte(rootCommit + ":" + remoteOrPath))
	return hex.EncodeToString(sum[:])
}

// GetRootCommit returns the oldest root commit hash in the repository.
func GetRootCommit(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "rev-list", "--max-parents=0", "--reverse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", errors.New("no root commit found")
	}
	return strings.TrimSpace(lines[0]), nil
}

// GetCanonicalOriginURL returns the normalized origin remote URL if one exists.
func GetCanonicalOriginURL(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		// Fallback to git remote get-url origin
		cmd2 := exec.Command("git", "-C", repoRoot, "remote", "get-url", "origin")
		output2, err2 := cmd2.Output()
		if err2 != nil || len(strings.TrimSpace(string(output2))) == 0 {
			return "", nil
		}
		output = output2
	}

	canonical := CanonicalURL(strings.TrimSpace(string(output)))
	return canonical, nil
}

// GetMainWorktreePath resolves the real filesystem path of the main worktree.
func GetMainWorktreePath(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "worktree ") {
				p := strings.TrimPrefix(line, "worktree ")
				if real, err := filepath.EvalSymlinks(p); err == nil {
					return real, nil
				}
				return filepath.Clean(p), nil
			}
		}
	}

	// Fallback to show-toplevel
	cmdTop := exec.Command("git", "-C", repoRoot, "rev-parse", "--show-toplevel")
	if output, err := cmdTop.Output(); err == nil {
		p := strings.TrimSpace(string(output))
		if real, err := filepath.EvalSymlinks(p); err == nil {
			return real, nil
		}
		return filepath.Clean(p), nil
	}

	if real, err := filepath.EvalSymlinks(repoRoot); err == nil {
		return real, nil
	}
	return filepath.Clean(repoRoot), nil
}

// GetWorktreePaths returns all active worktree paths for the git repository.
func GetWorktreePaths(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		// Not a git worktree or error; return repoRoot
		return []string{repoRoot}, nil
	}

	var paths []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			p := strings.TrimPrefix(line, "worktree ")
			if real, err := filepath.EvalSymlinks(p); err == nil {
				paths = append(paths, real)
			} else {
				paths = append(paths, filepath.Clean(p))
			}
		}
	}
	if len(paths) == 0 {
		paths = append(paths, repoRoot)
	}
	return paths, nil
}

// FindRepoRoot determines the root directory of the repository containing startDir.
func FindRepoRoot(startDir string) (string, error) {
	if startDir == "" {
		startDir = "."
	}
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return startDir, err
	}

	cmd := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel")
	if output, err := cmd.Output(); err == nil {
		p := strings.TrimSpace(string(output))
		return filepath.Clean(p), nil
	}

	// Walk upwards looking for .git or .memory directory
	curr := abs
	for {
		if _, err := os.Stat(filepath.Join(curr, ".git")); err == nil {
			return curr, nil
		}
		if _, err := os.Stat(filepath.Join(curr, ".memory")); err == nil {
			return curr, nil
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return abs, nil
}

// GenerateRepoID generates the canonical Repo ID for the repository:
// sha256(root commit + ":" + canonical origin URL), falling back to main worktree real path if no remote URL exists.
func GenerateRepoID(repoRoot string) (string, error) {
	rootCommit, err := GetRootCommit(repoRoot)
	if err != nil {
		// If git repo has no commits yet or git command fails, use empty root commit
		rootCommit = ""
	}

	originURL, err := GetCanonicalOriginURL(repoRoot)
	if err == nil && originURL != "" {
		return ComputeRepoID(rootCommit, originURL), nil
	}

	mainWorktree, err := GetMainWorktreePath(repoRoot)
	if err != nil {
		mainWorktree = repoRoot
	}
	if real, err := filepath.EvalSymlinks(mainWorktree); err == nil {
		mainWorktree = real
	}
	return ComputeRepoID(rootCommit, mainWorktree), nil
}
