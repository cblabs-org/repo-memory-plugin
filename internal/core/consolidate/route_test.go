/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package consolidate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func TestClassifyTier(t *testing.T) {
	repoRoot := t.TempDir()
	existingFile := filepath.Join(repoRoot, "internal", "core", "ops", "op.go")
	if err := os.MkdirAll(filepath.Dir(existingFile), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(existingFile, []byte("package ops\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	t.Run("explicit tier prefixes", func(t *testing.T) {
		cases := []struct {
			rawScope string
			wantTier ScopeTier
			wantRest string
		}{
			{"user:workflow", TierUser, "workflow"},
			{"USER: preferences/git ", TierUser, "preferences/git"},
			{"branch:core/ops", TierBranch, "core/ops"},
			{"repo:core/ops", TierRepo, "core/ops"},
			{"session:scratch", TierSession, "scratch"},
			{"temp:scratch", TierTemp, "scratch"},
		}
		for _, tc := range cases {
			gotTier, gotRest := StripTierPrefix(tc.rawScope)
			if gotTier != tc.wantTier || gotRest != tc.wantRest {
				t.Errorf("StripTierPrefix(%q) = (%q, %q), want (%q, %q)",
					tc.rawScope, gotTier, gotRest, tc.wantTier, tc.wantRest)
			}
			classified := ClassifyTier(&inbox.Note{Branch: "main"}, inbox.NoteItem{Scope: tc.rawScope}, repoRoot, nil)
			if classified != tc.wantTier {
				t.Errorf("ClassifyTier(%q) = %q, want %q", tc.rawScope, classified, tc.wantTier)
			}
		}
	})

	t.Run("personal workstation heuristic", func(t *testing.T) {
		for _, scope := range []string{"user", "user-dossiers/editor", "USER-DOSSIERS"} {
			got := ClassifyTier(&inbox.Note{Branch: "main"}, inbox.NoteItem{Scope: scope}, repoRoot, nil)
			if got != TierUser {
				t.Errorf("ClassifyTier(scope=%q) = %q, want %q", scope, got, TierUser)
			}
		}
	})

	t.Run("unmerged feature branch with missing anchor on main routes to TierBranch", func(t *testing.T) {
		note := &inbox.Note{
			ID:     "note-feat-1",
			Branch: "feat/new-pkg",
		}
		item := inbox.NoteItem{
			Type:    dossiers.EntryTypeInvariant,
			Title:   "NewPkg requires non-nil context",
			Anchors: []string{"internal/newpkg/foo.go#NewPkg"},
		}
		isMerged := func(branch string) bool { return false }

		got := ClassifyTier(note, item, repoRoot, isMerged)
		if got != TierBranch {
			t.Fatalf("ClassifyTier unmerged feature branch with branch-only anchor = %q, want %q", got, TierBranch)
		}
	})

	t.Run("merged feature branch or existing anchor on main routes to TierRepo", func(t *testing.T) {
		note := &inbox.Note{
			ID:     "note-feat-2",
			Branch: "feat/new-pkg",
		}
		missingAnchorItem := inbox.NoteItem{
			Type:    dossiers.EntryTypeInvariant,
			Title:   "NewPkg requires non-nil context",
			Anchors: []string{"internal/newpkg/foo.go"},
		}

		// 1. Branch is already merged -> TierRepo even if anchor is missing.
		if got := ClassifyTier(note, missingAnchorItem, repoRoot, func(string) bool { return true }); got != TierRepo {
			t.Errorf("ClassifyTier(merged branch) = %q, want %q", got, TierRepo)
		}

		// 2. Unmerged branch, but anchor exists in repoRoot (main) -> TierRepo.
		existingAnchorItem := inbox.NoteItem{
			Type:    dossiers.EntryTypeInvariant,
			Title:   "OpBatch version must be 1",
			Anchors: []string{"internal/core/ops/op.go"},
		}
		if got := ClassifyTier(note, existingAnchorItem, repoRoot, func(string) bool { return false }); got != TierRepo {
			t.Errorf("ClassifyTier(existing anchor on main) = %q, want %q", got, TierRepo)
		}

		// 3. Default branch ("main") with missing anchor -> TierRepo.
		mainNote := &inbox.Note{ID: "note-main", Branch: "main"}
		if got := ClassifyTier(mainNote, missingAnchorItem, repoRoot, nil); got != TierRepo {
			t.Errorf("ClassifyTier(main branch) = %q, want %q", got, TierRepo)
		}
	})
}

func TestRouteItem(t *testing.T) {
	scopeCfg := &config.ScopeConfig{
		Topics: []config.DeclaredScope{
			{
				Name:        "topics/persistence",
				Kind:        dossiers.DossierKindTopic,
				Keywords:    []string{"sqlite", "wal", "pragma"},
				Description: "SQLite storage and WAL concurrency",
			},
		},
		Externals: []config.DeclaredScope{
			{
				Name:        "external/antigravity",
				Kind:        dossiers.DossierKindExternal,
				Keywords:    []string{"antigravity", "hook"},
				ToolNames:   []string{"mcp_antigravity"},
				Description: "Antigravity external hook behaviour",
			},
		},
		Procedures: []config.DeclaredScope{
			{
				Name:        "procedures/release",
				Kind:        dossiers.DossierKindProcedure,
				Keywords:    []string{"goreleaser", "semver"},
				Commands:    []string{"git tag", "make release"},
				Description: "Release tagging workflow",
			},
		},
	}
	modules := []config.ModuleScope{
		{
			Name:      "core/ops",
			Kind:      dossiers.DossierKindModule,
			PathGlobs: []string{"internal/core/ops/op.go", "internal/core/ops/writer.go"},
			Symbols:   []string{"OpBatch"},
		},
	}
	aliases := config.NewAliasMap()
	aliases.SetAlias("old/scope", "new/scope")
	aliases.SetAlias("legacy/ops", "core/ops")

	resolver := config.NewScopeResolver(scopeCfg, modules, aliases)
	note := &inbox.Note{
		ID:             "note-01",
		Source:         inbox.SourceTurnNote,
		ConversationID: "conv-123",
		Branch:         "main",
		HeadCommit:     "abc1234",
		CreatedAt:      time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC),
	}

	t.Run("explicit scope with alias resolution", func(t *testing.T) {
		item := inbox.NoteItem{
			Type:  dossiers.EntryTypeInvariant,
			Title: "Preserve split history",
			Scope: "repo:old/scope",
		}
		routed := RouteItem(note, item, resolver, "", nil)
		if routed.Tier != TierRepo {
			t.Errorf("Tier = %q, want %q", routed.Tier, TierRepo)
		}
		if routed.DossierScope != "new/scope" {
			t.Errorf("DossierScope = %q, want %q", routed.DossierScope, "new/scope")
		}
		if routed.DossierKind != dossiers.DossierKindModule {
			t.Errorf("DossierKind = %q, want %q", routed.DossierKind, dossiers.DossierKindModule)
		}
		if routed.Item.Scope != "new/scope" {
			t.Errorf("Item.Scope = %q, want %q", routed.Item.Scope, "new/scope")
		}
	})

	t.Run("matched existing module scope in ScopeResolver", func(t *testing.T) {
		item := inbox.NoteItem{
			Type:    dossiers.EntryTypeContract,
			Title:   "Op writer writes atomic ULID batches",
			Anchors: []string{"internal/core/ops/writer.go#Write"},
		}
		routed := RouteItem(note, item, resolver, "", nil)
		if routed.DossierScope != "core/ops" {
			t.Errorf("DossierScope = %q, want %q", routed.DossierScope, "core/ops")
		}
		if routed.DossierKind != dossiers.DossierKindModule {
			t.Errorf("DossierKind = %q, want %q", routed.DossierKind, dossiers.DossierKindModule)
		}
		if routed.DerivedNew {
			t.Errorf("DerivedNew = true, want false for matched existing module scope")
		}
	})

	t.Run("unmatched file anchor rolled up via DeriveModuleScopes with DerivedNew true", func(t *testing.T) {
		item := inbox.NoteItem{
			Type:  dossiers.EntryTypeInvariant,
			Title: "Consolidator validates echo tokens",
			Anchors: []string{
				"internal/core/consolidate/filter.go",
				"internal/core/consolidate/route.go",
				"internal/core/consolidate/llm.go",
			},
		}
		routed := RouteItem(note, item, resolver, "", nil)
		if routed.DossierScope != "core/consolidate" {
			t.Errorf("DossierScope = %q, want %q", routed.DossierScope, "core/consolidate")
		}
		if routed.DossierKind != dossiers.DossierKindModule {
			t.Errorf("DossierKind = %q, want %q", routed.DossierKind, dossiers.DossierKindModule)
		}
		if !routed.DerivedNew {
			t.Errorf("DerivedNew = false, want true for newly derived module scope")
		}
	})

	t.Run("unanchored item routed by keywords to topic, external, and procedure scopes plus fallback", func(t *testing.T) {
		// 1. Topic keyword match
		topicItem := inbox.NoteItem{
			Type:  dossiers.EntryTypeInvariant,
			Title: "Enable SQLite WAL pragma on connection open",
			Do:    "Set busy_timeout and journal_mode=WAL",
		}
		rTopic := RouteItem(note, topicItem, resolver, "", nil)
		if rTopic.DossierScope != "topics/persistence" || rTopic.DossierKind != dossiers.DossierKindTopic {
			t.Errorf("topic route = (%q, %q), want (topics/persistence, topic)", rTopic.DossierScope, rTopic.DossierKind)
		}

		// 2. External keyword/tool match
		extItem := inbox.NoteItem{
			Type:  dossiers.EntryTypeGotcha,
			Title: "mcp_antigravity hook payload omits empty fields",
		}
		rExt := RouteItem(note, extItem, resolver, "", nil)
		if rExt.DossierScope != "external/antigravity" || rExt.DossierKind != dossiers.DossierKindExternal {
			t.Errorf("external route = (%q, %q), want (external/antigravity, external)", rExt.DossierScope, rExt.DossierKind)
		}

		// 3. Procedure command match
		procItem := inbox.NoteItem{
			Type:     dossiers.EntryTypeRecipe,
			Title:    "Cut a production release",
			Commands: []string{"git tag -a v1.0.0", "make release"},
		}
		rProc := RouteItem(note, procItem, resolver, "", nil)
		if rProc.DossierScope != "procedures/release" || rProc.DossierKind != dossiers.DossierKindProcedure {
			t.Errorf("procedure route = (%q, %q), want (procedures/release, procedure)", rProc.DossierScope, rProc.DossierKind)
		}

		// 4. Fallbacks when no keywords match
		fbTopic := RouteItem(note, inbox.NoteItem{
			Type:  dossiers.EntryTypeDecision,
			Title: "Unrelated architectural choice",
		}, resolver, "", nil)
		if fbTopic.DossierScope != "general" || fbTopic.DossierKind != dossiers.DossierKindTopic {
			t.Errorf("fallback topic = (%q, %q), want (general, topic)", fbTopic.DossierScope, fbTopic.DossierKind)
		}

		fbProc := RouteItem(note, inbox.NoteItem{
			Type:  dossiers.EntryTypeProcedure,
			Title: "Unrelated local verification steps",
		}, resolver, "", nil)
		if fbProc.DossierScope != "procedures/general" || fbProc.DossierKind != dossiers.DossierKindProcedure {
			t.Errorf("fallback procedure = (%q, %q), want (procedures/general, procedure)", fbProc.DossierScope, fbProc.DossierKind)
		}

		fbExt := RouteItem(note, inbox.NoteItem{
			Type:  dossiers.EntryTypeExternalBehaviour,
			Title: "Unrelated third-party service quirk",
		}, resolver, "", nil)
		if fbExt.DossierScope != "external/general" || fbExt.DossierKind != dossiers.DossierKindExternal {
			t.Errorf("fallback external = (%q, %q), want (external/general, external)", fbExt.DossierScope, fbExt.DossierKind)
		}
	})
}

