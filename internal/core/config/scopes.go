/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/scopes.go
 * Description: Scope configuration loading, anchor-to-module scope derivation,
 *              split-operation alias resolution, and multi-signal scope matching.
 *
 * Exported Types / Functions:
 * - DeclaredScope:          Hand-declared scope (topic, external system, or procedure area).
 *
 *   yaml.Unmarshaler:
 *   - UnmarshalYAML():      Decodes DeclaredScope supporting both tool_names and toolNames keys.
 *
 * - ScopeConfig:            Collection of declared topics, external systems, and procedures.
 * - LoadScopeConfig():      Reads declared scopes from the `scopes:` section of config.yaml.
 * - ModuleScope:            Derived module scope rolled up from file path anchors and symbols.
 * - DeriveModuleScopes():   Rolls up file path and symbol anchors into sorted ModuleScope entries.
 * - AliasMap:               Maps split or renamed scope names to their current target scopes.
 *   - NewAliasMap():        Creates an empty AliasMap.
 *   - Add():                Registers an old scope mapping to one or more replacement scopes.
 *   - SetAlias():           Sets a direct single oldScope -> newScope alias.
 *   - HasAlias():           Reports whether the given scope has a registered alias.
 *   - Targets():            Returns all replacement dossier scopes for an old scope.
 *   - Resolve():            Follows the alias chain with cycle and depth protection.
 *   - AllAliases():         Returns all old and new scope names known to the alias map, sorted.
 * - BuildAliasMap():        Replays OpSplit operations from an ops.Reader to construct an AliasMap.
 * - ScopeResolver:          Resolves anchors and keywords across aliases, modules, and declared scopes.
 *   - NewScopeResolver():   Creates a ScopeResolver from declared scopes, derived modules, and aliases.
 *   - ResolveScope():       Resolves anchors and keywords to the highest-scoring scope name.
 *   - AllScopes():          Returns all known scope names deduplicated and sorted.
 *   - ScopeKind():          Returns the DossierKind for a scope name after alias resolution.
 */

package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"gopkg.in/yaml.v3"
)

// DeclaredScope represents a hand-declared scope (topic, external system, or procedure area).
type DeclaredScope struct {
	Name        string               `json:"name" yaml:"name"`                                   // Unique scope slug rendered into .memory/dossiers/<name>.md.
	Kind        dossiers.DossierKind `json:"kind,omitempty" yaml:"kind,omitempty"`               // Dossier kind (topic, external, or procedure).
	Keywords    []string             `json:"keywords,omitempty" yaml:"keywords,omitempty"`       // Keywords associated with this scope for matching.
	ToolNames   []string             `json:"tool_names,omitempty" yaml:"tool_names,omitempty"`   // MCP or harness tool names associated with this scope.
	Commands    []string             `json:"commands,omitempty" yaml:"commands,omitempty"`       // CLI commands associated with this scope.
	Description string               `json:"description,omitempty" yaml:"description,omitempty"` // Human-readable summary of the scope.
}

// UnmarshalYAML supports both tool_names and toolNames in YAML.
func (d *DeclaredScope) UnmarshalYAML(value *yaml.Node) error {
	// --- Step 1: Decode YAML node into an auxiliary struct accepting both snake_case and camelCase ---
	type rawDeclaredScope struct {
		Name         string               `yaml:"name"`        // Scope name.
		Kind         dossiers.DossierKind `yaml:"kind"`        // Explicit dossier kind.
		Keywords     []string             `yaml:"keywords"`    // Associated keywords.
		ToolNames    []string             `yaml:"tool_names"`  // Snake-case tool names list.
		ToolNamesAlt []string             `yaml:"toolNames"`   // Camel-case toolNames fallback list.
		Commands     []string             `yaml:"commands"`    // Associated CLI commands.
		Description  string               `yaml:"description"` // Scope description.
	}
	var raw rawDeclaredScope
	if err := value.Decode(&raw); err != nil {
		return err
	}

	// --- Step 2: Populate DeclaredScope fields and reconcile toolNames fallback ---
	d.Name = raw.Name
	d.Kind = raw.Kind
	d.Keywords = raw.Keywords
	d.ToolNames = raw.ToolNames
	if len(d.ToolNames) == 0 && len(raw.ToolNamesAlt) > 0 {
		d.ToolNames = raw.ToolNamesAlt
	}
	d.Commands = raw.Commands
	d.Description = raw.Description
	return nil
}

