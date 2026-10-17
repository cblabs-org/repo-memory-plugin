/*
 * repo-memory-plugin (c) 2026 CBLabs
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/seed/scanner.go
 * Description: Walks Principal Architect workspace directories (.agents/plugins/ or
 *              .agents/skills/), parses WORKORDER.md and PROJECT.md reports, validates
 *              repository paths against Git trees, and writes deduplicated seed ops.
 *
 * Exported Types / Functions:
 * - SeedOptions:                  Configures repository root, time-cut commit, and ops output directory.
 * - ParsedWorkorder:              Holds a parsed completion report alongside its path and Git commit time.
 * - ScanWorkorders():             Discovers WORKORDER.md files containing completion reports.
 * - ScanProjectsWithDistillation(): Discovers PROJECT.md files containing architectural distillation blocks.
 * - ScanAndParse():               Scans, parses, and validates WORKORDER.md and PROJECT.md reports.
 * - SeedFromWorkorders():         End-to-end scan, parse, and op-log emission for a PA directory.
 * - SeedFromParsed():             Writes deduplicated seed ops for pre-parsed completion reports.
 */

package seed

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// SeedOptions configures a Principal Architect seeding run.
type SeedOptions struct {
	RepoRoot     string // Absolute or relative path to the target Git repository root.
	BeforeCommit string // Optional Git commit SHA/ref for time-cut historical seeding.
	OpsDir       string // Path to .memory/ops/ for deduplication against existing op batches.
}

// ParsedWorkorder pairs a parsed CompletionReport with its source path and Git commit timestamp.
type ParsedWorkorder struct {
	Path       string            // Filesystem path to WORKORDER.md or PROJECT.md.
	RelPath    string            // Repository-relative path used for deterministic ID hashing.
	Report     *CompletionReport // Normalized decisions, dead ends, findings, and file changes.
	CommitTime time.Time         // Commit timestamp of the report file in Git history.
}

// ScanWorkorders walks paDir and returns all WORKORDER.md files containing a completion report.
func ScanWorkorders(paDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(paDir, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) || os.IsPermission(walkErr) {
				return nil
			}
			return walkErr
		}
		if !info.IsDir() && filepath.Base(filePath) == "WORKORDER.md" {
			content, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return readErr
			}
			s := string(content)
			if strings.Contains(s, "## Completion Report") || strings.Contains(s, "\ncompletion:") {
				files = append(files, filePath)
			}
		}
		return nil
	})
	return files, err
}

// ScanProjectsWithDistillation walks paDir and returns PROJECT.md files that contain
// Part III architectural distillation blocks ("Key Architectural Decisions").
func ScanProjectsWithDistillation(paDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(paDir, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) || os.IsPermission(walkErr) {
				return nil
			}
			return walkErr
		}
		if !info.IsDir() && filepath.Base(filePath) == "PROJECT.md" {
			content, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return readErr
			}
			if strings.Contains(string(content), "Key Architectural Decisions") {
				files = append(files, filePath)
			}
		}
		return nil
	})
	return files, err
}

// SeedFromWorkorders scans paDir and emits deduplicated seed operations via opsWriter.
func SeedFromWorkorders(paDir string, opsWriter *ops.Writer, resolver *config.ScopeResolver, opts SeedOptions) error {
	parsed, err := ScanAndParse(paDir, opts)
	if err != nil {
		return err
	}
	return SeedFromParsed(parsed, opsWriter, resolver, opts)
}

// buildRepoPathValidator constructs a predicate that verifies whether a candidate anchor
// path exists in the repository tree (at opts.BeforeCommit when specified, or on disk).
func buildRepoPathValidator(opts SeedOptions) func(string) bool {
	if opts.RepoRoot == "" {
		return func(string) bool { return true }
	}
	setActiveRepoSymbolIndex(opts.RepoRoot)

	// --- Step 1: Build historical tree lookup when BeforeCommit is set ---
	if opts.BeforeCommit != "" {
		cmd := exec.Command("git", "ls-tree", "-r", "--name-only", opts.BeforeCommit)
		cmd.Dir = opts.RepoRoot
		if out, err := cmd.Output(); err == nil {
			known := make(map[string]bool)
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				clean := filepath.Clean(line)
				known[clean] = true
				for dir := filepath.Dir(clean); dir != "." && dir != "/" && dir != ""; dir = filepath.Dir(dir) {
					if known[dir] {
						break
					}
					known[dir] = true
				}
			}
			return func(p string) bool {
				filePart, ok := stripAnchorSymbol(p)
				if !ok {
					return false
				}
				clean := filepath.Clean(strings.TrimSuffix(filePart, "/"))
				if clean == "" || clean == "." || strings.HasPrefix(clean, "..") {
					return false
				}
				return known[clean]
			}
		}
	}

	// --- Step 2: Validate against current working tree on disk ---
	return func(p string) bool {
		filePart, ok := stripAnchorSymbol(p)
		if !ok {
			return false
		}
		clean := filepath.Clean(strings.TrimSuffix(filePart, "/"))
		if clean == "" || clean == "." || strings.HasPrefix(clean, "..") {
			return false
		}
		_, err := os.Stat(filepath.Join(opts.RepoRoot, clean))
		return err == nil
	}
}

