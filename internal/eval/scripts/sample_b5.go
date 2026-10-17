/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/eval/scripts/sample_b5.go
 * Description: Standalone utility script that samples up to 20 episode headers
 *              from the local episode store (or synthetic fallback headers) and
 *              writes a CSV labeling sheet for benchmark B5 human evaluation.
 *
 * Exported Types / Functions:
 * - None (package main binary entrypoint).
 */

// The sample-b5 command samples episode headers and generates a CSV labeling sheet
// for the B5 human evaluation benchmark.
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
)

// main loads episode headers, randomly samples up to 20 episodes, and writes
// b5_labeling_sheet.csv in the current working directory.
func main() {
	// --- Step 1: Load episode headers from the local episode store ---
	fmt.Println("Generating B5 Labeling Sheet...")

	dataDir := config.DataDir()
	store := episode.NewStore(dataDir, "cblabs-org/repo-memory-plugin")

	headers, err := store.ListHeaders()
	if err != nil {
		log.Fatalf("Failed to list episodes: %v", err)
	}

	if len(headers) == 0 {
		fmt.Println("No episodes found, generating dummy data for sheet.")
		headers = generateDummyHeaders()
	}

	// --- Step 2: Randomly shuffle and select up to 20 episodes ---
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(headers), func(i, j int) {
		headers[i], headers[j] = headers[j], headers[i]
	})

	sampleSize := 20
	if len(headers) < sampleSize {
		sampleSize = len(headers)
	}

	sample := headers[:sampleSize]

	// --- Step 3: Write the sampled episodes to b5_labeling_sheet.csv ---
	file, err := os.Create("b5_labeling_sheet.csv")
	if err != nil {
		log.Fatalf("Failed to create csv file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headerRow := []string{"Episode ID", "Session ID", "Date", "Correct", "Wrong", "Not Durable", "Missed Knowledge", "Notes"}
	if err := writer.Write(headerRow); err != nil {
		log.Fatalf("Failed to write header row: %v", err)
	}

	for _, h := range sample {
		row := []string{
			h.EpisodeID,
			h.SessionID,
			h.Started.Format(time.RFC3339),
			"", // Correct
			"", // Wrong
			"", // Not Durable
			"", // Missed Knowledge
			"", // Notes
		}
		if err := writer.Write(row); err != nil {
			log.Fatalf("Failed to write row: %v", err)
		}
	}

	fmt.Printf("Successfully generated b5_labeling_sheet.csv with %d episodes.\n", len(sample))
}

// generateDummyHeaders synthesizes 25 placeholder episode headers when no stored episodes exist.
func generateDummyHeaders() []*episode.Header {
	var headers []*episode.Header
	for i := 0; i < 25; i++ {
		headers = append(headers, &episode.Header{
			EpisodeID: fmt.Sprintf("ep-dummy-%d", i),
			SessionID: fmt.Sprintf("session-%d", i),
			Started:   time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}
	return headers
}
