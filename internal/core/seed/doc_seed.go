package seed

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// DocSeedOptions configures a doc-seeding run.
type DocSeedOptions struct {
	RepoRoot string
	OpsDir   string
	DocPaths []string // Optional explicit repo-relative paths; if empty, uses config.ResolveDocSeedGlobs(RepoRoot, RepoCfg)
	RepoCfg  config.RepoConfig
	DryRun   bool
}

// DocSeedFileStat summarizes doc-seeding activity for a single file.
type DocSeedFileStat struct {
	Path    string `json:"path"`
	BlobSHA string `json:"blob_sha"`
	Added   int    `json:"added"`
	Amended int    `json:"amended"`
	Retired int    `json:"retired"`
	Skipped bool   `json:"skipped"`
}

// DocSeedResult holds the aggregate results of a SeedDocs run.
type DocSeedResult struct {
	FilesScanned  int               `json:"files_scanned"`
	FilesSeeded   int               `json:"files_seeded"`
	FilesSkipped  int               `json:"files_skipped"`
	EntriesAdded  int               `json:"entries_added"`
	EntriesAmend  int               `json:"entries_amended"`
	EntriesRetire int               `json:"entries_retired"`
	Files         []DocSeedFileStat `json:"files,omitempty"`
	Ops           []ops.Op          `json:"ops,omitempty"`
}

// DocSeedSource wraps deterministic doc seeding and reconciliation.
type DocSeedSource struct {
	Opts     DocSeedOptions
	ScopeCfg *config.ScopeConfig
	Reader   *ops.Reader
	Writer   *ops.Writer
}

// Seed executes the doc seed reconciliation.
func (s *DocSeedSource) Seed(ctx context.Context) (*DocSeedResult, error) {
	return SeedDocs(ctx, s.Opts, s.ScopeCfg, s.Reader, s.Writer)
}

