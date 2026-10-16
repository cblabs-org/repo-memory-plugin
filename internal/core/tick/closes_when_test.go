package tick

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitCmd(t, dir, "init", "-b", "main")
	runGitCmd(t, dir, "config", "user.name", "Test User")
	runGitCmd(t, dir, "config", "user.email", "test@users.noreply.github.com")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runGitCmd(t, dir, "add", "README.md")
	runGitCmd(t, dir, "commit", "-m", "initial commit")
}

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestEvaluateClosesWhenCondition_FileAndSymbolPredicates(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo"

	srcFile := filepath.Join(repoRoot, "internal", "svc", "handler.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package svc\n\n// TODO: remove legacy path\nfunc HandleV2() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		closesWhen string
		want       bool
	}{
		{
			name:       "path_exists existing file",
			closesWhen: "path_exists:internal/svc/handler.go",
			want:       true,
		},
		{
			name:       "file_exists alias with existing symbol",
			closesWhen: "file_exists:internal/svc/handler.go#HandleV2",
			want:       true,
		},
		{
			name:       "path_exists missing symbol",
			closesWhen: "path_exists:internal/svc/handler.go#MissingFunc",
			want:       false,
		},
		{
			name:       "path_absent missing file",
			closesWhen: "path_absent:internal/svc/legacy.go",
			want:       true,
		},
		{
			name:       "deleted alias for absent symbol in existing file",
			closesWhen: "deleted:internal/svc/handler.go:HandleV1",
			want:       true,
		},
		{
			name:       "path_absent fails when file and symbol exist",
			closesWhen: "path_absent:internal/svc/handler.go:HandleV2",
			want:       false,
		},
		{
			name:       "file_contains literal match",
			closesWhen: "file_contains:internal/svc/handler.go:func HandleV2()",
			want:       true,
		},
		{
			name:       "file_contains regex match",
			closesWhen: `file_contains:internal/svc/handler.go:re:func\s+HandleV[0-9]+\(\)`,
			want:       true,
		},
		{
			name:       "file_contains non-matching literal",
			closesWhen: "file_contains:internal/svc/handler.go:func HandleV3()",
			want:       false,
		},
		{
			name:       "file_not_contains when pattern absent",
			closesWhen: "file_not_contains:internal/svc/handler.go:DEPRECATED_FLAG",
			want:       true,
		},
		{
			name:       "file_not_contains when pattern present",
			closesWhen: "file_not_contains:internal/svc/handler.go:TODO: remove legacy path",
			want:       false,
		},
		{
			name:       "file_not_contains when file absent",
			closesWhen: "file_not_contains:internal/svc/nonexistent.go:anything",
			want:       true,
		},
		{
			name:       "compound AND satisfied",
			closesWhen: "path_exists:internal/svc/handler.go && file_contains:internal/svc/handler.go:HandleV2",
			want:       true,
		},
		{
			name:       "compound semicolon AND unsatisfied",
			closesWhen: "path_exists:internal/svc/handler.go; path_exists:internal/svc/missing.go",
			want:       false,
		},
		{
			name:       "compound OR satisfied on second branch",
			closesWhen: "path_exists:internal/svc/missing.go || file_contains:internal/svc/handler.go:HandleV2",
			want:       true,
		},
		{
			name:       "file_contains rejects parent traversal",
			closesWhen: "file_contains:../outside.txt:secret",
			want:       false,
		},
		{
			name:       "file_contains rejects absolute path",
			closesWhen: "file_contains:/etc/passwd:root",
			want:       false,
		},
		{
			name:       "file_not_contains rejects parent traversal",
			closesWhen: "file_not_contains:../outside.txt:secret",
			want:       false,
		},
		{
			name:       "file_not_contains rejects absolute path",
			closesWhen: "file_not_contains:/etc/passwd:nonexistent_token",
			want:       false,
		},
		{
			name:       "workorder_complete rejects parent traversal",
			closesWhen: "workorder_complete:../outside.txt",
			want:       false,
		},
		{
			name:       "workorder_complete rejects absolute path",
			closesWhen: "workorder_complete:/etc/passwd",
			want:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := dossiers.Entry{
				ID:         "iss-1",
				Type:       dossiers.EntryTypeOpenIssue,
				ClosesWhen: tc.closesWhen,
			}
			got, reason := EvaluateClosesWhenCondition(ctx, repoRoot, dataDir, repoID, nil, entry)
			if got != tc.want {
				t.Fatalf("EvaluateClosesWhenCondition(%q) = (%v, %q), want %v", tc.closesWhen, got, reason, tc.want)
			}
			if got && strings.TrimSpace(reason) == "" {
				t.Fatalf("expected non-empty reason when condition %q is satisfied", tc.closesWhen)
			}
		})
	}
}

