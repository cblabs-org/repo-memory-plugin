/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_status.go
 * Description: Implements the `repo-memory status` CLI command and active
 *              dossier projection helper across repo-shared, branch-local, and
 *              user-local operation log tiers.
 *
 * Exported Types / Functions:
 * - StatusOutput: Structured summary of repository enrollment, freshness, and active memory counts.
 * - RunStatus():  Inspects repository memory status and emits human-readable or JSON output.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/coverage"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
)

var (
	statusRepoDir string // Optional --repo directory override for `repo-memory status`.
	statusJSON    bool   // When true, outputs repository status as structured JSON.
)

// StatusOutput represents the structured output of `repo-memory status`.
type StatusOutput struct {
	RepoRoot        string   `json:"repo_root"`         // Absolute path to the repository root.
	RepoID          string   `json:"repo_id,omitempty"` // Stable registry slug derived from remote origin URL or root path hash.
	Enrolled        bool     `json:"enrolled"`          // Whether the repository is actively enrolled.
	Freshness       string   `json:"freshness"`         // Single-line freshness status header.
	ActiveDossiers  int      `json:"active_dossiers"`   // Number of scopes with active dossier entries.
	ActiveEntries   int      `json:"active_entries"`    // Total active (non-retired) dossier entries.
	GraduatedSkills []string `json:"graduated_skills"`  // Slugs of graduated procedural skills.
}

// statusCmd defines the `repo-memory status` subcommand.
var statusCmd = &cobra.Command{
	Use:   "status [repo-dir]",
	Short: "Display current repository memory status",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if strings.TrimSpace(statusRepoDir) != "" {
			targetDir = statusRepoDir
		} else if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			targetDir = args[0]
		}
		return RunStatus(cmd, targetDir, statusJSON)
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output structured JSON result")
}

// RunStatus inspects repository memory status and writes human-readable or JSON output to cmd.OutOrStdout().
func RunStatus(cmd *cobra.Command, targetDir string, asJSON bool) error {
	// --- Step 1: Resolve repository root, registry ID, and enrollment status ---
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		if absDir, absErr := filepath.Abs(targetDir); absErr == nil {
			repoRoot = absDir
		} else {
			repoRoot = targetDir
		}
	}

	dataDir := config.DataDir()
	repoID := ""
	if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil {
		repoID = entry.RepoID
	}
	if repoID == "" {
		repoID, _ = gitops.GenerateRepoID(repoRoot)
	}

	enrolled := config.IsEnrolledFast(repoRoot)
	freshness := coverage.FormatFreshnessHeader(repoRoot, dataDir, repoID)

	// --- Step 2: Project active dossier entries and check graduated skills ---
	activeDossiers, activeEntries := loadProjectedActiveEntries(ctx, repoRoot, dataDir, repoID)

	var graduated []string
	if gradRes, err := skills.SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, true); err == nil && gradRes != nil {
		graduated = gradRes.Graduated
	}
	if graduated == nil {
		graduated = []string{}
	}

	out := StatusOutput{
		RepoRoot:        repoRoot,
		RepoID:          repoID,
		Enrolled:        enrolled,
		Freshness:       freshness,
		ActiveDossiers:  activeDossiers,
		ActiveEntries:   len(activeEntries),
		GraduatedSkills: graduated,
	}

	// --- Step 3: Render JSON or human-readable status summary ---
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintln(w, out.Freshness)
	fmt.Fprintf(w, "Repository: %s\n", out.RepoRoot)
	fmt.Fprintf(w, "Enrolled:   %v\n", out.Enrolled)
	fmt.Fprintf(w, "Dossiers:   %d active (%d active entries)\n", out.ActiveDossiers, out.ActiveEntries)
	fmt.Fprintf(w, "Skills:     %d graduated\n", len(out.GraduatedSkills))
	return nil
}

// loadProjectedActiveEntries replays operation logs across repo-shared (Tier 5),
// branch-local (Tier 4), and user-local (Tier 3) directories, returning the count
// of active scopes and the deduplicated slice of active entries.
func loadProjectedActiveEntries(ctx context.Context, repoRoot, dataDir, repoID string) (int, []dossiers.Entry) {
	// --- Step 1: Define per-directory projection closure ---
	byID := make(map[string]dossiers.Entry)
	retiredIDs := make(map[string]bool)
	var orderedIDs []string

	projectOpsDir := func(opsDir string) {
		info, err := os.Stat(opsDir)
		if err != nil || !info.IsDir() {
			return
		}
		projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
		if err != nil {
			return
		}
		for scopeKey, d := range projected {
			if d == nil {
				continue
			}
			for _, h := range d.History {
				retiredIDs[h.ID] = true
				delete(byID, h.ID)
			}
			for _, e := range d.Entries {
				if e.Retired || strings.TrimSpace(e.SupersededBy) != "" {
					retiredIDs[e.ID] = true
					delete(byID, e.ID)
					continue
				}
				if retiredIDs[e.ID] {
					continue
				}
				if strings.TrimSpace(e.DossierScope) == "" {
					if strings.TrimSpace(d.Scope) != "" {
						e.DossierScope = strings.TrimSpace(d.Scope)
					} else {
						e.DossierScope = strings.TrimSpace(scopeKey)
					}
				}
				if _, exists := byID[e.ID]; !exists {
					orderedIDs = append(orderedIDs, e.ID)
				}
				byID[e.ID] = e
			}
		}
	}

	// --- Step 2: Project Tier 5 (repo-shared), Tier 4 (branch-local), and Tier 3 (user-local) ops ---
	if strings.TrimSpace(repoRoot) != "" {
		// Tier 5: Repo-shared ops (.memory/ops)
		projectOpsDir(filepath.Join(repoRoot, ".memory", "ops"))

		// Tier 4: Branch-local ops (.memory/local/ops and .memory/local/ops/<branch>)
		localOpsRoot := filepath.Join(repoRoot, ".memory", "local", "ops")
		projectOpsDir(localOpsRoot)
		if dirEntries, err := os.ReadDir(localOpsRoot); err == nil {
			for _, de := range dirEntries {
				if !de.IsDir() {
					continue
				}
				if _, parseErr := time.Parse("2006-01", de.Name()); parseErr == nil {
					continue
				}
				projectOpsDir(filepath.Join(localOpsRoot, de.Name()))
			}
		}
	}

	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		// Tier 3: User-local ops
		projectOpsDir(filepath.Join(dataDir, "repos", repoID, "user-dossiers", "ops"))
		projectOpsDir(filepath.Join(dataDir, repoID, "user-dossiers", "ops"))
	}

	// --- Step 3: Collect active entries in deterministic order and count distinct scopes ---
	activeScopes := make(map[string]bool)
	activeEntries := make([]dossiers.Entry, 0, len(byID))
	for _, id := range orderedIDs {
		e, ok := byID[id]
		if !ok {
			continue
		}
		activeEntries = append(activeEntries, e)
		scope := strings.TrimSpace(e.DossierScope)
		if scope == "" {
			scope = "_default"
		}
		activeScopes[scope] = true
	}

	return len(activeScopes), activeEntries
}
