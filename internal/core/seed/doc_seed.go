/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/seed/doc_seed.go
 * Description: Deterministically extracts structured, repository-anchored knowledge
 *              operations from Markdown documentation, developer guides, and SKILL.md
 *              files, and reconciles them idempotently against existing doc-seed entries.
 *
 * Exported Types / Functions:
 * - DocSeedOptions:      Configures repository root, ops directory, and doc globs for doc seeding.
 * - DocSeedFileStat:     Per-file summary of added, amended, retired, or skipped doc-seed entries.
 * - DocSeedResult:       Aggregate counts and emitted operations from a SeedDocs execution.
 * - DocSeedSource:       Wraps deterministic documentation seeding and reconciliation.
 *   - Seed():            Executes the documentation seed reconciliation pass.
 * - ComputeGitBlobSHA(): Computes the canonical Git blob SHA-1 for a file or byte slice.
 * - ParseDocToOps():     Extracts structured, anchored dossier operations from a Markdown document.
 * - SeedDocs():          Scans configured docs and reconciles doc-seed entries idempotently.
 */

// Package seed extracts and reconciles initial repository knowledge from Principal Architect reports and Markdown documentation.
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
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// DocSeedOptions configures a doc-seeding run.
type DocSeedOptions struct {
	RepoRoot string            // Root directory of the target Git repository.
	OpsDir   string            // Path to .memory/ops/ where operation batches are stored.
	DocPaths []string          // Optional explicit repo-relative paths; if empty, uses config.ResolveDocSeedGlobs(RepoRoot, RepoCfg).
	RepoCfg  config.RepoConfig // Repository configuration containing doc-seed inclusion/exclusion globs.
	DryRun   bool              // When true, computes reconciliation results without writing ops batches.
}

// DocSeedFileStat summarizes doc-seeding activity for a single file.
type DocSeedFileStat struct {
	Path    string `json:"path"`     // Repository-relative Markdown file path.
	BlobSHA string `json:"blob_sha"` // Canonical Git blob SHA-1 of the file contents.
	Added   int    `json:"added"`    // Count of newly added doc-seed entries.
	Amended int    `json:"amended"`  // Count of existing doc-seed entries amended or moved.
	Retired int    `json:"retired"`  // Count of stale doc-seed entries retired.
	Skipped bool   `json:"skipped"`  // True when the file was unchanged or yielded zero changes.
}

// DocSeedResult holds the aggregate results of a SeedDocs run.
type DocSeedResult struct {
	FilesScanned  int               `json:"files_scanned"`   // Total Markdown files evaluated.
	FilesSeeded   int               `json:"files_seeded"`    // Files that produced at least one add, amend, or retire op.
	FilesSkipped  int               `json:"files_skipped"`   // Files skipped due to matching blob SHA or no changes.
	EntriesAdded  int               `json:"entries_added"`   // Total OpAdd operations emitted across all files.
	EntriesAmend  int               `json:"entries_amended"` // Total OpAmend operations emitted across all files.
	EntriesRetire int               `json:"entries_retired"` // Total OpRetire operations emitted across all files.
	Files         []DocSeedFileStat `json:"files,omitempty"` // Per-file seeding statistics.
	Ops           []ops.Op          `json:"ops,omitempty"`   // Ordered slice of all emitted reconciliation operations.
}

// DocSeedSource wraps deterministic doc seeding and reconciliation.
type DocSeedSource struct {
	Opts     DocSeedOptions      // Seeding options and repository path configuration.
	ScopeCfg *config.ScopeConfig // Optional declared scope configuration for anchor resolution.
	Reader   *ops.Reader         // Reader for projecting existing doc-seed entries.
	Writer   *ops.Writer         // Writer for persisting reconciled doc-seed operation batches.
}

// Seed executes the doc seed reconciliation.
func (s *DocSeedSource) Seed(ctx context.Context) (*DocSeedResult, error) {
	return SeedDocs(ctx, s.Opts, s.ScopeCfg, s.Reader, s.Writer)
}

