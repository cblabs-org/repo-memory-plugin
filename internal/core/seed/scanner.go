package seed

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	return SeedWithTimeCut(paDir, opsWriter, resolver, opts)
}

type ParsedWorkorder struct {
	Path    string
	RelPath string
	Report  *CompletionReport
}

func ScanAndParse(paDir string, opts SeedOptions) ([]*ParsedWorkorder, error) {
	files, err := ScanWorkorders(paDir)
	if err != nil {
		return nil, err
	}

	var results []*ParsedWorkorder
	for _, path := range files {
		var content []byte
		relPath := path
		if opts.RepoRoot != "" {
			relPath, _ = filepath.Rel(opts.RepoRoot, path)
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

		results = append(results, &ParsedWorkorder{
			Path:    path,
			RelPath: relPath,
			Report:  report,
		})
	}
	return results, nil
}

func SeedWithTimeCut(paDir string, opsWriter *ops.Writer, resolver *config.ScopeResolver, opts SeedOptions) error {
	pws, err := ScanAndParse(paDir, opts)
	if err != nil {
		return err
	}

	seededOps := 0
	workordersSeeded := 0

	var existingIDs = make(map[string]bool)
	if opts.OpsDir != "" {
		reader := ops.NewReader(opts.OpsDir)
		it := ops.NewIterator(reader, context.Background())
		for batch, err := range it.Seq() {
			if err != nil {
				continue
			}
			for _, op := range batch.Ops {
				existingIDs[op.EntryID] = true
			}
		}
	}

	for _, pw := range pws {
		newOps := ReportToOps(pw.Report, resolver, pw.RelPath, existingIDs)
		if len(newOps) > 0 {
			if opsWriter != nil {
				batch := ops.NewBatch(newOps)
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

func ScanAllAnchors(paDir string, opts SeedOptions) ([]string, error) {
	pws, err := ScanAndParse(paDir, opts)
	if err != nil {
		return nil, err
	}

	var allAnchors []string
	for _, pw := range pws {
		for _, c := range pw.Report.Changes {
			allAnchors = append(allAnchors, c.Path)
		}
	}
	return allAnchors, nil
}
