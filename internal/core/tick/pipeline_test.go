package tick_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

func TestPipelineBudgetCapping(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755)

	cfg := config.DefaultRepoConfig()
	cfg.Budgets.DailyTokens = 1000
	config.SaveRepoConfig(repoRoot, cfg)

	store, err := jobs.Open(filepath.Join(dataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Enqueue a job that will trigger budget check
	store.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindTriage,
		Priority:       10,
		IdempotencyKey: "triage-1",
		Payload:        "triage-1",
	})

	ticker := tick.NewTicker(dataDir, time.Minute)
	_ = ticker
}
