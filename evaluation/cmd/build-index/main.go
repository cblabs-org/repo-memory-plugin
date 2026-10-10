// build-index builds a standalone index.db from a snapshot ops directory,
// filtering out ops after a timestamp cutoff. Used by the X6-lite evaluation
// to create per-run indexes for condition (c).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func main() {
	opsDir := flag.String("ops", "", "source ops dir (e.g. evaluation/data/<wo>/ops)")
	filteredDir := flag.String("filtered", "", "output dir for cutoff-filtered ops (e.g. $RUN_ROOT/ops)")
	dbPath := flag.String("db", "", "output path for index.db")
	cutoff := flag.String("cutoff", "", "RFC3339 timestamp cutoff (ops after this are dropped)")
	flag.Parse()

	if *opsDir == "" || *dbPath == "" || *cutoff == "" || *filteredDir == "" {
		fmt.Fprintf(os.Stderr, "Usage: build-index -ops <dir> -filtered <dir> -db <path> -cutoff <RFC3339>\n")
		os.Exit(1)
	}

	cutoffTime, err := time.Parse(time.RFC3339, *cutoff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: invalid cutoff time %q: %v\n", *cutoff, err)
		os.Exit(1)
	}

	// Step 1: Filter ops — copy only batches where ALL ops are <= cutoff.
	droppedBatches, droppedOps, keptEntries, latestTimestamp, err := filterOps(*opsDir, *filteredDir, cutoffTime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: filtering ops: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Filtered ops: kept %d entries, dropped %d batches (%d ops), latest=%s\n",
		keptEntries, droppedBatches, droppedOps, latestTimestamp.Format(time.RFC3339))

	if keptEntries == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: no entries survived the cutoff filter\n")
		os.Exit(1)
	}

	// Step 2: Build index from filtered ops.
	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: creating db dir: %v\n", err)
		os.Exit(1)
	}

	idx, err := index.NewIndex(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: creating index: %v\n", err)
		os.Exit(1)
	}
	defer idx.Close()

	reader := ops.NewReader(*filteredDir)
	if err := idx.RebuildFromOpLog(context.Background(), reader); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: rebuilding index: %v\n", err)
		os.Exit(1)
	}

	// Step 2b: Remove placeholder/junk entries that waste injection budget.
	junkDeleted, err := deleteJunkEntries(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: cleaning junk entries: %v\n", err)
	} else if junkDeleted > 0 {
		fmt.Printf("Cleaned %d junk entries (None., (pending), N/A, empty)\n", junkDeleted)
	}

	// Step 3: Verify.
	count, maxTS, err := verifyIndex(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: verifying index: %v\n", err)
		os.Exit(1)
	}

	if count == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: index has 0 entries after rebuild\n")
		os.Exit(1)
	}

	if maxTS.After(cutoffTime) {
		fmt.Fprintf(os.Stderr, "ERROR: index contains entries newer than cutoff: max=%s cutoff=%s\n",
			maxTS.Format(time.RFC3339), cutoffTime.Format(time.RFC3339))
		os.Exit(1)
	}

	fmt.Printf("Index built: %d entries, max_timestamp=%s, cutoff=%s ✓\n",
		count, maxTS.Format(time.RFC3339), cutoffTime.Format(time.RFC3339))
}

// filterOps copies batch files from srcDir into dstDir, dropping any batch
// that contains ops with timestamps after cutoff. Fails if a batch mixes
// pre- and post-cutoff ops (safety check).
func filterOps(srcDir, dstDir string, cutoff time.Time) (droppedBatches, droppedOps, keptEntries int, latest time.Time, err error) {
	if err = os.MkdirAll(dstDir, 0o755); err != nil {
		return
	}

	// Walk month directories (ops/2026-10/...)
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return
	}

	for _, monthEntry := range entries {
		if !monthEntry.IsDir() {
			continue
		}
		monthSrc := filepath.Join(srcDir, monthEntry.Name())
		monthDst := filepath.Join(dstDir, monthEntry.Name())

		batchFiles, err2 := os.ReadDir(monthSrc)
		if err2 != nil {
			err = err2
			return
		}

		for _, bf := range batchFiles {
			if filepath.Ext(bf.Name()) != ".json" {
				continue
			}

			data, err2 := os.ReadFile(filepath.Join(monthSrc, bf.Name()))
			if err2 != nil {
				err = err2
				return
			}

			var batch ops.OpBatch
			if err2 = json.Unmarshal(data, &batch); err2 != nil {
				err = fmt.Errorf("parsing %s: %w", bf.Name(), err2)
				return
			}

			// Check: are ALL ops before cutoff, ALL after, or mixed?
			before, after := 0, 0
			for _, op := range batch.Ops {
				if op.Timestamp.After(cutoff) {
					after++
				} else {
					before++
					if op.Timestamp.After(latest) {
						latest = op.Timestamp
					}
				}
			}

			if before > 0 && after > 0 {
				err = fmt.Errorf("batch %s has %d ops before cutoff and %d after — mixed batch",
					bf.Name(), before, after)
				return
			}

			if after > 0 {
				droppedBatches++
				droppedOps += after
				continue
			}

			// Keep this batch — copy to filtered dir.
			if err2 = os.MkdirAll(monthDst, 0o755); err2 != nil {
				err = err2
				return
			}
			if err2 = os.WriteFile(filepath.Join(monthDst, bf.Name()), data, 0o644); err2 != nil {
				err = err2
				return
			}
			keptEntries += len(batch.Ops)
		}
	}
	return
}

// verifyIndex opens the built index.db and returns the entry count and max timestamp.
func verifyIndex(dbPath string) (count int, maxTS time.Time, err error) {
	db, err := openSQLite(dbPath)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer db.Close()

	row := db.QueryRow("SELECT COUNT(*), COALESCE(MAX(created_at), '') FROM entries")
	var maxTSStr string
	if err = row.Scan(&count, &maxTSStr); err != nil {
		return
	}

	if maxTSStr != "" {
		// Try multiple time formats
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
			if t, e := time.Parse(layout, maxTSStr); e == nil {
				maxTS = t
				break
			}
		}
	}
	return
}

func openSQLite(path string) (*sqlDB, error) {
	// Use the same driver the index package uses
	idx, err := index.NewIndex(path)
	if err != nil {
		return nil, err
	}
	// We need raw SQL access — close the index wrapper and open directly
	idx.Close()

	// Re-open with database/sql directly
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return &sqlDB{db}, nil
}

type sqlDB struct {
	*sql.DB
}

// deleteJunkEntries removes placeholder and empty entries from the index
// that would waste injection budget. Returns the number of entries deleted.
func deleteJunkEntries(dbPath string) (int64, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	// Delete entries whose text is a known placeholder pattern
	result, err := db.Exec(`
		DELETE FROM entries
		WHERE TRIM(text) IN ('', 'None.', 'None', '(pending)', 'N/A')
		   OR TRIM(text) LIKE 'None encountered%'
	`)
	if err != nil {
		return 0, err
	}
	deleted, _ := result.RowsAffected()

	// Also clean up FTS entries for deleted rows
	if deleted > 0 {
		db.Exec(`DELETE FROM entries_fts WHERE rowid NOT IN (SELECT rowid FROM entries)`)
	}

	return deleted, nil
}
