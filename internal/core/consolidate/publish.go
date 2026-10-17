/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/consolidate/publish.go
 * Description: Manages multi-tier filesystem paths, user/branch/repo tier op
 *              recording, automatic promotion of merged feature-branch ops into
 *              shared repository memory, and multi-tier SQLite FTS5 index rebuilds.
 *
 * Exported Types / Functions:
 * - BranchPromotionResult:       Holds branches and op batches promoted from .memory/local/ops/ to .memory/ops/.
 * - SanitizeBranchSlug():        Normalizes a branch name into a filesystem-safe lowercase slug.
 * - EnsureMemoryGitignore():     Ensures <repoRoot>/.memory/.gitignore contains required local exclusions.
 * - UserTierBaseDir():           Returns the Tier 3 (user:) base directory (<dataDir>/<repoID>/user-dossiers).
 * - BranchTierOpsDir():          Returns the Tier 4 (branch:) ops directory.
 * - BranchTierDossiersDir():     Returns the Tier 4 (branch:) dossiers directory.
 * - RepoTierBaseDir():           Returns the Tier 5 (repo:) base directory (<repoRoot>/.memory).
 * - RecordUserTier():            Records validOps into the developer-local Tier 3 directory and renders dossiers.
 * - RecordRepoTier():            Records validOps into the shared repository Tier 5 directory and renders dossiers.
 * - RecordBranchTier():          Records validOps into the gitignored Tier 4 branch-local lane and renders dossiers.
 * - PromoteMergedBranchOps():    Promotes op batches from merged feature branches into shared .memory/ops/.
 * - DefaultBranchMergedChecker(): Returns a predicate that checks whether a branch is merged into the default branch.
 * - RebuildMultiTierIndex():     Rebuilds the SQLite FTS5 index from repo, active branch, and user tier op logs.
 */

package consolidate

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// nonAlnumSlugRe matches runs of non-alphanumeric characters replaced by hyphens in branch slugs.
var nonAlnumSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// SanitizeBranchSlug normalizes a branch name into a filesystem-safe lowercase slug
// (e.g. "feat/new-thing" -> "feat-new-thing").
func SanitizeBranchSlug(branch string) string {
	s := strings.ToLower(strings.TrimSpace(branch))
	s = strings.TrimPrefix(s, "refs/heads/")
	s = strings.TrimPrefix(s, "origin/")
	s = nonAlnumSlugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "default"
	}
	return s
}

// EnsureMemoryGitignore ensures <repoRoot>/.memory/.gitignore exists and contains
// .cache/, .enrolled, and local/.
func EnsureMemoryGitignore(repoRoot string) error {
	return config.EnsureMemoryGitignore(repoRoot)
}

// UserTierBaseDir returns the Tier 3 (user:) base directory (<dataDir>/<repoID>/user-dossiers).
func UserTierBaseDir(dataDir, repoID string) string {
	return filepath.Join(dataDir, repoID, "user-dossiers")
}

// BranchTierOpsDir returns the Tier 4 (branch:) ops directory (<repoRoot>/.memory/local/ops/<branch-slug>).
func BranchTierOpsDir(repoRoot, branchSlug string) string {
	return filepath.Join(repoRoot, ".memory", "local", "ops", SanitizeBranchSlug(branchSlug))
}

// BranchTierDossiersDir returns the Tier 4 (branch:) dossiers directory (<repoRoot>/.memory/local/dossiers/<branch-slug>).
func BranchTierDossiersDir(repoRoot, branchSlug string) string {
	return filepath.Join(repoRoot, ".memory", "local", "dossiers", SanitizeBranchSlug(branchSlug))
}

// RepoTierBaseDir returns the Tier 5 (repo:) base directory (<repoRoot>/.memory).
func RepoTierBaseDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".memory")
}