// ScanAndParse discovers and parses WORKORDER.md and PROJECT.md reports under paDir,
// filtering file anchors against the repository tree and resolving Git commit timestamps.
func ScanAndParse(paDir string, opts SeedOptions) ([]*ParsedWorkorder, error) {
	// --- Step 1: Discover candidate WORKORDER.md and PROJECT.md files ---
	woFiles, err := ScanWorkorders(paDir)
	if err != nil {
		return nil, err
	}
	projFiles, err := ScanProjectsWithDistillation(paDir)
	if err != nil {
		return nil, err
	}

	files := append([]string{}, woFiles...)
	files = append(files, projFiles...)

	isValidRepoPath := buildRepoPathValidator(opts)

	// --- Step 2: Read, parse, validate anchors, and resolve commit times ---
	var results []*ParsedWorkorder
	for _, filePath := range files {
		var content []byte
		relPath := filePath
		if opts.RepoRoot != "" {
			var relErr error
			relPath, relErr = filepath.Rel(opts.RepoRoot, filePath)
			if relErr != nil {
				return nil, fmt.Errorf("cannot relativize %s against %s: %w", filePath, opts.RepoRoot, relErr)
			}
		}

		if opts.BeforeCommit != "" {
			cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", opts.BeforeCommit, relPath))
			if opts.RepoRoot != "" {
				cmd.Dir = opts.RepoRoot
			}
			out, showErr := cmd.Output()
			if showErr != nil {
				continue
			}
			content = out
		} else {
			var readErr error
			content, readErr = os.ReadFile(filePath)
			if readErr != nil {
				return nil, readErr
			}
		}

		var report *CompletionReport
		var parseErr error
		if filepath.Base(filePath) == "PROJECT.md" {
			report, parseErr = parseProjectReportWithPath(string(content), filePath)
		} else {
			report, parseErr = parseCompletionReportWithPath(string(content), filePath)
		}
		if parseErr != nil || report == nil {
			continue
		}

		if opts.RepoRoot != "" && len(report.Changes) > 0 {
			filtered := report.Changes[:0]
			for _, c := range report.Changes {
				if isValidRepoPath(c.Path) {
					filtered = append(filtered, c)
				}
			}
			report.Changes = filtered
		}

		var commitTime time.Time
		var cmd *exec.Cmd
		if opts.BeforeCommit != "" {
			cmd = exec.Command("git", "log", "-1", "--format=%cI", opts.BeforeCommit, "--", relPath)
		} else {
			cmd = exec.Command("git", "log", "-1", "--format=%cI", "--", relPath)
		}
		if opts.RepoRoot != "" {
			cmd.Dir = opts.RepoRoot
		}
		if out, logErr := cmd.Output(); logErr == nil {
			if t, timeErr := time.Parse(time.RFC3339, strings.TrimSpace(string(out))); timeErr == nil {
				commitTime = t
			}
		}

		results = append(results, &ParsedWorkorder{
			Path:       filePath,
			RelPath:    relPath,
			Report:     report,
			CommitTime: commitTime,
		})
	}
	return results, nil
}

// SeedFromParsed converts parsed workorder/project reports into ops and writes them to opsWriter.
func SeedFromParsed(parsed []*ParsedWorkorder, opsWriter *ops.Writer, resolver *config.ScopeResolver, opts SeedOptions) error {
	seededOps := 0
	workordersSeeded := 0

	// --- Step 1: Load existing entry IDs for idempotency ---
	existingIDs := make(map[string]bool)
	if opts.OpsDir != "" {
		reader := ops.NewReader(opts.OpsDir)
		it := ops.NewIterator(reader, context.Background())
		for batch, err := range it.Seq() {
			if err != nil {
				return fmt.Errorf("reading existing ops for dedup: %w", err)
			}
			for _, op := range batch.Ops {
				existingIDs[op.EntryID] = true
			}
		}
	}

	// --- Step 2: Convert each parsed report into op batches and persist ---
	for _, pw := range parsed {
		newOps := ReportToOps(pw.Report, resolver, pw.RelPath, pw.CommitTime, existingIDs)
		if len(newOps) > 0 {
			for _, op := range newOps {
				existingIDs[op.EntryID] = true
			}
			if opsWriter != nil {
				batch := ops.NewBatchWithTime(newOps, pw.CommitTime)
				if err := opsWriter.Write(batch); err != nil {
					return err
				}
			}
			seededOps += len(newOps)
			workordersSeeded++
		}
	}

	fmt.Printf("Seeded %d entries from %d workorders\n", seededOps, workordersSeeded)
	return nil
}
