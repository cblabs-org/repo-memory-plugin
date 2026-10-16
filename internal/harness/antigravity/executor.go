package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

type Housekeeper struct {
	DataDir string
	RepoID  string
	Model   string
	Timeout string
}

func NewHousekeeper(dataDir, repoID, model, timeout string) *Housekeeper {
	if model == "" {
		model = "Gemini 3.8 Flash (High)"
	}
	if timeout == "" {
		timeout = "5m"
	}
	return &Housekeeper{
		DataDir: dataDir,
		RepoID:  repoID,
		Model:   model,
		Timeout: timeout,
	}
}

// Ensure it implements the interface
var _ harness.Housekeeper = (*Housekeeper)(nil)

type streamResult struct {
	Event  string `json:"event"`
	Result struct {
		Status   string `json:"status"`
		Response string `json:"response"`
		Error    string `json:"error"`
		Usage    struct {
			InputTokens     int `json:"input_tokens"`
			OutputTokens    int `json:"output_tokens"`
			ThinkingTokens  int `json:"thinking_tokens"`
			CacheReadTokens int `json:"cache_read_tokens"`
		} `json:"usage"`
	} `json:"result"`
}

func (h *Housekeeper) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	workDir := filepath.Join(h.DataDir, "runs", h.RepoID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return harness.Result{}, fmt.Errorf("failed to create work dir: %w", err)
	}

	projectArg := fmt.Sprintf("repo-memory-%s", h.RepoID)

	var prompt string
	if stage == harness.StageDistill || stage == harness.StageConsolidate {
		if s, ok := input.(string); ok {
			prompt = s
		} else {
			inputBytes, err := json.Marshal(input)
			if err != nil {
				return harness.Result{}, fmt.Errorf("failed to marshal input: %w", err)
			}
			prompt = string(inputBytes)
		}
	} else {
		// Prepare input string
		inputBytes, err := json.Marshal(input)
		if err != nil {
			return harness.Result{}, fmt.Errorf("failed to marshal input: %w", err)
		}

		var schemaGuide string
		switch stage {
		case harness.StageReconcile:
			schemaGuide = "Return strictly JSON in the form:\n{\n  \"actions\": [\n    {\"op\": \"create|support|refine|contradict|supersede|promote|discard\", \"candidate\": ..., \"target_id\": \"clm-...\", \"rationale\": \"...\"}\n  ]\n}"
		case harness.StageVerify:
			schemaGuide = "Return strictly JSON in the form:\n{\n  \"consistent\": true,\n  \"rationale\": \"...\"\n}"
		case harness.StageTriage:
			schemaGuide = "Return strictly JSON in the form:\n{\n  \"decisions\": [\n    {\"id\": \"...\", \"keep\": true, \"reason\": \"...\"}\n  ]\n}"
		}

		prompt = fmt.Sprintf("You are executing the %s pipeline stage. DO NOT call any tools. %s\n\nInput:\n%s", stage, schemaGuide, string(inputBytes))
	}

	args := []string{
		"-p", prompt,
		"--model", h.Model,
		"--output-format", "stream-json",
		"--print-timeout", h.Timeout,
		"--project", projectArg,
		"--dangerously-skip-permissions",
	}
	if stage == harness.StageDistill || stage == harness.StageConsolidate {
		args = append(args, "--agent", "repo-memory-distiller")
	}

	cmd := exec.CommandContext(ctx, "agy", args...)

	cmd.Env = append(os.Environ(), "REPO_MEMORY_HOUSEKEEPING=1", "REPO_MEMORY_INTERNAL=1")
	cmd.Dir = workDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			return harness.Result{}, harness.ErrQuotaExhausted
		}
		return harness.Result{}, fmt.Errorf("agy execution failed: %v\nStderr: %s", err, stderr.String())
	}

	var result harness.Result
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var msg streamResult
		if err := decoder.Decode(&msg); err != nil {
			return result, fmt.Errorf("failed to decode stream-json: %w", err)
		}
		if msg.Event == "result" {
			if msg.Result.Status == "ERROR" {
				return result, fmt.Errorf("agy returned error: %s", msg.Result.Error)
			}
			payload := msg.Result.Response

			if stage != harness.StageDistill && stage != harness.StageConsolidate {
				// Find the first { and last }
				first := strings.Index(payload, "{")
				last := strings.LastIndex(payload, "}")
				if first != -1 && last != -1 && last >= first {
					payload = payload[first : last+1]
				}
			}

			result.Payload = strings.TrimSpace(payload)
			result.Usage = harness.Usage{
				InputTokens:     msg.Result.Usage.InputTokens,
				OutputTokens:    msg.Result.Usage.OutputTokens,
				ThinkingTokens:  msg.Result.Usage.ThinkingTokens,
				CacheReadTokens: msg.Result.Usage.CacheReadTokens,
			}
		}
	}

	if result.Payload == "" {
		return result, fmt.Errorf("agy returned empty payload. Stdout: %s\nStderr: %s", stdout.String(), stderr.String())
	}
	return result, nil
}

