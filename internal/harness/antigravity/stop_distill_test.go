package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
)

func writeTranscriptSteps(t *testing.T, convID string, lines []string) string {
	t.Helper()
	transcriptDir := filepath.Join(t.TempDir(), "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("creating transcript dir: %v", err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}
	return transcriptPath
}

func TestStopDistill_FiveMilestones_Continue(t *testing.T) {
	ctx := context.Background()

	t.Run("1. git commit milestone", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		convID := "conv-ms-commit"
		lines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"commit the fix"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m 'fix bug'"}}]}`,
			`{"step_index":2,"source":"SYSTEM","type":"GENERIC","status":"DONE","content":"[main abc1234] fix bug\n 1 file changed"}`,
			`{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Committed."}`,
		}
		tp := writeTranscriptSteps(t, convID, lines)

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil {
			t.Fatalf("HandleStop error: %v", err)
		}
		var resp map[string]string
		if err := json.Unmarshal(out, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp["decision"] != "continue" {
			t.Fatalf("expected decision=continue, got %s", string(out))
		}
		if !strings.Contains(resp["reason"], "```repo-memory-note") {
			t.Errorf("expected canonical prompt with repo-memory-note block, got %s", resp["reason"])
		}
	})

	t.Run("2. gh pr create milestone", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		convID := "conv-ms-pr"
		lines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"open a PR"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"function":{"name":"bash","arguments":"{\"command\":\"gh pr create --title test --body test\"}"}}]}`,
			`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"PR opened."}`,
		}
		tp := writeTranscriptSteps(t, convID, lines)

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil {
			t.Fatalf("HandleStop error: %v", err)
		}
		var resp map[string]string
		_ = json.Unmarshal(out, &resp)
		if resp["decision"] != "continue" {
			t.Fatalf("expected decision=continue for gh pr create, got %s", string(out))
		}
	})

	t.Run("3. WORKORDER.md Completion Report milestone", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		convID := "conv-ms-wo"
		lines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"finish WO"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"edit_file","args":{"Path":".agents/skills/pa/wo1/WORKORDER.md","Replacement":"## Completion Report\nAll tests pass."}}]}`,
			`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Updated work order."}`,
		}
		tp := writeTranscriptSteps(t, convID, lines)

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil {
			t.Fatalf("HandleStop error: %v", err)
		}
		var resp map[string]string
		_ = json.Unmarshal(out, &resp)
		if resp["decision"] != "continue" {
			t.Fatalf("expected decision=continue for WORKORDER.md completion report, got %s", string(out))
		}
	})

	t.Run("4. Command/test failure resolved after >= 15 steps", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		convID := "conv-ms-err-resolved"
		var lines []string
		lines = append(lines, `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"fix failing tests"}`)
		lines = append(lines, `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"go test ./..."}}]}`)
		lines = append(lines, `{"step_index":2,"source":"SYSTEM","type":"GENERIC","status":"ERROR","content":"--- FAIL: TestFoo\nFAIL\texample/pkg\nexit status 1"}`)
		for i := 3; i <= 15; i++ {
			lines = append(lines, fmt.Sprintf(`{"step_index":%d,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"debugging step %d"}`, i, i))
		}
		// Step 16: 16 - 1 = 15 >= 15, go test succeeds
		lines = append(lines, `{"step_index":16,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"go test ./..."}}]}`)
		lines = append(lines, `{"step_index":17,"source":"SYSTEM","type":"GENERIC","status":"DONE","content":"ok  \texample/pkg\t0.012s"}`)
		lines = append(lines, `{"step_index":18,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Tests fixed."}`)

		tp := writeTranscriptSteps(t, convID, lines)

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil {
			t.Fatalf("HandleStop error: %v", err)
		}
		var resp map[string]string
		_ = json.Unmarshal(out, &resp)
		if resp["decision"] != "continue" {
			t.Fatalf("expected decision=continue for test failure resolved after >= 15 steps, got %s", string(out))
		}
	})

	t.Run("5. Edits to >= 2 distinct files followed by go test", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		convID := "conv-ms-2edits"
		lines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"update two files"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"write_to_file","args":{"TargetFile":"internal/a.go","CodeContent":"package a"}}]}`,
			`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"replace_file_content","args":{"TargetFile":"internal/b.go","ReplacementContent":"package b"}}]}`,
			`{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"go test ./..."}}]}`,
			`{"step_index":4,"source":"SYSTEM","type":"GENERIC","status":"DONE","content":"ok"}`,
			`{"step_index":5,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Done."}`,
		}
		tp := writeTranscriptSteps(t, convID, lines)

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil {
			t.Fatalf("HandleStop error: %v", err)
		}
		var resp map[string]string
		_ = json.Unmarshal(out, &resp)
		if resp["decision"] != "continue" {
			t.Fatalf("expected decision=continue for >=2 file edits + go test, got %s", string(out))
		}
	})
}

