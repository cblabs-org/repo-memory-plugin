/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/delivery/workorder_brief.go
 * Description: Constructs deterministic, token-bounded repository memory briefs
 *              for Principal Architect workorders by extracting candidate file
 *              paths from WORKORDER.md, resolving module scopes, ranking active
 *              dossier entries, summarizing unconsolidated notes, and rendering
 *              bounded Markdown output.
 *
 * Exported Types / Functions:
 * - DefaultBriefMaxTokens:   Default token ceiling for Principal Architect workorder briefs.
 * - WorkorderBrief:          Deterministic, token-bounded memory brief for a workorder.
 * - BriefNoteSummary:        Compact summary of an unconsolidated note relevant to a workorder brief.
 * - ExtractWorkorderPaths(): Deterministically extracts repository-relative paths from WORKORDER.md content.
 * - BuildWorkorderBrief():   Constructs a token-bounded WorkorderBrief from a WORKORDER.md file and/or scopes.
 */

package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
)

// DefaultBriefMaxTokens is the default token ceiling for PA workorder briefs.
const DefaultBriefMaxTokens = 600

// WorkorderBrief is a deterministic, token-bounded memory brief for a Principal Architect workorder.
type WorkorderBrief struct {
	WorkorderPath       string             `json:"workorder_path,omitempty"` // Path to the source WORKORDER.md file, if provided.
	ExtractedFiles      []string           `json:"extracted_files"`          // Repository-relative file and directory paths extracted from the workorder.
	ResolvedScopes      []string           `json:"resolved_scopes"`          // Dossier scopes resolved from explicit scopes and extracted paths.
	StatusLine          string             `json:"status_line"`              // Single-line summary of index freshness, unconsolidated notes, and suspect entries.
	CoverageGaps        []string           `json:"coverage_gaps"`            // Warnings for primary scopes lacking dossiers or having suspect entries.
	Entries             []RankedEntry      `json:"entries"`                  // Ranked dossier entries selected within the token budget.
	UnconsolidatedNotes []BriefNoteSummary `json:"unconsolidated_notes"`     // Unconsolidated notes matching the resolved scopes within the token budget.
	TokenCount          int                `json:"token_count"`              // Estimated token count of BriefMarkdown.
	BriefMarkdown       string             `json:"brief_markdown"`           // Rendered Markdown representation of the workorder brief.
}

// BriefNoteSummary is a compact representation of an unconsolidated note relevant to a workorder brief.
type BriefNoteSummary struct {
	ID        string `json:"id"`                   // Unique note identifier or filename stem.
	Kind      string `json:"kind"`                 // Cognitive kind or note type label.
	Scope     string `json:"scope"`                // Target module dossier scope for the note.
	Statement string `json:"statement"`            // Single-line summary statement of the note.
	CreatedAt string `json:"created_at,omitempty"` // Optional ISO-8601 creation timestamp string.
}

var (
	// backtickRegex matches inline code spans (`...`) on a single line.
	backtickRegex = regexp.MustCompile("`([^`\\n]+)`")
	// mdLinkRegex matches Markdown link targets ([label](target)).
	mdLinkRegex = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)\)`)
)

// ExtractWorkorderPaths deterministically extracts repository-relative file and directory paths
// from WORKORDER.md markdown content, prioritizing Permitted Mutable Files and Forbidden Files sections.
func ExtractWorkorderPaths(markdown string) []string {
	// --- Step 1: Scan Markdown lines and partition candidates by section priority ---
	lines := strings.Split(markdown, "\n")

	var priorityCandidates []string
	var generalCandidates []string

	inPrioritySection := false
	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "#") {
			lowerHeading := strings.ToLower(trimmed)
			if strings.Contains(lowerHeading, "permitted mutable files") ||
				strings.Contains(lowerHeading, "forbidden files") ||
				strings.Contains(lowerHeading, "target files") ||
				strings.Contains(lowerHeading, "files to modify") {
				inPrioritySection = true
			} else {
				inPrioritySection = false
			}
			continue
		}
		if trimmed == "---" {
			inPrioritySection = false
			continue
		}

		extracted := extractLinePathCandidates(trimmed)
		if inPrioritySection {
			priorityCandidates = append(priorityCandidates, extracted...)
		} else {
			generalCandidates = append(generalCandidates, extracted...)
		}
	}

	// --- Step 2: Normalize and deduplicate candidate paths while preserving priority order ---
	allCandidates := append(priorityCandidates, generalCandidates...)
	seen := make(map[string]bool, len(allCandidates))
	out := make([]string, 0, len(allCandidates))
	for _, cand := range allCandidates {
		norm := normalizeWorkorderCandidatePath(cand)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
}

// extractLinePathCandidates extracts raw path candidates from inline code spans,
// Markdown links, and plain bullet items on a single line.
func extractLinePathCandidates(line string) []string {
	// --- Step 1: Extract candidates from backtick spans and Markdown links ---
	var out []string
	for _, m := range backtickRegex.FindAllStringSubmatch(line, -1) {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	for _, m := range mdLinkRegex.FindAllStringSubmatch(line, -1) {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}

	// --- Step 2: Extract leading token from plain bullet lines (e.g. "- internal/core/foo.go (NEW)") ---
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		rest := strings.TrimSpace(trimmed[2:])
		if !strings.HasPrefix(rest, "`") && !strings.HasPrefix(rest, "[") {
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				out = append(out, fields[0])
			}
		}
	}
	return out
}

// recognizedExtensions lists file extensions accepted as repository file paths in workorders.
var recognizedExtensions = map[string]bool{
	".go":    true,
	".md":    true,
	".json":  true,
	".yaml":  true,
	".yml":   true,
	".sh":    true,
	".sql":   true,
	".ts":    true,
	".tsx":   true,
	".js":    true,
	".jsx":   true,
	".py":    true,
	".rs":    true,
	".toml":  true,
	".proto": true,
	".txt":   true,
	".html":  true,
	".css":   true,
}

// recognizedTopFiles lists extensionless or top-level repository filenames accepted in workorders.
var recognizedTopFiles = map[string]bool{
	"Makefile":   true,
	"Dockerfile": true,
	"go.mod":     true,
	"go.sum":     true,
	"README.md":  true,
	"PROJECT.md": true,
	"MEMORY.md":  true,
	"SPEC.md":    true,
}

