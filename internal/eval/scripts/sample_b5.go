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

func main() {
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

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(headers), func(i, j int) {
		headers[i], headers[j] = headers[j], headers[i]
	})

	sampleSize := 20
	if len(headers) < sampleSize {
		sampleSize = len(headers)
	}

	sample := headers[:sampleSize]

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
