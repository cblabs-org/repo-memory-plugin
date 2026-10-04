package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
)

func TestStoreSetupAndPragmas(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "subdir", "jobs.sqlite")

	store, err := jobs.Open(dbPath,
		jobs.WithBusyTimeout(4000*time.Millisecond),
		jobs.WithMaxAttempts(3),
		jobs.WithDefaultLeaseDuration(2*time.Minute),
	)
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	// Verify WAL mode
	var journalMode string
	err = store.DB().QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode 'wal', got %q", journalMode)
	}

	// Verify busy_timeout
	var busyTimeout int
	err = store.DB().QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout)
	if err != nil {
		t.Fatalf("failed to query busy_timeout: %v", err)
	}
	if busyTimeout != 4000 {
		t.Errorf("expected busy_timeout 4000, got %d", busyTimeout)
	}

	// Verify schema columns
	rows, err := store.DB().Query("PRAGMA table_info(jobs);")
	if err != nil {
		t.Fatalf("failed to query table_info: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]string)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan table_info: %v", err)
		}
		columns[name] = colType
	}

	requiredColumns := []string{
		"id",
		"kind",
		"priority",
		"idempotency_key",
		"state",
		"lease_expires",
		"attempts",
		"next_attempt_at",
		"cost",
	}

	for _, col := range requiredColumns {
		if _, exists := columns[col]; !exists {
			t.Errorf("missing required column %q in jobs table", col)
		}
	}
}

func TestEnqueueAndIdempotency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath)
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Enqueue without idempotency key
	job1, err := store.Enqueue(ctx, &jobs.Job{
		Kind:     jobs.KindCapture,
		Priority: jobs.PriorityBatch,
		Payload:  `{"transcript":"path/to/transcript.jsonl"}`,
	})
	if err != nil {
		t.Fatalf("failed to enqueue job1: %v", err)
	}
	if job1.ID == "" {
		t.Error("expected generated ID for job1, got empty string")
	}
	if job1.State != jobs.StateQueued {
		t.Errorf("expected StateQueued, got %s", job1.State)
	}

	// 2. Enqueue with idempotency key
	key := "idem-test-key-12345"
	job2, err := store.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindCompact,
		Priority:       jobs.PriorityInteractive,
		IdempotencyKey: key,
		Payload:        "original payload",
	})
	if err != nil {
		t.Fatalf("failed to enqueue job2: %v", err)
	}
	if job2.IdempotencyKey != key {
		t.Errorf("expected key %s, got %s", key, job2.IdempotencyKey)
	}

	// 3. Re-enqueue with the SAME idempotency key (must be a no-op returning existing job)
	job3, err := store.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindCompact,
		Priority:       jobs.PriorityNormal,
		IdempotencyKey: key,
		Payload:        "different payload that should be ignored",
	})
	if err != nil {
		t.Fatalf("expected no-op on duplicate idempotency key, got error: %v", err)
	}
	if job3.ID != job2.ID {
		t.Errorf("expected existing job ID %s, got %s", job2.ID, job3.ID)
	}
	if job3.Payload != "original payload" {
		t.Errorf("expected existing job payload to be preserved, got %s", job3.Payload)
	}

	// 4. Verify total count in DB
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("store.Stats failed: %v", err)
	}
	if stats.Total != 2 {
		t.Errorf("expected total 2 jobs, got %d", stats.Total)
	}
}

func TestConcurrentEnqueueIdempotency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath)
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	const key = "concurrent-idempotency-key"
	const goroutines = 10

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	jobIDs := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			j, err := store.Enqueue(ctx, &jobs.Job{
				Kind:           jobs.KindTriage,
				Priority:       jobs.PriorityNormal,
				IdempotencyKey: key,
				Payload:        fmt.Sprintf("worker-%d", idx),
			})
			if err != nil {
				errs <- err
				return
			}
			jobIDs <- j.ID
		}(i)
	}

	wg.Wait()
	close(errs)
	close(jobIDs)

	for err := range errs {
		t.Errorf("concurrent enqueue error: %v", err)
	}

	firstID := ""
	for id := range jobIDs {
		if firstID == "" {
			firstID = id
		} else if id != firstID {
			t.Errorf("expected consistent job ID %s, got %s", firstID, id)
		}
	}

	// Exactly 1 job should be in DB
	jobsList, err := store.List(ctx, jobs.Filter{})
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}
	if len(jobsList) != 1 {
		t.Errorf("expected 1 job in DB, got %d", len(jobsList))
	}
}

