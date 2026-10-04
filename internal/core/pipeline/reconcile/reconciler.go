package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/extract"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

type Operation string

const (
	OpCreate     Operation = "create"
	OpSupport    Operation = "support"
	OpRefine     Operation = "refine"
	OpContradict Operation = "contradict"
	OpSupersede  Operation = "supersede"
	OpPromote    Operation = "promote"
	OpDiscard    Operation = "discard"
)

type ReconcileAction struct {
	Op        Operation         `json:"op"`
	Candidate extract.Candidate `json:"candidate"`
	TargetID  string            `json:"target_id,omitempty"`
	Rationale string            `json:"rationale"`
}

type ReconcileResult struct {
	Actions []ReconcileAction `json:"actions"`
}

type ReconcileInput struct {
	Candidate      extract.Candidate `json:"candidate"`
	ExistingClaims []*claims.Claim   `json:"existing_claims"`
}

type Store interface {
	ListAll() ([]*claims.Claim, error)
	Create(claim *claims.Claim) error
	Update(claim *claims.Claim) error
	Delete(id string) error
	Read(id string) (*claims.Claim, error)
}

type Reconciler struct {
	hk    harness.Housekeeper
	store Store
	mu    sync.Mutex
}

func NewReconciler(hk harness.Housekeeper, store Store) *Reconciler {
	return &Reconciler{
		hk:    hk,
		store: store,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, ep *episode.Episode, candidates []extract.Candidate) ([]ReconcileAction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var applied []ReconcileAction

	allClaims, err := r.store.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list claims: %w", err)
	}

	for _, cand := range candidates {
		existing := r.lookup(cand, allClaims)
		if len(existing) > 30 {
			existing = existing[:30]
		}

		if len(existing) == 0 {
			action := ReconcileAction{
				Op:        OpCreate,
				Candidate: cand,
				Rationale: "New claim without existing matches",
			}
			if r.applyAction(ep, action) {
				applied = append(applied, action)
			}
			continue
		}

		in := ReconcileInput{
			Candidate:      cand,
			ExistingClaims: existing,
		}

		res, err := r.hk.Run(ctx, harness.StageReconcile, in)
		if err != nil {
			return nil, fmt.Errorf("housekeeper error: %w", err)
		}

		var out ReconcileResult
		if err := json.Unmarshal([]byte(res.Payload), &out); err != nil {
			return nil, fmt.Errorf("failed to parse reconcile output: %w", err)
		}

		for _, action := range out.Actions {
			if r.applyAction(ep, action) {
				applied = append(applied, action)
			}
		}
	}

	return applied, nil
}

func (r *Reconciler) lookup(cand extract.Candidate, all []*claims.Claim) []*claims.Claim {
	var match []*claims.Claim
	seen := make(map[string]bool)

	// Anchors
	for _, candAnchor := range cand.Anchors {
		if candAnchor.Ref == "" {
			continue
		}
		for _, c := range all {
			if seen[c.ID] {
				continue
			}
			for _, a := range c.Anchors {
				if a.Ref == candAnchor.Ref {
					match = append(match, c)
					seen[c.ID] = true
					break
				}
			}
		}
	}

	// Same type with statement word overlap (or empty statement for tests)
	candWords := tokenize(cand.Statement)
	for _, c := range all {
		if seen[c.ID] {
			continue
		}
		if c.Type == cand.Type {
			if len(candWords) == 0 || hasSignificantOverlap(candWords, tokenize(c.Title)) {
				match = append(match, c)
				seen[c.ID] = true
			}
		}
	}

	return match
}

func tokenize(s string) []string {
	words := strings.Fields(strings.ToLower(s))
	var res []string
	stopWords := map[string]bool{
		"this": true, "that": true, "with": true, "from": true, "have": true,
		"were": true, "been": true, "will": true, "uses": true, "used": true,
		"using": true, "does": true, "each": true, "their": true, "when": true,
		"repo-memory": true, "repo": true, "memory": true, "transcript": true,
		"transcripts": true, "plugin": true, "internal": true,
	}
	for _, w := range words {
		w = strings.Trim(w, `.,:;!?'"()[]{}`)
		if len(w) >= 4 && !stopWords[w] {
			res = append(res, w)
		}
	}
	return res
}

func hasSignificantOverlap(a, b []string) bool {
	bMap := make(map[string]bool, len(b))
	for _, w := range b {
		bMap[w] = true
	}
	overlap := 0
	for _, w := range a {
		if bMap[w] {
			overlap++
			if overlap >= 2 {
				return true
			}
		}
	}
	return false
}