// normalizeWorkorderCandidatePath cleans a raw path token from a workorder and
// returns a normalized repository-relative path, or "" if the token is not a valid repo path.
func normalizeWorkorderCandidatePath(raw string) string {
	// --- Step 1: Strip file:// prefixes, URL schemes, and line-number fragments ---
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "file://") {
		s = strings.TrimPrefix(s, "file://")
		// Extract repo-relative tail if recognizable.
		for _, marker := range []string{"/internal/", "/cmd/", "/pkg/", "/docs/", "/.memory/", "/skills/", "/hooks/"} {
			if idx := strings.Index(s, marker); idx >= 0 {
				s = s[idx+1:]
				break
			}
		}
	}
	if strings.Contains(s, "://") {
		return ""
	}
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.Index(s, ":L"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// --- Step 2: Reject shell flags, wildcards, placeholders, and parent-traversal segments ---
	if strings.ContainsAny(s, " \t\n\r*?<>{}|$;\"'") {
		return ""
	}
	if strings.HasPrefix(s, "-") || strings.Contains(s, "...") {
		return ""
	}

	s = filepath.ToSlash(s)
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimSuffix(s, "/")
	if strings.Contains(s, "..") {
		return ""
	}
	s = path.Clean(s)
	if s == "" || s == "." || s == ".." || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "../") || strings.Contains(s, "..") {
		return ""
	}

	// --- Step 3: Validate against external module domains, recognized extensions, and top-level directories ---
	firstSeg := s
	if slash := strings.IndexByte(s, '/'); slash >= 0 {
		firstSeg = s[:slash]
	}
	// Filter external module domains like github.com/... or golang.org/...
	if strings.Contains(firstSeg, ".") && !strings.HasPrefix(firstSeg, ".") && !recognizedTopFiles[s] {
		ext := strings.ToLower(filepath.Ext(s))
		if strings.Contains(s, "/") || !recognizedExtensions[ext] {
			return ""
		}
	}

	if recognizedTopFiles[s] {
		return s
	}

	ext := strings.ToLower(filepath.Ext(s))
	if recognizedExtensions[ext] {
		return s
	}

	// Allow multi-segment repository directory paths (e.g. internal/core/delivery, cmd/repo-memory).
	if strings.Contains(s, "/") {
		switch firstSeg {
		case "internal", "cmd", "pkg", "docs", "skills", "hooks", "scripts", "test", "tests", ".memory", ".agents":
			return s
		}
	}
	return ""
}

// BuildWorkorderBrief constructs a deterministic, token-bounded WorkorderBrief from a WORKORDER.md file
// and/or explicit comma-separated scopes.
func BuildWorkorderBrief(
	ctx context.Context,
	idx *index.Index,
	repoRoot, dataDir, repoID, workorderPath, explicitScope string,
	maxTokens int,
) (*WorkorderBrief, error) {
	// --- Step 1: Normalize defaults and extract candidate file paths from WORKORDER.md ---
	if ctx == nil {
		ctx = context.Background()
	}
	if maxTokens <= 0 {
		maxTokens = DefaultBriefMaxTokens
	}

	var extractedFiles []string
	cleanWorkorderPath := strings.TrimSpace(workorderPath)
	if cleanWorkorderPath != "" {
		content, err := readWorkorderFile(repoRoot, cleanWorkorderPath)
		if err != nil {
			return nil, err
		}
		extractedFiles = ExtractWorkorderPaths(content)
	}
	if extractedFiles == nil {
		extractedFiles = []string{}
	}

	// --- Step 2: Load active entries, resolve scopes, and filter unconsolidated notes ---
	allActiveEntries, dossierFileScopes := loadActiveEntriesForBrief(ctx, idx, repoRoot, dataDir, repoID)
	resolvedScopes, primaryScopes := resolveBriefScopes(explicitScope, extractedFiles, allActiveEntries, dossierFileScopes)

	allUnconsolidatedNotes := loadUnconsolidatedBriefNotes(dataDir, repoID, repoRoot)
	matchedNotes := filterNotesForScopes(allUnconsolidatedNotes, resolvedScopes, extractedFiles)

	// --- Step 3: Compute status line, coverage gaps, and ranked entries ---
	statusLine := buildBriefStatusLine(ctx, idx, repoRoot, dataDir, repoID, len(allUnconsolidatedNotes), allActiveEntries)
	coverageGaps := computeBriefCoverageGaps(primaryScopes, allActiveEntries, dossierFileScopes, allUnconsolidatedNotes)

	rankedEntries := rankEntriesForBrief(ctx, idx, allActiveEntries, extractedFiles, resolvedScopes)

	brief := &WorkorderBrief{
		WorkorderPath:       cleanWorkorderPath,
		ExtractedFiles:      extractedFiles,
		ResolvedScopes:      resolvedScopes,
		StatusLine:          statusLine,
		CoverageGaps:        coverageGaps,
		Entries:             rankedEntries,
		UnconsolidatedNotes: matchedNotes,
	}

	// --- Step 4: Fit entries and notes into the token ceiling and render Markdown ---
	selectedEntries, selectedNotes, markdown := buildBoundedBriefMarkdown(brief, repoRoot, maxTokens)
	brief.Entries = selectedEntries
	brief.UnconsolidatedNotes = selectedNotes
	brief.BriefMarkdown = markdown
	brief.TokenCount = dossiers.EstimateTokens(markdown)

	return brief, nil
}

