/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/inbox/free_harvest.go
 * Description: Implements Stage 1b deterministic zero-token harvesting of
 *              anchored candidate notes from episode checkpoints and conversation
 *              markdown artifacts.
 *
 * Exported Types / Functions:
 * - HarvestCheckpointsAndArtifacts(): Extracts deduplicated candidate inbox notes from checkpoints and artifacts.
 */

package inbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

// maxFreeHarvestItemsPerNote bounds the number of candidate NoteItems extracted into a single harvested note.
const maxFreeHarvestItemsPerNote = 6

var (
	// harvestBacktickRe extracts inline backtick-quoted code spans for anchor candidate inspection.
	harvestBacktickRe = regexp.MustCompile("`([^`]+)`")
	// harvestNumberedBulletRe matches numbered markdown list items (e.g. "1. " or "2) ").
	harvestNumberedBulletRe = regexp.MustCompile(`^\d+[.)]\s+(.*)$`)
	// harvestBoldPrefixRe matches a leading bold title prefix followed by a colon or dash separator.
	harvestBoldPrefixRe = regexp.MustCompile(`^\*\*([^*]{3,80})\*\*\s*[:—-]\s*(.+)$`)
	// harvestPathTokenRe matches unquoted repository-like file paths and known source file extensions in prose.
	harvestPathTokenRe = regexp.MustCompile(`(?:^|[\s(\[,'"])([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+(?:[#:][A-Za-z0-9_.-]+)?|[A-Za-z0-9_-]+\.(?:go|md|yaml|yml|json|toml|sh|txt|mod|sum|sql|py|js|ts|c|h|cpp|rs|proto|mk)(?:[#:][A-Za-z0-9_.-]+)?)`)
)

// HarvestCheckpointsAndArtifacts implements Stage 1b deterministic zero-token harvest
// (SPEC §6.3.2 / design-v2 §5.3): extracts structured candidate inbox notes
// (`source: "checkpoint"` and `source: "artifact"`) from episode Checkpoints and
// `<dataDir>/<repoID>/evidence/artifacts/<convID>/**/*.md` anchored to repository paths,
// deduplicated idempotently across ticks.
func HarvestCheckpointsAndArtifacts(
	ctx context.Context,
	dataDir, repoID, repoRoot string,
	epStore *episode.Store,
	inboxStore *Store,
) (int, error) {
	// --- Step 1: Initialize default context and stores when omitted by caller ---
	if ctx == nil {
		ctx = context.Background()
	}
	if inboxStore == nil {
		inboxStore = NewStore(dataDir, repoID)
	}
	if epStore == nil && dataDir != "" && repoID != "" {
		epStore = episode.NewStore(dataDir, repoID)
	}

	notesCreated := 0

	// --- Step 2: Harvest Episode CHECKPOINT Summaries (source: "checkpoint") ---
	if epStore != nil {
		n, err := harvestEpisodeCheckpoints(ctx, repoRoot, epStore, inboxStore)
		if err != nil {
			return notesCreated, err
		}
		notesCreated += n
	}

	// --- Step 3: Harvest Conversation Artifacts (source: "artifact") ---
	if dataDir != "" && repoID != "" {
		n, err := harvestConversationArtifacts(ctx, dataDir, repoID, repoRoot, inboxStore)
		if err != nil {
			return notesCreated, err
		}
		notesCreated += n
	}

	return notesCreated, nil
}

// isValidRepoAnchor reports whether cand has valid anchor syntax and resolves to an existing path in repoRoot.
func isValidRepoAnchor(repoRoot, cand string) bool {
	// --- Step 1: Validate anchor syntax and strip trailing slashes ---
	cand = strings.TrimSpace(cand)
	cand = strings.TrimSuffix(cand, "/")
	if cand == "" || !seed.IsValidAnchorPathSyntax(cand) {
		return false
	}

	// --- Step 2: Strip optional #symbol or :line suffixes to isolate the file path ---
	pathPart := cand
	if idx := strings.IndexAny(pathPart, "#:"); idx >= 0 {
		pathPart = strings.TrimSuffix(pathPart[:idx], "/")
	}
	pathPart = strings.TrimSpace(pathPart)
	if pathPart == "" {
		return false
	}

	// --- Step 3: Verify filesystem existence under repoRoot when provided ---
	if repoRoot != "" {
		fullPath := filepath.Join(repoRoot, filepath.FromSlash(pathPart))
		if _, err := os.Stat(fullPath); err != nil {
			return false
		}
	}
	return true
}

