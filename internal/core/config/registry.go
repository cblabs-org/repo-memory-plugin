/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/registry.go
 * Description: Enrolled repository registry (<data-dir>/registry.json) persistence,
 *              mtime-based in-memory caching, symlink-aware workspace lookup,
 *              and subdirectory resolution.
 *
 * Exported Types / Functions:
 * - RegistryEntry:              Metadata for an enrolled workspace in <data-dir>/registry.json.
 *
 *   json.Unmarshaler:
 *   - UnmarshalJSON():          Decodes both snake_case and legacy PascalCase JSON keys.
 *
 * - Registry:                   Map of workspace root paths to RegistryEntry values.
 * - RegistryPath():             Returns the path to registry.json inside the data directory.
 * - IsEnrolled():               Reports whether a repository root is enrolled via fast marker or registry.
 * - LoadRegistry():             Reads and caches <data-dir>/registry.json by modification time.
 * - SaveRegistry():             Atomically persists the Registry to <data-dir>/registry.json.
 * - AddToRegistry():            Registers one or more workspace paths for a repoID and rootPath.
 * - SetCreatedInstructionFile(): Updates CreatedInstructionFile on registry entries matching repoRoot.
 * - RemoveFromRegistry():       Deletes all registry entries matching repoRoot and returns removed RepoIDs.
 * - FindRegistryEntry():        Resolves a workspace or subdirectory path to its enrolled RegistryEntry.
 * - IsSubdir():                 Reports whether child is a strict subdirectory of parent.
 */

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RegistryEntry represents an enrolled repository entry in <data-dir>/registry.json.
type RegistryEntry struct {
	RepoID                 string `json:"repo_id"`                            // Stable slug derived from git origin remote or root path hash.
	RootPath               string `json:"root_path"`                          // Primary filesystem root path of the enrolled repository.
	Enabled                *bool  `json:"enabled,omitempty"`                  // Optional per-entry enable/disable override.
	CreatedInstructionFile bool   `json:"created_instruction_file,omitempty"` // Whether enrolment created GEMINI.md from scratch.
}

// UnmarshalJSON supports both snake_case (repo_id, root_path) and PascalCase (RepoID, RootPath).
func (e *RegistryEntry) UnmarshalJSON(data []byte) error {
	// --- Step 1: Decode JSON into an alias struct with legacy PascalCase fallback fields ---
	type Alias RegistryEntry
	var aux struct {
		Alias                                   // Embedded alias for standard snake_case fields.
		AltRepoID                 string `json:"RepoID"`                 // Legacy PascalCase RepoID field.
		AltRootPath               string `json:"RootPath"`               // Legacy PascalCase RootPath field.
		AltEnabled                *bool  `json:"Enabled"`                // Legacy PascalCase Enabled field.
		AltCreatedInstructionFile *bool  `json:"CreatedInstructionFile"` // Legacy PascalCase CreatedInstructionFile field.
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// --- Step 2: Populate RegistryEntry fields and apply PascalCase fallbacks ---
	*e = RegistryEntry(aux.Alias)
	if e.RepoID == "" && aux.AltRepoID != "" {
		e.RepoID = aux.AltRepoID
	}
	if e.RootPath == "" && aux.AltRootPath != "" {
		e.RootPath = aux.AltRootPath
	}
	if e.Enabled == nil && aux.AltEnabled != nil {
		e.Enabled = aux.AltEnabled
	}
	if !e.CreatedInstructionFile && aux.AltCreatedInstructionFile != nil {
		e.CreatedInstructionFile = *aux.AltCreatedInstructionFile
	}
	return nil
}

// Registry is a map of workspace paths to RegistryEntry.
type Registry map[string]RegistryEntry

// RegistryPath returns the path to registry.json within the given data directory.
func RegistryPath(dataDir string) string {
	if dataDir == "" {
		dataDir = DataDir()
	}
	return filepath.Join(dataDir, "registry.json")
}

var (
	registryCacheMu     sync.RWMutex // Guards in-memory cached registry state.
	cachedRegistry      Registry     // Last parsed Registry snapshot.
	cachedRegistryPath  string       // Filesystem path of the cached registry.json.
	cachedRegistryMtime time.Time    // Modification time of registry.json when cached.
)

// IsEnrolled reports whether repoRoot is enrolled in repo-memory.
// It checks the fast filesystem marker (.memory/.enrolled) first, and falls back
// to the cached registry lookup.
func IsEnrolled(repoRoot string) bool {
	if IsEnrolledFast(repoRoot) {
		return true
	}
	entry, found, err := FindRegistryEntry("", repoRoot)
	return err == nil && found && entry != nil
}

// LoadRegistry reads and parses <data-dir>/registry.json.
// If the file does not exist or is empty, an empty Registry is returned without error.
// Parsed results are cached by file modification time.
func LoadRegistry(dataDir string) (Registry, error) {
	// --- Step 1: Stat registry.json and handle non-existent file ---
	path := RegistryPath(dataDir)

	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			registryCacheMu.Lock()
			cachedRegistry = make(Registry)
			cachedRegistryPath = path
			cachedRegistryMtime = time.Time{}
			registryCacheMu.Unlock()
			return make(Registry), nil
		}
		return nil, fmt.Errorf("reading registry %s: %w", path, err)
	}

	// --- Step 2: Return a shallow copy if the cached mtime matches ---
	registryCacheMu.RLock()
	if cachedRegistry != nil && cachedRegistryPath == path && cachedRegistryMtime.Equal(fi.ModTime()) {
		regCopy := make(Registry, len(cachedRegistry))
		for k, v := range cachedRegistry {
			regCopy[k] = v
		}
		registryCacheMu.RUnlock()
		return regCopy, nil
	}
	registryCacheMu.RUnlock()

	// --- Step 3: Read and unmarshal registry.json from disk ---
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading registry %s: %w", path, err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return make(Registry), nil
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing registry %s: %w", path, err)
	}
	if reg == nil {
		reg = make(Registry)
	}

	// --- Step 4: Update the in-memory cache and return a defensive copy ---
	registryCacheMu.Lock()
	cachedRegistry = reg
	cachedRegistryPath = path
	cachedRegistryMtime = fi.ModTime()
	registryCacheMu.Unlock()

	regCopy := make(Registry, len(reg))
	for k, v := range reg {
		regCopy[k] = v
	}
	return regCopy, nil
}

