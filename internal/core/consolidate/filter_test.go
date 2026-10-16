package consolidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func intPtr(v int) *int {
	return &v
}

func TestHelperUtilities(t *testing.T) {
	t.Run("NormalizeTitle", func(t *testing.T) {
		got := NormalizeTitle("### [inv-01J9ABC] Always pass `--validate` on Chat_Mutations (v2.2)")
		want := "always pass validate on chat mutations v2 2"
		if got != want {
			t.Errorf("NormalizeTitle() = %q, want %q", got, want)
		}
	})

	t.Run("ExtractEntryTitleAndDo", func(t *testing.T) {
		entryText := "### [inv-01J9ABC] Use pointer-file swap on reindex\nWhy: Prevents SQLite lock contention.\nDo: Write to a new SQLite file and swap index-pointer."
		if title := ExtractEntryTitle(entryText); title != "Use pointer-file swap on reindex" {
			t.Errorf("ExtractEntryTitle() = %q, want %q", title, "Use pointer-file swap on reindex")
		}
		if doLine := ExtractEntryDo(entryText); doLine != "Write to a new SQLite file and swap index-pointer." {
			t.Errorf("ExtractEntryDo() = %q, want %q", doLine, "Write to a new SQLite file and swap index-pointer.")
		}
	})

	t.Run("StripAnchorSymbolAndOverlap", func(t *testing.T) {
		if got := StripAnchorSymbol("file:./internal/core/index/db.go#NewIndex"); got != "internal/core/index/db.go" {
			t.Errorf("StripAnchorSymbol() = %q, want %q", got, "internal/core/index/db.go")
		}
		if got := StripAnchorSymbol("/internal/core/index/db.go:42"); got != "internal/core/index/db.go" {
			t.Errorf("StripAnchorSymbol() = %q, want %q", got, "internal/core/index/db.go")
		}
		if got := StripAnchorSymbol("symbol:NewIndex"); got != "" {
			t.Errorf("StripAnchorSymbol(symbol:NewIndex) = %q, want empty", got)
		}

		if !AnchorsOverlap([]string{"internal/core/index/db.go#NewIndex"}, []string{"internal/core/index"}) {
			t.Error("expected directory prefix anchor overlap")
		}
		if !AnchorsOverlap(nil, nil) {
			t.Error("expected empty anchor lists to overlap")
		}
		if AnchorsOverlap([]string{"internal/core/index/db.go"}, []string{"internal/core/ops/writer.go"}) {
			t.Error("expected disjoint paths not to overlap")
		}
	})

	t.Run("FlattenActiveEntries", func(t *testing.T) {
		dm := map[string]*dossiers.Dossier{
			"core/ops": {
				Scope: "core/ops",
				Entries: []dossiers.Entry{
					{ID: "inv-2", Text: "Second"},
					{ID: "inv-1", Text: "First"},
					{ID: "inv-retired", Text: "Retired", Retired: true},
					{ID: "inv-superseded", Text: "Superseded", SupersededBy: "inv-2"},
				},
			},
			"core/index": {
				Scope: "core/index",
				Entries: []dossiers.Entry{
					{ID: "con-1", Text: "Index contract"},
				},
			},
		}
		active := FlattenActiveEntries(dm)
		if len(active) != 3 {
			t.Fatalf("len(active) = %d, want 3", len(active))
		}
		if active[0].ID != "con-1" || active[1].ID != "inv-1" || active[2].ID != "inv-2" {
			t.Errorf("unexpected order: %+v", active)
		}
	})
}

