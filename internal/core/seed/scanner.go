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

type SeedOptions struct {
	RepoRoot     string
	BeforeCommit string
	OpsDir       string
}

func ScanWorkorders(paDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(paDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files/dirs we don't have access to
			if os.IsNotExist(err) || os.IsPermission(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() && filepath.Base(path) == "WORKORDER.md" {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), "## Completion Report") {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}

func SeedFromWorkorders(paDir string, opsWriter *ops.Writer, resolver *config.ScopeResolver, opts SeedOptions) error {
	parsed, err := ScanAndParse(paDir, opts)
	if err != nil {
		return err
	}
	return SeedFromParsed(parsed, opsWriter, resolver, opts)
}

type ParsedWorkorder struct {
	Path       string
	RelPath    string
	Report     *CompletionReport
	CommitTime time.Time
}

func buildRepoPathValidator(opts SeedOptions) func(string) bool {
	if opts.RepoRoot == "" {
		return func(string) bool { return true }
	}

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

func ScanAndParse(paDir string, opts SeedOptions) ([]*ParsedWorkorder, error) {
	files, err := ScanWorkorders(paDir)
	if err != nil {
		return nil, err
	}

	isValidRepoPath := buildRepoPathValidator(opts)

	var results []*ParsedWorkorder
	for _, path := range files {
		var content []byte
		relPath := path
		if opts.RepoRoot != "" {
			var err error
			relPath, err = filepath.Rel(opts.RepoRoot, path)
			if err != nil {
				return nil, fmt.Errorf("cannot relativize %s against %s: %w", path, opts.RepoRoot, err)
			}
		}

		if opts.BeforeCommit != "" {
			cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", opts.BeforeCommit, relPath))
			if opts.RepoRoot != "" {
				cmd.Dir = opts.RepoRoot
			}
			out, err := cmd.Output()
			if err != nil {
				continue
			}
			content = out
		} else {
			var err error
			content, err = os.ReadFile(path)
			if err != nil {
				return nil, err
			}
		}

		report, err := parseCompletionReportWithPath(string(content), path)
		if err != nil || report == nil {
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
		out, err := cmd.Output()
		if err == nil {
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
			if err == nil {
				commitTime = t
			}
		}

		results = append(results, &ParsedWorkorder{
			Path:       path,
			RelPath:    relPath,
			Report:     report,
			CommitTime: commitTime,
		})
	}
	return results, nil
}

func SeedFromParsed(parsed []*ParsedWorkorder, opsWriter *ops.Writer, resolver *config.ScopeResolver, opts SeedOptions) error {
	seededOps := 0
	workordersSeeded := 0

	var existingIDs = make(map[string]bool)
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

	for _, pw := range parsed {
		newOps := ReportToOps(pw.Report, resolver, pw.RelPath, pw.CommitTime, existingIDs)
		if len(newOps) > 0 {
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
