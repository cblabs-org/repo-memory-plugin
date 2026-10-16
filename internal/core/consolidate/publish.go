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

// RecordRepoTier records validOps into the shared repository Tier 5 directory (<repoRoot>/.memory)
// and renders its dossiers and ATLAS.md.
func RecordRepoTier(ctx context.Context, repoRoot string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error) {
	if err := EnsureMemoryGitignore(repoRoot); err != nil {
		return nil, err
	}
	return RecordAndRenderTier(ctx, RepoTierBaseDir(repoRoot), validOps, inboxStore, consolidatedNoteIDs, now)
}

// RecordBranchTier records validOps into the gitignored Tier 4 branch-local lane
// (.memory/local/ops/<branch-slug>/ and .memory/local/dossiers/<branch-slug>/).
func RecordBranchTier(ctx context.Context, repoRoot, branchSlug string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error) {
	if err := EnsureMemoryGitignore(repoRoot); err != nil {
		return nil, err
	}

	opsDir := BranchTierOpsDir(repoRoot, branchSlug)
	dossiersDir := BranchTierDossiersDir(repoRoot, branchSlug)

	res, err := RecordAndRenderDirs(ctx, opsDir, dossiersDir, validOps, inboxStore, consolidatedNoteIDs, now)
	if err != nil {
		return nil, err
	}

	// Also render <scope>.md and <scope>.history.md directly under BranchTierDossiersDir(repoRoot, branchSlug)
	// so callers inspecting .memory/local/dossiers/<branch-slug>/<scope>.md or via dossiers.NewWriter both succeed.
	if err := writeDirectDossierFiles(dossiersDir, res.Dossiers); err != nil {
		return nil, fmt.Errorf("writing direct branch dossiers to %s: %w", dossiersDir, err)
	}

	return res, nil
}

func writeDirectDossierFiles(dossiersDir string, projected map[string]*dossiers.Dossier) error {
	for _, d := range projected {
		if d == nil || (len(d.Entries) == 0 && len(d.History) == 0) {
			continue
		}
		scopeRel := filepath.FromSlash(d.Scope)
		scopePath := filepath.Join(dossiersDir, scopeRel+".md")
		historyPath := filepath.Join(dossiersDir, scopeRel+".history.md")

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
	PromotedBranches []string      `json:"promoted_branches,omitempty"`
	PromotedBatches  []ops.OpBatch `json:"promoted_batches,omitempty"`
}

// PromoteMergedBranchOps scans <repoRoot>/.memory/local/ops/ for branch-scoped op batches whose feature
// branch has merged into the default branch, promotes those batch files into <repoRoot>/.memory/ops/,
// deletes the promoted .memory/local/{ops,dossiers}/<branch-slug>/ directories, and re-renders .memory/dossiers/ and .memory/ATLAS.md.
func PromoteMergedBranchOps(ctx context.Context, repoRoot string) (*BranchPromotionResult, error) {
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

	if len(res.PromotedBatches) > 0 {
		if _, err := RecordAndRenderTier(ctx, RepoTierBaseDir(repoRoot), nil, nil, nil, time.Now().UTC()); err != nil {
			return nil, fmt.Errorf("re-rendering shared dossiers after branch promotion: %w", err)
		}
	}

	return res, nil
}

type branchBatchFile struct {
	relPath  string
	rawBytes []byte
}

func readAllBranchBatches(ctx context.Context, branchOpsDir string) ([]ops.OpBatch, []branchBatchFile, error) {
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

func isBranchReadyForPromotion(ctx context.Context, repoRoot, branchSlug string, batches []ops.OpBatch) bool {
	currentBranch, _ := gitops.GetCurrentBranchWithContext(ctx, repoRoot)
	defaultBranch := resolveDefaultBranch(ctx, repoRoot)

	onDefault := IsDefaultBranch(currentBranch) ||
		(defaultBranch != "" && strings.EqualFold(strings.TrimSpace(currentBranch), defaultBranch))
	if !onDefault {
		return false
	}

	targetSlug := SanitizeBranchSlug(branchSlug)

	// 1. Check git branch --merged HEAD for a merged branch matching targetSlug.
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

	// 2. Fallback if the feature branch ref was deleted after merge: verify the branch ref does not exist
	// as an unmerged local branch and all file anchors referenced in batches now exist on disk under repoRoot.
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

func refExists(ctx context.Context, repoRoot, ref string) bool {
	if strings.TrimSpace(ref) == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", "--quiet", ref)
	return cmd.Run() == nil
}

// DefaultBranchMergedChecker returns a function that checks whether a given branch
// has been merged into main, master, or HEAD.
func DefaultBranchMergedChecker(repoRoot string) func(string) bool {
	return func(branch string) bool {
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

func extractBatchFileAnchors(batches []ops.OpBatch) []string {
	seen := make(map[string]bool)
	var out []string
	addAnchor := func(raw string) {
		p := StripAnchorSymbol(raw)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

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

	return idx.RebuildFromOpLogs(ctx, readers...)
}
