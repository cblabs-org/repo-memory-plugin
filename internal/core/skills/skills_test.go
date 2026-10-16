package skills

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "user.email", "test@example.com")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestExtractAndMatchRecipeCommands(t *testing.T) {
	text := "Build and test firmware:\n1. Run `pio run -e esp32dev`\n2. Run `pio test -e native`\n```bash\n# comment\n$ make lint\n```\n"
	cmds := ExtractRecipeCommands(text)
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d: %v", len(cmds), cmds)
	}
	if cmds[0] != "pio run -e esp32dev" || cmds[1] != "pio test -e native" || cmds[2] != "make lint" {
		t.Errorf("unexpected extracted commands: %v", cmds)
	}

	if !CommandMatchesRecipe("$ pio   run -e esp32dev", cmds) {
		t.Errorf("expected exact normalized command to match")
	}
	if !CommandMatchesRecipe("pio run -e esp32dev --verbose", cmds) {
		t.Errorf("expected prefix command with extra flags to match")
	}
	if !CommandMatchesRecipe("cd firmware && pio test -e native", cmds) {
		t.Errorf("expected compound command segment to match")
	}
	if CommandMatchesRecipe("cargo build", cmds) {
		t.Errorf("did not expect unrelated command to match")
	}
}

func TestQualifiesAndRenderSkillMarkdown(t *testing.T) {
	entry := dossiers.Entry{
		ID:           "rec-build-01",
		Type:         dossiers.EntryTypeRecipe,
		DossierScope: "firmware",
		Text:         "Build and verify ESP32 firmware\n1. Run `pio run -e esp32dev`\n2. Run `pio test -e native`",
		Anchors:      []string{"platformio.ini", "src/main.cpp"},
		Evidence:     []string{"conv:c1", "conv:c2"},
		Source:       "consolidation",
		Verified:     "abc1234",
		SuccessCount: 2,
		FailureCount: 0,
	}

	if steps := CountRecipeSteps(entry.Text); steps < 2 {
		t.Fatalf("expected >= 2 steps, got %d", steps)
	}

	// Single conversation in outcomes -> should NOT qualify despite SuccessCount == 2.
	singleConvOutcomes := &OutcomeState{
		Entries: map[string]*EntryOutcome{
			"rec-build-01": {
				SucceededConversations: []string{"c1"},
			},
		},
	}
	entryNoConvEvidence := entry
	entryNoConvEvidence.Evidence = nil
	if ok, _ := QualifiesForSkill(entryNoConvEvidence, singleConvOutcomes); ok {
		t.Errorf("expected false when only 1 conversation succeeded")
	}
	if ok, _ := QualifiesForSkill(entryNoConvEvidence, nil); ok {
		t.Errorf("expected false when outcomes == nil and Evidence has no conv: tags")
	}
	entryOneConvEvidence := entry
	entryOneConvEvidence.Evidence = []string{"conv:c1"}
	if ok, _ := QualifiesForSkill(entryOneConvEvidence, nil); ok {
		t.Errorf("expected false when outcomes == nil and Evidence has only 1 conv: tag")
	}

	// Two distinct conversations in outcomes -> qualifies.
	twoConvOutcomes := &OutcomeState{
		Entries: map[string]*EntryOutcome{
			"rec-build-01": {
				SucceededConversations: []string{"c1", "c2"},
			},
		},
	}
	ok, slug := QualifiesForSkill(entry, twoConvOutcomes)
	if !ok {
		t.Fatalf("expected entry to qualify for skill graduation")
	}
	if !strings.HasPrefix(slug, "rm-") {
		t.Errorf("expected rm- prefix on slug, got %q", slug)
	}

	gotSlug, md := RenderSkillMarkdown(entry)
	if gotSlug != slug {
		t.Errorf("RenderSkillMarkdown slug %q != QualifiesForSkill slug %q", gotSlug, slug)
	}
	if !IsGeneratedByRepoMemory(md) {
		t.Errorf("expected IsGeneratedByRepoMemory to be true for rendered skill:\n%s", md)
	}
	for _, want := range []string{
		"name: " + slug,
		"generated_by: repo-memory",
		"source_entry_id: rec-build-01",
		"## Procedure",
		"## Anchors",
		"## Provenance",
		"Generated from [rec-build-01] by repo-memory. Edit the dossier entry or op log, not this file.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("expected rendered SKILL.md to contain %q, got:\n%s", want, md)
		}
	}

	// Deterministic idempotency check
	_, md2 := RenderSkillMarkdown(entry)
	if md != md2 {
		t.Errorf("expected RenderSkillMarkdown to be strictly deterministic")
	}

	// Explicit RenderAsSkill overrides step/count requirements
	explicitEntry := dossiers.Entry{
		ID:            "rec-explicit-01",
		Type:          dossiers.EntryTypeRecipe,
		DossierScope:  "ops",
		Text:          "Single step deploy: `make deploy`",
		RenderAsSkill: "deploy-prod",
	}
	okExplicit, explicitSlug := QualifiesForSkill(explicitEntry, nil)
	if !okExplicit || explicitSlug != "rm-deploy-prod" {
		t.Errorf("expected explicit RenderAsSkill to qualify as rm-deploy-prod, got ok=%v slug=%q", okExplicit, explicitSlug)
	}
}