// ComputeGitBlobSHA computes the canonical Git blob SHA-1 ("blob <len>\x00<content>").
// If content is nil and repoRoot/relPath are provided, it reads the file from disk first.
func ComputeGitBlobSHA(repoRoot, relPath string, content []byte) string {
	// --- Step 1: Load file bytes from disk or git hash-object when content is nil ---
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

	// --- Step 2: Hash the canonical Git blob header and payload ---
	header := fmt.Sprintf("blob %d\x00", len(content))
	h := sha1.New()
	_, _ = h.Write([]byte(header))
	_, _ = h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

var (
	numberedBulletRe = regexp.MustCompile(`^\d+[.)]\s+(.*)$`)                            // Matches numbered Markdown list items ("1. " or "1) ").
	boldPrefixRe     = regexp.MustCompile(`^\*\*([^*]{3,80})\*\*\s*[:—-]\s*(.+)$`)       // Matches bold lead-in labels at the start of a bullet.
	tocLinkOnlyRe    = regexp.MustCompile(`^\[[^\]]+\]\(#[^)]+\)$`)                      // Matches pure table-of-contents anchor links.
	wordSplitRe      = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)                             // Splits text into alphanumeric keyword tokens.
	markdownLinkRe   = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)\)`)                     // Extracts target URLs/paths from Markdown links.
	alertTagRe       = regexp.MustCompile(`^\[!(?:IMPORTANT|WARNING|CAUTION|NOTE|TIP)\]\s*`) // Matches GitHub-style alert tags in blockquotes.
)

func init() {
	for _, ext := range []string{".ini", ".hpp", ".hxx", ".cc", ".csv", ".html", ".css", ".defaults"} {
		allowedSingleSegmentExts[ext] = true
	}
	allowedSingleSegmentNames["CMakeLists.txt"] = true
}

// isValidDocDirAnchor reports whether p is a recognized top-level repository directory anchor.
func isValidDocDirAnchor(p string) bool {
	switch strings.TrimSuffix(strings.TrimSpace(p), "/") {
	case "src", "include", "modules", "test", "scripts", "web", "lib", "hardware", "docs", "template-app", "sample-app":
		return true
	}
	return false
}

// expandSubmoduleAnchorsForScopes synthesizes placeholder header paths under modules/*, src/*, and lib/*
// so config.DeriveModuleScopes recognizes two-level module directories with >= 3 files.
func expandSubmoduleAnchorsForScopes(anchors []string) []string {
	// --- Step 1: Copy original anchors and append synthetic module probes ---
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

// repoSymbolIndex caches C++/Go symbol-to-module and method-to-header mappings for a repository root.
type repoSymbolIndex struct {
	symbolToModule      map[string]string   // Maps PascalCase symbol or interface names to their owning module header path.
	methodToHeaders     map[string][]string // Maps method names (e.g. "begin", "register*") to declaring header paths.
	orchestratorHdr     string              // Header path of the highest-fanout composition root module.
	orchestratorStemLow string              // Lowercase stem of the orchestrator module name.
	defaultAnchors      []string            // Ordered fallback repository anchor paths discovered on disk.
}

var (
	symIdxMu          sync.RWMutex                                                          // Protects symIdxByRoot and activeRepoSymIdx.
	symIdxByRoot      = make(map[string]*repoSymbolIndex)                                   // Cache of built symbol indexes keyed by cleaned repoRoot.
	activeRepoSymIdx  *repoSymbolIndex                                                      // Most recently activated repository symbol index.
	typeDeclRe        = regexp.MustCompile(`\b(?:class|struct)\s+([A-Z][A-Za-z0-9_]{2,})\b`) // Matches C++ class or struct declarations.
	quotedIncludeRe   = regexp.MustCompile(`#include\s+"([^"]+)"`)                          // Matches quoted C/C++ #include directives.
	cCommentRe        = regexp.MustCompile(`(?s)//[^\n]*|/\*.*?\*/`)                        // Matches C/C++ line and block comments for stripping.
	methodDeclRe      = regexp.MustCompile(`\b(?:void|bool)\s+([a-z][A-Za-z0-9_]{2,})\s*\(([^)]*)\)`) // Matches C++ void/bool method signatures.
	ifaceParamRe      = regexp.MustCompile(`\bI[A-Z][A-Za-z0-9_]+\b`)                       // Matches I-prefixed interface type names in parameter lists.
	forwardDeclLineRe = regexp.MustCompile(`^\s*(?:class|struct)\s+\w+\s*;`)                // Matches C++ forward declaration lines.
	classHeaderLineRe = regexp.MustCompile(`(?:class|struct)\s+\w+\s*:`)                    // Matches C++ class inheritance header lines.
)

// genericSymbolRoleTokens lists architectural role suffixes excluded when computing domain token overlap.
var genericSymbolRoleTokens = map[string]bool{
	"contributor": true,
	"provider":    true,
	"registry":    true,
	"adapter":     true,
	"verifier":    true,
	"board":       true,
	"module":      true,
	"manager":     true,
	"server":      true,
	"engine":      true,
}

// ignoredMethodKeywords lists C/C++ control-flow and operator keywords excluded from method extraction.
var ignoredMethodKeywords = map[string]bool{
	"if":            true,
	"for":           true,
	"while":         true,
	"switch":        true,
	"return":        true,
	"sizeof":        true,
	"alignof":       true,
	"static_assert": true,
	"catch":         true,
}

// isUpperASCII reports whether b is an uppercase ASCII letter ('A'–'Z').
func isUpperASCII(b byte) bool { return b >= 'A' && b <= 'Z' }

// isLowerASCII reports whether b is a lowercase ASCII letter ('a'–'z').
func isLowerASCII(b byte) bool { return b >= 'a' && b <= 'z' }

