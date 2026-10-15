package dossiers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func TestDossiersRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	opsWriter := ops.NewWriter(tempDir)
	opsReader := ops.NewReader(tempDir)

	now := time.Now().UTC()
	op1 := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      ops.NewEntryID("inv-"),
		DossierScope: "core/test-module",
		Timestamp:    now,
		Source:       ops.SourceSeed,
		Fields: map[string]any{
			"kind":       "module",
			"title":      "Test Module",
			"purpose":    "Testing dossiers",
			"entry_type": "invariant",
			"text":       "First invariant text\nSecond line",
			"anchors":    []string{"file.go"},
			"verified":   "2026-10-06 (PR-01)",
		},
	}
	op2 := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      ops.NewEntryID("rul-"),
		DossierScope: "core/test-module",
		Timestamp:    now.Add(time.Minute),
		Source:       ops.SourceSeed,
		Fields: map[string]any{
			"entry_type": "ruled-out",
			"text":       "Ruled out text",
		},
	}

	batch := ops.NewBatch([]ops.Op{op1, op2})
	if err := opsWriter.Write(batch); err != nil {
		t.Fatalf("failed to write batch: %v", err)
	}

	projector := NewProjector(opsReader)
	ctx := context.Background()
	dossiersMap, err := projector.Project(ctx)
	if err != nil {
		t.Fatalf("failed to project: %v", err)
	}

	d, ok := dossiersMap["core/test-module"]
	if !ok {
		t.Fatalf("expected dossier core/test-module")
	}

	rendered1 := RenderDossier(d)
	if rendered1 == "" {
		t.Fatalf("rendered output is empty")
	}

	writer := NewWriter(tempDir)
	if err := writer.WriteAll(dossiersMap); err != nil {
		t.Fatalf("failed to write dossiers: %v", err)
	}

	onDisk, err := os.ReadFile(filepath.Join(tempDir, "dossiers", "core/test-module.md"))
	if err != nil {
		t.Fatalf("failed to read written dossier: %v", err)
	}

	if string(onDisk) != rendered1 {
		t.Fatalf("written file does not match rendered output")
	}
	
	atlasDisk, err := os.ReadFile(filepath.Join(tempDir, "ATLAS.md"))
	if err != nil {
		t.Fatalf("failed to read written ATLAS: %v", err)
	}
	if len(atlasDisk) == 0 {
		t.Fatalf("ATLAS is empty")
	}

	// hand edits detection
	newOps, err := DetectHandEdits(rendered1, string(onDisk), "core")
	if err != nil {
		t.Fatalf("DetectHandEdits error: %v", err)
	}
	if len(newOps) > 0 {
		t.Fatalf("expected 0 ops from hand edits for identical files, got %d", len(newOps))
	}
	
	// linting
	warnings := LintDossier(d)
	if len(warnings) > 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestProjectorOperations(t *testing.T) {
	tempDir := t.TempDir()
	opsWriter := ops.NewWriter(tempDir)
	opsReader := ops.NewReader(tempDir)

	now := time.Now().UTC()
	id1 := ops.NewEntryID("inv-")
	id2 := ops.NewEntryID("rul-")
	id3 := ops.NewEntryID("dec-")

	opAdd := ops.Op{Type: ops.OpAdd, EntryID: id1, DossierScope: "core/mod1", Timestamp: now, Source: ops.SourceSeed, Fields: map[string]any{"entry_type": "invariant", "text": "v1"}}
	opAmend := ops.Op{Type: ops.OpAmend, EntryID: id1, DossierScope: "core/mod1", Timestamp: now.Add(time.Second), Source: ops.SourceSeed, Fields: map[string]any{"text": "v2"}}
	opSupersede := ops.Op{Type: ops.OpSupersede, EntryID: id1, DossierScope: "core/mod1", Timestamp: now.Add(2*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"new_entry_id": id2, "text": "v3"}}
	opRetire := ops.Op{Type: ops.OpRetire, EntryID: id2, DossierScope: "core/mod1", Timestamp: now.Add(3*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"reason": "obsolete"}}
	
	opAdd2 := ops.Op{Type: ops.OpAdd, EntryID: id3, DossierScope: "core/mod1", Timestamp: now.Add(4*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"entry_type": "decision", "text": "dec1"}}
	opMove := ops.Op{Type: ops.OpMove, EntryID: id3, DossierScope: "core/mod1", Timestamp: now.Add(5*time.Second), Source: ops.SourceSeed, Fields: map[string]any{"to_dossier": "core/mod2"}}

	batch := ops.NewBatch([]ops.Op{opAdd, opAmend, opSupersede, opRetire, opAdd2, opMove})
	opsWriter.Write(batch)

	dossiersMap, _ := NewProjector(opsReader).Project(context.Background())

	d1 := dossiersMap["core/mod1"]
	d2 := dossiersMap["core/mod2"]

	if len(d1.Entries) != 0 {
		t.Errorf("mod1 should have 0 active entries, got %d", len(d1.Entries))
	}
	if len(d1.History) != 2 {
		t.Errorf("mod1 should have 2 history entries, got %d", len(d1.History))
	}
	if len(d2.Entries) != 1 || d2.Entries[0].ID != id3 {
		t.Errorf("mod2 should have id3 entry")
	}

	// Verify Writer.WriteAll with retired-only scope (mod1) and orphaned file cleanup
	orphanPath := filepath.Join(tempDir, "dossiers", "core", "orphan.md")
	if err := os.MkdirAll(filepath.Dir(orphanPath), 0755); err != nil {
		t.Fatalf("mkdir orphan dir: %v", err)
	}
	if err := os.WriteFile(orphanPath, []byte("# core/orphan\n"), 0644); err != nil {
		t.Fatalf("write orphan file: %v", err)
	}
	staleMod1Path := filepath.Join(tempDir, "dossiers", "core", "mod1.md")
	if err := os.WriteFile(staleMod1Path, []byte("# core/mod1\n"), 0644); err != nil {
		t.Fatalf("write stale mod1 file: %v", err)
	}

	writer := NewWriter(tempDir)
	if err := writer.WriteAll(dossiersMap); err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}

	if _, err := os.Stat(staleMod1Path); !os.IsNotExist(err) {
		t.Errorf("expected retired-only core/mod1.md to be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "dossiers", "core", "mod1.history.md")); err != nil {
		t.Errorf("expected core/mod1.history.md to exist on disk: %v", err)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Errorf("expected orphaned core/orphan.md to be removed, stat err = %v", err)
	}
	atlasBytes, err := os.ReadFile(filepath.Join(tempDir, "ATLAS.md"))
	if err != nil {
		t.Fatalf("reading ATLAS.md: %v", err)
	}
	atlasStr := string(atlasBytes)
	if strings.Contains(atlasStr, "core/mod1") || strings.Contains(atlasStr, "0001-01-01") || strings.Contains(atlasStr, "| 0 |") {
		t.Errorf("expected ATLAS.md not to contain retired-only core/mod1 or 0-entry row, got:\n%s", atlasStr)
	}
}