// readWorkorderFile reads workorderPath directly or relative to repoRoot.
func readWorkorderFile(repoRoot, workorderPath string) (string, error) {
	if data, err := os.ReadFile(workorderPath); err == nil {
		return string(data), nil
	}
	if repoRoot != "" && !filepath.IsAbs(workorderPath) {
		alt := filepath.Join(repoRoot, workorderPath)
		if data, err := os.ReadFile(alt); err == nil {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("reading workorder file %s: %w", workorderPath, os.ErrNotExist)
}

// resolveBriefScopes derives resolved and primary dossier scopes from explicit
// comma-separated scopes and extracted file paths.
func resolveBriefScopes(
	explicitScope string,
	extractedFiles []string,
	activeEntries []dossiers.Entry,
	dossierFileScopes map[string]bool,
) (resolved []string, primary []string) {
	// --- Step 1: Initialize scope accumulators and add explicit scopes ---
	seenResolved := make(map[string]bool)
	seenPrimary := make(map[string]bool)

	addResolved := func(s string) {
		s = normalizeScopeName(s)
		if s == "" || seenResolved[s] {
			return
		}
		seenResolved[s] = true
		resolved = append(resolved, s)
	}
	addPrimary := func(s string) {
		s = normalizeScopeName(s)
		if s == "" || seenPrimary[s] {
			return
		}
		seenPrimary[s] = true
		primary = append(primary, s)
		addResolved(s)
	}

	for _, part := range strings.Split(explicitScope, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			addPrimary(trimmed)
		}
	}

	// --- Step 2: Index known dossier scopes and check whether non-.agents code files exist ---
	// Check if there are non-.agents files so .agents/WORKORDER.md doesn't pollute primary code scopes.
	hasCodeFiles := false
	for _, f := range extractedFiles {
		if !strings.HasPrefix(f, ".agents/") {
			hasCodeFiles = true
			break
		}
	}

	knownDossierScopes := make(map[string]bool)
	for s := range dossierFileScopes {
		knownDossierScopes[normalizeScopeName(s)] = true
	}
	for _, e := range activeEntries {
		if s := normalizeScopeName(e.DossierScope); s != "" {
			knownDossierScopes[s] = true
		}
	}

	// --- Step 3: Map extracted file paths and their ancestor directories to scopes ---
	for _, f := range extractedFiles {
		if hasCodeFiles && strings.HasPrefix(f, ".agents/") {
			continue
		}
		ext := filepath.Ext(f)
		dir := f
		if ext != "" {
			dir = filepath.ToSlash(filepath.Dir(f))
		}
		if dir != "." && dir != "" {
			addPrimary(dir)
			// Also add ancestor directories to resolved scopes.
			curr := dir
			for {
				parent := filepath.ToSlash(filepath.Dir(curr))
				if parent == "." || parent == "/" || parent == curr {
					break
				}
				addResolved(parent)
				if knownDossierScopes[parent] {
					addPrimary(parent)
				}
				curr = parent
			}
			// If base name matches a known dossier scope (e.g. "delivery" for "internal/core/delivery"), add it to resolved.
			base := filepath.Base(dir)
			if knownDossierScopes[base] {
				addResolved(base)
			}
		} else if knownDossierScopes[f] {
			addPrimary(f)
		}
	}

	if resolved == nil {
		resolved = []string{}
	}
	if primary == nil {
		primary = []string{}
	}
	return resolved, primary
}

// normalizeScopeName normalizes a dossier scope identifier to slash-separated relative form.
func normalizeScopeName(s string) string {
	s = filepath.ToSlash(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimSuffix(s, "/")
	if s == "." {
		return ""
	}
	return s
}

// loadActiveEntriesForBrief loads active (non-retired, non-superseded) dossier
// entries from the SQLite index, op-log directories, and rendered dossier files.
func loadActiveEntriesForBrief(
	ctx context.Context,
	idx *index.Index,
	repoRoot, dataDir, repoID string,
) ([]dossiers.Entry, map[string]bool) {
	// --- Step 1: Set up deduplicating entry map and load entries from the SQLite index ---
	byID := make(map[string]dossiers.Entry)
	var orderedIDs []string
	dossierScopes := make(map[string]bool)

	addEntry := func(e dossiers.Entry) {
		if e.Retired || strings.TrimSpace(e.SupersededBy) != "" {
			delete(byID, e.ID)
			return
		}
		if e.ID == "" {
			return
		}
		if scope := normalizeScopeName(e.DossierScope); scope != "" {
			dossierScopes[scope] = true
		}
		if _, exists := byID[e.ID]; !exists {
			orderedIDs = append(orderedIDs, e.ID)
		}
		byID[e.ID] = e
	}

	// 1. From SQLite index if provided
	if idx != nil {
		if list, err := idx.ListAllEntries(ctx); err == nil {
			for _, e := range list {
				addEntry(e)
			}
		}
	}

	// --- Step 2: Project active entries from shared, local, and user op-log directories ---
	var opsDirs []string
	if strings.TrimSpace(repoRoot) != "" {
		opsDirs = append(opsDirs, filepath.Join(repoRoot, ".memory", "ops"))
		localOps := filepath.Join(repoRoot, ".memory", "local", "ops")
		opsDirs = append(opsDirs, localOps)
		if entries, err := os.ReadDir(localOps); err == nil {
			for _, de := range entries {
				if de.IsDir() {
					opsDirs = append(opsDirs, filepath.Join(localOps, de.Name()))
				}
			}
		}
	}
	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		opsDirs = append(opsDirs,
			filepath.Join(dataDir, "repos", repoID, "user-dossiers", "ops"),
			filepath.Join(dataDir, repoID, "user-dossiers", "ops"),
		)
	}
	for _, dir := range opsDirs {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			if projected, err := dossiers.NewProjector(ops.NewReader(dir)).Project(ctx); err == nil {
				for scopeKey, d := range projected {
					if d == nil {
						continue
					}
					sName := normalizeScopeName(d.Scope)
					if sName == "" {
						sName = normalizeScopeName(scopeKey)
					}
					if sName != "" {
						dossierScopes[sName] = true
					}
					for _, e := range d.Entries {
						if strings.TrimSpace(e.DossierScope) == "" {
							e.DossierScope = sName
						}
						addEntry(e)
					}
				}
			}
		}
	}

	// --- Step 3: Augment with entries parsed from .memory/dossiers/ Markdown or JSON files ---
	if strings.TrimSpace(repoRoot) != "" {
		dossiersDir := filepath.Join(repoRoot, ".memory", "dossiers")
		_ = filepath.WalkDir(dossiersDir, func(filePath string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".history.md") {
				return nil
			}
			rel, relErr := filepath.Rel(dossiersDir, filePath)
			scopeFromPath := ""
			if relErr == nil {
				scopeFromPath = normalizeScopeName(strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel)))
				scopeFromPath = strings.ReplaceAll(scopeFromPath, "__", "/")
			}
			if scopeFromPath != "" {
				dossierScopes[scopeFromPath] = true
			}
			parsedEntries, fileScope := parseDossierFileEntries(filePath, scopeFromPath)
			if fileScope != "" {
				dossierScopes[normalizeScopeName(fileScope)] = true
			}
			for _, pe := range parsedEntries {
				if existing, exists := byID[pe.ID]; exists {
					if existing.StaleSuspect || !pe.StaleSuspect {
						continue
					}
				}
				addEntry(pe)
			}
			return nil
		})
	}

	// --- Step 4: Collect active entries in deterministic insertion order ---
	out := make([]dossiers.Entry, 0, len(byID))
	for _, id := range orderedIDs {
		if e, ok := byID[id]; ok {
			out = append(out, e)
		}
	}
	return out, dossierScopes
}

