/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/skills/graduate.go
 * Description: Evaluates procedural recipe/procedure entries for graduation into
 *              .agents/skills/rm-<slug>/SKILL.md files, renders deterministic
 *              idempotent Markdown, and synchronizes or lints graduated skills.
 *
 * Exported Types / Functions:
 * - GraduationResult:           Summarizes graduated, written, removed, and unchanged skill files.
 * - CountRecipeSteps():         Estimates the number of procedural steps in a recipe entry.
 * - EntryTitleLine():           Extracts a clean single-line title from an entry's text.
 * - SkillSlug():                Returns the deterministic "rm-<slug>" identifier for an entry.
 * - QualifiesForSkill():        Reports whether an entry qualifies for skill graduation and its slug.
 * - RenderSkillMarkdown():      Deterministically renders a qualifying entry into SKILL.md content.
 * - IsGeneratedByRepoMemory():  Reports whether a SKILL.md carries generated_by: repo-memory.
 * - GraduatedSkillSlug():       Returns the active graduated skill slug for an entry if present.
 * - SyncGraduatedSkills():      Writes qualifying skills and removes stale generated skills.
 * - CheckGraduatedSkillsLint(): Verifies on-disk generated skills match active op-log state.
 */

package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

var (
	numberedOrBulletLineRe = regexp.MustCompile(`^(?:\d+[.)]\s+|[-*+]\s+|(?i:step\s+\d+\s*[:.-]\s*))`) // Matches numbered or bulleted procedural step prefixes.
	nonSlugAlnumRe         = regexp.MustCompile(`[^a-z0-9]+`)                                          // Matches non-alphanumeric characters for slug normalization.
	entryIDBracketPrefixRe = regexp.MustCompile(`^\[[a-zA-Z0-9_-]+\]\s*`)                              // Matches leading bracketed entry ID prefixes ("[id] ").
	leadingHeaderBulletRe  = regexp.MustCompile(`^(?:#+\s*|[-*+]\s+|\d+[.)]\s+)+`)                     // Matches leading Markdown header or list markers.
	inlineStepSeparatorRe  = regexp.MustCompile(`(?:\s+->\s+|\s*;\s*|\s+&&\s+|\s+then\s+)`)            // Matches inline step separators in single-line recipes.
)

// GraduationResult summarizes a SyncGraduatedSkills run.
type GraduationResult struct {
	Graduated []string `json:"graduated"`           // Sorted list of qualifying "rm-<slug>" identifiers.
	Written   []string `json:"written,omitempty"`   // Repository-relative paths of newly created or updated SKILL.md files.
	Removed   []string `json:"removed,omitempty"`   // Repository-relative paths of stale generated SKILL.md files removed.
	Unchanged []string `json:"unchanged,omitempty"` // Repository-relative paths of qualifying SKILL.md files already up to date.
	DryRun    bool     `json:"dry_run,omitempty"`   // True when synchronization was evaluated without mutating disk.
}

// CountRecipeSteps estimates the number of procedural steps in a recipe/procedure entry text.
func CountRecipeSteps(text string) int {
	// --- Step 1: Count fenced command lines and Markdown list steps ---
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}

	lines := strings.Split(trimmed, "\n")
	listSteps := 0
	nonTitleLines := 0
	inFence := false
	fenceCmdLines := 0

	for idx, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
				fenceCmdLines++
			}
			continue
		}
		if line == "" {
			continue
		}
		if numberedOrBulletLineRe.MatchString(line) {
			listSteps++
		}
		if idx > 0 {
			nonTitleLines++
		}
	}

	// --- Step 2: Count extracted shell commands and compound sub-commands ---
	extractedCmds := ExtractRecipeCommands(trimmed)
	cmdStepCount := 0
	for _, cmd := range extractedCmds {
		parts := compoundSplitRe.Split(cmd, -1)
		validParts := 0
		for _, p := range parts {
			if NormalizeCommand(p) != "" {
				validParts++
			}
		}
		if validParts > 1 {
			cmdStepCount += validParts
		} else {
			cmdStepCount++
		}
	}

	// --- Step 3: Check inline step clauses and select the highest step estimate ---
	inlineClauseCount := 0
	if len(lines) == 1 {
		clauses := inlineStepSeparatorRe.Split(trimmed, -1)
		if len(clauses) > 1 {
			for _, c := range clauses {
				if strings.TrimSpace(c) != "" {
					inlineClauseCount++
				}
			}
		}
	}

	best := listSteps
	if fenceCmdLines > best {
		best = fenceCmdLines
	}
	if cmdStepCount > best {
		best = cmdStepCount
	}
	if inlineClauseCount > best {
		best = inlineClauseCount
	}
	if best == 0 && nonTitleLines >= 2 {
		best = nonTitleLines
	}
	if best == 0 {
		best = 1
	}
	return best
}

