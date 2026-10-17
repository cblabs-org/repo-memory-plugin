/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/reproject.go
 * Description: Multi-directory operation log reprojection, human-edit capture,
 *              loss-prevention validation for pre-existing entry IDs, verified
 *              stamps, and human decisions, and diff classification against on-disk dossiers.
 *
 * Exported Types / Functions:
 * - ErrHumanDecisionLost: Sentinel error returned when reprojecting would drop existing entries/decisions.
 * - LostEntryViolation:   Details of a pre-existing dossier entry missing from the projected op-log.
 * - ReprojectOptions:     Configuration parameters for an op-log reprojection run.
 * - ReprojectResult:      Summary counts, file diff lists, and loss violations from a reprojection run.
 * - Reproject():          Replays op-logs across directories, validates against data loss, and writes dossiers.
 */

package dossiers

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// ErrHumanDecisionLost is returned when reprojecting the op-log would lose an existing
// dossier entry ID, verified stamp, or human decision unless AllowLoss is set.
var ErrHumanDecisionLost = errors.New("reproject would lose existing entry ID, verified stamp, or human decision")

// LostEntryViolation describes an existing dossier entry that is missing from the projected op-log.
type LostEntryViolation struct {
	EntryID string `json:"entry_id"` // Identifier of the missing dossier entry.
	Scope   string `json:"scope"`    // Dossier scope where the entry existed on disk.
	Reason  string `json:"reason"`   // Explanation of why the loss was flagged.
}

// ReprojectOptions configures an op-log reprojection run.
type ReprojectOptions struct {
	MemoryDir string   // Target .memory directory (e.g. filepath.Join(repoRoot, ".memory")).
	OpsDirs   []string // Ordered op-log directories (e.g. [<repoRoot>/.memory/ops, <dataDir>/repos/<repoID>/ops-local, <dataDir>/user/ops]).
	DryRun    bool     // When true, computes diffs and loss checks without writing files.
	AllowLoss bool     // When true, proceeds even if pre-existing on-disk entries are absent from the op-log.
}

// ReprojectResult reports the outcome of an op-log reprojection run.
type ReprojectResult struct {
	DryRun                  bool                 `json:"dry_run"`                      // Whether the run was executed in dry-run mode.
	DossiersProjected       int                  `json:"dossiers_projected"`           // Count of non-empty dossiers produced by projection.
	ActiveEntries           int                  `json:"active_entries"`               // Total active entries across all projected dossiers.
	HistoryEntries          int                  `json:"history_entries"`              // Total retired/superseded entries across all projected dossiers.
	PreExistingEntries      int                  `json:"pre_existing_entries"`         // Count of entries found in on-disk dossiers prior to reprojection.
	VerifiedPreserved       int                  `json:"verified_preserved"`           // Count of on-disk verified stamps preserved in projected entries.
	HumanDecisionsPreserved int                  `json:"human_decisions_preserved"`    // Count of human decisions/invariants preserved in projected entries.
	HandEditsCaptured       int                  `json:"hand_edits_captured,omitempty"` // Count of manual Markdown edits captured into the op-log.
	FilesAdded              []string             `json:"files_added,omitempty"`        // Relative paths of newly created dossier or Atlas files.
	FilesUpdated            []string             `json:"files_updated,omitempty"`      // Relative paths of modified dossier or Atlas files.
	FilesUnchanged          []string             `json:"files_unchanged,omitempty"`    // Relative paths of unchanged dossier or Atlas files.
	FilesRemoved            []string             `json:"files_removed,omitempty"`      // Relative paths of orphaned dossier files removed on write.
	LostEntries             []LostEntryViolation `json:"lost_entries,omitempty"`       // Pre-existing entries missing from the projected op-log.
	QuestionsUpdated        bool                 `json:"questions_updated,omitempty"`  // Whether open questions state was updated during the run.
	SkillsGraduated         int                  `json:"skills_graduated,omitempty"`   // Number of procedural entries graduated to skills.
}