// parseDossierFileEntries parses dossier entries and the resolved scope name
// from a rendered Markdown or JSON dossier file at filePath.
func parseDossierFileEntries(filePath, defaultScope string) ([]dossiers.Entry, string) {
	// --- Step 1: Read file content and handle JSON dossier format if applicable ---
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, defaultScope
	}
	content := string(data)
	if strings.HasSuffix(strings.ToLower(filePath), ".json") {
		var entries []dossiers.Entry
		if err := json.Unmarshal(data, &entries); err == nil && len(entries) > 0 {
			for i := range entries {
				if entries[i].DossierScope == "" {
					entries[i].DossierScope = defaultScope
				}
			}
			return entries, defaultScope
		}
	}

	// --- Step 2: Extract scope from frontmatter or top-level Markdown heading ---
	scope := defaultScope
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			h := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			if h != "" && !strings.Contains(strings.ToLower(h), "(history)") {
				scope = h
			}
			break
		}
		if strings.HasPrefix(trimmed, "scope:") {
			val := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "scope:")), `"'`)
			if val != "" {
				scope = val
			}
		}
	}

	// --- Step 3: Parse Markdown entry headings, metadata lines, and body text ---
	var entries []dossiers.Entry
	var current *dossiers.Entry
	var bodyLines []string

	flush := func() {
		if current == nil {
			return
		}
		if len(bodyLines) > 0 {
			if current.Text != "" {
				current.Text = current.Text + "\n" + strings.TrimSpace(strings.Join(bodyLines, "\n"))
			} else {
				current.Text = strings.TrimSpace(strings.Join(bodyLines, "\n"))
			}
		}
		if current.Type == "" {
			current.Type = inferEntryTypeFromID(current.ID)
		}
		if current.Kind == "" {
			current.Kind = dossiers.DeriveCognitiveKind(current.Type)
		}
		entries = append(entries, *current)
		current = nil
		bodyLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "### [") {
			flush()
			closeIdx := strings.Index(trimmed, "]")
			if closeIdx > 5 {
				id := strings.TrimSpace(trimmed[5:closeIdx])
				title := strings.TrimSpace(trimmed[closeIdx+1:])
				current = &dossiers.Entry{
					ID:           id,
					DossierScope: scope,
					Text:         title,
				}
				continue
			}
		}
		if current != nil {
			if strings.HasPrefix(trimmed, "`") && strings.HasSuffix(trimmed, "`") && strings.Contains(trimmed, "·") {
				metaLine := strings.Trim(trimmed, "`")
				parts := strings.Split(metaLine, "·")
				for idxPart, rawPart := range parts {
					p := strings.TrimSpace(rawPart)
					if idxPart == 0 && p != "" && !strings.Contains(p, ":") {
						current.Type = dossiers.EntryType(p)
					} else if strings.HasPrefix(p, "kind:") {
						current.Kind = dossiers.CognitiveKind(strings.TrimSpace(strings.TrimPrefix(p, "kind:")))
					} else if strings.HasPrefix(p, "anchors:") {
						for _, a := range strings.Split(strings.TrimPrefix(p, "anchors:"), ",") {
							if ta := strings.TrimSpace(a); ta != "" {
								current.Anchors = append(current.Anchors, ta)
							}
						}
					} else if strings.HasPrefix(p, "evidence:") {
						for _, ev := range strings.Split(strings.TrimPrefix(p, "evidence:"), ",") {
							if tev := strings.TrimSpace(ev); tev != "" {
								current.Evidence = append(current.Evidence, tev)
							}
						}
					} else if strings.HasPrefix(p, "verified ") {
						current.Verified = strings.TrimSpace(strings.TrimPrefix(p, "verified "))
						if strings.EqualFold(current.Verified, "suspect") {
							current.StaleSuspect = true
						}
					} else if strings.EqualFold(p, "suspect") || strings.EqualFold(p, "stale_suspect: true") {
						current.StaleSuspect = true
					}
				}
				continue
			}
			bodyLines = append(bodyLines, line)
		}
	}
	flush()

	return entries, scope
}

// inferEntryTypeFromID infers a dossiers.EntryType from a conventional entry ID prefix
// when disk markdown sections do not explicitly encode the entry's sub-type.
func inferEntryTypeFromID(id string) dossiers.EntryType {
	// --- Step 1: Normalize entry ID prefix for case-insensitive matching ---
	lower := strings.ToLower(strings.TrimSpace(id))

	// --- Step 2: Map conventional ID prefix to its canonical dossiers.EntryType ---
	switch {
	case strings.HasPrefix(lower, "inv-"):
		return dossiers.EntryTypeInvariant
	case strings.HasPrefix(lower, "con-"):
		return dossiers.EntryTypeContract
	case strings.HasPrefix(lower, "rul-"):
		return dossiers.EntryTypeRuledOut
	case strings.HasPrefix(lower, "fail-"):
		return dossiers.EntryTypeFailureSignature
	case strings.HasPrefix(lower, "got-"):
		return dossiers.EntryTypeGotcha
	case strings.HasPrefix(lower, "dec-"):
		return dossiers.EntryTypeDecision
	case strings.HasPrefix(lower, "proc-"), strings.HasPrefix(lower, "ver-"):
		return dossiers.EntryTypeProcedure
	case strings.HasPrefix(lower, "rec-"):
		return dossiers.EntryTypeRecipe
	case strings.HasPrefix(lower, "iface-"), strings.HasPrefix(lower, "arch-"):
		return dossiers.EntryTypeInterface
	case strings.HasPrefix(lower, "issue-"):
		return dossiers.EntryTypeOpenIssue
	default:
		return dossiers.EntryTypeInvariant
	}
}

