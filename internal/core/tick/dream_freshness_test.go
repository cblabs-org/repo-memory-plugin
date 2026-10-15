package tick_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

func writeRepoFile(t *testing.T, repoRoot, relPath, content string) {
	t.Helper()
	full := filepath.Join(repoRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestCheckDocFreshness_NoOpWhenUnseeded(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	repoID := "repo-unseeded"

	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatalf("mkdir repoRoot: %v", err)
	}
	// Create an unrelated doc in docs/ to verify unseeded workspaces do not auto-seed unless configured or queued
	writeRepoFile(t, repoRoot, "internal/foo/bar.go", "package foo\n")
	writeRepoFile(t, repoRoot, "docs/arch.md", "# Arch\n- **Rule One**: All calls in `internal/foo/bar.go` must validate inputs.\n")

	repoDataDir := filepath.Join(dataDir, repoID)
	if err := os.MkdirAll(repoDataDir, 0755); err != nil {
		t.Fatalf("mkdir repoDataDir: %v", err)
	}
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer jobStore.Close()

	epStore := episode.NewStore(dataDir, repoID)
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := exec.CheckDocFreshness(ctx)
	if err != nil {
		t.Fatalf("CheckDocFreshness failed: %v", err)
	}
	if res.FilesScanned != 0 || res.EntriesAdded != 0 {
		t.Errorf("expected no-op on unseeded workspace, got %+v", res)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".memory", "dossiers")); !os.IsNotExist(err) {
		t.Errorf("expected .memory/dossiers not to be created on unseeded no-op")
	}
}