func TestMergeExactDuplicates(t *testing.T) {
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

	dossiersMap := map[string]*dossiers.Dossier{
		"core/ops": {
			Scope: "core/ops",
			Kind:  dossiers.DossierKindModule,
			Entries: []dossiers.Entry{
				{
					ID:           "con-01EXISTING",
					Type:         dossiers.EntryTypeContract,
					DossierScope: "core/ops",
					Text:         "### [con-01EXISTING] Batch files use ULID names\n- Why: deterministic chronological ordering\n- Do: generate names with NewBatchIDWithTime",
					Anchors:      []string{"internal/core/ops/op.go"},
					Evidence:     []string{"note-001"},
				},
			},
		},
	}

	routed := []RoutedItem{
		// 1. Matches con-01EXISTING via alias-equivalent type (convention == contract)
		{
			NoteID:         "note-002",
			ConversationID: "conv-A",
			Tier:           TierRepo,
			DossierScope:   "core/ops",
			DossierKind:    dossiers.DossierKindModule,
			Item: inbox.NoteItem{
				Type:     dossiers.EntryTypeConvention,
				Title:    "Batch files use ULID names",
				Anchors:  []string{"internal/core/ops/op.go", "internal/core/ops/writer.go"},
				Evidence: "ep-002",
				Verified: "2026-10-16 (abc1234, test-pass)",
			},
		},
		// 2. Second candidate in the same batch matching the SAME existing entry con-01EXISTING -> must coalesce into one OpAmend
		{
			NoteID:         "note-003",
			ConversationID: "conv-B",
			Tier:           TierRepo,
			DossierScope:   "core/ops",
			DossierKind:    dossiers.DossierKindModule,
			Item: inbox.NoteItem{
				Type:    dossiers.EntryTypeContract,
				Title:   "### [con-01EXISTING] Batch files use ULID names",
				Anchors: []string{"internal/core/ops/op.go"},
			},
		},
		// 3. New candidate item (not in dossiersMap)
		{
			NoteID:         "note-004",
			ConversationID: "conv-C",
			Tier:           TierRepo,
			DossierScope:   "core/ops",
			DossierKind:    dossiers.DossierKindModule,
			Item: inbox.NoteItem{
				Type:    dossiers.EntryTypeRecipe,
				Title:   "Run ops unit tests with race detector",
				Anchors: []string{"internal/core/ops/ops_test.go"},
			},
		},
		// 4. Duplicate of candidate #3 in the same batch (procedure == recipe) -> must coalesce in Remaining
		{
			NoteID:         "note-005",
			ConversationID: "conv-D",
			Tier:           TierRepo,
			DossierScope:   "core/ops",
			DossierKind:    dossiers.DossierKindModule,
			Item: inbox.NoteItem{
				Type:     dossiers.EntryTypeProcedure,
				Title:    "run ops unit tests with race detector",
				Anchors:  []string{"internal/core/ops/ops_test.go", "internal/core/ops/validate.go"},
				Verified: "2026-10-16 (def5678, test-pass)",
			},
		},
	}

	res := MergeExactDuplicates(routed, dossiersMap, now)

	// Verify coalesced OpAmend for con-01EXISTING
	if len(res.AmendOps) != 1 {
		t.Fatalf("len(AmendOps) = %d, want 1", len(res.AmendOps))
	}
	amend := res.AmendOps[0]
	if amend.Type != ops.OpAmend || amend.EntryID != "con-01EXISTING" || amend.DossierScope != "core/ops" {
		t.Errorf("unexpected OpAmend header: %+v", amend)
	}
	wantEvidence := []string{"note-001", "note-002", "ep-002", "note-003"}
	gotEvidence, _ := amend.Fields["evidence"].([]string)
	if !reflect.DeepEqual(gotEvidence, wantEvidence) {
		t.Errorf("AmendOps[0].Fields[evidence] = %v, want %v", gotEvidence, wantEvidence)
	}
	wantAnchors := []string{"internal/core/ops/op.go", "internal/core/ops/writer.go"}
	gotAnchors, _ := amend.Fields["anchors"].([]string)
	if !reflect.DeepEqual(gotAnchors, wantAnchors) {
		t.Errorf("AmendOps[0].Fields[anchors] = %v, want %v", gotAnchors, wantAnchors)
	}
	if amend.Fields["verified"] != "2026-10-16 (abc1234, test-pass)" {
		t.Errorf("AmendOps[0].Fields[verified] = %v, want verified stamp", amend.Fields["verified"])
	}

	// Verify coalesced Remaining candidate
	if len(res.Remaining) != 1 {
		t.Fatalf("len(Remaining) = %d, want 1", len(res.Remaining))
	}
	rem := res.Remaining[0]
	if rem.NoteID != "note-004" {
		t.Errorf("Remaining[0].NoteID = %q, want note-004", rem.NoteID)
	}
	if rem.Item.Evidence != "note-004, note-005" {
		t.Errorf("Remaining[0].Item.Evidence = %q, want %q", rem.Item.Evidence, "note-004, note-005")
	}
	wantRemAnchors := []string{"internal/core/ops/ops_test.go", "internal/core/ops/validate.go"}
	if !reflect.DeepEqual(rem.Item.Anchors, wantRemAnchors) {
		t.Errorf("Remaining[0].Item.Anchors = %v, want %v", rem.Item.Anchors, wantRemAnchors)
	}
	if rem.Item.Verified != "2026-10-16 (def5678, test-pass)" {
		t.Errorf("Remaining[0].Item.Verified = %q, want verified stamp from duplicate", rem.Item.Verified)
	}
}