// ComputeGitBlobSHA computes the canonical Git blob SHA-1 ("blob <len>\x00<content>").
// If content is nil and repoRoot/relPath are provided, it reads the file from disk first.
func ComputeGitBlobSHA(repoRoot, relPath string, content []byte) string {
	if content == nil && repoRoot != "" && relPath != "" {
		fullPath := filepath.Join(repoRoot, filepath.FromSlash(relPath))
		data, err := os.ReadFile(fullPath)
		if err == nil {
			content = data
		} else {
			cmd := exec.Command("git", "hash-object", "--", filepath.FromSlash(relPath))
			cmd.Dir = repoRoot
			if out, gitErr := cmd.Output(); gitErr == nil {
				return strings.TrimSpace(string(out))
			}
			return ""
		}
	}
	header := fmt.Sprintf("blob %d\x00", len(content))
	h := sha1.New()
	_, _ = h.Write([]byte(header))
	_, _ = h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

var (
	numberedBulletRe = regexp.MustCompile(`^\d+[.)]\s+(.*)$`)
	boldPrefixRe     = regexp.MustCompile(`^\*\*([^*]{3,80})\*\*\s*[:—-]\s*(.+)$`)
	tocLinkOnlyRe    = regexp.MustCompile(`^\[[^\]]+\]\(#[^)]+\)$`)
	wordSplitRe      = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	markdownLinkRe   = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)\)`)
	alertTagRe       = regexp.MustCompile(`^\[!(?:IMPORTANT|WARNING|CAUTION|NOTE|TIP)\]\s*`)
)

func init() {
	for _, ext := range []string{".ini", ".hpp", ".hxx", ".cc", ".csv", ".html", ".css", ".defaults"} {
		allowedSingleSegmentExts[ext] = true
	}
	allowedSingleSegmentNames["CMakeLists.txt"] = true
}

func isValidDocDirAnchor(p string) bool {
	switch strings.TrimSuffix(strings.TrimSpace(p), "/") {
	case "src", "include", "modules", "test", "scripts", "web", "lib", "hardware", "docs", "template-app", "sample-app":
		return true
	}
	return false
}

func expandSubmoduleAnchorsForScopes(anchors []string) []string {
	out := make([]string, 0, len(anchors))
	seenDir := make(map[string]bool)
	for _, a := range anchors {
		out = append(out, a)
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(a)))
		clean = strings.TrimPrefix(clean, "./")
		segs := strings.Split(clean, "/")
		if (segs[0] == "modules" || segs[0] == "src" || segs[0] == "lib") &&
			(len(segs) >= 3 || (len(segs) == 2 && filepath.Ext(segs[1]) == "")) {
			subDir := segs[0] + "/" + segs[1]
			if !seenDir[subDir] {
				seenDir[subDir] = true
				out = append(out, subDir+"/.1.hpp", subDir+"/.2.hpp", subDir+"/.3.hpp")
			}
		}
	}
	return out
}

var interfaceToModuleMap = map[string]string{
	"IDataSource":              "modules/DataManager/DataManager.hpp",
	"FetchContext":             "modules/DataManager/DataManager.hpp",
	"DeadlineToken":            "modules/DataManager/DataManager.hpp",
	"IWebContributor":          "modules/WebServer/WebServer.hpp",
	"ITabRegistry":             "modules/WebServer/WebServer.hpp",
	"PsychicHttpServer":        "modules/WebServer/WebServer.hpp",
	"IMCPContributor":          "modules/MCPServer/MCPServer.hpp",
	"MCPToolRegistry":          "modules/MCPServer/MCPServer.hpp",
	"MCPBuiltinTools":          "modules/MCPServer/MCPServer.hpp",
	"IDisplayBoard":            "modules/U8G2Display/U8G2Display.hpp",
	"ILVGLHardwareAdapter":     "modules/LVGLDisplay/LVGLDisplay.hpp",
	"IConfigurable":            "modules/ConfigManager/ConfigManager.hpp",
	"IOTAVerifier":             "modules/OTAUpdateManager/OTAUpdateManager.hpp",
	"IPreconditionContributor": "modules/PreconditionEngine/PreconditionEngine.hpp",
	"IPreconditionRegistry":    "modules/PreconditionEngine/PreconditionEngine.hpp",
	"IBLEServiceProvider":      "modules/BLEManager/BLEManager.hpp",
	"IBLEClientProvider":       "modules/BLEManager/BLEManager.hpp",
}

func extractDocAnchors(text, docRelPath string, isValidRepoPath func(string) bool) []string {
	var valid []string
	seen := make(map[string]bool)
	addAnchor := func(cand string) {
		cand = filepath.ToSlash(strings.TrimSuffix(strings.TrimSpace(cand), "/"))
		cand = strings.TrimPrefix(cand, "./")
		if cand == "" || cand == "." || seen[cand] || isExcludedDocSeedTargetAnchor(cand) {
			return
		}
		if (IsValidAnchorPathSyntax(cand) || isValidDocDirAnchor(cand)) && isValidRepoPath(cand) {
			seen[cand] = true
			valid = append(valid, cand)
		}
	}

	for _, a := range filterValidAnchors(extractBacktickPaths(text), isValidRepoPath) {
		if !seen[a] {
			seen[a] = true
			valid = append(valid, a)
		}
	}

	// Inspect backtick spans for C++/PlatformIO headers, directories, and module/interface symbols.
	matches := backtickRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		rawTok := strings.TrimSpace(m[1])
		rawTok = strings.TrimPrefix(rawTok, "#include ")
		rawTok = strings.Trim(rawTok, `"' `)
		rawTok = strings.TrimSuffix(rawTok, "/")
		if rawTok == "" {
			continue
		}

		addAnchor(rawTok)
		if !strings.HasPrefix(rawTok, "include/") && !strings.Contains(rawTok, " ") {
			addAnchor("include/" + rawTok)
		}

		// Extract base C++ class/module/interface identifier before ::, ., or (
		ident := rawTok
		if idx := strings.Index(ident, "::"); idx > 0 {
			ident = ident[:idx]
		}
		if idx := strings.IndexByte(ident, '('); idx > 0 {
			ident = ident[:idx]
		}
		if strings.HasPrefix(ident, "app.") {
			rest := strings.TrimPrefix(ident, "app.")
			if rest == "registerModule" || rest == "begin" {
				addAnchor("modules/Application/Application.hpp")
				addAnchor("src/core")
			}
		}
		if idx := strings.IndexByte(ident, '.'); idx > 0 {
			ident = ident[:idx]
		}
		ident = strings.TrimSpace(ident)
		if len(ident) >= 3 && ident[0] >= 'A' && ident[0] <= 'Z' && !strings.ContainsAny(ident, " /\\:*?\"<>|") {
			stems := []string{ident}
			if s := strings.TrimSuffix(ident, "Module"); s != "" && s != ident {
				stems = append(stems, s)
			}
			if s := strings.TrimSuffix(ident, "Manager"); s != "" && s != ident {
				stems = append(stems, s)
			}
			for _, stem := range stems {
				addAnchor("modules/" + stem + "/" + stem + ".hpp")
				addAnchor("modules/" + stem + "/" + ident + ".hpp")
				addAnchor("src/modules/" + stem + "/" + stem + ".hpp")
				addAnchor("src/modules/" + stem)
				addAnchor("src/core/" + stem + ".hpp")
				addAnchor("lib/" + stem + "/" + stem + ".hpp")
			}
			addAnchor("include/interfaces/" + ident + ".hpp")
			addAnchor("include/interfaces/" + ident + ".h")
			addAnchor("include/types/" + ident + ".hpp")
			addAnchor("include/events/" + ident + ".hpp")
			addAnchor("include/" + ident + ".hpp")
			addAnchor("include/" + ident + ".h")
			if modHdr, ok := interfaceToModuleMap[ident]; ok {
				addAnchor(modHdr)
			}
		}
	}

		// Inspect Markdown links [label](target) for relative paths or file:// repo paths.
	linkMatches := markdownLinkRe.FindAllStringSubmatch(text, -1)
	for _, lm := range linkMatches {
		target := strings.TrimSpace(lm[1])
		if idx := strings.IndexByte(target, '#'); idx >= 0 {
			target = target[:idx]
		}
		if target == "" || strings.HasSuffix(strings.ToLower(target), ".md") {
			continue
		}
		lowerTarget := strings.ToLower(target)
		if strings.HasPrefix(lowerTarget, "http:") || strings.HasPrefix(lowerTarget, "https:") || strings.HasPrefix(lowerTarget, "mailto:") {
			continue
		}
		var resolved string
		if strings.HasPrefix(lowerTarget, "file://") {
			for _, rootSeg := range []string{"/modules/", "/include/", "/src/", "/test/", "/template-app/", "/sample-app/", "/scripts/", "/web/", "/lib/", "/hardware/"} {
				if idx := strings.Index(target, rootSeg); idx >= 0 {
					resolved = target[idx+1:]
					break
				}
			}
		} else if !strings.Contains(target, ":") && docRelPath != "" {
			resolved = filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(docRelPath), target)))
		}
		if resolved != "" {
			addAnchor(resolved)
			baseIdent := strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved))
			if modHdr, ok := interfaceToModuleMap[baseIdent]; ok {
				addAnchor(modHdr)
			}
		}
	}

	return valid
}