// harvestEpisodeCheckpoints scans stored episode headers for checkpoint sections and writes
// deduplicated checkpoint notes for bullets anchored to valid repository paths.
func harvestEpisodeCheckpoints(
	ctx context.Context,
	repoRoot string,
	epStore *episode.Store,
	inboxStore *Store,
) (int, error) {
	// --- Step 1: Load all episode headers from the episode store ---
	headers, err := epStore.ListHeaders()
	if err != nil {
		return 0, err
	}

	sectionOrder := []string{
		"Model Knowledge",
		"User Knowledge",
		"Work Accomplished",
	}

	notesCreated := 0
	for _, h := range headers {
		if err := ctx.Err(); err != nil {
			return notesCreated, err
		}
		if h == nil || len(h.Checkpoints) == 0 {
			continue
		}

		// --- Step 2: Compute a content hash across target checkpoint sections for idempotency ---
		var hashBuf strings.Builder
		for _, sec := range sectionOrder {
			txt := strings.TrimSpace(h.Checkpoints[sec])
			if txt != "" {
				hashBuf.WriteString(sec)
				hashBuf.WriteByte('\n')
				hashBuf.WriteString(txt)
				hashBuf.WriteByte('\n')
			}
		}
		if hashBuf.Len() == 0 {
			continue
		}

		sum := sha256.Sum256([]byte(hashBuf.String()))
		sha256Hex := hex.EncodeToString(sum[:])
		dedupKey := fmt.Sprintf("checkpoint:%s:%s", h.SessionID, sha256Hex[:16])

		exists, err := inboxStore.HasDedupKey(dedupKey)
		if err != nil {
			return notesCreated, err
		}
		if exists {
			continue
		}

		// --- Step 3: Extract anchored candidate items from markdown bullets in each section ---
		fallbackAnchors := extractCandidateAnchors(repoRoot, h.Checkpoints["Files and Code"])

		var items []NoteItem
		seenBullet := make(map[string]bool)

		for _, sec := range sectionOrder {
			secText := strings.TrimSpace(h.Checkpoints[sec])
			if secText == "" {
				continue
			}
			bullets := extractMarkdownBullets(secText)
			for _, bullet := range bullets {
				bullet = strings.TrimSpace(bullet)
				if bullet == "" || seed.IsJunkText(bullet) {
					continue
				}
				if sec == "Work Accomplished" && !isQualifyingWorkAccomplishedBullet(bullet) {
					continue
				}

				normBullet := strings.ToLower(strings.Join(strings.Fields(bullet), " "))
				if seenBullet[normBullet] {
					continue
				}

				anchors := extractCandidateAnchors(repoRoot, bullet)
				if len(anchors) == 0 && len(fallbackAnchors) > 0 && hasModuleRuleOrDecisionCue(bullet) {
					anchors = append([]string(nil), fallbackAnchors...)
				}
				if len(anchors) == 0 {
					continue
				}

				seenBullet[normBullet] = true
				item := buildNoteItemFromBullet(
					repoRoot,
					bullet,
					anchors,
					fmt.Sprintf("checkpoint (%s)", sec),
				)
				items = append(items, item)
				if len(items) >= maxFreeHarvestItemsPerNote {
					break
				}
			}
			if len(items) >= maxFreeHarvestItemsPerNote {
				break
			}
		}

		if len(items) == 0 {
			continue
		}

		// --- Step 4: Persist the harvested checkpoint note to the inbox store ---
		note := &Note{
			Source:         SourceCheckpoint,
			ConversationID: h.SessionID,
			Model:          h.Model,
			HarnessVersion: h.HarnessVersion,
			Branch:         h.Branch,
			HeadCommit:     nonEmpty(h.HeadEnd, h.HeadStart),
			Items:          items,
			DedupKey:       dedupKey,
		}
		if _, err := inboxStore.Write(note); err != nil {
			return notesCreated, err
		}
		notesCreated++
	}

	return notesCreated, nil
}

