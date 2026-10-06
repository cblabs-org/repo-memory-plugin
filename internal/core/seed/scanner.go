package seed

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

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

func SeedFromWorkorders(paDir string, opsWriter *ops.Writer, resolver *config.ScopeResolver) error {
	return SeedWithTimeCut(paDir, opsWriter, resolver, "")
}

func SeedWithTimeCut(paDir string, opsWriter *ops.Writer, resolver *config.ScopeResolver, beforeCommit string) error {
	files, err := ScanWorkorders(paDir)
	if err != nil {
		return err
	}

	seededOps := 0
	workordersSeeded := 0

	for _, path := range files {
		if beforeCommit != "" {
			// Use git to check if the workorder file existed at the given commit
			cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", beforeCommit, path))
			if err := cmd.Run(); err != nil {
				// file didn't exist at that commit or other error, skip
				continue
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		report, err := parseCompletionReportWithPath(string(content), path)
		if err != nil || report == nil {
			continue
		}

		newOps := ReportToOps(report, resolver)
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
