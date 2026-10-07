package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/antigravity"
)

var (
	hookWorkspaceFlag     string
	hookProbeFlag         bool
	hookProbeDecisionFlag string
	probeTimeNow          = time.Now
)

var hookCmd = &cobra.Command{
	Use:   "hook <harness> <event>",
	Short: "Execute harness hook event path",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine the event name early for fail-open handling.
		eventName := ""
		if len(args) >= 2 {
			eventName = args[1]
		} else if len(args) == 1 {
			eventName = args[0]
		}
		isPreToolUse := strings.EqualFold(eventName, "PreToolUse") || strings.EqualFold(eventName, "pre_tool_use")

		// FAIL-OPEN: For PreToolUse, any panic or error must result in
		// {"decision":"allow"} on stdout and exit 0. A non-zero exit or
		// empty/invalid output causes an implicit deny that blocks ALL
		// tool calls across the entire machine.
		if isPreToolUse {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintln(cmd.OutOrStdout(), `{"decision":"allow","reason":"fail-open: panic recovery"}`)
				}
			}()
		}

		// Fast path for non-enrolled repositories / kill switch (target <= 5ms)
		var payload []byte
		if !hookProbeFlag {
			if os.Getenv("REPO_MEMORY_DISABLED") == "1" {
				if isPreToolUse {
					fmt.Fprintln(cmd.OutOrStdout(), `{"decision":"allow"}`)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), `{}`)
				}
				return nil
			}

			targetDir := hookWorkspaceFlag
			if targetDir == "" {
				payload, _ = io.ReadAll(cmd.InOrStdin())
				type fastPayload struct {
					WorkspacePaths      []string `json:"workspacePaths"`
					WorkspacePathsSnake []string `json:"workspace_paths"`
				}
				var parsed fastPayload
				if len(payload) > 0 {
					_ = json.Unmarshal(payload, &parsed)
				}
				if len(parsed.WorkspacePaths) > 0 && parsed.WorkspacePaths[0] != "" {
					targetDir = parsed.WorkspacePaths[0]
				} else if len(parsed.WorkspacePathsSnake) > 0 && parsed.WorkspacePathsSnake[0] != "" {
					targetDir = parsed.WorkspacePathsSnake[0]
				} else {
					targetDir, _ = os.Getwd()
				}
			}

			if !config.IsEnrolledFast(targetDir) {
				if isPreToolUse {
					fmt.Fprintln(cmd.OutOrStdout(), `{"decision":"allow"}`)
				} else if !strings.EqualFold(eventName, "PreInvocation") {
					fmt.Fprintln(cmd.OutOrStdout(), `{}`)
					return nil
				}
			}
		}

		// Wrap the real logic so we can catch errors and fail open.
		err := hookRunE(cmd, args, eventName, isPreToolUse, payload)
		if err != nil && isPreToolUse {
			// Fail open: return allow instead of propagating the error.
			fmt.Fprintln(cmd.OutOrStdout(), `{"decision":"allow","reason":"fail-open: `+strings.ReplaceAll(err.Error(), `"`, `'`)+`"}`)
			return nil
		}
		return err
	},
}

func hookRunE(cmd *cobra.Command, args []string, eventName string, isPreToolUse bool, preReadPayload []byte) error {
	if hookProbeFlag {
		event := "unknown"
		if len(args) >= 2 {
			event = args[1]
		} else if len(args) == 1 {
			if strings.EqualFold(args[0], "antigravity") || strings.EqualFold(args[0], "claudecode") || strings.EqualFold(args[0], "fake") {
				event = "unknown"
			} else {
				event = args[0]
			}
		}

		payload := preReadPayload
		if len(payload) == 0 {
			var err error
			payload, err = io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("reading hook payload: %w", err)
			}
		}

		out, err := handleProbe(event, payload)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}

	harnessName := ""
	if len(args) >= 2 {
		harnessName = args[0]
	}

	if (strings.EqualFold(harnessName, "antigravity") || harnessName == "") && strings.EqualFold(eventName, "stop") {
		payload := preReadPayload
		if len(payload) == 0 {
			var err error
			payload, err = io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("reading hook payload: %w", err)
			}
		}

		out, err := antigravity.HandleStop(cmd.Context(), payload, antigravity.WithWorkspace(hookWorkspaceFlag))
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}

	if (strings.EqualFold(harnessName, "antigravity") || harnessName == "") && strings.EqualFold(eventName, "PreInvocation") {
		payload := preReadPayload
		if len(payload) == 0 {
			var err error
			payload, err = io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("reading hook payload: %w", err)
			}
		}
		out, err := antigravity.HandlePreInvocation(cmd.Context(), payload, antigravity.WithWorkspace(hookWorkspaceFlag))
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "{}")
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}

	if (strings.EqualFold(harnessName, "antigravity") || harnessName == "") && strings.EqualFold(eventName, "PostToolUse") {
		// Read payload if not already read
		payload := preReadPayload
		if len(payload) == 0 {
			payload, _ = io.ReadAll(cmd.InOrStdin())
		}
		out, err := antigravity.HandlePostToolUse(cmd.Context(), payload, antigravity.WithWorkspace(hookWorkspaceFlag))
		if err != nil {
			fmt.Fprintln(cmd.OutOrStdout(), "{}")
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}

	valid, err := harness.FastPathValidation(hookWorkspaceFlag, "")
	if err != nil || !valid {
		// Sub-5ms fast-path: immediately return harness no-op output
		if isPreToolUse {
			fmt.Fprintln(cmd.OutOrStdout(), `{"decision":"allow"}`)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "{}")
		}
		return nil
	}

	if len(args) >= 2 {
		fmt.Fprintf(cmd.OutOrStdout(), "hook subcommand stub: harness=%s event=%s\n", args[0], args[1])
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "hook subcommand stub")
	}
	return nil
}

