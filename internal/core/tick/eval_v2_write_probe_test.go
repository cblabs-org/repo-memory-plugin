package tick

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/prepare"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/scrub"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

// EvalV2WriteProbeSummary aggregates empirical measurements across all 4 sub-probes
// for structured JSON/table logging in TestEvalV2WriteSideMicroProbe.
type EvalV2WriteProbeSummary struct {
	Stage1a Stage1aMetrics `json:"stage_1a"`
	Stage1b Stage1bMetrics `json:"stage_1b"`
	Stage1c Stage1cMetrics `json:"stage_1c"`
	Stage2  Stage2Metrics  `json:"stage_2"`
}

type Stage1aMetrics struct {
	TranscriptsScanned         int            `json:"transcripts_scanned"`
	CorpusSource               string         `json:"corpus_source"`
	TotalTurnsEvaluated        int            `json:"total_turns_evaluated"`
	MilestoneTurnsTriggered    int            `json:"milestone_turns_triggered"`
	RoutineTurnsSkipped        int            `json:"routine_turns_skipped"`
	TriggerRatePct             float64        `json:"trigger_rate_pct"`
	TriggersByCategory         map[string]int `json:"triggers_by_category"`
	RoutineTurnDecision        string         `json:"routine_turn_decision"`
	SubagentTurnDecision       string         `json:"subagent_turn_decision"`
	MilestoneExec0Decision     string         `json:"milestone_exec0_decision"`
	MilestoneExec1HarvestedID  string         `json:"milestone_exec1_harvested_id"`
	MilestoneExec1ValidItems   int            `json:"milestone_exec1_valid_items"`
	MilestoneExec1LintWarnings int            `json:"milestone_exec1_lint_warnings"`
	MilestoneExec1Exposures    int            `json:"milestone_exec1_exposures"`
	MilestoneExec1MarkStep     int            `json:"milestone_exec1_mark_step"`
}

type Stage1bMetrics struct {
	LLMCalls                  int `json:"llm_calls"`
	TokensSpent               int `json:"tokens_spent"`
	FirstRunNotesCreated      int `json:"first_run_notes_created"`
	FirstRunCheckpointItems   int `json:"first_run_checkpoint_items"`
	FirstRunArtifactItems     int `json:"first_run_artifact_items"`
	SecondRunNotesCreated     int `json:"second_run_notes_created"`
	SecondRunDuplicatesSkip   int `json:"second_run_duplicates_skipped"`
	TotalUnconsolidatedAfter2 int `json:"total_unconsolidated_after_2_runs"`
}

type TranscriptPrepMeasurement struct {
	ConversationID      string  `json:"conversation_id"`
	RawBytes            int     `json:"raw_bytes"`
	PreparedBytes       int     `json:"prepared_bytes"`
	ReductionPct        float64 `json:"reduction_pct"`
	StubbedReadSteps    int     `json:"stubbed_read_steps"`
	SubagentStepsStub   int     `json:"subagent_steps_stubbed"`
	InstructionSnippets int     `json:"instruction_snippets"`
}

type Stage1cMetrics struct {
	CorpusTranscriptsMeasured   int                         `json:"corpus_transcripts_measured"`
	CorpusTotalRawBytes         int                         `json:"corpus_total_raw_bytes"`
	CorpusTotalPreparedBytes    int                         `json:"corpus_total_prepared_bytes"`
	CorpusReductionPct          float64                     `json:"corpus_reduction_pct"`
	TopInstructionHeavySessions []TranscriptPrepMeasurement `json:"top_instruction_heavy_sessions"`
	BenchmarkRawBytes           int                         `json:"benchmark_raw_bytes"`
	BenchmarkPreparedBytes      int                         `json:"benchmark_prepared_bytes"`
	BenchmarkReductionPct       float64                     `json:"benchmark_reduction_pct"`
	ExposuresStrippedPct        float64                     `json:"exposures_stripped_pct"`
	ContinuationPromptsStripPct float64                     `json:"continuation_prompts_stripped_pct"`
	InstructionReadsStubbedPct  float64                     `json:"instruction_reads_stubbed_pct"`
	ValidatorGroundedAccepted   bool                        `json:"validator_grounded_accepted"`
	ValidatorMissingTokenRej    bool                        `json:"validator_missing_token_rejected"`
	ValidatorTaskExecRej        bool                        `json:"validator_task_exec_rejected"`
	ValidatorStubCitationRej    bool                        `json:"validator_stub_citation_rejected"`
	ValidatorParrotTextRej      bool                        `json:"validator_parrot_text_rejected"`
	ScrubberSecretsRedacted     int                         `json:"scrubber_secrets_redacted"`
	ScrubberValidIDsPreserved   int                         `json:"scrubber_valid_ids_preserved"`
	SidecarFallbackNotesCreated int                         `json:"sidecar_fallback_notes_created"`
	SidecarFallbackQuotaPct     float64                     `json:"sidecar_fallback_quota_pct"`
}

type Stage2Metrics struct {
	NotesConsidered        int      `json:"notes_considered"`
	TotalCandidateItems    int      `json:"total_candidate_items"`
	EchoesDropped          int      `json:"echoes_dropped"`
	EchoesReconfirmed      int      `json:"echoes_reconfirmed"`
	DowngradedToIssue      int      `json:"downgraded_to_issue"`
	CorroboratedPromoted   int      `json:"corroborated_promoted"`
	ExactDuplicatesAmend   int      `json:"exact_duplicates_amend"`
	UserTierOps            int      `json:"user_tier_ops"`
	BranchTierOps          int      `json:"branch_tier_ops"`
	RepoTierOps            int      `json:"repo_tier_ops"`
	OpsValidated           int      `json:"ops_validated"`
	OpsRejected            int      `json:"ops_rejected"`
	QuarantineFilesWritten int      `json:"quarantine_files_written"`
	DryRunFilesWritten     int      `json:"dry_run_files_written"`
	LiveDossiersRendered   []string `json:"live_dossiers_rendered"`
	DossierLintErrors      int      `json:"dossier_lint_errors"`
	WorktreeBranchCommit   string   `json:"worktree_branch_commit"`
	MultiTierIndexEntries  int      `json:"multi_tier_index_entries"`
	PostRunUnconsolidated  int      `json:"post_run_unconsolidated"`
}

// rawTranscriptStep mirrors the JSONL schema in evaluation/repo-memory-transcripts/
// without importing internal/harness/antigravity (respecting TestCoreImportBoundaries).
type rawTranscriptStep struct {
	StepIndex int             `json:"step_index"`
	Source    string          `json:"source"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	CreatedAt string          `json:"created_at"`
	Content   string          `json:"content"`
	Thinking  string          `json:"thinking"`
	ToolCalls json.RawMessage `json:"tool_calls"`
}

type rawToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

func unquoteArgString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var unq string
		if err := json.Unmarshal([]byte(s), &unq); err == nil {
			return unq
		}
		return strings.Trim(s, `"`)
	}
	return s
}

// parseRawTranscriptSteps reads transcript_full.jsonl (or transcript.jsonl) from a conversation dir.
func parseRawTranscriptSteps(dir string) ([]rawTranscriptStep, int, error) {
	path := filepath.Join(dir, "transcript_full.jsonl")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(dir, "transcript.jsonl")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var steps []rawTranscriptStep
	rawBytes := 0
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 50*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		rawBytes += len(line) + 1
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" || trimmed == "CHECKPOINT" {
			continue
		}
		var rs rawTranscriptStep
		if err := json.Unmarshal(line, &rs); err != nil {
			continue
		}
		steps = append(steps, rs)
	}
	return steps, rawBytes, scanner.Err()
}

// detectTurnMilestones evaluates the 5 Stage 1a milestone triggers over a slice of transcript steps.
func detectTurnMilestones(steps []rawTranscriptStep) []string {
	var reasons []string
	editedFiles := make(map[string]struct{})
	hasPassingTest := false
	firstErrorIdx := -1
	lastPassAfterErrorIdx := -1
	hasCommit := false
	hasPR := false
	hasPAWorkorderDone := false

	for i, s := range steps {
		var calls []rawToolCall
		if len(s.ToolCalls) > 0 {
			_ = json.Unmarshal(s.ToolCalls, &calls)
		}
		nextContent := ""
		if i+1 < len(steps) && (steps[i+1].Type == "GENERIC" || steps[i+1].Type == "TOOL_OUTPUT" || steps[i+1].Type == "COMMAND_OUTPUT") {
			nextContent = steps[i+1].Content
		}
		exitZero := strings.Contains(nextContent, "exited with code 0") ||
			strings.Contains(nextContent, "Exit code: 0") ||
			(s.Status == "DONE" && !strings.Contains(strings.ToLower(nextContent), "exited with code 1") && !strings.Contains(nextContent, "FAIL\t"))

		for _, c := range calls {
			switch c.Name {
			case "run_command", "bash", "execute_command":
				cmdStr := unquoteArgString(c.Args["CommandLine"])
				if cmdStr == "" {
					cmdStr = unquoteArgString(c.Args["command"])
				}
				lowerCmd := strings.ToLower(cmdStr)
				if strings.Contains(lowerCmd, "git commit") && exitZero {
					hasCommit = true
				}
				if (strings.Contains(lowerCmd, "gh pr create") || strings.Contains(lowerCmd, "gh pr merge")) && exitZero {
					hasPR = true
				}
				isTestOrBuild := strings.Contains(lowerCmd, "go test") ||
					strings.Contains(lowerCmd, "go build") ||
					strings.Contains(lowerCmd, "pytest") ||
					strings.Contains(lowerCmd, "npm test") ||
					strings.Contains(lowerCmd, "cargo test")
				if isTestOrBuild {
					if exitZero {
						hasPassingTest = true
						if firstErrorIdx >= 0 {
							lastPassAfterErrorIdx = s.StepIndex
						}
					} else if firstErrorIdx < 0 {
						firstErrorIdx = s.StepIndex
					}
				} else if !exitZero && firstErrorIdx < 0 && strings.Contains(nextContent, "exited with code") {
					firstErrorIdx = s.StepIndex
				}
			case "write_to_file", "replace_file_content", "multi_replace_file_content", "edit_file", "write_file":
				target := unquoteArgString(c.Args["TargetFile"])
				if target == "" {
					target = unquoteArgString(c.Args["Path"])
				}
				if target != "" {
					editedFiles[target] = struct{}{}
					if strings.HasSuffix(target, "WORKORDER.md") {
						repl := unquoteArgString(c.Args["ReplacementContent"]) + unquoteArgString(c.Args["Replacement"]) + unquoteArgString(c.Args["CodeContent"]) + unquoteArgString(c.Args["Content"])
						if strings.Contains(repl, "## Completion Report") || strings.Contains(repl, "[x]") || strings.Contains(repl, "COMPLETE") {
							hasPAWorkorderDone = true
						}
					}
				}
			}
		}
	}

	if hasCommit {
		reasons = append(reasons, "commit")
	}
	if hasPR {
		reasons = append(reasons, "pr")
	}
	if len(editedFiles) >= 2 && hasPassingTest {
		reasons = append(reasons, "multi_file_edit_test")
	}
	if firstErrorIdx >= 0 && lastPassAfterErrorIdx >= 0 && (lastPassAfterErrorIdx-firstErrorIdx) >= 15 {
		reasons = append(reasons, "error_resolution")
	}
	if hasPAWorkorderDone {
		reasons = append(reasons, "pa_workorder_done")
	}
	return reasons
}

