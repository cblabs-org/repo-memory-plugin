package reconcile_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims/store"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/extract"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/reconcile"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

func TestReconciler(t *testing.T) {
	st, _ := store.New(t.TempDir())
	hk := fake.NewFakeHousekeeper()
	r := reconcile.NewReconciler(hk, st)

	cand := extract.Candidate{
		Type:      claims.TypeFact,
		Statement: "Test cand",
		Anchors:   []claims.Anchor{{Kind: "file", Ref: "main.go"}},
	}

	result := reconcile.ReconcileResult{
		Actions: []reconcile.ReconcileAction{
			{
				Op:        reconcile.OpCreate,
				Candidate: cand,
				Rationale: "New claim",
			},
		},
	}
	resultBytes, _ := json.Marshal(result)
	hk.Results[harness.StageReconcile] = harness.Result{Payload: string(resultBytes)}

	ep := &episode.Episode{
		Steps: []harness.Step{
			{Kind: harness.StepKindCommand, Command: "ls"},
		},
	}

	ctx := context.Background()
	actions, err := r.Reconcile(ctx, ep, []extract.Candidate{cand})
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	all, _ := st.ListAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 claim created, got %d", len(all))
	}
}

func TestReconciler_P13(t *testing.T) {
	st, _ := store.New(t.TempDir())
	hk := fake.NewFakeHousekeeper()
	r := reconcile.NewReconciler(hk, st)

	existingClaim := &claims.Claim{
		ID:    "clm-123",
		Type:  claims.TypeFact,
		Title: "Existing claim",
		Anchors: []claims.Anchor{{Kind: "file", Ref: "target.go"}},
		Conditions: map[string]string{"enforced_by": "make test"},
	}
	st.Create(existingClaim)

	cand := extract.Candidate{
		Type:      claims.TypeFact,
		Statement: "Test cand",
		Anchors:   []claims.Anchor{{Kind: "file", Ref: "target.go"}},
	}

	result := reconcile.ReconcileResult{
		Actions: []reconcile.ReconcileAction{
			{
				Op:        reconcile.OpSupport,
				Candidate: cand,
				TargetID:  "clm-123",
			},
		},
	}
	resultBytes, _ := json.Marshal(result)
	hk.Results[harness.StageReconcile] = harness.Result{Payload: string(resultBytes)}

	// No outcome touching anchor after exposure
	epFail := &episode.Episode{
		Steps: []harness.Step{
			{Kind: harness.StepKindExposure, Claims: []string{"clm-123"}},
			{Kind: harness.StepKindCommand, Command: "echo hi"},
		},
	}

	actions, _ := r.Reconcile(context.Background(), epFail, []extract.Candidate{cand})
	if len(actions) != 0 {
		t.Fatalf("P13 should block support if no valid outcome, got %d actions", len(actions))
	}

	// Outcome touched anchor file
	epSuccess := &episode.Episode{
		Steps: []harness.Step{
			{Kind: harness.StepKindExposure, Claims: []string{"clm-123"}},
			{Kind: harness.StepKindEdit, File: "target.go"},
		},
	}

	actions, _ = r.Reconcile(context.Background(), epSuccess, []extract.Candidate{cand})
	if len(actions) != 1 {
		t.Fatalf("P13 should allow support if valid outcome, got %d actions", len(actions))
	}
}

func TestReconciler_AnchorLimit(t *testing.T) {
	st, _ := store.New(t.TempDir())
	hk := fake.NewFakeHousekeeper()
	r := reconcile.NewReconciler(hk, st)

	for i := 0; i < 40; i++ {
		st.Create(&claims.Claim{
			ID:    claims.GenerateID(),
			Type:  claims.TypeFact,
		})
	}

	cand := extract.Candidate{
		Type: claims.TypeFact,
	}

	hk.Results[harness.StageReconcile] = harness.Result{Payload: `{"actions":[]}`}
	
	r.Reconcile(context.Background(), nil, []extract.Candidate{cand})
}

func TestReconciler_Operations(t *testing.T) {
	st, _ := store.New(t.TempDir())
	hk := fake.NewFakeHousekeeper()
	r := reconcile.NewReconciler(hk, st)

	targetID := "clm-ops"
	st.Create(&claims.Claim{
		ID:    targetID,
		Type:  claims.TypeFact,
		Title: "Target",
		Status: claims.StatusObserved,
	})

	actionsList := []reconcile.ReconcileAction{
		{Op: reconcile.OpRefine, TargetID: targetID, Candidate: extract.Candidate{Statement: "Refined"}},
		{Op: reconcile.OpPromote, TargetID: targetID, Candidate: extract.Candidate{ProposedStatus: claims.StatusVerified}},
		{Op: reconcile.OpContradict, TargetID: targetID, Candidate: extract.Candidate{Type: claims.TypeFact}},
		{Op: reconcile.OpSupersede, TargetID: targetID, Candidate: extract.Candidate{Type: claims.TypeFact}},
		{Op: reconcile.OpDiscard},
	}

	for _, a := range actionsList {
		res := reconcile.ReconcileResult{Actions: []reconcile.ReconcileAction{a}}
		b, _ := json.Marshal(res)
		hk.Results[harness.StageReconcile] = harness.Result{Payload: string(b)}
		
		r.Reconcile(context.Background(), nil, []extract.Candidate{a.Candidate})
	}
}