func TestJobLifecycleSuccess(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath)
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Enqueue job
	enqueued, err := store.Enqueue(ctx, &jobs.Job{
		Kind:     jobs.KindExtract,
		Priority: jobs.PriorityInteractive,
		Payload:  "test-session-123",
	})
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Acquire job (queued -> running)
	leased, err := store.AcquireNext(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("failed to acquire next: %v", err)
	}
	if leased == nil {
		t.Fatal("expected acquired job, got nil")
	}
	if leased.ID != enqueued.ID {
		t.Errorf("expected job %s, got %s", enqueued.ID, leased.ID)
	}
	if leased.State != jobs.StateRunning {
		t.Errorf("expected StateRunning, got %s", leased.State)
	}
	if leased.Attempts != 1 {
		t.Errorf("expected attempts 1, got %d", leased.Attempts)
	}
	if leased.LeaseExpires == nil || leased.LeaseExpires.Before(time.Now().UTC()) {
		t.Errorf("expected future lease_expires, got %v", leased.LeaseExpires)
	}

	// Renew lease while running
	err = store.RenewLease(ctx, leased.ID, 1*time.Minute)
	if err != nil {
		t.Fatalf("failed to renew lease: %v", err)
	}

	refreshed, err := store.Get(ctx, leased.ID)
	if err != nil {
		t.Fatalf("failed to get refreshed job: %v", err)
	}
	if refreshed.LeaseExpires == nil || refreshed.LeaseExpires.Before(*leased.LeaseExpires) {
		t.Errorf("expected extended lease_expires, got %v", refreshed.LeaseExpires)
	}

	// Complete job (running -> done)
	cost := &jobs.Cost{
		InputTokens:  16813,
		OutputTokens: 5,
		DurationMS:   12800,
		Model:        "gemini-3.8-flash-low",
	}
	err = store.Complete(ctx, leased.ID, cost)
	if err != nil {
		t.Fatalf("failed to complete job: %v", err)
	}

	doneJob, err := store.Get(ctx, leased.ID)
	if err != nil {
		t.Fatalf("failed to get done job: %v", err)
	}
	if doneJob.State != jobs.StateDone {
		t.Errorf("expected StateDone, got %s", doneJob.State)
	}
	if doneJob.LeaseExpires != nil {
		t.Errorf("expected nil lease_expires after done, got %v", doneJob.LeaseExpires)
	}
	if doneJob.Cost == nil {
		t.Fatal("expected cost to be recorded, got nil")
	}
	if doneJob.Cost.InputTokens != 16813 || doneJob.Cost.Model != "gemini-3.8-flash-low" {
		t.Errorf("unexpected cost values: %+v", doneJob.Cost)
	}

	// No more jobs available
	next, err := store.AcquireNext(ctx, 30*time.Second)
	if !errors.Is(err, jobs.ErrNoJobAvailable) {
		t.Fatalf("AcquireNext expected ErrNoJobAvailable, got: %v", err)
	}
	if next != nil {
		t.Errorf("expected nil next job, got %+v", next)
	}
}