// RecordUserTier records validOps into the developer-local Tier 3 directory (<dataDir>/<repoID>/user-dossiers)
// and renders its dossiers and ATLAS.md.
func RecordUserTier(ctx context.Context, dataDir, repoID string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error) {
	return RecordAndRenderTier(ctx, UserTierBaseDir(dataDir, repoID), validOps, inboxStore, consolidatedNoteIDs, now)
}

// RecordRepoTier records validOps into the shared repository Tier 5 directory (<repoRoot>/.memory),
// ensuring .memory/.gitignore is initialized before projecting shared dossiers and ATLAS.md.
func RecordRepoTier(ctx context.Context, repoRoot string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error) {
	if err := EnsureMemoryGitignore(repoRoot); err != nil {
		return nil, err
	}
	return RecordAndRenderTier(ctx, RepoTierBaseDir(repoRoot), validOps, inboxStore, consolidatedNoteIDs, now)
}

// RecordBranchTier records validOps into the gitignored Tier 4 branch-local lane
// (.memory/local/ops/<branch-slug>/ and .memory/local/dossiers/<branch-slug>/).
func RecordBranchTier(ctx context.Context, repoRoot, branchSlug string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error) {
	// --- Step 1: Ensure .memory/.gitignore exists and resolve branch-tier directories ---
	if err := EnsureMemoryGitignore(repoRoot); err != nil {
		return nil, err
	}

	opsDir := BranchTierOpsDir(repoRoot, branchSlug)
	dossiersDir := BranchTierDossiersDir(repoRoot, branchSlug)

	// --- Step 2: Record the op batch and render projected dossiers ---
	res, err := RecordAndRenderDirs(ctx, opsDir, dossiersDir, validOps, inboxStore, consolidatedNoteIDs, now)
	if err != nil {
		return nil, err
	}

	// --- Step 3: Write direct <scope>.md and <scope>.history.md files under dossiersDir for direct layout compatibility ---
	if err := writeDirectDossierFiles(dossiersDir, res.Dossiers); err != nil {
		return nil, fmt.Errorf("writing direct branch dossiers to %s: %w", dossiersDir, err)
	}

	return res, nil
}