func TestDossiersEmptySections(t *testing.T) {
	d := &Dossier{
		Scope: "core/empty",
		Entries: []Entry{
			{ID: "inv-1", Type: EntryTypeInvariant, Text: "Just invariant"},
		},
	}
	rendered := RenderDossier(d)
	if strings.Contains(rendered, "## Decisions") {
		t.Errorf("empty section 'Decisions' was rendered")
	}
	if !strings.Contains(rendered, "## Invariants") {
		t.Errorf("non-empty section 'Invariants' was not rendered")
	}
}

func TestProjector_ReAddUnderDifferentScopeMovesEntry(t *testing.T) {
	tempDir := t.TempDir()
	opsWriter := ops.NewWriter(tempDir)
	opsReader := ops.NewReader(tempDir)

	now := time.Now().UTC()
	entryID := "inv-cross-scope-001"

	op1 := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      entryID,
		DossierScope: "core/mod-a",
		Timestamp:    now,
		Source:       ops.SourceSeed,
		Fields: map[string]any{
			"entry_type": "invariant",
			"text":       "Original text in mod-a",
		},
	}
	op2 := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      entryID,
		DossierScope: "core/mod-b",
		Timestamp:    now.Add(time.Minute),
		Source:       ops.SourceSeed,
		Fields: map[string]any{
			"entry_type": "invariant",
			"text":       "Updated text moved to mod-b",
		},
	}

	batch := ops.NewBatch([]ops.Op{op1, op2})
	if err := opsWriter.Write(batch); err != nil {
		t.Fatalf("failed to write batch: %v", err)
	}

	dossiersMap, err := NewProjector(opsReader).Project(context.Background())
	if err != nil {
		t.Fatalf("failed to project: %v", err)
	}

	if modA, ok := dossiersMap["core/mod-a"]; ok && len(modA.Entries) > 0 {
		t.Errorf("expected 0 entries in core/mod-a after cross-scope re-add, got %d: %v", len(modA.Entries), modA.Entries)
	}
	if _, ok := dossiersMap["core/mod-a"]; ok {
		t.Errorf("expected empty shell core/mod-a dossier to be removed from projected map")
	}

	modB, ok := dossiersMap["core/mod-b"]
	if !ok {
		t.Fatalf("expected core/mod-b dossier to exist")
	}
	if len(modB.Entries) != 1 {
		t.Fatalf("expected 1 entry in core/mod-b, got %d: %v", len(modB.Entries), modB.Entries)
	}
	if modB.Entries[0].ID != entryID {
		t.Errorf("expected ID %s in core/mod-b, got %s", entryID, modB.Entries[0].ID)
	}
	if modB.Entries[0].DossierScope != "core/mod-b" {
		t.Errorf("expected DossierScope core/mod-b, got %s", modB.Entries[0].DossierScope)
	}
	if modB.Entries[0].Text != "Updated text moved to mod-b" {
		t.Errorf("expected updated text in core/mod-b, got %q", modB.Entries[0].Text)
	}

	totalOccurrences := 0
	for _, d := range dossiersMap {
		for _, e := range d.Entries {
			if e.ID == entryID {
				totalOccurrences++
			}
		}
	}
	if totalOccurrences != 1 {
		t.Errorf("expected exactly 1 occurrence of %s across all scopes, got %d", entryID, totalOccurrences)
	}

	writer := NewWriter(tempDir)
	if err := writer.WriteAll(dossiersMap); err != nil {
		t.Fatalf("WriteAll failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "dossiers", "core", "mod-a.md")); !os.IsNotExist(err) {
		t.Errorf("expected no empty core/mod-a.md on disk, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "dossiers", "core", "mod-b.md")); err != nil {
		t.Errorf("expected core/mod-b.md to exist on disk: %v", err)
	}
	atlasBytes, err := os.ReadFile(filepath.Join(tempDir, "ATLAS.md"))
	if err != nil {
		t.Fatalf("failed to read ATLAS.md: %v", err)
	}
	if strings.Contains(string(atlasBytes), "core/mod-a") {
		t.Errorf("expected ATLAS.md not to contain 0-entry core/mod-a row, got:\n%s", string(atlasBytes))
	}
}

func TestCognitiveKindAndEntryFields(t *testing.T) {
	// 1. DeriveCognitiveKind & EffectiveKind
	cases := []struct {
		entryType EntryType
		expected  CognitiveKind
	}{
		{EntryTypeInvariant, CognitiveKindSemantic},
		{EntryTypeContract, CognitiveKindSemantic},
		{EntryTypeConvention, CognitiveKindSemantic},
		{EntryTypeInterface, CognitiveKindSemantic},
		{EntryTypeGotcha, CognitiveKindSemantic},
		{EntryTypeExternalBehaviour, CognitiveKindSemantic},
		{EntryTypeOpenIssue, CognitiveKindSemantic},
		{EntryType(""), CognitiveKindSemantic},
		{EntryType("unknown"), CognitiveKindSemantic},
		{EntryTypeRecipe, CognitiveKindProcedural},
		{EntryTypeProcedure, CognitiveKindProcedural},
		{EntryTypeFailureSignature, CognitiveKindProcedural},
		{EntryTypeDecision, CognitiveKindEpisodic},
		{EntryTypeRuledOut, CognitiveKindEpisodic},
	}

	for _, tc := range cases {
		got := DeriveCognitiveKind(tc.entryType)
		if got != tc.expected {
			t.Errorf("DeriveCognitiveKind(%q) = %q, want %q", tc.entryType, got, tc.expected)
		}
		e := Entry{Type: tc.entryType}
		if e.EffectiveKind() != tc.expected {
			t.Errorf("Entry{Type: %q}.EffectiveKind() = %q, want %q", tc.entryType, e.EffectiveKind(), tc.expected)
		}
	}

	explicitEntry := Entry{Type: EntryTypeInvariant, Kind: CognitiveKindProcedural}
	if explicitEntry.EffectiveKind() != CognitiveKindProcedural {
		t.Errorf("expected explicit valid Kind to take precedence, got %q", explicitEntry.EffectiveKind())
	}

	if CognitiveKind("invalid").Valid() {
		t.Errorf("expected invalid CognitiveKind to return Valid() == false")
	}

	// 2. Projector add / amend / supersede with v2.2 fields and kind disambiguation
	tempDir := t.TempDir()
	opsWriter := ops.NewWriter(tempDir)
	opsReader := ops.NewReader(tempDir)
	now := time.Now().UTC()

	conID := ops.NewEntryID(EntryTypePrefixes[EntryTypeContract])
	recID := ops.NewEntryID(EntryTypePrefixes[EntryTypeRecipe])
	recID2 := ops.NewEntryID(EntryTypePrefixes[EntryTypeRecipe])

	batch := ops.NewBatch([]ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      conID,
			DossierScope: "core/contracts",
			Timestamp:    now,
			Source:       ops.SourceDocSeed,
			Fields: map[string]any{
				"kind":                "module",
				"entry_type":          string(EntryTypeContract),
				"text":                "Contract invariant from docs",
				"source_doc_path":     "docs/architecture.md",
				"source_doc_blob_sha": "abc123def456",
			},
		},
		{
			Type:         ops.OpAdd,
			EntryID:      recID,
			DossierScope: "core/contracts",
			Timestamp:    now.Add(time.Second),
			Source:       ops.SourceDocSeed,
			Fields: map[string]any{
				"entry_type":          string(EntryTypeRecipe),
				"text":                "Step 1: run go test",
				"source_doc_path":     ".agents/skills/build/SKILL.md",
				"source_doc_blob_sha": "111222333",
				"success_count":       float64(3),
				"failure_count":       1,
				"last_succeeded_at":   "2026-10-15T12:00:00Z",
				"stale_suspect":       true,
				"render_as_skill":     "rm-build-recipe",
			},
		},
		{
			Type:         ops.OpAmend,
			EntryID:      recID,
			DossierScope: "core/contracts",
			Timestamp:    now.Add(2 * time.Second),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"success_count":     int64(5),
				"failure_count":     0,
				"stale_suspect":     false,
				"last_succeeded_at": "2026-10-15T13:00:00Z",
			},
		},
		{
			Type:         ops.OpSupersede,
			EntryID:      recID,
			DossierScope: "core/contracts",
			Timestamp:    now.Add(3 * time.Second),
			Source:       ops.SourceDocSeed,
			Fields: map[string]any{
				"new_entry_id":        recID2,
				"text":                "Step 1: run go test -race",
				"source_doc_blob_sha": "444555666",
			},
		},
	})

	if err := opsWriter.Write(batch); err != nil {
		t.Fatalf("Write batch: %v", err)
	}

	dossiersMap, err := NewProjector(opsReader).Project(context.Background())
	if err != nil {
		t.Fatalf("Project: %v", err)
	}

	d := dossiersMap["core/contracts"]
	if d == nil {
		t.Fatalf("expected core/contracts dossier")
	}
	if d.Kind != DossierKindModule {
		t.Errorf("expected dossier Kind %q, got %q", DossierKindModule, d.Kind)
	}
	if len(d.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(d.Entries))
	}

	// Entries are sorted by typeOrder: contract (1) before recipe (6)
	gotCon := d.Entries[0]
	if gotCon.ID != conID || gotCon.Type != EntryTypeContract || gotCon.Kind != CognitiveKindSemantic {
		t.Errorf("unexpected contract entry: %+v", gotCon)
	}
	if gotCon.SourceDocPath != "docs/architecture.md" || gotCon.SourceDocBlobSHA != "abc123def456" {
		t.Errorf("unexpected contract doc-seed provenance: path=%q sha=%q", gotCon.SourceDocPath, gotCon.SourceDocBlobSHA)
	}

	gotRec := d.Entries[1]
	if gotRec.ID != recID2 || gotRec.Type != EntryTypeRecipe || gotRec.Kind != CognitiveKindProcedural {
		t.Errorf("unexpected superseded recipe entry: %+v", gotRec)
	}
	// Inherited SourceDocPath and RenderAsSkill, overridden SourceDocBlobSHA, reset execution counters
	if gotRec.SourceDocPath != ".agents/skills/build/SKILL.md" {
		t.Errorf("expected inherited SourceDocPath, got %q", gotRec.SourceDocPath)
	}
	if gotRec.SourceDocBlobSHA != "444555666" {
		t.Errorf("expected updated SourceDocBlobSHA, got %q", gotRec.SourceDocBlobSHA)
	}
	if gotRec.RenderAsSkill != "rm-build-recipe" {
		t.Errorf("expected inherited RenderAsSkill, got %q", gotRec.RenderAsSkill)
	}
	if gotRec.SuccessCount != 0 || gotRec.FailureCount != 0 || gotRec.StaleSuspect || gotRec.LastSucceededAt != "" {
		t.Errorf("expected execution counters reset on supersede when not specified, got %+v", gotRec)
	}
}