// isLowerOrDigitASCII reports whether b is a lowercase ASCII letter or decimal digit.
func isLowerOrDigitASCII(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// extractCamelTokens splits a PascalCase or camelCase identifier into lowercase word tokens of length >= 3.
func extractCamelTokens(s string) []string {
	// --- Step 1: Strip leading 'I' interface prefix when followed by an uppercase letter ---
	if len(s) >= 2 && s[0] == 'I' && isUpperASCII(s[1]) {
		s = s[1:]
	}

	// --- Step 2: Scan PascalCase words and acronym runs ---
	var out []string
	i := 0
	for i < len(s) {
		if !isUpperASCII(s[i]) {
			i++
			continue
		}
		if i+1 < len(s) && isLowerOrDigitASCII(s[i+1]) {
			j := i + 1
			for j < len(s) && isLowerOrDigitASCII(s[j]) {
				j++
			}
			tok := strings.ToLower(s[i:j])
			if len(tok) >= 3 {
				out = append(out, tok)
			}
			i = j
			continue
		}
		j := i + 1
		for j < len(s) && isUpperASCII(s[j]) {
			if j+1 < len(s) && isLowerASCII(s[j+1]) {
				break
			}
			j++
		}
		tok := strings.ToLower(s[i:j])
		if len(tok) >= 3 {
			out = append(out, tok)
		}
		i = j
	}
	return out
}

// isCandidateSymbol reports whether sym is a multi-token PascalCase identifier or an I*able interface name.
func isCandidateSymbol(sym string) bool {
	if len(sym) < 3 || !isUpperASCII(sym[0]) {
		return false
	}
	toks := extractCamelTokens(sym)
	if len(toks) >= 2 {
		return true
	}
	return len(sym) >= 4 && sym[0] == 'I' && isUpperASCII(sym[1]) && strings.HasSuffix(sym, "able")
}

// setActiveRepoSymbolIndex builds or retrieves the symbol index for repoRoot and sets it as active.
func setActiveRepoSymbolIndex(repoRoot string) *repoSymbolIndex {
	idx := getOrBuildRepoSymbolIndex(repoRoot)
	symIdxMu.Lock()
	activeRepoSymIdx = idx
	symIdxMu.Unlock()
	return idx
}

// getActiveRepoSymbolIndex returns the currently active repository symbol index.
func getActiveRepoSymbolIndex() *repoSymbolIndex {
	symIdxMu.RLock()
	defer symIdxMu.RUnlock()
	return activeRepoSymIdx
}

// getOrBuildRepoSymbolIndex returns the cached repoSymbolIndex for repoRoot or builds and caches a new one.
func getOrBuildRepoSymbolIndex(repoRoot string) *repoSymbolIndex {
	// --- Step 1: Check the read-locked symbol index cache ---
	cleanRoot := filepath.Clean(strings.TrimSpace(repoRoot))
	if cleanRoot == "" {
		return &repoSymbolIndex{}
	}
	symIdxMu.RLock()
	if cached, ok := symIdxByRoot[cleanRoot]; ok {
		symIdxMu.RUnlock()
		return cached
	}
	symIdxMu.RUnlock()

	// --- Step 2: Build the symbol index from disk and store in cache ---
	built := buildRepoSymbolIndex(cleanRoot)
	symIdxMu.Lock()
	symIdxByRoot[cleanRoot] = built
	symIdxMu.Unlock()
	return built
}

// buildRepoSymbolIndex scans include/ and modules/ directories under repoRoot to map symbols and methods to headers.
func buildRepoSymbolIndex(repoRoot string) *repoSymbolIndex {
	// --- Step 1: Initialize empty symbol index and validate repoRoot ---
	idx := &repoSymbolIndex{
		symbolToModule:  make(map[string]string),
		methodToHeaders: make(map[string][]string),
	}
	if repoRoot == "" {
		return idx
	}
	if fi, err := os.Stat(repoRoot); err != nil || !fi.IsDir() {
		return idx
	}

	type moduleInfo struct {
		name            string          // Module directory name.
		mainHdr         string          // Primary module header path relative to repoRoot.
		fileStems       map[string]bool // Set of filename stems present inside the module directory.
		hdrText         string          // Concatenated comment-stripped text of the primary module header.
		allText         string          // Concatenated comment-stripped text of all headers and sources in the module.
		hdrLines        []string        // Individual lines of hdrText.
		modTokens       map[string]bool // Lowercase camelCase tokens extracted from the module name.
		interfaceFanout int             // Count of "interfaces/" includes in the primary header.
		registerCount   int             // Count of register* methods declared on the module.
	}

	var modules []moduleInfo
	candidateSymSet := make(map[string]bool)
	fromTypeFile := make(map[string]bool)

	// --- Step 2: Collect contract symbols from include/interfaces/ and include/types/ headers ---
	includeRoot := filepath.Join(repoRoot, "include")
	var bestUmbrellaHdr string
	bestUmbrellaIncludes := -1
	if fi, err := os.Stat(includeRoot); err == nil && fi.IsDir() {
		var ifaceFiles, typeFiles []string
		var ifaceTxtBuilder strings.Builder
		_ = filepath.WalkDir(includeRoot, func(p string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if ext != ".hpp" && ext != ".h" && ext != ".hxx" {
				return nil
			}
			rel, relErr := filepath.Rel(repoRoot, p)
			if relErr != nil {
				return nil
			}
			relSlash := filepath.ToSlash(rel)
			relDir := filepath.ToSlash(filepath.Dir(relSlash))
			if relDir == "include" {
				if data, readErr := os.ReadFile(p); readErr == nil && len(data) <= 256*1024 {
					incCount := strings.Count(string(data), "#include")
					if incCount > bestUmbrellaIncludes {
						bestUmbrellaIncludes = incCount
						bestUmbrellaHdr = relSlash
					}
				}
			}
			if strings.HasPrefix(relSlash, "include/interfaces/") {
				ifaceFiles = append(ifaceFiles, p)
				if data, readErr := os.ReadFile(p); readErr == nil && len(data) <= 256*1024 {
					ifaceTxtBuilder.Write(data)
					ifaceTxtBuilder.WriteByte('\n')
				}
			} else if strings.HasPrefix(relSlash, "include/types/") {
				typeFiles = append(typeFiles, p)
			}
			return nil
		})

		ifaceAggTxt := ifaceTxtBuilder.String()
		for _, p := range ifaceFiles {
			stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
			if isCandidateSymbol(stem) {
				candidateSymSet[stem] = true
			}
			data, readErr := os.ReadFile(p)
			if readErr != nil || len(data) > 256*1024 {
				continue
			}
			txt := cCommentRe.ReplaceAllString(string(data), "")
			for _, m := range typeDeclRe.FindAllStringSubmatch(txt, -1) {
				sym := m[1]
				if !isCandidateSymbol(sym) {
					continue
				}
				if (sym[0] == 'I' && isUpperASCII(sym[1])) || strings.Contains(txt, "class "+sym+";") {
					candidateSymSet[sym] = true
				}
			}
		}
		for _, p := range typeFiles {
			stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
			if !strings.Contains(ifaceAggTxt, stem) {
				continue
			}
			data, readErr := os.ReadFile(p)
			if readErr != nil || len(data) > 256*1024 {
				continue
			}
			txt := cCommentRe.ReplaceAllString(string(data), "")
			for _, m := range typeDeclRe.FindAllStringSubmatch(txt, -1) {
				if isCandidateSymbol(m[1]) {
					candidateSymSet[m[1]] = true
					fromTypeFile[m[1]] = true
				}
			}
		}
	}

	// --- Step 3: Scan module directories under modules/ and src/modules/ ---
	for _, modParent := range []string{"modules", "src/modules"} {
		parentFull := filepath.Join(repoRoot, filepath.FromSlash(modParent))
		entries, err := os.ReadDir(parentFull)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			modName := ent.Name()
			modDirFull := filepath.Join(parentFull, modName)
			var mainHdr string
			for _, ext := range []string{".hpp", ".h", ".hxx"} {
				cand := filepath.ToSlash(filepath.Join(modParent, modName, modName+ext))
				if _, statErr := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(cand))); statErr == nil {
					mainHdr = cand
					break
				}
			}
			if mainHdr == "" {
				continue
			}

			mainHdrRawBytes, _ := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(mainHdr)))
			mainHdrRaw := string(mainHdrRawBytes)
			mainHdrClean := cCommentRe.ReplaceAllString(mainHdrRaw, "")

			modCamelToks := extractCamelTokens(modName)
			firstModTok := ""
			if len(modCamelToks) > 0 {
				firstModTok = modCamelToks[0]
			}
			stems := make(map[string]bool)
			var hdrBuilder, cppBuilder strings.Builder
			_ = filepath.WalkDir(modDirFull, func(p string, d os.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(d.Name()))
				stem := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
				if stem != "" {
					stems[stem] = true
					isHdr := ext == ".hpp" || ext == ".h" || ext == ".hxx"
					if isHdr && stem != modName && isCandidateSymbol(stem) {
						stemToks := extractCamelTokens(stem)
						lastTok := ""
						if len(stemToks) > 0 {
							lastTok = stemToks[len(stemToks)-1]
						}
						isRoleInterface := stem[0] == 'I' && isUpperASCII(stem[1]) && genericSymbolRoleTokens[lastTok]
						isBuiltinTools := firstModTok != "" && strings.HasPrefix(strings.ToLower(stem), firstModTok) && strings.HasSuffix(stem, "BuiltinTools")
						if isRoleInterface || isBuiltinTools {
							candidateSymSet[stem] = true
						}
					}
				}
				if ext != ".hpp" && ext != ".h" && ext != ".hxx" && ext != ".cpp" && ext != ".cc" && ext != ".c" {
					return nil
				}
				data, readErr := os.ReadFile(p)
				if readErr != nil || len(data) > 256*1024 {
					return nil
				}
				cleanSrc := cCommentRe.ReplaceAllString(string(data), "")
				if ext == ".hpp" || ext == ".h" || ext == ".hxx" {
					hdrBuilder.WriteString(cleanSrc)
					hdrBuilder.WriteByte('\n')
				} else {
					cppBuilder.WriteString(cleanSrc)
					cppBuilder.WriteByte('\n')
				}
				return nil
			})

			hdrTxt := hdrBuilder.String()
			allTxt := hdrTxt + "\n" + cppBuilder.String()

			// Extract method declarations from module headers.
			seenMethods := make(map[string]bool)
			regCnt := 0
			for _, hdrLine := range strings.Split(mainHdrClean, "\n") {
				if strings.Contains(hdrLine, "override") {
					continue
				}
				for _, m := range methodDeclRe.FindAllStringSubmatch(hdrLine, -1) {
					mName := m[1]
					mParams := m[2]
					if ignoredMethodKeywords[mName] || seenMethods[mName] {
						continue
					}
					seenMethods[mName] = true
					if strings.HasPrefix(mName, "register") {
						regCnt++
					}
					if mName == "begin" || (strings.HasPrefix(mName, "register") && !ifaceParamRe.MatchString(mParams)) {
						idx.methodToHeaders[mName] = append(idx.methodToHeaders[mName], mainHdr)
					}
				}
			}

			modToks := make(map[string]bool)
			for _, t := range modCamelToks {
				modToks[t] = true
			}

			fanout := strings.Count(mainHdrRaw, "interfaces/")
			modules = append(modules, moduleInfo{
				name:            modName,
				mainHdr:         mainHdr,
				fileStems:       stems,
				hdrText:         mainHdrClean,
				allText:         allTxt,
				hdrLines:        strings.Split(mainHdrClean, "\n"),
				modTokens:       modToks,
				interfaceFanout: fanout,
				registerCount:   regCnt,
			})
		}
	}

	// --- Step 4: Identify the top-level orchestrator module ---
	var bestOrchestratorHdr string
	bestFanout := -1
	bestOrchScore := -1
	for _, mod := range modules {
		orchScore := mod.interfaceFanout*10 + mod.registerCount*15
		if orchScore > bestOrchScore {
			bestOrchScore = orchScore
			bestFanout = mod.interfaceFanout
			bestOrchestratorHdr = mod.mainHdr
			idx.orchestratorHdr = mod.mainHdr
			idx.orchestratorStemLow = strings.ToLower(mod.name)
		}
	}

	// --- Step 5: Score and map each discovered symbol to its owning module header ---
	modNameSet := make(map[string]bool, len(modules))
	for _, mod := range modules {
		modNameSet[mod.name] = true
	}
	for sym := range candidateSymSet {
		if modNameSet[sym] {
			continue
		}
		wordRe, err := regexp.Compile(`\b` + regexp.QuoteMeta(sym) + `\b`)
		if err != nil {
			continue
		}
		inheritRe, _ := regexp.Compile(`(?s)(?:class|struct)\s+\w+\s*:[^{;]*\b` + regexp.QuoteMeta(sym) + `\b`)
		visInheritRe, _ := regexp.Compile(`^\s*(?:public|private|protected)\s+` + regexp.QuoteMeta(sym) + `\b`)

		var symToks []string
		for _, t := range extractCamelTokens(sym) {
			if !genericSymbolRoleTokens[t] {
				symToks = append(symToks, t)
			}
		}

		type scoredMod struct {
			score   int    // Relevance score for assigning sym to this module.
			fanout  int    // Number of interface headers included by this module.
			mainHdr string // Primary module header path.
		}
		var scored []scoredMod

		for _, mod := range modules {
			if mod.fileStems[sym] && sym != mod.name {
				scored = append(scored, scoredMod{score: 100, fanout: mod.interfaceFanout, mainHdr: mod.mainHdr})
				continue
			}

			prefixOverlap := 0
			for _, st := range symToks {
				if mod.modTokens[st] {
					prefixOverlap++
				}
			}

			// Do not assign cross-cutting shared interfaces without token overlap to a high-fanout composition root.
			if bestFanout >= 8 && mod.mainHdr == bestOrchestratorHdr && prefixOverlap == 0 {
				continue
			}

			inheritsSym := inheritRe != nil && inheritRe.MatchString(mod.allText)

			var nonOverrideLines []string
			if wordRe.MatchString(mod.hdrText) {
				for _, line := range mod.hdrLines {
					trim := strings.TrimSpace(line)
					if trim == "" ||
						strings.Contains(line, "override") ||
						strings.HasPrefix(trim, "#include") ||
						strings.HasPrefix(trim, "//") ||
						strings.HasPrefix(trim, "*") ||
						forwardDeclLineRe.MatchString(line) ||
						classHeaderLineRe.MatchString(line) ||
						(visInheritRe != nil && visInheritRe.MatchString(line)) {
						continue
					}
					if wordRe.MatchString(line) {
						nonOverrideLines = append(nonOverrideLines, line)
					}
				}
			}

			isIface := sym[0] == 'I' && isUpperASCII(sym[1])
			score := 0
			if inheritsSym && strings.HasSuffix(sym, "Registry") {
				score = 95
			} else if len(nonOverrideLines) > 0 {
				hasPtrOrRef := false
				for _, l := range nonOverrideLines {
					if strings.Contains(l, sym+"*") || strings.Contains(l, sym+"&") {
						hasPtrOrRef = true
						break
					}
				}
				if !inheritsSym || hasPtrOrRef {
					bonus := len(nonOverrideLines)
					if bonus > 10 {
						bonus = 10
					}
					score = 60 + bonus
				}
			} else if !isIface && wordRe.MatchString(mod.allText) && !inheritsSym {
				score = 40
			}

			if prefixOverlap > 0 && !inheritsSym {
				score += prefixOverlap * 30
			}
			if score > 0 {
				scored = append(scored, scoredMod{score: score, fanout: mod.interfaceFanout, mainHdr: mod.mainHdr})
			}
		}

		if len(scored) == 0 {
			continue
		}
		if fromTypeFile[sym] && len(scored) > 1 {
			continue
		}
		sort.Slice(scored, func(i, j int) bool {
			if scored[i].score != scored[j].score {
				return scored[i].score > scored[j].score
			}
			if scored[i].fanout != scored[j].fanout {
				return scored[i].fanout < scored[j].fanout
			}
			return scored[i].mainHdr < scored[j].mainHdr
		})

		// Prefer a dedicated subsystem module (interfaceFanout < 4) over a high-fanout composition root when available.
		best := scored[0].mainHdr
		for _, cand := range scored {
			if cand.fanout < 4 && cand.score >= 40 {
				best = cand.mainHdr
				break
			}
		}
		idx.symbolToModule[sym] = best
	}

	// --- Step 6: Populate default fallback anchors from discovered repository structure ---
	addDefault := func(p string) {
		if p == "" {
			return
		}
		for _, existing := range idx.defaultAnchors {
			if existing == p {
				return
			}
		}
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(p))); err == nil {
			idx.defaultAnchors = append(idx.defaultAnchors, p)
		}
	}
	for _, dirCand := range []string{"src/modules", "src/core"} {
		addDefault(dirCand)
	}
	addDefault(bestOrchestratorHdr)
	addDefault(bestUmbrellaHdr)
	for _, dirCand := range []string{"include/interfaces", "modules", "src", "include", "internal", "pkg", "cmd", "test", "docs/developer"} {
		addDefault(dirCand)
	}

	return idx
}

