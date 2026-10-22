/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package consolidate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestBuildConsolidationPrompt_DataFraming(t *testing.T) {
	token, err := GenerateConsolidationEchoToken()
	if err != nil {
		t.Fatalf("GenerateConsolidationEchoToken failed: %v", err)
	}
	if len(token) != 16 {
		t.Fatalf("expected 16-char hex token, got %q (len=%d)", token, len(token))
	}

	input := DossierConsolidationInput{
		Scope: "internal/harness",
		Kind:  dossiers.DossierKindModule,
		Tier:  TierRepo,
		ExistingEntries: []dossiers.Entry{
			{
				ID:       "inv-01EXISTING",
				Type:     dossiers.EntryTypeInvariant,
				Text:     "**Keep core pure**\nWhy: Architectural boundary\nDo: Never import concrete harness adapters",
				Anchors:  []string{"internal/harness/interfaces.go"},
				Evidence: []string{"note-01OLD"},
				Verified: "test-pass (2026-10-15)",
			},
		},
		CandidateItems: []RoutedItem{
			{
				NoteID:         "note-01NEW",
				NoteSource:     inbox.SourceTurnNote,
				ConversationID: "conv-123",
				Tier:           TierRepo,
				DossierScope:   "internal/harness",
				DossierKind:    dossiers.DossierKindModule,
				Item: inbox.NoteItem{
					Type:     dossiers.EntryTypeGotcha,
					Title:    "Multi-turn stream-json usage counters are cumulative",
					Why:      "Summing raw usage across turns double-counts prompt tokens.",
					Do:       "Compute per-turn usage delta from prevUsage in RunMultiTurn.",
					Anchors:  []string{"internal/harness/antigravity/executor.go#RunMultiTurn"},
					Evidence: "step 4",
					Verified: "test-pass (2026-10-16)",
				},
			},
		},
		BrokenAnchors: []BrokenAnchorReport{
			{
				EntryID:        "got-01OLD",
				DossierScope:   "internal/harness",
				EntryType:      dossiers.EntryTypeGotcha,
				Title:          "Legacy runner path",
				MissingAnchors: []string{"internal/harness/legacy.go"},
				AllMissing:     true,
			},
		},
		AtlasSummary: "| internal/harness | module | 4 | 2026-10-16T00:00:00Z |",
	}

	prompt, err := BuildConsolidationPrompt(input, token)
	if err != nil {
		t.Fatalf("BuildConsolidationPrompt failed: %v", err)
	}

	openTag := "<consolidation-batch-" + token + ">"
	closeTag := "</consolidation-batch-" + token + ">"
	openIdx := strings.Index(prompt, openTag)
	closeIdx := strings.Index(prompt, closeTag)
	if openIdx < 0 || closeIdx < 0 || closeIdx <= openIdx {
		t.Fatalf("missing or misordered delimiter tags %s ... %s in prompt:\n%s", openTag, closeTag, prompt)
	}

	preFraming := prompt[:openIdx]
	postFraming := prompt[closeIdx+len(closeTag):]

	if !strings.Contains(strings.ToLower(preFraming), "historical repository memory data") ||
		!strings.Contains(strings.ToLower(preFraming), "not instructions") {
		t.Errorf("expected pre-batch §5.6 data-framing warning, got:\n%s", preFraming)
	}
	if !strings.Contains(strings.ToLower(postFraming), "not instructions") ||
		!strings.Contains(postFraming, token) {
		t.Errorf("expected post-batch §5.6 data-framing warning + echo_token instruction, got:\n%s", postFraming)
	}

	innerJSON := strings.TrimSpace(prompt[openIdx+len(openTag) : closeIdx])
	var parsed map[string]any
	if err := json.Unmarshal([]byte(innerJSON), &parsed); err != nil {
		t.Fatalf("inner batch payload is not valid JSON: %v\nPayload:\n%s", err, innerJSON)
	}

	if parsed["scope"] != "internal/harness" {
		t.Errorf("expected scope internal/harness, got %v", parsed["scope"])
	}
	if parsed["atlas_summary"] != input.AtlasSummary {
		t.Errorf("expected atlas_summary %q, got %v", input.AtlasSummary, parsed["atlas_summary"])
	}

	existingSlice, ok := parsed["existing_entries"].([]any)
	if !ok || len(existingSlice) != 1 {
		t.Fatalf("expected 1 existing entry in JSON, got %v", parsed["existing_entries"])
	}

	candSlice, ok := parsed["candidate_items"].([]any)
	if !ok || len(candSlice) != 1 {
		t.Fatalf("expected 1 candidate item in JSON, got %v", parsed["candidate_items"])
	}
	candMap := candSlice[0].(map[string]any)
	if candMap["note_id"] != "note-01NEW" || candMap["source"] != "turn-note" {
		t.Errorf("unexpected candidate metadata: %+v", candMap)
	}
	symAnchors, ok := candMap["symbol_anchors"].([]any)
	if !ok || len(symAnchors) != 1 || symAnchors[0] != "internal/harness/antigravity/executor.go#RunMultiTurn" {
		t.Errorf("expected symbol_anchors populated, got %v", candMap["symbol_anchors"])
	}

	brokenSlice, ok := parsed["broken_anchors"].([]any)
	if !ok || len(brokenSlice) != 1 {
		t.Fatalf("expected 1 broken anchor in JSON, got %v", parsed["broken_anchors"])
	}
}