// ScopeConfig holds declared architectural topics, external systems, and procedure areas.
type ScopeConfig struct {
	Topics     []DeclaredScope `json:"topics,omitempty" yaml:"topics,omitempty"`         // Declared cross-cutting architectural topics.
	Externals  []DeclaredScope `json:"externals,omitempty" yaml:"externals,omitempty"`   // Declared external systems and dependencies.
	Procedures []DeclaredScope `json:"procedures,omitempty" yaml:"procedures,omitempty"` // Declared operational procedures and workflows.
}

// LoadScopeConfig reads declared scopes from config.yaml under the `scopes:` key.
// If configPath does not exist, or the `scopes:` key is missing, it returns an empty config without error.
func LoadScopeConfig(configPath string) (*ScopeConfig, error) {
	// --- Step 1: Read config file bytes or return empty ScopeConfig when absent ---
	if configPath == "" {
		return &ScopeConfig{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ScopeConfig{}, nil
		}
		return nil, fmt.Errorf("reading scope config %s: %w", configPath, err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return &ScopeConfig{}, nil
	}

	// --- Step 2: Unmarshal the top-level `scopes:` section ---
	var root struct {
		Scopes *ScopeConfig `yaml:"scopes"` // Nested scopes configuration block.
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing scope config %s: %w", configPath, err)
	}

	if root.Scopes == nil {
		return &ScopeConfig{}, nil
	}

	// --- Step 3: Default missing Kind fields based on section membership ---
	cfg := root.Scopes
	for i := range cfg.Topics {
		if cfg.Topics[i].Kind == "" {
			cfg.Topics[i].Kind = dossiers.DossierKindTopic
		}
	}
	for i := range cfg.Externals {
		if cfg.Externals[i].Kind == "" {
			cfg.Externals[i].Kind = dossiers.DossierKindExternal
		}
	}
	for i := range cfg.Procedures {
		if cfg.Procedures[i].Kind == "" {
			cfg.Procedures[i].Kind = dossiers.DossierKindProcedure
		}
	}

	return cfg, nil
}

// ModuleScope represents a derived module scope rolled up from file path anchors and symbols.
type ModuleScope struct {
	Name      string               `json:"name" yaml:"name"`                             // Derived module scope name (e.g. "core/config").
	Kind      dossiers.DossierKind `json:"kind,omitempty" yaml:"kind,omitempty"`         // Dossier kind (always DossierKindModule).
	PathGlobs []string             `json:"path_globs,omitempty" yaml:"path_globs,omitempty"` // Sorted file path globs anchored to this module.
	Symbols   []string             `json:"symbols,omitempty" yaml:"symbols,omitempty"`   // Sorted code symbols anchored to this module.
}

// parsedAnchor holds a decomposed file path and/or symbol name extracted from a raw anchor string.
type parsedAnchor struct {
	filePath string // Extracted repository-relative file path, if any.
	symbol   string // Extracted code symbol name, if any.
}

// parseAnchor decomposes a raw anchor string (such as "file:path.go", "symbol:Foo",
// "path.go#Foo", or "path.go:Foo") into its file path and symbol components.
func parseAnchor(raw string) parsedAnchor {
	// --- Step 1: Trim whitespace and check explicit file: or symbol: prefixes ---
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return parsedAnchor{}
	}

	if strings.HasPrefix(trimmed, "file:") {
		trimmed = strings.TrimPrefix(trimmed, "file:")
	}

	if strings.HasPrefix(trimmed, "symbol:") {
		return parsedAnchor{symbol: strings.TrimPrefix(trimmed, "symbol:")}
	}

	// --- Step 2: Split compound path#symbol or path:symbol anchors ---
	// Check if path#symbol
	if strings.Contains(trimmed, "#") {
		parts := strings.SplitN(trimmed, "#", 2)
		return parsedAnchor{
			filePath: strings.TrimSpace(parts[0]),
			symbol:   strings.TrimSpace(parts[1]),
		}
	}

	// Check if path:symbol (avoid drive letter like C:)
	if idx := strings.Index(trimmed, ":"); idx > 1 {
		p := strings.TrimSpace(trimmed[:idx])
		s := strings.TrimSpace(trimmed[idx+1:])
		if strings.Contains(p, "/") || strings.Contains(p, ".") {
			return parsedAnchor{
				filePath: p,
				symbol:   s,
			}
		}
	}

	return parsedAnchor{filePath: trimmed}
}