// resolveSymbol returns the owning module header path for ident, or "" if unknown.
func (s *repoSymbolIndex) resolveSymbol(ident string) string {
	if s == nil || s.symbolToModule == nil {
		return ""
	}
	return s.symbolToModule[ident]
}

// resolveObjectMethod resolves an object.method call token (e.g. "app.registerModule") to matching module headers.
func (s *repoSymbolIndex) resolveObjectMethod(obj, method string) []string {
	// --- Step 1: Validate receiver and method lookup inputs ---
	if s == nil || s.methodToHeaders == nil || method == "" {
		return nil
	}

	// --- Step 2: Match lowercase object variable prefix against declaring module headers ---
	if len(obj) >= 3 && isLowerASCII(obj[0]) {
		lowObj := strings.ToLower(obj)
		var matched []string
		for _, h := range s.methodToHeaders[method] {
			if s.orchestratorHdr != "" && h != s.orchestratorHdr {
				continue
			}
			stem := strings.TrimSuffix(filepath.Base(h), filepath.Ext(h))
			if strings.HasPrefix(strings.ToLower(stem), lowObj) {
				matched = append(matched, h)
			}
		}
		if len(matched) > 0 {
			return matched
		}
	}
	return nil
}

// fallbackAnchors returns up to maxCount valid fallback repository anchors for unanchored skill or developer docs.
func (s *repoSymbolIndex) fallbackAnchors(isValidRepoPath func(string) bool, maxCount int) []string {
	// --- Step 1: Collect discovered default anchors and generic directory fallbacks ---
	var out []string
	seen := make(map[string]bool)
	tryAdd := func(cand string) {
		if cand == "" || seen[cand] || len(out) >= maxCount {
			return
		}
		if isValidRepoPath(cand) {
			seen[cand] = true
			out = append(out, cand)
		}
	}
	if s != nil {
		for _, cand := range s.defaultAnchors {
			tryAdd(cand)
		}
	}
	for _, genericDir := range []string{"src/modules", "src/core", "include/interfaces", "modules", "src", "include", "internal", "pkg", "cmd", "test", "docs/developer"} {
		tryAdd(genericDir)
	}
	return out
}