func TestEvaluateClosesWhenCondition_BranchMerged(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	initTestGitRepo(t, repoRoot)

	runGitCmd(t, repoRoot, "checkout", "-b", "feat/issue-fix")
	if err := os.WriteFile(filepath.Join(repoRoot, "fix.txt"), []byte("fixed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repoRoot, "add", "fix.txt")
	runGitCmd(t, repoRoot, "commit", "-m", "fix issue on branch")

	entry := dossiers.Entry{
		ID:         "iss-branch",
		Type:       dossiers.EntryTypeOpenIssue,
		ClosesWhen: "branch_merged:feat/issue-fix",
	}

	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, "", "", nil, entry); ok {
		t.Fatalf("expected unmerged branch feat/issue-fix to evaluate false")
	}

	runGitCmd(t, repoRoot, "checkout", "main")
	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, "", "", nil, entry); ok {
		t.Fatalf("expected unmerged branch feat/issue-fix while on main to evaluate false")
	}

	runGitCmd(t, repoRoot, "merge", "--no-ff", "feat/issue-fix", "-m", "Merge feat/issue-fix")
	ok, reason := EvaluateClosesWhenCondition(ctx, repoRoot, "", "", nil, entry)
	if !ok {
		t.Fatalf("expected merged branch feat/issue-fix to evaluate true")
	}
	if !strings.Contains(reason, "feat/issue-fix") {
		t.Fatalf("expected reason to mention feat/issue-fix, got %q", reason)
	}
}