func TestRenderDossier_OptionalSectionsAndMetadata(t *testing.T) {
	d := &Dossier{
		Scope:   "core/v2-rendering",
		Purpose: "Verify v2.2 optional section resolution and metadata",
		Entries: []Entry{
			{
				ID:            "inv-001",
				Type:          EntryTypeInvariant,
				Text:          "Invariant title\nInvariant body",
				Anchors:       []string{"internal/core/dossiers/renderer.go"},
				SourceDocPath: "docs/guide.md",
				Verified:      "2026-10-15",
				Evidence:      []string{"commit-123"},
			},
			{
				ID:   "con-001",
				Type: EntryTypeContract,
				Text: "Contract title",
			},
			{
				ID:   "con-002",
				Type: EntryTypeConvention,
				Text: "Convention title",
			},
			{
				ID:   "dec-001",
				Type: EntryTypeDecision,
				Text: "Decision title",
			},
			{
				ID:            "proc-001",
				Type:          EntryTypeProcedure,
				Text:          "Procedure title\nStep 1: test",
				SourceDocPath: "docs/guide.md",
			},
			{
				ID:   "rec-001",
				Type: EntryTypeRecipe,
				Text: "Recipe title\nStep 1: build",
			},
		},
	}

	rendered := RenderDossier(d)

	// 1. Non-empty sections are rendered with canonical v2.2 titles
	for _, wantSection := range []string{
		"## Invariants & contracts",
		"## Decisions",
		"## Procedures / Recipes",
	} {
		if !strings.Contains(rendered, wantSection) {
			t.Errorf("expected rendered dossier to contain section %q, got:\n%s", wantSection, rendered)
		}
	}

	// 2. Empty optional {section?} headings are completely omitted
	for _, omittedSection := range []string{
		"## Ruled out, failure signatures and gotchas",
		"## Interfaces and dependencies",
		"## Open issues",
	} {
		if strings.Contains(rendered, omittedSection) {
			t.Errorf("expected empty optional section %q to be omitted, got:\n%s", omittedSection, rendered)
		}
	}

	// 3. Backtick metadata line includes kind (semantic/procedural/episodic) and source_doc when set
	wantMetaLines := []string{
		"`invariant · kind: semantic · anchors: internal/core/dossiers/renderer.go · source_doc: docs/guide.md · verified 2026-10-15 · evidence: commit-123`",
		"`contract · kind: semantic`",
		"`convention · kind: semantic`",
		"`decision · kind: episodic`",
		"`procedure · kind: procedural · source_doc: docs/guide.md`",
		"`recipe · kind: procedural`",
	}
	for _, wantMeta := range wantMetaLines {
		if !strings.Contains(rendered, wantMeta) {
			t.Errorf("expected rendered dossier to contain metadata line %s, got:\n%s", wantMeta, rendered)
		}
	}
}