// getAnchorDir extracts the normalized directory path from a file path or glob pattern.
func getAnchorDir(filePath string) string {
	// --- Step 1: Normalize path separators and strip leading prefixes or trailing wildcards ---
	clean := filepath.ToSlash(filepath.Clean(filePath))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")

	// If glob pattern ending in /** or /*
	clean = strings.TrimSuffix(clean, "/**")
	clean = strings.TrimSuffix(clean, "/*")

	// --- Step 2: Strip filename when an extension is present ---
	if filepath.Ext(clean) != "" {
		dir := filepath.ToSlash(filepath.Dir(clean))
		if dir == "." || dir == "/" {
			return ""
		}
		return dir
	}

	if clean == "." || clean == "/" {
		return ""
	}
	return clean
}

// getParentMeaningfulDir rolls a sparse directory up to its parent directory without
// collapsing past top-level packages under internal/, cmd/, or the repository root.
func getParentMeaningfulDir(dir string) string {
	// --- Step 1: Compute immediate parent directory ---
	if dir == "" || dir == "." || dir == "/" {
		return ""
	}
	parent := filepath.ToSlash(filepath.Dir(dir))
	if parent == "." || parent == "/" {
		parent = ""
	}

	// --- Step 2: Enforce boundary stops at internal/, cmd/, and repo root ---
	if strings.HasPrefix(dir, "internal/") {
		if parent == "internal" || parent == "" {
			return dir
		}
		return parent
	}

	if strings.HasPrefix(dir, "cmd/") {
		if parent == "cmd" || parent == "" {
			return dir
		}
		return parent
	}

	if parent == "" {
		return dir
	}
	return parent
}

// deriveScopeName converts a repository directory path into a canonical hyphenated scope name.
func deriveScopeName(dir string) string {
	// --- Step 1: Strip leading internal/ prefix and normalize root ---
	clean := filepath.ToSlash(filepath.Clean(dir))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")

	var rel string
	if strings.HasPrefix(clean, "internal/") {
		rel = strings.TrimPrefix(clean, "internal/")
	} else if strings.HasPrefix(clean, "cmd/") {
		rel = clean
	} else {
		rel = clean
	}

	if rel == "" || rel == "." {
		rel = "root"
	}

	// --- Step 2: Replace underscores with hyphens in each path segment ---
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(p, "_", "-")
	}
	return strings.Join(parts, "/")
}

