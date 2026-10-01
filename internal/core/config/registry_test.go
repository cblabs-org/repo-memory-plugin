package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_AddFindRemove(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(t.TempDir(), "my-repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}

	repoID := "test-repo-id-12345"

	// 1. Initial state: not found
	entry, found, err := FindRegistryEntry(dataDir, repoRoot)
	if err != nil {
		t.Fatalf("FindRegistryEntry failed: %v", err)
	}
	if found || entry != nil {
		t.Errorf("expected not found initially, got %v", entry)
	}

	// 2. Add to registry
	worktree1 := filepath.Join(t.TempDir(), "my-worktree")
	if err := os.MkdirAll(worktree1, 0755); err != nil {
		t.Fatal(err)
	}

	if err := AddToRegistry(dataDir, []string{repoRoot, worktree1}, repoID, repoRoot); err != nil {
		t.Fatalf("AddToRegistry failed: %v", err)
	}

	// 3. Find root
	entry, found, err = FindRegistryEntry(dataDir, repoRoot)
	if err != nil || !found || entry == nil {
		t.Fatalf("FindRegistryEntry for root failed: found=%v, err=%v", found, err)
	}
	if entry.RepoID != repoID || entry.RootPath != repoRoot {
		t.Errorf("got entry %+v, want RepoID=%q RootPath=%q", entry, repoID, repoRoot)
	}

	// 4. Find worktree
	entry, found, err = FindRegistryEntry(dataDir, worktree1)
	if err != nil || !found || entry == nil {
		t.Fatalf("FindRegistryEntry for worktree failed: found=%v, err=%v", found, err)
	}
	if entry.RepoID != repoID || entry.RootPath != repoRoot {
		t.Errorf("got entry %+v, want RepoID=%q RootPath=%q", entry, repoID, repoRoot)
	}

	// 5. Find subdirectory of root
	subDir := filepath.Join(repoRoot, "src", "components")
	entry, found, err = FindRegistryEntry(dataDir, subDir)
	if err != nil || !found || entry == nil {
		t.Fatalf("FindRegistryEntry for subDir failed: found=%v, err=%v", found, err)
	}
	if entry.RepoID != repoID {
		t.Errorf("got entry RepoID=%q, want %q", entry.RepoID, repoID)
	}

	// 6. Verify file format on disk
	regFile := RegistryPath(dataDir)
	data, err := os.ReadFile(regFile)
	if err != nil {
		t.Fatalf("reading registry file: %v", err)
	}
	var rawMap map[string]map[string]string
	if err := json.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("unmarshaling registry as map: %v", err)
	}
	if rawMap[repoRoot]["repo_id"] != repoID {
		t.Errorf("expected json repo_id=%q, got %q", repoID, rawMap[repoRoot]["repo_id"])
	}

	// 7. Remove from registry
	removed, err := RemoveFromRegistry(dataDir, repoRoot)
	if err != nil {
		t.Fatalf("RemoveFromRegistry failed: %v", err)
	}
	if len(removed) != 1 || removed[0] != repoID {
		t.Errorf("expected removed IDs [%s], got %v", repoID, removed)
	}

	// 8. Verify no longer found
	entry, found, err = FindRegistryEntry(dataDir, repoRoot)
	if err != nil {
		t.Fatalf("FindRegistryEntry after remove failed: %v", err)
	}
	if found || entry != nil {
		t.Errorf("expected not found after removal, got %v", entry)
	}

	entry, found, err = FindRegistryEntry(dataDir, worktree1)
	if err != nil {
		t.Fatalf("FindRegistryEntry worktree after remove failed: %v", err)
	}
	if found || entry != nil {
		t.Errorf("expected worktree not found after removal, got %v", entry)
	}
}

func TestRegistry_UnmarshalFlexibility(t *testing.T) {
	// Tests PascalCase unmarshaling support
	jsonContent := `{
		"/workspace/a": {
			"RepoID": "id-a",
			"RootPath": "/workspace/a"
		},
		"/workspace/b": {
			"repo_id": "id-b",
			"root_path": "/workspace/b"
		}
	}`

	var reg Registry
	if err := json.Unmarshal([]byte(jsonContent), &reg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if reg["/workspace/a"].RepoID != "id-a" || reg["/workspace/a"].RootPath != "/workspace/a" {
		t.Errorf("unexpected entry for /workspace/a: %+v", reg["/workspace/a"])
	}
	if reg["/workspace/b"].RepoID != "id-b" || reg["/workspace/b"].RootPath != "/workspace/b" {
		t.Errorf("unexpected entry for /workspace/b: %+v", reg["/workspace/b"])
	}
}

func TestSetMachineEnabled(t *testing.T) {
	dataDir := t.TempDir()

	// Initial default should be enabled
	cfg, err := LoadMachineConfig(dataDir)
	if err != nil {
		t.Fatalf("initial LoadMachineConfig: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected initial default Enabled to be true")
	}

	// Disable machine
	if err := SetMachineEnabled(dataDir, false); err != nil {
		t.Fatalf("SetMachineEnabled(false) failed: %v", err)
	}

	cfg, err = LoadMachineConfig(dataDir)
	if err != nil {
		t.Fatalf("LoadMachineConfig after disable: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected Enabled to be false after disable")
	}
	if cfg.HousekeepingExecutor != "agy" {
		t.Errorf("expected default HousekeepingExecutor 'agy' preserved, got %q", cfg.HousekeepingExecutor)
	}

	// Re-enable machine
	if err := SetMachineEnabled(dataDir, true); err != nil {
		t.Fatalf("SetMachineEnabled(true) failed: %v", err)
	}

	cfg, err = LoadMachineConfig(dataDir)
	if err != nil {
		t.Fatalf("LoadMachineConfig after enable: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected Enabled to be true after enable")
	}
}

func TestSetRepoEnabled(t *testing.T) {
	repoRoot := t.TempDir()

	// Initially, RepoConfig is enabled by default
	cfg, err := LoadRepoConfig(repoRoot)
	if err != nil {
		t.Fatalf("initial LoadRepoConfig: %v", err)
	}
	if !cfg.IsEnabled() {
		t.Error("expected default IsEnabled() to be true")
	}

	// Disable repo
	if err := SetRepoEnabled(repoRoot, false); err != nil {
		t.Fatalf("SetRepoEnabled(false) failed: %v", err)
	}

	cfg, err = LoadRepoConfig(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoConfig after disable: %v", err)
	}
	if cfg.IsEnabled() {
		t.Error("expected IsEnabled() to be false after disable")
	}
	if cfg.Enabled == nil || *cfg.Enabled != false {
		t.Errorf("expected *cfg.Enabled to be false, got %v", cfg.Enabled)
	}

	// Re-enable repo
	if err := SetRepoEnabled(repoRoot, true); err != nil {
		t.Fatalf("SetRepoEnabled(true) failed: %v", err)
	}

	cfg, err = LoadRepoConfig(repoRoot)
	if err != nil {
		t.Fatalf("LoadRepoConfig after enable: %v", err)
	}
	if !cfg.IsEnabled() {
		t.Error("expected IsEnabled() to be true after enable")
	}
	if cfg.Enabled == nil || *cfg.Enabled != true {
		t.Errorf("expected *cfg.Enabled to be true, got %v", cfg.Enabled)
	}
}