func TestDetectHandEdits_RoundTripAndDeterministicPrefixes(t *testing.T) {
	d := &Dossier{
		Scope: "core/handedit-test",
		Entries: []Entry{
			{
				ID:            "inv-existing",
				Type:          EntryTypeInvariant,
				Text:          "Existing title\nExisting body line",
				Anchors:       []string{"file.go"},
				SourceDocPath: "docs/guide.md",
			},
		},
	}

	rendered := RenderDossier(d)

	// 1. Unmodified round-trip produces 0 ops
	opsUnmodified, err := DetectHandEdits(rendered, rendered, d.Scope)
	if err != nil {
		t.Fatalf("DetectHandEdits unmodified failed: %v", err)
	}
	if len(opsUnmodified) != 0 {
		t.Fatalf("expected 0 ops when unmodified, got %d: %+v", len(opsUnmodified), opsUnmodified)
	}

	// 2. Amending body text and adding new entries with v2.2 prefixes
	edited := strings.Replace(rendered, "Existing body line", "Amended body line", 1) + `
### [con-new01] Contract added by hand
` + "`contract · kind: semantic`" + `
Contract body

### [rec-new02] Recipe added by hand
Recipe body

### [err-new03] Error signature added by hand

### [iss-new04] Open issue added by hand

### [got-new05] Gotcha added by hand
`

	// Run multiple iterations to prove deterministic prefix resolution (no map iteration randomness)
	for iter := 0; iter < 20; iter++ {
		detected, err := DetectHandEdits(rendered, edited, d.Scope)
		if err != nil {
			t.Fatalf("DetectHandEdits failed on iter %d: %v", iter, err)
		}
		if len(detected) != 6 {
			t.Fatalf("expected 6 ops (1 amend + 5 add), got %d: %+v", len(detected), detected)
		}

		byID := make(map[string]ops.Op, len(detected))
		for _, op := range detected {
			byID[op.EntryID] = op
		}

		amendOp, ok := byID["inv-existing"]
		if !ok || amendOp.Type != ops.OpAmend {
			t.Fatalf("expected amend op for inv-existing, got %+v", amendOp)
		}
		if amendOp.Fields["text"] != "Existing title\nAmended body line" {
			t.Errorf("unexpected amended text: %q", amendOp.Fields["text"])
		}

		expectedAdds := map[string]EntryType{
			"con-new01": EntryTypeContract,
			"rec-new02": EntryTypeRecipe,
			"err-new03": EntryTypeFailureSignature,
			"iss-new04": EntryTypeOpenIssue,
			"got-new05": EntryTypeGotcha,
		}
		for id, wantType := range expectedAdds {
			addOp, ok := byID[id]
			if !ok || addOp.Type != ops.OpAdd {
				t.Fatalf("iter %d: expected add op for %s, got %+v", iter, id, addOp)
			}
			if gotType := addOp.Fields["entry_type"]; gotType != string(wantType) {
				t.Errorf("iter %d: %s entry_type = %q, want %q", iter, id, gotType, wantType)
			}
		}
	}

	// 3. Direct unit check on InferEntryTypeFromID across all supported prefixes
	prefixCases := []struct {
		id   string
		want EntryType
	}{
		{"inv-1", EntryTypeInvariant},
		{"con-1", EntryTypeContract},
		{"rul-1", EntryTypeRuledOut},
		{"fail-1", EntryTypeFailureSignature},
		{"err-1", EntryTypeFailureSignature},
		{"got-1", EntryTypeGotcha},
		{"dec-1", EntryTypeDecision},
		{"rec-1", EntryTypeRecipe},
		{"proc-1", EntryTypeProcedure},
		{"iface-1", EntryTypeInterface},
		{"issue-1", EntryTypeOpenIssue},
		{"iss-1", EntryTypeOpenIssue},
		{"docseed-1", EntryTypeInvariant},
		{"unknown-1", EntryTypeInvariant},
	}
	for _, tc := range prefixCases {
		if got := InferEntryTypeFromID(tc.id); got != tc.want {
			t.Errorf("InferEntryTypeFromID(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