// preExistingDiskEntry captures metadata extracted from an existing on-disk
// dossier Markdown file prior to reprojection.
type preExistingDiskEntry struct {
	ID              string    // Entry identifier parsed from the H3 heading.
	Scope           string    // Dossier scope derived from the relative file path.
	Type            EntryType // Entry type parsed from metadata or inferred from ID prefix.
	Source          string    // Entry source parsed from metadata, if present.
	Verified        string    // Verified stamp parsed from metadata, if present.
	IsHistory       bool      // Whether the entry was parsed from a .history.md file.
	IsHumanDecision bool      // Whether the entry represents a human decision, invariant, or ruled-out item.
}

// Reproject replays the op-log across OpsDirs, verifies that no pre-existing entry IDs,
// verified stamps, or human decisions in MemoryDir/dossiers are lost, and (when !DryRun)
// writes the regenerated dossiers and ATLAS.md.
// It returns ErrHumanDecisionLost (discoverable via errors.Is) if any pre-existing entry is
// missing and opts.AllowLoss is false.
func Reproject(ctx context.Context, opts ReprojectOptions) (*ReprojectResult, map[string]*Dossier, error) {
	// --- Step 1: Normalize options and filter existing op-log directories ---
	if opts.MemoryDir == "" {
		opts.MemoryDir = ".memory"
	}
	if len(opts.OpsDirs) == 0 {
		opts.OpsDirs = []string{filepath.Join(opts.MemoryDir, "ops")}
	}

	var existingOpsDirs []string
	seenDirs := make(map[string]bool)
	for _, d := range opts.OpsDirs {
		if d == "" {
			continue
		}
		clean := filepath.Clean(d)
		if seenDirs[clean] {
			continue
		}
		seenDirs[clean] = true
		if info, err := os.Stat(clean); err == nil && info.IsDir() {
			existingOpsDirs = append(existingOpsDirs, clean)
		}
	}

	// --- Step 2: Project dossiers across all configured op-log directories ---
	dossiersMap, err := projectAcrossOpsDirs(ctx, existingOpsDirs)
	if err != nil {
		return nil, nil, err
	}

	res := &ReprojectResult{
		DryRun: opts.DryRun,
	}

	// Build index of projected entries (both active and history) across all scopes.
	projectedActive := make(map[string]*Entry)
	projectedHistory := make(map[string]*Entry)
	for _, d := range dossiersMap {
		for i := range d.Entries {
			e := &d.Entries[i]
			projectedActive[e.ID] = e
		}
		for i := range d.History {
			h := &d.History[i]
			projectedHistory[h.ID] = h
		}
	}

	// --- Step 3: Audit pre-existing on-disk dossiers for lost entries or verified stamps ---
	dossiersDir := filepath.Join(opts.MemoryDir, "dossiers")
	diskEntries, err := scanPreExistingDossierEntries(dossiersDir)
	if err != nil {
		return nil, nil, err
	}

	for _, de := range diskEntries {
		res.PreExistingEntries++
		projEntry, inActive := projectedActive[de.ID]
		if !inActive {
			projEntry = projectedHistory[de.ID]
		}

		if projEntry == nil {
			reason := "entry_id missing from projected ops"
			if de.IsHumanDecision {
				reason = "human decision missing from projected ops"
			}
			res.LostEntries = append(res.LostEntries, LostEntryViolation{
				EntryID: de.ID,
				Scope:   de.Scope,
				Reason:  reason,
			})
			continue
		}

		if de.IsHumanDecision || isHumanDecisionTypeOrSource(projEntry.Type, projEntry.Source) {
			res.HumanDecisionsPreserved++
		}
		if de.Verified != "" {
			if projEntry.Verified == "" {
				projEntry.Verified = de.Verified
			}
			res.VerifiedPreserved++
		}
	}

	// Compute summary counts.
	for _, d := range dossiersMap {
		if len(d.Entries) == 0 && len(d.History) == 0 {
			continue
		}
		res.DossiersProjected++
		res.ActiveEntries += len(d.Entries)
		res.HistoryEntries += len(d.History)
	}

	if len(res.LostEntries) > 0 && !opts.AllowLoss {
		return res, dossiersMap, fmt.Errorf("%w: %d entries/decisions missing", ErrHumanDecisionLost, len(res.LostEntries))
	}

	// --- Step 4: Detect and capture human hand-edits in on-disk dossier files ---
	var handOps []ops.Op
	var sortedScopes []string
	for s := range dossiersMap {
		sortedScopes = append(sortedScopes, s)
	}
	sort.Strings(sortedScopes)

	for _, scopeKey := range sortedScopes {
		d := dossiersMap[scopeKey]
		if d == nil || len(d.Entries) == 0 {
			continue
		}
		scopeName := d.Scope
		if scopeName == "" {
			scopeName = scopeKey
		}
		diskPath := filepath.Join(dossiersDir, filepath.FromSlash(scopeName)+".md")
		existingBytes, err := os.ReadFile(diskPath)
		if err != nil {
			continue
		}
		rendered := RenderDossier(d)
		detected, err := DetectHandEdits(rendered, string(existingBytes), scopeName)
		if err != nil || len(detected) == 0 {
			continue
		}
		for _, op := range detected {
			if op.Type == ops.OpAdd && opts.AllowLoss {
				continue
			}
			handOps = append(handOps, op)
		}
	}

	if len(handOps) > 0 {
		res.HandEditsCaptured = len(handOps)
		entryIndex := make(map[string]string)
		for sKey, d := range dossiersMap {
			if d == nil {
				continue
			}
			for _, e := range d.Entries {
				entryIndex[e.ID] = sKey
			}
		}
		if err := applyBatch(dossiersMap, entryIndex, &ops.OpBatch{Ops: handOps}); err != nil {
			return res, dossiersMap, err
		}
		for _, d := range dossiersMap {
			if d != nil {
				sortDossier(d)
			}
		}
		if !opts.DryRun && len(opts.OpsDirs) > 0 && strings.TrimSpace(opts.OpsDirs[0]) != "" {
			batch := ops.NewBatch(handOps)
			if err := ops.NewWriter(opts.OpsDirs[0]).Write(batch); err != nil {
				return res, dossiersMap, err
			}
		}
	}

	// --- Step 5: Classify file diffs and write regenerated dossiers when not in DryRun ---
	expectedFiles := buildExpectedRenderedFiles(dossiersMap)
	classifyRenderedFileDiff(opts.MemoryDir, expectedFiles, res)

	if !opts.DryRun {
		writer := NewWriter(opts.MemoryDir)
		if err := writer.WriteAll(dossiersMap); err != nil {
			return res, dossiersMap, err
		}
	}

	return res, dossiersMap, nil
}