func TestStopDistill_GateRejections(t *testing.T) {
	ctx := context.Background()

	commitLines := []string{
		`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"commit"}`,
		`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m 'msg'"}}]}`,
		`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"done"}`,
	}

	t.Run("no milestone in steps", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		convID := "conv-no-ms"
		lines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"explain code"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Here is the explanation."}`,
		}
		tp := writeTranscriptSteps(t, convID, lines)
		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil || string(out) != "{}" {
			t.Fatalf("expected '{}', got %s (err=%v)", string(out), err)
		}
	})

	t.Run("terminationReason != NO_TOOL_CALL or fullyIdle == false", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		convID := "conv-not-idle"
		tp := writeTranscriptSteps(t, convID, commitLines)

		// Case A: wrong terminationReason
		rawA, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "USER_CANCELLED",
			FullyIdle:         true,
		})
		outA, _ := HandleStop(ctx, rawA, WithDataDir(dataDir))
		if string(outA) != "{}" {
			t.Errorf("expected '{}' for USER_CANCELLED, got %s", string(outA))
		}

		// Case B: fullyIdle == false
		rawB, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         false,
		})
		outB, _ := HandleStop(ctx, rawB, WithDataDir(dataDir))
		if string(outB) != "{}" {
			t.Errorf("expected '{}' when fullyIdle=false, got %s", string(outB))
		}
	})

	t.Run("subagent conversation excluded via ParentConversationID or SubagentStore", func(t *testing.T) {
		dataDir, repoDir, repoID := setupEnrolledRepo(t)
		convID1 := "conv-sub-payload"
		tp1 := writeTranscriptSteps(t, convID1, commitLines)

		raw1, _ := json.Marshal(StopPayload{
			ConversationID:       convID1,
			ParentConversationID: "conv-parent-1",
			TranscriptPath:       tp1,
			WorkspacePaths:       []string{repoDir},
			ExecutionNum:         0,
			TerminationReason:    "NO_TOOL_CALL",
			FullyIdle:            true,
		})
		out1, _ := HandleStop(ctx, raw1, WithDataDir(dataDir))
		if string(out1) != "{}" {
			t.Errorf("expected '{}' for payload with ParentConversationID, got %s", string(out1))
		}

		convID2 := "conv-sub-sqlite"
		tp2 := writeTranscriptSteps(t, convID2, commitLines)
		if err := state.RecordSubagent(ctx, dataDir, repoID, convID2, "conv-parent-2", "worker"); err != nil {
			t.Fatalf("RecordSubagent: %v", err)
		}

		raw2, _ := json.Marshal(StopPayload{
			ConversationID:    convID2,
			TranscriptPath:    tp2,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out2, _ := HandleStop(ctx, raw2, WithDataDir(dataDir))
		if string(out2) != "{}" {
			t.Errorf("expected '{}' for registered subagent in subagents.sqlite, got %s", string(out2))
		}
	})

	t.Run("turn_notes off and fallback-only skip Stop continuation", func(t *testing.T) {
		for _, mode := range []string{"off", "fallback-only"} {
			dataDir, repoDir, _ := setupEnrolledRepo(t)
			repoCfg, _ := config.LoadRepoConfig(repoDir)
			repoCfg.TurnNotes = mode
			if err := config.SaveRepoConfig(repoDir, repoCfg); err != nil {
				t.Fatalf("SaveRepoConfig: %v", err)
			}

			convID := "conv-mode-" + mode
			tp := writeTranscriptSteps(t, convID, commitLines)
			raw, _ := json.Marshal(StopPayload{
				ConversationID:    convID,
				TranscriptPath:    tp,
				WorkspacePaths:    []string{repoDir},
				ExecutionNum:      0,
				TerminationReason: "NO_TOOL_CALL",
				FullyIdle:         true,
			})
			out, _ := HandleStop(ctx, raw, WithDataDir(dataDir))
			if string(out) != "{}" {
				t.Errorf("expected '{}' for turn_notes=%s, got %s", mode, string(out))
			}
		}
	})

	t.Run("role opt-out and /goal active excluded", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		repoCfg, _ := config.LoadRepoConfig(repoDir)
		repoCfg.TurnNotesRoleOptOut = []string{"principal-architect"}
		_ = config.SaveRepoConfig(repoDir, repoCfg)

		// Role opt-out match
		convRole := "conv-role-optout"
		roleLines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"Act as the Principal-Architect for this repo"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m 'plan'"}}]}`,
		}
		tpRole := writeTranscriptSteps(t, convRole, roleLines)
		rawRole, _ := json.Marshal(StopPayload{
			ConversationID:    convRole,
			TranscriptPath:    tpRole,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		outRole, _ := HandleStop(ctx, rawRole, WithDataDir(dataDir))
		if string(outRole) != "{}" {
			t.Errorf("expected '{}' for opted-out role, got %s", string(outRole))
		}

		// /goal active match
		convGoal := "conv-goal-active"
		goalLines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"/goal migrate all tests overnight"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m 'wip'"}}]}`,
		}
		tpGoal := writeTranscriptSteps(t, convGoal, goalLines)
		rawGoal, _ := json.Marshal(StopPayload{
			ConversationID:    convGoal,
			TranscriptPath:    tpGoal,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		outGoal, _ := HandleStop(ctx, rawGoal, WithDataDir(dataDir))
		if string(outGoal) != "{}" {
			t.Errorf("expected '{}' for /goal session, got %s", string(outGoal))
		}
	})

	t.Run("30m cooldown blocks non-commit milestone but is waived for git commit and gh pr create", func(t *testing.T) {
		dataDir, repoDir, repoID := setupEnrolledRepo(t)
		convID := "conv-cooldown"

		// Pre-populate a mark with LastNoteAt = 5 minutes ago at step 1
		m := &state.Mark{
			ConversationID: convID,
			StepIndex:      1,
			LastNoteAt:     time.Now().UTC().Add(-5 * time.Minute),
			NoteCount:      1,
		}
		if err := state.SaveMark(dataDir, repoID, m); err != nil {
			t.Fatalf("SaveMark: %v", err)
		}

		// Non-commit milestone (WORKORDER.md completion report) at step 2 -> blocked by cooldown
		woLines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"start"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"step 1"}`,
			`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"edit_file","args":{"Path":"WORKORDER.md","Replacement":"## Completion Report\nDone"}}]}`,
			`{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"done"}`,
		}
		tpWO := writeTranscriptSteps(t, convID, woLines)
		rawWO, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tpWO,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		outWO, _ := HandleStop(ctx, rawWO, WithDataDir(dataDir))
		if string(outWO) != "{}" {
			t.Fatalf("expected cooldown to block non-commit milestone, got %s", string(outWO))
		}

		// Commit milestone at step 2 -> waives cooldown!
		commitAfterMark := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"start"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"step 1"}`,
			`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m 'new commit'"}}]}`,
			`{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"committed"}`,
		}
		tpCommit := writeTranscriptSteps(t, convID, commitAfterMark)
		rawCommit, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tpCommit,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		outCommit, _ := HandleStop(ctx, rawCommit, WithDataDir(dataDir))
		var resp map[string]string
		_ = json.Unmarshal(outCommit, &resp)
		if resp["decision"] != "continue" {
			t.Fatalf("expected git commit to waive 30m cooldown, got %s", string(outCommit))
		}
	})

	t.Run("daily cap of 20 turn-note notes blocks 21st request", func(t *testing.T) {
		dataDir, repoDir, repoID := setupEnrolledRepo(t)
		store := inbox.NewStore(dataDir, repoID)
		for i := 0; i < TurnNoteDailyCap; i++ {
			_, err := store.Write(&inbox.Note{
				Source:         inbox.SourceTurnNote,
				ConversationID: fmt.Sprintf("conv-earlier-%d", i),
				Items: []inbox.NoteItem{
					{Type: "decision", Title: "t", Why: "w"},
				},
			})
			if err != nil {
				t.Fatalf("writing note %d: %v", i, err)
			}
		}

		convID := "conv-cap-21"
		tp := writeTranscriptSteps(t, convID, commitLines)
		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, _ := HandleStop(ctx, raw, WithDataDir(dataDir))
		if string(out) != "{}" {
			t.Fatalf("expected daily cap of 20 to block 21st request, got %s", string(out))
		}
	})

	t.Run("mid-turn memory_note MCP call advancing mark deduplicates Stop gate", func(t *testing.T) {
		dataDir, repoDir, repoID := setupEnrolledRepo(t)
		convID := "conv-mcp-dedup"
		tp := writeTranscriptSteps(t, convID, commitLines)

		// Simulate memory_note advancing mark.StepIndex to 2 (covering the commit at step 1)
		if err := state.SaveMark(dataDir, repoID, &state.Mark{
			ConversationID: convID,
			StepIndex:      2,
			LastNoteAt:     time.Now().UTC(),
			NoteCount:      1,
			NoteTitles:     []string{"Recorded via MCP"},
		}); err != nil {
			t.Fatalf("SaveMark: %v", err)
		}

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, _ := HandleStop(ctx, raw, WithDataDir(dataDir))
		if string(out) != "{}" {
			t.Fatalf("expected Stop gate to deduplicate after mid-turn memory_note advanced mark, got %s", string(out))
		}
	})

	t.Run("REPO_MEMORY_INTERNAL=1 skips Stop hook", func(t *testing.T) {
		dataDir, repoDir, _ := setupEnrolledRepo(t)
		t.Setenv("REPO_MEMORY_INTERNAL", "1")
		convID := "conv-internal"
		tp := writeTranscriptSteps(t, convID, commitLines)
		res, err := HandleStopEvent(ctx, &StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      0,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		}, WithDataDir(dataDir))
		if err != nil {
			t.Fatalf("HandleStopEvent: %v", err)
		}
		if !res.Skipped || !strings.Contains(res.Reason, "REPO_MEMORY_INTERNAL") {
			t.Fatalf("expected skip due to REPO_MEMORY_INTERNAL, got %+v", res)
		}
	})
}

