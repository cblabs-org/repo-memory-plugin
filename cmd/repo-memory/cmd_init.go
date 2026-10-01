package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Enroll a repository for memory tracking",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}
		return RunInit(cmd, targetDir)
	},
}

const mapIncludeLine = "@[memory map](.memory/views/MAP.md)"

// RunInit executes repository enrolment.
func RunInit(cmd *cobra.Command, targetDir string) error {
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	// 1. Check if .memory/ directory exists before init (template warning)
	memDir := filepath.Join(repoRoot, ".memory")
	if fi, err := os.Stat(memDir); err == nil && fi.IsDir() {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: .memory/ directory already exists (inherited from template?)\n")
	}

	// 2. Create .memory/ directory and .memory/.gitignore (ignoring .cache/)
	if err := config.EnsureRepoLayout(repoRoot); err != nil {
		return fmt.Errorf("ensuring repo layout: %w", err)
	}

	// 3. Append MAP include line to GEMINI.md if missing or create GEMINI.md
	geminiPath := filepath.Join(repoRoot, "GEMINI.md")
	if err := ensureMapIncludeInGemini(geminiPath); err != nil {
		return fmt.Errorf("updating GEMINI.md: %w", err)
	}

	// 4. Generate Repo ID
	repoID, err := gitops.GenerateRepoID(repoRoot)
	if err != nil {
		return fmt.Errorf("generating repo ID: %w", err)
	}

	// 5. Ensure data-dir layout for this repository
	dataDir := config.DataDir()
	if err := config.EnsureDataDirLayout(dataDir, repoID); err != nil {
		return fmt.Errorf("ensuring data dir layout: %w", err)
	}

	// 6. Add to <data-dir>/registry.json
	worktrees, _ := gitops.GetWorktreePaths(repoRoot)
	workspacePaths := append([]string{repoRoot}, worktrees...)
	if err := config.AddToRegistry(dataDir, workspacePaths, repoID, repoRoot); err != nil {
		return fmt.Errorf("adding to registry: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Enrolled repository %s (repo-id: %s)\n", repoRoot, repoID)
	return nil
}

func ensureMapIncludeInGemini(geminiPath string) error {
	data, err := os.ReadFile(geminiPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create GEMINI.md with MAP include line
			return os.WriteFile(geminiPath, []byte(mapIncludeLine+"\n"), 0644)
		}
		return err
	}

	content := string(data)
	if strings.Contains(content, mapIncludeLine) || strings.Contains(content, ".memory/views/MAP.md") {
		// Already included
		return nil
	}

	// Append MAP include line
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += mapIncludeLine + "\n"

	return os.WriteFile(geminiPath, []byte(content), 0644)
}
