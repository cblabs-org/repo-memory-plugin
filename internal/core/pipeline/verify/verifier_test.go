package verify_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims/store"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/verify"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

func TestVerifier(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo for testing branch scoping
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "Initial commit").Run()
	exec.Command("git", "-C", tmpDir, "branch", "-M", "main").Run()
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature").Run()
	exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "Feature commit").Run()
	exec.Command("git", "-C", tmpDir, "checkout", "main").Run()
	exec.Command("git", "-C", tmpDir, "merge", "feature").Run() // merged branch

	exec.Command("git", "-C", tmpDir, "checkout", "-b", "abandoned").Run()
	exec.Command("git", "-C", tmpDir, "commit", "--allow-empty", "-m", "Abandoned commit").Run()
	exec.Command("git", "-C", tmpDir, "checkout", "main").Run()
	exec.Command("git", "-C", tmpDir, "branch", "-D", "abandoned").Run() // deleted unmerged branch

	// Create test file for anchor
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\n"), 0644)

	st, _ := store.New(filepath.Join(tmpDir, ".memory"))
	hk := fake.NewFakeHousekeeper()
	v := verify.NewVerifier(hk, st, tmpDir)

	// Claim on merged branch
	clmMerged := &claims.Claim{
		ID:     "clm-merged",
		Type:   claims.TypeFact,
		Status: claims.StatusVerified,
		Scope:  map[string]string{"branch": "feature"},
	}
	
	// Claim on abandoned branch
	clmAbandoned := &claims.Claim{
		ID:     "clm-abandoned",
		Type:   claims.TypeRuledOut,
		Status: claims.StatusVerified,
		Scope:  map[string]string{"branch": "abandoned"},
	}

	// Claim needing verification
	clmVerify := &claims.Claim{
		ID:     "clm-verify",
		Type:   claims.TypeInvariant,
		Status: claims.StatusHypothesis,
		Anchors: []claims.Anchor{{Kind: "file", Ref: "test.go"}},
	}

	st.Create(clmMerged)
	st.Create(clmAbandoned)
	st.Create(clmVerify)

	// Set Housekeeper response
	resMap := map[string]interface{}{
		"consistent": true,
		"rationale":  "looks good",
	}
	b, _ := json.Marshal(resMap)
	hk.Results[harness.StageVerify] = harness.Result{Payload: string(b)}

	err := v.VerifyClaims(context.Background(), []*claims.Claim{clmMerged, clmAbandoned, clmVerify})
	if err != nil {
		t.Fatalf("VerifyClaims error: %v", err)
	}

	// Check merged
	updatedMerged, _ := st.Read("clm-merged")
	if updatedMerged.Scope["branch"] != "main" {
		t.Errorf("expected branch main, got %v", updatedMerged.Scope["branch"])
	}
	if updatedMerged.Status != claims.StatusMerged {
		t.Errorf("expected status merged, got %v", updatedMerged.Status)
	}

	// Check abandoned
	updatedAbandoned, _ := st.Read("clm-abandoned")
	if updatedAbandoned.Scope["branch"] != "main" {
		t.Errorf("expected branch main for ruled-out on abandoned branch, got %v", updatedAbandoned.Scope["branch"])
	}

	// Check verify
	updatedVerify, _ := st.Read("clm-verify")
	if updatedVerify.Status != claims.StatusVerified {
		t.Errorf("expected status verified, got %v", updatedVerify.Status)
	}
	if updatedVerify.Anchors[0].FP == "" {
		t.Errorf("expected fingerprint to be set")
	}
	if updatedVerify.VerifiedAt == "" {
		t.Errorf("expected verified_at commit to be set")
	}
}