func TestStopDistill_HarvestPass(t *testing.T) {
	ctx := context.Background()

	t.Run("valid repo-memory-note block writes turn-note, checks anchors, attaches ExposureList, scrubs secrets, and advances mark", func(t *testing.T) {
		dataDir, repoDir, repoID := setupEnrolledRepo(t)
		convID := "conv-harvest-valid"

		// Record an exposure in the conversation ledger
		_ = ModifyLedger(dataDir, repoID, convID, func(l *delivery.Ledger) {
			l.RecordInjection(delivery.InjectionRecord{
				EntryID:   "inv-EXISTING1",
				Channel:   delivery.ChannelUser,
				Moment:    delivery.MomentTaskStart,
				StepIndex: 0,
				Timestamp: time.Now().UTC(),
			})
		})

		// Save mark from executionNum == 0 request at step 2
		if err := state.SaveMark(dataDir, repoID, &state.Mark{
			ConversationID:     convID,
			StepIndex:          0,
			PendingRequestStep: 2,
		}); err != nil {
			t.Fatalf("SaveMark: %v", err)
		}

		noteReply := "Here is the note:\n```repo-memory-note\n" +
			"- type: ruled-out\n" +
			"  title: Relative paths in hooks.json\n" +
			"  why: Hook commands run from the hooks.json directory using key AKIAIOSFODNN7EXAMPLE.\n" +
			"  do: Always generate absolute paths at install time.\n" +
			"  anchors:\n" +
			"    - README.md\n" +
			"    - nonexistent/missing.go\n" +
			"  evidence: verified in test\n" +
			"```"
		replyJSON, _ := json.Marshal(noteReply)

		lines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-10-16T06:00:00Z","content":"fix hooks"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-16T06:00:05Z","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m 'fix'"}}]}`,
			`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-16T06:00:10Z","status":"DONE","content":"Done."}`,
			`{"step_index":3,"source":"SYSTEM","type":"USER_INPUT","created_at":"2026-10-16T06:00:11Z","content":"repo-memory: before you stop..."}`,
			fmt.Sprintf(`{"step_index":4,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-16T06:00:15Z","status":"DONE","content":%s}`, string(replyJSON)),
		}
		tp := writeTranscriptSteps(t, convID, lines)

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      1,
			ModelName:         "gemini-2.5-pro",
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil {
			t.Fatalf("HandleStop harvest failed: %v", err)
		}
		if string(out) != "{}" {
			t.Fatalf("expected '{}' on harvest pass, got %s", string(out))
		}

		// Check inbox store
		store := inbox.NewStore(dataDir, repoID)
		notes, err := store.ListAll()
		if err != nil {
			t.Fatalf("ListAll notes: %v", err)
		}
		if len(notes) != 1 {
			t.Fatalf("expected 1 inbox note, got %d", len(notes))
		}
		n := notes[0]
		if n.Source != inbox.SourceTurnNote {
			t.Errorf("expected source turn-note, got %s", n.Source)
		}
		if n.StepsCovered != [2]int{0, 4} {
			t.Errorf("expected StepsCovered [0, 4], got %v", n.StepsCovered)
		}
		if len(n.ExposureList) != 1 || n.ExposureList[0].EntryID != "inv-EXISTING1" {
			t.Errorf("expected ExposureList with inv-EXISTING1, got %+v", n.ExposureList)
		}
		if len(n.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(n.Items))
		}
		item := n.Items[0]
		if strings.Contains(item.Why, "AKIAIOSFODNN7EXAMPLE") || !strings.Contains(item.Why, "«secret:aws_key»") {
			t.Errorf("expected AWS key to be scrubbed in Why, got %q", item.Why)
		}
		if len(item.AnchorChecks) != 2 {
			t.Fatalf("expected 2 anchor checks, got %d", len(item.AnchorChecks))
		}
		if !item.AnchorChecks[0].Exists || item.AnchorChecks[1].Exists {
			t.Errorf("expected README.md exists=true and nonexistent/missing.go exists=false, got %+v", item.AnchorChecks)
		}

		// Check updated mark
		mark, err := state.LoadMark(dataDir, repoID, convID)
		if err != nil {
			t.Fatalf("LoadMark: %v", err)
		}
		if mark.StepIndex != 4 {
			t.Errorf("expected mark.StepIndex=4, got %d", mark.StepIndex)
		}
		if mark.PendingRequestStep != 0 {
			t.Errorf("expected mark.PendingRequestStep=0, got %d", mark.PendingRequestStep)
		}
		if mark.NoteCount != 1 || len(mark.NoteTitles) != 1 || mark.NoteTitles[0] != "Relative paths in hooks.json" {
			t.Errorf("unexpected mark note metadata: %+v", mark)
		}
		if mark.LastNoteAt.IsZero() {
			t.Error("expected mark.LastNoteAt to be set")
		}
	})

	t.Run("NONE reply advances mark and writes no inbox note", func(t *testing.T) {
		dataDir, repoDir, repoID := setupEnrolledRepo(t)
		convID := "conv-harvest-none"
		_ = state.SaveMark(dataDir, repoID, &state.Mark{
			ConversationID:     convID,
			StepIndex:          0,
			PendingRequestStep: 1,
		})

		lines := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"commit"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Done."}`,
			`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-16T06:10:00Z","status":"DONE","content":"NONE"}`,
		}
		tp := writeTranscriptSteps(t, convID, lines)

		raw, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      1,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
		if err != nil || string(out) != "{}" {
			t.Fatalf("expected '{}', got %s (err=%v)", string(out), err)
		}

		notes, _ := inbox.NewStore(dataDir, repoID).ListAll()
		if len(notes) != 0 {
			t.Errorf("expected 0 notes for NONE reply, got %d", len(notes))
		}
		mark, _ := state.LoadMark(dataDir, repoID, convID)
		if mark.StepIndex != 2 || mark.PendingRequestStep != 0 {
			t.Errorf("expected mark.StepIndex=2 and PendingRequestStep=0, got %+v", mark)
		}
	})

	t.Run("malformed reply or reply with tool calls advances mark (loop protection) and writes no note", func(t *testing.T) {
		dataDir, repoDir, repoID := setupEnrolledRepo(t)
		convID := "conv-harvest-malformed"
		_ = state.SaveMark(dataDir, repoID, &state.Mark{
			ConversationID:     convID,
			StepIndex:          0,
			PendingRequestStep: 1,
		})

		// Case 1: reply contains tool calls
		toolCallReplyJSON, _ := json.Marshal("```repo-memory-note\n- type: decision\n  title: t\n  why: w\n```")
		linesWithTools := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"commit"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Done."}`,
			fmt.Sprintf(`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":%s,"tool_calls":[{"name":"run_command","args":{"CommandLine":"ls"}}]}`, string(toolCallReplyJSON)),
		}
		tp1 := writeTranscriptSteps(t, convID, linesWithTools)
		raw1, _ := json.Marshal(StopPayload{
			ConversationID:    convID,
			TranscriptPath:    tp1,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      1,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out1, _ := HandleStop(ctx, raw1, WithDataDir(dataDir))
		if string(out1) != "{}" {
			t.Fatalf("expected '{}' when tool calls present, got %s", string(out1))
		}
		notes1, _ := inbox.NewStore(dataDir, repoID).ListAll()
		if len(notes1) != 0 {
			t.Errorf("expected 0 notes when reply had tool calls, got %d", len(notes1))
		}
		mark1, _ := state.LoadMark(dataDir, repoID, convID)
		if mark1.StepIndex != 2 {
			t.Errorf("expected mark.StepIndex advanced to 2, got %d", mark1.StepIndex)
		}

		// Case 2: malformed YAML without fenced block
		convID2 := "conv-harvest-bad-yaml"
		linesBad := []string{
			`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"commit"}`,
			`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"Sorry, I forgot the format."}`,
		}
		tp2 := writeTranscriptSteps(t, convID2, linesBad)
		raw2, _ := json.Marshal(StopPayload{
			ConversationID:    convID2,
			TranscriptPath:    tp2,
			WorkspacePaths:    []string{repoDir},
			ExecutionNum:      1,
			TerminationReason: "NO_TOOL_CALL",
			FullyIdle:         true,
		})
		out2, _ := HandleStop(ctx, raw2, WithDataDir(dataDir))
		if string(out2) != "{}" {
			t.Fatalf("expected '{}' on malformed reply, got %s", string(out2))
		}
		mark2, _ := state.LoadMark(dataDir, repoID, convID2)
		if mark2.StepIndex != 1 {
			t.Errorf("expected mark.StepIndex advanced to 1, got %d", mark2.StepIndex)
		}
	})
}