func TestFilterEchoes_DropsUneditedEcho(t *testing.T) {
	activeEntries := []dossiers.Entry{
		{
			ID:           "inv-7Q2M",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "harness/antigravity",
			Text:         "Never return empty JSON from PreToolUse\nWhy: Empty {} denies the tool call.\nDo: Return {\"decision\": \"allow\"} or empty stdout.",
			Anchors:      []string{"internal/harness/antigravity/hooks.go"},
		},
	}

	t.Run("ByExplicitEntryID", func(t *testing.T) {
		note := &inbox.Note{
			ID:     "note-01",
			Source: inbox.SourceTurnNote,
			ExposureList: []delivery.ExposureRecord{
				{EntryID: "inv-7Q2M"},
			},
			Items: []inbox.NoteItem{
				{
					Type:     dossiers.EntryTypeInvariant,
					Title:    "PreToolUse hook rule [inv-7Q2M]",
					Why:      "Confirmed inv-7Q2M during inspection.",
					Do:       "Always return allow decision.",
					Anchors:  []string{"internal/harness/antigravity/hooks.go"},
					Evidence: "step 3",
				},
			},
		}
		ep := &episode.Episode{
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindRead, File: "internal/harness/antigravity/hooks.go"},
				{Index: 2, Kind: harness.StepKindTest, ExitCode: intPtr(0), Passed: 5},
			},
		}

		res := FilterEchoes(note, activeEntries, ep)
		if res.EchoSuppressionCount != 1 {
			t.Fatalf("EchoSuppressionCount = %d, want 1", res.EchoSuppressionCount)
		}
		if len(res.Kept) != 0 {
			t.Fatalf("len(Kept) = %d, want 0", len(res.Kept))
		}
		if len(res.Suppressed) != 1 {
			t.Fatalf("len(Suppressed) = %d, want 1", len(res.Suppressed))
		}
	})

	t.Run("ByMatchingTitleAndOverlappingAnchor", func(t *testing.T) {
		note := &inbox.Note{
			ID:     "note-02",
			Source: inbox.SourceTurnNote,
			ExposureList: []delivery.ExposureRecord{
				{EntryID: "inv-7Q2M"},
			},
			Items: []inbox.NoteItem{
				{
					Type:    dossiers.EntryTypeInvariant,
					Title:   "Never return empty JSON from PreToolUse",
					Why:     "Empty JSON denies calls.",
					Do:      "Return allow or empty output.",
					Anchors: []string{"internal/harness/antigravity/hooks.go#HandlePreToolUse"},
				},
				{
					Type:    dossiers.EntryTypeGotcha,
					Title:   "Brand new unrelated discovery",
					Why:     "Discovered new edge case.",
					Do:      "Handle nil pointer.",
					Anchors: []string{"internal/core/ops/writer.go"},
				},
			},
		}
		ep := &episode.Episode{
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindEdit, File: "internal/core/ops/writer.go"},
			},
		}

		res := FilterEchoes(note, activeEntries, ep)
		if res.EchoSuppressionCount != 1 {
			t.Fatalf("EchoSuppressionCount = %d, want 1", res.EchoSuppressionCount)
		}
		if len(res.Kept) != 1 || res.Kept[0].Title != "Brand new unrelated discovery" {
			t.Fatalf("unexpected Kept items: %+v", res.Kept)
		}
	})
}

func TestFilterEchoes_DropsEditedEchoWithoutPassingTest(t *testing.T) {
	activeEntries := []dossiers.Entry{
		{
			ID:           "inv-7Q2M",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "harness/antigravity",
			Text:         "Never return empty JSON from PreToolUse\nWhy: Empty {} denies the tool call.\nDo: Return {\"decision\": \"allow\"} or empty stdout.",
			Anchors:      []string{"internal/harness/antigravity/hooks.go"},
		},
	}

	note := &inbox.Note{
		ID:     "note-03",
		Source: inbox.SourceTurnNote,
		ExposureList: []delivery.ExposureRecord{
			{EntryID: "inv-7Q2M"},
		},
		Items: []inbox.NoteItem{
			{
				Type:    dossiers.EntryTypeInvariant,
				Title:   "Never return empty JSON from PreToolUse",
				Anchors: []string{"internal/harness/antigravity/hooks.go"},
			},
		},
	}

	t.Run("FailingTestExitCode", func(t *testing.T) {
		ep := &episode.Episode{
			Steps: []harness.Step{
				{Index: 5, Kind: harness.StepKindEdit, File: "internal/harness/antigravity/hooks.go"},
				{Index: 8, Kind: harness.StepKindCommand, Command: "go test ./...", ExitCode: intPtr(1), Outcome: "failed"},
			},
		}
		res := FilterEchoes(note, activeEntries, ep)
		if res.EchoSuppressionCount != 1 || len(res.Kept) != 0 {
			t.Fatalf("expected failing test after edit to suppress echo, got %+v", res)
		}
	})

	t.Run("FailingTestCount", func(t *testing.T) {
		ep := &episode.Episode{
			Steps: []harness.Step{
				{Index: 5, Kind: harness.StepKindEdit, File: "internal/harness/antigravity/hooks.go"},
				{Index: 8, Kind: harness.StepKindTest, Passed: 3, Failed: 1},
			},
		}
		res := FilterEchoes(note, activeEntries, ep)
		if res.EchoSuppressionCount != 1 || len(res.Kept) != 0 {
			t.Fatalf("expected Failed > 0 test step to suppress echo, got %+v", res)
		}
	})

	t.Run("NoTestAfterEdit", func(t *testing.T) {
		ep := &episode.Episode{
			Steps: []harness.Step{
				{Index: 2, Kind: harness.StepKindTest, ExitCode: intPtr(0), Passed: 4},
				{Index: 5, Kind: harness.StepKindEdit, File: "internal/harness/antigravity/hooks.go"},
			},
		}
		res := FilterEchoes(note, activeEntries, ep)
		if res.EchoSuppressionCount != 1 || len(res.Kept) != 0 {
			t.Fatalf("expected test before edit (none after) to suppress echo, got %+v", res)
		}
	})
}