// writeDirectDossierFiles renders <scope>.md and <scope>.history.md directly under dossiersDir.
func writeDirectDossierFiles(dossiersDir string, projected map[string]*dossiers.Dossier) error {
	// --- Step 1: Iterate over projected dossiers and compute direct file paths ---
	for _, d := range projected {
		if d == nil || (len(d.Entries) == 0 && len(d.History) == 0) {
			continue
		}
		scopeRel := filepath.FromSlash(d.Scope)
		scopePath := filepath.Join(dossiersDir, scopeRel+".md")
		historyPath := filepath.Join(dossiersDir, scopeRel+".history.md")

		// --- Step 2: Write or remove the active dossier markdown file ---
		if len(d.Entries) == 0 {
			if err := os.Remove(scopePath); err != nil && !os.IsNotExist(err) {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(scopePath), 0o755); err != nil {
				return err
			}
			rendered := dossiers.RenderDossier(d)
			if err := os.WriteFile(scopePath, []byte(rendered), 0o644); err != nil {
				return err
			}
		}

		// --- Step 3: Write the companion history markdown file when history entries exist ---
		if len(d.History) > 0 {
			if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
				return err
			}
			renderedHistory := dossiers.RenderHistory(d)
			if err := os.WriteFile(historyPath, []byte(renderedHistory), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// BranchPromotionResult holds the branches and op batches promoted from .memory/local/ops/ to .memory/ops/.
type BranchPromotionResult struct {
	PromotedBranches []string      `json:"promoted_branches,omitempty"` // Sanitized branch slugs whose ops were promoted.
	PromotedBatches  []ops.OpBatch `json:"promoted_batches,omitempty"`  // OpBatch records moved into shared .memory/ops/.
}

// PromoteMergedBranchOps scans <repoRoot>/.memory/local/ops/ for branch-scoped op batches whose feature
// branch has merged into the default branch, promotes those batch files into <repoRoot>/.memory/ops/,
// deletes the promoted .memory/local/{ops,dossiers}/<branch-slug>/ directories, and re-renders .memory/dossiers/ and .memory/ATLAS.md.
func PromoteMergedBranchOps(ctx context.Context, repoRoot string) (*BranchPromotionResult, error) {
	// --- Step 1: Enumerate branch subdirectories under .memory/local/ops ---
	if ctx == nil {
		ctx = context.Background()
	}
	res := &BranchPromotionResult{}
	if strings.TrimSpace(repoRoot) == "" {
		return res, nil
	}

	localOpsRoot := filepath.Join(repoRoot, ".memory", "local", "ops")
	entries, err := os.ReadDir(localOpsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return nil, fmt.Errorf("reading local ops directory %s: %w", localOpsRoot, err)
	}

	var branchSlugs []string
	for _, e := range entries {
		if e.IsDir() {
			branchSlugs = append(branchSlugs, e.Name())
		}
	}
	sort.Strings(branchSlugs)

	// --- Step 2: Check each branch for merge readiness and promote batch files to .memory/ops ---
	for _, branchSlug := range branchSlugs {
		branchOpsDir := filepath.Join(localOpsRoot, branchSlug)
		batches, batchFiles, err := readAllBranchBatches(ctx, branchOpsDir)
		if err != nil {
			return nil, fmt.Errorf("reading branch ops in %s: %w", branchOpsDir, err)
		}
		if len(batches) == 0 {
			continue
		}

		if !isBranchReadyForPromotion(ctx, repoRoot, branchSlug, batches) {
			continue
		}

		repoOpsRoot := filepath.Join(repoRoot, ".memory", "ops")
		for _, bf := range batchFiles {
			dstPath := filepath.Join(repoOpsRoot, bf.relPath)
			if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
				return nil, fmt.Errorf("creating shared ops month directory %s: %w", filepath.Dir(dstPath), err)
			}
			if err := os.WriteFile(dstPath, bf.rawBytes, 0o644); err != nil {
				return nil, fmt.Errorf("promoting batch file to %s: %w", dstPath, err)
			}
		}

		// --- Step 3: Remove promoted branch-local ops and dossier directories ---
		if err := os.RemoveAll(branchOpsDir); err != nil {
			return nil, fmt.Errorf("removing promoted branch ops directory %s: %w", branchOpsDir, err)
		}
		branchDossiersDir := filepath.Join(repoRoot, ".memory", "local", "dossiers", branchSlug)
		if err := os.RemoveAll(branchDossiersDir); err != nil {
			return nil, fmt.Errorf("removing promoted branch dossiers directory %s: %w", branchDossiersDir, err)
		}

		res.PromotedBranches = append(res.PromotedBranches, branchSlug)
		res.PromotedBatches = append(res.PromotedBatches, batches...)
	}

	// --- Step 4: Re-render shared repository dossiers and ATLAS.md when batches were promoted ---
	if len(res.PromotedBatches) > 0 {
		if _, err := RecordAndRenderTier(ctx, RepoTierBaseDir(repoRoot), nil, nil, nil, time.Now().UTC()); err != nil {
			return nil, fmt.Errorf("re-rendering shared dossiers after branch promotion: %w", err)
		}
	}

	return res, nil
}

// branchBatchFile holds the relative path and raw JSON bytes of a branch-local op batch file.
type branchBatchFile struct {
	relPath  string // Path relative to the branch ops directory (e.g. "2026-10/01J....json").
	rawBytes []byte // Raw JSON file contents to copy into .memory/ops/.
}

// readAllBranchBatches loads all OpBatch records and raw JSON batch files from branchOpsDir.
func readAllBranchBatches(ctx context.Context, branchOpsDir string) ([]ops.OpBatch, []branchBatchFile, error) {
	// --- Step 1: Iterate parsed OpBatch records in chronological ULID order ---
	reader := ops.NewReader(branchOpsDir)
	var batches []ops.OpBatch
	for batch, err := range reader.Iterate(ctx) {
		if err != nil {
			return nil, nil, err
		}
		if batch != nil {
			batches = append(batches, *batch)
		}
	}

	// --- Step 2: Walk branchOpsDir to collect raw .json files for promotion ---
	var files []branchBatchFile
	err := filepath.WalkDir(branchOpsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		rel, err := filepath.Rel(branchOpsDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, branchBatchFile{
			relPath:  rel,
			rawBytes: data,
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return batches, files, nil
}

// isBranchReadyForPromotion reports whether repoRoot is currently on the default branch and
// branchSlug has either been merged into HEAD or deleted after its anchored files landed on disk.
func isBranchReadyForPromotion(ctx context.Context, repoRoot, branchSlug string, batches []ops.OpBatch) bool {
	// --- Step 1: Verify the working tree is currently on the default branch ---
	currentBranch, _ := gitops.GetCurrentBranchWithContext(ctx, repoRoot)
	defaultBranch := resolveDefaultBranch(ctx, repoRoot)

	onDefault := IsDefaultBranch(currentBranch) ||
		(defaultBranch != "" && strings.EqualFold(strings.TrimSpace(currentBranch), defaultBranch))
	if !onDefault {
		return false
	}

	targetSlug := SanitizeBranchSlug(branchSlug)

	// --- Step 2: Check `git branch --merged HEAD` for a merged branch matching targetSlug ---
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "branch", "--merged", "HEAD")
	if out, err := cmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			name := strings.TrimSpace(strings.TrimLeft(line, "*+ "))
			if name == "" || IsDefaultBranch(name) || strings.EqualFold(name, defaultBranch) {
				continue
			}
			if SanitizeBranchSlug(name) == targetSlug {
				return true
			}
		}
	}

	// --- Step 3: If the branch ref was deleted after merge, verify it is not an unmerged local branch and all anchored files exist ---
	allBranchesCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "branch", "--format=%(refname:short)")
	if out, err := allBranchesCmd.Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(line)
			if name != "" && SanitizeBranchSlug(name) == targetSlug {
				// The branch ref still exists locally and was NOT in `git branch --merged HEAD`.
				return false
			}
		}
	}

	anchors := extractBatchFileAnchors(batches)
	if len(anchors) == 0 {
		return false
	}
	for _, a := range anchors {
		fullPath := filepath.Join(repoRoot, filepath.FromSlash(a))
		if _, err := os.Stat(fullPath); err != nil {
			return false
		}
	}
	return true
}

