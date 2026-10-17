/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/eval/x6_replay.go
 * Description: Implements the X6 work-order replay harness for comparing agent
 *              execution across no-memory, Principal Architect document, and
 *              dossier memory conditions.
 *
 * Exported Types / Functions:
 * - Condition:          Experimental memory condition identifier for X6 replay runs.
 * - ConditionNoMemory:  Replay condition with repository memory disabled.
 * - ConditionPADocs:    Replay condition seeded with Principal Architect documents.
 * - ConditionDossiers:  Replay condition seeded with time-cut repository dossiers.
 * - ReplayResult:       Outcome metrics (status, steps, token usage) from a single replay run.
 * - ReplayHarness:      Orchestrates git checkout, condition injection, and headless worker execution.
 *   - NewReplayHarness(): Constructs a ReplayHarness for a repository root and data directory.
 *   - CheckoutParent():   Checks out the parent commit of the commit that introduced a work order.
 *   - InjectCondition():  Configures workspace memory state for the specified Condition.
 *   - RunWorker():        Executes the headless agy worker on a work order prompt.
 */

package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

// Condition identifies an experimental memory configuration for X6 replay runs.
type Condition string

const (
	ConditionNoMemory Condition = "no_memory"    // Disables repository memory for the replay run.
	ConditionPADocs   Condition = "pa_documents" // Injects time-cut Principal Architect workspace documents.
	ConditionDossiers Condition = "dossiers"     // Injects time-cut repository memory dossiers.
)

// ReplayResult captures the pass/fail status, step count, and token spend of a single replay run.
type ReplayResult struct {
	WorkOrder    string `json:"work_order"`      // Identifier or path of the replayed work order.
	Condition    string `json:"condition"`       // Experimental Condition under which the worker ran.
	Status       string `json:"status"`          // Execution outcome ("pass" or "fail").
	StepsTaken   int    `json:"steps_taken"`     // Number of agent steps recorded during execution.
	TotalTokens  int    `json:"total_tokens"`    // Sum of input and output tokens consumed.
	ErrorMessage string `json:"error,omitempty"` // Failure diagnostic when Status is "fail".
}

// ReplayHarness manages git state, memory condition setup, and worker invocation for X6 replays.
type ReplayHarness struct {
	RepoRoot string // Absolute path to the target git repository under evaluation.
	DataDir  string // Root data directory for replay runs and dossier storage.
}

// NewReplayHarness constructs a ReplayHarness for repoRoot and dataDir.
func NewReplayHarness(repoRoot, dataDir string) *ReplayHarness {
	return &ReplayHarness{
		RepoRoot: repoRoot,
		DataDir:  dataDir,
	}
}

// CheckoutParent checks out the parent commit of the commit that first added woPath.
func (r *ReplayHarness) CheckoutParent(ctx context.Context, woPath string) error {
	// --- Step 1: Locate the commit hash that introduced woPath ---
	cmd := exec.CommandContext(ctx, "git", "-C", r.RepoRoot, "log", "--diff-filter=A", "--format=%H", "-1", "--", woPath)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("finding commit for %s: %w", woPath, err)
	}

	commit := strings.TrimSpace(string(out))
	if commit == "" {
		return fmt.Errorf("could not find commit where %s was added", woPath)
	}

	// --- Step 2: Check out the parent commit (commit~1) ---
	checkoutCmd := exec.CommandContext(ctx, "git", "-C", r.RepoRoot, "checkout", commit+"~1")
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checking out parent of %s: %w, %s", commit, err, out)
	}
	return nil
}

// InjectCondition configures the workspace based on the requested condition.
func (r *ReplayHarness) InjectCondition(condition Condition, timeCutDocsPath, timeCutDossiersPath string) error {
	// --- Step 1: Toggle repo enrollment and copy condition-specific artifacts ---
	switch condition {
	case ConditionNoMemory:
		return config.SetRepoEnabled(r.RepoRoot, false)
	case ConditionPADocs:
		if err := config.SetRepoEnabled(r.RepoRoot, true); err != nil {
			return err
		}
		targetDir := filepath.Join(r.RepoRoot, ".agents", "skills", "principal-architect-workspace", "memory")
		return copyDir(timeCutDocsPath, targetDir)
	case ConditionDossiers:
		if err := config.SetRepoEnabled(r.RepoRoot, true); err != nil {
			return err
		}
		targetDir := filepath.Join(r.DataDir, "dossiers")
		return copyDir(timeCutDossiersPath, targetDir)
	default:
		return fmt.Errorf("unknown condition: %s", condition)
	}
}

// RunWorker executes the headless executor on the Work Order text.
func (r *ReplayHarness) RunWorker(ctx context.Context, woText string, repoID string) (ReplayResult, error) {
	// --- Step 1: Prepare isolated run working directory and CLI command ---
	workDir := filepath.Join(r.DataDir, "runs", repoID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return ReplayResult{}, fmt.Errorf("failed to create work dir: %w", err)
	}
	projectArg := fmt.Sprintf("repo-memory-%s", repoID)

	cmd := exec.CommandContext(ctx, "agy",
		"--agent", "Execution-Worker",
		"-p", woText,
		"--output-format", "json",
		"--project", projectArg,
		"--dangerously-skip-permissions",
	)

	cmd.Env = append(os.Environ(), "REPO_MEMORY_HOUSEKEEPING=1")
	cmd.Dir = workDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// --- Step 2: Execute the worker process and record pass/fail status ---
	err := cmd.Run()

	res := ReplayResult{}
	if err != nil {
		res.Status = "fail"
		res.ErrorMessage = err.Error() + ": " + stderr.String()
	} else {
		res.Status = "pass"
	}

	// --- Step 3: Parse JSON output from stdout to extract token usage and step count ---
	var outMap map[string]interface{}
	if jsonErr := json.Unmarshal(stdout.Bytes(), &outMap); jsonErr == nil {
		if usage, ok := outMap["usage"].(map[string]interface{}); ok {
			inTok := int(usage["input_tokens"].(float64))
			outTok := int(usage["output_tokens"].(float64))
			res.TotalTokens = inTok + outTok
		}
		if steps, ok := outMap["steps"].([]interface{}); ok {
			res.StepsTaken = len(steps)
		}
	}

	return res, nil
}

// copyDir recursively copies the contents of src into dst using cp -R.
func copyDir(src string, dst string) error {
	if src == "" || dst == "" {
		return nil
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	cmd := exec.Command("cp", "-R", src+"/", dst)
	return cmd.Run()
}