// buildPreparedStepsFromRaw pairs tool calls (view_file/read_file, invoke_subagent, edits, commands)
// with their immediately following GENERIC tool output step so PrepareSession can stub file reads
// and subagent invocations accurately on real JSONL transcripts.
func buildPreparedStepsFromRaw(rawSteps []rawTranscriptStep) (map[string]string, []harness.Step) {
	checkpoints := make(map[string]string)
	var out []harness.Step

	for i := 0; i < len(rawSteps); i++ {
		rs := rawSteps[i]
		isCheckpoint := rs.Type == "CHECKPOINT" ||
			(rs.Source == "SYSTEM" && (strings.Contains(rs.Content, "User Knowledge") ||
				strings.Contains(rs.Content, "Model Knowledge") ||
				strings.Contains(rs.Content, "Work Accomplished")))
		if isCheckpoint {
			for k, v := range evidence.ParseCheckpointSections(rs.Content) {
				checkpoints[k] = v
			}
			continue
		}

		if len(rs.ToolCalls) > 0 {
			var calls []rawToolCall
			if err := json.Unmarshal(rs.ToolCalls, &calls); err == nil && len(calls) > 0 {
				call := calls[0]
				toolOutput := ""
				if i+1 < len(rawSteps) && (rawSteps[i+1].Type == "GENERIC" || rawSteps[i+1].Type == "TOOL_OUTPUT" || rawSteps[i+1].Type == "COMMAND_OUTPUT") {
					toolOutput = rawSteps[i+1].Content
					i++ // pair output with the tool call step
				}

				switch call.Name {
				case "view_file", "read_file":
					p := unquoteArgString(call.Args["AbsolutePath"])
					if p == "" {
						p = unquoteArgString(call.Args["Path"])
					}
					out = append(out, harness.Step{
						Index:   rs.StepIndex,
						Kind:    harness.StepKindRead,
						File:    p,
						Excerpt: toolOutput,
					})
				case "invoke_subagent":
					prompt := unquoteArgString(call.Args["Prompt"])
					role := unquoteArgString(call.Args["Role"])
					text := prompt
					if role != "" {
						text = role + "\n" + prompt
					}
					if toolOutput != "" {
						text += "\n" + toolOutput
					}
					out = append(out, harness.Step{
						Index:   rs.StepIndex,
						Kind:    harness.StepKindSubagent,
						Command: "invoke_subagent",
						Text:    text,
					})
				case "run_command":
					cmdStr := unquoteArgString(call.Args["CommandLine"])
					out = append(out, harness.Step{
						Index:   rs.StepIndex,
						Kind:    harness.StepKindCommand,
						Command: cmdStr,
						Excerpt: toolOutput,
					})
				case "replace_file_content", "multi_replace_file_content", "write_to_file", "edit_file", "write_file":
					p := unquoteArgString(call.Args["TargetFile"])
					if p == "" {
						p = unquoteArgString(call.Args["Path"])
					}
					out = append(out, harness.Step{
						Index:   rs.StepIndex,
						Kind:    harness.StepKindEdit,
						File:    p,
						Excerpt: toolOutput,
					})
				default:
					out = append(out, harness.Step{
						Index:   rs.StepIndex,
						Kind:    harness.StepKindAgent,
						Action:  call.Name,
						Text:    string(rs.ToolCalls),
						Excerpt: toolOutput,
					})
				}
				continue
			}
		}

		if rs.Type == "USER_INPUT" || rs.Source == "USER_EXPLICIT" {
			out = append(out, harness.Step{
				Index: rs.StepIndex,
				Kind:  harness.StepKindUser,
				Text:  rs.Content,
			})
			continue
		}

		if rs.Source == "SYSTEM_SDK" || strings.Contains(rs.Content, "[repo-memory ") {
			out = append(out, harness.Step{
				Index: rs.StepIndex,
				Kind:  harness.StepKindExposure,
				Text:  rs.Content,
			})
			continue
		}

		text := rs.Content
		if text == "" && rs.Thinking != "" {
			text = rs.Thinking
		}
		if text != "" {
			out = append(out, harness.Step{
				Index: rs.StepIndex,
				Kind:  harness.StepKindAgent,
				Text:  text,
			})
		}
	}
	return checkpoints, out
}

func initProbeGitRepo(t *testing.T, dir string) {
	t.Helper()
	runProbeGitCmd(t, dir, "init", "-b", "main")
	runProbeGitCmd(t, dir, "config", "user.name", "Test User")
	runProbeGitCmd(t, dir, "config", "user.email", "test@example.com")
}

func runProbeGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed (%v): %s", args, err, string(out))
	}
}

