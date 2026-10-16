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

	// At MomentFirstTouch, a graduated recipe renders as a 1-line pointer.
	firstTouchOut := FormatEntriesForMoment(ranked, MomentFirstTouch, workspace)
	wantPointer := "[repo-memory rec-build-01] skill " + slug + " covers this procedure: Build and flash ESP32 firmware"
	if !strings.Contains(firstTouchOut, wantPointer) {
		t.Errorf("expected 1-line pointer %q in MomentFirstTouch output, got:\n%s", wantPointer, firstTouchOut)
	}
	if strings.Contains(firstTouchOut, "pio run -t upload") {
		t.Errorf("expected MomentFirstTouch pointer to omit full multi-step body, got:\n%s", firstTouchOut)
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
