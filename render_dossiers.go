package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func main() {
	entries, err := os.ReadDir("evaluation/data")
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		woName := entry.Name()
		opsDir := filepath.Join("evaluation/data", woName, "ops")
		outDir := filepath.Join("evaluation/data", woName)

		fmt.Printf("Rendering dossiers for %s...\n", woName)
		reader := ops.NewReader(opsDir)
		
		projector := dossiers.NewProjector(reader)
		dossierMap, err := projector.Project(context.Background())
		if err != nil {
			fmt.Printf("Error projecting %s: %v\n", woName, err)
			continue
		}

		writer := dossiers.NewWriter(outDir)
		if err := writer.WriteAll(dossierMap); err != nil {
			fmt.Printf("Error writing %s: %v\n", woName, err)
		}
	}
}