// resolveDefaultBranch queries refs/remotes/origin/HEAD for the repository's default branch name, falling back to "main".
func resolveDefaultBranch(ctx context.Context, repoRoot string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if out, err := cmd.Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		ref = strings.TrimPrefix(ref, "origin/")
		if ref != "" {
			return ref
		}
	}
	return "main"
}

// refExists reports whether ref resolves to a valid git revision in repoRoot.
func refExists(ctx context.Context, repoRoot, ref string) bool {
	if strings.TrimSpace(ref) == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", "--quiet", ref)
	return cmd.Run() == nil
}

// DefaultBranchMergedChecker returns a function that checks whether a given branch
// has been merged into main, master, or HEAD.
func DefaultBranchMergedChecker(repoRoot string) func(branch string) bool {
	return func(branch string) bool {
		// --- Step 1: Normalize target branch name and handle default branches ---
		target := strings.TrimSpace(branch)
		if strings.TrimSpace(repoRoot) == "" || target == "" {
			return false
		}
		targetShort := strings.TrimPrefix(strings.TrimPrefix(target, "refs/heads/"), "origin/")
		if IsDefaultBranch(targetShort) {
			return true
		}
		targetSlug := SanitizeBranchSlug(targetShort)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// --- Step 2: Collect existing default base refs (origin default, main, master, trunk, HEAD) ---
		currentBranch, _ := gitops.GetCurrentBranchWithContext(ctx, repoRoot)
		currentShort := strings.TrimSpace(currentBranch)

		var baseRefs []string
		seenRef := make(map[string]bool)
		for _, candidate := range []string{resolveDefaultBranch(ctx, repoRoot), "main", "master", "trunk"} {
			c := strings.TrimSpace(candidate)
			if c == "" || seenRef[c] {
				continue
			}
			seenRef[c] = true
			if refExists(ctx, repoRoot, c) {
				baseRefs = append(baseRefs, c)
			}
		}

		if len(baseRefs) == 0 && currentShort != "" && !strings.EqualFold(currentShort, targetShort) && SanitizeBranchSlug(currentShort) != targetSlug {
			baseRefs = append(baseRefs, "HEAD")
		} else if len(baseRefs) > 0 && (IsDefaultBranch(currentShort) || currentShort == "") && !seenRef["HEAD"] {
			baseRefs = append(baseRefs, "HEAD")
		}

		// --- Step 3: Check `git branch --merged` and `git merge-base --is-ancestor` against each base ref ---
		for _, baseRef := range baseRefs {
			cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "branch", "--merged", baseRef)
			if out, err := cmd.Output(); err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					name := strings.TrimSpace(strings.TrimLeft(line, "*+ "))
					if name == "" {
						continue
					}
					if strings.EqualFold(name, targetShort) || SanitizeBranchSlug(name) == targetSlug {
						return true
					}
				}
			}
			if refExists(ctx, repoRoot, targetShort) {
				ancCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "merge-base", "--is-ancestor", targetShort, baseRef)
				if err := ancCmd.Run(); err == nil {
					return true
				}
			}
		}

		return false
	}
}