func TestParseConsolidatorOutput_ValidAndFlatFields(t *testing.T) {
	now := time.Date(2026, 10, 16, 8, 30, 0, 0, time.UTC)
	token := "a1b2c3d4e5f60718"

	raw := "```json\n" + `{
  "ops": [
    {
      "op": "add",
      "type": "gotcha",
      "title": "Line one\nLine two title",
      "why": "Reason line 1\nReason line 2",
      "do": "Do line 1\nDo line 2",
      "anchors": ["internal/a.go"],
      "evidence": ["note-01A", "step 2"],
      "verified": "test-pass (2026-10-16)"
    },
    {
      "op": "amend",
      "entry_id": "inv-01EXIST",
      "dossier_scope": "internal/custom",
      "fields": {
        "text": "**Updated invariant**\nWhy: Better reason\nDo: Enforce strictly",
        "evidence": ["note-01OLD", "note-01NEW"]
      }
    },
    {
      "op": "supersede",
      "entry_id": "dec-01OLD",
      "new_entry_id": "dec-01NEW",
      "new_text": "**Use stream-json multi-turn**\nWhy: Saves 90% prefix cache\nDo: Call RunMultiTurn"
    },
    {
      "op": "retire",
      "entry_id": "got-01DEAD",
      "reason": "all anchored files no longer exist"
    },
    {
      "op": "move",
      "entry_id": "con-01MOVE",
      "target_scope": "internal/target"
    },
    {
      "op": "split",
      "dossier_scope": "internal/monolith",
      "new_scopes": ["internal/monolith/part1", "internal/monolith/part2"],
      "entry_moves": {
        "inv-1": "internal/monolith/part1",
        "inv-2": "internal/monolith/part2"
      }
    }
  ],
  "echo_token": "a1b2c3d4e5f60718"
}` + "\n```"

	parsed, err := ParseConsolidatorOutput(raw, token, "internal/default", now)
	if err != nil {
		t.Fatalf("ParseConsolidatorOutput failed: %v", err)
	}
	if len(parsed) != 6 {
		t.Fatalf("expected 6 ops, got %d", len(parsed))
	}

	// Verify every op passes ops.ValidateOp!
	for i, op := range parsed {
		if err := ops.ValidateOp(op); err != nil {
			t.Errorf("op[%d] (%s) failed ops.ValidateOp: %v (op=%+v)", i, op.Type, err, op)
		}
		if !op.Timestamp.Equal(now) {
			t.Errorf("op[%d] timestamp = %v, want %v", i, op.Timestamp, now)
		}
	}

	// 0: add (flat fields -> synthesized text)
	addOp := parsed[0]
	if addOp.Type != ops.OpAdd || addOp.DossierScope != "internal/default" {
		t.Errorf("unexpected addOp header: %+v", addOp)
	}
	if !strings.HasPrefix(addOp.EntryID, "got-") {
		t.Errorf("expected got- prefix on generated add EntryID, got %q", addOp.EntryID)
	}
	wantText := "**Line one Line two title**\nWhy: Reason line 1 Reason line 2\nDo: Do line 1 Do line 2"
	if addOp.Fields["text"] != wantText {
		t.Errorf("addOp text = %q, want %q", addOp.Fields["text"], wantText)
	}
	if lines := strings.Split(addOp.Fields["text"].(string), "\n"); len(lines) > 6 {
		t.Errorf("expected <= 6 lines, got %d", len(lines))
	}

	// 1: amend (canonical fields)
	amendOp := parsed[1]
	if amendOp.Type != ops.OpAmend || amendOp.EntryID != "inv-01EXIST" || amendOp.DossierScope != "internal/custom" {
		t.Errorf("unexpected amendOp: %+v", amendOp)
	}

	// 2: supersede (flat new_entry_id + new_text)
	supOp := parsed[2]
	if supOp.Type != ops.OpSupersede || supOp.EntryID != "dec-01OLD" || supOp.Fields["new_entry_id"] != "dec-01NEW" {
		t.Errorf("unexpected supOp: %+v", supOp)
	}
	if supOp.Fields["text"] == "" {
		t.Errorf("expected supOp.Fields[text] populated from new_text, got %+v", supOp.Fields)
	}

	// 3: retire
	retOp := parsed[3]
	if retOp.Type != ops.OpRetire || retOp.EntryID != "got-01DEAD" || retOp.Fields["reason"] != "all anchored files no longer exist" {
		t.Errorf("unexpected retOp: %+v", retOp)
	}

	// 4: move
	moveOp := parsed[4]
	if moveOp.Type != ops.OpMove || moveOp.EntryID != "con-01MOVE" || moveOp.Fields["to_dossier"] != "internal/target" || moveOp.Fields["target_scope"] != "internal/target" {
		t.Errorf("unexpected moveOp: %+v", moveOp)
	}

	// 5: split
	splitOp := parsed[5]
	if splitOp.Type != ops.OpSplit || splitOp.DossierScope != "internal/monolith" {
		t.Errorf("unexpected splitOp: %+v", splitOp)
	}
}