func TestFilterEchoes_RetainsReconfirmedEchoAfterEditAndPassingTest(t *testing.T) {
	activeEntries := []dossiers.Entry{
		{
			ID:           "inv-7Q2M",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "harness/antigravity",
			Text:         "Never return empty JSON from PreToolUse\nWhy: Empty {} denies the tool call.\nDo: Return {\"decision\": \"allow\"} or empty stdout.",
			Anchors:      []string{"internal/harness/antigravity/hooks.go"},
		},
	}

	note := &inbox.Note{
		ID:         "note-04",
		Source:     inbox.SourceTurnNote,
		HeadCommit: "abcdef1234567",
		CreatedAt:  time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC),
		ExposureList: []delivery.ExposureRecord{
			{EntryID: "inv-7Q2M"},
		},
		Items: []inbox.NoteItem{
			{
				Type:    dossiers.EntryTypeInvariant,
				Title:   "Never return empty JSON from PreToolUse",
				Anchors: []string{"internal/harness/antigravity/hooks.go"},
			},
		},
	}

	ep := &episode.Episode{
		Steps: []harness.Step{
			{Index: 5, Kind: harness.StepKindEdit, File: "internal/harness/antigravity/hooks.go"},
			{Index: 8, Kind: harness.StepKindCommand, Command: "go test ./...", ExitCode: intPtr(0)},
		},
	}

	res := FilterEchoes(note, activeEntries, ep)
	if res.EchoSuppressionCount != 0 {
		t.Fatalf("EchoSuppressionCount = %d, want 0", res.EchoSuppressionCount)
	}
	if len(res.Kept) != 1 {
		t.Fatalf("len(Kept) = %d, want 1", len(res.Kept))
	}
	if len(res.Reconfirmed) != 1 {
		t.Fatalf("len(Reconfirmed) = %d, want 1", len(res.Reconfirmed))
	}
	if !IsVerifiedStamp(res.Kept[0].Verified) || !strings.Contains(res.Kept[0].Verified, "reconfirmed") {
		t.Errorf("expected Kept[0].Verified to be populated with reconfirmed stamp, got %q", res.Kept[0].Verified)
	}
}

