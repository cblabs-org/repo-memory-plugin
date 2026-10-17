package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
)

func TestFormatEntriesForMoment_GraduatedSkillPointer(t *testing.T) {
	workspace := t.TempDir()

	recipe := dossiers.Entry{
		ID:           "rec-build-01",
		Type:         dossiers.EntryTypeRecipe,
		DossierScope: "firmware",
		Text:         "Build and flash ESP32 firmware\n1. Run `pio run -e esp32`\n2. Run `pio run -t upload`",
		Anchors:      []string{"platformio.ini"},
		Evidence:     []string{"conv:c1", "conv:c2"},
		SuccessCount: 2,
		FailureCount: 0,
	}

	slug, md := skills.RenderSkillMarkdown(recipe)
	skillDir := filepath.Join(workspace, ".agents", "skills", slug)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	ranked := []RankedEntry{{Entry: recipe}}

	// At MomentFirstTouch and MomentFirstEntry, a graduated recipe with SKILL.md on disk renders as a 1-line pointer.
	firstTouchOut := FormatEntriesForMoment(ranked, MomentFirstTouch, workspace)
	wantPointer := "[repo-memory rec-build-01] skill " + slug + " covers this procedure: Build and flash ESP32 firmware"
	if !strings.Contains(firstTouchOut, wantPointer) {
		t.Errorf("expected 1-line pointer %q in MomentFirstTouch output, got:\n%s", wantPointer, firstTouchOut)
	}
	if strings.Contains(firstTouchOut, "pio run -t upload") {
		t.Errorf("expected MomentFirstTouch pointer to omit full multi-step body, got:\n%s", firstTouchOut)
	}

	firstEntryOut := FormatEntriesForMoment(ranked, MomentFirstEntry, workspace)
	if !strings.Contains(firstEntryOut, wantPointer) {
		t.Errorf("expected 1-line pointer %q in MomentFirstEntry output, got:\n%s", wantPointer, firstEntryOut)
	}
	if strings.Contains(firstEntryOut, "pio run -t upload") {
		t.Errorf("expected MomentFirstEntry pointer to omit full multi-step body, got:\n%s", firstEntryOut)
	}

	// When workspaceRoot has no .agents/skills/rm-<slug>/SKILL.md on disk, FormatEntryForMoment
	// must deliver the full recipe body rather than a phantom 1-line skill pointer.
	emptyWorkspace := t.TempDir()
	noSkillTouchOut := FormatEntryForMoment(recipe, MomentFirstTouch, emptyWorkspace)
	if strings.Contains(noSkillTouchOut, "covers this procedure:") || !strings.Contains(noSkillTouchOut, "pio run -t upload") {
		t.Errorf("expected full recipe text when SKILL.md is absent under workspaceRoot, got:\n%s", noSkillTouchOut)
	}
	noSkillEntryOut := FormatEntryForMoment(recipe, MomentFirstEntry, emptyWorkspace)
	if strings.Contains(noSkillEntryOut, "covers this procedure:") || !strings.Contains(noSkillEntryOut, "pio run -t upload") {
		t.Errorf("expected full recipe text at MomentFirstEntry when SKILL.md is absent under workspaceRoot, got:\n%s", noSkillEntryOut)
	}

	// At MomentTaskStart, full entry formatting is preserved.
	taskStartOut := FormatEntriesForMoment(ranked, MomentTaskStart, workspace)
	if !strings.Contains(taskStartOut, "pio run -t upload") {
		t.Errorf("expected MomentTaskStart to include full recipe text, got:\n%s", taskStartOut)
	}

	// When flagged StaleSuspect, MomentFirstTouch falls back to full entry text.
	staleRecipe := recipe
	staleRecipe.StaleSuspect = true
	staleOut := FormatEntryForMoment(staleRecipe, MomentFirstTouch, workspace)
	if strings.Contains(staleOut, "covers this procedure:") {
		t.Errorf("expected stale_suspect recipe not to use skill pointer, got: %s", staleOut)
	}
}

func TestFormatEntries_CompactBulletListWithoutSetextHeading(t *testing.T) {
	ranked := []RankedEntry{
		{
			Entry: dossiers.Entry{
				ID:   "inv-01",
				Type: dossiers.EntryTypeInvariant,
				Text: "First invariant rule.",
			},
		},
		{
			Entry: dossiers.Entry{
				ID:   "got-02",
				Type: dossiers.EntryTypeGotcha,
				Text: "Second gotcha rule.\nWith continuation line.",
			},
		},
	}

	out := FormatEntries(ranked)
	if !strings.HasPrefix(out, "\n--- repo-memory context ---\n\n- [repo-memory inv-01]") {
		t.Errorf("expected leading blank line, context header, blank line, and bullet item, got:\n%q", out)
	}
	if strings.HasSuffix(strings.TrimSpace(out), "---") {
		t.Errorf("expected no trailing Setext '---' underline, got:\n%q", out)
	}
	if !strings.Contains(out, "- [repo-memory got-02] gotcha: Second gotcha rule.\n  With continuation line.") {
		t.Errorf("expected multi-line entry continuation to be indented under bullet, got:\n%q", out)
	}

	coalesced := CoalesceContextBlocks([]string{out, FormatEntries(ranked[:1])})
	if strings.Count(coalesced, "[repo-memory inv-01]") != 1 {
		t.Errorf("expected deduplicated inv-01 in CoalesceContextBlocks, got:\n%q", coalesced)
	}
	if strings.HasSuffix(strings.TrimSpace(coalesced), "---") {
		t.Errorf("expected no trailing Setext '---' underline in CoalesceContextBlocks, got:\n%q", coalesced)
	}
}