func TestParseConsolidatorOutput_RejectsEchoMismatchAndDerailments(t *testing.T) {
	now := time.Now().UTC()
	expectedToken := "0123456789abcdef"

	cases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{
			name:    "empty output",
			raw:     "   \n\t  ",
			wantErr: ErrEmptyConsolidatorOutput,
		},
		{
			name:    "missing echo_token",
			raw:     `{"ops": []}`,
			wantErr: ErrConsolidatorEchoMismatch,
		},
		{
			name:    "wrong echo_token",
			raw:     `{"ops": [], "echo_token": "deadbeefdeadbeef"}`,
			wantErr: ErrConsolidatorEchoMismatch,
		},
		{
			name:    "tool_call xml tag",
			raw:     `<tool_call>{"name": "run_command"}</tool_call>{"ops": [], "echo_token": "0123456789abcdef"}`,
			wantErr: ErrConsolidatorDerailment,
		},
		{
			name:    "tool_calls json key",
			raw:     `{"tool_calls": [{"name": "write_to_file"}], "ops": [], "echo_token": "0123456789abcdef"}`,
			wantErr: ErrConsolidatorDerailment,
		},
		{
			name:    "replace_file_content tool call",
			raw:     `I will call replace_file_content now. {"ops": [], "echo_token": "0123456789abcdef"}`,
			wantErr: ErrConsolidatorDerailment,
		},
		{
			name:    "generated markdown dossier comment",
			raw:     "<!-- generated from .memory/ops -->\n# internal/core\n",
			wantErr: ErrConsolidatorDerailment,
		},
		{
			name:    "full markdown dossier rewrite",
			raw:     "# Module: internal/core\n\n## Architecture & Invariants\n\n### [inv-1] Keep pure\n\n## Gotchas\n",
			wantErr: ErrConsolidatorDerailment,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseConsolidatorOutput(tc.raw, expectedToken, "internal/core", now)
			if err == nil {
				t.Fatalf("expected error %v, got nil", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error wrapping %v, got %v", tc.wantErr, err)
			}
		})
	}
}