// SaveRegistry writes the Registry to <data-dir>/registry.json atomically.
func SaveRegistry(dataDir string, reg Registry) error {
	// --- Step 1: Ensure data directory exists and marshal Registry as indented JSON ---
	if dataDir == "" {
		dataDir = DataDir()
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory %s: %w", dataDir, err)
	}

	path := RegistryPath(dataDir)
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling registry: %w", err)
	}

	// --- Step 2: Atomically write temporary file and rename over registry.json ---
	tmpFile := filepath.Join(dataDir, fmt.Sprintf("registry-%d.tmp", os.Getpid()))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("writing temporary registry %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming registry file to %s: %w", path, err)
	}

	// --- Step 3: Invalidate the in-memory registry cache ---
	registryCacheMu.Lock()
	cachedRegistry = nil
	registryCacheMu.Unlock()

	return nil
}

// AddToRegistry records workspace paths mapping to the given repoID and rootPath.
func AddToRegistry(dataDir string, workspacePaths []string, repoID, rootPath string) error {
	// --- Step 1: Load existing registry and preserve flags from prior registration ---
	reg, err := LoadRegistry(dataDir)
	if err != nil {
		return err
	}

	cleanRoot := filepath.Clean(rootPath)
	evalRoot, _ := filepath.EvalSymlinks(cleanRoot)

	entry := RegistryEntry{
		RepoID:   repoID,
		RootPath: cleanRoot,
	}
	if existing, ok := reg[cleanRoot]; ok {
		entry.Enabled = existing.Enabled
		entry.CreatedInstructionFile = existing.CreatedInstructionFile
	} else if evalRoot != "" {
		if existing, ok := reg[evalRoot]; ok {
			entry.Enabled = existing.Enabled
			entry.CreatedInstructionFile = existing.CreatedInstructionFile
		}
	}

	// --- Step 2: Register canonical and symlink-resolved paths for root and workspaces ---
	reg[cleanRoot] = entry
	if evalRoot != "" && evalRoot != cleanRoot {
		reg[evalRoot] = entry
	}

	for _, ws := range workspacePaths {
		cleanWs := filepath.Clean(ws)
		reg[cleanWs] = entry
		if evalWs, err := filepath.EvalSymlinks(cleanWs); err == nil && evalWs != cleanWs {
			reg[evalWs] = entry
		}
	}

	return SaveRegistry(dataDir, reg)
}