func TestFilterEchoes_UnknownExposureExternalCapture(t *testing.T) {
	activeEntries := []dossiers.Entry{
		{
			ID:           "inv-9X1P",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "core/scrub",
			Text:         "Preserve ULIDs and git commit SHAs in scrubber\nWhy: Avoid breaking op references.\nDo: Add known IDs to allow-list before entropy scanning.",
			Anchors:      []string{"internal/core/scrub/scrubber.go"},
		},
	}

	note := &inbox.Note{
		ID:           "note-ext-01",
		Source:       inbox.NoteSource("capture:external"),
		ExposureList: nil, // Empty exposure list on external capture -> unknown exposure
		Items: []inbox.NoteItem{
			{
				Type:    dossiers.EntryTypeInvariant,
				Title:   "Preserve ULIDs and git commit SHAs in scrubber",
				Do:      "Add known IDs to allow-list before entropy scanning.",
				Anchors: []string{"internal/core/scrub/scrubber.go"},
			},
		},
	}

	ep := &episode.Episode{
		Steps: []harness.Step{
			{Index: 1, Kind: harness.StepKindRead, File: "internal/core/scrub/scrubber.go"},
		},
	}

	if !IsUnknownExposureNote(note) {
		t.Fatal("expected IsUnknownExposureNote to return true for capture:external with empty ExposureList")
	}

	res := FilterEchoes(note, activeEntries, ep)
	if res.EchoSuppressionCount != 1 {
		t.Fatalf("EchoSuppressionCount = %d, want 1", res.EchoSuppressionCount)
	}
	if len(res.Kept) != 0 {
		t.Fatalf("len(Kept) = %d, want 0", len(res.Kept))
	}
}

func TestEnforceCorroborationGate_DowngradesUncorroboratedRules(t *testing.T) {
	note := &inbox.Note{
		ID:     "note-uncorroborated",
		Source: inbox.SourceTurnNote,
	}

	items := []inbox.NoteItem{
		{
			Type:     dossiers.EntryTypeInvariant,
			Title:    "Hypothetical invariant on missing file",
			Do:       "Never call foo directly.",
			Anchors:  []string{"internal/nonexistent/foo.go"},
			Verified: "null",
		},
		{
			Type:     dossiers.EntryTypeContract,
			Title:    "Hypothetical contract without anchors",
			Do:       "Always pass context.",
			Verified: "",
		},
		{
			Type:     dossiers.EntryTypeConvention,
			Title:    "Hypothetical convention",
			Do:       "Use snake_case keys.",
			Anchors:  []string{"internal/nonexistent/conv.go"},
			Verified: "unverified",
		},
		{
			Type:     dossiers.EntryTypeRecipe,
			Title:    "Uncorroborated build recipe",
			Do:       "Run custom_builder --fast",
			Commands: []string{"custom_builder --fast"},
			Verified: "",
		},
		{
			Type:     dossiers.EntryTypeGotcha,
			Title:    "Gotcha requesting gate: edit without corroboration",
			Do:       "gate: edit — require approval before touching bar.go",
			Anchors:  []string{"internal/nonexistent/bar.go"},
			Verified: "null",
		},
		// Non-rule types that should pass through unchanged:
		{
			Type:     dossiers.EntryTypeDecision,
			Title:    "Architectural decision",
			Why:      "Chose SQLite over BoltDB.",
			Verified: "",
		},
		{
			Type:     dossiers.EntryTypeRuledOut,
			Title:    "Ruled out cgo SQLite",
			Why:      "Breaks cross-compilation.",
			Verified: "null",
		},
		{
			Type:     dossiers.EntryTypeGotcha,
			Title:    "Regular gotcha without edit gate",
			Why:      "Watch out for nil maps.",
			Verified: "",
		},
		{
			Type:     dossiers.EntryTypeFailureSignature,
			Title:    "SQLITE_BUSY error signature",
			Verified: "",
		},
		{
			Type:       dossiers.EntryTypeOpenIssue,
			Title:      "Existing open issue",
			ClosesWhen: "path_exists:internal/future/file.go",
		},
	}

	ep := &episode.Episode{
		Steps: []harness.Step{
			{Index: 1, Kind: harness.StepKindReasoning, Text: "I think maybe we should never call foo directly."},
		},
	}

	res := EnforceCorroborationGate(note, items, ep, t.TempDir())
	if res.CorroborationDowngradeCount != 5 {
		t.Fatalf("CorroborationDowngradeCount = %d, want 5", res.CorroborationDowngradeCount)
	}
	if len(res.Downgraded) != 5 {
		t.Fatalf("len(Downgraded) = %d, want 5", len(res.Downgraded))
	}
	if len(res.Items) != len(items) {
		t.Fatalf("len(Items) = %d, want %d", len(res.Items), len(items))
	}

	// Verify downgraded items 0..4 are now open-issue with deterministic ClosesWhen.
	for i := 0; i < 5; i++ {
		if res.Items[i].Type != dossiers.EntryTypeOpenIssue {
			t.Errorf("Items[%d].Type = %q, want %q", i, res.Items[i].Type, dossiers.EntryTypeOpenIssue)
		}
		if res.Items[i].Verified != "unverified" {
			t.Errorf("Items[%d].Verified = %q, want \"unverified\"", i, res.Items[i].Verified)
		}
	}
	if res.Items[0].ClosesWhen != "path_exists:internal/nonexistent/foo.go" {
		t.Errorf("Items[0].ClosesWhen = %q, want path_exists:internal/nonexistent/foo.go", res.Items[0].ClosesWhen)
	}
	if res.Items[1].ClosesWhen != "outcome:build_or_test_pass" {
		t.Errorf("Items[1].ClosesWhen = %q, want outcome:build_or_test_pass", res.Items[1].ClosesWhen)
	}
	if res.Items[3].ClosesWhen != "command_passes:custom_builder --fast" {
		t.Errorf("Items[3].ClosesWhen = %q, want command_passes:custom_builder --fast", res.Items[3].ClosesWhen)
	}

	// Verify non-rule types 5..9 were not downgraded.
	wantTypes := []dossiers.EntryType{
		dossiers.EntryTypeDecision,
		dossiers.EntryTypeRuledOut,
		dossiers.EntryTypeGotcha,
		dossiers.EntryTypeFailureSignature,
		dossiers.EntryTypeOpenIssue,
	}
	for offset, wantType := range wantTypes {
		idx := 5 + offset
		if res.Items[idx].Type != wantType {
			t.Errorf("Items[%d].Type = %q, want %q", idx, res.Items[idx].Type, wantType)
		}
	}
}