// projectAcrossOpsDirs merges and chronologically sorts operation batches across
// one or more op-log directories and projects them into a Dossier map.
func projectAcrossOpsDirs(ctx context.Context, dirs []string) (map[string]*Dossier, error) {
	// --- Step 1: Handle zero or single directory fast paths ---
	if len(dirs) == 0 {
		return make(map[string]*Dossier), nil
	}
	if len(dirs) == 1 {
		return NewProjector(ops.NewReader(dirs[0])).Project(ctx)
	}

	// --- Step 2: Collect and deduplicate batches across all directories ---
	var allBatches []*ops.OpBatch
	seenBatchID := make(map[string]bool)
	for _, dir := range dirs {
		reader := ops.NewReader(dir)
		for batch, err := range reader.Iterate(ctx) {
			if err != nil {
				return nil, err
			}
			if batch == nil {
				continue
			}
			if batch.BatchID != "" && seenBatchID[batch.BatchID] {
				continue
			}
			if batch.BatchID != "" {
				seenBatchID[batch.BatchID] = true
			}
			allBatches = append(allBatches, batch)
		}
	}

	// --- Step 3: Sort batches chronologically (breaking ties by BatchID) and project ---
	sort.SliceStable(allBatches, func(i, j int) bool {
		if !allBatches[i].CreatedAt.Equal(allBatches[j].CreatedAt) {
			return allBatches[i].CreatedAt.Before(allBatches[j].CreatedAt)
		}
		return allBatches[i].BatchID < allBatches[j].BatchID
	})

	seq := func(yield func(*ops.OpBatch, error) bool) {
		for _, b := range allBatches {
			if err := ctx.Err(); err != nil {
				yield(nil, err)
				return
			}
			if !yield(b, nil) {
				return
			}
		}
	}
	return NewProjector(nil).projectFromSeq(iter.Seq2[*ops.OpBatch, error](seq))
}