// SetCreatedInstructionFile updates CreatedInstructionFile on registry entries matching repoRoot.
func SetCreatedInstructionFile(dataDir, repoRoot string, created bool) error {
	// --- Step 1: Load registry and normalize target root path ---
	reg, err := LoadRegistry(dataDir)
	if err != nil {
		return err
	}

	cleanRoot := filepath.Clean(repoRoot)
	evalRoot, _ := filepath.EvalSymlinks(cleanRoot)

	// --- Step 2: Match entries by direct or symlink-evaluated root path and update flag ---
	updated := false
	for k, entry := range reg {
		kClean := filepath.Clean(k)
		kEval, _ := filepath.EvalSymlinks(kClean)
		rootClean := filepath.Clean(entry.RootPath)
		rootEval, _ := filepath.EvalSymlinks(rootClean)

		matches := kClean == cleanRoot ||
			(evalRoot != "" && kClean == evalRoot) ||
			(kEval != "" && kEval == cleanRoot) ||
			(kEval != "" && evalRoot != "" && kEval == evalRoot) ||
			rootClean == cleanRoot ||
			(evalRoot != "" && rootClean == evalRoot) ||
			(rootEval != "" && rootEval == cleanRoot) ||
			(rootEval != "" && evalRoot != "" && rootEval == evalRoot)

		if matches {
			entry.CreatedInstructionFile = entry.CreatedInstructionFile || created
			reg[k] = entry
			updated = true
		}
	}

	if !updated {
		return nil
	}
	return SaveRegistry(dataDir, reg)
}

// RemoveFromRegistry removes any entries matching repoRoot (or its worktrees) from registry.json.
// Returns the list of distinct RepoIDs removed.
func RemoveFromRegistry(dataDir, repoRoot string) ([]string, error) {
	// --- Step 1: Load registry and normalize target root path ---
	reg, err := LoadRegistry(dataDir)
	if err != nil {
		return nil, err
	}

	cleanRoot := filepath.Clean(repoRoot)
	evalRoot, _ := filepath.EvalSymlinks(cleanRoot)

	var removedIDs []string
	seen := make(map[string]bool)

	// --- Step 2: Delete all entries matching repoRoot directly or via symlinks ---
	for k, entry := range reg {
		kClean := filepath.Clean(k)
		kEval, _ := filepath.EvalSymlinks(kClean)
		rootClean := filepath.Clean(entry.RootPath)
		rootEval, _ := filepath.EvalSymlinks(rootClean)

		matches := kClean == cleanRoot ||
			(evalRoot != "" && kClean == evalRoot) ||
			(kEval != "" && kEval == cleanRoot) ||
			(kEval != "" && evalRoot != "" && kEval == evalRoot) ||
			rootClean == cleanRoot ||
			(evalRoot != "" && rootClean == evalRoot) ||
			(rootEval != "" && rootEval == cleanRoot) ||
			(rootEval != "" && evalRoot != "" && rootEval == evalRoot)

		if matches {
			if entry.RepoID != "" && !seen[entry.RepoID] {
				seen[entry.RepoID] = true
				removedIDs = append(removedIDs, entry.RepoID)
			}
			delete(reg, k)
		}
	}

	// --- Step 3: Persist updated registry ---
	if err := SaveRegistry(dataDir, reg); err != nil {
		return nil, err
	}

	return removedIDs, nil
}

// FindRegistryEntry looks up a workspace path in the registry.
// Returns (*RegistryEntry, true, nil) if found, (nil, false, nil) if not enrolled.
func FindRegistryEntry(dataDir, workspacePath string) (*RegistryEntry, bool, error) {
	// --- Step 1: Load registry and check direct path match ---
	reg, err := LoadRegistry(dataDir)
	if err != nil {
		return nil, false, err
	}

	cleanPath := filepath.Clean(workspacePath)

	if entry, ok := reg[cleanPath]; ok {
		return &entry, true, nil
	}

	// --- Step 2: Check symlink-evaluated path match ---
	evalPath, err := filepath.EvalSymlinks(cleanPath)
	if err == nil && evalPath != cleanPath {
		if entry, ok := reg[evalPath]; ok {
			return &entry, true, nil
		}
	}

	// --- Step 3: Check whether workspacePath is a subdirectory of any registered root ---
	for regPath, entry := range reg {
		if isSubdir(cleanPath, regPath) {
			return &entry, true, nil
		}
		if evalPath != "" && isSubdir(evalPath, regPath) {
			return &entry, true, nil
		}
	}

	return nil, false, nil
}

// IsSubdir reports whether child is a subdirectory of parent.
func IsSubdir(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..") && !strings.HasPrefix(rel, "/") && !strings.HasPrefix(rel, `\`)
}

// isSubdir delegates to IsSubdir for package-internal callers.
func isSubdir(child, parent string) bool {
	return IsSubdir(child, parent)
}