func init() {
	hookCmd.Flags().StringVar(&hookWorkspaceFlag, "workspace", "", "Workspace directory override for hook evaluation")
	hookCmd.Flags().BoolVar(&hookProbeFlag, "probe", false, "Enable hook probe recording")
	hookCmd.Flags().StringVar(&hookProbeDecisionFlag, "probe-decision", "allow", "Probe decision for PreToolUse (allow, deny, ask)")
}

func handleProbe(event string, payload []byte) ([]byte, error) {
	decision := strings.ToLower(strings.TrimSpace(hookProbeDecisionFlag))
	if decision == "" {
		decision = "allow"
	}
	switch decision {
	case "allow", "deny", "ask":
	default:
		return nil, fmt.Errorf("invalid --probe-decision: %q (must be allow, deny, or ask)", hookProbeDecisionFlag)
	}

	type probePayload struct {
		TranscriptPath      string   `json:"transcriptPath"`
		TranscriptPathSnake string   `json:"transcript_path"`
		WorkspacePaths      []string `json:"workspacePaths"`
		WorkspacePathsSnake []string `json:"workspace_paths"`
		RepoID              string   `json:"repo_id"`
		RepoIDAlt           string   `json:"repoId"`
	}

	var parsed probePayload
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &parsed)
	}
	if parsed.TranscriptPath == "" && parsed.TranscriptPathSnake != "" {
		parsed.TranscriptPath = parsed.TranscriptPathSnake
	}
	if len(parsed.WorkspacePaths) == 0 && len(parsed.WorkspacePathsSnake) > 0 {
		parsed.WorkspacePaths = parsed.WorkspacePathsSnake
	}
	if parsed.RepoID == "" && parsed.RepoIDAlt != "" {
		parsed.RepoID = parsed.RepoIDAlt
	}

	dataDir := config.DataDir()

	repoID := parsed.RepoID
	if repoID == "" {
		candidate := hookWorkspaceFlag
		if candidate == "" && len(parsed.WorkspacePaths) > 0 && parsed.WorkspacePaths[0] != "" {
			candidate = parsed.WorkspacePaths[0]
		}
		if candidate == "" && parsed.TranscriptPath != "" {
			if root, err := gitops.FindRepoRoot(filepath.Dir(parsed.TranscriptPath)); err == nil {
				candidate = root
			}
		}
		if candidate == "" {
			candidate, _ = os.Getwd()
		}

		if entry, found, _ := config.FindRegistryEntry(dataDir, candidate); found && entry != nil && entry.RepoID != "" {
			repoID = entry.RepoID
		}

		if repoID == "" && candidate != "" {
			if root, err := gitops.FindRepoRoot(candidate); err == nil && root != "" {
				if id, err := gitops.GenerateRepoID(root); err == nil && id != "" {
					repoID = id
				}
			}
			if repoID == "" {
				if id, err := gitops.GenerateRepoID(candidate); err == nil && id != "" {
					repoID = id
				}
			}
		}

		if repoID == "" {
			repoID = "unknown"
		}
	}

	logsDir := filepath.Join(dataDir, repoID, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating probe logs directory %s: %w", logsDir, err)
	}

	timestamp := probeTimeNow().UTC().Format("20060102T150405.000000000Z")

	probeFile := filepath.Join(logsDir, fmt.Sprintf("probe_%s_%s.json", event, timestamp))
	rawPayload := payload
	if len(bytes.TrimSpace(rawPayload)) == 0 {
		rawPayload = []byte("{}")
	}
	if err := os.WriteFile(probeFile, rawPayload, 0644); err != nil {
		return nil, fmt.Errorf("writing probe log %s: %w", probeFile, err)
	}

	if parsed.TranscriptPath != "" {
		targetPath := parsed.TranscriptPath
		if targetPath == "~" {
			if home, err := os.UserHomeDir(); err == nil {
				targetPath = home
			}
		} else if strings.HasPrefix(targetPath, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				targetPath = filepath.Join(home, targetPath[2:])
			}
		}
		if !filepath.IsAbs(targetPath) {
			if hookWorkspaceFlag != "" {
				targetPath = filepath.Join(hookWorkspaceFlag, targetPath)
			} else if len(parsed.WorkspacePaths) > 0 && parsed.WorkspacePaths[0] != "" {
				targetPath = filepath.Join(parsed.WorkspacePaths[0], targetPath)
			}
		}
		if fi, err := os.Stat(targetPath); err == nil && !fi.IsDir() {
			if data, err := os.ReadFile(targetPath); err == nil {
				snapshotFile := filepath.Join(logsDir, fmt.Sprintf("probe_%s_%s_transcript.jsonl", event, timestamp))
				_ = os.WriteFile(snapshotFile, data, 0644)
				_ = config.RotateLogFile(snapshotFile, config.DefaultMaxLogBytes, config.DefaultMaxRotatedFiles)
			}
		}
	}

	// Clean up probe logs older than 7 days
	_, _ = config.CleanProbeLogs(logsDir, 7*24*time.Hour)

	if strings.EqualFold(event, "PreToolUse") || strings.EqualFold(event, "pre_tool_use") {
		resp := map[string]string{
			"decision": decision,
			"reason":   "probe " + decision,
		}
		return json.Marshal(resp)
	}

	return []byte("{}"), nil
}