type mockMultiTurnHK struct {
	multiTurnCalls int
	singleCalls    int
	promptsSeen    []string
}

var tokenExtractRe = regexp.MustCompile(`<consolidation-batch-([a-fA-F0-9]+)>`)

func (m *mockMultiTurnHK) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	m.singleCalls++
	return harness.Result{}, errors.New("unexpected single-turn Run call on MultiTurnHousekeeper")
}

func (m *mockMultiTurnHK) RunMultiTurn(ctx context.Context, stage harness.Stage, prompts []string) ([]harness.Result, error) {
	m.multiTurnCalls++
	m.promptsSeen = append(m.promptsSeen, prompts...)
	results := make([]harness.Result, len(prompts))
	for i, p := range prompts {
		match := tokenExtractRe.FindStringSubmatch(p)
		tok := ""
		if len(match) >= 2 {
			tok = match[1]
		}
		payload := fmt.Sprintf(`{"ops":[{"op":"add","type":"invariant","title":"Rule %d","why":"Why %d","do":"Do %d","anchors":["f%d.go"],"evidence":["note-%d"]}],"echo_token":%q}`, i+1, i+1, i+1, i+1, i+1, tok)
		results[i] = harness.Result{
			Payload: payload,
			Usage: harness.Usage{
				InputTokens:     500 * (i + 1),
				OutputTokens:    40,
				CacheReadTokens: 400 * i,
			},
		}
	}
	return results, nil
}

type mockSingleTurnHK struct {
	singleCalls int
}

func (m *mockSingleTurnHK) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	m.singleCalls++
	p, _ := input.(string)
	match := tokenExtractRe.FindStringSubmatch(p)
	tok := ""
	if len(match) >= 2 {
		tok = match[1]
	}
	payload := fmt.Sprintf(`{"ops":[{"op":"retire","entry_id":"got-01OLD","reason":"obsolete"}],"echo_token":%q}`, tok)
	return harness.Result{
		Payload: payload,
		Usage:   harness.Usage{InputTokens: 300, OutputTokens: 25},
	}, nil
}