// loadUnconsolidatedBriefNotes loads and deduplicates unconsolidated notes from
// dataDir evidence directories and the repository .memory/inbox directory.
func loadUnconsolidatedBriefNotes(dataDir, repoID, repoRoot string) []BriefNoteSummary {
	// --- Step 1: Collect candidate note directories ---
	var dirs []string
	if dataDir != "" && repoID != "" {
		dirs = append(dirs,
			filepath.Join(dataDir, "repos", repoID, "evidence", "notes"),
			filepath.Join(dataDir, repoID, "evidence", "notes"),
		)
	}
	if repoRoot != "" {
		dirs = append(dirs, filepath.Join(repoRoot, ".memory", "inbox"))
	}

	// --- Step 2: Parse and deduplicate JSON note files across directories ---
	seen := make(map[string]bool)
	var out []BriefNoteSummary

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			fullPath := filepath.Join(dir, entry.Name())
			notes := parseBriefNotesFromFile(fullPath)
			for _, bn := range notes {
				key := bn.ID + "|" + bn.Scope + "|" + bn.Statement
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, bn)
			}
		}
	}

	// --- Step 3: Sort unconsolidated notes chronologically by CreatedAt then ID ---
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt < out[j].CreatedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// parseBriefNotesFromFile parses unconsolidated BriefNoteSummary items from a JSON note file at filePath.
func parseBriefNotesFromFile(filePath string) []BriefNoteSummary {
	// --- Step 1: Unmarshal JSON note payload and skip already-consolidated notes ---
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var raw struct {
		ID             string   `json:"id"`              // Note ID.
		Kind           string   `json:"kind"`            // Cognitive kind.
		Type           string   `json:"type"`            // Entry type.
		Scope          string   `json:"scope"`           // Target dossier scope.
		Statement      string   `json:"statement"`       // Primary note statement.
		Title          string   `json:"title"`           // Fallback note title.
		CreatedAt      any      `json:"created_at"`      // Timestamp value.
		Consolidated   bool     `json:"consolidated"`    // True if already consolidated.
		ConsolidatedAt string   `json:"consolidated_at"` // Consolidation timestamp.
		SupersededBy   string   `json:"superseded_by"`   // Superseding entry ID.
		Anchors        []string `json:"anchors"`         // Associated file/symbol anchors.
		Items          []struct {
			Type    string   `json:"type"`    // Item type.
			Title   string   `json:"title"`   // Item title.
			Why     string   `json:"why"`     // Item rationale.
			Do      string   `json:"do"`      // Item action.
			Scope   string   `json:"scope"`   // Item scope.
			Anchors []string `json:"anchors"` // Item anchors.
		} `json:"items"` // Multi-item batch entries.
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	if raw.Consolidated || strings.TrimSpace(raw.ConsolidatedAt) != "" || strings.TrimSpace(raw.SupersededBy) != "" {
		return nil
	}

	id := strings.TrimSpace(raw.ID)
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(filePath), ".json")
	}
	createdAtStr := ""
	if s, ok := raw.CreatedAt.(string); ok {
		createdAtStr = strings.TrimSpace(s)
	}

	// --- Step 2: Extract summaries from multi-item note payloads if present ---
	if len(raw.Items) > 0 {
		var res []BriefNoteSummary
		for _, it := range raw.Items {
			kind := strings.TrimSpace(it.Type)
			if kind == "" {
				kind = "note"
			}
			scope := normalizeScopeName(it.Scope)
			if scope == "" && len(it.Anchors) > 0 {
				scope = normalizeScopeName(stripBriefAnchor(it.Anchors[0]))
			}
			if scope == "" {
				scope = normalizeScopeName(raw.Scope)
			}
			stmt := firstLineBrief(it.Title)
			if stmt == "" {
				stmt = firstLineBrief(it.Why)
			}
			if stmt == "" {
				stmt = firstLineBrief(it.Do)
			}
			if stmt == "" {
				continue
			}
			res = append(res, BriefNoteSummary{
				ID:        id,
				Kind:      kind,
				Scope:     scope,
				Statement: stmt,
				CreatedAt: createdAtStr,
			})
		}
		return res
	}

	// --- Step 3: Extract summary from a single-note payload ---
	kind := strings.TrimSpace(raw.Kind)
	if kind == "" {
		kind = strings.TrimSpace(raw.Type)
	}
	if kind == "" {
		kind = "note"
	}
	scope := normalizeScopeName(raw.Scope)
	if scope == "" && len(raw.Anchors) > 0 {
		scope = normalizeScopeName(stripBriefAnchor(raw.Anchors[0]))
	}
	stmt := firstLineBrief(raw.Statement)
	if stmt == "" {
		stmt = firstLineBrief(raw.Title)
	}
	if stmt == "" {
		return nil
	}
	return []BriefNoteSummary{
		{
			ID:        id,
			Kind:      kind,
			Scope:     scope,
			Statement: stmt,
			CreatedAt: createdAtStr,
		},
	}
}

// filterNotesForScopes filters unconsolidated notes to those overlapping resolvedScopes or extractedFiles.
func filterNotesForScopes(notes []BriefNoteSummary, resolvedScopes, extractedFiles []string) []BriefNoteSummary {
	if len(notes) == 0 {
		return []BriefNoteSummary{}
	}
	if len(resolvedScopes) == 0 && len(extractedFiles) == 0 {
		return notes
	}

	out := make([]BriefNoteSummary, 0)
	for _, n := range notes {
		if scopeOverlapsTargets(n.Scope, resolvedScopes, extractedFiles) {
			out = append(out, n)
		}
	}
	return out
}

// scopeOverlapsTargets reports whether scope matches or is an ancestor/descendant of resolvedScopes or extractedFiles.
func scopeOverlapsTargets(scope string, resolvedScopes, extractedFiles []string) bool {
	// --- Step 1: Treat repo-wide default scopes as overlapping all targets ---
	s := normalizeScopeName(scope)
	if s == "" || s == "repo" || s == "general" || s == "_default" {
		return true
	}

	// --- Step 2: Check prefix or exact overlap against extracted files and resolved scopes ---
	for _, f := range extractedFiles {
		nf := normalizeScopeName(f)
		if nf == s || strings.HasPrefix(nf, s+"/") || strings.HasPrefix(s, nf+"/") {
			return true
		}
	}
	for _, rs := range resolvedScopes {
		nrs := normalizeScopeName(rs)
		if nrs == "" {
			continue
		}
		if s == nrs || filepath.Base(nrs) == s || filepath.Base(s) == nrs {
			return true
		}
		if strings.HasPrefix(s, nrs+"/") || strings.HasPrefix(nrs, s+"/") {
			return true
		}
	}
	return false
}