func (r *Reconciler) checkP13(ep *episode.Episode, claimID string) bool {
	if ep == nil || claimID == "" {
		return true
	}

	targetClaim, err := r.store.Read(claimID)
	if err != nil {
		return true
	}

	exposureIndex := -1
	for i, step := range ep.Steps {
		if step.Kind == harness.StepKindExposure {
			for _, id := range step.Claims {
				if id == claimID {
					exposureIndex = i
					break
				}
			}
		}
		if exposureIndex != -1 {
			break
		}
	}

	if exposureIndex == -1 {
		return true // no exposure, so independent
	}

	enforcedBy := targetClaim.Conditions["enforced_by"]
	for i := exposureIndex + 1; i < len(ep.Steps); i++ {
		step := ep.Steps[i]
		
		for _, anchor := range targetClaim.Anchors {
			if strings.Contains(step.File, anchor.Ref) || strings.Contains(step.Command, anchor.Ref) {
				return true
			}
		}

		if enforcedBy != "" && strings.Contains(step.Command, enforcedBy) {
			return true
		}
	}

	return false
}

func (r *Reconciler) applyAction(ep *episode.Episode, action ReconcileAction) bool {
	if action.Op == OpSupport {
		if !r.checkP13(ep, action.TargetID) {
			return false
		}
	}
	
	switch action.Op {
	case OpCreate:
		stmt := strings.TrimSpace(action.Candidate.Statement)
		if stmt == "" {
			return false
		}
		cType := action.Candidate.Type
		if cType == "" {
			cType = claims.TypeFact
		}
		cStatus := action.Candidate.ProposedStatus
		if cStatus == "" {
			cStatus = claims.StatusVerified
		}
		claim := &claims.Claim{
			ID:           claims.GenerateID(),
			Type:         cType,
			Title:        stmt,
			RelevantWhen: action.Candidate.RelevantWhen,
			Apply:        action.Candidate.Apply,
			Status:       cStatus,
			Anchors:      action.Candidate.Anchors,
			Evidence:     action.Candidate.Evidence,
			Created:      time.Now(),
			Updated:      time.Now(),
		}
		if len(action.Candidate.Conditions) > 0 {
			claim.Conditions = make(map[string]string)
			for _, cond := range action.Candidate.Conditions {
				claim.Conditions[cond.Target] = cond.State
			}
		}
		_ = r.store.Create(claim)

	case OpSupport:
		claim, err := r.store.Read(action.TargetID)
		if err == nil {
			claim.Evidence = append(claim.Evidence, action.Candidate.Evidence...)
			if claim.Status == claims.StatusObserved && action.Candidate.ProposedStatus == claims.StatusVerified {
				claim.Status = claims.StatusVerified
			}
			claim.Updated = time.Now()
			_ = r.store.Update(claim)
		}

	case OpRefine:
		claim, err := r.store.Read(action.TargetID)
		if err == nil {
			claim.Title = action.Candidate.Statement
			claim.RelevantWhen = action.Candidate.RelevantWhen
			claim.Apply = action.Candidate.Apply
			claim.Updated = time.Now()
			_ = r.store.Update(claim)
		}

	case OpContradict:
		claim, err := r.store.Read(action.TargetID)
		if err == nil {
			if claim.Relations == nil {
				claim.Relations = &claims.Relations{}
			}
			// we add contradict relation. Wait, contradict points to what?
			// create a new claim and mark it as contradicting the old one?
			// Let's create a new claim and contradict the old one.
			newClaim := &claims.Claim{
				ID:           claims.GenerateID(),
				Type:         action.Candidate.Type,
				Title:        action.Candidate.Statement,
				Status:       claims.StatusVerified,
				Relations:    &claims.Relations{Contradicts: []string{action.TargetID}},
				Created:      time.Now(),
				Updated:      time.Now(),
			}
			_ = r.store.Create(newClaim)
		}

	case OpSupersede:
		claim, err := r.store.Read(action.TargetID)
		if err == nil {
			claim.Status = claims.StatusSuperseded
			claim.Updated = time.Now()
			_ = r.store.Update(claim)

			newClaim := &claims.Claim{
				ID:           claims.GenerateID(),
				Type:         action.Candidate.Type,
				Title:        action.Candidate.Statement,
				Status:       action.Candidate.ProposedStatus,
				Relations:    &claims.Relations{Supersedes: []string{action.TargetID}},
				Created:      time.Now(),
				Updated:      time.Now(),
			}
			_ = r.store.Create(newClaim)
		}

	case OpPromote:
		claim, err := r.store.Read(action.TargetID)
		if err == nil {
			// check if confirmed claim is being lowered
			if claim.Status == claims.StatusConfirmed && action.Candidate.ProposedStatus != claims.StatusConfirmed {
				// queue for human review
				return false
			}
			claim.Status = action.Candidate.ProposedStatus
			claim.Updated = time.Now()
			_ = r.store.Update(claim)
		}

	case OpDiscard:
		// do nothing, discard
	}

	return true
}
