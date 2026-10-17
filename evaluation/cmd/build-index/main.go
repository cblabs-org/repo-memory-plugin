/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: evaluation/cmd/build-index/main.go
 * Description: Standalone CLI tool that builds an evaluation SQLite index.db from a
 *              snapshot operation-log directory, dropping batches whose operations
 *              occurred after a configured RFC3339 timestamp cutoff.
 *
 * Exported Types / Functions:
 * - None (package main binary entrypoint).
 */

// The build-index command builds a standalone index.db from a snapshot ops directory,
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

// main parses CLI flags, filters operation batches up to the cutoff timestamp,
// rebuilds the SQLite index from the surviving batches, and verifies the result.
func main() {
	// --- Step 1: Parse and validate command-line flags ---
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

	// --- Step 2: Filter ops and copy only batches where all ops precede the cutoff ---
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

	// --- Step 3: Build SQLite index from the cutoff-filtered ops ---
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

	// --- Step 4: Verify built index entry count and maximum timestamp ---
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
// that contains ops with timestamps after cutoff. It returns an error if a
// batch mixes pre- and post-cutoff ops.
func filterOps(srcDir, dstDir string, cutoff time.Time) (droppedBatches, droppedOps, keptEntries int, latest time.Time, err error) {
	// --- Step 1: Ensure destination directory exists and list month subdirectories ---
	if err = os.MkdirAll(dstDir, 0o755); err != nil {
		return
	}

	// Walk month directories (ops/2026-10/...)
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return
	}

	// --- Step 2: Inspect each batch file in each month directory ---
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

			// --- Step 3: Classify batch ops relative to cutoff and reject mixed batches ---
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

			// --- Step 4: Copy surviving pre-cutoff batch to the filtered destination ---
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
	// --- Step 1: Open the SQLite database for direct inspection ---
	db, err := openSQLite(dbPath)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer db.Close()

	// --- Step 2: Query entry count and maximum creation timestamp ---
	row := db.QueryRow("SELECT COUNT(*), COALESCE(MAX(created_at), '') FROM entries")
	var maxTSStr string
	if err = row.Scan(&count, &maxTSStr); err != nil {
		return
	}

	// --- Step 3: Parse maximum timestamp across supported SQLite date layouts ---
	if maxTSStr != "" {
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
			if t, e := time.Parse(layout, maxTSStr); e == nil {
				maxTS = t
				break
			}
		}
	}
	return
}

// openSQLite initializes the SQLite schema via index.NewIndex and re-opens a raw
// database/sql handle for verification queries.
func openSQLite(path string) (*sqlDB, error) {
	// --- Step 1: Ensure schema is initialized via the index package ---
	idx, err := index.NewIndex(path)
	if err != nil {
		return nil, err
	}
	idx.Close()

	// --- Step 2: Open raw database/sql handle ---
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	return &sqlDB{db}, nil
}

// sqlDB wraps a standard sql.DB handle for verification queries.
type sqlDB struct {
	*sql.DB // Underlying SQLite database connection.
}