// buildBriefStatusLine formats the single-line index freshness and health header for a workorder brief.
func buildBriefStatusLine(
	ctx context.Context,
	idx *index.Index,
	repoRoot, dataDir, repoID string,
	unconsolidatedCount int,
	activeEntries []dossiers.Entry,
) string {
	// --- Step 1: Count suspect entries and read optional freshness.json state ---
	suspectCount := 0
	for _, e := range activeEntries {
		if e.StaleSuspect || strings.EqualFold(strings.TrimSpace(e.Verified), "suspect") {
			suspectCount++
		}
	}

	var st struct {
		UnconsolidatedNotes int    `json:"unconsolidated_notes"`   // Unconsolidated note count from freshness state.
		IndexCurrentToOpLog *bool  `json:"index_current_to_oplog"` // Whether the SQLite index is caught up with the op-log.
		IndexHeadSHA        string `json:"index_head_sha"`         // Git HEAD SHA recorded at index build time.
		HeadSHA             string `json:"head_sha"`               // Fallback HEAD SHA field.
		LastCommit          string `json:"last_commit"`            // Fallback last-commit SHA field.
		SuspectEntries      int    `json:"suspect_entries"`        // Suspect entry count from freshness state.
		UpdatedAt           string `json:"updated_at"`             // Timestamp of freshness state update.
	}
	freshnessFound := false
	if dataDir != "" && repoID != "" {
		for _, p := range []string{
			filepath.Join(dataDir, "repos", repoID, "state", "freshness.json"),
			filepath.Join(dataDir, repoID, "state", "freshness.json"),
		} {
			if data, err := os.ReadFile(p); err == nil {
				if json.Unmarshal(data, &st) == nil {
					freshnessFound = true
					break
				}
			}
		}
	}

	if unconsolidatedCount == 0 && st.UnconsolidatedNotes > 0 {
		unconsolidatedCount = st.UnconsolidatedNotes
	}
	if suspectCount == 0 && st.SuspectEntries > 0 {
		suspectCount = st.SuspectEntries
	}

	// --- Step 2: Resolve short git HEAD SHA and format index status label ---
	headSHA := strings.TrimSpace(st.IndexHeadSHA)
	if headSHA == "" {
		headSHA = strings.TrimSpace(st.HeadSHA)
	}
	if headSHA == "" {
		headSHA = strings.TrimSpace(st.LastCommit)
	}
	if headSHA == "" && repoRoot != "" {
		if sha, err := gitops.GetHeadCommitWithContext(ctx, repoRoot); err == nil {
			headSHA = strings.TrimSpace(sha)
		}
	}
	if len(headSHA) > 7 {
		headSHA = headSHA[:7]
	}

	indexStatus := "index status unknown"
	if freshnessFound {
		if st.IndexCurrentToOpLog != nil && !*st.IndexCurrentToOpLog {
			indexStatus = "index behind op-log"
		} else if headSHA != "" {
			indexStatus = fmt.Sprintf("index current (HEAD %s)", headSHA)
		} else {
			indexStatus = "index current"
		}
	} else if idx != nil {
		if headSHA != "" {
			indexStatus = fmt.Sprintf("index current (HEAD %s)", headSHA)
		} else {
			indexStatus = "index current"
		}
	} else if headSHA != "" {
		indexStatus = fmt.Sprintf("index status unknown (HEAD %s)", headSHA)
	}

	return fmt.Sprintf("memory: %s · %d notes not yet consolidated · %d entries suspect",
		indexStatus, unconsolidatedCount, suspectCount)
}

// computeBriefCoverageGaps returns up to 3 coverage gap warnings for primaryScopes
// that have no dossier or contain suspect entries.
func computeBriefCoverageGaps(
	primaryScopes []string,
	activeEntries []dossiers.Entry,
	dossierFileScopes map[string]bool,
	allUnconsolidatedNotes []BriefNoteSummary,
) []string {
	// --- Step 1: Evaluate each primary scope for dossier presence and suspect entries ---
	var gaps []string
	seenScope := make(map[string]bool)

	for _, rawScope := range primaryScopes {
		scope := normalizeScopeName(rawScope)
		if scope == "" || seenScope[scope] {
			continue
		}
		seenScope[scope] = true

		// Count active entries and suspect entries covering this scope.
		activeInScope := 0
		suspectInScope := 0
		for _, e := range activeEntries {
			if entryMatchesScopeOrFile(e, []string{scope}, nil) {
				activeInScope++
				if e.StaleSuspect || strings.EqualFold(strings.TrimSpace(e.Verified), "suspect") {
					suspectInScope++
				}
			}
		}

		// --- Step 2: Emit gap warning when a scope lacks a dossier or has suspect entries ---
		hasDossier := activeInScope > 0 || dossierFileScopes[scope] || dossierFileScopes[filepath.Base(scope)]
		if !hasDossier {
			pendingNotes := 0
			for _, n := range allUnconsolidatedNotes {
				if scopeOverlapsTargets(n.Scope, []string{scope}, nil) {
					pendingNotes++
				}
			}
			gaps = append(gaps, fmt.Sprintf("coverage: %s has no dossier yet (%d unconsolidated notes pending)", scope, pendingNotes))
		} else if suspectInScope > 0 {
			gaps = append(gaps, fmt.Sprintf("coverage: %s has %d suspect entries needing verification", scope, suspectInScope))
		}

		if len(gaps) >= 3 {
			break
		}
	}

	if gaps == nil {
		gaps = []string{}
	}
	return gaps
}