func capFallbackAnchors(anchors []string, maxCount int) []string {
	if len(anchors) <= maxCount {
		return append([]string(nil), anchors...)
	}
	var multiSeg, singleSeg []string
	for _, a := range anchors {
		if strings.Contains(a, "/") {
			multiSeg = append(multiSeg, a)
		} else {
			singleSeg = append(singleSeg, a)
		}
	}
	ordered := append(multiSeg, singleSeg...)
	return append([]string(nil), ordered[:maxCount]...)
}

func generateDocSeedID(relPath, entryType, normalizedKey string) string {
	sum := sha256.Sum256([]byte(relPath + ":" + entryType + ":" + normalizedKey))
	return fmt.Sprintf("docseed-%x", sum[:4])
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

type docCandidate struct {
	heading        string
	text           string
	keyHint        string
	isRecipe       bool
	sectionAnchors []string
}

// ParseDocToOps deterministically extracts structured, repo-anchored knowledge ops from a markdown document.
func ParseDocToOps(
	relPath string,
	content string,
	blobSHA string,
	commitTime time.Time,
	resolver *config.ScopeResolver,
	isValidRepoPath func(string) bool,
) []ops.Op {
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	if isValidRepoPath == nil {
		isValidRepoPath = IsValidAnchorPathSyntax
	}
	if commitTime.IsZero() {
		commitTime = time.Now().UTC()
	} else {
		commitTime = commitTime.UTC()
	}

	isSkill := strings.HasSuffix(relPath, "SKILL.md") || strings.Contains(relPath, ".agents/skills/")
	isDeveloperDoc := strings.HasPrefix(relPath, "docs/developer/")

	// Collect valid repo paths referenced anywhere in the document (used as fallback for SKILL.md and developer guides).
	var docAnchors []string
	if isSkill || isDeveloperDoc {
		docAnchors = capFallbackAnchors(extractDocAnchors(content, relPath, isValidRepoPath), 4)
		if len(docAnchors) == 0 {
			for _, fb := range []string{
				"src/modules",
				"src/core",
				"modules/Application/Application.hpp",
				"include/AppFoundation.h",
				"include/interfaces",
				"modules",
				"src",
				"include",
				"platformio.ini",
				"test",
				"docs/developer",
			} {
				if isValidRepoPath(fb) {
					docAnchors = append(docAnchors, fb)
					if len(docAnchors) >= 4 {
						break
					}
				}
			}
		}
	}

	lines := strings.Split(strings.TrimPrefix(content, "\uFEFF"), "\n")
	startIdx := skipYAMLFrontmatter(lines)

	var (
		currentHeading       string
		currentHeadingLevel  int
		parentHeadingAnchors []string
		sectionAnchors       []string
		inCodeFence          bool
		codeFenceLang        string
		codeFenceLines       []string
		candidates           []docCandidate
		lastCandIdx          = -1
		questionHeadingFresh bool
	)

	mergeUniqueAnchors := func(base, extra []string) []string {
		seen := make(map[string]bool, len(base)+len(extra))
		var out []string
		for _, a := range base {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
		for _, a := range extra {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
		return out
	}

	for i := startIdx; i < len(lines); i++ {
		rawLine := lines[i]
		trimLine := strings.TrimSpace(rawLine)

		if strings.HasPrefix(trimLine, "```") || strings.HasPrefix(trimLine, "~~~") {
			if !inCodeFence {
				inCodeFence = true
				codeFenceLang = strings.ToLower(strings.TrimSpace(strings.TrimLeft(trimLine, "`~")))
				codeFenceLines = nil
			} else {
				inCodeFence = false
				firstCmd := extractFirstCommandLine(codeFenceLang, codeFenceLines)
				if firstCmd != "" {
					if lastCandIdx >= 0 && !strings.Contains(candidates[lastCandIdx].text, firstCmd) {
						candidates[lastCandIdx].text = clampText(candidates[lastCandIdx].text + " (`" + firstCmd + "`)")
						candidates[lastCandIdx].isRecipe = true
					} else if isSkill || isDeveloperDoc || isProceduralHeading(currentHeading) {
						candidates = append(candidates, docCandidate{
							heading:        currentHeading,
							text:           clampText(fmt.Sprintf("%s: `%s`", nonEmptyOrDefault(currentHeading, "Command"), firstCmd)),
							keyHint:        strings.ToLower(currentHeading + ":" + firstCmd),
							isRecipe:       true,
							sectionAnchors: append([]string(nil), sectionAnchors...),
						})
						lastCandIdx = len(candidates) - 1
					}
				}
				codeFenceLang = ""
				codeFenceLines = nil
			}
			questionHeadingFresh = false
			continue
		}

		if inCodeFence {
			codeFenceLines = append(codeFenceLines, rawLine)
			continue
		}

		if trimLine == "" || trimLine == "---" {
			continue
		}

		// Track headings
		if strings.HasPrefix(trimLine, "#") {
			hashes := 0
			for hashes < len(trimLine) && trimLine[hashes] == '#' {
				hashes++
			}
			if hashes >= 1 && hashes <= 6 && (hashes == len(trimLine) || trimLine[hashes] == ' ') {
				headingText := strings.TrimSpace(trimLine[hashes:])
				currentHeading = headingText
				currentHeadingLevel = hashes
				hAnchors := extractDocAnchors(headingText, relPath, isValidRepoPath)
				if hashes <= 2 {
					parentHeadingAnchors = hAnchors
					sectionAnchors = append([]string(nil), hAnchors...)
				} else {
					if len(hAnchors) > 0 {
						sectionAnchors = mergeUniqueAnchors(hAnchors, parentHeadingAnchors)
					} else {
						sectionAnchors = append([]string(nil), parentHeadingAnchors...)
					}
				}
				lastCandIdx = -1
				questionHeadingFresh = (isSkill || isDeveloperDoc) && hashes >= 3 && strings.HasSuffix(headingText, "?")
				continue
			}
		}

		// Skip keyword-trigger catalog sections in skills
		if strings.EqualFold(currentHeading, "Trigger Reference") {
			continue
		}

		// Skip markdown table rows
		if strings.HasPrefix(trimLine, "|") && strings.HasSuffix(trimLine, "|") {
			questionHeadingFresh = false
			continue
		}

		// Extract GitHub/Markdown callout blockquotes (> [!IMPORTANT], > [!WARNING], > [!CAUTION], > **Rule:** ...)
		if strings.HasPrefix(trimLine, ">") {
			calloutLine := strings.TrimSpace(strings.TrimPrefix(trimLine, ">"))
			isAlertHeader := alertTagRe.MatchString(calloutLine)
			isBoldCallout := strings.HasPrefix(calloutLine, "**Rule") ||
				strings.HasPrefix(calloutLine, "**Important") ||
				strings.HasPrefix(calloutLine, "**Warning") ||
				strings.HasPrefix(calloutLine, "**Caution") ||
				strings.HasPrefix(calloutLine, "**Strict")
			if isAlertHeader || isBoldCallout {
				calloutParts := []string{}
				cleanedFirst := strings.TrimSpace(alertTagRe.ReplaceAllString(calloutLine, ""))
				if cleanedFirst != "" {
					calloutParts = append(calloutParts, cleanedFirst)
				}
				for i+1 < len(lines) {
					nextTrim := strings.TrimSpace(lines[i+1])
					if !strings.HasPrefix(nextTrim, ">") {
						break
					}
					i++
					sub := strings.TrimSpace(strings.TrimPrefix(nextTrim, ">"))
					if sub == "" {
						continue
					}
					if bText, ok := extractBulletLine(sub); ok {
						sub = bText
					}
					calloutParts = append(calloutParts, sub)
				}
				calloutText := collapseWhitespace(strings.Join(calloutParts, " "))
				if calloutText != "" {
					keyHint := extractKeyHint(currentHeading, calloutText)
					candidates = append(candidates, docCandidate{
						heading:        currentHeading,
						text:           clampText(calloutText),
						keyHint:        keyHint,
						sectionAnchors: append([]string(nil), sectionAnchors...),
					})
					lastCandIdx = len(candidates) - 1
				}
			}
			questionHeadingFresh = false
			continue
		}

		// Markdown bullets and numbered items
		if bulletText, ok := extractBulletLine(trimLine); ok {
			questionHeadingFresh = false
			// Join immediate indented continuation lines
			for i+1 < len(lines) {
				nextRaw := lines[i+1]
				nextTrim := strings.TrimSpace(nextRaw)
				if nextTrim == "" ||
					strings.HasPrefix(nextTrim, "#") ||
					strings.HasPrefix(nextTrim, "```") ||
					strings.HasPrefix(nextTrim, "~~~") ||
					strings.HasPrefix(nextTrim, "|") ||
					isBulletStart(nextTrim) {
					break
				}
				if strings.HasPrefix(nextRaw, "  ") || strings.HasPrefix(nextRaw, "\t") {
					bulletText += " " + nextTrim
					i++
				} else {
					break
				}
			}
			bulletText = collapseWhitespace(bulletText)
			keyHint := extractKeyHint(currentHeading, bulletText)
			candidates = append(candidates, docCandidate{
				heading:        currentHeading,
				text:           clampText(bulletText),
				keyHint:        keyHint,
				sectionAnchors: append([]string(nil), sectionAnchors...),
			})
			lastCandIdx = len(candidates) - 1
			continue
		}

		// For non-bullet prose lines: enrich sectionAnchors (for developer docs & skills)
		// and capture concise Q&A answers under ### ...? headings.
		if isSkill || isDeveloperDoc {
			lineAnchors := extractDocAnchors(trimLine, relPath, isValidRepoPath)
			if len(lineAnchors) > 0 {
				sectionAnchors = capFallbackAnchors(mergeUniqueAnchors(sectionAnchors, lineAnchors), 4)
				if currentHeadingLevel <= 2 {
					parentHeadingAnchors = capFallbackAnchors(mergeUniqueAnchors(parentHeadingAnchors, lineAnchors), 4)
				}
			}
		}

		if questionHeadingFresh {
			questionHeadingFresh = false
			paraParts := []string{trimLine}
			for i+1 < len(lines) {
				nextTrim := strings.TrimSpace(lines[i+1])
				if nextTrim == "" ||
					strings.HasPrefix(nextTrim, "#") ||
					strings.HasPrefix(nextTrim, "```") ||
					strings.HasPrefix(nextTrim, "~~~") ||
					strings.HasPrefix(nextTrim, "|") ||
					strings.HasPrefix(nextTrim, ">") ||
					isBulletStart(nextTrim) {
					break
				}
				i++
				paraParts = append(paraParts, nextTrim)
			}
			paraText := collapseWhitespace(strings.Join(paraParts, " "))
			qaText := fmt.Sprintf("**%s**: %s", currentHeading, paraText)
			candidates = append(candidates, docCandidate{
				heading:        currentHeading,
				text:           clampText(qaText),
				keyHint:        strings.ToLower(collapseWhitespace(currentHeading)),
				sectionAnchors: append([]string(nil), sectionAnchors...),
			})
			lastCandIdx = len(candidates) - 1
		}
	}

	var (
		emitted []ops.Op
		seenIDs = make(map[string]bool)
		usedKey = make(map[string]int)
	)

	for _, cand := range candidates {
		cleaned := strings.TrimSpace(cand.text)
		if !isSelfContainedDocStatement(cleaned) || IsJunkText(cleaned) || tocLinkOnlyRe.MatchString(cleaned) {
			continue
		}

		anchors := extractDocAnchors(cleaned, relPath, isValidRepoPath)
		if len(anchors) == 0 && len(cand.sectionAnchors) > 0 {
			anchors = append([]string(nil), cand.sectionAnchors...)
		}
		if len(anchors) == 0 && (isSkill || isDeveloperDoc) && len(docAnchors) > 0 {
			anchors = append([]string(nil), docAnchors...)
		}

		var scopeAnchors []string
		var nonIncludeScopeAnchors []string
		for _, a := range anchors {
			if strings.Contains(a, "/") && !strings.HasPrefix(a, ".") {
				scopeAnchors = append(scopeAnchors, a)
				if !strings.HasPrefix(a, "include/") {
					nonIncludeScopeAnchors = append(nonIncludeScopeAnchors, a)
				}
			}
		}
		if len(nonIncludeScopeAnchors) > 0 {
			scopeAnchors = nonIncludeScopeAnchors
		}

		keywords := extractKeywords(cand.heading, relPath)
		var dossierScope string
		var matchedDeclared bool
		if resolver != nil {
			dossierScope = resolver.ResolveScope(scopeAnchors, keywords)
			if dossierScope != "" && len(anchors) == 0 {
				matchedDeclared = true
			}
		}

		// For non-skill, non-developer docs, strictly require at least 1 valid repo anchor.
		if !isSkill && !isDeveloperDoc && len(anchors) == 0 {
			continue
		}
		if len(anchors) == 0 && !matchedDeclared {
			continue
		}

		if dossierScope == "" && len(scopeAnchors) > 0 {
			mods := config.DeriveModuleScopes(expandSubmoduleAnchorsForScopes(scopeAnchors))
			for _, m := range mods {
				if m.Name != "" && !isPollutedScope(m.Name) {
					dossierScope = m.Name
					break
				}
			}
		}
		if dossierScope == "" || isPollutedScope(dossierScope) {
			if isSkill {
				if defScope := deriveDocDefaultScope(relPath); !isPollutedScope(defScope) {
					dossierScope = defScope
				} else {
					dossierScope = "root"
				}
			} else {
				for _, a := range anchors {
					if isValidDocDirAnchor(a) && !isPollutedScope(a) {
						dossierScope = a
						break
					}
				}
				if dossierScope == "" || isPollutedScope(dossierScope) {
					dossierScope = "root"
				}
			}
		}

		entryType := classifyDocEntryType(relPath, cand.heading, cleaned, cand.isRecipe)
		kind := dossiers.DeriveCognitiveKind(entryType)

		normKey := normalizeCandidateKey(cand.keyHint, cleaned)
		if usedKey[normKey] > 0 {
			normKey = normalizeCandidateKey("", cleaned)
		}
		usedKey[normKey]++

		entryID := generateDocSeedID(relPath, string(entryType), normKey)
		if seenIDs[entryID] {
			continue
		}
		seenIDs[entryID] = true

		emitted = append(emitted, ops.Op{
			Type:         ops.OpAdd,
			EntryID:      entryID,
			DossierScope: dossierScope,
			Timestamp:    commitTime,
			Source:       ops.SourceDocSeed,
			Fields: map[string]any{
				"entry_type":          string(entryType),
				"kind":                string(kind),
				"text":                cleaned,
				"anchors":             anchors,
				"source_doc_path":     relPath,
				"source_doc_blob_sha": blobSHA,
				"evidence":            []string{"doc:" + relPath},
			},
		})
	}

	return emitted
}

// SeedDocs scans configured documentation and skill files, extracts anchored knowledge ops,
// and reconciles them idempotently against existing active `source: "doc-seed"` entries.
func SeedDocs(
	ctx context.Context,
	opts DocSeedOptions,
	scopeCfg *config.ScopeConfig,
	opsReader *ops.Reader,
	opsWriter *ops.Writer,
) (*DocSeedResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.RepoRoot == "" {
		opts.RepoRoot = "."
	}

	existingByDoc := make(map[string][]dossiers.Entry)
	existingByID := make(map[string]dossiers.Entry)

	if opsReader != nil {
		proj := dossiers.NewProjector(opsReader)
		projected, err := proj.Project(ctx)
		if err != nil {
			return nil, fmt.Errorf("projecting existing ops: %w", err)
		}
		for _, d := range projected {
			for _, e := range d.Entries {
				if e.Retired {
					continue
				}
				if e.Source == ops.SourceDocSeed && e.SourceDocPath != "" {
					normDoc := filepath.ToSlash(e.SourceDocPath)
					existingByDoc[normDoc] = append(existingByDoc[normDoc], e)
					existingByID[e.ID] = e
				}
			}
		}
	}

	var docPaths []string
	if len(opts.DocPaths) > 0 {
		seen := make(map[string]bool)
		for _, p := range opts.DocPaths {
			norm := filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "./"))
			if norm != "" && !seen[norm] {
				seen[norm] = true
				docPaths = append(docPaths, norm)
			}
		}
		sort.Strings(docPaths)
	} else {
		resolved, err := config.ResolveDocSeedGlobs(opts.RepoRoot, opts.RepoCfg)
		if err != nil {
			return nil, fmt.Errorf("resolving doc seed globs: %w", err)
		}
		seen := make(map[string]bool, len(resolved))
		for _, p := range resolved {
			seen[p] = true
			docPaths = append(docPaths, p)
		}
		// Include any previously seeded doc paths that were deleted or excluded so their entries can be retired.
		for prevDoc := range existingByDoc {
			if !seen[prevDoc] {
				seen[prevDoc] = true
				docPaths = append(docPaths, prevDoc)
			}
		}
		sort.Strings(docPaths)
	}

	isValidRepoPath := buildRepoPathValidator(SeedOptions{RepoRoot: opts.RepoRoot})
	repoFiles := expandSubmoduleAnchorsForScopes(collectRepoFileAnchors(opts.RepoRoot))
	rawModuleScopes := config.DeriveModuleScopes(repoFiles)
	var moduleScopes []config.ModuleScope
	for _, m := range rawModuleScopes {
		if !isPollutedScope(m.Name) {
			moduleScopes = append(moduleScopes, m)
		}
	}
	aliasMap, err := config.BuildAliasMap(opsReader, ctx)
	if err != nil {
		aliasMap = config.NewAliasMap()
	}
	resolver := config.NewScopeResolver(scopeCfg, moduleScopes, aliasMap)

	res := &DocSeedResult{}
	var allEmittedOps []ops.Op
	now := time.Now().UTC()

	for _, relPath := range docPaths {
		relPath = filepath.ToSlash(relPath)
		fullPath := filepath.Join(opts.RepoRoot, filepath.FromSlash(relPath))
		res.FilesScanned++

		contentBytes, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				retiredCount := 0
				for _, oldEntry := range existingByDoc[relPath] {
					allEmittedOps = append(allEmittedOps, ops.Op{
						Type:         ops.OpRetire,
						EntryID:      oldEntry.ID,
						DossierScope: oldEntry.DossierScope,
						Timestamp:    now,
						Source:       ops.SourceDocSeed,
						Fields: map[string]any{
							"reason": "source_doc_deleted: " + relPath,
						},
					})
					retiredCount++
					res.EntriesRetire++
				}
				if retiredCount > 0 {
					res.FilesSeeded++
				}
				res.Files = append(res.Files, DocSeedFileStat{
					Path:    relPath,
					Retired: retiredCount,
				})
				continue
			}
			return nil, fmt.Errorf("reading doc seed %s: %w", relPath, readErr)
		}

		blobSHA := ComputeGitBlobSHA(opts.RepoRoot, relPath, contentBytes)
		prevEntries := existingByDoc[relPath]

		if len(prevEntries) > 0 && allHaveBlobSHA(prevEntries, blobSHA) {
			res.FilesSkipped++
			res.Files = append(res.Files, DocSeedFileStat{
				Path:    relPath,
				BlobSHA: blobSHA,
				Skipped: true,
			})
			continue
		}

		commitTime := lookupGitCommitTime(opts.RepoRoot, relPath, now)
		candidateOps := ParseDocToOps(relPath, string(contentBytes), blobSHA, commitTime, resolver, isValidRepoPath)

		stat := DocSeedFileStat{
			Path:    relPath,
			BlobSHA: blobSHA,
		}
		newIDSet := make(map[string]bool, len(candidateOps))

		for _, cand := range candidateOps {
			newIDSet[cand.EntryID] = true
			candText, _ := cand.Fields["text"].(string)
			candType, _ := cand.Fields["entry_type"].(string)

			if existingEntry, ok := existingByID[cand.EntryID]; ok {
				if existingEntry.Text != candText ||
					existingEntry.SourceDocBlobSHA != blobSHA ||
					existingEntry.DossierScope != cand.DossierScope ||
					string(existingEntry.Type) != candType {
					if existingEntry.DossierScope != cand.DossierScope && cand.DossierScope != "" {
						allEmittedOps = append(allEmittedOps, ops.Op{
							Type:         ops.OpMove,
							EntryID:      cand.EntryID,
							DossierScope: existingEntry.DossierScope,
							Timestamp:    now,
							Source:       ops.SourceDocSeed,
							Fields: map[string]any{
								"to_dossier": cand.DossierScope,
							},
						})
					}
					allEmittedOps = append(allEmittedOps, ops.Op{
						Type:         ops.OpAmend,
						EntryID:      cand.EntryID,
						DossierScope: cand.DossierScope,
						Timestamp:    now,
						Source:       ops.SourceDocSeed,
						Fields:       cand.Fields,
					})
					stat.Amended++
					res.EntriesAmend++
				}
			} else {
				allEmittedOps = append(allEmittedOps, cand)
				stat.Added++
				res.EntriesAdded++
			}
		}

		for _, oldEntry := range prevEntries {
			if !newIDSet[oldEntry.ID] {
				allEmittedOps = append(allEmittedOps, ops.Op{
					Type:         ops.OpRetire,
					EntryID:      oldEntry.ID,
					DossierScope: oldEntry.DossierScope,
					Timestamp:    now,
					Source:       ops.SourceDocSeed,
					Fields: map[string]any{
						"reason": fmt.Sprintf("source_doc_updated: %s (%s)", relPath, shortSHA(blobSHA)),
					},
				})
				stat.Retired++
				res.EntriesRetire++
			}
		}

		if stat.Added > 0 || stat.Amended > 0 || stat.Retired > 0 {
			res.FilesSeeded++
		} else {
			stat.Skipped = true
			res.FilesSkipped++
		}
		res.Files = append(res.Files, stat)
	}

	res.Ops = allEmittedOps

	if !opts.DryRun && opsWriter != nil && len(allEmittedOps) > 0 {
		batch := ops.NewBatch(allEmittedOps)
		if err := opsWriter.Write(batch); err != nil {
			return nil, fmt.Errorf("writing doc-seed ops batch: %w", err)
		}
	}

	return res, nil
}