// harvestConversationArtifacts walks <dataDir>/<repoID>/evidence/artifacts/<convID>/ for markdown
// artifacts and writes deduplicated candidate notes for items anchored to repository paths.
func harvestConversationArtifacts(
	ctx context.Context,
	dataDir, repoID, repoRoot string,
	inboxStore *Store,
) (int, error) {
	// --- Step 1: Enumerate conversation directories under evidence/artifacts ---
	artifactsRoot := filepath.Join(dataDir, repoID, "evidence", "artifacts")
	convEntries, err := os.ReadDir(artifactsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	sort.Slice(convEntries, func(i, j int) bool {
		return convEntries[i].Name() < convEntries[i].Name()
	})

	notesCreated := 0
	for _, convEntry := range convEntries {
		if err := ctx.Err(); err != nil {
			return notesCreated, err
		}
		if !convEntry.IsDir() {
			continue
		}
		convID := convEntry.Name()
		if strings.HasPrefix(convID, ".") {
			continue
		}

		// --- Step 2: Collect eligible regular .md files within the conversation directory ---
		convDir := filepath.Join(artifactsRoot, convID)
		var mdFiles []string
		walkErr := filepath.WalkDir(convDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if name == ".system_generated" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(name, ".") ||
				strings.HasSuffix(name, ".metadata.json") ||
				strings.HasSuffix(name, ".resolved") ||
				strings.HasSuffix(name, ".md.resolved") {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(name), ".md") {
				return nil
			}
			info, infoErr := d.Info()
			if infoErr != nil || !info.Mode().IsRegular() {
				return nil
			}
			mdFiles = append(mdFiles, path)
			return nil
		})
		if walkErr != nil {
			return notesCreated, walkErr
		}

		sort.Strings(mdFiles)

		// --- Step 3: Check SHA-256 deduplication key and extract anchored items per file ---
		for _, fullPath := range mdFiles {
			if err := ctx.Err(); err != nil {
				return notesCreated, err
			}
			relPath, err := filepath.Rel(convDir, fullPath)
			if err != nil {
				continue
			}
			slashRel := filepath.ToSlash(relPath)

			contentBytes, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			content := string(contentBytes)
			if strings.TrimSpace(content) == "" {
				continue
			}

			sum := sha256.Sum256(contentBytes)
			sha256Hex := hex.EncodeToString(sum[:])
			dedupKey := fmt.Sprintf("artifact:%s:%s:%s", convID, slashRel, sha256Hex[:16])

			exists, err := inboxStore.HasDedupKey(dedupKey)
			if err != nil {
				return notesCreated, err
			}
			if exists {
				continue
			}

			evidenceRef := fmt.Sprintf("artifact:%s", slashRel)
			items := extractItemsFromArtifact(repoRoot, slashRel, content, evidenceRef)
			if len(items) == 0 {
				continue
			}
			if len(items) > maxFreeHarvestItemsPerNote {
				items = items[:maxFreeHarvestItemsPerNote]
			}

			// --- Step 4: Write the harvested artifact note to the inbox store ---
			note := &Note{
				Source:         SourceArtifact,
				ConversationID: convID,
				Items:          items,
				DedupKey:       dedupKey,
			}
			if _, err := inboxStore.Write(note); err != nil {
				return notesCreated, err
			}
			notesCreated++
		}
	}

	return notesCreated, nil
}