// rankEntriesForBrief filters and ranks active entries for a workorder brief using
// target anchors, FTS5 BM25 scores, scope boosts, and Principal Architect type priorities.
func rankEntriesForBrief(
	ctx context.Context,
	idx *index.Index,
	allActiveEntries []dossiers.Entry,
	extractedFiles, resolvedScopes []string,
) []RankedEntry {
	if len(allActiveEntries) == 0 {
		return []RankedEntry{}
	}

	// --- Step 1: Build deduplicated target anchors and optional FTS5 BM25 score map ---
	var targetAnchors []string
	seenAnchor := make(map[string]bool)
	addAnchor := func(a string) {
		a = normalizeScopeName(a)
		if a == "" || seenAnchor[a] {
			return
		}
		seenAnchor[a] = true
		targetAnchors = append(targetAnchors, a)
	}
	for _, f := range extractedFiles {
		addAnchor(f)
	}
	for _, s := range resolvedScopes {
		addAnchor(s)
	}

	ftsScores := make(map[string]float64)
	if idx != nil {
		if ftsQuery := buildBriefFTSQuery(extractedFiles, resolvedScopes); ftsQuery != "" {
			if scored, err := idx.SearchEntriesWithScore(ctx, ftsQuery); err == nil {
				for _, se := range scored {
					ftsScores[se.Entry.ID] = se.Score
				}
			}
		}
	}

	// --- Step 2: Filter to entries matching resolved scopes, extracted files, or FTS hits ---
	var matched []dossiers.Entry
	for _, e := range allActiveEntries {
		if entryMatchesScopeOrFile(e, resolvedScopes, extractedFiles) || ftsScores[e.ID] > 0 {
			matched = append(matched, e)
		}
	}
	if len(matched) == 0 {
		matched = allActiveEntries
	}

	// --- Step 3: Compute base rankings and apply workorder-specific scope and type boosts ---
	ranked := RankEntries(matched, targetAnchors, ftsScores)
	for i := range ranked {
		e := ranked[i].Entry
		// Boost entries whose DossierScope directly matches a resolved scope.
		for _, rs := range resolvedScopes {
			if strings.EqualFold(e.DossierScope, rs) || strings.EqualFold(e.DossierScope, filepath.Base(rs)) {
				ranked[i].Score += 300
				if ranked[i].AnchorMatch == "" {
					ranked[i].AnchorMatch = "scope"
				}
				break
			}
		}
		// Boost priority PA types: invariant, gotcha, verification/procedure, architecture/interface, recent_change/decision.
		switch strings.ToLower(string(e.Type)) {
		case "invariant", "contract", "convention":
			ranked[i].Score += 90
		case "gotcha", "failure-signature", "ruled-out":
			ranked[i].Score += 80
		case "verification", "procedure", "recipe":
			ranked[i].Score += 70
		case "architecture", "interface":
			ranked[i].Score += 60
		case "recent_change", "decision":
			ranked[i].Score += 50
		}
	}

	// --- Step 4: Sort deterministically by descending score then ascending entry ID ---
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].Entry.ID < ranked[j].Entry.ID
	})
	return ranked
}

// entryMatchesScopeOrFile reports whether entry e matches any of resolvedScopes or extractedFiles
// via its DossierScope or its file/symbol Anchors.
func entryMatchesScopeOrFile(e dossiers.Entry, resolvedScopes, extractedFiles []string) bool {
	// --- Step 1: Check whether the entry's DossierScope matches any resolved scope ---
	eScope := normalizeScopeName(e.DossierScope)
	for _, rs := range resolvedScopes {
		nrs := normalizeScopeName(rs)
		if nrs == "" {
			continue
		}
		if eScope == nrs || eScope == filepath.Base(nrs) || filepath.Base(eScope) == nrs {
			return true
		}
		if eScope != "" && (strings.HasPrefix(eScope, nrs+"/") || strings.HasPrefix(nrs, eScope+"/")) {
			return true
		}
	}

	// --- Step 2: Check whether any of the entry's anchors match extracted files or resolved scopes ---
	for _, rawAnchor := range e.Anchors {
		a := stripBriefAnchor(rawAnchor)
		if a == "" {
			continue
		}
		for _, f := range extractedFiles {
			nf := normalizeScopeName(f)
			if a == nf || strings.HasPrefix(nf, a+"/") || strings.HasPrefix(a, nf+"/") {
				return true
			}
		}
		for _, rs := range resolvedScopes {
			nrs := normalizeScopeName(rs)
			if a == nrs || strings.HasPrefix(a, nrs+"/") || strings.HasPrefix(nrs, a+"/") {
				return true
			}
		}
	}
	return false
}

// buildBriefFTSQuery constructs a bounded OR-joined FTS5 query from path segments
// in resolvedScopes and filename stems in extractedFiles.
func buildBriefFTSQuery(extractedFiles, resolvedScopes []string) string {
	// --- Step 1: Define token sanitizer excluding generic directory names ---
	seen := make(map[string]bool)
	var terms []string
	addToken := func(tok string) {
		tok = strings.TrimSpace(tok)
		if len(tok) < 3 {
			return
		}
		for _, r := range tok {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
				return
			}
		}
		lower := strings.ToLower(tok)
		if lower == "internal" || lower == "core" || lower == "cmd" || seen[lower] {
			return
		}
		seen[lower] = true
		terms = append(terms, lower)
	}

	// --- Step 2: Collect tokens from scopes and file basenames and join with OR ---
	for _, s := range resolvedScopes {
		for _, part := range strings.Split(s, "/") {
			addToken(part)
		}
	}
	for _, f := range extractedFiles {
		base := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		addToken(base)
	}
	if len(terms) == 0 {
		return ""
	}
	if len(terms) > 8 {
		terms = terms[:8]
	}
	return strings.Join(terms, " OR ")
}

// buildBoundedBriefMarkdown progressively tightens entry counts, note counts, and
// per-line character limits until the rendered Markdown fits within maxTokens.
func buildBoundedBriefMarkdown(
	brief *WorkorderBrief,
	repoRoot string,
	maxTokens int,
) ([]RankedEntry, []BriefNoteSummary, string) {
	// --- Step 1: Define progressive compaction tiers ---
	type budgetAttempt struct {
		maxEntries   int // Maximum number of ranked entries to include.
		maxNotes     int // Maximum number of unconsolidated notes to include.
		maxTextChars int // Maximum character length for a single entry or note line.
	}
	attempts := []budgetAttempt{
		{12, 8, 220},
		{8, 6, 180},
		{6, 4, 140},
		{4, 3, 110},
		{3, 2, 90},
		{2, 1, 75},
		{1, 1, 60},
		{1, 0, 48},
		{0, 0, 36},
	}

	// --- Step 2: Try each compaction tier until the rendered Markdown fits maxTokens ---
	for _, att := range attempts {
		eCount := len(brief.Entries)
		if eCount > att.maxEntries {
			eCount = att.maxEntries
		}
		nCount := len(brief.UnconsolidatedNotes)
		if nCount > att.maxNotes {
			nCount = att.maxNotes
		}
		selEntries := brief.Entries[:eCount]
		selNotes := brief.UnconsolidatedNotes[:nCount]
		md := renderWorkorderBriefMarkdown(
			repoRoot,
			brief.StatusLine,
			brief.CoverageGaps,
			brief.ResolvedScopes,
			selEntries,
			len(brief.Entries)-eCount,
			selNotes,
			len(brief.UnconsolidatedNotes)-nCount,
			att.maxTextChars,
		)
		if dossiers.EstimateTokens(md) <= maxTokens {
			return selEntries, selNotes, md
		}
	}

	// --- Step 3: Fall back to minimal header-only rendering truncated to maxBytes ---
	md := renderWorkorderBriefMarkdown(repoRoot, brief.StatusLine, nil, brief.ResolvedScopes, nil, len(brief.Entries), nil, len(brief.UnconsolidatedNotes), 32)
	maxBytes := maxTokens * 4
	if maxBytes > 0 && len(md) > maxBytes {
		md = strings.TrimSpace(md[:maxBytes])
	}
	return []RankedEntry{}, []BriefNoteSummary{}, md
}