func TestCheckDocFreshness_DetectsModifiedAndDeletedDocs(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	repoID := "repo-dream-freshness"

	writeRepoFile(t, repoRoot, "internal/foo/bar.go", "package foo\n")

	cfg := config.DefaultRepoConfig()
	cfg.DocSeeds = []string{"docs/arch.md"}
	if err := config.SaveRepoConfig(repoRoot, cfg); err != nil {
		t.Fatalf("SaveRepoConfig: %v", err)
	}

	initialDoc := `# Architecture Rules

## Invariants
- **Strict Validation**: Every handler in ` + "`internal/foo/bar.go`" + ` must validate input payloads before mutating state.
- **No Global Locks**: Never hold a global mutex in ` + "`internal/foo/bar.go`" + ` across network I/O boundaries.
`
	writeRepoFile(t, repoRoot, "docs/arch.md", initialDoc)
	initialSHA := seed.ComputeGitBlobSHA(repoRoot, "docs/arch.md", []byte(initialDoc))

	repoDataDir := filepath.Join(dataDir, repoID)
	if err := os.MkdirAll(repoDataDir, 0755); err != nil {
		t.Fatalf("mkdir repoDataDir: %v", err)
	}
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer jobStore.Close()

	epStore := episode.NewStore(dataDir, repoID)
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Step 1: Initial freshness run seeds the 2 entries, projects dossiers, and rebuilds index.db
	res1, err := exec.CheckDocFreshness(ctx)
	if err != nil {
		t.Fatalf("Step 1 CheckDocFreshness failed: %v", err)
	}
	if res1.EntriesAdded != 2 {
		t.Fatalf("Step 1 EntriesAdded = %d, want 2", res1.EntriesAdded)
	}

	idx, err := index.NewIndex(index.IndexPath(dataDir, repoID))
	if err != nil {
		t.Fatalf("opening index: %v", err)
	}
	defer idx.Close()

	entries1, err := idx.ListEntriesBySource(ctx, ops.SourceDocSeed)
	if err != nil {
		t.Fatalf("ListEntriesBySource Step 1: %v", err)
	}
	if len(entries1) != 2 {
		t.Fatalf("Step 1 indexed doc-seed entries = %d, want 2", len(entries1))
	}
	for _, e := range entries1 {
		if e.SourceDocPath != "docs/arch.md" {
			t.Errorf("entry %s SourceDocPath = %q, want docs/arch.md", e.ID, e.SourceDocPath)
		}
		if e.SourceDocBlobSHA != initialSHA {
			t.Errorf("entry %s SourceDocBlobSHA = %q, want %q", e.ID, e.SourceDocBlobSHA, initialSHA)
		}
	}

	dossierBytes, err := os.ReadFile(filepath.Join(repoRoot, ".memory", "dossiers", "foo.md"))
	if err != nil {
		t.Fatalf("reading projected dossier foo.md: %v", err)
	}
	dossierStr := string(dossierBytes)
	if !strings.Contains(dossierStr, "source_doc: docs/arch.md") {
		t.Errorf("expected foo.md to contain 'source_doc: docs/arch.md', got:\n%s", dossierStr)
	}
	if !strings.Contains(dossierStr, "kind: semantic") {
		t.Errorf("expected foo.md to contain 'kind: semantic', got:\n%s", dossierStr)
	}

	// Step 2: Unchanged doc should be skipped in O(1) via blob SHA comparison
	res2, err := exec.CheckDocFreshness(ctx)
	if err != nil {
		t.Fatalf("Step 2 CheckDocFreshness failed: %v", err)
	}
	if res2.FilesSkipped != 1 || res2.EntriesAdded != 0 || res2.EntriesAmend != 0 || res2.EntriesRetire != 0 {
		t.Fatalf("Step 2 expected 1 skipped and 0 mutations, got %+v", res2)
	}

	// Step 3: Modify docs/arch.md (amend 1st bullet, remove 2nd bullet)
	modifiedDoc := `# Architecture Rules

## Invariants
- **Strict Validation**: Every handler in ` + "`internal/foo/bar.go`" + ` must validate input payloads and enforce strict timeouts.
`
	writeRepoFile(t, repoRoot, "docs/arch.md", modifiedDoc)
	modifiedSHA := seed.ComputeGitBlobSHA(repoRoot, "docs/arch.md", []byte(modifiedDoc))

	res3, err := exec.CheckDocFreshness(ctx)
	if err != nil {
		t.Fatalf("Step 3 CheckDocFreshness failed: %v", err)
	}
	if res3.EntriesAmend != 1 {
		t.Errorf("Step 3 EntriesAmend = %d, want 1", res3.EntriesAmend)
	}
	if res3.EntriesRetire != 1 {
		t.Errorf("Step 3 EntriesRetire = %d, want 1", res3.EntriesRetire)
	}

	entries3, err := idx.ListEntriesBySource(ctx, ops.SourceDocSeed)
	if err != nil {
		t.Fatalf("ListEntriesBySource Step 3: %v", err)
	}
	if len(entries3) != 1 {
		t.Fatalf("Step 3 indexed doc-seed entries = %d, want 1", len(entries3))
	}
	if entries3[0].SourceDocBlobSHA != modifiedSHA {
		t.Errorf("Step 3 entry blob SHA = %q, want %q", entries3[0].SourceDocBlobSHA, modifiedSHA)
	}
	if !strings.Contains(entries3[0].Text, "strict timeouts") {
		t.Errorf("Step 3 entry text not updated, got %q", entries3[0].Text)
	}

	// Step 4: Delete docs/arch.md from disk -> remaining entry is retired with source_doc_deleted
	if err := os.Remove(filepath.Join(repoRoot, "docs", "arch.md")); err != nil {
		t.Fatalf("removing docs/arch.md: %v", err)
	}

	res4, err := exec.CheckDocFreshness(ctx)
	if err != nil {
		t.Fatalf("Step 4 CheckDocFreshness failed: %v", err)
	}
	if res4.EntriesRetire != 1 {
		t.Fatalf("Step 4 EntriesRetire = %d, want 1", res4.EntriesRetire)
	}

	entries4, err := idx.ListEntriesBySource(ctx, ops.SourceDocSeed)
	if err != nil {
		t.Fatalf("ListEntriesBySource Step 4: %v", err)
	}
	if len(entries4) != 0 {
		t.Fatalf("Step 4 expected 0 active doc-seed entries after file deletion, got %d", len(entries4))
	}

	hist, err := idx.QueryHistoryByScope(ctx, "foo")
	if err != nil {
		t.Fatalf("QueryHistoryByScope Step 4: %v", err)
	}
	foundDeletedReason := false
	for _, h := range hist {
		if h.RetiredReason == "source_doc_deleted: docs/arch.md" {
			foundDeletedReason = true
			break
		}
	}
	if !foundDeletedReason {
		t.Errorf("expected history entry with RetiredReason 'source_doc_deleted: docs/arch.md', got %+v", hist)
	}
}

func TestCheckDocFreshness_DrainsDreamFreshnessJob(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	repoID := "repo-dream-job"

	writeRepoFile(t, repoRoot, "internal/foo/bar.go", "package foo\n")
	writeRepoFile(t, repoRoot, "docs/arch.md", "# Arch\n- **Invariant**: Handlers in `internal/foo/bar.go` must be idempotent across retries.\n")

	repoDataDir := filepath.Join(dataDir, repoID)
	if err := os.MkdirAll(repoDataDir, 0755); err != nil {
		t.Fatalf("mkdir repoDataDir: %v", err)
	}
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer jobStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dreamJob, err := jobStore.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindDreamFreshness,
		Priority:       jobs.PriorityLow,
		IdempotencyKey: "dream-freshness-1",
	})
	if err != nil {
		t.Fatalf("Enqueue KindDreamFreshness: %v", err)
	}

	epStore := episode.NewStore(dataDir, repoID)
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)

	if err := exec.Run(ctx); err != nil {
		t.Fatalf("exec.Run failed: %v", err)
	}

	updatedJob, err := jobStore.Get(ctx, dreamJob.ID)
	if err != nil {
		t.Fatalf("jobStore.Get: %v", err)
	}
	if updatedJob.State != jobs.StateDone {
		t.Errorf("expected KindDreamFreshness job state %q, got %q", jobs.StateDone, updatedJob.State)
	}
}