func TestRecordCommandOutcomeAndSkillGraduationLifecycle(t *testing.T) {
	ctx := context.Background()
	repoRoot := initGitRepo(t)
	dataDir := t.TempDir()
	repoID := "test-repo-id"

	if err := os.WriteFile(filepath.Join(repoRoot, "Makefile"), []byte("all:\n\t@echo ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoRoot, "add", "Makefile")
	runGit(t, repoRoot, "commit", "-m", "initial Makefile")

	// Seed a 2-step recipe in .memory/ops/ and index.db
	opsDir := filepath.Join(repoRoot, ".memory", "ops")
	if err := os.MkdirAll(opsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-10 * time.Minute)
	addOp := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      "rec-make-check",
		DossierScope: "build",
		Timestamp:    now,
		Source:       ops.SourceConsolidation,
		Fields: map[string]any{
			"entry_type": "recipe",
			"kind":       "procedural",
			"text":       "Build and verify project\n1. Run `make build`\n2. Run `make test`",
			"anchors":    []string{"Makefile"},
		},
	}
	if err := ops.NewWriter(opsDir).Write(ops.NewBatchWithTime([]ops.Op{addOp}, now)); err != nil {
		t.Fatal(err)
	}

	idxPath := index.IndexPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.RebuildFromOpLog(ctx, ops.NewReader(opsDir)); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Create a hand-authored skill to verify it is never removed.
	handSkillDir := filepath.Join(repoRoot, ".agents", "skills", "rm-hand-authored")
	if err := os.MkdirAll(handSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	handSkillFile := filepath.Join(handSkillDir, "SKILL.md")
	if err := os.WriteFile(handSkillFile, []byte("---\nname: rm-hand-authored\ndescription: Hand written\n---\n# Hand Written\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Success 1 in conv-1 -> SuccessCount = 1 (not yet graduated)
	matched, err := RecordCommandOutcome(ctx, dataDir, repoID, repoRoot, "conv-1", "make build", true)
	if err != nil || len(matched) != 1 || matched[0] != "rec-make-check" {
		t.Fatalf("RecordCommandOutcome conv-1: matched=%v err=%v", matched, err)
	}

	// Success 2 in same conv-1 -> SuccessCount = 2, but only 1 distinct conversation -> not graduated
	_, err = RecordCommandOutcome(ctx, dataDir, repoID, repoRoot, "conv-1", "make test", true)
	if err != nil {
		t.Fatal(err)
	}
	gradRes1, err := SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(gradRes1.Graduated) != 0 {
		t.Fatalf("expected 0 graduated skills after only 1 distinct conversation, got %v", gradRes1.Graduated)
	}

	// Success 3 in conv-2 -> 2 distinct conversations -> graduates to .agents/skills/rm-<slug>/SKILL.md!
	_, err = RecordCommandOutcome(ctx, dataDir, repoID, repoRoot, "conv-2", "make build", true)
	if err != nil {
		t.Fatal(err)
	}

	slug := SkillSlug(dossiers.Entry{
		ID:   "rec-make-check",
		Type: dossiers.EntryTypeRecipe,
		Text: "Build and verify project\n1. Run `make build`\n2. Run `make test`",
	})
	skillFile := filepath.Join(repoRoot, ".agents", "skills", slug, "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Fatalf("expected graduated skill file %s to exist: %v", skillFile, err)
	}

	// Two consecutive failures across two sessions (conv-3, conv-4) -> flags stale_suspect and removes graduated skill.
	_, err = RecordCommandOutcome(ctx, dataDir, repoID, repoRoot, "conv-3", "make build", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(skillFile); err != nil {
		// Wait: after 1 failure, FailureCount == 1 != 0, so QualifiesForSkill is already false unless RenderAsSkill is set!
	}
	_, err = RecordCommandOutcome(ctx, dataDir, repoID, repoRoot, "conv-4", "make build", false)
	if err != nil {
		t.Fatal(err)
	}

	// Verify entry is flagged stale_suspect in index.db and skill file is removed
	idx, err = index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	updatedEntry, err := idx.GetEntry(ctx, "rec-make-check")
	idx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !updatedEntry.StaleSuspect {
		t.Errorf("expected StaleSuspect=true after 2 consecutive failed conversations")
	}
	if updatedEntry.FailureCount != 2 {
		t.Errorf("expected FailureCount=2, got %d", updatedEntry.FailureCount)
	}
	if _, err := os.Stat(skillFile); !os.IsNotExist(err) {
		t.Errorf("expected stale_suspect skill file %s to be removed", skillFile)
	}
	// Hand-authored skill must remain intact.
	if _, err := os.Stat(handSkillFile); err != nil {
		t.Errorf("expected hand-authored skill %s to be preserved: %v", handSkillFile, err)
	}
}

func TestCheckBuildManifestFreshness(t *testing.T) {
	ctx := context.Background()
	repoRoot := initGitRepo(t)
	dataDir := t.TempDir()
	repoID := "manifest-repo"

	for _, p := range []string{
		"Makefile",
		"CMakeLists.txt",
		"platformio.ini",
		"sdkconfig.defaults",
		"go.mod",
		"package.json",
		"Taskfile.yml",
		".github/workflows/ci.yml",
	} {
		if !IsBuildManifestPath(p) {
			t.Errorf("expected IsBuildManifestPath(%q) == true", p)
		}
	}
	if IsBuildManifestPath("internal/core/skills/tracker.go") {
		t.Errorf("expected IsBuildManifestPath on regular Go file to be false")
	}

	manifestPath := filepath.Join(repoRoot, "platformio.ini")
	if err := os.WriteFile(manifestPath, []byte("[env:esp32]\nplatform = espressif32\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoRoot, "add", "platformio.ini")
	runGit(t, repoRoot, "commit", "-m", "initial platformio.ini")

	opsDir := filepath.Join(repoRoot, ".memory", "ops")
	if err := os.MkdirAll(opsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	addOp := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      "rec-pio-01",
		DossierScope: "firmware",
		Timestamp:    now,
		Source:       ops.SourceConsolidation,
		Fields: map[string]any{
			"entry_type":      "recipe",
			"kind":            "procedural",
			"text":            "Flash ESP32 image\n1. Run `pio run -e esp32`\n2. Run `pio run -t upload`",
			"anchors":         []string{"platformio.ini"},
			"render_as_skill": "rm-pio-flash",
		},
	}
	if err := ops.NewWriter(opsDir).Write(ops.NewBatchWithTime([]ops.Op{addOp}, now)); err != nil {
		t.Fatal(err)
	}

	// Record a successful command execution to snapshot platformio.ini's blob SHA
	if _, err := RecordCommandOutcome(ctx, dataDir, repoID, repoRoot, "conv-a", "pio run -e esp32", true); err != nil {
		t.Fatal(err)
	}

	skillPath := filepath.Join(repoRoot, ".agents", "skills", "rm-pio-flash", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected graduated skill at %s: %v", skillPath, err)
	}

	// Unmodified manifest -> CheckBuildManifestFreshness should flag 0 entries
	flagged, err := CheckBuildManifestFreshness(ctx, repoRoot, dataDir, repoID, nil)
	if err != nil || len(flagged) != 0 {
		t.Fatalf("expected 0 flagged on unmodified manifest, got %v (err=%v)", flagged, err)
	}

	// Modify platformio.ini in working tree -> CheckBuildManifestFreshness should flag rec-pio-01 stale_suspect and remove skill
	if err := os.WriteFile(manifestPath, []byte("[env:esp32]\nplatform = espressif32@6.5.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flagged, err = CheckBuildManifestFreshness(ctx, repoRoot, dataDir, repoID, nil)
	if err != nil || len(flagged) != 1 || flagged[0] != "rec-pio-01" {
		t.Fatalf("expected [rec-pio-01] flagged after manifest edit, got %v (err=%v)", flagged, err)
	}
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Errorf("expected graduated skill %s to be removed when flagged stale_suspect", skillPath)
	}
}