// renderWorkorderBriefMarkdown renders the workorder brief status header,
// coverage gaps, resolved scopes, ranked dossier entries, and unconsolidated notes.
func renderWorkorderBriefMarkdown(
	repoRoot string,
	statusLine string,
	coverageGaps []string,
	resolvedScopes []string,
	entries []RankedEntry,
	omittedEntries int,
	notes []BriefNoteSummary,
	omittedNotes int,
	maxTextChars int,
) string {
	// --- Step 1: Write status line, coverage gap warnings, and resolved scopes ---
	var b strings.Builder

	if strings.TrimSpace(statusLine) != "" {
		b.WriteString(strings.TrimSpace(statusLine) + "\n")
	}
	for _, gap := range coverageGaps {
		if strings.TrimSpace(gap) != "" {
			b.WriteString(strings.TrimSpace(gap) + "\n")
		}
	}

	b.WriteString("\n## Workorder Brief\n")
	if len(resolvedScopes) > 0 {
		scopesLine := strings.Join(resolvedScopes, ", ")
		if maxTextChars > 0 && len(scopesLine) > maxTextChars*2 {
			scopesLine = scopesLine[:maxTextChars*2-3] + "..."
		}
		b.WriteString(fmt.Sprintf("Scopes: %s\n", scopesLine))
	} else {
		b.WriteString("Scopes: (none)\n")
	}

	// --- Step 2: Render relevant dossier entries and omitted-entry count ---
	b.WriteString("\n### Relevant Dossier Entries\n")
	if len(entries) == 0 {
		if omittedEntries > 0 {
			b.WriteString(fmt.Sprintf("- ... (+%d more)\n", omittedEntries))
		} else {
			b.WriteString("- (none)\n")
		}
	} else {
		for _, re := range entries {
			b.WriteString(formatBriefEntryLine(re.Entry, repoRoot, maxTextChars) + "\n")
		}
		if omittedEntries > 0 {
			b.WriteString(fmt.Sprintf("- ... (+%d more)\n", omittedEntries))
		}
	}

	// --- Step 3: Render unconsolidated notes and omitted-note count ---
	if len(notes) > 0 || omittedNotes > 0 {
		b.WriteString("\n### Unconsolidated Notes\n")
		for _, n := range notes {
			scope := strings.TrimSpace(n.Scope)
			if scope == "" {
				scope = "repo"
			}
			stmt := compactSingleLine(n.Statement, maxTextChars)
			b.WriteString(fmt.Sprintf("- [%s] (%s, %s) %s\n", n.ID, n.Kind, scope, stmt))
		}
		if omittedNotes > 0 {
			b.WriteString(fmt.Sprintf("- ... (+%d more)\n", omittedNotes))
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

// formatBriefEntryLine formats a single dossier entry bullet line for a workorder brief,
// substituting a skill pointer when a procedural entry has graduated to a skill.
func formatBriefEntryLine(e dossiers.Entry, repoRoot string, maxTextChars int) string {
	// --- Step 1: Resolve scope and type labels and check for graduated skills ---
	scopeLabel := strings.TrimSpace(e.DossierScope)
	if scopeLabel == "" {
		scopeLabel = "repo"
	}
	kindLabel := strings.TrimSpace(string(e.Type))
	if kindLabel == "" {
		kindLabel = "invariant"
	}
	if !e.Retired && !e.StaleSuspect && (e.Type == dossiers.EntryTypeRecipe || e.Type == dossiers.EntryTypeProcedure) {
		slug := skills.GraduatedSkillSlug(e, repoRoot)
		if slug == "" && strings.TrimSpace(e.RenderAsSkill) != "" {
			slug = skills.SkillSlug(e)
		}
		if slug != "" {
			return fmt.Sprintf("- [%s] (%s, %s) skill %s covers this procedure: %s", e.ID, kindLabel, scopeLabel, slug, skills.EntryTitleLine(e))
		}
	}

	// --- Step 2: Format single-line statement with optional provenance suffix ---
	stmt := compactSingleLine(e.Text, maxTextChars)
	prov := formatEntryProvenance(e)
	if prov != "" {
		return fmt.Sprintf("- [%s] (%s, %s) %s — %s", e.ID, kindLabel, scopeLabel, stmt, prov)
	}
	return fmt.Sprintf("- [%s] (%s, %s) %s", e.ID, kindLabel, scopeLabel, stmt)
}

// formatEntryProvenance formats verification status, evidence count, or source metadata for an entry.
func formatEntryProvenance(e dossiers.Entry) string {
	// --- Step 1: Collect verification status ---
	var parts []string
	if v := strings.TrimSpace(e.Verified); v != "" {
		parts = append(parts, "verified "+v)
	} else if e.StaleSuspect {
		parts = append(parts, "suspect")
	}

	// --- Step 2: Append primary evidence reference, source, or update date ---
	if len(e.Evidence) > 0 {
		ev := strings.TrimSpace(e.Evidence[0])
		if len(e.Evidence) > 1 {
			ev = fmt.Sprintf("%s +%d", ev, len(e.Evidence)-1)
		}
		parts = append(parts, ev)
	} else if src := strings.TrimSpace(e.Source); src != "" {
		parts = append(parts, src)
	} else if !e.UpdatedAt.IsZero() {
		parts = append(parts, e.UpdatedAt.UTC().Format(time.DateOnly))
	}
	return strings.Join(parts, ", ")
}

// compactSingleLine collapses whitespace in s into single spaces and truncates to maxChars.
func compactSingleLine(s string, maxChars int) string {
	collapsed := strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if maxChars > 10 && len(collapsed) > maxChars {
		return collapsed[:maxChars-3] + "..."
	}
	return collapsed
}

// firstLineBrief returns the first non-empty trimmed line of s.
func firstLineBrief(s string) string {
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// stripBriefAnchor strips `#symbol` and `:L123` suffixes from an anchor string and normalizes it.
func stripBriefAnchor(anchor string) string {
	a := strings.TrimSpace(anchor)
	if idx := strings.IndexByte(a, '#'); idx >= 0 {
		a = a[:idx]
	}
	if idx := strings.Index(a, ":L"); idx >= 0 {
		a = a[:idx]
	}
	return normalizeScopeName(a)
}
