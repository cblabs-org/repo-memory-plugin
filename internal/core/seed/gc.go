package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// DefaultStaleEntryIDs lists known-false seeded entry IDs that should always
// be pruned during ops garbage collection.
var DefaultStaleEntryIDs = []string{
	"seed-b1617c95",
}

// GCOptions configures an ops garbage collection and dossier re-projection run.
type GCOptions struct {
	OpsDir      string
	DossiersDir string
	ConfigPath  string
	PruneIDs    []string
	DryRun      bool
}

// GCStats summarizes the results of a RunOpsGC execution.
type GCStats struct {
	BatchesScanned  int
	BatchesModified int
	BatchesDeleted  int
	OpsRemoved      int
	AnchorsRemoved  int
	ScopesRepaired  int
}

type trackedBatch struct {
	path     string
	batch    ops.OpBatch
	modified bool
}

func isPollutedScope(scope string) bool {
	s := strings.TrimSpace(scope)
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "~") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "pr-") {
		return true
	}
	if strings.ContainsAny(s, "<>~*?") || strings.Contains(s, "..") {
		return true
	}
	switch s {
	case "modernc.org", ".agents", "pending", "Makefile", "Dockerfile", "LICENSE",
		"bin", "internal", "memory", "repo-memory", "rules", "runs":
		return true
	}
	firstSeg := s
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		firstSeg = s[:idx]
	}
	if strings.Contains(firstSeg, ".") || strings.Contains(s, ".") || strings.HasPrefix(s, "-") || strings.Contains(s, "/-") {
		return true
	}
	return false
}

