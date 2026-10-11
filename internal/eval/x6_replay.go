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

type Condition string

const (
	ConditionNoMemory Condition = "no_memory"
	ConditionPADocs   Condition = "pa_documents"
	ConditionDossiers Condition = "dossiers"
)

type ReplayResult struct {
	WorkOrder    string `json:"work_order"`
	Condition    string `json:"condition"`
	Status       string `json:"status"` // pass/fail
	StepsTaken   int    `json:"steps_taken"`
	TotalTokens  int    `json:"total_tokens"`
	ErrorMessage string `json:"error,omitempty"`
}

type ReplayHarness struct {
	RepoRoot string
	DataDir  string
}

func NewReplayHarness(repoRoot, dataDir string) *ReplayHarness {
	return &ReplayHarness{
		RepoRoot: repoRoot,
		DataDir:  dataDir,
	}
}

// CheckoutParent checks out the parent commit of the specified work order commit.
func (r *ReplayHarness) CheckoutParent(ctx context.Context, woPath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", r.RepoRoot, "log", "--diff-filter=A", "--format=%H", "-1", "--", woPath)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("finding commit for %s: %w", woPath, err)
	}

	commit := strings.TrimSpace(string(out))
	if commit == "" {
		return fmt.Errorf("could not find commit where %s was added", woPath)
	}

	checkoutCmd := exec.CommandContext(ctx, "git", "-C", r.RepoRoot, "checkout", commit+"~1")
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checking out parent of %s: %w, %s", commit, err, out)
	}
	return nil
}

// InjectCondition configures the workspace based on the requested condition.
func (r *ReplayHarness) InjectCondition(condition Condition, timeCutDocsPath, timeCutDossiersPath string) error {
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
	
	err := cmd.Run()
	
	res := ReplayResult{}
	if err != nil {
		res.Status = "fail"
		res.ErrorMessage = err.Error() + ": " + stderr.String()
	} else {
		res.Status = "pass"
	}
	
	// We could parse JSON output from stdout to get tokens and steps
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