// extractBatchFileAnchors collects deduplicated normalized file paths from all "anchors" fields across batches.
func extractBatchFileAnchors(batches []ops.OpBatch) []string {
	// --- Step 1: Define deduplicating file-path collector ---
	seen := make(map[string]bool)
	var out []string
	addAnchor := func(raw string) {
		p := StripAnchorSymbol(raw)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	// --- Step 2: Scan all operations across batches for string or slice anchor fields ---
	for _, b := range batches {
		for _, op := range b.Ops {
			if op.Fields == nil {
				continue
			}
			switch raw := op.Fields["anchors"].(type) {
			case []string:
				for _, s := range raw {
					addAnchor(s)
				}
			case []any:
				for _, item := range raw {
					if s, ok := item.(string); ok {
						addAnchor(s)
					}
				}
			}
		}
	}
	return out
}

// RebuildMultiTierIndex rebuilds idx from all applicable tier op logs:
// 1. Shared repo: ops (<repoRoot>/.memory/ops),
// 2. Active branch: ops (<repoRoot>/.memory/local/ops/<SanitizeBranchSlug(currentBranch)>), and
// 3. Personal user: ops (<dataDir>/<repoID>/user-dossiers/ops).
func RebuildMultiTierIndex(ctx context.Context, idx *index.Index, dataDir, repoID, repoRoot, currentBranch string) error {
	// --- Step 1: Validate index handle and initialize default context ---
	if idx == nil {
		return fmt.Errorf("index cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var readers []*ops.Reader
	addReaderIfDirExists := func(dir string) {
		if strings.TrimSpace(dir) == "" {
			return
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			readers = append(readers, ops.NewReader(dir))
		}
	}

	// --- Step 2: Attach op-log readers for existing repo, active branch, and user tier directories ---
	if strings.TrimSpace(repoRoot) != "" {
		addReaderIfDirExists(filepath.Join(repoRoot, ".memory", "ops"))
		if strings.TrimSpace(currentBranch) != "" {
			addReaderIfDirExists(BranchTierOpsDir(repoRoot, currentBranch))
		}
	}
	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		addReaderIfDirExists(filepath.Join(UserTierBaseDir(dataDir, repoID), "ops"))
	}

	if len(readers) == 0 {
		return nil
	}

	// --- Step 3: Rebuild the SQLite FTS5 index from the combined op-log readers ---
	return idx.RebuildFromOpLogs(ctx, readers...)
}