func TestJobLifecycleRetryAndDead(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath, jobs.WithMaxAttempts(3))
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Enqueue
	j, err := store.Enqueue(ctx, &jobs.Job{
		Kind:     jobs.KindReconcile,
		Priority: jobs.PriorityNormal,
	})
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}
	if j.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// Attempt 1: Acquire and fail
	job, err := store.AcquireNext(ctx, 10*time.Second)
	if err != nil || job == nil {
		t.Fatalf("failed to acquire attempt 1: %v", err)
	}
	if job.Attempts != 1 {
		t.Errorf("expected attempt 1, got %d", job.Attempts)
	}

	// Fail with backoff of 0 (defaults to backoff logic)
	err = store.Fail(ctx, job.ID, "transient network error", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to fail job: %v", err)
	}

	failedJob, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if failedJob.State != jobs.StateFailed {
		t.Errorf("expected StateFailed, got %s", failedJob.State)
	}
	if failedJob.LastError != "transient network error" {
		t.Errorf("expected last_error recorded, got %q", failedJob.LastError)
	}
	if failedJob.NextAttemptAt == nil {
		t.Fatal("expected next_attempt_at to be set")
	}

	// Wait for backoff to elapse
	time.Sleep(20 * time.Millisecond)

	// Attempt 2: Re-acquired from failed state
	job2, err := store.AcquireNext(ctx, 10*time.Second)
	if err != nil || job2 == nil {
		t.Fatalf("failed to acquire attempt 2: %v", err)
	}
	if job2.Attempts != 2 {
		t.Errorf("expected attempt 2, got %d", job2.Attempts)
	}
	if job2.State != jobs.StateRunning {
		t.Errorf("expected running, got %s", job2.State)
	}

	// Fail attempt 2
	err = store.Fail(ctx, job2.ID, "still broken", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to fail job attempt 2: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	// Attempt 3: Max attempts is 3. Re-acquire.
	job3, err := store.AcquireNext(ctx, 10*time.Second)
	if err != nil || job3 == nil {
		t.Fatalf("failed to acquire attempt 3: %v", err)
	}
	if job3.Attempts != 3 {
		t.Errorf("expected attempt 3, got %d", job3.Attempts)
	}

	// Fail attempt 3: Since attempts (3) >= maxAttempts (3), it must transition to dead!
	err = store.Fail(ctx, job3.ID, "fatal after 3 attempts", 0)
	if err != nil {
		t.Fatalf("failed to fail job attempt 3: %v", err)
	}

	deadJob, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed to get dead job: %v", err)
	}
	if deadJob.State != jobs.StateDead {
		t.Errorf("expected StateDead after 3 attempts, got %s", deadJob.State)
	}
	if deadJob.NextAttemptAt != nil {
		t.Errorf("expected nil next_attempt_at for dead job, got %v", deadJob.NextAttemptAt)
	}
}

func TestExplicitMarkDead(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath)
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	job, err := store.Enqueue(ctx, &jobs.Job{
		Kind: jobs.KindVerify,
	})
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	leased, err := store.AcquireNext(ctx, 10*time.Second)
	if err != nil || leased == nil {
		t.Fatalf("acquire failed: %v", err)
	}

	err = store.MarkDead(ctx, job.ID, "malformed inputs; non-retryable")
	if err != nil {
		t.Fatalf("mark dead failed: %v", err)
	}

	deadJob, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if deadJob.State != jobs.StateDead {
		t.Errorf("expected StateDead, got %s", deadJob.State)
	}
	if deadJob.LastError != "malformed inputs; non-retryable" {
		t.Errorf("expected custom error message, got %q", deadJob.LastError)
	}
}

func TestPriorityAndKindScheduling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath)
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Enqueue batch job first
	batchJob, _ := store.Enqueue(ctx, &jobs.Job{
		Kind:     jobs.KindPublish,
		Priority: jobs.PriorityBatch,
	})

	// Enqueue low job second
	lowJob, _ := store.Enqueue(ctx, &jobs.Job{
		Kind:     jobs.KindSweep,
		Priority: jobs.PriorityLow,
	})

	// Enqueue interactive job third
	interactiveJob, _ := store.Enqueue(ctx, &jobs.Job{
		Kind:     jobs.KindBrief,
		Priority: jobs.PriorityInteractive,
	})

	// First acquire should get interactiveJob despite enqueued last
	a1, err := store.AcquireNext(ctx, 10*time.Second)
	if err != nil || a1 == nil {
		t.Fatalf("failed to acquire 1: %v", err)
	}
	if a1.ID != interactiveJob.ID {
		t.Errorf("expected interactive job %s first, got %s", interactiveJob.ID, a1.ID)
	}

	// Filter by kind: ask specifically for sweep (which has PriorityLow)
	aSweep, err := store.AcquireNext(ctx, 10*time.Second, jobs.KindSweep)
	if err != nil || aSweep == nil {
		t.Fatalf("failed to acquire sweep: %v", err)
	}
	if aSweep.ID != lowJob.ID {
		t.Errorf("expected sweep job %s, got %s", lowJob.ID, aSweep.ID)
	}

	// Next general acquire gets batchJob
	a3, err := store.AcquireNext(ctx, 10*time.Second)
	if err != nil || a3 == nil {
		t.Fatalf("failed to acquire 3: %v", err)
	}
	if a3.ID != batchJob.ID {
		t.Errorf("expected batch job %s, got %s", batchJob.ID, a3.ID)
	}
}