// extractItemsFromArtifact extracts up to maxFreeHarvestItemsPerNote anchored candidate items
// from a markdown artifact using structured doc parsing with a bullet-level fallback.
func extractItemsFromArtifact(repoRoot, slashRel, content, evidenceRef string) []NoteItem {
	var items []NoteItem
	seenTitle := make(map[string]bool)

	// --- Step 1: Attempt structured extraction via seed.ParseDocToOps ---
	docOps := seed.ParseDocToOps(
		slashRel,
		content,
		"",
		time.Now().UTC(),
		nil,
		func(p string) bool { return isValidRepoAnchor(repoRoot, p) },
	)

	// --- Step 2: Filter and convert structured ops that have valid repository anchors ---
	for _, op := range docOps {
		text, _ := op.Fields["text"].(string)
		text = strings.TrimSpace(text)
		if text == "" || seed.IsJunkText(text) {
			continue
		}

		var rawAnchors []string
		switch a := op.Fields["anchors"].(type) {
		case []string:
			rawAnchors = a
		case []any:
			for _, v := range a {
				if s, ok := v.(string); ok {
					rawAnchors = append(rawAnchors, s)
				}
			}
		}

		var validAnchors []string
		seenAnchor := make(map[string]bool)
		for _, a := range rawAnchors {
			a = strings.TrimSuffix(strings.TrimSpace(a), "/")
			if isValidRepoAnchor(repoRoot, a) && !seenAnchor[a] {
				seenAnchor[a] = true
				validAnchors = append(validAnchors, a)
			}
		}
		if len(validAnchors) == 0 {
			validAnchors = extractCandidateAnchors(repoRoot, text)
		}
		if len(validAnchors) == 0 {
			continue
		}

		entryTypeStr, _ := op.Fields["entry_type"].(string)
		entryType := dossiers.EntryType(entryTypeStr)
		if entryType == "" {
			entryType = classifyBulletType(text)
		} else {
			// If keyword cues in the bullet strongly indicate ruled-out, invariant, failure-signature, or gotcha,
			// honour classifyBulletType when ParseDocToOps defaulted to convention/decision.
			kwType := classifyBulletType(text)
			if kwType != dossiers.EntryTypeDecision && (entryType == dossiers.EntryTypeConvention || entryType == dossiers.EntryTypeDecision) {
				entryType = kwType
			}
		}

		title, why, do := splitBulletFields(text, entryType)
		if title == "" || seed.IsJunkText(title) {
			continue
		}

		normKey := strings.ToLower(title)
		if seenTitle[normKey] {
			continue
		}
		seenTitle[normKey] = true

		items = append(items, NoteItem{
			Type:         entryType,
			Title:        title,
			Why:          why,
			Do:           do,
			Anchors:      validAnchors,
			AnchorChecks: CheckAnchors(repoRoot, validAnchors),
			Evidence:     evidenceRef,
			Scope:        op.DossierScope,
		})
		if len(items) >= maxFreeHarvestItemsPerNote {
			return items
		}
	}

	if len(items) > 0 {
		return items
	}

	// --- Step 3: Fall back to anchored markdown bullet extraction when structured ops yield 0 items ---
	bullets := extractMarkdownBullets(content)
	for _, bullet := range bullets {
		bullet = strings.TrimSpace(bullet)
		if bullet == "" || seed.IsJunkText(bullet) {
			continue
		}
		anchors := extractCandidateAnchors(repoRoot, bullet)
		if len(anchors) == 0 {
			continue
		}
		item := buildNoteItemFromBullet(repoRoot, bullet, anchors, evidenceRef)
		if item.Title == "" || seed.IsJunkText(item.Title) {
			continue
		}
		normKey := strings.ToLower(item.Title)
		if seenTitle[normKey] {
			continue
		}
		seenTitle[normKey] = true
		items = append(items, item)
		if len(items) >= maxFreeHarvestItemsPerNote {
			break
		}
	}

	return items
}

// buildNoteItemFromBullet constructs a NoteItem from a markdown bullet, its verified anchors, and an evidence reference.
func buildNoteItemFromBullet(repoRoot, bullet string, anchors []string, evidenceRef string) NoteItem {
	entryType := classifyBulletType(bullet)
	title, why, do := splitBulletFields(bullet, entryType)
	return NoteItem{
		Type:         entryType,
		Title:        title,
		Why:          why,
		Do:           do,
		Anchors:      anchors,
		AnchorChecks: CheckAnchors(repoRoot, anchors),
		Evidence:     evidenceRef,
	}
}