func extractOpAnchors(fields map[string]any) ([]string, bool) {
	if fields == nil {
		return nil, false
	}
	raw, exists := fields["anchors"]
	if !exists || raw == nil {
		return nil, false
	}
	switch v := raw.(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

// RunOpsGC scans all batch JSON files under opts.OpsDir, prunes placeholder/junk ops
// and stale entry IDs, strips syntactically invalid anchor tokens, repairs polluted
// dossier scopes via config.ScopeResolver, writes back or deletes empty batches,
// and optionally re-projects dossiers and ATLAS.md when opts.DossiersDir is set.
func RunOpsGC(ctx context.Context, opts GCOptions) (*GCStats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(opts.OpsDir) == "" {
		return nil, errors.New("ops directory is required")
	}

	pruneSet := make(map[string]bool)
	for _, id := range DefaultStaleEntryIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			pruneSet[trimmed] = true
		}
	}
	for _, id := range opts.PruneIDs {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			pruneSet[trimmed] = true
		}
	}

	var batchPaths []string
	err := filepath.WalkDir(opts.OpsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".json" {
			batchPaths = append(batchPaths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking ops dir %s: %w", opts.OpsDir, err)
	}
	sort.Strings(batchPaths)

	stats := &GCStats{}
	batches := make([]*trackedBatch, 0, len(batchPaths))
	var allValidAnchors []string

	// Pass 1: Prune junk/stale ops and sanitize anchors.
	for _, p := range batchPaths {
		stats.BatchesScanned++
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading batch %s: %w", p, err)
		}

		var batch ops.OpBatch
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, fmt.Errorf("unmarshaling batch %s: %w", p, err)
		}

		tb := &trackedBatch{
			path:  p,
			batch: batch,
		}

		keptOps := make([]ops.Op, 0, len(batch.Ops))
		for _, op := range batch.Ops {
			if pruneSet[strings.TrimSpace(op.EntryID)] {
				stats.OpsRemoved++
				tb.modified = true
				continue
			}

			if op.Fields != nil {
				if text, ok := op.Fields["text"].(string); ok && IsJunkText(text) {
					stats.OpsRemoved++
					tb.modified = true
					continue
				}
			}

			if rawAnchors, hasAnchors := extractOpAnchors(op.Fields); hasAnchors {
				cleanAnchors := make([]string, 0, len(rawAnchors))
				for _, a := range rawAnchors {
					if !IsValidAnchorPathSyntax(a) && !isValidDocDirAnchor(a) {
						stats.AnchorsRemoved++
						tb.modified = true
						continue
					}
					trimmed := strings.TrimSuffix(strings.TrimSpace(a), "/")
					if trimmed != a {
						tb.modified = true
					}
					cleanAnchors = append(cleanAnchors, trimmed)
					if strings.Contains(trimmed, "/") && !strings.HasPrefix(trimmed, ".") {
						allValidAnchors = append(allValidAnchors, trimmed)
					}
				}
				op.Fields["anchors"] = cleanAnchors
			}

			keptOps = append(keptOps, op)
		}

		tb.batch.Ops = keptOps
		batches = append(batches, tb)
	}

	// Pass 2: Repair polluted dossier scopes using ScopeResolver.
	scopeCfg, err := config.LoadScopeConfig(opts.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading scope config: %w", err)
	}
	moduleScopes := config.DeriveModuleScopes(expandSubmoduleAnchorsForScopes(allValidAnchors))
	resolver := config.NewScopeResolver(scopeCfg, moduleScopes, nil)

	for _, tb := range batches {
		for i := range tb.batch.Ops {
			op := &tb.batch.Ops[i]
			if !isPollutedScope(op.DossierScope) {
				continue
			}

			rawAnchors, _ := extractOpAnchors(op.Fields)
			var scopeAnchors []string
			for _, a := range rawAnchors {
				if strings.Contains(a, "/") && !strings.HasPrefix(a, ".") {
					scopeAnchors = append(scopeAnchors, a)
				}
			}

			resolved := resolver.ResolveScope(scopeAnchors, nil)
			if resolved == "" || isPollutedScope(resolved) {
				resolved = "root"
			}

			if op.DossierScope != resolved {
				op.DossierScope = resolved
				stats.ScopesRepaired++
				tb.modified = true
			}
		}
	}

	// Persist batch updates or delete empty batch files.
	for _, tb := range batches {
		if len(tb.batch.Ops) == 0 {
			stats.BatchesDeleted++
			if !opts.DryRun {
				if err := os.Remove(tb.path); err != nil && !os.IsNotExist(err) {
					return nil, fmt.Errorf("removing empty batch %s: %w", tb.path, err)
				}
			}
			continue
		}

		if tb.modified {
			stats.BatchesModified++
			if !opts.DryRun {
				encoded, err := json.MarshalIndent(tb.batch, "", "  ")
				if err != nil {
					return nil, fmt.Errorf("marshaling batch %s: %w", tb.path, err)
				}
				encoded = append(encoded, '\n')
				if err := os.WriteFile(tb.path, encoded, 0o644); err != nil {
					return nil, fmt.Errorf("writing batch %s: %w", tb.path, err)
				}
			}
		}
	}

	// Re-render dossiers and ATLAS.md when DossiersDir is configured.
	if strings.TrimSpace(opts.DossiersDir) != "" && !opts.DryRun {
		dossiersSubdir := filepath.Join(opts.DossiersDir, "dossiers")
		if err := os.RemoveAll(dossiersSubdir); err != nil {
			return nil, fmt.Errorf("removing stale dossiers dir %s: %w", dossiersSubdir, err)
		}
		atlasPath := filepath.Join(opts.DossiersDir, "ATLAS.md")
		if err := os.Remove(atlasPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("removing stale ATLAS.md %s: %w", atlasPath, err)
		}

		reader := ops.NewReader(opts.OpsDir)
		projector := dossiers.NewProjector(reader)
		dossierMap, err := projector.Project(ctx)
		if err != nil {
			return nil, fmt.Errorf("projecting dossiers from %s: %w", opts.OpsDir, err)
		}

		writer := dossiers.NewWriter(opts.DossiersDir)
		if err := writer.WriteAll(dossierMap); err != nil {
			return nil, fmt.Errorf("writing dossiers to %s: %w", opts.DossiersDir, err)
		}
	}

	return stats, nil
}