func TestEnforceCorroborationGate_CorroboratedByTestAnchorOrUser(t *testing.T) {
	note := &inbox.Note{
		ID:         "note-corroborated",
		Source:     inbox.SourceTurnNote,
		HeadCommit: "1234567890abcdef",
		CreatedAt:  time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC),
	}

	t.Run("SubtestA_CorroboratedByEditAndPassingTest", func(t *testing.T) {
		items := []inbox.NoteItem{
			{
				Type:     dossiers.EntryTypeInvariant,
				Title:    "Lock mutex before writing note file",
				Do:       "Hold Store.mu across writeNoteLocked.",
				Anchors:  []string{"internal/core/inbox/note.go"},
				Verified: "null",
			},
		}
		ep := &episode.Episode{
			Steps: []harness.Step{
				{Index: 2, Kind: harness.StepKindEdit, File: "internal/core/inbox/note.go"},
				{Index: 4, Kind: harness.StepKindTest, ExitCode: intPtr(0), Passed: 8, Failed: 0},
			},
		}

		res := EnforceCorroborationGate(note, items, ep, "")
		if res.CorroborationDowngradeCount != 0 {
			t.Fatalf("CorroborationDowngradeCount = %d, want 0", res.CorroborationDowngradeCount)
		}
		if res.Items[0].Type != dossiers.EntryTypeInvariant {
			t.Fatalf("Items[0].Type = %q, want invariant", res.Items[0].Type)
		}
		if !IsVerifiedStamp(res.Items[0].Verified) || !strings.Contains(res.Items[0].Verified, "test-pass") {
			t.Errorf("Items[0].Verified = %q, want test-pass stamp", res.Items[0].Verified)
		}
	})

	t.Run("SubtestB_CorroboratedByVerifiedStaticAnchor", func(t *testing.T) {
		tmpRepo := t.TempDir()
		pkgDir := filepath.Join(tmpRepo, "internal", "core", "consolidate")
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		filePath := filepath.Join(pkgDir, "sample.go")
		if err := os.WriteFile(filePath, []byte("package consolidate\n\nfunc VerifyContractSymbol() {}\n"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		items := []inbox.NoteItem{
			{
				Type:     dossiers.EntryTypeContract,
				Title:    "VerifyContractSymbol must remain exported",
				Do:       "Keep VerifyContractSymbol exported for callers.",
				Anchors:  []string{"internal/core/consolidate/sample.go#VerifyContractSymbol"},
				Verified: "",
			},
		}

		res := EnforceCorroborationGate(note, items, nil, tmpRepo)
		if res.CorroborationDowngradeCount != 0 {
			t.Fatalf("CorroborationDowngradeCount = %d, want 0", res.CorroborationDowngradeCount)
		}
		if res.Items[0].Type != dossiers.EntryTypeContract {
			t.Fatalf("Items[0].Type = %q, want contract", res.Items[0].Type)
		}
		if !IsVerifiedStamp(res.Items[0].Verified) || !strings.Contains(res.Items[0].Verified, "anchor-verified") {
			t.Errorf("Items[0].Verified = %q, want anchor-verified stamp", res.Items[0].Verified)
		}

		// Also verify that if the symbol does NOT exist in the file, Check 2 fails and downgrades.
		badSymbolItems := []inbox.NoteItem{
			{
				Type:     dossiers.EntryTypeContract,
				Title:    "NonExistentSymbol contract",
				Do:       "Keep NonExistentSymbol exported.",
				Anchors:  []string{"internal/core/consolidate/sample.go#NonExistentSymbol"},
				Verified: "",
			},
		}
		badRes := EnforceCorroborationGate(note, badSymbolItems, nil, tmpRepo)
		if badRes.CorroborationDowngradeCount != 1 || badRes.Items[0].Type != dossiers.EntryTypeOpenIssue {
			t.Fatalf("expected missing symbol in existing file to fail static anchor check and downgrade, got %+v", badRes)
		}
	})

	t.Run("SubtestC_CorroboratedByExplicitUserConfirmationAndRejectsSystemSDK", func(t *testing.T) {
		item := inbox.NoteItem{
			Type:     dossiers.EntryTypeInvariant,
			Title:    "Never import antigravity adapter in core consolidate",
			Do:       "Keep internal/core/consolidate free of adapter imports.",
			Anchors:  []string{"internal/core/consolidate/filter.go"},
			Verified: "",
		}

		// 1. Hook-injected SYSTEM_SDK step ([repo-memory ...]) must NOT count as human confirmation.
		epSystemSDK := &episode.Episode{
			Steps: []harness.Step{
				{
					Index:  1,
					Kind:   harness.StepKindUser,
					Author: "SYSTEM_SDK",
					Text:   "[repo-memory] Yes, confirmed invariant: Never import antigravity adapter in core consolidate (internal/core/consolidate/filter.go).",
				},
			},
		}
		resSDK := EnforceCorroborationGate(note, []inbox.NoteItem{item}, epSystemSDK, "")
		if resSDK.CorroborationDowngradeCount != 1 || resSDK.Items[0].Type != dossiers.EntryTypeOpenIssue {
			t.Fatalf("expected SYSTEM_SDK [repo-memory] step NOT to corroborate invariant, got %+v", resSDK)
		}

		// 2. Genuine USER_EXPLICIT step confirming the rule must corroborate it.
		epHuman := &episode.Episode{
			Steps: []harness.Step{
				{
					Index:  3,
					Kind:   harness.StepKindUser,
					Author: "USER_EXPLICIT",
					Text:   "Yes, confirmed: we must never import the antigravity adapter in core consolidate.",
				},
			},
		}
		resHuman := EnforceCorroborationGate(note, []inbox.NoteItem{item}, epHuman, "")
		if resHuman.CorroborationDowngradeCount != 0 {
			t.Fatalf("CorroborationDowngradeCount = %d, want 0", resHuman.CorroborationDowngradeCount)
		}
		if resHuman.Items[0].Type != dossiers.EntryTypeInvariant {
			t.Fatalf("Items[0].Type = %q, want invariant", resHuman.Items[0].Type)
		}
		if !IsVerifiedStamp(resHuman.Items[0].Verified) || !strings.Contains(resHuman.Items[0].Verified, "user-confirmed") {
			t.Errorf("Items[0].Verified = %q, want user-confirmed stamp", resHuman.Items[0].Verified)
		}
	})
}
