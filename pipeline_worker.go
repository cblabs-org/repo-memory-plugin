package tick

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	claimstore "github.com/cblabs-org/repo-memory-plugin/internal/core/claims/store"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/extract"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/publish"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/reconcile"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/verify"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

type PipelineExecutor struct {
	hk           harness.Housekeeper
	dataDir      string
	repoRoot     string
	repoID       string
	store        *jobs.Store
	epStore      *episode.Store
	claimStore   *claimstore.Store
	idx          *index.Index
	cfg          config.RepoConfig
	budgetUsed   int
	budgetLimit  int
}

func (p *PipelineExecutor) Run(ctx context.Context) error {
	return nil
}