// classifyBulletType infers a dossier EntryType from lexical cues in a bullet string.
func classifyBulletType(bullet string) dossiers.EntryType {
	// --- Step 1: Check for ruled-out and invariant cues ---
	lower := strings.ToLower(bullet)

	for _, kw := range []string{"ruled out", "failed", "did not work", "abandoned", "avoid"} {
		if strings.Contains(lower, kw) {
			return dossiers.EntryTypeRuledOut
		}
	}
	for _, kw := range []string{"must ", "never ", "always ", "invariant"} {
		if strings.Contains(lower, kw) {
			return dossiers.EntryTypeInvariant
		}
	}

	// --- Step 2: Check for failure-signature and gotcha cues, defaulting to decision ---
	for _, kw := range []string{"error", "panic", "symptom", "fix:"} {
		if strings.Contains(lower, kw) {
			return dossiers.EntryTypeFailureSignature
		}
	}
	for _, kw := range []string{"gotcha", "caveat", "quirk", "note:"} {
		if strings.Contains(lower, kw) {
			return dossiers.EntryTypeGotcha
		}
	}
	return dossiers.EntryTypeDecision
}

// isQualifyingWorkAccomplishedBullet reports whether a bullet from "Work Accomplished" contains
// durable architectural, invariant, failure, or decision cues rather than routine status narration.
func isQualifyingWorkAccomplishedBullet(bullet string) bool {
	// --- Step 1: Normalize bullet text and scan for durable engineering cue words ---
	lower := strings.ToLower(bullet)
	cues := []string{
		"ruled out", "failed", "did not work", "abandoned", "avoid",
		"must ", "never ", "always ", "invariant",
		"error", "panic", "symptom", "fix:",
		"gotcha", "caveat", "quirk", "note:",
		"decided", "decision", "chose", "because", "instead of", "rather than",
	}

	// --- Step 2: Return true on the first matching cue ---
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

// hasModuleRuleOrDecisionCue reports whether a bullet carries rule, contract, or decision keywords
// sufficient to inherit fallback file anchors from the checkpoint's "Files and Code" section.
func hasModuleRuleOrDecisionCue(bullet string) bool {
	// --- Step 1: Normalize bullet text and define qualifying rule/decision cues ---
	lower := strings.ToLower(bullet)
	cues := []string{
		"ruled out", "failed", "did not work", "abandoned", "avoid",
		"must ", "never ", "always ", "invariant",
		"error", "panic", "symptom", "fix:",
		"gotcha", "caveat", "quirk", "note:",
		"decided", "decision", "chose", "because", "instead of", "rather than",
		"require", "requires", "enforces", "contract", "convention",
	}

	// --- Step 2: Check whether any cue appears in the bullet ---
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

// splitBulletFields decomposes a raw markdown bullet into Title, Why, and Do strings based on
// bold prefixes, punctuation separators, and entry type requirements.
func splitBulletFields(bullet string, entryType dossiers.EntryType) (title, why, do string) {
	// --- Step 1: Collapse whitespace and handle empty input ---
	cleaned := strings.Join(strings.Fields(strings.TrimSpace(bullet)), " ")
	if cleaned == "" {
		return "", "", ""
	}

	why = cleaned
	rawTitle := cleaned
	var bodyAfterPrefix string

	// --- Step 2: Extract a concise title prefix from bold spans or punctuation separators ---
	if m := harvestBoldPrefixRe.FindStringSubmatch(cleaned); len(m) == 3 {
		rawTitle = strings.TrimSpace(m[1])
		bodyAfterPrefix = strings.TrimSpace(m[2])
	} else {
		for _, sep := range []string{": ", " — ", " - "} {
			if idx := strings.Index(cleaned, sep); idx > 3 && idx <= 80 {
				rawTitle = strings.TrimSpace(cleaned[:idx])
				bodyAfterPrefix = strings.TrimSpace(cleaned[idx+len(sep):])
				break
			}
		}
	}

	rawTitle = strings.Trim(rawTitle, "*_ \t")
	if len(rawTitle) > 80 {
		rawTitle = strings.TrimSpace(rawTitle[:77]) + "..."
	}
	title = rawTitle

	// --- Step 3: Populate the actionable Do field for procedural and rule-bearing entry types ---
	switch entryType {
	case dossiers.EntryTypeRuledOut,
		dossiers.EntryTypeFailureSignature,
		dossiers.EntryTypeInvariant,
		dossiers.EntryTypeProcedure,
		dossiers.EntryTypeRecipe:
		if bodyAfterPrefix != "" {
			do = bodyAfterPrefix
		} else {
			do = cleaned
		}
	}

	return title, why, do
}

// extractMarkdownBullets parses top-level and indented markdown list items while ignoring fenced code blocks.
func extractMarkdownBullets(text string) []string {
	// --- Step 1: Split text into lines and track fenced code block state ---
	lines := strings.Split(strings.TrimPrefix(text, "\uFEFF"), "\n")
	var bullets []string
	inCodeFence := false

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence || trimmed == "" {
			continue
		}

		// --- Step 2: Parse the start of a bullet item and join indented continuation lines ---
		bulletText, ok := parseSingleBulletLine(trimmed)
		if !ok {
			continue
		}

		for i+1 < len(lines) {
			nextRaw := lines[i+1]
			nextTrim := strings.TrimSpace(nextRaw)
			if nextTrim == "" ||
				strings.HasPrefix(nextTrim, "#") ||
				strings.HasPrefix(nextTrim, "```") ||
				strings.HasPrefix(nextTrim, "~~~") ||
				strings.HasPrefix(nextTrim, "|") {
				break
			}
			if _, isNextBullet := parseSingleBulletLine(nextTrim); isNextBullet {
				break
			}
			if strings.HasPrefix(nextRaw, "  ") || strings.HasPrefix(nextRaw, "\t") {
				bulletText += " " + nextTrim
				i++
			} else {
				break
			}
		}

		// --- Step 3: Collapse internal whitespace and append non-empty bullet text ---
		bulletText = strings.Join(strings.Fields(bulletText), " ")
		if bulletText != "" {
			bullets = append(bullets, bulletText)
		}
	}

	return bullets
}

// parseSingleBulletLine strips unordered (-, *, +), checkbox ([ ], [x]), or numbered list prefixes from a line.
func parseSingleBulletLine(trimmed string) (string, bool) {
	// --- Step 1: Check unordered list markers and strip optional task-list checkboxes ---
	for _, pfx := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, pfx) {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, pfx))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "[ ]"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "[x]"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "[X]"))
			return rest, rest != ""
		}
	}
	// --- Step 2: Check numbered list markers ---
	if m := harvestNumberedBulletRe.FindStringSubmatch(trimmed); len(m) == 2 {
		rest := strings.TrimSpace(m[1])
		return rest, rest != ""
	}
	return "", false
}