func allHaveBlobSHA(entries []dossiers.Entry, blobSHA string) bool {
	if blobSHA == "" || len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if e.SourceDocBlobSHA != blobSHA {
			return false
		}
	}
	return true
}

func skipYAMLFrontmatter(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 1
		}
	}
	return 0
}

func extractFirstCommandLine(lang string, fenceLines []string) string {
	isShellFence := lang == "bash" || lang == "sh" || lang == "shell" || lang == "zsh" || lang == ""
	if !isShellFence {
		return ""
	}
	for _, raw := range fenceLines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "$ ")
		if lang == "bash" || lang == "sh" || lang == "shell" || lang == "zsh" ||
			strings.HasPrefix(line, "repo-memory ") ||
			strings.HasPrefix(line, "go ") ||
			strings.HasPrefix(line, "git ") ||
			strings.HasPrefix(line, "make ") ||
			strings.HasPrefix(line, "pio ") ||
			strings.HasPrefix(line, "platformio ") ||
			strings.HasPrefix(line, "python ") ||
			strings.HasPrefix(line, "python3 ") ||
			strings.HasPrefix(line, "cmake ") ||
			strings.HasPrefix(line, "./") {
			if len(line) <= 120 {
				return line
			}
		}
		break
	}
	return ""
}

func isProceduralHeading(heading string) bool {
	lower := strings.ToLower(heading)
	for _, cue := range []string{"procedure", "recipe", "workflow", "command", "usage", "verification", "build", "test", "cli", "setup"} {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

func isSelfContainedDocStatement(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 40 {
		return false
	}
	if strings.HasSuffix(s, ";") {
		return false
	}
	if strings.HasPrefix(s, "`") {
		return true
	}
	// Strip leading emphasis markers (* or _) to inspect first word character
	trimmed := strings.TrimLeft(s, "*_ \t")
	if len(trimmed) == 0 {
		return false
	}
	if strings.HasPrefix(trimmed, "`") {
		return true
	}
	firstRune := []rune(trimmed)[0]
	if firstRune >= 'a' && firstRune <= 'z' {
		return false
	}
	return true
}

func isExcludedDocSeedTargetAnchor(anchor string) bool {
	norm := filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(anchor), "./"))
	return strings.HasPrefix(norm, "docs/experiments/") ||
		norm == "docs/experiments" ||
		strings.HasPrefix(norm, "docs/evaluations/") ||
		norm == "docs/evaluations" ||
		strings.HasPrefix(norm, ".agents/skills/principal-architect-workspace/") ||
		norm == ".agents/skills/principal-architect-workspace" ||
		strings.Contains(norm, "/principal-architect-workspace/")
}

