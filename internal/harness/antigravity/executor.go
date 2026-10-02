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

	// Prepare input string
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return harness.Result{}, fmt.Errorf("failed to marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, "agy",
		"-p", "-",
		"--model", h.Model,
		"--output-format", "stream-json",
		"--print-timeout", h.Timeout,
		"--project", projectArg,
		"--dangerously-skip-permissions",
	)

	cmd.Env = append(os.Environ(), "REPO_MEMORY_HOUSEKEEPING=1")
	cmd.Dir = workDir

	cmd.Stdin = bytes.NewReader(inputBytes)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
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
			// Strip markdown json block if present
			if strings.HasPrefix(payload, "```json") {
				payload = strings.TrimPrefix(payload, "```json")
				if strings.HasSuffix(strings.TrimSpace(payload), "```") {
					payload = strings.TrimSuffix(strings.TrimSpace(payload), "```")
				}
			} else if strings.HasPrefix(payload, "```") {
				payload = strings.TrimPrefix(payload, "```")
				if strings.HasSuffix(strings.TrimSpace(payload), "```") {
					payload = strings.TrimSuffix(strings.TrimSpace(payload), "```")
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