// normalizeSkillSlug converts raw into a bounded, lowercase "rm-<slug>" directory name.
func normalizeSkillSlug(raw string) string {
	// --- Step 1: Lowercase, strip "rm-" prefix, and collapse non-alphanumeric runs ---
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "rm-")
	s = nonSlugAlnumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	// --- Step 2: Truncate to at most 48 characters on a clean word boundary ---
	if len(s) > 48 {
		s = s[:48]
		if lastDash := strings.LastIndex(s, "-"); lastDash >= 16 {
			s = s[:lastDash]
		}
		s = strings.Trim(s, "-")
	}
	if s == "" {
		return "rm-procedure"
	}
	return "rm-" + s
}

// EntryTitleLine extracts a clean single-line title from an entry's text.
func EntryTitleLine(entry dossiers.Entry) string {
	// --- Step 1: Find the first non-empty, non-fence line and strip markers ---
	lines := strings.Split(strings.TrimSpace(entry.Text), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		line = entryIDBracketPrefixRe.ReplaceAllString(line, "")
		line = leadingHeaderBulletRe.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	// --- Step 2: Fall back to entry ID or generic title ---
	if strings.TrimSpace(entry.ID) != "" {
		return strings.TrimSpace(entry.ID)
	}
	return "Repository Procedure"
}

// SkillSlug returns the deterministic "rm-<slug>" identifier for a procedural entry.
func SkillSlug(entry dossiers.Entry) string {
	// --- Step 1: Honor explicit RenderAsSkill override if present ---
	if ras := strings.TrimSpace(entry.RenderAsSkill); ras != "" {
		return normalizeSkillSlug(ras)
	}

	// --- Step 2: Derive slug from the leading title clause or entry ID ---
	title := EntryTitleLine(entry)
	// Strip inline code backticks for slug derivation.
	title = strings.ReplaceAll(title, "`", " ")
	for _, sep := range []string{":", " — ", " – ", " - "} {
		if idx := strings.Index(title, sep); idx >= 4 {
			title = title[:idx]
			break
		}
	}
	slug := normalizeSkillSlug(title)
	if slug == "rm-procedure" && strings.TrimSpace(entry.ID) != "" {
		return normalizeSkillSlug(entry.ID)
	}
	return slug
}

// distinctSucceededConvCount counts distinct conversation IDs across outcomes and entry.Evidence.
func distinctSucceededConvCount(entry dossiers.Entry, outcomes *OutcomeState) (count int, hasOutcomeRecord bool) {
	// --- Step 1: Collect succeeded conversation IDs from OutcomeState ---
	seen := make(map[string]bool)
	if outcomes != nil && outcomes.Entries != nil {
		if eo, ok := outcomes.Entries[entry.ID]; ok && eo != nil {
			hasOutcomeRecord = true
			for _, c := range eo.SucceededConversations {
				trimmed := strings.TrimSpace(c)
				if trimmed != "" {
					seen[trimmed] = true
				}
			}
		}
	}

	// --- Step 2: Merge "conv:<id>" evidence tags from the dossier entry ---
	for _, ev := range entry.Evidence {
		trimmed := strings.TrimSpace(ev)
		if strings.HasPrefix(trimmed, "conv:") {
			id := strings.TrimSpace(strings.TrimPrefix(trimmed, "conv:"))
			if id != "" {
				seen[id] = true
			}
		}
	}
	return len(seen), hasOutcomeRecord
}

// QualifiesForSkill determines whether entry qualifies for graduation to .agents/skills/rm-<slug>/SKILL.md.
func QualifiesForSkill(entry dossiers.Entry, outcomes *OutcomeState) (bool, string) {
	// --- Step 1: Reject retired, stale_suspect, or non-procedural entries ---
	if entry.Retired || entry.StaleSuspect {
		return false, ""
	}
	if entry.Type != dossiers.EntryTypeRecipe && entry.Type != dossiers.EntryTypeProcedure {
		return false, ""
	}

	// --- Step 2: Check explicit RenderAsSkill or automatic multi-conversation graduation criteria ---
	// Path 1: Explicit render_as_skill attribute.
	if strings.TrimSpace(entry.RenderAsSkill) != "" {
		return true, SkillSlug(entry)
	}

	// Path 2: Automatic graduation (>= 2 steps, success_count >= 2 across >= 2 conversations, failure_count == 0).
	if CountRecipeSteps(entry.Text) < 2 {
		return false, ""
	}
	if entry.SuccessCount < 2 || entry.FailureCount != 0 {
		return false, ""
	}

	convCount, _ := distinctSucceededConvCount(entry, outcomes)
	if convCount < 2 {
		return false, ""
	}

	return true, SkillSlug(entry)
}

// sanitizeYAMLDescription normalizes whitespace and quotes s if it contains YAML indicator characters.
func sanitizeYAMLDescription(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = whitespaceRunRe.ReplaceAllString(strings.TrimSpace(s), " ")
	if strings.ContainsAny(s, `:#{}[]&*!|>'"%@`+"`") {
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}

// RenderSkillMarkdown deterministically renders a qualifying recipe/procedure entry into SKILL.md content.
// It uses zero wall-clock timestamps so regeneration is strictly idempotent.
func RenderSkillMarkdown(entry dossiers.Entry) (slug string, content string) {
	// --- Step 1: Build frontmatter metadata and description ---
	slug = SkillSlug(entry)
	title := EntryTitleLine(entry)

	desc := title
	if scope := strings.TrimSpace(entry.DossierScope); scope != "" {
		desc = fmt.Sprintf("%s (scope: %s)", title, scope)
	}
	if len(entry.Anchors) > 0 {
		anchorsCopy := append([]string(nil), entry.Anchors...)
		sort.Strings(anchorsCopy)
		desc = fmt.Sprintf("%s [anchors: %s]", desc, strings.Join(anchorsCopy, ", "))
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %s\n", slug))
	b.WriteString(fmt.Sprintf("description: %s\n", sanitizeYAMLDescription(desc)))
	b.WriteString("generated_by: repo-memory\n")
	b.WriteString(fmt.Sprintf("source_entry_id: %s\n", strings.TrimSpace(entry.ID)))
	b.WriteString("---\n\n")

	// --- Step 2: Render procedure body and sorted anchor list ---
	b.WriteString(fmt.Sprintf("# %s\n\n", title))
	b.WriteString("## Procedure\n\n")
	b.WriteString(strings.TrimSpace(entry.Text) + "\n\n")

	b.WriteString("## Anchors\n\n")
	if len(entry.Anchors) > 0 {
		anchorsCopy := append([]string(nil), entry.Anchors...)
		sort.Strings(anchorsCopy)
		for _, a := range anchorsCopy {
			if trimmed := strings.TrimSpace(a); trimmed != "" {
				b.WriteString(fmt.Sprintf("- `%s`\n", trimmed))
			}
		}
	} else {
		b.WriteString("- (none)\n")
	}
	b.WriteString("\n")

	// --- Step 3: Render provenance metadata footer ---
	b.WriteString("## Provenance\n\n")
	b.WriteString(fmt.Sprintf("- Entry ID: `%s`\n", strings.TrimSpace(entry.ID)))
	if scope := strings.TrimSpace(entry.DossierScope); scope != "" {
		b.WriteString(fmt.Sprintf("- Dossier Scope: `%s`\n", scope))
	}
	if src := strings.TrimSpace(entry.Source); src != "" {
		b.WriteString(fmt.Sprintf("- Source: `%s`\n", src))
	}
	if ver := strings.TrimSpace(entry.Verified); ver != "" {
		b.WriteString(fmt.Sprintf("- Verified: `%s`\n", ver))
	}
	if len(entry.Evidence) > 0 {
		evCopy := append([]string(nil), entry.Evidence...)
		sort.Strings(evCopy)
		b.WriteString(fmt.Sprintf("- Evidence: `%s`\n", strings.Join(evCopy, ", ")))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Generated from [%s] by repo-memory. Edit the dossier entry or op log, not this file.\n", strings.TrimSpace(entry.ID)))

	return slug, b.String()
}

// IsGeneratedByRepoMemory checks whether a SKILL.md file content carries `generated_by: repo-memory` in its YAML frontmatter.
func IsGeneratedByRepoMemory(content string) bool {
	// --- Step 1: Scan leading YAML frontmatter for generated_by: repo-memory ---
	trimmed := strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, "generated_by:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "generated_by:"))
			val = strings.Trim(val, `"'`)
			if val == "repo-memory" {
				return true
			}
		}
	}
	return false
}

// extractSourceEntryIDFromSkill extracts the `source_entry_id` field from a SKILL.md YAML frontmatter block.
func extractSourceEntryIDFromSkill(content string) string {
	// --- Step 1: Scan leading YAML frontmatter for source_entry_id ---
	trimmed := strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if strings.HasPrefix(line, "source_entry_id:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "source_entry_id:"))
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}

// GraduatedSkillSlug returns the graduated skill slug ("rm-<slug>") for entry when it is an active,
// non-stale_suspect recipe/procedure whose graduated SKILL.md exists on disk under workspaceRoot
// or qualifies directly via RenderAsSkill / SuccessCount.
func GraduatedSkillSlug(entry dossiers.Entry, workspaceRoot string) string {
	// --- Step 1: Validate that entry is an active, non-stale procedural entry ---
	if entry.Retired || entry.StaleSuspect {
		return ""
	}
	if entry.Type != dossiers.EntryTypeRecipe && entry.Type != dossiers.EntryTypeProcedure {
		return ""
	}

	// --- Step 2: Check on-disk .agents/skills/rm-*/SKILL.md when workspaceRoot is provided ---
	slug := SkillSlug(entry)
	if strings.TrimSpace(workspaceRoot) != "" {
		skillPath := filepath.Join(workspaceRoot, ".agents", "skills", slug, "SKILL.md")
		if data, err := os.ReadFile(skillPath); err == nil {
			if IsGeneratedByRepoMemory(string(data)) {
				return slug
			}
		}
		// Also check if any .agents/skills/rm-*/SKILL.md references source_entry_id: <entry.ID>.
		skillsRoot := filepath.Join(workspaceRoot, ".agents", "skills")
		if dirEntries, err := os.ReadDir(skillsRoot); err == nil {
			for _, de := range dirEntries {
				if !de.IsDir() || !strings.HasPrefix(de.Name(), "rm-") {
					continue
				}
				candidate := filepath.Join(skillsRoot, de.Name(), "SKILL.md")
				if data, readErr := os.ReadFile(candidate); readErr == nil {
					if IsGeneratedByRepoMemory(string(data)) && extractSourceEntryIDFromSkill(string(data)) == entry.ID {
						return de.Name()
					}
				}
			}
		}
		return ""
	}

	// --- Step 3: Fall back to in-memory qualification check when workspaceRoot is empty ---
	if ok, qSlug := QualifiesForSkill(entry, nil); ok {
		return qSlug
	}
	return ""
}

// loadAllProceduralEntriesForSync loads active recipe and procedure entries across .memory/ops/, .memory/local/ops/, and index.db.
func loadAllProceduralEntriesForSync(ctx context.Context, repoRoot, dataDir, repoID string) ([]dossiers.Entry, error) {
	// --- Step 1: Define helper to project an ops directory and filter active procedural entries ---
	byID := make(map[string]dossiers.Entry)
	retiredIDs := make(map[string]bool)

	projectDir := func(opsDir string) error {
		info, err := os.Stat(opsDir)
		if err != nil || !info.IsDir() {
			return nil
		}
		projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
		if err != nil {
			return err
		}
		for _, d := range projected {
			if d == nil {
				continue
			}
			for _, h := range d.History {
				retiredIDs[h.ID] = true
				delete(byID, h.ID)
			}
			for _, e := range d.Entries {
				if e.Retired {
					retiredIDs[e.ID] = true
					delete(byID, e.ID)
					continue
				}
				if e.Type == dossiers.EntryTypeRecipe || e.Type == dossiers.EntryTypeProcedure {
					byID[e.ID] = e
				}
			}
		}
		return nil
	}

	// --- Step 2: Project shared .memory/ops and any local .memory/local/ops directories ---
	if strings.TrimSpace(repoRoot) != "" {
		if err := projectDir(filepath.Join(repoRoot, ".memory", "ops")); err != nil {
			return nil, fmt.Errorf("projecting .memory/ops: %w", err)
		}
		localOpsRoot := filepath.Join(repoRoot, ".memory", "local", "ops")
		if dirEntries, err := os.ReadDir(localOpsRoot); err == nil {
			for _, de := range dirEntries {
				if de.IsDir() {
					_ = projectDir(filepath.Join(localOpsRoot, de.Name()))
				}
			}
		}
	}

	// --- Step 3: Supplement with any non-retired procedural entries from the SQLite index ---
	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		idxPath := index.IndexPath(dataDir, repoID)
		if _, err := os.Stat(idxPath); err == nil {
			if idx, idxErr := index.NewIndex(idxPath); idxErr == nil {
				if idxEntries, listErr := idx.ListProceduralEntries(ctx); listErr == nil {
					for _, ie := range idxEntries {
						if retiredIDs[ie.ID] {
							continue
						}
						if ie.Type != dossiers.EntryTypeRecipe && ie.Type != dossiers.EntryTypeProcedure {
							continue
						}
						if _, exists := byID[ie.ID]; !exists {
							byID[ie.ID] = ie
						}
					}
				}
				_ = idx.Close()
			}
		}
	}

	// --- Step 4: Sort deduplicated entries deterministically by ID ---
	entries := make([]dossiers.Entry, 0, len(byID))
	for _, e := range byID {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
	return entries, nil
}

// SyncGraduatedSkills projects active procedural entries, writes/updates qualifying .agents/skills/rm-<slug>/SKILL.md
// files, and removes any stale repo-memory-generated skills that no longer qualify.
// Hand-authored skills without `generated_by: repo-memory` are never modified or removed.
func SyncGraduatedSkills(ctx context.Context, repoRoot, dataDir, repoID string, dryRun bool) (*GraduationResult, error) {
	// --- Step 1: Load active procedural entries and outcome state ---
	if ctx == nil {
		ctx = context.Background()
	}
	res := &GraduationResult{
		Graduated: []string{},
		DryRun:    dryRun,
	}
	if strings.TrimSpace(repoRoot) == "" {
		return res, nil
	}

	entries, err := loadAllProceduralEntriesForSync(ctx, repoRoot, dataDir, repoID)
	if err != nil {
		return nil, err
	}

	outcomes, _ := LoadOutcomeState(dataDir, repoID)

	// --- Step 2: Render Markdown for all qualifying procedural entries ---
	type renderedSkill struct {
		slug    string // Target "rm-<slug>" directory name.
		content string // Rendered SKILL.md content.
		entryID string // Source dossier entry ID.
	}
	qualifyingBySlug := make(map[string]renderedSkill)
	var orderedSlugs []string

	for _, entry := range entries {
		ok, _ := QualifiesForSkill(entry, outcomes)
		if !ok {
			continue
		}
		slug, content := RenderSkillMarkdown(entry)
		if _, exists := qualifyingBySlug[slug]; !exists {
			orderedSlugs = append(orderedSlugs, slug)
		}
		qualifyingBySlug[slug] = renderedSkill{
			slug:    slug,
			content: content,
			entryID: entry.ID,
		}
	}
	sort.Strings(orderedSlugs)
	res.Graduated = append(res.Graduated, orderedSlugs...)

	skillsRoot := filepath.Join(repoRoot, ".agents", "skills")

	// --- Step 3: Write or update qualifying SKILL.md files on disk ---
	for _, slug := range orderedSlugs {
		rs := qualifyingBySlug[slug]
		relSkillPath := filepath.ToSlash(filepath.Join(".agents", "skills", slug, "SKILL.md"))
		fullSkillPath := filepath.Join(skillsRoot, slug, "SKILL.md")

		existingBytes, readErr := os.ReadFile(fullSkillPath)
		if readErr == nil {
			existingContent := string(existingBytes)
			if !IsGeneratedByRepoMemory(existingContent) {
				// Never overwrite a hand-authored skill without generated_by: repo-memory.
				continue
			}
			if existingContent == rs.content {
				res.Unchanged = append(res.Unchanged, relSkillPath)
				continue
			}
		}

		res.Written = append(res.Written, relSkillPath)
		if !dryRun {
			if err := os.MkdirAll(filepath.Dir(fullSkillPath), 0o755); err != nil {
				return nil, fmt.Errorf("creating skill directory %s: %w", filepath.Dir(fullSkillPath), err)
			}
			if err := os.WriteFile(fullSkillPath, []byte(rs.content), 0o644); err != nil {
				return nil, fmt.Errorf("writing skill file %s: %w", fullSkillPath, err)
			}
		}
	}

	// --- Step 4: Remove stale repo-memory-generated skills that no longer qualify ---
	if dirEntries, err := os.ReadDir(skillsRoot); err == nil {
		for _, de := range dirEntries {
			if !de.IsDir() {
				continue
			}
			dirName := de.Name()
			if !strings.HasPrefix(dirName, "rm-") {
				continue
			}
			if _, stillQualifies := qualifyingBySlug[dirName]; stillQualifies {
				continue
			}
			fullSkillPath := filepath.Join(skillsRoot, dirName, "SKILL.md")
			existingBytes, readErr := os.ReadFile(fullSkillPath)
			if readErr != nil {
				continue
			}
			if !IsGeneratedByRepoMemory(string(existingBytes)) {
				continue
			}
			relSkillPath := filepath.ToSlash(filepath.Join(".agents", "skills", dirName, "SKILL.md"))
			res.Removed = append(res.Removed, relSkillPath)
			if !dryRun {
				_ = os.Remove(fullSkillPath)
				_ = os.Remove(filepath.Join(skillsRoot, dirName))
			}
		}
	}

	return res, nil
}

// CheckGraduatedSkillsLint verifies that every .agents/skills/rm-*/SKILL.md carrying
// `generated_by: repo-memory` still qualifies for graduation and matches deterministic
// regeneration from .memory/ops/. Hand-authored skills without `generated_by: repo-memory`
// are ignored.
func CheckGraduatedSkillsLint(ctx context.Context, repoRoot, dataDir, repoID string) ([]string, error) {
	// --- Step 1: Discover existing repo-memory-generated SKILL.md files on disk ---
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(repoRoot) == "" {
		return nil, nil
	}

	skillsRoot := filepath.Join(repoRoot, ".agents", "skills")
	dirEntries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, nil
	}

	type generatedSkillOnDisk struct {
		dirName string // Skill directory name ("rm-<slug>").
		relPath string // Repository-relative SKILL.md path.
		content string // Raw file contents on disk.
	}
	var onDisk []generatedSkillOnDisk
	for _, de := range dirEntries {
		if !de.IsDir() {
			continue
		}
		dirName := de.Name()
		if !strings.HasPrefix(dirName, "rm-") {
			continue
		}
		fullSkillPath := filepath.Join(skillsRoot, dirName, "SKILL.md")
		existingBytes, readErr := os.ReadFile(fullSkillPath)
		if readErr != nil {
			continue
		}
		existingContent := string(existingBytes)
		if !IsGeneratedByRepoMemory(existingContent) {
			continue
		}
		onDisk = append(onDisk, generatedSkillOnDisk{
			dirName: dirName,
			relPath: filepath.ToSlash(filepath.Join(".agents", "skills", dirName, "SKILL.md")),
			content: existingContent,
		})
	}
	if len(onDisk) == 0 {
		return nil, nil
	}

	// --- Step 2: Project active procedural entries and render expected SKILL.md contents ---
	entries, err := loadAllProceduralEntriesForSync(ctx, repoRoot, dataDir, repoID)
	if err != nil {
		return nil, err
	}
	outcomes, _ := LoadOutcomeState(dataDir, repoID)

	qualifyingBySlug := make(map[string]string)
	for _, entry := range entries {
		ok, _ := QualifiesForSkill(entry, outcomes)
		if !ok {
			continue
		}
		slug, content := RenderSkillMarkdown(entry)
		qualifyingBySlug[slug] = content
	}

	// --- Step 3: Compare on-disk files against expected contents and collect lint warnings ---
	var warnings []string
	for _, diskSkill := range onDisk {
		expectedContent, ok := qualifyingBySlug[diskSkill.dirName]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Graduated skill %s is stale or its source entry no longer qualifies; run 'repo-memory promote-skills'.", diskSkill.relPath))
		} else if diskSkill.content != expectedContent {
			warnings = append(warnings, fmt.Sprintf("Graduated skill %s is out of sync with .memory/ops/; run 'repo-memory promote-skills'.", diskSkill.relPath))
		}
	}

	sort.Strings(warnings)
	return warnings, nil
}