func TestLeaseReclamation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath, jobs.WithMaxAttempts(3))
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Enqueue job that will expire but has attempts left (attempt 1 of 3)
	j1, _ := store.Enqueue(ctx, &jobs.Job{
		Kind: jobs.KindCompact,
	})
	leased1, _ := store.AcquireNext(ctx, 50*time.Millisecond)

	// 2. Enqueue job that will expire and has reached max attempts (simulating 3 crashes)
	j2, _ := store.Enqueue(ctx, &jobs.Job{
		Kind: jobs.KindTriage,
	})
	// Manually simulate 3 previous attempts
	_, err = store.DB().Exec("UPDATE jobs SET attempts = 3 WHERE id = ?", j2.ID)
	if err != nil {
		t.Fatalf("setup update failed: %v", err)
	}
	leased2, _ := store.AcquireNext(ctx, 50*time.Millisecond)

	// 3. Enqueue job with a long lease that should NOT expire
	j3, _ := store.Enqueue(ctx, &jobs.Job{
		Kind: jobs.KindExtract,
	})
	leased3, _ := store.AcquireNext(ctx, 1*time.Hour)
	if leased3 == nil || leased3.ID != j3.ID {
		t.Fatalf("expected leased3 ID to match j3 ID")
	}

	// Wait for the short leases to expire
	time.Sleep(100 * time.Millisecond)

	// Run lease reclamation
	now := time.Now().UTC()
	result, err := store.ReclaimExpiredLeasesDetailed(ctx, now)
	if err != nil {
		t.Fatalf("ReclaimExpiredLeasesDetailed failed: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected 2 total reclaimed jobs, got %d", result.Total)
	}
	if result.Requeued != 1 {
		t.Errorf("expected 1 requeued job, got %d", result.Requeued)
	}
	if result.Deadened != 1 {
		t.Errorf("expected 1 deadened job, got %d", result.Deadened)
	}

	// Verify j1 was reset to queued
	check1, _ := store.Get(ctx, leased1.ID)
	if check1.State != jobs.StateQueued {
		t.Errorf("expected j1 to be queued, got %s", check1.State)
	}
	if check1.LeaseExpires != nil {
		t.Errorf("expected nil lease_expires for j1, got %v", check1.LeaseExpires)
	}

	// Verify j2 was marked dead
	check2, _ := store.Get(ctx, leased2.ID)
	if check2.State != jobs.StateDead {
		t.Errorf("expected j2 to be dead, got %s", check2.State)
	}
	if check2.LeaseExpires != nil {
		t.Errorf("expected nil lease_expires for j2, got %v", check2.LeaseExpires)
	}

	// Verify j3 is still running with active lease
	check3, _ := store.Get(ctx, leased3.ID)
	if check3.State != jobs.StateRunning {
		t.Errorf("expected j3 to still be running, got %s", check3.State)
	}
	if check3.LeaseExpires == nil || check3.LeaseExpires.Before(now) {
		t.Errorf("expected valid active lease for j3, got %v", check3.LeaseExpires)
	}

	// Verify j1 can be acquired again!
	reacquired, err := store.AcquireNext(ctx, 10*time.Second)
	if err != nil || reacquired == nil {
		t.Fatalf("failed to reacquire j1: %v", err)
	}
	if reacquired.ID != j1.ID {
		t.Errorf("expected reacquired j1 ID %s, got %s", j1.ID, reacquired.ID)
	}
	if reacquired.Attempts != 2 {
		t.Errorf("expected attempts 2 for reacquired j1, got %d", reacquired.Attempts)
	}
}

func TestStoreListAndDelete(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "jobs.sqlite")

	store, err := jobs.Open(dbPath)
	if err != nil {
		t.Fatalf("jobs.Open failed: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	_, _ = store.Enqueue(ctx, &jobs.Job{Kind: jobs.KindCapture, Priority: 1})
	_, _ = store.Enqueue(ctx, &jobs.Job{Kind: jobs.KindCapture, Priority: 2})
	j3, _ := store.Enqueue(ctx, &jobs.Job{Kind: jobs.KindBrief, Priority: 3})

	// List with Kind filter
	captures, err := store.List(ctx, jobs.Filter{Kind: jobs.KindCapture})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(captures) != 2 {
		t.Errorf("expected 2 capture jobs, got %d", len(captures))
	}

	// Delete job
	err = store.Delete(ctx, j3.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get(ctx, j3.ID)
	if err != jobs.ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound after delete, got %v", err)
	}

	// Deleting again should return ErrJobNotFound
	err = store.Delete(ctx, j3.ID)
	if err != jobs.ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound on second delete, got %v", err)
	}
}
