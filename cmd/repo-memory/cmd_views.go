package main

import (
	"fmt"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/views"
	"github.com/spf13/cobra"
)

var viewsCmd = &cobra.Command{
	Use:   "views [repoID]",
	Short: "Generate markdown views from claims",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoID := args[0]
		dataDir := config.DataDir()
		
		reg, err := config.LoadRegistry(dataDir)
		if err != nil {
			return err
		}
		
		var repoRoot string
		for _, entry := range reg {
			if entry.RepoID == repoID {
				repoRoot = entry.RootPath
				break
			}
		}
		if repoRoot == "" {
			return fmt.Errorf("repo %s not enrolled", repoID)
		}

		claimsDir := filepath.Join(repoRoot, ".memory", "claims")
		viewsDir := filepath.Join(repoRoot, ".memory", "views")
		
		gen, err := views.NewGenerator(claimsDir, viewsDir)
		if err != nil {
			return err
		}

		fmt.Printf("Generating views in %s...\n", viewsDir)
		if err := gen.Generate(); err != nil {
			return err
		}
		
		fmt.Println("Views generated successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(viewsCmd)
}