// extractDocAnchors extracts and resolves valid repository file, directory, and symbol anchors from Markdown text.
func extractDocAnchors(text, docRelPath string, isValidRepoPath func(string) bool) []string {
	// --- Step 1: Extract direct backtick file path anchors ---
	symIdx := getActiveRepoSymbolIndex()
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

	// --- Step 2: Inspect backtick spans for headers, directories, method calls, and module/interface symbols ---
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

		// Extract base class/module/interface identifier before ::, ., or (
		ident := rawTok
		if idx := strings.Index(ident, "::"); idx > 0 {
			ident = ident[:idx]
		}
		if idx := strings.IndexByte(ident, '('); idx > 0 {
			ident = ident[:idx]
		}
		if dotIdx := strings.IndexByte(ident, '.'); dotIdx > 0 {
			obj := strings.TrimSpace(ident[:dotIdx])
			method := strings.TrimSpace(ident[dotIdx+1:])
			for _, hdr := range symIdx.resolveObjectMethod(obj, method) {
				addAnchor(hdr)
				addAnchor("src/core")
			}
			ident = obj
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
			if modHdr := symIdx.resolveSymbol(ident); modHdr != "" {
				addAnchor(modHdr)
			}
		}
	}

	// --- Step 3: Inspect Markdown links [label](target) for relative paths or file:// repo paths ---
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
			if modHdr := symIdx.resolveSymbol(baseIdent); modHdr != "" {
				addAnchor(modHdr)
			}
		}
	}

	return valid
}