func TestStopDistill_RewindClampingAndCheckpointTitles(t *testing.T) {
	ctx := context.Background()
	dataDir, repoDir, repoID := setupEnrolledRepo(t)
	convID := "conv-rewind-stop"

	// Mark is at step 15 from before a /rewind, with an earlier note title
	if err := state.SaveMark(dataDir, repoID, &state.Mark{
		ConversationID: convID,
		StepIndex:      15,
		ByteOffset:     9999,
		StepTimestamp:  "2026-10-16T05:00:00Z",
		NoteTitles:     []string{"Earlier architectural decision"},
		NoteCount:      1,
	}); err != nil {
		t.Fatalf("SaveMark: %v", err)
	}

	// Transcript rewound to 4 steps, includes a CHECKPOINT and a git commit
	lines := []string{
		`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-10-16T06:00:00Z","content":"try again"}`,
		`CHECKPOINT`,
		`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-16T06:01:00Z","status":"DONE","tool_calls":[{"name":"run_command","args":{"CommandLine":"git commit -m 'rewound commit'"}}]}`,
		`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-16T06:01:05Z","status":"DONE","content":"Committed after rewind."}`,
	}
	tp := writeTranscriptSteps(t, convID, lines)

	raw, _ := json.Marshal(StopPayload{
		ConversationID:    convID,
		TranscriptPath:    tp,
		WorkspacePaths:    []string{repoDir},
		ExecutionNum:      0,
		TerminationReason: "NO_TOOL_CALL",
		FullyIdle:         true,
	})
	out, err := HandleStop(ctx, raw, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("HandleStop failed: %v", err)
	}
	var resp map[string]string
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["decision"] != "continue" {
		t.Fatalf("expected rewind clamping to allow detecting new milestone at step 1, got %s", string(out))
	}
	if !strings.Contains(resp["reason"], "Earlier architectural decision") {
		t.Errorf("expected continuation prompt after CHECKPOINT to include earlier note title, got %s", resp["reason"])
	}

	mark, _ := state.LoadMark(dataDir, repoID, convID)
	if mark.StepIndex != 0 || mark.PendingRequestStep != 2 {
		t.Errorf("expected clamped mark with StepIndex=0 and PendingRequestStep=2, got %+v", mark)
	}
}

