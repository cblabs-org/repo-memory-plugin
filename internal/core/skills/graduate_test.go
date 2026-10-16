package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func TestCheckGraduatedSkillsLint(t *testing.T) {
	ctx := context.Background()
	repoRoot := initGitRepo(t)
	dataDir := t.TempDir()
	repoID := "lint-repo-id"

	// 1. Clean repository without .agents/skills/ -> zero warnings
	warnings, err := CheckGraduatedSkillsLint(ctx, repoRoot, dataDir, repoID)
	if err != nil {
		t.Fatalf("CheckGraduatedSkillsLint on empty repo failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings on empty repo, got %v", warnings)
	}

	// Seed a qualifying procedural recipe in .memory/ops/
	opsDir := filepath.Join(repoRoot, ".memory", "ops")
	if err := os.MkdirAll(opsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	addOp := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      "rec-lint-01",
		DossierScope: "build",
		Timestamp:    now,
		Source:       ops.SourceConsolidation,
		Fields: map[string]any{
			"entry_type":      "recipe",
			"kind":            "procedural",
			"text":            "Run verification suite\n1. Run `go vet ./...`\n2. Run `go test ./...`",
			"anchors":         []string{"go.mod"},
			"render_as_skill": "rm-verify-suite",
		},
	}
	if err := ops.NewWriter(opsDir).Write(ops.NewBatchWithTime([]ops.Op{addOp}, now)); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, false); err != nil {
		t.Fatalf("SyncGraduatedSkills failed: %v", err)
	}

	skillPath := filepath.Join(repoRoot, ".agents", "skills", "rm-verify-suite", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected graduated skill at %s: %v", skillPath, err)
	}

	// 2. Verify clean return when .agents/skills/rm-verify-suite/SKILL.md matches SyncGraduatedSkills
	warnings, err = CheckGraduatedSkillsLint(ctx, repoRoot, dataDir, repoID)
	if err != nil {
		t.Fatalf("CheckGraduatedSkillsLint failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings when skill matches SyncGraduatedSkills, got %v", warnings)
	}

	// 3. Verify hand-authored .agents/skills/rm-custom/SKILL.md without generated_by: repo-memory produces zero warnings
	customSkillDir := filepath.Join(repoRoot, ".agents", "skills", "rm-custom")
	if err := os.MkdirAll(customSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customSkillDir, "SKILL.md"), []byte("---\nname: rm-custom\ndescription: Custom skill\n---\n# Custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err = CheckGraduatedSkillsLint(ctx, repoRoot, dataDir, repoID)
	if err != nil {
		t.Fatalf("CheckGraduatedSkillsLint failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings with hand-authored rm-custom skill, got %v", warnings)
	}

	// 4. Verify out-of-sync warning when .agents/skills/rm-verify-suite/SKILL.md body is manually modified while keeping generated_by: repo-memory
	origBytes, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	modifiedContent := string(origBytes) + "\nManual uncoordinated edit.\n"
	if err := os.WriteFile(skillPath, []byte(modifiedContent), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err = CheckGraduatedSkillsLint(ctx, repoRoot, dataDir, repoID)
	if err != nil {
		t.Fatalf("CheckGraduatedSkillsLint failed: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Graduated skill .agents/skills/rm-verify-suite/SKILL.md is out of sync with .memory/ops/") {
		t.Fatalf("expected out-of-sync warning for rm-verify-suite, got %v", warnings)
	}

	// Restore clean skill via SyncGraduatedSkills
	if _, err := SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, false); err != nil {
		t.Fatal(err)
	}

	// 5. Verify stale warning when .agents/skills/rm-stale/SKILL.md has generated_by: repo-memory for a retired/non-existent entry
	staleSkillDir := filepath.Join(repoRoot, ".agents", "skills", "rm-stale")
	if err := os.MkdirAll(staleSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	staleContent := "---\nname: rm-stale\ndescription: Stale generated skill\ngenerated_by: repo-memory\nsource_entry_id: rec-deleted\n---\n# Stale\n"
	if err := os.WriteFile(filepath.Join(staleSkillDir, "SKILL.md"), []byte(staleContent), 0o644); err != nil {
		t.Fatal(err)
	}
	warnings, err = CheckGraduatedSkillsLint(ctx, repoRoot, dataDir, repoID)
	if err != nil {
		t.Fatalf("CheckGraduatedSkillsLint failed: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Graduated skill .agents/skills/rm-stale/SKILL.md is stale or its source entry no longer qualifies") {
		t.Fatalf("expected stale warning for rm-stale, got %v", warnings)
	}
}