// capFallbackAnchors limits anchors to maxCount, prioritizing multi-segment paths over single-segment paths.
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

// generateDocSeedID hashes relPath, entryType, and normalizedKey into a deterministic "docseed-<hex>" entry ID.
func generateDocSeedID(relPath, entryType, normalizedKey string) string {
	sum := sha256.Sum256([]byte(relPath + ":" + entryType + ":" + normalizedKey))
	return fmt.Sprintf("docseed-%x", sum[:4])
}

// shortSHA truncates a hex SHA string to at most 12 characters.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// docCandidate holds a single extracted statement or command block prior to anchor filtering and scope resolution.
type docCandidate struct {
	heading        string   // Enclosing Markdown section heading.
	text           string   // Clamped statement or command text.
	keyHint        string   // Stable key hint derived from heading and bold lead-in label.
	isRecipe       bool     // True when the candidate was derived from or enriched with a command block.
	sectionAnchors []string // Inherited anchors from the enclosing heading hierarchy.
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
	// --- Step 1: Normalize inputs and extract document-level fallback anchors ---
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	if isValidRepoPath == nil {
		isValidRepoPath = IsValidAnchorPathSyntax
	}
	if commitTime.IsZero() {
		commitTime = time.Now().UTC()
	} else {
		commitTime = commitTime.UTC()
	}

	isSkill := strings.HasSuffix(relPath, "SKILL.md") || strings.Contains(relPath, ".agents/skills/") || strings.Contains(relPath, ".agents/rules/")
	upperRel := strings.ToUpper(relPath)
	isDeveloperDoc := strings.HasPrefix(relPath, "docs/developer/") || strings.HasSuffix(upperRel, "_SETUP.MD") || strings.HasSuffix(upperRel, "_GUIDE.MD")

	// Collect valid repo paths referenced anywhere in the document (used as fallback for SKILL.md, rules, and developer guides).
	var docAnchors []string
	if isSkill || isDeveloperDoc {
		docAnchors = capFallbackAnchors(extractDocAnchors(content, relPath, isValidRepoPath), 4)
		if len(docAnchors) == 0 {
			docAnchors = getActiveRepoSymbolIndex().fallbackAnchors(isValidRepoPath, 4)
		}
		if len(docAnchors) == 0 && relPath != "" && isValidRepoPath(relPath) {
			docAnchors = []string{relPath}
		}
	}

	// --- Step 2: Scan Markdown lines to collect candidate statements, callouts, and recipes ---
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

	// --- Step 3: Filter candidates, resolve dossier scopes, and emit deterministic OpAdd operations ---
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
		if len(anchors) == 0 && len(docAnchors) > 0 {
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

		// For non-skill, non-developer docs without fallback docAnchors, strictly require at least 1 valid repo anchor.
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
			if isSkill || (isDeveloperDoc && len(anchors) == 1 && anchors[0] == relPath) {
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
	// --- Step 1: Project existing active doc-seed entries by document path and entry ID ---
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

	// --- Step 2: Resolve candidate Markdown document paths to scan ---
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

	// --- Step 3: Build repository path validator and scope resolver ---
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

	// --- Step 4: Reconcile each document against existing doc-seed entries ---
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

	// --- Step 5: Persist emitted operations batch when not in DryRun mode ---
	res.Ops = allEmittedOps

	if !opts.DryRun && opsWriter != nil && len(allEmittedOps) > 0 {
		batch := ops.NewBatch(allEmittedOps)
		if err := opsWriter.Write(batch); err != nil {
			return nil, fmt.Errorf("writing doc-seed ops batch: %w", err)
		}
	}

	return res, nil
}

// allHaveBlobSHA reports whether every entry in entries was seeded from blobSHA.
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

// skipYAMLFrontmatter returns the line index immediately following a leading YAML frontmatter block, or 0 if none.
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

// extractFirstCommandLine returns the first non-comment shell command line from a fenced code block.
func extractFirstCommandLine(lang string, fenceLines []string) string {
	// --- Step 1: Verify the fence language allows shell commands ---
	isShellFence := lang == "bash" || lang == "sh" || lang == "shell" || lang == "zsh" || lang == ""
	if !isShellFence {
		return ""
	}

	// --- Step 2: Inspect the first non-empty, non-comment line for CLI invocation patterns ---
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

// isProceduralHeading reports whether heading contains keywords indicating a procedural or command section.
func isProceduralHeading(heading string) bool {
	lower := strings.ToLower(heading)
	for _, cue := range []string{"procedure", "recipe", "workflow", "command", "usage", "verification", "build", "test", "cli", "setup"} {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

// isSelfContainedDocStatement reports whether s is a substantive, self-contained statement (>= 40 chars).
func isSelfContainedDocStatement(s string) bool {
	// --- Step 1: Enforce length and trailing semicolon filters ---
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

	// --- Step 2: Strip leading emphasis markers (* or _) to inspect the first word character ---
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

// isExcludedDocSeedTargetAnchor reports whether anchor points to experiment logs, evaluations, or PA workspace files.
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

// isBulletStart reports whether trimLine begins a Markdown unordered or numbered list item.
func isBulletStart(trimLine string) bool {
	if strings.HasPrefix(trimLine, "- ") || strings.HasPrefix(trimLine, "* ") || strings.HasPrefix(trimLine, "+ ") {
		return true
	}
	return numberedBulletRe.MatchString(trimLine)
}

// extractBulletLine strips leading list markers and task-list checkboxes from trimLine.
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

// extractKeyHint derives a stable lowercase key hint from heading and any bold lead-in label in bulletText.
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

// normalizeCandidateKey returns the normalized lowercase key hint if non-empty, or the normalized text otherwise.
func normalizeCandidateKey(keyHint, text string) string {
	if strings.TrimSpace(keyHint) != "" {
		return strings.ToLower(collapseWhitespace(keyHint))
	}
	return strings.ToLower(collapseWhitespace(text))
}

// collapseWhitespace replaces runs of whitespace in s with a single space and trims leading/trailing spaces.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// clampText collapses whitespace and truncates s to at most 400 characters with an ellipsis.
func clampText(s string) string {
	s = collapseWhitespace(s)
	const maxLen = 400
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimSpace(s[:maxLen-3]) + "..."
}

// nonEmptyOrDefault returns strings.TrimSpace(s) when non-empty, or def otherwise.
func nonEmptyOrDefault(s, def string) string {
	if strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return def
}

// filterValidAnchors filters candidates to non-excluded, syntactically valid repository paths.
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

// extractKeywords tokenizes heading and the filename stem of relPath into unique lowercase keywords.
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

// deriveDocDefaultScope derives a fallback dossier scope name from a skill directory or Markdown filename.
func deriveDocDefaultScope(relPath string) string {
	// --- Step 1: Use skill directory slug when relPath is under .agents/skills/ ---
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if strings.HasPrefix(clean, ".agents/skills/") {
		rest := strings.TrimPrefix(clean, ".agents/skills/")
		segs := strings.Split(rest, "/")
		if len(segs) > 0 && segs[0] != "" {
			return strings.ReplaceAll(segs[0], "_", "-")
		}
	}

	// --- Step 2: Normalize the Markdown filename stem as fallback scope ---
	base := strings.TrimSuffix(filepath.Base(clean), filepath.Ext(clean))
	base = strings.ToLower(strings.ReplaceAll(base, "_", "-"))
	base = strings.ReplaceAll(base, " ", "-")
	if base == "" || base == "." {
		return "docs"
	}
	return base
}

// classifyDocEntryType classifies a documentation candidate into a dossiers.EntryType based on cues in heading and text.
func classifyDocEntryType(relPath, heading, text string, isRecipeHint bool) dossiers.EntryType {
	// --- Step 1: Check for failure-signature cues ---
	combined := strings.ToLower(heading + " " + text)

	for _, cue := range []string{
		"error", "404", "429", "panic", "fails with", "symptom", "mismatch", "ratelimitexceeded", "failure signature",
	} {
		if strings.Contains(combined, cue) {
			return dossiers.EntryTypeFailureSignature
		}
	}

	// --- Step 2: Check for procedural/recipe cues ---
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

	// --- Step 3: Check for gotcha, contract, or default invariant cues ---
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

// collectRepoFileAnchors walks repoRoot and returns all non-hidden multi-segment regular file paths.
func collectRepoFileAnchors(repoRoot string) []string {
	// --- Step 1: Walk repository tree skipping hidden, vendor, and PA workspace directories ---
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

// lookupGitCommitTime queries the latest Git commit timestamp for relPath, returning fallback on failure.
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