func TestEvaluateClosesWhenCondition_WorkorderComplete(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()

	woDir := filepath.Join(repoRoot, ".agents", "skills", "principal-architect-workspace", "pr-test", "wo1-foundation")
	if err := os.MkdirAll(woDir, 0o755); err != nil {
		t.Fatal(err)
	}
	woPath := filepath.Join(woDir, "WORKORDER.md")

	// Template with fenced Completion Report placeholder should NOT count as complete.
	incompleteMD := "# Work Order: wo1-foundation\n\nStatus: In Progress\n\n```markdown\n## Completion Report (<YYYY-MM-DD>)\n- **Commit:** `<sha>`\n```\n"
	if err := os.WriteFile(woPath, []byte(incompleteMD), 0o644); err != nil {
		t.Fatal(err)
	}

	entryBySlug := dossiers.Entry{
		ID:         "iss-wo1",
		Type:       dossiers.EntryTypeOpenIssue,
		ClosesWhen: "workorder_complete:wo1-foundation",
	}
	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, "", "", nil, entryBySlug); ok {
		t.Fatalf("expected incomplete workorder with template block to evaluate false")
	}

	// Real Completion Report outside fence satisfies workorder_complete.
	completeReportMD := incompleteMD + "\n## Completion Report (2026-10-16)\n- **Commit:** `a1b2c3d` — implemented foundation\n"
	if err := os.WriteFile(woPath, []byte(completeReportMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, "", "", nil, entryBySlug); !ok {
		t.Fatalf("expected workorder with real Completion Report to evaluate true")
	}

	// Also verify parent PROJECT.md marking a workorder slug as ✅ Complete.
	wo2Dir := filepath.Join(repoRoot, ".agents", "skills", "principal-architect-workspace", "pr-test", "wo2-followup")
	if err := os.MkdirAll(wo2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wo2Dir, "WORKORDER.md"), []byte("# Work Order: wo2-followup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projPath := filepath.Join(filepath.Dir(wo2Dir), "PROJECT.md")
	if err := os.WriteFile(projPath, []byte("| `wo2-followup` | Followup | ✅ Complete |\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entryWO2 := dossiers.Entry{
		ID:         "iss-wo2",
		Type:       dossiers.EntryTypeOpenIssue,
		ClosesWhen: "workorder_complete:wo2-followup",
	}
	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, "", "", nil, entryWO2); !ok {
		t.Fatalf("expected workorder marked ✅ Complete in parent PROJECT.md to evaluate true")
	}
}

func TestEvaluateClosesWhenCondition_CommandPasses(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo"
	epStore := episode.NewStore(dataDir, repoID)

	issueCreated := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	entry := dossiers.Entry{
		ID:         "iss-cmd",
		Type:       dossiers.EntryTypeOpenIssue,
		CreatedAt:  issueCreated,
		ClosesWhen: "command_passes:go test ./internal/core/tick/...",
	}

	exitZero := 0
	exitOne := 1

	// 1. Episode BEFORE entry.CreatedAt should not satisfy closes_when.
	oldEp := episode.NewEpisode(episode.Header{
		SessionID: "sess-old",
		Started:   issueCreated.Add(-2 * time.Hour),
		Ended:     issueCreated.Add(-1 * time.Hour),
		Status:    "success",
	}, harness.Step{
		Index:    1,
		Kind:     harness.StepKindCommand,
		Command:  "go test ./internal/core/tick/...",
		ExitCode: &exitZero,
	})
	if _, err := epStore.WriteEpisode(oldEp); err != nil {
		t.Fatal(err)
	}
	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, dataDir, repoID, epStore, entry); ok {
		t.Fatalf("expected episode recorded before entry.CreatedAt not to satisfy command_passes")
	}

	// 2. Failing episode AFTER entry.CreatedAt should not satisfy closes_when.
	failEp := episode.NewEpisode(episode.Header{
		SessionID: "sess-fail",
		Started:   issueCreated.Add(10 * time.Minute),
		Ended:     issueCreated.Add(15 * time.Minute),
	}, harness.Step{
		Index:    1,
		Kind:     harness.StepKindCommand,
		Command:  "go test ./internal/core/tick/...",
		ExitCode: &exitOne,
	})
	if _, err := epStore.WriteEpisode(failEp); err != nil {
		t.Fatal(err)
	}
	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, dataDir, repoID, epStore, entry); ok {
		t.Fatalf("expected failing command episode not to satisfy command_passes")
	}

	// 3. Passing episode AFTER entry.CreatedAt matching via CommandMatchesRecipe satisfies closes_when.
	passEp := episode.NewEpisode(episode.Header{
		SessionID: "sess-pass",
		Started:   issueCreated.Add(20 * time.Minute),
		Ended:     issueCreated.Add(25 * time.Minute),
	}, harness.Step{
		Index:    1,
		Kind:     harness.StepKindCommand,
		Command:  "go test ./internal/core/tick/... -v",
		ExitCode: &exitZero,
	})
	if _, err := epStore.WriteEpisode(passEp); err != nil {
		t.Fatal(err)
	}
	ok, reason := EvaluateClosesWhenCondition(ctx, repoRoot, dataDir, repoID, epStore, entry)
	if !ok {
		t.Fatalf("expected passing episode recorded after entry.CreatedAt to satisfy command_passes")
	}
	if !strings.Contains(reason, "sess-pass") {
		t.Fatalf("expected reason to cite sess-pass, got %q", reason)
	}

	// 4. Also verify OutcomeState direct check via test_passes alias.
	dataDir2 := t.TempDir()
	st := &skills.OutcomeState{
		Entries: map[string]*skills.EntryOutcome{
			"pytest tests/unit": {
				SucceededConversations: []string{"conv-1"},
			},
		},
	}
	if err := skills.SaveOutcomeState(dataDir2, repoID, st); err != nil {
		t.Fatal(err)
	}
	aliasEntry := dossiers.Entry{
		ID:         "iss-pytest",
		Type:       dossiers.EntryTypeOpenIssue,
		ClosesWhen: "test_passes:pytest tests/unit",
	}
	if ok, _ := EvaluateClosesWhenCondition(ctx, repoRoot, dataDir2, repoID, nil, aliasEntry); !ok {
		t.Fatalf("expected test_passes to be satisfied by OutcomeState")
	}
}

