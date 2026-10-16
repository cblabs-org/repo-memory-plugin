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
	EntryID string `json:"entry_id"`
	Scope   string `json:"scope"`
	Reason  string `json:"reason"`
}

// ReprojectOptions configures an op-log reprojection run.
type ReprojectOptions struct {
	MemoryDir string   // e.g. filepath.Join(repoRoot, ".memory")
	OpsDirs   []string // e.g. [<repoRoot>/.memory/ops, <dataDir>/repos/<repoID>/ops-local, <dataDir>/user/ops]
	DryRun    bool
	AllowLoss bool
}

// ReprojectResult reports the outcome of an op-log reprojection run.
type ReprojectResult struct {
	DryRun                  bool                 `json:"dry_run"`
	DossiersProjected       int                  `json:"dossiers_projected"`
	ActiveEntries           int                  `json:"active_entries"`
	HistoryEntries          int                  `json:"history_entries"`
	PreExistingEntries      int                  `json:"pre_existing_entries"`
	VerifiedPreserved       int                  `json:"verified_preserved"`
	HumanDecisionsPreserved int                  `json:"human_decisions_preserved"`
	HandEditsCaptured       int                  `json:"hand_edits_captured,omitempty"`
	FilesAdded              []string             `json:"files_added,omitempty"`
	FilesUpdated            []string             `json:"files_updated,omitempty"`
	FilesUnchanged          []string             `json:"files_unchanged,omitempty"`
	FilesRemoved            []string             `json:"files_removed,omitempty"`
	LostEntries             []LostEntryViolation `json:"lost_entries,omitempty"`
	QuestionsUpdated        bool                 `json:"questions_updated,omitempty"`
	SkillsGraduated         int                  `json:"skills_graduated,omitempty"`
}

type preExistingDiskEntry struct {
	ID              string
	Scope           string
	Type            EntryType
	Source          string
	Verified        string
	IsHistory       bool
	IsHumanDecision bool
}

// Reproject replays the op-log across OpsDirs, verifies that no pre-existing entry IDs,
// verified stamps, or human decisions in MemoryDir/dossiers are lost, and (when !DryRun)
// writes the regenerated dossiers and ATLAS.md.
func Reproject(ctx context.Context, opts ReprojectOptions) (*ReprojectResult, map[string]*Dossier, error) {
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

	// Scan pre-existing dossier files on disk for entry IDs, verified stamps, and human decisions.
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

	// Detect and capture any human prose edits (OpAmend / OpAdd) on existing on-disk dossier files before re-rendering.
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

	// Compute expected rendered files under opts.MemoryDir and compare against disk.
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

func projectAcrossOpsDirs(ctx context.Context, dirs []string) (map[string]*Dossier, error) {
	if len(dirs) == 0 {
		return make(map[string]*Dossier), nil
	}
	if len(dirs) == 1 {
		return NewProjector(ops.NewReader(dirs[0])).Project(ctx)
	}

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

func scanPreExistingDossierEntries(dossiersDir string) ([]preExistingDiskEntry, error) {
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

func parseDiskMarkdownEntries(content, scope string, isHistory bool) []preExistingDiskEntry {
	renderedMap := parseMarkdownEntries(content)
	lines := strings.Split(content, "\n")

	metaByID := make(map[string]struct {
		entryType EntryType
		source    string
		verified  string
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

func isHumanDecisionTypeOrSource(entryType EntryType, source string) bool {
	t := strings.ToLower(strings.TrimSpace(string(entryType)))
	switch t {
	case string(EntryTypeDecision), string(EntryTypeInvariant), string(EntryTypeRuledOut), "ruled_out":
		return true
	}
	s := strings.ToLower(strings.TrimSpace(source))
	return s == "human" || s == "hand-edit" || strings.HasPrefix(s, "human") || strings.HasPrefix(s, "hand-edit")
}

func buildExpectedRenderedFiles(dossiersMap map[string]*Dossier) map[string]string {
	expected := make(map[string]string)
	atlas := &Atlas{}

	var scopes []string
	for s := range dossiersMap {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)

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

	expected["ATLAS.md"] = RenderAtlas(atlas)
	return expected
}

func classifyRenderedFileDiff(memoryDir string, expected map[string]string, res *ReprojectResult) {
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

	for rel := range existingOnDisk {
		if _, ok := expected[rel]; !ok {
			res.FilesRemoved = append(res.FilesRemoved, rel)
		}
	}
	sort.Strings(res.FilesRemoved)
}