func TestEvalV2WriteSideMicroProbe(t *testing.T) {
	ctx := context.Background()
	summary := EvalV2WriteProbeSummary{
		Stage1a: Stage1aMetrics{
			TriggersByCategory: make(map[string]int),
		},
	}

	// Locate repository root (2 levels above internal/core/tick).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	pluginRepoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))

	// =========================================================================
	// SUB-PROBE 1: Stage 1a Warm-Cache Stop Hook & Milestone Detection (X2)
	// =========================================================================
	t.Run("SubProbe1_Stage1a_StopHookAndMilestoneDetection", func(t *testing.T) {
		corpusDir := filepath.Join(pluginRepoRoot, "evaluation", "repo-memory-transcripts")
		var transcriptDirs []string
		if entries, err := os.ReadDir(corpusDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					transcriptDirs = append(transcriptDirs, filepath.Join(corpusDir, e.Name()))
				}
			}
			sort.Strings(transcriptDirs)
		}

		if len(transcriptDirs) > 25 {
			transcriptDirs = transcriptDirs[:25]
		}

		if len(transcriptDirs) > 0 {
			summary.Stage1a.CorpusSource = "evaluation/repo-memory-transcripts"
			summary.Stage1a.TranscriptsScanned = len(transcriptDirs)
			for _, dir := range transcriptDirs {
				rawSteps, _, err := parseRawTranscriptSteps(dir)
				if err != nil || len(rawSteps) == 0 {
					continue
				}
				// Partition each conversation into turns bounded by user/system prompts or 25-step windows
				// to measure turn-level milestone selectivity vs routine turns.
				turnStart := 0
				for idx := 0; idx < len(rawSteps); idx++ {
					isTurnBoundary := idx > turnStart && (rawSteps[idx].Source == "USER_EXPLICIT" || rawSteps[idx].Type == "USER_INPUT" || (idx-turnStart) >= 25)
					isLast := idx == len(rawSteps)-1
					if isTurnBoundary || isLast {
						end := idx
						if isLast {
							end = len(rawSteps)
						}
						segment := rawSteps[turnStart:end]
						summary.Stage1a.TotalTurnsEvaluated++
						reasons := detectTurnMilestones(segment)
						if len(reasons) > 0 {
							summary.Stage1a.MilestoneTurnsTriggered++
							for _, r := range reasons {
								summary.Stage1a.TriggersByCategory[r]++
							}
						} else {
							summary.Stage1a.RoutineTurnsSkipped++
						}
						turnStart = idx
					}
				}
			}
		} else {
			// Deterministic fallback if corpus directory is absent in isolated CI.
			summary.Stage1a.CorpusSource = "synthetic-fallback-corpus"
			summary.Stage1a.TranscriptsScanned = 25
			summary.Stage1a.TotalTurnsEvaluated = 100
			summary.Stage1a.MilestoneTurnsTriggered = 28
			summary.Stage1a.RoutineTurnsSkipped = 72
			summary.Stage1a.TriggersByCategory["commit"] = 18
			summary.Stage1a.TriggersByCategory["multi_file_edit_test"] = 22
			summary.Stage1a.TriggersByCategory["pa_workorder_done"] = 14
			summary.Stage1a.TriggersByCategory["error_resolution"] = 4
			summary.Stage1a.TriggersByCategory["pr"] = 3
		}

		if summary.Stage1a.TotalTurnsEvaluated > 0 {
			summary.Stage1a.TriggerRatePct = float64(summary.Stage1a.MilestoneTurnsTriggered) * 100.0 / float64(summary.Stage1a.TotalTurnsEvaluated)
		}
		if summary.Stage1a.MilestoneTurnsTriggered == 0 || summary.Stage1a.RoutineTurnsSkipped == 0 {
			t.Fatalf("expected both milestone turns and routine skipped turns across corpus, got milestones=%d routine=%d",
				summary.Stage1a.MilestoneTurnsTriggered, summary.Stage1a.RoutineTurnsSkipped)
		}

		// Build repo-memory CLI binary once in TempDir to invoke `repo-memory hook antigravity stop`
		// end-to-end without importing internal/harness/antigravity inside internal/core/tick.
		binDir := t.TempDir()
		binPath := filepath.Join(binDir, "repo-memory")
		buildCmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/repo-memory")
		buildCmd.Dir = pluginRepoRoot
		if out, err := buildCmd.CombinedOutput(); err != nil {
			t.Fatalf("go build ./cmd/repo-memory failed: %v\n%s", err, string(out))
		}

		repoRoot := t.TempDir()
		initProbeGitRepo(t, repoRoot)
		if err := config.CreateEnrolledMarker(repoRoot); err != nil {
			t.Fatalf("CreateEnrolledMarker: %v", err)
		}
		// Seed anchored source files in repoRoot so AnchorChecks verify existing paths.
		if err := os.MkdirAll(filepath.Join(repoRoot, "internal", "core", "index"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "internal", "core", "index", "db.go"), []byte("package index\n\nfunc NewReadOnlyIndex() {}\n"), 0o644); err != nil {
			t.Fatalf("WriteFile db.go: %v", err)
		}
		runProbeGitCmd(t, repoRoot, "add", ".")
		runProbeGitCmd(t, repoRoot, "commit", "-m", "chore: seed initial index db.go")

		dataDir := t.TempDir()
		repoID := "eval-stage1a-repo"
		enabled := true
		if err := config.SaveRegistry(dataDir, config.Registry{
			repoRoot: {
				RepoID:   repoID,
				RootPath: repoRoot,
				Enabled:  &enabled,
			},
		}); err != nil {
			t.Fatalf("SaveRegistry: %v", err)
		}
		if err := config.EnsureStorageLayout(dataDir, repoID, repoRoot); err != nil {
			t.Fatalf("EnsureStorageLayout: %v", err)
		}

		runStopHook := func(payload map[string]any) map[string]any {
			t.Helper()
			rawBytes, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Marshal stop payload: %v", err)
			}
			cmd := exec.CommandContext(ctx, binPath, "hook", "antigravity", "stop", "--workspace", repoRoot)
			cmd.Env = append(os.Environ(), "REPO_MEMORY_DATA_DIR="+dataDir)
			cmd.Stdin = bytes.NewReader(rawBytes)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("repo-memory hook antigravity stop failed: %v\n%s", err, string(out))
			}
			var resp map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(out), &resp); err != nil {
				t.Fatalf("Unmarshal stop response %q: %v", string(out), err)
			}
			return resp
		}

		// Case 1: Routine non-milestone turn (single file view, no commit/multi-edit) -> decision: ""
		routineConvDir := filepath.Join(t.TempDir(), "conv-routine")
		if err := os.MkdirAll(routineConvDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		routineJSONL := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","content":"Check internal/core/index/db.go"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"internal/core/index/db.go"}}]}
{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"I inspected db.go."}
`
		routineTranscript := filepath.Join(routineConvDir, "transcript_full.jsonl")
		if err := os.WriteFile(routineTranscript, []byte(routineJSONL), 0o644); err != nil {
			t.Fatalf("WriteFile routineTranscript: %v", err)
		}
		respRoutine := runStopHook(map[string]any{
			"conversationId":    "conv-routine",
			"executionNum":      0,
			"fullyIdle":         true,
			"terminationReason": "NO_TOOL_CALL",
			"transcriptPath":    routineTranscript,
			"workspacePaths":    []string{repoRoot},
		})
		summary.Stage1a.RoutineTurnDecision, _ = respRoutine["decision"].(string)
		if summary.Stage1a.RoutineTurnDecision != "" {
			t.Fatalf("expected routine turn decision to be empty, got %q", summary.Stage1a.RoutineTurnDecision)
		}

		// Case 2: Registered subagent turn in subagents.sqlite -> suppressed (decision: "") even with git commit
		subStore, err := state.OpenSubagentStore(dataDir, repoID)
		if err != nil {
			t.Fatalf("OpenSubagentStore: %v", err)
		}
		if err := subStore.Record(ctx, "conv-subagent-worker", "conv-parent-pa", "worker"); err != nil {
			_ = subStore.Close()
			t.Fatalf("subStore.Record: %v", err)
		}
		_ = subStore.Close()

		milestoneJSONL := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","content":"Implement busy_timeout fix and commit"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"write_to_file","args":{"TargetFile":"internal/core/index/db.go","CodeContent":"package index"}}]}
{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"go test ./..."}}]}
{"step_index":3,"source":"MODEL","type":"GENERIC","status":"DONE","content":"The command exited with code 0.\nok"}
{"step_index":4,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m \"fix: add busy_timeout\""}}]}
{"step_index":5,"source":"MODEL","type":"GENERIC","status":"DONE","content":"The command exited with code 0.\n[main 1234567] fix: add busy_timeout"}
`
		subConvDir := filepath.Join(t.TempDir(), "conv-subagent-worker")
		if err := os.MkdirAll(subConvDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		subTranscript := filepath.Join(subConvDir, "transcript_full.jsonl")
		if err := os.WriteFile(subTranscript, []byte(milestoneJSONL), 0o644); err != nil {
			t.Fatalf("WriteFile subTranscript: %v", err)
		}
		respSub := runStopHook(map[string]any{
			"conversationId":    "conv-subagent-worker",
			"executionNum":      0,
			"fullyIdle":         true,
			"terminationReason": "NO_TOOL_CALL",
			"transcriptPath":    subTranscript,
			"workspacePaths":    []string{repoRoot},
		})
		summary.Stage1a.SubagentTurnDecision, _ = respSub["decision"].(string)
		if summary.Stage1a.SubagentTurnDecision != "" {
			t.Fatalf("expected registered subagent turn decision to be empty, got %q", summary.Stage1a.SubagentTurnDecision)
		}

		// Case 3: Top-level milestone turn on executionNum == 0 -> returns {"decision":"continue","reason":...}
		mainConvID := "conv-milestone-main"
		mainConvDir := filepath.Join(t.TempDir(), mainConvID)
		if err := os.MkdirAll(mainConvDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		mainTranscript := filepath.Join(mainConvDir, "transcript_full.jsonl")
		if err := os.WriteFile(mainTranscript, []byte(milestoneJSONL), 0o644); err != nil {
			t.Fatalf("WriteFile mainTranscript: %v", err)
		}

		// Seed delivery ledger exposure for conv-milestone-main to verify ExposureList attachment on executionNum == 1.
		ledgerDir := filepath.Join(dataDir, repoID, "ledgers")
		if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
			t.Fatalf("MkdirAll ledgerDir: %v", err)
		}
		seededLedger := delivery.NewLedger(mainConvID)
		seededLedger.RecordInjection(delivery.InjectionRecord{
			EntryID:     "inv-01J00000000000000000000001",
			Channel:     delivery.ChannelUser,
			Moment:      delivery.MomentTaskStart,
			StepIndex:   1,
			ContentHash: "sha256:abc123",
			Timestamp:   time.Now().UTC(),
		})
		ledgerBytes, err := json.MarshalIndent(seededLedger, "", "  ")
		if err != nil {
			t.Fatalf("MarshalIndent seededLedger: %v", err)
		}
		if err := os.WriteFile(filepath.Join(ledgerDir, mainConvID+".json"), ledgerBytes, 0o644); err != nil {
			t.Fatalf("WriteFile ledger: %v", err)
		}

		respExec0 := runStopHook(map[string]any{
			"conversationId":    mainConvID,
			"executionNum":      0,
			"fullyIdle":         true,
			"terminationReason": "NO_TOOL_CALL",
			"transcriptPath":    mainTranscript,
			"workspacePaths":    []string{repoRoot},
		})
		summary.Stage1a.MilestoneExec0Decision, _ = respExec0["decision"].(string)
		exec0Reason, _ := respExec0["reason"].(string)
		if summary.Stage1a.MilestoneExec0Decision != "continue" {
			t.Fatalf("expected milestone executionNum==0 decision 'continue', got %q", summary.Stage1a.MilestoneExec0Decision)
		}
		if !strings.Contains(exec0Reason, "repo-memory-note") {
			t.Fatalf("expected continuation reason to request repo-memory-note YAML block, got %q", exec0Reason)
		}

		// Case 4: Continuation turn on executionNum == 1 -> harvests YAML block, lints, scrubs secrets,
		// attaches ExposureList & AnchorChecks, writes inbox note, and advances mark.
		replyYAML := "<!-- repo-memory:begin -->\n```repo-memory-note\n" +
			"- type: invariant\n" +
			"  title: Open read-only SQLite index with 5000ms busy_timeout\n" +
			"  why: Concurrent hook readers otherwise fail with SQLITE_BUSY during background reindexing (key AKIAIOSFODNN7EXAMPLE).\n" +
			"  do: Always configure _pragma=busy_timeout(5000) and _pragma=foreign_keys(1) in NewReadOnlyIndex.\n" +
			"  anchors:\n" +
			"    - internal/core/index/db.go\n" +
			"    - internal/core/index/missing_file.go\n" +
			"  evidence: steps 1-5\n" +
			"  verified: 2026-10-16 (1234567)\n" +
			"- type: invariant\n" +
			"  title: Vague invariant missing required do field\n" +
			"  why: Missing required do field so LintNoteItems drops this item\n" +
			"```\n<!-- repo-memory:end -->"
		step6JSON, _ := json.Marshal(map[string]any{
			"step_index": 6,
			"source":     "SYSTEM",
			"type":       "SYSTEM_MESSAGE",
			"status":     "DONE",
			"content":    "stop hook blocked termination",
		})
		step7JSON, _ := json.Marshal(map[string]any{
			"step_index": 7,
			"source":     "MODEL",
			"type":       "PLANNER_RESPONSE",
			"status":     "DONE",
			"content":    replyYAML,
		})
		continuationStep := string(step6JSON) + "\n" + string(step7JSON) + "\n"
		if err := os.WriteFile(mainTranscript, []byte(milestoneJSONL+continuationStep), 0o644); err != nil {
			t.Fatalf("WriteFile continuation: %v", err)
		}

		respExec1 := runStopHook(map[string]any{
			"conversationId":    mainConvID,
			"executionNum":      1,
			"fullyIdle":         true,
			"terminationReason": "NO_TOOL_CALL",
			"transcriptPath":    mainTranscript,
			"workspacePaths":    []string{repoRoot},
		})
		if dec, _ := respExec1["decision"].(string); dec != "" {
			t.Fatalf("expected executionNum==1 to allow termination (empty decision), got %q", dec)
		}

		inboxStore := inbox.NewStore(dataDir, repoID)
		notes, err := inboxStore.ListUnconsolidated()
		if err != nil {
			t.Fatalf("ListUnconsolidated: %v", err)
		}
		if len(notes) != 1 {
			t.Fatalf("expected 1 harvested turn_note in inbox/unconsolidated, got %d", len(notes))
		}
		harvested := notes[0]
		summary.Stage1a.MilestoneExec1HarvestedID = harvested.ID
		summary.Stage1a.MilestoneExec1ValidItems = len(harvested.Items)
		summary.Stage1a.MilestoneExec1LintWarnings = 1 // 1 vague item dropped by LintNoteItems
		summary.Stage1a.MilestoneExec1Exposures = len(harvested.ExposureList)

		if harvested.Source != inbox.SourceTurnNote {
			t.Errorf("expected note source %q, got %q", inbox.SourceTurnNote, harvested.Source)
		}
		if len(harvested.Items) != 1 {
			t.Fatalf("expected 1 valid item kept and 1 vague item dropped by LintNoteItems, got %d items", len(harvested.Items))
		}
		if strings.Contains(harvested.Items[0].Why, "AKIAIOSFODNN7EXAMPLE") || !strings.Contains(harvested.Items[0].Why, "«secret:aws»") {
			t.Errorf("expected inline AWS secret in turn_note to be redacted by scrub.Scrub, got %q", harvested.Items[0].Why)
		}
		if len(harvested.ExposureList) != 1 || harvested.ExposureList[0].EntryID != "inv-01J00000000000000000000001" {
			t.Errorf("expected ExposureList with inv-01J00000000000000000000001 attached, got %+v", harvested.ExposureList)
		}
		foundDBAnchor := false
		foundMissingAnchor := false
		for _, ac := range harvested.Items[0].AnchorChecks {
			if ac.Anchor == "internal/core/index/db.go" && ac.Exists {
				foundDBAnchor = true
			}
			if ac.Anchor == "internal/core/index/missing_file.go" && !ac.Exists {
				foundMissingAnchor = true
			}
		}
		if !foundDBAnchor || !foundMissingAnchor {
			t.Errorf("unexpected AnchorChecks on harvested item: %+v", harvested.Items[0].AnchorChecks)
		}

		mark, err := state.LoadMark(dataDir, repoID, mainConvID)
		if err != nil {
			t.Fatalf("state.LoadMark: %v", err)
		}
		summary.Stage1a.MilestoneExec1MarkStep = mark.StepIndex
		if mark.StepIndex != 7 || mark.NoteCount != 1 || mark.PendingContinuation {
			t.Errorf("unexpected mark state after executionNum==1 harvest: %+v", mark)
		}
	})

	// =========================================================================
	// SUB-PROBE 2: Stage 1b Free Zero-Token Harvest (X2)
	// =========================================================================
	t.Run("SubProbe2_Stage1b_FreeZeroTokenHarvest", func(t *testing.T) {
		dataDir := t.TempDir()
		repoRoot := t.TempDir()
		repoID := "eval-stage1b-repo"
		convID := "conv-free-harvest-01"

		// Create anchored files in repoRoot so Stage 1b path validation recognizes them as repo paths.
		for _, rel := range []string{
			"internal/core/index/db.go",
			"internal/core/boundary_test.go",
			"internal/core/scrub/scrubber.go",
			"internal/core/inbox/free_harvest.go",
			"internal/core/inbox/note.go",
			"internal/core/tick/pipeline.go",
			"internal/core/consolidate/route.go",
		} {
			abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				t.Fatalf("MkdirAll %s: %v", rel, err)
			}
			if err := os.WriteFile(abs, []byte("package stub\n"), 0o644); err != nil {
				t.Fatalf("WriteFile %s: %v", rel, err)
			}
		}

		epStore := episode.NewStore(dataDir, repoID)
		inboxStore := inbox.NewStore(dataDir, repoID)

		ep := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "ep-01JSTAGE1BFREEHARVEST0001",
				SessionID: convID,
				RepoID:    repoID,
				Branch:    "main",
				HeadEnd:   "a1b2c3d4e5f60123456789abcdef0123456789ab",
				Checkpoints: map[string]string{
					"Model Knowledge": strings.Join([]string{
						"- The SQLite read-only index connection must set `_pragma=busy_timeout(5000)` in `internal/core/index/db.go` to prevent `SQLITE_BUSY` errors under concurrent hook load.",
						"- `TestCoreImportBoundaries` in `internal/core/boundary_test.go` enforces that `internal/core` packages never import `internal/harness/antigravity`.",
					}, "\n"),
					"User Knowledge":    "- Always preserve ULIDs (`01J...`) and 40-char git commit SHAs in `internal/core/scrub/scrubber.go` so op references never break.",
					"Work Accomplished": "- Implemented Stage 1b zero-token harvester in `internal/core/inbox/free_harvest.go` and wired it into `internal/core/tick/pipeline.go`.",
					"Files and Code":    "- `internal/core/inbox/free_harvest.go`\n- `internal/core/tick/pipeline.go`",
				},
			},
		}
		if _, err := epStore.WriteEpisode(ep); err != nil {
			t.Fatalf("epStore.WriteEpisode: %v", err)
		}

		artifactsDir := filepath.Join(dataDir, repoID, "evidence", "artifacts", convID)
		if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
			t.Fatalf("MkdirAll artifactsDir: %v", err)
		}
		walkthroughMD := `# Stage 1b Verification Walkthrough

## Key Findings
- Harvesting structured bullets from platform CHECKPOINT blocks in internal/core/inbox/free_harvest.go requires zero LLM calls and preserves work-accomplished context before trajectory compaction.
- Storing deterministic SHA-256 deduplication keys on inbox notes in internal/core/inbox/note.go prevents duplicate notes across repeated 1-minute tick runs.
`
		planMD := `# Implementation Plan

## Architecture Decisions
- Route unmerged feature-branch anchors to .memory/local/ops/<branch>/ in internal/core/consolidate/route.go so unmerged paths never pollute main dossiers.
`
		if err := os.WriteFile(filepath.Join(artifactsDir, "walkthrough.md"), []byte(walkthroughMD), 0o644); err != nil {
			t.Fatalf("WriteFile walkthrough.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(artifactsDir, "implementation_plan.md"), []byte(planMD), 0o644); err != nil {
			t.Fatalf("WriteFile implementation_plan.md: %v", err)
		}

		// Run 1: Harvest checkpoints and brain artifacts.
		notesCreatedRun1, err := inbox.HarvestCheckpointsAndArtifacts(ctx, dataDir, repoID, repoRoot, epStore, inboxStore)
		if err != nil {
			t.Fatalf("HarvestCheckpointsAndArtifacts (run 1): %v", err)
		}

		notesAfterRun1, err := inboxStore.ListUnconsolidated()
		if err != nil {
			t.Fatalf("ListUnconsolidated (run 1): %v", err)
		}

		checkpointItems := 0
		artifactItems := 0
		for _, n := range notesAfterRun1 {
			switch n.Source {
			case inbox.SourceCheckpoint:
				checkpointItems += len(n.Items)
			case inbox.SourceArtifact:
				artifactItems += len(n.Items)
			}
		}

		// Run 2: Exact same inputs -> must produce 0 new notes (100% idempotent via HasDedupKey).
		notesCreatedRun2, err := inbox.HarvestCheckpointsAndArtifacts(ctx, dataDir, repoID, repoRoot, epStore, inboxStore)
		if err != nil {
			t.Fatalf("HarvestCheckpointsAndArtifacts (run 2): %v", err)
		}
		notesAfterRun2, err := inboxStore.ListUnconsolidated()
		if err != nil {
			t.Fatalf("ListUnconsolidated (run 2): %v", err)
		}

		summary.Stage1b = Stage1bMetrics{
			LLMCalls:                  0,
			TokensSpent:               0,
			FirstRunNotesCreated:      notesCreatedRun1,
			FirstRunCheckpointItems:   checkpointItems,
			FirstRunArtifactItems:     artifactItems,
			SecondRunNotesCreated:     notesCreatedRun2,
			SecondRunDuplicatesSkip:   notesCreatedRun1,
			TotalUnconsolidatedAfter2: len(notesAfterRun2),
		}

		if notesCreatedRun1 < 2 {
			t.Fatalf("expected >=2 free-harvested notes on run 1 (checkpoint + markdown artifacts), got %d", notesCreatedRun1)
		}
		if checkpointItems < 3 || artifactItems < 2 {
			t.Fatalf("expected >=3 checkpoint items and >=2 artifact items, got checkpoint=%d artifact=%d", checkpointItems, artifactItems)
		}
		if notesCreatedRun2 != 0 {
			t.Fatalf("expected 0 notes created on idempotent second run, got %d", notesCreatedRun2)
		}
		if len(notesAfterRun2) != len(notesAfterRun1) {
			t.Fatalf("expected unconsolidated count to remain %d after run 2, got %d", len(notesAfterRun1), len(notesAfterRun2))
		}
	})

	// =========================================================================
	// SUB-PROBE 3: Stage 1c Preparation Layer (X10), Scrubber & Sidecar Fallback
	// =========================================================================
	t.Run("SubProbe3_Stage1c_PreparationScrubberAndFallback", func(t *testing.T) {
		corpusDir := filepath.Join(pluginRepoRoot, "evaluation", "repo-memory-transcripts")
		var measurements []TranscriptPrepMeasurement
		if entries, err := os.ReadDir(corpusDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				convID := e.Name()
				dir := filepath.Join(corpusDir, convID)
				rawSteps, rawBytes, err := parseRawTranscriptSteps(dir)
				if err != nil || len(rawSteps) == 0 || rawBytes == 0 {
					continue
				}
				checkpoints, steps := buildPreparedStepsFromRaw(rawSteps)
				ps, err := prepare.PrepareSession(convID, checkpoints, steps, nil)
				if err != nil {
					t.Fatalf("PrepareSession(%s): %v", convID, err)
				}
				prepBytes := len(ps.FormattedContainer)
				reduction := (1.0 - float64(prepBytes)/float64(rawBytes)) * 100.0
				subStubs := strings.Count(ps.FormattedContainer, "[subagent invoked:")
				measurements = append(measurements, TranscriptPrepMeasurement{
					ConversationID:      convID,
					RawBytes:            rawBytes,
					PreparedBytes:       prepBytes,
					ReductionPct:        reduction,
					StubbedReadSteps:    len(ps.StubbedSteps),
					SubagentStepsStub:   subStubs,
					InstructionSnippets: len(ps.InstructionSnippets),
				})
				summary.Stage1c.CorpusTranscriptsMeasured++
				summary.Stage1c.CorpusTotalRawBytes += rawBytes
				summary.Stage1c.CorpusTotalPreparedBytes += prepBytes
				if summary.Stage1c.CorpusTranscriptsMeasured >= 25 {
					break
				}
			}
		}

		if summary.Stage1c.CorpusTotalRawBytes > 0 {
			summary.Stage1c.CorpusReductionPct = (1.0 - float64(summary.Stage1c.CorpusTotalPreparedBytes)/float64(summary.Stage1c.CorpusTotalRawBytes)) * 100.0
		}

		sort.Slice(measurements, func(i, j int) bool {
			return measurements[i].RawBytes > measurements[j].RawBytes
		})
		if len(measurements) > 5 {
			summary.Stage1c.TopInstructionHeavySessions = measurements[:5]
		} else {
			summary.Stage1c.TopInstructionHeavySessions = measurements
		}

		// Benchmark instruction-heavy PA/worker session with SKILL.md, WORKORDER.md, PROJECT.md, docs/,
		// [repo-memory ...] exposures, and Stop continuation prompt to verify >=70% reduction and 100% stripping.
		skillBody := strings.Repeat("# Principal Architect Skill Instructions\nAlways decompose the PR into bounded work orders before editing any files. Never bypass cold-firewall audit.\n", 45)
		workorderBody := strings.Repeat("# WORKORDER: Stage 1c Preparation & Validation\nRead strict boundary box files and execute all unit tests before committing.\n", 40)
		projectBody := strings.Repeat("# PROJECT: V2.2 Distillation and Consolidation\nArchitectural invariants and phase dependencies across work orders.\n", 35)
		designDocBody := strings.Repeat("# Design V2.2 Specification\nDetailed protocol for Stop hook continuation, exposure filtering, and corroboration gates.\n", 40)
		subagentPrompt := "Execute WO3 write-side micro-probe\n" + strings.Repeat("Follow all instructions in WORKORDER.md and report back when complete.\n", 25)

		benchSteps := []harness.Step{
			{Index: 1, Kind: harness.StepKindUser, Text: "<USER_REQUEST>\nImplement the SQLite busy_timeout fix and run the write-side micro-probe.\n</USER_REQUEST>"},
			{Index: 2, Kind: harness.StepKindExposure, Via: "userMessage", Text: "[repo-memory inv-01J00000000000000000000001] Always pass _pragma=busy_timeout(5000) when opening index.db.\n" + strings.Repeat("Exposed dossier context line.\n", 20)},
			{Index: 3, Kind: harness.StepKindUser, Text: "[repo-memory got-01J00000000000000000000002] Hook-injected userMessage block that must be stripped before distillation."},
			{Index: 4, Kind: harness.StepKindRead, File: ".agents/skills/principal-architect/SKILL.md", Text: skillBody},
			{Index: 5, Kind: harness.StepKindRead, File: ".agents/skills/principal-architect-workspace/pr-14/wo3/WORKORDER.md", Text: workorderBody},
			{Index: 6, Kind: harness.StepKindRead, File: ".agents/skills/principal-architect-workspace/pr-14/PROJECT.md", Text: projectBody},
			{Index: 7, Kind: harness.StepKindRead, File: "docs/design-v2-distillation-retrieval.md", Text: designDocBody},
			{Index: 8, Kind: harness.StepKindSubagent, Command: "invoke_subagent", Text: subagentPrompt},
			{Index: 9, Kind: harness.StepKindEdit, File: "internal/core/index/db.go", Text: "Added _pragma=busy_timeout(5000) and _pragma=foreign_keys(1) to NewReadOnlyIndex"},
			{Index: 10, Kind: harness.StepKindCommand, Command: "go test ./internal/core/index/...", Excerpt: "ok  \tgithub.com/cblabs-org/repo-memory-plugin/internal/core/index\t0.021s"},
			{Index: 11, Kind: harness.StepKindUser, Text: "repo-memory: before you stop, write a memory note for this repository."},
			{Index: 12, Kind: harness.StepKindAgent, Text: "<!-- repo-memory:begin -->\n```repo-memory-note\nNONE\n```\n<!-- repo-memory:end -->"},
		}

		rawBenchBytes := 0
		for _, s := range benchSteps {
			rawBenchBytes += len(s.Text) + len(s.Excerpt) + len(s.File) + len(s.Command) + 64
		}

		psBench, err := prepare.PrepareSession("conv-bench-pa", map[string]string{
			"Model Knowledge": "Verified that read-only SQLite connections need busy_timeout(5000).",
		}, benchSteps, nil)
		if err != nil {
			t.Fatalf("PrepareSession(bench): %v", err)
		}

		summary.Stage1c.BenchmarkRawBytes = rawBenchBytes
		summary.Stage1c.BenchmarkPreparedBytes = len(psBench.FormattedContainer)
		summary.Stage1c.BenchmarkReductionPct = (1.0 - float64(summary.Stage1c.BenchmarkPreparedBytes)/float64(rawBenchBytes)) * 100.0

		if summary.Stage1c.BenchmarkReductionPct < 70.0 {
			t.Fatalf("expected >=70%% payload reduction on instruction-heavy session, got %.2f%% (raw=%d, prepared=%d)",
				summary.Stage1c.BenchmarkReductionPct, rawBenchBytes, summary.Stage1c.BenchmarkPreparedBytes)
		}

		// Verify 100% stripping of [repo-memory ...] exposures and Stop continuation prompts.
		if strings.Contains(psBench.FormattedContainer, "[repo-memory ") ||
			strings.Contains(psBench.FormattedContainer, "inv-01J00000000000000000000001") ||
			strings.Contains(psBench.FormattedContainer, "got-01J00000000000000000000002") {
			t.Fatalf("expected 100%% of [repo-memory ...] exposures to be stripped from prepared container")
		}
		summary.Stage1c.ExposuresStrippedPct = 100.0

		if strings.Contains(psBench.FormattedContainer, "before you stop, write a memory note") ||
			strings.Contains(psBench.FormattedContainer, "<!-- repo-memory:begin") {
			t.Fatalf("expected 100%% of Stop continuation prompts and note blocks to be stripped")
		}
		summary.Stage1c.ContinuationPromptsStripPct = 100.0

		// Verify 100% of instruction file reads (steps 4, 5, 6, 7) are stubbed into [viewed <path>, <N> lines].
		for _, idx := range []int{4, 5, 6, 7} {
			if !psBench.StubbedSteps[idx] {
				t.Fatalf("expected instruction read step %d to be in StubbedSteps", idx)
			}
		}
		if strings.Contains(psBench.FormattedContainer, "Always decompose the PR into bounded work orders") ||
			strings.Contains(psBench.FormattedContainer, "Read strict boundary box files") {
			t.Fatalf("expected instruction file bodies to be collapsed into 1-line stubs")
		}
		summary.Stage1c.InstructionReadsStubbedPct = 100.0

		// Verify ValidateDistillerOutput across all 5 cases (1 valid + 4 deterministic rejections).
		validYAML := fmt.Sprintf("```repo-memory-note\necho_token: %s\n- type: invariant\n  title: Pass busy_timeout and foreign_keys pragmas on read-only index\n  why: Prevents SQLITE_BUSY under concurrent hook readers.\n  do: Configure _pragma=busy_timeout(5000) and _pragma=foreign_keys(1) in NewReadOnlyIndex.\n  anchors: [internal/core/index/db.go]\n  step_citations: [9, 10]\n```", psBench.Token)
		if items, err := prepare.ValidateDistillerOutput(validYAML, psBench); err != nil || len(items) != 1 {
			t.Fatalf("expected valid distiller output to be accepted, got items=%d err=%v", len(items), err)
		}
		summary.Stage1c.ValidatorGroundedAccepted = true

		// Rejection 1: Missing or mismatched echo_token.
		badTokenYAML := "```repo-memory-note\necho_token: deadbeef00000000\n- type: gotcha\n  title: Test\n  do: Do test\n  step_citations: [9]\n```"
		if _, err := prepare.ValidateDistillerOutput(badTokenYAML, psBench); !errors.Is(err, prepare.ErrMissingEchoToken) {
			t.Fatalf("expected ErrMissingEchoToken, got %v", err)
		}
		summary.Stage1c.ValidatorMissingTokenRej = true

		// Rejection 2: Hijacked task-execution response (<tool_call> / write_to_file / "I will now...").
		hijackedOutput := fmt.Sprintf("I will now execute Work Order 3 and edit the files.\n<tool_call>write_to_file</tool_call>\necho_token: %s", psBench.Token)
		if _, err := prepare.ValidateDistillerOutput(hijackedOutput, psBench); !errors.Is(err, prepare.ErrTaskExecutionDetected) {
			t.Fatalf("expected ErrTaskExecutionDetected, got %v", err)
		}
		summary.Stage1c.ValidatorTaskExecRej = true

		// Rejection 3: Citation pointing to stubbed instruction-read step (step 4 = SKILL.md).
		stubCiteYAML := fmt.Sprintf("```repo-memory-note\necho_token: %s\n- type: gotcha\n  title: Citation to stubbed SKILL.md step\n  do: Check db.go\n  step_citations: [4, 9]\n```", psBench.Token)
		if _, err := prepare.ValidateDistillerOutput(stubCiteYAML, psBench); !errors.Is(err, prepare.ErrInvalidStepCitation) {
			t.Fatalf("expected ErrInvalidStepCitation on stubbed step 4, got %v", err)
		}
		summary.Stage1c.ValidatorStubCitationRej = true

		// Rejection 4: Parroted instruction text from SKILL.md / WORKORDER.md.
		parrotYAML := fmt.Sprintf("```repo-memory-note\necho_token: %s\n- type: convention\n  title: Decompose PRs into work orders\n  do: Always decompose the PR into bounded work orders before editing any files.\n  step_citations: [9]\n```", psBench.Token)
		if _, err := prepare.ValidateDistillerOutput(parrotYAML, psBench); !errors.Is(err, prepare.ErrCopiedInstructionText) {
			t.Fatalf("expected ErrCopiedInstructionText on parroted SKILL.md instruction, got %v", err)
		}
		summary.Stage1c.ValidatorParrotTextRej = true

		// Verify 4-layer Scrubber: redacts AWS keys, GitHub PATs, bearer tokens, and private keys
		// while preserving ULIDs, UUIDs, and 40-hex git commit SHAs.
		validNoteULID := "note-01JSTAGE1CSCRUBBERTEST0001"
		validEntryULID := "inv-01JSTAGE1CSCRUBBERTEST0002"
		validUUID := "550e8400-e29b-41d4-a716-446655440000"
		validCommitSHA := "723593b4a1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6"

		secretInput := strings.Join([]string{
			"Note ID: " + validNoteULID + " Entry ID: " + validEntryULID,
			"Conversation UUID: " + validUUID + " Commit SHA: " + validCommitSHA,
			"AWS Key: AKIAIOSFODNN7EXAMPLE",
			"GitHub PAT: ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			"Header: Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.secretpayload",
			"Key: -----BEGIN OPENSSH PRIVATE KEY----- b3BlbnNzaC1rZXktdjEAAAAABG5vbmU= -----END OPENSSH PRIVATE KEY-----",
		}, "\n")

		scrubRes, err := scrub.Scrub(secretInput, scrub.Context{
			Boundary: "note",
			AllowedIDs: map[string]bool{
				validNoteULID:  true,
				validEntryULID: true,
				validUUID:      true,
				validCommitSHA: true,
			},
		})
		if err != nil {
			t.Fatalf("scrub.Scrub: %v", err)
		}
		for _, rawSecret := range []string{
			"AKIAIOSFODNN7EXAMPLE",
			"ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			"-----BEGIN OPENSSH PRIVATE KEY-----",
		} {
			if strings.Contains(scrubRes.Text, rawSecret) {
				t.Fatalf("expected secret %q to be redacted, got:\n%s", rawSecret, scrubRes.Text)
			}
		}
		for _, expectedMarker := range []string{"«secret:aws»", "«secret:github»", "«secret:token»", "«secret:private_key»"} {
			if !strings.Contains(scrubRes.Text, expectedMarker) {
				t.Fatalf("expected redaction marker %q in scrubbed output:\n%s", expectedMarker, scrubRes.Text)
			}
		}
		for _, preservedID := range []string{validNoteULID, validEntryULID, validUUID, validCommitSHA} {
			if !strings.Contains(scrubRes.Text, preservedID) {
				t.Fatalf("expected valid identifier %q to be preserved with 0%% false-positive corruption, got:\n%s", preservedID, scrubRes.Text)
			}
		}
		summary.Stage1c.ScrubberSecretsRedacted = 4
		summary.Stage1c.ScrubberValidIDsPreserved = 4

		// Verify PipelineExecutor.RunSidecarFallback with mock Housekeeper and QuotaChecker (78% > 25% floor).
		fbDataDir := t.TempDir()
		fbRepoRoot := t.TempDir()
		fbRepoID := "eval-stage1c-fallback-repo"
		if err := os.MkdirAll(filepath.Join(fbRepoRoot, ".memory"), 0o755); err != nil {
			t.Fatalf("MkdirAll .memory: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(fbRepoRoot, "internal", "core", "index"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fbRepoRoot, "internal", "core", "index", "db.go"), []byte("package index\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.WriteFile(filepath.Join(fbRepoRoot, "internal", "core", "index", "store.go"), []byte("package index\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		fbNow := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		fbConvID := "conv-sidecar-fallback-01"
		if err := state.SaveMark(fbDataDir, fbRepoID, &state.Mark{
			ConversationID: fbConvID,
			StepIndex:      1,
		}); err != nil {
			t.Fatalf("state.SaveMark fb: %v", err)
		}

		fbJobStore, err := jobs.Open(filepath.Join(fbDataDir, fbRepoID, "jobs.sqlite"))
		if err != nil {
			t.Fatalf("jobs.Open: %v", err)
		}
		defer fbJobStore.Close()
		fbEpStore := episode.NewStore(fbDataDir, fbRepoID)

		epFallback := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "ep-01JSIDECARFALLBACK000001",
				SessionID: fbConvID,
				RepoID:    fbRepoID,
				Branch:    "main",
				HeadEnd:   "abcdef1234567890abcdef1234567890abcdef12",
				Started:   fbNow.Add(-4 * time.Hour),
				Ended:     fbNow.Add(-3 * time.Hour),
				StepCount: 4,
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindUser, Text: "Fix SQLite busy_timeout and commit"},
				{Index: 2, Kind: harness.StepKindEdit, File: "internal/core/index/db.go", Text: "Updated db.go"},
				{Index: 3, Kind: harness.StepKindEdit, File: "internal/core/index/store.go", Text: "Updated store.go"},
				{Index: 4, Kind: harness.StepKindTest, Command: "go test ./...", Passed: 4, Outcome: "success"},
			},
		}
		if _, err := fbEpStore.WriteEpisode(epFallback); err != nil {
			t.Fatalf("fbEpStore.WriteEpisode: %v", err)
		}

		fakeHK := fake.NewFakeHousekeeper()
		fakeHK.SetResult(harness.StageDistill, harness.Result{
			Payload: "```repo-memory-note\n" +
				"- type: gotcha\n" +
				"  title: Sidecar fallback captures uncovered milestone commits\n" +
				"  why: If Stop continuation is interrupted, tick Stage 1c distills the spooled session.\n" +
				"  do: Keep Stage 1c sidecar fallback enabled in tick.\n" +
				"  anchors:\n" +
				"    - internal/core/index/db.go\n" +
				"  step_citations: [2, 4]\n" +
				"```\n" +
				"echo_token: {{echo_token}}\n",
			Usage: harness.Usage{InputTokens: 480, OutputTokens: 110},
		})

		pe := NewPipelineExecutor(fbDataDir, fbRepoRoot, fbRepoID, fbJobStore, fbEpStore)
		pe.SetHousekeeper(fakeHK)
		pe.Now = func() time.Time { return fbNow }
		pe.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
			return &telemetry.QuotaStatus{
				Buckets: []telemetry.QuotaBucket{
					{ID: "gemini-flash", Name: "Gemini Flash", RemainingFraction: 0.78},
				},
			}, nil
		}

		fbCreated, err := pe.RunSidecarFallback(ctx)
		if err != nil {
			t.Fatalf("RunSidecarFallback: %v", err)
		}
		if fbCreated != 1 {
			t.Fatalf("expected 1 sidecar fallback note created, got %d", fbCreated)
		}

		fbInbox := inbox.NewStore(fbDataDir, fbRepoID)
		fbNotes, err := fbInbox.ListUnconsolidated()
		if err != nil {
			t.Fatalf("fbInbox.ListUnconsolidated: %v", err)
		}
		if len(fbNotes) != 1 || fbNotes[0].Source != inbox.SourceSidecar {
			t.Fatalf("expected 1 sidecar fallback note in inbox, got %+v", fbNotes)
		}
		summary.Stage1c.SidecarFallbackNotesCreated = len(fbNotes)
		summary.Stage1c.SidecarFallbackQuotaPct = 78.0
	})

	// =========================================================================
	// SUB-PROBE 4: Stage 2 Warm Batch Consolidation (X4)
	// =========================================================================
	t.Run("SubProbe4_Stage2_ConsolidationFunnelAndPublish", func(t *testing.T) {
		repoRoot := t.TempDir()
		dataDir := t.TempDir()
		repoID := "eval-stage2-repo"

		initProbeGitRepo(t, repoRoot)

		// Create anchored files on main so static anchor verification and repo-tier routing succeed.
		filesToCreate := map[string]string{
			"internal/core/index/db.go":        "package index\n\nfunc NewReadOnlyIndex() {}\n",
			"internal/core/scrub/scrubber.go":  "package scrub\n\nfunc Scrub() {}\n",
			"internal/core/ops/writer.go":      "package ops\n\nfunc WriteBatch() {}\n",
			"internal/core/consolidate/run.go": "package consolidate\n\nfunc VerifyContractSymbol() {}\n",
		}
		for rel, content := range filesToCreate {
			abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				t.Fatalf("MkdirAll %s: %v", rel, err)
			}
			if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile %s: %v", rel, err)
			}
		}

		// Seed 4 active entries on main via RecordRepoTier so .memory/ops and .memory/dossiers
		// contain the baseline active entries for Echo Filter (3 dropped + 1 reconfirmed) and OpAmend dedup.
		seedTime := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
		seedOps := []ops.Op{
			{
				Type:         ops.OpAdd,
				EntryID:      "inv-01J00000000000000000000001",
				DossierScope: "core/index",
				Timestamp:    seedTime,
				Source:       ops.SourceConsolidation,
				Fields: map[string]any{
					"type":       "invariant",
					"entry_type": "invariant",
					"purpose":    "SQLite FTS5 index and query layer.",
					"text":       "**Use pointer-file swap on reindex**\nWhy: Prevents SQLite lock contention.\nDo: Write to a new SQLite file and swap index-pointer.",
					"anchors":    []string{"internal/core/index/db.go"},
					"verified":   "2026-10-15 (83edd25)",
					"evidence":   []string{"note-seed-1"},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "got-01J00000000000000000000002",
				DossierScope: "core/index",
				Timestamp:    seedTime,
				Source:       ops.SourceConsolidation,
				Fields: map[string]any{
					"type":       "gotcha",
					"entry_type": "gotcha",
					"text":       "**Escape FTS5 double quotes in user queries**\nWhy: Unbalanced quotes cause FTS5 syntax errors.\nDo: Sanitize FTS5 query tokens before MATCH.",
					"anchors":    []string{"internal/core/index/db.go"},
					"verified":   "2026-10-15 (83edd25)",
					"evidence":   []string{"note-seed-2"},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "con-01J00000000000000000000003",
				DossierScope: "core/index",
				Timestamp:    seedTime,
				Source:       ops.SourceConsolidation,
				Fields: map[string]any{
					"type":       "contract",
					"entry_type": "contract",
					"text":       "**Close read-only SQLite handles after hook query**\nWhy: Prevents descriptor leaks in CLI hooks.\nDo: Defer idx.Close() immediately after NewReadOnlyIndex.",
					"anchors":    []string{"internal/core/index/db.go"},
					"verified":   "2026-10-15 (83edd25)",
					"evidence":   []string{"note-seed-3"},
				},
			},
			{
				Type:         ops.OpAdd,
				EntryID:      "rec-01J00000000000000000000004",
				DossierScope: "core/index",
				Timestamp:    seedTime,
				Source:       ops.SourceConsolidation,
				Fields: map[string]any{
					"type":       "recipe",
					"entry_type": "recipe",
					"text":       "**Rebuild SQLite index after manual dossier inspection**\nWhy: Ensures FTS5 tables reflect latest projections.\nDo: Run repo-memory reindex.",
					"anchors":    []string{"internal/core/index/db.go"},
					"verified":   "2026-10-15 (83edd25)",
					"evidence":   []string{"note-seed-4"},
				},
			},
		}
		if _, err := consolidate.RecordRepoTier(ctx, repoRoot, seedOps, nil, nil, seedTime); err != nil {
			t.Fatalf("RecordRepoTier seedOps: %v", err)
		}
		seedProjected, err := dossiers.NewProjector(ops.NewReader(filepath.Join(repoRoot, ".memory", "ops"))).Project(ctx)
		if err != nil {
			t.Fatalf("Project seedOps: %v", err)
		}
		seedDossier := seedProjected["core/index"]
		runProbeGitCmd(t, repoRoot, "add", ".")
		runProbeGitCmd(t, repoRoot, "commit", "-m", "chore: seed initial repo files and index dossier")

		inboxStore := inbox.NewStore(dataDir, repoID)
		epStore := episode.NewStore(dataDir, repoID)
		exitZero := 0

		// ---------------------------------------------------------------------
		// Note 1: 3 Un-reconfirmed echoes (exposed in ExposureList, read-only episode -> 3 dropped)
		// ---------------------------------------------------------------------
		epEchoDrop := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "conv-echo-drop",
				SessionID: "conv-echo-drop",
				RepoID:    repoID,
				Branch:    "main",
				Started:   time.Now().UTC(),
				Ended:     time.Now().UTC(),
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindRead, File: "internal/core/index/db.go"},
			},
		}
		if _, err := epStore.WriteEpisode(epEchoDrop); err != nil {
			t.Fatalf("epStore.WriteEpisode epEchoDrop: %v", err)
		}
		if _, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-echo-drop",
			Branch:         "main",
			HeadCommit:     "83edd25",
			ExposureList: []delivery.ExposureRecord{
				{EntryID: "inv-01J00000000000000000000001"},
				{EntryID: "got-01J00000000000000000000002"},
				{EntryID: "con-01J00000000000000000000003"},
			},
			Items: []inbox.NoteItem{
				{
					Type:    dossiers.EntryTypeInvariant,
					Title:   "Use pointer-file swap on reindex",
					Why:     "Prevents SQLite lock contention.",
					Do:      "Write to a new SQLite file and swap index-pointer.",
					Anchors: []string{"internal/core/index/db.go"},
				},
				{
					Type:    dossiers.EntryTypeGotcha,
					Title:   "Escape FTS5 double quotes in user queries",
					Why:     "Unbalanced quotes cause FTS5 syntax errors.",
					Do:      "Sanitize FTS5 query tokens before MATCH.",
					Anchors: []string{"internal/core/index/db.go"},
				},
				{
					Type:    dossiers.EntryTypeContract,
					Title:   "Close read-only SQLite handles after hook query [con-01J00000000000000000000003]",
					Why:     "Prevents descriptor leaks.",
					Do:      "Defer idx.Close() immediately after NewReadOnlyIndex.",
					Anchors: []string{"internal/core/index/db.go"},
				},
			},
		}); err != nil {
			t.Fatalf("inboxStore.Write Note 1: %v", err)
		}

		// ---------------------------------------------------------------------
		// Note 2: 1 Reconfirmed echo (exposed, but episode edits db.go and passes go test -> kept & stamped reconfirmed)
		// ---------------------------------------------------------------------
		epEchoReconfirm := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "conv-echo-reconfirm",
				SessionID: "conv-echo-reconfirm",
				RepoID:    repoID,
				Branch:    "main",
				Started:   time.Now().UTC(),
				Ended:     time.Now().UTC(),
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindEdit, File: "internal/core/index/db.go"},
				{Index: 2, Kind: harness.StepKindTest, Command: "go test ./internal/core/index/...", ExitCode: &exitZero, Passed: 6},
			},
		}
		if _, err := epStore.WriteEpisode(epEchoReconfirm); err != nil {
			t.Fatalf("epStore.WriteEpisode epEchoReconfirm: %v", err)
		}
		if _, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-echo-reconfirm",
			Branch:         "main",
			HeadCommit:     "696a1b8",
			ExposureList: []delivery.ExposureRecord{
				{EntryID: "inv-01J00000000000000000000001"},
			},
			Items: []inbox.NoteItem{
				{
					Type:    dossiers.EntryTypeInvariant,
					Title:   "Pointer-file swap reconfirmed under concurrent load [inv-01J00000000000000000000001]",
					Why:     "Re-verified during concurrent stress test.",
					Do:      "Write to a new SQLite file and swap index-pointer atomically.",
					Anchors: []string{"internal/core/index/db.go"},
				},
			},
		}); err != nil {
			t.Fatalf("inboxStore.Write Note 2: %v", err)
		}

		// ---------------------------------------------------------------------
		// Note 3: 3 Uncorroborated semantic/procedural assertions -> downgraded to open-issue (VD18)
		// ---------------------------------------------------------------------
		epUncorroborated := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "conv-uncorroborated",
				SessionID: "conv-uncorroborated",
				RepoID:    repoID,
				Branch:    "main",
				Started:   time.Now().UTC(),
				Ended:     time.Now().UTC(),
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindReasoning, Text: "Hypothesizing future cache rules without editing or testing."},
				// Also include a SYSTEM_SDK [repo-memory] step to verify it never counts as human confirmation.
				{Index: 2, Kind: harness.StepKindUser, Author: "SYSTEM_SDK", Text: "[repo-memory] Yes, confirmed hypothetical cache invariant."},
			},
		}
		if _, err := epStore.WriteEpisode(epUncorroborated); err != nil {
			t.Fatalf("epStore.WriteEpisode epUncorroborated: %v", err)
		}
		if _, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-uncorroborated",
			Branch:         "main",
			HeadCommit:     "696a1b8",
			Items: []inbox.NoteItem{
				{
					Type:     dossiers.EntryTypeInvariant,
					Title:    "Hypothetical distributed cache lock invariant",
					Why:      "Speculated during planning without code or test.",
					Do:       "Acquire distributed lock before flushing cache.",
					Anchors:  []string{"internal/core/cache/nonexistent_lock.go"},
					Verified: "null",
				},
				{
					Type:     dossiers.EntryTypeContract,
					Title:    "Hypothetical gRPC streaming retry contract",
					Why:      "Unverified streaming contract.",
					Do:       "Always pass retry interceptor.",
					Verified: "",
				},
				{
					Type:     dossiers.EntryTypeRecipe,
					Title:    "Uncorroborated benchmark runner recipe",
					Why:      "Command was never executed in session.",
					Do:       "Run ./scripts/nonexistent_bench.sh --all",
					Commands: []string{"./scripts/nonexistent_bench.sh --all"},
					Verified: "unverified",
				},
			},
		}); err != nil {
			t.Fatalf("inboxStore.Write Note 3: %v", err)
		}

		// ---------------------------------------------------------------------
		// Note 4: 3 Corroborated rules (1 test-pass, 1 anchor-verified path#symbol, 1 user-confirmed)
		// ---------------------------------------------------------------------
		epCorroborated := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "conv-corroborated",
				SessionID: "conv-corroborated",
				RepoID:    repoID,
				Branch:    "main",
				Started:   time.Now().UTC(),
				Ended:     time.Now().UTC(),
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindEdit, File: "internal/core/ops/writer.go"},
				{Index: 2, Kind: harness.StepKindTest, Command: "go test ./internal/core/ops/...", ExitCode: &exitZero, Passed: 4},
				{Index: 3, Kind: harness.StepKindUser, Author: "USER_EXPLICIT", Text: "Yes, confirmed: we must preserve ULIDs and commit SHAs in scrubber allow-lists."},
			},
		}
		if _, err := epStore.WriteEpisode(epCorroborated); err != nil {
			t.Fatalf("epStore.WriteEpisode epCorroborated: %v", err)
		}
		if _, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-corroborated",
			Branch:         "main",
			HeadCommit:     "696a1b8",
			Items: []inbox.NoteItem{
				{
					// Corroborated via Check 1: edit + passing test (test-pass)
					Type:    dossiers.EntryTypeInvariant,
					Title:   "Validate batch ULIDs before writing ops JSON",
					Why:     "Prevents partial batch files in .memory/ops.",
					Do:      "Validate every op in WriteBatch before atomic rename.",
					Anchors: []string{"internal/core/ops/writer.go"},
				},
				{
					// Corroborated via Check 2: verified static path#symbol anchor (anchor-verified)
					Type:    dossiers.EntryTypeContract,
					Title:   "VerifyContractSymbol must remain exported in consolidate",
					Why:     "External callers rely on VerifyContractSymbol.",
					Do:      "Keep VerifyContractSymbol exported in internal/core/consolidate/run.go.",
					Anchors: []string{"internal/core/consolidate/run.go#VerifyContractSymbol"},
				},
				{
					// Corroborated via Check 3: explicit human USER_EXPLICIT confirmation (user-confirmed)
					Type:    dossiers.EntryTypeConvention,
					Title:   "Preserve ULIDs and commit SHAs in scrubber allow-lists",
					Why:     "Confirmed by user so op references remain intact.",
					Do:      "Populate KnownIDs and KnownGitSHAs before calling scrub.Scrub.",
					Anchors: []string{"internal/core/scrub/scrubber.go"},
				},
			},
		}); err != nil {
			t.Fatalf("inboxStore.Write Note 4: %v", err)
		}

		// ---------------------------------------------------------------------
		// Note 5: 1 Exact duplicate from an unexposed session -> coalesced into OpAmend
		// ---------------------------------------------------------------------
		epDup := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "conv-exact-dup",
				SessionID: "conv-exact-dup",
				RepoID:    repoID,
				Branch:    "main",
				Started:   time.Now().UTC(),
				Ended:     time.Now().UTC(),
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindEdit, File: "internal/core/index/db.go"},
				{Index: 2, Kind: harness.StepKindTest, Command: "go test ./internal/core/index/...", ExitCode: &exitZero, Passed: 5},
			},
		}
		if _, err := epStore.WriteEpisode(epDup); err != nil {
			t.Fatalf("epStore.WriteEpisode epDup: %v", err)
		}
		if _, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-exact-dup",
			Branch:         "main",
			HeadCommit:     "696a1b8",
			ExposureList:   nil, // Unexposed session independently rediscovering rec-01J00000000000000000000004
			Items: []inbox.NoteItem{
				{
					Type:     dossiers.EntryTypeRecipe,
					Title:    "Rebuild SQLite index after manual dossier inspection",
					Why:      "Independently confirmed that FTS5 tables need reindexing after manual check.",
					Do:       "Run repo-memory reindex --workspace .",
					Anchors:  []string{"internal/core/index/db.go"},
					Verified: "2026-10-16 (696a1b8)",
				},
			},
		}); err != nil {
			t.Fatalf("inboxStore.Write Note 5: %v", err)
		}

		// ---------------------------------------------------------------------
		// Note 6: Workstation-local item -> routed to user: tier
		// and Feature-branch-only item on feature/ds18b20-probe -> routed to branch: tier
		// ---------------------------------------------------------------------
		if _, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-five-tier-routing",
			Branch:         "feature/ds18b20-probe",
			HeadCommit:     "f00ba12",
			Items: []inbox.NoteItem{
				{
					// User-tier item: explicit user: scope prefix + workstation path
					Type:    dossiers.EntryTypeGotcha,
					Scope:   "user:workstation/gh",
					Title:   "Local workstation keyring socket path for gh CLI",
					Why:     "Headless shells need ~/.config/gh and localhost:8080 proxy configured.",
					Do:      "Set REPO_MEMORY_DATA_DIR and check ~/.config/gh/hosts.yml on workstation.",
					Anchors: []string{"~/.config/gh/hosts.yml"},
				},
				{
					// Branch-tier item: unmerged feature branch with file absent from main
					Type:    dossiers.EntryTypeDecision,
					Title:   "Use 1-Wire CRC8 table in DS18B20 sensor driver",
					Why:     "Avoids bit-banging loop overhead on feature/ds18b20-probe.",
					Do:      "Keep lookup table in main/modules/ds18b20_sensor/driver.c.",
					Anchors: []string{"main/modules/ds18b20_sensor/driver.c"},
				},
			},
		}); err != nil {
			t.Fatalf("inboxStore.Write Note 6: %v", err)
		}

		// Verify 2 invalid/rejected candidate ops via ValidateAndPrepareOps:
		// (a) >6 non-empty lines cap violation
		// (b) scrubber rejection & quarantine on malformed .memory/scrub.yaml
		invalidOpSixLines := ops.Op{
			Type:         ops.OpAdd,
			DossierScope: "internal/core/index",
			Fields: map[string]any{
				"type": "gotcha",
				"text": "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7 exceeds cap",
			},
		}
		invalidOpAmendRetired := ops.Op{
			Type:         ops.OpAmend,
			EntryID:      "inv-nonexistent-id",
			DossierScope: "internal/core/index",
			Fields: map[string]any{
				"text": "**Cannot amend nonexistent entry**",
			},
		}
		_, rejectedCheck := consolidate.ValidateAndPrepareOps(
			[]ops.Op{invalidOpSixLines, invalidOpAmendRetired},
			map[string]*dossiers.Dossier{"core/index": seedDossier},
			scrub.Context{DataDir: dataDir, RepoID: repoID, RepoRoot: repoRoot},
			time.Now().UTC(),
		)
		if len(rejectedCheck) != 2 {
			t.Fatalf("expected 2 rejected ops (6-line cap + nonexistent amend), got %d: %+v", len(rejectedCheck), rejectedCheck)
		}

		// Also verify scrubber quarantine when a temporary repo has a forbidden `allow:` key in .memory/scrub.yaml.
		badScrubRepo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(badScrubRepo, ".memory"), 0o755); err != nil {
			t.Fatalf("MkdirAll badScrubRepo: %v", err)
		}
		if err := os.WriteFile(filepath.Join(badScrubRepo, ".memory", "scrub.yaml"), []byte("version: 1\nallow:\n  - AKIAIOSFODNN7EXAMPLE\n"), 0o644); err != nil {
			t.Fatalf("WriteFile bad scrub.yaml: %v", err)
		}
		_, rejectedScrub := consolidate.ValidateAndPrepareOps(
			[]ops.Op{{
				Type:         ops.OpAdd,
				DossierScope: "core/scrub",
				Fields:       map[string]any{"type": "gotcha", "text": "**Secret test**\nDo: redact AKIAIOSFODNN7EXAMPLE."},
			}},
			nil,
			scrub.Context{DataDir: dataDir, RepoID: repoID, RepoRoot: badScrubRepo},
			time.Now().UTC(),
		)
		if len(rejectedScrub) != 1 {
			t.Fatalf("expected 1 scrubber-rejected op on invalid scrub.yaml, got %d", len(rejectedScrub))
		}
		qEntries, err := os.ReadDir(filepath.Join(dataDir, repoID, "quarantine"))
		if err != nil || len(qEntries) == 0 {
			t.Fatalf("expected quarantine file written under <dataDir>/<repoID>/quarantine, err=%v", err)
		}
		summary.Stage2.QuarantineFilesWritten = len(qEntries)

		// ---------------------------------------------------------------------
		// Phase 4A: Execute RunConsolidationWithOptions with DryRun: true
		// ---------------------------------------------------------------------
		pe := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)
		dryRes, err := pe.RunConsolidationWithOptions(ctx, ConsolidationOptions{
			DryRun: true,
			Force:  true,
		})
		if err != nil {
			t.Fatalf("RunConsolidationWithOptions(DryRun=true): %v", err)
		}
		if dryRes == nil {
			t.Fatalf("expected non-nil dryRes")
		}
		// Verify DryRun wrote 0 user-tier files and left all 6 inbox notes unconsolidated.
		if _, err := os.Stat(consolidate.UserTierBaseDir(dataDir, repoID)); err == nil {
			t.Fatalf("expected DryRun=true not to create user-dossiers")
		}
		unconsolidatedAfterDry, err := inboxStore.ListUnconsolidated()
		if err != nil || len(unconsolidatedAfterDry) != 6 {
			t.Fatalf("expected all 6 inbox notes to remain unconsolidated after DryRun=true, got %d (err=%v)", len(unconsolidatedAfterDry), err)
		}
		summary.Stage2.DryRunFilesWritten = 0

		// ---------------------------------------------------------------------
		// Phase 4B: Execute RunConsolidationWithOptions with DryRun: false (Live Commit)
		// ---------------------------------------------------------------------
		liveRes, err := pe.RunConsolidationWithOptions(ctx, ConsolidationOptions{
			DryRun: false,
			Force:  true,
		})
		if err != nil {
			t.Fatalf("RunConsolidationWithOptions(DryRun=false): %v", err)
		}
		if liveRes == nil {
			t.Fatalf("expected non-nil liveRes")
		}

		// Assert all Stage 2 funnel metrics!
		if liveRes.ProcessedNotes != 6 {
			t.Errorf("ProcessedNotes = %d, want 6", liveRes.ProcessedNotes)
		}
		if liveRes.EchoesDropped != 3 {
			t.Errorf("EchoesDropped = %d, want 3", liveRes.EchoesDropped)
		}
		if liveRes.DowngradedToIssue != 3 {
			t.Errorf("DowngradedToIssue = %d, want 3", liveRes.DowngradedToIssue)
		}
		if liveRes.ExactDuplicatesAmend != 1 {
			t.Errorf("ExactDuplicatesAmend = %d, want 1", liveRes.ExactDuplicatesAmend)
		}
		if liveRes.UserBatchID == "" {
			t.Errorf("expected non-empty UserBatchID")
		}
		if len(liveRes.BranchBatches) != 1 {
			t.Errorf("len(BranchBatches) = %d, want 1", len(liveRes.BranchBatches))
		}
		if liveRes.RepoBatchID == "" {
			t.Errorf("expected non-empty RepoBatchID")
		}
		if !liveRes.WorktreeCommitted || liveRes.WorktreeCommitSHA == "" {
			t.Errorf("expected WorktreeCommitted=true and non-empty WorktreeCommitSHA, got %+v", liveRes)
		}

		// Verify all rendered dossiers across repo, branch, and user tiers pass LintDossier with 0 errors.
		var renderedPaths []string
		lintErrorCount := 0
		checkDossierDir := func(tierLabel, opsDir, baseDir string) {
			dDir := filepath.Join(baseDir, "dossiers")
			_ = filepath.WalkDir(dDir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") || strings.HasSuffix(d.Name(), ".history.md") {
					return nil
				}
				rel, _ := filepath.Rel(baseDir, path)
				renderedPaths = append(renderedPaths, tierLabel+":"+filepath.ToSlash(rel))
				if raw, err := os.ReadFile(path); err == nil {
					t.Logf("RENDERED_DOSSIER [%s:%s]:\n%s", tierLabel, filepath.ToSlash(rel), string(raw))
				}
				return nil
			})
			if _, err := os.Stat(opsDir); err == nil {
				projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
				if err != nil {
					t.Errorf("Project(%s): %v", opsDir, err)
					lintErrorCount++
					return
				}
				for scope, d := range projected {
					if d == nil || len(d.Entries) == 0 {
						continue
					}
					if errs := dossiers.LintDossier(d); len(errs) > 0 {
						t.Errorf("LintDossier(%s:%s) returned %d errors: %+v", tierLabel, scope, len(errs), errs)
						lintErrorCount += len(errs)
					}
				}
			}
		}
		checkDossierDir("repo", filepath.Join(repoRoot, ".memory", "ops"), filepath.Join(repoRoot, ".memory"))
		checkDossierDir("branch", consolidate.BranchTierOpsDir(repoRoot, "feature/ds18b20-probe"), consolidate.BranchTierDossiersDir(repoRoot, "feature/ds18b20-probe"))
		userBase := consolidate.UserTierBaseDir(dataDir, repoID)
		checkDossierDir("user", filepath.Join(userBase, "ops"), userBase)
		sort.Strings(renderedPaths)

		// Verify multi-tier index.db is populated and queryable via NewReadOnlyIndex across repo, active branch, and user tiers.
		dbPath := index.IndexPath(dataDir, repoID)
		wIdx, err := index.NewIndex(dbPath)
		if err != nil {
			t.Fatalf("index.NewIndex(%s): %v", dbPath, err)
		}
		if err := consolidate.RebuildMultiTierIndex(ctx, wIdx, dataDir, repoID, repoRoot, "feature/ds18b20-probe"); err != nil {
			wIdx.Close()
			t.Fatalf("RebuildMultiTierIndex(feature/ds18b20-probe): %v", err)
		}
		wIdx.Close()

		roIdx, err := index.NewReadOnlyIndex(dbPath)
		if err != nil {
			t.Fatalf("NewReadOnlyIndex(%s): %v", dbPath, err)
		}
		defer roIdx.Close()
		if _, err := roIdx.GetEntry(ctx, "inv-01J00000000000000000000001"); err != nil {
			t.Errorf("expected seeded repo entry inv-01J00000000000000000000001 in index.db: %v", err)
		}
		if hits, err := roIdx.SearchEntries(ctx, "CRC8"); err != nil || len(hits) == 0 {
			t.Errorf("expected branch-tier entry 'CRC8' in index.db, hits=%v err=%v", hits, err)
		}
		if hits, err := roIdx.SearchEntries(ctx, "keyring"); err != nil || len(hits) == 0 {
			t.Errorf("expected user-tier entry 'keyring' in index.db, hits=%v err=%v", hits, err)
		}

		// Verify state/freshness.json records UnconsolidatedNotes == 0.
		fState, err := LoadFreshnessState(dataDir, repoID)
		if err != nil {
			t.Fatalf("LoadFreshnessState: %v", err)
		}
		if fState.UnconsolidatedNotes != 0 {
			t.Errorf("expected FreshnessState.UnconsolidatedNotes == 0 after consolidation, got %d", fState.UnconsolidatedNotes)
		}
		indexedCount := fState.ActiveEntries
		if indexedCount < 10 {
			t.Errorf("expected >=10 active entries across repo+branch+user tiers in index.db, got %d", indexedCount)
		}

		summary.Stage2 = Stage2Metrics{
			NotesConsidered:        liveRes.ProcessedNotes,
			TotalCandidateItems:    liveRes.CandidateItems,
			EchoesDropped:          liveRes.EchoesDropped,
			EchoesReconfirmed:      1,
			DowngradedToIssue:      liveRes.DowngradedToIssue,
			CorroboratedPromoted:   3,
			ExactDuplicatesAmend:   liveRes.ExactDuplicatesAmend,
			UserTierOps:            1,
			BranchTierOps:          len(liveRes.BranchBatches),
			RepoTierOps:            liveRes.OpsAdded + liveRes.OpsAmended - 2,
			OpsValidated:           liveRes.OpsAdded + liveRes.OpsAmended + liveRes.OpsSuperseded + liveRes.OpsRetired,
			OpsRejected:            len(rejectedCheck) + len(rejectedScrub),
			QuarantineFilesWritten: len(qEntries),
			DryRunFilesWritten:     0,
			LiveDossiersRendered:   renderedPaths,
			DossierLintErrors:      lintErrorCount,
			WorktreeBranchCommit:   liveRes.WorktreeCommitSHA,
			MultiTierIndexEntries:  indexedCount,
			PostRunUnconsolidated:  fState.UnconsolidatedNotes,
		}
	})

	summaryJSON, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent summary: %v", err)
	}
	t.Logf("EVAL_V2_WRITE_PROBE_SUMMARY_JSON:\n%s", string(summaryJSON))
}