func TestCheckDossierAnchors(t *testing.T) {
	repoRoot := t.TempDir()
	validFile := filepath.Join(repoRoot, "internal", "core", "ops", "op.go")
	if err := os.MkdirAll(filepath.Dir(validFile), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(validFile, []byte("package ops\n\ntype OpBatch struct{}\n"), 0o644); err != nil {
		t.Fatalf("write validFile: %v", err)
	}

	d := &dossiers.Dossier{
		Scope: "core/ops",
		Kind:  dossiers.DossierKindModule,
		Entries: []dossiers.Entry{
			// 1. All anchors exist (including #OpBatch symbol) -> ignored
			{
				ID:      "inv-VALID",
				Type:    dossiers.EntryTypeInvariant,
				Text:    "### [inv-VALID] OpBatch is valid",
				Anchors: []string{"internal/core/ops/op.go#OpBatch"},
			},
			// 2. Partially missing anchors -> AllMissing == false
			{
				ID:      "con-PARTIAL",
				Type:    dossiers.EntryTypeContract,
				Text:    "### [con-PARTIAL] Partial anchor entry",
				Anchors: []string{"internal/core/ops/op.go", "internal/core/ops/deleted.go"},
			},
			// 3. 100% missing anchors (one missing file, one existing file with missing symbol) -> AllMissing == true
			{
				ID:      "got-ALLMISSING",
				Type:    dossiers.EntryTypeGotcha,
				Text:    "### [got-ALLMISSING] Completely stale entry",
				Anchors: []string{"internal/core/ops/gone.go", "internal/core/ops/op.go#RemovedSymbol"},
			},
			// 4. Retired entry with missing anchor -> ignored
			{
				ID:      "inv-RETIRED",
				Type:    dossiers.EntryTypeInvariant,
				Text:    "### [inv-RETIRED] Already retired",
				Anchors: []string{"internal/core/ops/nonexistent.go"},
				Retired: true,
			},
		},
	}

	reports := CheckDossierAnchors(repoRoot, d)
	if len(reports) != 2 {
		t.Fatalf("len(reports) = %d, want 2: %+v", len(reports), reports)
	}

	// Report 0: con-PARTIAL
	r0 := reports[0]
	if r0.EntryID != "con-PARTIAL" || r0.AllMissing {
		t.Errorf("reports[0] = %+v, want EntryID=con-PARTIAL, AllMissing=false", r0)
	}
	if !reflect.DeepEqual(r0.MissingAnchors, []string{"internal/core/ops/deleted.go"}) {
		t.Errorf("reports[0].MissingAnchors = %v", r0.MissingAnchors)
	}
	if !reflect.DeepEqual(r0.ValidAnchors, []string{"internal/core/ops/op.go"}) {
		t.Errorf("reports[0].ValidAnchors = %v", r0.ValidAnchors)
	}

	// Report 1: got-ALLMISSING
	r1 := reports[1]
	if r1.EntryID != "got-ALLMISSING" || !r1.AllMissing {
		t.Errorf("reports[1] = %+v, want EntryID=got-ALLMISSING, AllMissing=true", r1)
	}
	if len(r1.MissingAnchors) != 2 || len(r1.ValidAnchors) != 0 {
		t.Errorf("reports[1] anchors = missing:%v valid:%v", r1.MissingAnchors, r1.ValidAnchors)
	}
}