func isBulletStart(trimLine string) bool {
	if strings.HasPrefix(trimLine, "- ") || strings.HasPrefix(trimLine, "* ") || strings.HasPrefix(trimLine, "+ ") {
		return true
	}
	return numberedBulletRe.MatchString(trimLine)
}

func extractBulletLine(trimLine string) (string, bool) {
	for _, pfx := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimLine, pfx) {
			rest := strings.TrimSpace(strings.TrimPrefix(trimLine, pfx))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "[ ]"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "[x]"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "[X]"))
			return rest, rest != ""
		}
	}
	if m := numberedBulletRe.FindStringSubmatch(trimLine); m != nil {
		rest := strings.TrimSpace(m[1])
		return rest, rest != ""
	}
	return "", false
}

func extractKeyHint(heading, bulletText string) string {
	if m := boldPrefixRe.FindStringSubmatch(bulletText); m != nil {
		label := strings.ToLower(collapseWhitespace(strings.TrimSpace(m[1])))
		if heading != "" {
			return strings.ToLower(collapseWhitespace(heading)) + ":" + label
		}
		return label
	}
	return ""
}

func normalizeCandidateKey(keyHint, text string) string {
	if strings.TrimSpace(keyHint) != "" {
		return strings.ToLower(collapseWhitespace(keyHint))
	}
	return strings.ToLower(collapseWhitespace(text))
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func clampText(s string) string {
	s = collapseWhitespace(s)
	const maxLen = 400
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimSpace(s[:maxLen-3]) + "..."
}

func nonEmptyOrDefault(s, def string) string {
	if strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return def
}

func filterValidAnchors(candidates []string, isValidRepoPath func(string) bool) []string {
	var valid []string
	seen := make(map[string]bool)
	for _, c := range candidates {
		if isExcludedDocSeedTargetAnchor(c) {
			continue
		}
		if isValidRepoPath(c) && !seen[c] {
			seen[c] = true
			valid = append(valid, c)
		}
	}
	return valid
}

func extractKeywords(heading, relPath string) []string {
	combined := heading + " " + strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	parts := wordSplitRe.Split(strings.ToLower(combined), -1)
	seen := make(map[string]bool)
	var out []string
	for _, p := range parts {
		if len(p) < 3 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func deriveDocDefaultScope(relPath string) string {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if strings.HasPrefix(clean, ".agents/skills/") {
		rest := strings.TrimPrefix(clean, ".agents/skills/")
		segs := strings.Split(rest, "/")
		if len(segs) > 0 && segs[0] != "" {
			return strings.ReplaceAll(segs[0], "_", "-")
		}
	}
	base := strings.TrimSuffix(filepath.Base(clean), filepath.Ext(clean))
	base = strings.ToLower(strings.ReplaceAll(base, "_", "-"))
	base = strings.ReplaceAll(base, " ", "-")
	if base == "" || base == "." {
		return "docs"
	}
	return base
}

func classifyDocEntryType(relPath, heading, text string, isRecipeHint bool) dossiers.EntryType {
	combined := strings.ToLower(heading + " " + text)

	for _, cue := range []string{
		"error", "404", "429", "panic", "fails with", "symptom", "mismatch", "ratelimitexceeded", "failure signature",
	} {
		if strings.Contains(combined, cue) {
			return dossiers.EntryTypeFailureSignature
		}
	}

	isSkill := strings.HasSuffix(relPath, "SKILL.md") || strings.Contains(relPath, ".agents/skills/")
	if isRecipeHint {
		return dossiers.EntryTypeRecipe
	}
	for _, cue := range []string{
		"procedure", "recipe", "workflow", "command", "usage", "go test", "go build", "repo-memory ",
		"pio ", "platformio ", "python ", "python3 ", "cmake ",
	} {
		if strings.Contains(combined, cue) {
			return dossiers.EntryTypeRecipe
		}
	}
	if isSkill {
		for _, cue := range []string{"run ", "step ", "execute ", "invoke ", "flag ", "--"} {
			if strings.Contains(combined, cue) {
				return dossiers.EntryTypeRecipe
			}
		}
	}

	for _, cue := range []string{
		"gotcha", "caveat", "trap", "quirk", "warning", "caution", "important", "lifetime", "scrubbed", "ignored",
	} {
		if strings.Contains(combined, cue) {
			return dossiers.EntryTypeGotcha
		}
	}

	for _, cue := range []string{
		"contract", "schema", "interface", "convention", "wire format",
	} {
		if strings.Contains(combined, cue) {
			return dossiers.EntryTypeContract
		}
	}

	return dossiers.EntryTypeInvariant
}

func collectRepoFileAnchors(repoRoot string) []string {
	var files []string
	_ = filepath.WalkDir(repoRoot, func(fullPath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, fullPath)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "." {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if (strings.HasPrefix(base, ".") && base != ".") ||
				base == "node_modules" || base == "vendor" ||
				relSlash == ".agents/skills/principal-architect-workspace" ||
				strings.HasSuffix(relSlash, "/principal-architect-workspace") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && strings.Contains(relSlash, "/") && !strings.HasPrefix(relSlash, ".") {
			files = append(files, relSlash)
		}
		return nil
	})
	return files
}

func lookupGitCommitTime(repoRoot, relPath string, fallback time.Time) time.Time {
	if repoRoot == "" || relPath == "" {
		return fallback
	}
	cmd := exec.Command("git", "log", "-1", "--format=%cI", "--", filepath.FromSlash(relPath))
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err == nil {
		if t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(string(out))); parseErr == nil {
			return t.UTC()
		}
	}
	return fallback
}