// extractCandidateAnchors extracts deduplicated, filesystem-verified repository anchors from backtick spans and path tokens in text.
func extractCandidateAnchors(repoRoot, text string) []string {
	// --- Step 1: Define normalization and validation helper for candidate tokens ---
	if strings.TrimSpace(text) == "" {
		return nil
	}

	var anchors []string
	seen := make(map[string]bool)

	addIfValid := func(raw string) {
		cand := strings.TrimSpace(raw)
		cand = strings.Trim(cand, "`'\"()[]{}<>,.;:")
		cand = strings.TrimPrefix(cand, "./")
		cand = strings.TrimSuffix(cand, "/")
		if cand == "" {
			return
		}
		if idx := strings.IndexAny(cand, "#:"); idx >= 0 {
			cand = strings.TrimSuffix(cand[:idx], "/") + cand[idx:]
		}
		if isValidRepoAnchor(repoRoot, cand) && !seen[cand] {
			seen[cand] = true
			anchors = append(anchors, cand)
		}
	}

	// --- Step 2: Inspect backtick-quoted spans and their whitespace-delimited tokens ---
	for _, m := range harvestBacktickRe.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		span := strings.TrimSpace(m[1])
		addIfValid(span)
		if strings.Contains(span, " ") {
			for _, tok := range strings.Fields(span) {
				addIfValid(tok)
			}
		}
	}

	// --- Step 3: Inspect unquoted path-like tokens in the text ---
	for _, m := range harvestPathTokenRe.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			addIfValid(m[1])
		}
	}

	return anchors
}

// nonEmpty returns the first non-whitespace string in vals, or "" if all are empty.
func nonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