func TestPipelineExecutor_EvaluateClosesWhen_EndToEnd(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()
	dataDir := t.TempDir()
	repoID := "test-repo"
	initTestGitRepo(t, repoRoot)

	// Create the target file that satisfies iss-1's closes_when condition.
	targetPath := filepath.Join(repoRoot, "internal", "feature.go")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("package internal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	initialOps := []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "iss-satisfied",
			DossierScope: "internal",
			Timestamp:    now.Add(-1 * time.Hour),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type":  string(dossiers.EntryTypeOpenIssue),
				"text":        "**Missing feature.go**\nDo: Add internal/feature.go.",
				"closes_when": "path_exists:internal/feature.go",
			},
		},
		{
			Type:         ops.OpAdd,
			EntryID:      "iss-unsatisfied",
			DossierScope: "internal",
			Timestamp:    now.Add(-1 * time.Hour),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type":  string(dossiers.EntryTypeOpenIssue),
				"text":        "**Missing other.go**\nDo: Add internal/other.go.",
				"closes_when": "path_exists:internal/other.go",
			},
		},
	}
	if _, err := consolidate.RecordRepoTier(ctx, repoRoot, initialOps, nil, nil, now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("seeding initial repo ops: %v", err)
	}

	// Also seed a branch-local open-issue in .memory/local/ops/<branch>.
	branchOps := []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "iss-branch-satisfied",
			DossierScope: "internal",
			Timestamp:    now.Add(-1 * time.Hour),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type":  string(dossiers.EntryTypeOpenIssue),
				"text":        "**Branch issue resolved**\nDo: Verify feature.go exists.",
				"closes_when": "file_exists:internal/feature.go",
			},
		},
	}
	if _, err := consolidate.RecordBranchTier(ctx, repoRoot, "feat-branch", branchOps, nil, nil, now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("seeding branch ops: %v", err)
	}

	idxPath := index.IndexPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	if err := consolidate.RebuildMultiTierIndex(ctx, idx, dataDir, repoID, repoRoot, "feat-branch"); err != nil {
		t.Fatal(err)
	}

	exec := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, episode.NewStore(dataDir, repoID))
	exec.SetIndex(idx)
	exec.Now = func() time.Time { return now }

	res, err := exec.EvaluateClosesWhen(ctx)
	if err != nil {
		t.Fatalf("EvaluateClosesWhen failed: %v", err)
	}

	if res.Evaluated != 3 {
		t.Fatalf("expected 3 evaluated open-issues, got %d", res.Evaluated)
	}
	if len(res.Closed) != 2 {
		t.Fatalf("expected 2 closed open-issues, got %d: %+v", len(res.Closed), res.Closed)
	}

	// Verify iss-satisfied is retired in .memory/ops/ and moved to History.
	projected, err := dossiers.NewProjector(ops.NewReader(filepath.Join(repoRoot, ".memory", "ops"))).Project(ctx)
	if err != nil {
		t.Fatal(err)
	}
	d := projected["internal"]
	if d == nil {
		t.Fatalf("expected dossier 'internal' to exist")
	}
	if len(d.Entries) != 1 || d.Entries[0].ID != "iss-unsatisfied" {
		t.Fatalf("expected only iss-unsatisfied to remain active, got %+v", d.Entries)
	}
	if len(d.History) != 1 || d.History[0].ID != "iss-satisfied" || !strings.HasPrefix(d.History[0].RetiredReason, "closes_when satisfied: ") {
		t.Fatalf("expected iss-satisfied in History with closes_when satisfied reason, got %+v", d.History)
	}

	// Verify SQLite index no longer contains iss-satisfied.
	if ent, _ := idx.GetEntry(ctx, "iss-satisfied"); ent != nil {
		t.Fatalf("expected iss-satisfied to be removed from active index, got %+v", ent)
	}
	if ent, _ := idx.GetEntry(ctx, "iss-unsatisfied"); ent == nil {
		t.Fatalf("expected iss-unsatisfied to remain in active index")
	}
}