// scanPreExistingDossierEntries walks dossiersDir and extracts all entry IDs and
// verification/human-decision metadata from existing .md and .history.md files.
func scanPreExistingDossierEntries(dossiersDir string) ([]preExistingDiskEntry, error) {
	// --- Step 1: Verify dossiersDir exists and collect all Markdown files ---
	info, err := os.Stat(dossiersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var mdFiles []string
	err = filepath.WalkDir(dossiersDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(mdFiles)

	// --- Step 2: Parse entries from each Markdown file, preferring active over history ---
	byID := make(map[string]preExistingDiskEntry)
	for _, fullPath := range mdFiles {
		rel, err := filepath.Rel(dossiersDir, fullPath)
		if err != nil {
			continue
		}
		relSlash := filepath.ToSlash(rel)
		isHistory := strings.HasSuffix(relSlash, ".history.md")
		scope := strings.TrimSuffix(relSlash, ".md")
		if isHistory {
			scope = strings.TrimSuffix(relSlash, ".history.md")
		}

		rawBytes, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, err
		}
		parsed := parseDiskMarkdownEntries(string(rawBytes), scope, isHistory)
		for _, e := range parsed {
			existing, ok := byID[e.ID]
			if !ok || (existing.IsHistory && !e.IsHistory) {
				byID[e.ID] = e
			}
		}
	}

	// --- Step 3: Return deduplicated entries sorted by ID ---
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]preExistingDiskEntry, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out, nil
}

// parseDiskMarkdownEntries parses H3 entry headers and their backtick metadata
// lines from a single dossier Markdown file.
func parseDiskMarkdownEntries(content, scope string, isHistory bool) []preExistingDiskEntry {
	// --- Step 1: Parse base entry map and scan lines for H3 headers and metadata ---
	renderedMap := parseMarkdownEntries(content)
	lines := strings.Split(content, "\n")

	metaByID := make(map[string]struct {
		entryType EntryType // Parsed entry type token.
		source    string    // Parsed source field.
		verified  string    // Parsed verified stamp.
	})

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "### [") {
			continue
		}
		closeIdx := strings.Index(line, "]")
		if closeIdx <= 5 {
			continue
		}
		id := strings.TrimSpace(line[5:closeIdx])
		if id == "" {
			continue
		}
		// Ensure even IDs with underscores or special chars are captured in renderedMap
		if _, ok := renderedMap[id]; !ok {
			renderedMap[id] = parsedEntry{id: id}
		}

		// Inspect the immediately following non-empty line for `type · ...` metadata.
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" {
				continue
			}
			if strings.HasPrefix(next, "### [") || strings.HasPrefix(next, "## ") {
				break
			}
			if strings.HasPrefix(next, "`") && strings.HasSuffix(next, "`") && len(next) >= 2 {
				metaLine := strings.TrimSuffix(strings.TrimPrefix(next, "`"), "`")
				parts := strings.Split(metaLine, "·")
				var eType EntryType
				var src, ver string
				for idx, rawPart := range parts {
					part := strings.TrimSpace(rawPart)
					if idx == 0 && part != "" {
						eType = EntryType(part)
						continue
					}
					if strings.HasPrefix(part, "verified ") {
						ver = strings.TrimSpace(strings.TrimPrefix(part, "verified "))
						if parenIdx := strings.Index(ver, " ("); parenIdx > 0 {
							ver = strings.TrimSpace(ver[:parenIdx])
						}
					} else if strings.HasPrefix(part, "source:") {
						src = strings.TrimSpace(strings.TrimPrefix(part, "source:"))
					}
				}
				metaByID[id] = struct {
					entryType EntryType
					source    string
					verified  string
				}{
					entryType: eType,
					source:    src,
					verified:  ver,
				}
			}
			break
		}
	}

	// --- Step 2: Assemble preExistingDiskEntry records with inferred fallback types ---
	var result []preExistingDiskEntry
	for id := range renderedMap {
		meta := metaByID[id]
		if meta.entryType == "" {
			meta.entryType = InferEntryTypeFromID(id)
		}
		isHuman := isHumanDecisionTypeOrSource(meta.entryType, meta.source)
		result = append(result, preExistingDiskEntry{
			ID:              id,
			Scope:           scope,
			Type:            meta.entryType,
			Source:          meta.source,
			Verified:        meta.verified,
			IsHistory:       isHistory,
			IsHumanDecision: isHuman,
		})
	}
	return result
}

