/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/gitops/repoid.go
 * Description: Normalizes Git remote URLs, resolves repository roots and active
 *              worktrees, and computes deterministic SHA-256 repository IDs.
 *
 * Exported Types / Functions:
 * - CanonicalURL():          Normalizes a Git remote URL into canonical https://host/owner/repo form.
 * - ComputeRepoID():         Hashes a root commit and remote URL or worktree path with SHA-256.
 * - GetRootCommit():         Returns the oldest root commit hash in the repository.
 * - GetCanonicalOriginURL(): Returns the normalized origin remote URL for a repository if present.
 * - GetMainWorktreePath():   Resolves the real filesystem path of the repository's main worktree.
 * - GetWorktreePaths():      Returns all active worktree paths for the repository.
 * - FindRepoRoot():          Locates the root directory of the repository containing startDir.
 * - GenerateRepoID():        Generates the canonical SHA-256 Repo ID for a repository root.
 */

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
// - Lowercases the host component.
func CanonicalURL(raw string) string {
	// --- Step 1: Trim whitespace, trailing slashes, and .git suffix ---
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.TrimSuffix(raw, "/")
	raw = strings.TrimSuffix(raw, ".git")

	// --- Step 2: Normalize scp-style [user@]host:path syntax ---
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

	// --- Step 3: Normalize standard URL schemes (https://, ssh://, git://) ---
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		host := strings.ToLower(u.Hostname())
		repoPath := strings.TrimPrefix(u.Path, "/")
		repoPath = strings.TrimSuffix(repoPath, "/")
		repoPath = strings.TrimSuffix(repoPath, ".git")
		return "https://" + host + "/" + repoPath
	}

	return raw
}

// ComputeRepoID hashes the root commit and remote URL/worktree real path using SHA-256:
// sha256(root commit + ":" + canonical origin URL or real path).
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
	// --- Step 1: Query remote.origin.url via git config with git remote fallback ---
	cmd := exec.Command("git", "-C", repoRoot, "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		// Fallback to git remote get-url origin.
		cmd2 := exec.Command("git", "-C", repoRoot, "remote", "get-url", "origin")
		output2, err2 := cmd2.Output()
		if err2 != nil || len(strings.TrimSpace(string(output2))) == 0 {
			return "", nil
		}
		output = output2
	}

	// --- Step 2: Normalize raw remote URL into canonical form ---
	canonical := CanonicalURL(strings.TrimSpace(string(output)))
	return canonical, nil
}

// GetMainWorktreePath resolves the real filesystem path of the main worktree.
func GetMainWorktreePath(repoRoot string) (string, error) {
	// --- Step 1: Inspect git worktree list --porcelain for the primary worktree entry ---
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

	// --- Step 2: Fall back to git rev-parse --show-toplevel or symlink-resolved repoRoot ---
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
	// --- Step 1: Run git worktree list --porcelain ---
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		// Not a git worktree or error; return repoRoot.
		return []string{repoRoot}, nil
	}

	// --- Step 2: Parse worktree lines and resolve symlinks ---
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
	// --- Step 1: Resolve startDir to an absolute path and query git rev-parse --show-toplevel ---
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

	// --- Step 2: Walk ancestor directories looking for .git or .memory markers ---
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
	// --- Step 1: Resolve oldest root commit and canonical origin URL ---
	rootCommit, err := GetRootCommit(repoRoot)
	if err != nil {
		// If git repo has no commits yet or git command fails, use empty root commit.
		rootCommit = ""
	}

	originURL, err := GetCanonicalOriginURL(repoRoot)
	if err == nil && originURL != "" {
		return ComputeRepoID(rootCommit, originURL), nil
	}

	// --- Step 2: Fall back to symlink-resolved main worktree path when no origin remote is configured ---
	mainWorktree, err := GetMainWorktreePath(repoRoot)
	if err != nil {
		mainWorktree = repoRoot
	}
	if real, err := filepath.EvalSymlinks(mainWorktree); err == nil {
		mainWorktree = real
	}
	return ComputeRepoID(rootCommit, mainWorktree), nil
}
