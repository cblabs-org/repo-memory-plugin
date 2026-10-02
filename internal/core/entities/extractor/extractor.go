package extractor

import (
	"io/fs"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// Extractor is the main No-LLM entity extractor.
type Extractor struct {
	root string
}

func New(root string) *Extractor {
	return &Extractor{root: root}
}

// Run executes the extraction process over the repository.
func (e *Extractor) Run() ([]entities.Entity, error) {
	var all []entities.Entity

	// 1. Extract from config.yaml if exists
	ruleEntities, err := ExtractFromRules(filepath.Join(e.root, "config.yaml"))
	if err == nil && ruleEntities != nil {
		all = append(all, ruleEntities...)
	}

	// 2. Walk directory
	err = Walk(e.root, func(path string, d fs.DirEntry) error {
		if d.IsDir() {
			return nil
		}

		// Config entities
		cfgEntities, err := ParseConfig(path)
		if err == nil && cfgEntities != nil {
			all = append(all, cfgEntities...)
		}

		// Symbol entities
		symEntities, err := ExtractSymbols(path)
		if err == nil && symEntities != nil {
			all = append(all, symEntities...)
		}

		// We could also emit file entities themselves
		rel, _ := filepath.Rel(e.root, path)
		all = append(all, entities.Entity{
			ID:   "file:" + rel,
			Kind: entities.KindFile,
			Name: rel,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return all, nil
}
