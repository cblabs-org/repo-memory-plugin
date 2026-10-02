package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

type Store interface {
	ListAll() ([]*claims.Claim, error)
	Update(claim *claims.Claim) error
	Read(id string) (*claims.Claim, error)
}

type Verifier struct {
	hk       harness.Housekeeper
	store    Store
	repoRoot string
	mu       sync.Mutex
}

func NewVerifier(hk harness.Housekeeper, store Store, repoRoot string) *Verifier {
	return &Verifier{
		hk:       hk,
		store:    store,
		repoRoot: repoRoot,
	}
}

func (v *Verifier) VerifyClaims(ctx context.Context, targetClaims []*claims.Claim) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, claim := range targetClaims {
		if claim.Scope == nil {
			claim.Scope = make(map[string]string)
		}

		// 1. Branch Scoping Logic
		branch, hasBranch := claim.Scope["branch"]
		if hasBranch && branch != "main" {
			merged := v.isBranchMergedToMain(branch)
			if merged {
				claim.Scope["branch"] = "main"
				if claim.Status == claims.StatusVerified {
					claim.Status = claims.StatusMerged
				}
			} else {
				abandoned := v.isBranchAbandoned(branch)
				if abandoned {
					if claim.Type == claims.TypeRuledOut || claim.Type == claims.TypeFailureSignature {
						claim.Scope["branch"] = "main"
					} else {
						claim.Status = claims.StatusSuperseded
						claim.Body = "Retired: branch abandoned.\n" + claim.Body
					}
				}
			}
		}

		// 2. Anchor resolution and fingerprinting
		for i, anchor := range claim.Anchors {
			fp, err := v.computeFingerprint(anchor)
			if err == nil && fp != "" {
				claim.Anchors[i].FP = fp
			}
		}

		// 3. Behavioral checks
		if (claim.Type == claims.TypeInvariant || claim.Type == claims.TypeFact || claim.Type == claims.TypeDecision) &&
			(claim.Status == claims.StatusHypothesis || claim.Status == claims.StatusObserved) {

			consistent, rationale, err := v.checkBehavior(ctx, claim)
			if err == nil {
				if !consistent {
					claim.Flags = append(claim.Flags, "needs-review")
					claim.Body += "\nBehavior check: inconsistent (" + rationale + ")"
				} else {
					claim.Status = claims.StatusVerified
				}
			}
		}

		// 4. Set verified_at commit
		head, err := v.getHeadCommit()
		if err == nil {
			claim.VerifiedAt = head
		}
		claim.Updated = time.Now()

		v.store.Update(claim)
	}

	return nil
}

func (v *Verifier) isBranchMergedToMain(branch string) bool {
	cmd := exec.Command("git", "-C", v.repoRoot, "branch", "--contains", branch)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(strings.TrimPrefix(line, "*")) == "main" {
			return true
		}
	}
	return false
}

func (v *Verifier) isBranchAbandoned(branch string) bool {
	// check if it exists
	cmd := exec.Command("git", "-C", v.repoRoot, "rev-parse", "--verify", branch)
	if err := cmd.Run(); err != nil {
		// deleted unmerged
		return true
	}

	cmd = exec.Command("git", "-C", v.repoRoot, "log", "-1", "--format=%ct", branch)
	out, err := cmd.Output()
	if err == nil {
		tsStr := strings.TrimSpace(string(out))
		if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
			if time.Now().Unix()-ts > 30*24*3600 {
				return true
			}
		}
	}
	return false
}

func (v *Verifier) computeFingerprint(anchor claims.Anchor) (string, error) {
	if anchor.Ref == "" {
		return "", fmt.Errorf("empty ref")
	}
	
	path := filepath.Join(v.repoRoot, anchor.Ref)
	if anchor.Kind == "file" || anchor.Kind == "symbol" {
		data, err := os.ReadFile(path)
		if err == nil {
			sum := sha256.Sum256(data)
			return "sha1:" + hex.EncodeToString(sum[:])[:12], nil
		}
	} else if anchor.Kind == "config" {
		return "val:unknown", nil
	}

	return "", fmt.Errorf("unsupported or missing")
}

type verifyInput struct {
	Claim    *claims.Claim `json:"claim"`
	Snippets []string      `json:"snippets"`
}

type verifyResult struct {
	Consistent bool   `json:"consistent"`
	Rationale  string `json:"rationale"`
}

func (v *Verifier) checkBehavior(ctx context.Context, claim *claims.Claim) (bool, string, error) {
	in := verifyInput{
		Claim:    claim,
		Snippets: make([]string, 0),
	}
	for _, a := range claim.Anchors {
		if a.Kind == "file" || a.Kind == "symbol" {
			data, err := os.ReadFile(filepath.Join(v.repoRoot, a.Ref))
			if err == nil {
				s := string(data)
				if len(s) > 1000 {
					s = s[:1000]
				}
				in.Snippets = append(in.Snippets, s)
			}
		}
	}

	res, err := v.hk.Run(ctx, harness.StageVerify, in)
	if err != nil {
		return false, "", err
	}

	var out verifyResult
	if err := json.Unmarshal([]byte(res.Payload), &out); err != nil {
		return false, "", err
	}

	return out.Consistent, out.Rationale, nil
}

func (v *Verifier) getHeadCommit() (string, error) {
	cmd := exec.Command("git", "-C", v.repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
