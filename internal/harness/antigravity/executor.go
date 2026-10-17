/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/executor.go
 * Description: Implements the headless Antigravity CLI (agy) Housekeeper and
 *              MultiTurnHousekeeper for executing single-turn and warm multi-turn
 *              stream-json LLM housekeeping stages.
 *
 * Exported Types / Functions:
 * - Housekeeper: Executes single-turn and multi-turn LLM pipeline stages via agy.
 *
 *   Lifecycle:
 *   - NewHousekeeper(): Constructs a Housekeeper configured with dataDir, repoID, model, and timeout.
 *
 *   Housekeeper (harness.Housekeeper):
 *   - Run(): Executes a single pipeline stage invocation via agy --output-format stream-json.
 *
 *   MultiTurnHousekeeper (harness.MultiTurnHousekeeper):
 *   - RunMultiTurn(): Executes multiple sequential prompts in a single warm stream-json session.
 */

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

// Housekeeper executes single-turn and warm multi-turn LLM pipeline stages via the agy CLI.
type Housekeeper struct {
	DataDir string // Root data directory under which isolated runs/<RepoID> directories are created.
	RepoID  string // Enrolled repository identifier used to scope the agy project and run directory.
	Model   string // Model identifier passed to agy --model.
	Timeout string // Print timeout duration string passed to agy --print-timeout.
}

// NewHousekeeper constructs a Housekeeper for dataDir and repoID, applying default model and timeout values when empty.
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

// _ verifies at compile time that Housekeeper satisfies harness.Housekeeper.
var _ harness.Housekeeper = (*Housekeeper)(nil)

// _ verifies at compile time that Housekeeper satisfies harness.MultiTurnHousekeeper.
var _ harness.MultiTurnHousekeeper = (*Housekeeper)(nil)

// streamInputUserEvent represents a single user turn event written to stdin in stream-json mode.
type streamInputUserEvent struct {
	Event   string `json:"event"` // Event type identifier (always "user").
	Message struct {
		Content string `json:"content"` // Prompt text for the turn.
	} `json:"message"` // Envelope containing the user turn prompt text sent to agy stdin.
}

// streamResult represents a decoded JSON event emitted on stdout by agy in stream-json mode.
type streamResult struct {
	Event  string `json:"event"` // Stream event type (e.g., "result").
	Result struct {
		Status   string `json:"status"`   // Execution status (e.g., "OK" or "ERROR").
		Response string `json:"response"` // Model response text for the turn.
		Error    string `json:"error"`    // Error description when Status is "ERROR".
		Usage    struct {
			InputTokens     int `json:"input_tokens"`      // Cumulative input tokens consumed.
			OutputTokens    int `json:"output_tokens"`     // Cumulative output tokens generated.
			ThinkingTokens  int `json:"thinking_tokens"`   // Cumulative thinking tokens consumed.
			CacheReadTokens int `json:"cache_read_tokens"` // Cumulative cache-read tokens.
		} `json:"usage"` // Token usage metrics reported for the turn.
	} `json:"result"` // Nested result payload when Event is "result".
}

// Run executes a single pipeline stage via the agy CLI and returns its output payload and token usage.
// It returns harness.ErrQuotaExhausted (discoverable via errors.Is) if agy exits with code 3.
func (h *Housekeeper) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	// --- Step 1: Prepare isolated working directory and format stage prompt ---
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

	// --- Step 2: Configure and execute the agy subprocess ---
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

	// --- Step 3: Decode stream-json output and extract the result payload and usage ---
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

// RunMultiTurn executes prompts sequentially in a single warm stream-json agy session
// and returns per-turn results with delta token usage.
// If the multi-turn agy process exits with code 3, RunMultiTurn returns harness.ErrQuotaExhausted.
func (h *Housekeeper) RunMultiTurn(ctx context.Context, stage harness.Stage, prompts []string) ([]harness.Result, error) {
	// --- Step 1: Validate prompts and encode stream-json user events into stdin buffer ---
	if len(prompts) == 0 {
		return nil, nil
	}

	workDir := filepath.Join(h.DataDir, "runs", h.RepoID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work dir: %w", err)
	}

	projectArg := fmt.Sprintf("repo-memory-%s", h.RepoID)

	var stdinBuf bytes.Buffer
	for _, prompt := range prompts {
		var evt streamInputUserEvent
		evt.Event = "user"
		evt.Message.Content = prompt
		line, err := json.Marshal(evt)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal stream-json user event: %w", err)
		}
		stdinBuf.Write(line)
		stdinBuf.WriteByte('\n')
	}

	// --- Step 2: Configure and run the multi-turn agy subprocess ---
	args := []string{
		"-p", "",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--model", h.Model,
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
	cmd.Stdin = &stdinBuf

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			return nil, harness.ErrQuotaExhausted
		}
		return nil, fmt.Errorf("agy multi-turn execution failed: %v\nStderr: %s", err, stderr.String())
	}

	// --- Step 3: Decode per-turn result events and compute incremental token usage ---
	var results []harness.Result
	var prevUsage harness.Usage

	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var msg streamResult
		if err := decoder.Decode(&msg); err != nil {
			return results, fmt.Errorf("failed to decode stream-json: %w", err)
		}
		if msg.Event == "result" {
			if msg.Result.Status == "ERROR" {
				return results, fmt.Errorf("agy returned error: %s", msg.Result.Error)
			}

			payload := strings.TrimSpace(msg.Result.Response)
			cur := harness.Usage{
				InputTokens:     msg.Result.Usage.InputTokens,
				OutputTokens:    msg.Result.Usage.OutputTokens,
				ThinkingTokens:  msg.Result.Usage.ThinkingTokens,
				CacheReadTokens: msg.Result.Usage.CacheReadTokens,
			}

			turnUsage := cur
			if len(results) > 0 && cur.InputTokens >= prevUsage.InputTokens && cur.OutputTokens >= prevUsage.OutputTokens {
				turnUsage = harness.Usage{
					InputTokens:     cur.InputTokens - prevUsage.InputTokens,
					OutputTokens:    cur.OutputTokens - prevUsage.OutputTokens,
					ThinkingTokens:  max(0, cur.ThinkingTokens-prevUsage.ThinkingTokens),
					CacheReadTokens: max(0, cur.CacheReadTokens-prevUsage.CacheReadTokens),
				}
			}
			prevUsage = cur

			results = append(results, harness.Result{
				Payload: payload,
				Usage:   turnUsage,
			})
		}
	}

	if len(results) != len(prompts) {
		return results, fmt.Errorf("agy multi-turn returned %d results for %d prompts. Stdout: %s\nStderr: %s", len(results), len(prompts), stdout.String(), stderr.String())
	}

	return results, nil
}