func TestLintNoteItems_RulesAndCap(t *testing.T) {
	items := []inbox.NoteItem{
		{Type: "procedure", Title: "Proc without why", Do: "run make"},                         // valid (procedure does not require why)
		{Type: "recipe", Title: "Recipe without why", Do: "run go test"},                       // valid
		{Type: "decision", Title: "Decision missing why"},                                      // invalid (missing why)
		{Type: "ruled-out", Title: "Ruled out missing do", Why: "failed"},                      // invalid (missing do)
		{Type: "invariant", Title: "Invariant with why and do", Why: "safety", Do: "check"},    // valid
		{Type: "failure-signature", Title: "Fail sig", Why: "cause", Do: "fix"},                // valid
		{Type: "gotcha", Title: "Gotcha", Why: "quirk"},                                        // valid
		{Type: "open-issue", Title: "Issue", Why: "bug", ClosesWhen: "pr merged"},              // valid
		{Type: "contract", Title: "Seventh valid item capped", Why: "contract why"},            // capped (7th valid item)
		{Type: "unknown-type", Title: "Bad type", Why: "w", Do: "d"},                           // invalid type
	}

	valid := LintNoteItems(items)
	if len(valid) != MaxNoteItems {
		t.Fatalf("expected %d valid items (capped at MaxNoteItems), got %d", MaxNoteItems, len(valid))
	}
	if valid[0].Title != "Proc without why" || valid[5].Title != "Issue" {
		t.Errorf("unexpected valid items order: %+v", valid)
	}
}