func TestConsolidateDossiers_MultiTurnAndDeterministic(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

	inputs := []DossierConsolidationInput{
		{
			Scope: "internal/mod1",
			Kind:  dossiers.DossierKindModule,
			CandidateItems: []RoutedItem{
				{
					NoteID:         "note-01",
					NoteSource:     inbox.SourceTurnNote,
					ConversationID: "conv-1",
					DossierScope:   "internal/mod1",
					Item: inbox.NoteItem{
						Type:     dossiers.EntryTypeInvariant,
						Title:    "Mod1 invariant",
						Why:      "Because mod1 requires it",
						Do:       "Always validate mod1",
						Anchors:  []string{"internal/mod1/a.go"},
						Verified: "test-pass (2026-10-16)",
					},
				},
			},
			BrokenAnchors: []BrokenAnchorReport{
				{
					EntryID:        "got-01BROKEN",
					DossierScope:   "internal/mod1",
					MissingAnchors: []string{"internal/mod1/deleted.go"},
					AllMissing:     true,
				},
			},
		},
		{
			Scope: "internal/mod2",
			Kind:  dossiers.DossierKindModule,
			CandidateItems: []RoutedItem{
				{
					NoteID:         "note-02",
					NoteSource:     inbox.SourceSidecar,
					ConversationID: "conv-2",
					DossierScope:   "internal/mod2",
					Item: inbox.NoteItem{
						Type:       dossiers.EntryTypeOpenIssue,
						Title:      "Mod2 open issue",
						Why:        "Unverified hypothesis",
						Do:         "Check mod2",
						Anchors:    []string{"internal/mod2/b.go"},
						ClosesWhen: "path_exists:internal/mod2/fix.go",
					},
				},
			},
		},
	}

	// 1. MultiTurnHousekeeper path: 1 multi-turn call across 2 dossiers.
	mthk := &mockMultiTurnHK{}
	mtOps, mtUsage, err := ConsolidateDossiers(context.Background(), mthk, inputs, now)
	if err != nil {
		t.Fatalf("ConsolidateDossiers(MultiTurnHousekeeper) failed: %v", err)
	}
	if mthk.multiTurnCalls != 1 || mthk.singleCalls != 0 {
		t.Errorf("expected 1 multiTurnCall and 0 singleCalls, got multi=%d single=%d", mthk.multiTurnCalls, mthk.singleCalls)
	}
	if len(mthk.promptsSeen) != 2 {
		t.Errorf("expected 2 prompts sent in multi-turn batch, got %d", len(mthk.promptsSeen))
	}
	if len(mtOps) != 2 {
		t.Fatalf("expected 2 ops from multi-turn consolidator, got %d", len(mtOps))
	}
	if mtOps[0].DossierScope != "internal/mod1" || mtOps[1].DossierScope != "internal/mod2" {
		t.Errorf("unexpected dossier scopes on returned ops: %q, %q", mtOps[0].DossierScope, mtOps[1].DossierScope)
	}
	if mtUsage.InputTokens != 1500 || mtUsage.OutputTokens != 80 || mtUsage.CacheReadTokens != 400 {
		t.Errorf("unexpected accumulated usage: %+v", mtUsage)
	}

	// 2. Standard Housekeeper fallback path: 2 sequential Run calls.
	sthk := &mockSingleTurnHK{}
	stOps, stUsage, err := ConsolidateDossiers(context.Background(), sthk, inputs, now)
	if err != nil {
		t.Fatalf("ConsolidateDossiers(SingleTurnHousekeeper) failed: %v", err)
	}
	if sthk.singleCalls != 2 {
		t.Errorf("expected 2 sequential Run calls, got %d", sthk.singleCalls)
	}
	if len(stOps) != 2 {
		t.Fatalf("expected 2 ops from single-turn consolidator, got %d", len(stOps))
	}
	if stUsage.InputTokens != 600 || stUsage.OutputTokens != 50 {
		t.Errorf("unexpected single-turn usage: %+v", stUsage)
	}

	// 3. Deterministic baseline (hk == nil): 1 OpRetire + 2 OpAdd = 3 ops, zero Usage.
	detOps, detUsage, err := ConsolidateDossiers(context.Background(), nil, inputs, now)
	if err != nil {
		t.Fatalf("ConsolidateDossiers(nil) failed: %v", err)
	}
	if detUsage.InputTokens != 0 || detUsage.OutputTokens != 0 {
		t.Errorf("expected zero usage for deterministic path, got %+v", detUsage)
	}
	if len(detOps) != 3 {
		t.Fatalf("expected 3 deterministic ops (1 retire + 2 add), got %d", len(detOps))
	}
	if detOps[0].Type != ops.OpRetire || detOps[0].EntryID != "got-01BROKEN" || detOps[0].DossierScope != "internal/mod1" {
		t.Errorf("unexpected detOps[0]: %+v", detOps[0])
	}
	if detOps[1].Type != ops.OpAdd || detOps[1].DossierScope != "internal/mod1" || detOps[1].Fields["verified"] != "test-pass (2026-10-16)" {
		t.Errorf("unexpected detOps[1]: %+v", detOps[1])
	}
	if detOps[2].Type != ops.OpAdd || detOps[2].DossierScope != "internal/mod2" || detOps[2].Fields["closes_when"] != "path_exists:internal/mod2/fix.go" {
		t.Errorf("unexpected detOps[2]: %+v", detOps[2])
	}
	for i, op := range detOps {
		if err := ops.ValidateOp(op); err != nil {
			t.Errorf("detOps[%d] failed ops.ValidateOp: %v", i, err)
		}
	}
}