// DeriveModuleScopes derives module scopes by rolling up file path anchors to the nearest meaningful parent directory:
// - Strip the filename, take the directory
// - If a directory has <= 2 files, roll up to the parent
// - Use the path relative to internal/ or cmd/ or the repo root as the scope name
// - Convert path separators to / and prefix with the top-level dir (e.g. core/transcript-parser)
// Returns deduplicated, sorted module scopes.
func DeriveModuleScopes(anchors []string) []ModuleScope {
	// --- Step 1: Parse anchors and count distinct files per directory ---
	if len(anchors) == 0 {
		return nil
	}

	var parsedList []parsedAnchor
	filesInDir := make(map[string]map[string]bool)

	for _, a := range anchors {
		p := parseAnchor(a)
		if p.filePath == "" && p.symbol == "" {
			continue
		}
		parsedList = append(parsedList, p)

		if p.filePath != "" {
			d := getAnchorDir(p.filePath)
			if filesInDir[d] == nil {
				filesInDir[d] = make(map[string]bool)
			}
			filesInDir[d][p.filePath] = true
		}
	}

	if len(parsedList) == 0 {
		return nil
	}

	// --- Step 2: Roll up sparse directories (<= 2 files) and accumulate globs and symbols per scope ---
	type modAccum struct {
		name      string          // Canonical module scope name.
		pathGlobs map[string]bool // Set of normalized file paths/globs.
		symbols   map[string]bool // Set of anchored symbol names.
	}
	accumMap := make(map[string]*modAccum)

	for _, p := range parsedList {
		var targetDir string
		if p.filePath != "" {
			rawDir := getAnchorDir(p.filePath)
			fileCount := len(filesInDir[rawDir])
			if fileCount <= 2 {
				targetDir = getParentMeaningfulDir(rawDir)
			} else {
				targetDir = rawDir
			}
		}

		scopeName := deriveScopeName(targetDir)
		acc, ok := accumMap[scopeName]
		if !ok {
			acc = &modAccum{
				name:      scopeName,
				pathGlobs: make(map[string]bool),
				symbols:   make(map[string]bool),
			}
			accumMap[scopeName] = acc
		}

		if p.filePath != "" {
			cleanPath := filepath.ToSlash(filepath.Clean(p.filePath))
			cleanPath = strings.TrimPrefix(cleanPath, "./")
			cleanPath = strings.TrimPrefix(cleanPath, "/")
			acc.pathGlobs[cleanPath] = true
		}
		if p.symbol != "" {
			acc.symbols[p.symbol] = true
		}
	}

	// --- Step 3: Build and sort the resulting ModuleScope slice ---
	result := make([]ModuleScope, 0, len(accumMap))
	for _, acc := range accumMap {
		globs := make([]string, 0, len(acc.pathGlobs))
		for g := range acc.pathGlobs {
			globs = append(globs, g)
		}
		sort.Strings(globs)

		syms := make([]string, 0, len(acc.symbols))
		for s := range acc.symbols {
			syms = append(syms, s)
		}
		sort.Strings(syms)

		result = append(result, ModuleScope{
			Name:      acc.name,
			Kind:      dossiers.DossierKindModule,
			PathGlobs: globs,
			Symbols:   syms,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// AliasMap maps old scope names to new ones (from split ops).
type AliasMap struct {
	aliases map[string]string   // Primary 1:1 alias target (first replacement scope) keyed by old scope.
	targets map[string][]string // Full list of replacement dossier scopes keyed by old scope.
}

// NewAliasMap creates an empty AliasMap.
func NewAliasMap() *AliasMap {
	return &AliasMap{
		aliases: make(map[string]string),
		targets: make(map[string][]string),
	}
}

// Add registers an old scope mapping to its replacement dossiers.
func (a *AliasMap) Add(oldScope string, newDossiers []string) {
	// --- Step 1: Guard against nil receiver and lazily initialize internal maps ---
	if a == nil {
		return
	}
	if a.aliases == nil {
		a.aliases = make(map[string]string)
	}
	if a.targets == nil {
		a.targets = make(map[string][]string)
	}

	// --- Step 2: Validate inputs and record 1:N target list plus primary 1:1 alias ---
	oldScope = strings.TrimSpace(oldScope)
	if oldScope == "" || len(newDossiers) == 0 {
		return
	}

	a.targets[oldScope] = append(a.targets[oldScope], newDossiers...)
	if _, exists := a.aliases[oldScope]; !exists {
		a.aliases[oldScope] = strings.TrimSpace(newDossiers[0])
	}
}

// SetAlias sets a direct single oldScope -> newScope alias.
func (a *AliasMap) SetAlias(oldScope, newScope string) {
	// --- Step 1: Guard against nil receiver and lazily initialize internal maps ---
	if a == nil {
		return
	}
	if a.aliases == nil {
		a.aliases = make(map[string]string)
	}
	if a.targets == nil {
		a.targets = make(map[string][]string)
	}

	// --- Step 2: Normalize non-empty scope names and bind 1:1 replacement ---
	oldScope = strings.TrimSpace(oldScope)
	newScope = strings.TrimSpace(newScope)
	if oldScope == "" || newScope == "" {
		return
	}
	a.aliases[oldScope] = newScope
	a.targets[oldScope] = []string{newScope}
}

// HasAlias checks if the given scope has an alias.
func (a *AliasMap) HasAlias(scope string) bool {
	if a == nil || a.aliases == nil {
		return false
	}
	_, ok := a.aliases[strings.TrimSpace(scope)]
	return ok
}

// Targets returns all replacement dossiers for an old scope.
func (a *AliasMap) Targets(scope string) []string {
	if a == nil || a.targets == nil {
		return nil
	}
	return a.targets[strings.TrimSpace(scope)]
}

// Resolve follows the alias chain to the current scope name.
// Prevents infinite loops (max depth 10 and cycle detection).
func (a *AliasMap) Resolve(scope string) string {
	// --- Step 1: Validate receiver and normalize starting scope ---
	if a == nil || a.aliases == nil {
		return scope
	}
	current := strings.TrimSpace(scope)
	if current == "" {
		return ""
	}

	// --- Step 2: Traverse alias hops up to maxDepth while detecting cycles ---
	maxDepth := 10
	visited := make(map[string]bool)

	for depth := 0; depth < maxDepth; depth++ {
		next, exists := a.aliases[current]
		if !exists || next == "" || next == current {
			break
		}
		if visited[next] {
			break
		}
		visited[current] = true
		current = next
	}
	return current
}

// AllAliases returns all old and new scope names known to the alias map, sorted.
func (a *AliasMap) AllAliases() []string {
	// --- Step 1: Collect unique scope names across keys and target slices ---
	if a == nil {
		return nil
	}
	seen := make(map[string]bool)
	for k := range a.aliases {
		if k != "" {
			seen[k] = true
		}
	}
	for _, tgts := range a.targets {
		for _, t := range tgts {
			if t != "" {
				seen[t] = true
			}
		}
	}

	// --- Step 2: Sort and return deduplicated scope names ---
	res := make([]string, 0, len(seen))
	for s := range seen {
		res = append(res, s)
	}
	sort.Strings(res)
	return res
}

// BuildAliasMap iterates all ops from the reader, collecting split ops to build the AliasMap.
func BuildAliasMap(opsReader *ops.Reader, ctx context.Context) (*AliasMap, error) {
	// --- Step 1: Initialize empty AliasMap and guard nil reader/context ---
	aliasMap := NewAliasMap()
	if opsReader == nil {
		return aliasMap, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// --- Step 2: Stream operation batches and record all OpSplit mappings ---
	for batch, err := range opsReader.Iterate(ctx) {
		if err != nil {
			return nil, fmt.Errorf("reading ops for alias map: %w", err)
		}
		if batch == nil {
			continue
		}
		for _, op := range batch.Ops {
			if op.Type == ops.OpSplit {
				oldScope := strings.TrimSpace(op.DossierScope)
				var newDossiers []string
				if raw, ok := op.Fields["new_dossiers"].([]string); ok {
					for _, s := range raw {
						if trimmed := strings.TrimSpace(s); trimmed != "" {
							newDossiers = append(newDossiers, trimmed)
						}
					}
				} else if raw, ok := op.Fields["new_dossiers"].([]any); ok {
					for _, item := range raw {
						if s, ok := item.(string); ok {
							if trimmed := strings.TrimSpace(s); trimmed != "" {
								newDossiers = append(newDossiers, trimmed)
							}
						}
					}
				}
				if oldScope != "" && len(newDossiers) > 0 {
					aliasMap.Add(oldScope, newDossiers)
				}
			}
		}
	}

	return aliasMap, nil
}

// ScopeResolver combines declared scopes, derived module scopes, and aliases.
type ScopeResolver struct {
	config  *ScopeConfig  // Declared topics, external systems, and procedures.
	modules []ModuleScope // Derived module scopes from file and symbol anchors.
	aliases *AliasMap     // Split/rename alias table.
}

// NewScopeResolver creates a ScopeResolver.
func NewScopeResolver(config *ScopeConfig, modules []ModuleScope, aliases *AliasMap) *ScopeResolver {
	if config == nil {
		config = &ScopeConfig{}
	}
	if aliases == nil {
		aliases = NewAliasMap()
	}
	return &ScopeResolver{
		config:  config,
		modules: modules,
		aliases: aliases,
	}
}

// ResolveScope resolves anchors and keywords to the best matching scope name:
// 1. First check aliases
// 2. Then try to match anchors against module scopes (path prefix match)
// 3. Then try to match keywords against declared topics, externals, procedures
// Returns the best matching scope name, or empty string if no match.
func (r *ScopeResolver) ResolveScope(anchors []string, keywords []string) string {
	if r == nil {
		return ""
	}

	// --- Step 1: Check direct alias matches across anchors and keywords ---
	if r.aliases != nil {
		for _, a := range anchors {
			trimmed := strings.TrimSpace(a)
			if trimmed != "" && r.aliases.HasAlias(trimmed) {
				return r.aliases.Resolve(trimmed)
			}
		}
		for _, kw := range keywords {
			trimmed := strings.TrimSpace(kw)
			if trimmed != "" && r.aliases.HasAlias(trimmed) {
				return r.aliases.Resolve(trimmed)
			}
		}
	}

	// --- Step 2: Score and match anchors against derived module scopes ---
	if len(anchors) > 0 && len(r.modules) > 0 {
		var bestMod *ModuleScope
		bestScore := 0

		for _, a := range anchors {
			cleanA := filepath.ToSlash(filepath.Clean(strings.TrimSpace(a)))
			cleanA = strings.TrimPrefix(cleanA, "file:")
			cleanA = strings.TrimPrefix(cleanA, "./")
			cleanA = strings.TrimPrefix(cleanA, "/")
			if cleanA == "" || cleanA == "." {
				continue
			}

			relA := cleanA
			if strings.HasPrefix(relA, "internal/") {
				relA = strings.TrimPrefix(relA, "internal/")
			}
			slugA := strings.ReplaceAll(relA, "_", "-")

			for i := range r.modules {
				mod := &r.modules[i]
				score := 0

				// Check exact or prefix match against mod.Name
				if mod.Name != "" {
					if mod.Name == slugA || mod.Name == relA {
						score = 100
					} else if strings.HasPrefix(slugA, mod.Name+"/") || strings.HasPrefix(relA, mod.Name+"/") {
						score = 50 + len(mod.Name)*2
					}
				}

				// Check against PathGlobs
				for _, glob := range mod.PathGlobs {
					cleanG := filepath.ToSlash(filepath.Clean(strings.TrimSpace(glob)))
					cleanG = strings.TrimPrefix(cleanG, "./")
					cleanG = strings.TrimPrefix(cleanG, "/")

					if cleanA == cleanG {
						if score < 90 {
							score = 90
						}
					} else if strings.HasPrefix(cleanA, cleanG+"/") || strings.HasPrefix(cleanA, filepath.Dir(cleanG)+"/") {
						cand := 40 + len(cleanG)
						if cand > score {
							score = cand
						}
					}
				}

				// Check against Symbols
				for _, sym := range mod.Symbols {
					if strings.Contains(a, sym) {
						if score < 80 {
							score = 80
						}
					}
				}

				if score > bestScore {
					bestScore = score
					bestMod = mod
				}
			}
		}

		if bestMod != nil && bestScore > 0 {
			resolved := bestMod.Name
			if r.aliases != nil {
				resolved = r.aliases.Resolve(resolved)
			}
			return resolved
		}
	}

	// --- Step 3: Score and match keywords against declared topics, externals, and procedures ---
	if len(keywords) > 0 && r.config != nil {
		allDeclared := make([]DeclaredScope, 0, len(r.config.Topics)+len(r.config.Externals)+len(r.config.Procedures))
		allDeclared = append(allDeclared, r.config.Topics...)
		allDeclared = append(allDeclared, r.config.Externals...)
		allDeclared = append(allDeclared, r.config.Procedures...)

		var bestDeclared *DeclaredScope
		bestScore := 0

		for i := range allDeclared {
			ds := &allDeclared[i]
			score := 0

			for _, kw := range keywords {
				normKW := strings.ToLower(strings.TrimSpace(kw))
				if normKW == "" {
					continue
				}

				nameLower := strings.ToLower(ds.Name)
				if nameLower == normKW {
					score += 30
				} else if strings.Contains(nameLower, normKW) {
					score += 15
				}

				for _, k := range ds.Keywords {
					kLower := strings.ToLower(strings.TrimSpace(k))
					if kLower == normKW {
						score += 20
					} else if strings.Contains(kLower, normKW) || strings.Contains(normKW, kLower) {
						score += 8
					}
				}

				for _, t := range ds.ToolNames {
					tLower := strings.ToLower(strings.TrimSpace(t))
					if tLower == normKW {
						score += 20
					} else if strings.Contains(tLower, normKW) || strings.Contains(normKW, tLower) {
						score += 8
					}
				}

				for _, c := range ds.Commands {
					cLower := strings.ToLower(strings.TrimSpace(c))
					if cLower == normKW {
						score += 20
					} else if strings.Contains(cLower, normKW) || strings.Contains(normKW, cLower) {
						score += 8
					}
				}

				if strings.Contains(strings.ToLower(ds.Description), normKW) {
					score += 2
				}
			}

			if score > bestScore {
				bestScore = score
				bestDeclared = ds
			}
		}

		if bestDeclared != nil && bestScore > 0 {
			resolved := bestDeclared.Name
			if r.aliases != nil {
				resolved = r.aliases.Resolve(resolved)
			}
			return resolved
		}
	}

	return ""
}

// AllScopes returns all known scope names (declared + derived + from aliases), deduplicated and sorted.
func (r *ScopeResolver) AllScopes() []string {
	// --- Step 1: Collect unique scope names across declared scopes, derived modules, and aliases ---
	if r == nil {
		return nil
	}
	seen := make(map[string]bool)

	if r.config != nil {
		for _, s := range r.config.Topics {
			if s.Name != "" {
				seen[s.Name] = true
			}
		}
		for _, s := range r.config.Externals {
			if s.Name != "" {
				seen[s.Name] = true
			}
		}
		for _, s := range r.config.Procedures {
			if s.Name != "" {
				seen[s.Name] = true
			}
		}
	}

	for _, m := range r.modules {
		if m.Name != "" {
			seen[m.Name] = true
		}
	}

	if r.aliases != nil {
		for _, a := range r.aliases.AllAliases() {
			if a != "" {
				seen[a] = true
			}
		}
	}

	// --- Step 2: Sort and return deduplicated scope list ---
	res := make([]string, 0, len(seen))
	for s := range seen {
		res = append(res, s)
	}
	sort.Strings(res)
	return res
}

// ScopeKind returns the DossierKind for the given scope, taking aliases into account.
func (r *ScopeResolver) ScopeKind(scope string) dossiers.DossierKind {
	// --- Step 1: Resolve aliases for the input scope ---
	if r == nil {
		return ""
	}
	if r.aliases != nil {
		scope = r.aliases.Resolve(scope)
	}

	// --- Step 2: Lookup matching kind in derived modules and declared scope lists ---
	for _, m := range r.modules {
		if m.Name == scope {
			return dossiers.DossierKindModule
		}
	}
	if r.config != nil {
		for _, t := range r.config.Topics {
			if t.Name == scope {
				return dossiers.DossierKindTopic
			}
		}
		for _, e := range r.config.Externals {
			if e.Name == scope {
				return dossiers.DossierKindExternal
			}
		}
		for _, p := range r.config.Procedures {
			if p.Name == scope {
				return dossiers.DossierKindProcedure
			}
		}
	}
	return ""
}