// isHumanDecisionTypeOrSource reports whether an entry's type or source marks it
// as a human decision, invariant, or ruled-out approach protected against loss.
func isHumanDecisionTypeOrSource(entryType EntryType, source string) bool {
	t := strings.ToLower(strings.TrimSpace(string(entryType)))
	switch t {
	case string(EntryTypeDecision), string(EntryTypeInvariant), string(EntryTypeRuledOut), "ruled_out":
		return true
	}
	s := strings.ToLower(strings.TrimSpace(source))
	return s == "human" || s == "hand-edit" || strings.HasPrefix(s, "human") || strings.HasPrefix(s, "hand-edit")
}

// buildExpectedRenderedFiles renders all non-empty active/history dossiers and
// ATLAS.md in memory, returning a map of relative path to expected Markdown content.
func buildExpectedRenderedFiles(dossiersMap map[string]*Dossier) map[string]string {
	// --- Step 1: Sort dossier scopes for deterministic iteration ---
	expected := make(map[string]string)
	atlas := &Atlas{}

	var scopes []string
	for s := range dossiersMap {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)

	// --- Step 2: Render active and history Markdown for each non-empty dossier ---
	for _, scope := range scopes {
		d := dossiersMap[scope]
		if len(d.Entries) == 0 && len(d.History) == 0 {
			continue
		}
		if len(d.History) > 0 {
			relHist := filepath.ToSlash(filepath.Join("dossiers", filepath.FromSlash(d.Scope)+".history.md"))
			expected[relHist] = RenderHistory(d)
		}
		if len(d.Entries) == 0 {
			continue
		}
		relActive := filepath.ToSlash(filepath.Join("dossiers", filepath.FromSlash(d.Scope)+".md"))
		expected[relActive] = RenderDossier(d)

		var lastUpdated time.Time
		for _, e := range d.Entries {
			if e.UpdatedAt.After(lastUpdated) {
				lastUpdated = e.UpdatedAt
			}
		}
		atlas.Dossiers = append(atlas.Dossiers, AtlasEntry{
			Scope:       d.Scope,
			Kind:        d.Kind,
			Title:       d.Title,
			EntryCount:  len(d.Entries),
			LastUpdated: lastUpdated,
		})
	}

	// --- Step 3: Render ATLAS.md ---
	expected["ATLAS.md"] = RenderAtlas(atlas)
	return expected
}

// classifyRenderedFileDiff compares expected rendered files against existing
// files under memoryDir and populates FilesAdded, FilesUpdated, FilesUnchanged,
// and FilesRemoved on res.
func classifyRenderedFileDiff(memoryDir string, expected map[string]string, res *ReprojectResult) {
	// --- Step 1: Load existing ATLAS.md and dossier Markdown files from disk ---
	existingOnDisk := make(map[string]string)

	atlasPath := filepath.Join(memoryDir, "ATLAS.md")
	if b, err := os.ReadFile(atlasPath); err == nil {
		existingOnDisk["ATLAS.md"] = string(b)
	}

	dossiersDir := filepath.Join(memoryDir, "dossiers")
	if info, err := os.Stat(dossiersDir); err == nil && info.IsDir() {
		_ = filepath.WalkDir(dossiersDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			rel, err := filepath.Rel(memoryDir, path)
			if err != nil {
				return nil
			}
			if b, err := os.ReadFile(path); err == nil {
				existingOnDisk[filepath.ToSlash(rel)] = string(b)
			}
			return nil
		})
	}

	// --- Step 2: Classify expected files as added, unchanged, or updated ---
	var keys []string
	for k := range expected {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, rel := range keys {
		wantContent := expected[rel]
		gotContent, exists := existingOnDisk[rel]
		if !exists {
			res.FilesAdded = append(res.FilesAdded, rel)
		} else if gotContent == wantContent {
			res.FilesUnchanged = append(res.FilesUnchanged, rel)
		} else {
			res.FilesUpdated = append(res.FilesUpdated, rel)
		}
	}

	// --- Step 3: Identify orphaned on-disk files slated for removal ---
	for rel := range existingOnDisk {
		if _, ok := expected[rel]; !ok {
			res.FilesRemoved = append(res.FilesRemoved, rel)
		}
	}
	sort.Strings(res.FilesRemoved)
}
