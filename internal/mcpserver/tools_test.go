/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package mcpserver_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/mcpserver"
)

func TestHandleMemoryBrief(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	// Insert test entries
	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "inv-1",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "auth",
		Text:         "All endpoints require JWT authorization token.",
	})
	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "dec-1",
		Type:         dossiers.EntryTypeDecision,
		DossierScope: "auth",
		Text:         "Use RS256 for asymmetric signature validation.",
	})
	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "rul-1",
		Type:         dossiers.EntryTypeRuledOut,
		DossierScope: "auth",
		Text:         "Basic auth is prohibited.",
		Retired:      true,
		RetiredReason: "insecure",
	})
	_ = idx.InsertHistory(ctx, dossiers.Entry{
		ID:            "rul-1",
		Type:          dossiers.EntryTypeRuledOut,
		DossierScope:  "auth",
		Text:          "Basic auth is prohibited.",
		RetiredReason: "insecure",
	}, "insecure")

	ledger := delivery.NewLedger("test-conv-brief")
	srv := mcpserver.NewServer(idx, mcpserver.WithLedger(ledger))

	t.Run("valid scope brief", func(t *testing.T) {
		resp, err := srv.HandleMemoryBrief(ctx, mcpserver.MemoryBriefRequest{
			Scope: "auth",
		})
		if err != nil {
			t.Fatalf("HandleMemoryBrief failed: %v", err)
		}

		if !strings.Contains(resp.Result, "Scope: auth (2 entries)") {
			t.Errorf("expected header with 2 active entries, got: %s", resp.Result)
		}
		if !strings.Contains(resp.Result, "[invariant] inv-1: All endpoints require JWT authorization token.") {
			t.Errorf("expected inv-1 in brief, got: %s", resp.Result)
		}
		if !strings.Contains(resp.Result, "[decision] dec-1: Use RS256 for asymmetric signature validation.") {
			t.Errorf("expected dec-1 in brief, got: %s", resp.Result)
		}
		if strings.Contains(resp.Result, "rul-1") {
			t.Errorf("retired entry rul-1 should not appear in brief, got: %s", resp.Result)
		}

		// Verify exposure ledger recorded pulls
		exposures := ledger.GetExposureList()
		if len(exposures) != 2 {
			t.Fatalf("expected 2 ledger exposures, got %d", len(exposures))
		}
		if exposures[0].EntryID != "dec-1" || exposures[1].EntryID != "inv-1" {
			t.Errorf("unexpected exposures: %+v", exposures)
		}
	})

	t.Run("truncation over 200 chars", func(t *testing.T) {
		longText := strings.Repeat("A", 250)
		_ = idx.UpsertEntry(ctx, dossiers.Entry{
			ID:           "inv-long",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "long-scope",
			Text:         longText,
		})

		resp, err := srv.HandleMemoryBrief(ctx, mcpserver.MemoryBriefRequest{
			Scope: "long-scope",
		})
		if err != nil {
			t.Fatalf("HandleMemoryBrief failed: %v", err)
		}
		expectedSnippet := strings.Repeat("A", 200)
		if !strings.Contains(resp.Result, expectedSnippet) {
			t.Errorf("expected 200 rune truncation, got: %s", resp.Result)
		}
		if strings.Contains(resp.Result, strings.Repeat("A", 201)) {
			t.Errorf("expected text to not exceed 200 chars")
		}
	})

	t.Run("missing scope error", func(t *testing.T) {
		_, err := srv.HandleMemoryBrief(ctx, mcpserver.MemoryBriefRequest{})
		if err == nil {
			t.Fatal("expected error on empty scope")
		}
	})
}

func TestHandleMemoryBrief_AliasResolution(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "inv-sec",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "security",
		Text:         "TLS 1.3 is enforced.",
	})
	_ = idx.UpsertAlias(ctx, "sec-alias", "security")

	srv := mcpserver.NewServer(idx)
	resp, err := srv.HandleMemoryBrief(ctx, mcpserver.MemoryBriefRequest{
		Scope: "sec-alias",
	})
	if err != nil {
		t.Fatalf("HandleMemoryBrief failed: %v", err)
	}

	if !strings.Contains(resp.Result, "Scope: security") {
		t.Errorf("expected resolved scope security in header, got: %s", resp.Result)
	}
	if !strings.Contains(resp.Result, "inv-sec: TLS 1.3 is enforced.") {
		t.Errorf("expected inv-sec entry in brief, got: %s", resp.Result)
	}
}

func TestHandleMemoryWhy(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	now := time.Now().UTC().Truncate(time.Second)
	entry := dossiers.Entry{
		ID:           "dec-42",
		Type:         dossiers.EntryTypeDecision,
		DossierScope: "database",
		Text:         "Adopted SQLite WAL mode for high concurrent read performance.",
		Anchors:      []string{"internal/core/index/index.go"},
		Source:       "PR-42 (architecture review)",
		Verified:     "verified",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatalf("failed to upsert entry: %v", err)
	}

	// Add history entry
	histEntry := dossiers.Entry{
		ID:            "dec-42",
		Type:          dossiers.EntryTypeDecision,
		DossierScope:  "database",
		Text:          "Initial design proposed MySQL.",
		RetiredReason: "complexity too high for single process daemon",
		SupersededBy:  "dec-42",
		UpdatedAt:     now.Add(-time.Hour),
	}
	if err := idx.InsertHistory(ctx, histEntry, histEntry.RetiredReason); err != nil {
		t.Fatalf("failed to insert history: %v", err)
	}

	ledger := delivery.NewLedger("test-conv-why")
	srv := mcpserver.NewServer(idx, mcpserver.WithLedger(ledger))

	t.Run("complete why output", func(t *testing.T) {
		resp, err := srv.HandleMemoryWhy(ctx, mcpserver.MemoryWhyRequest{
			EntryID: "dec-42",
		})
		if err != nil {
			t.Fatalf("HandleMemoryWhy failed: %v", err)
		}

		result := resp.Result
		if !strings.Contains(result, "Entry: dec-42") {
			t.Errorf("missing entry ID in output: %s", result)
		}
		if !strings.Contains(result, "Type: decision") {
			t.Errorf("missing entry type in output: %s", result)
		}
		if !strings.Contains(result, "Scope: database") {
			t.Errorf("missing scope in output: %s", result)
		}
		if !strings.Contains(result, "Text: Adopted SQLite WAL mode") {
			t.Errorf("missing text in output: %s", result)
		}
		if !strings.Contains(result, "Anchors: internal/core/index/index.go") {
			t.Errorf("missing anchors in output: %s", result)
		}
		if !strings.Contains(result, "Evidence: PR-42 (architecture review)") {
			t.Errorf("missing evidence/source in output: %s", result)
		}
		if !strings.Contains(result, "Verification Status: verified") {
			t.Errorf("missing verification status in output: %s", result)
		}
		if !strings.Contains(result, "History:") {
			t.Errorf("missing history section in output: %s", result)
		}
		if !strings.Contains(result, "Initial design proposed MySQL.") {
			t.Errorf("missing history entry text: %s", result)
		}
		if !strings.Contains(result, "complexity too high for single process daemon") {
			t.Errorf("missing retired reason: %s", result)
		}

		// Verify exposure ledger recorded pull
		exposures := ledger.GetExposureList()
		if len(exposures) != 1 || exposures[0].EntryID != "dec-42" {
			t.Errorf("unexpected exposure record: %+v", exposures)
		}
	})

	t.Run("missing entryId error", func(t *testing.T) {
		_, err := srv.HandleMemoryWhy(ctx, mcpserver.MemoryWhyRequest{})
		if err == nil {
			t.Fatal("expected error on empty entryId")
		}
	})

	t.Run("entry not found", func(t *testing.T) {
		_, err := srv.HandleMemoryWhy(ctx, mcpserver.MemoryWhyRequest{EntryID: "non-existent"})
		if err == nil {
			t.Fatal("expected error for non-existent entry")
		}
	})
}

func TestHandleMemorySearch(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "dec-01",
		Type:         dossiers.EntryTypeDecision,
		DossierScope: "db",
		Text:         "We chose PostgreSQL for ACID compliance in persistence.",
	})
	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "inv-01",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "db",
		Text:         "Database migrations must always be backward compatible.",
	})
	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "dec-02",
		Type:         dossiers.EntryTypeDecision,
		DossierScope: "cache",
		Text:         "Redis is used for temporary cache tokens.",
	})

	ledger := delivery.NewLedger("test-conv-search")
	srv := mcpserver.NewServer(idx, mcpserver.WithLedger(ledger))

	t.Run("search query match", func(t *testing.T) {
		resp, err := srv.HandleMemorySearch(ctx, mcpserver.MemorySearchRequest{
			Query: "PostgreSQL OR migrations",
		})
		if err != nil {
			t.Fatalf("HandleMemorySearch failed: %v", err)
		}

		if !strings.Contains(resp.Result, "dec-01") {
			t.Errorf("expected dec-01 in search result, got: %s", resp.Result)
		}
		if !strings.Contains(resp.Result, "inv-01") {
			t.Errorf("expected inv-01 in search result, got: %s", resp.Result)
		}
		if strings.Contains(resp.Result, "dec-02") {
			t.Errorf("did not expect dec-02 in search result, got: %s", resp.Result)
		}

		// Verify ranked format
		if !strings.Contains(resp.Result, "1. [") {
			t.Errorf("expected ranked list format '1. [', got: %s", resp.Result)
		}

		// Verify ledger exposures
		exposures := ledger.GetExposureList()
		if len(exposures) != 2 {
			t.Errorf("expected 2 exposures, got %d", len(exposures))
		}
	})

	t.Run("empty or no match", func(t *testing.T) {
		resp, err := srv.HandleMemorySearch(ctx, mcpserver.MemorySearchRequest{
			Query: "xyznonexistentword",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Result != "No results found." {
			t.Errorf("expected 'No results found.', got: %s", resp.Result)
		}

		respEmpty, err := srv.HandleMemorySearch(ctx, mcpserver.MemorySearchRequest{Query: ""})
		if err != nil {
			t.Fatalf("unexpected error on empty: %v", err)
		}
		if respEmpty.Result != "No results found." {
			t.Errorf("expected 'No results found.', got: %s", respEmpty.Result)
		}
	})
}

func TestHandleMemoryHistory(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	// Insert history entries
	histEntry := dossiers.Entry{
		ID:            "rul-10",
		Type:          dossiers.EntryTypeRuledOut,
		DossierScope:  "networking",
		Text:          "HTTP/1.0 protocol was considered.",
		RetiredReason: "HTTP/2 is required for multiplexing",
		SupersededBy:  "inv-20",
		UpdatedAt:     time.Now().UTC(),
	}
	if err := idx.InsertHistory(ctx, histEntry, histEntry.RetiredReason); err != nil {
		t.Fatalf("failed to insert history: %v", err)
	}

	// Insert an active entry in the same scope
	_ = idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "inv-20",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "networking",
		Text:         "All connections must use HTTP/2 or HTTP/3.",
	})

	ledger := delivery.NewLedger("test-conv-history")
	srv := mcpserver.NewServer(idx, mcpserver.WithLedger(ledger))

	t.Run("filter to retired and superseded entries", func(t *testing.T) {
		resp, err := srv.HandleMemoryHistory(ctx, mcpserver.MemoryHistoryRequest{
			Scope: "networking",
		})
		if err != nil {
			t.Fatalf("HandleMemoryHistory failed: %v", err)
		}

		if !strings.Contains(resp.Result, "rul-10") {
			t.Errorf("expected rul-10 in history, got: %s", resp.Result)
		}
		if !strings.Contains(resp.Result, "HTTP/2 is required for multiplexing") {
			t.Errorf("expected retired reason in history, got: %s", resp.Result)
		}
		if !strings.Contains(resp.Result, "superseded by: inv-20") {
			t.Errorf("expected superseded-by in history, got: %s", resp.Result)
		}
		// Active entry without history should not appear as retired entry
		if strings.Contains(resp.Result, "inv-20: All connections must use HTTP/2") {
			t.Errorf("active entry inv-20 should not be listed as retired entry, got: %s", resp.Result)
		}

		// Ledger exposure
		exposures := ledger.GetExposureList()
		if len(exposures) != 1 || exposures[0].EntryID != "rul-10" {
			t.Errorf("unexpected exposure record: %+v", exposures)
		}
	})

	t.Run("alias resolution in history", func(t *testing.T) {
		_ = idx.UpsertAlias(ctx, "net-alias", "networking")

		resp, err := srv.HandleMemoryHistory(ctx, mcpserver.MemoryHistoryRequest{
			Scope: "net-alias",
		})
		if err != nil {
			t.Fatalf("HandleMemoryHistory with alias failed: %v", err)
		}
		if !strings.Contains(resp.Result, "rul-10") {
			t.Errorf("expected rul-10 in history resolved from alias, got: %s", resp.Result)
		}
	})

	t.Run("missing scope error", func(t *testing.T) {
		_, err := srv.HandleMemoryHistory(ctx, mcpserver.MemoryHistoryRequest{})
		if err == nil {
			t.Fatal("expected error on empty scope")
		}
	})
}

func TestHandleMemoryNote(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	opsDir := t.TempDir()
	srv := mcpserver.NewServer(idx, mcpserver.WithOpsDir(opsDir))

	t.Run("valid note creation", func(t *testing.T) {
		resp, err := srv.HandleMemoryNote(ctx, mcpserver.MemoryNoteRequest{
			Scope: "billing",
			Text:  "Stripe is selected as the primary payment gateway.",
			Type:  "decision",
		})
		if err != nil {
			t.Fatalf("HandleMemoryNote failed: %v", err)
		}

		if !strings.HasPrefix(resp.Result, "Note recorded: dec-") {
			t.Errorf("expected result to start with 'Note recorded: dec-', got: %s", resp.Result)
		}
		entryID := strings.TrimPrefix(resp.Result, "Note recorded: ")

		// Verify op log batch file was written
		reader := ops.NewReader(opsDir)
		var batches []*ops.OpBatch
		for batch, err := range reader.Iterate(ctx) {
			if err != nil {
				t.Fatalf("reading ops: %v", err)
			}
			batches = append(batches, batch)
		}

		if len(batches) != 1 {
			t.Fatalf("expected 1 batch, got %d", len(batches))
		}
		batch := batches[0]
		if len(batch.Ops) != 1 {
			t.Fatalf("expected 1 op in batch, got %d", len(batch.Ops))
		}
		op := batch.Ops[0]
		if op.Type != ops.OpAdd {
			t.Errorf("expected op.Type OpAdd, got %s", op.Type)
		}
		if op.EntryID != entryID {
			t.Errorf("expected op.EntryID %s, got %s", entryID, op.EntryID)
		}
		if op.Source != "agent" {
			t.Errorf("expected op.Source 'agent', got %s", op.Source)
		}
		if op.DossierScope != "billing" {
			t.Errorf("expected dossier scope 'billing', got %s", op.DossierScope)
		}
		if op.Fields["text"] != "Stripe is selected as the primary payment gateway." {
			t.Errorf("unexpected text in fields: %v", op.Fields["text"])
		}

		// Verify entry was also upserted to index for immediate querying
		entry, err := idx.GetEntry(ctx, entryID)
		if err != nil {
			t.Fatalf("failed to retrieve entry from index: %v", err)
		}
		if entry.Text != "Stripe is selected as the primary payment gateway." {
			t.Errorf("unexpected index text: %s", entry.Text)
		}
	})

	t.Run("default type is decision", func(t *testing.T) {
		resp, err := srv.HandleMemoryNote(ctx, mcpserver.MemoryNoteRequest{
			Scope: "infra",
			Text:  "Kubernetes 1.30 used for deployment.",
		})
		if err != nil {
			t.Fatalf("HandleMemoryNote failed: %v", err)
		}
		if !strings.HasPrefix(resp.Result, "Note recorded: dec-") {
			t.Errorf("expected default decision prefix dec-, got: %s", resp.Result)
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		// Missing scope
		_, err := srv.HandleMemoryNote(ctx, mcpserver.MemoryNoteRequest{
			Text: "Some text",
		})
		if err == nil {
			t.Error("expected error for missing scope")
		}

		// Missing text
		_, err = srv.HandleMemoryNote(ctx, mcpserver.MemoryNoteRequest{
			Scope: "infra",
		})
		if err == nil {
			t.Error("expected error for missing text")
		}

		// Invalid type
		_, err = srv.HandleMemoryNote(ctx, mcpserver.MemoryNoteRequest{
			Scope: "infra",
			Text:  "Some text",
			Type:  "invalid-type",
		})
	})
}

func TestServerOptionsAndSetters(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	opsDir := t.TempDir()
	opsWriter := ops.NewWriter(opsDir)
	ledger := delivery.NewLedger("test-conv-options")

	// Test WithOpsWriter option
	srv := mcpserver.NewServer(idx, mcpserver.WithOpsWriter(opsWriter))
	srv.SetLedger(ledger)
	srv.SetOpsWriter(opsWriter)

	resp, err := srv.HandleMemoryNote(ctx, mcpserver.MemoryNoteRequest{
		Scope: "ops",
		Text:  "Testing WithOpsWriter option",
	})
	if err != nil {
		t.Fatalf("HandleMemoryNote failed: %v", err)
	}
	if !strings.HasPrefix(resp.Result, "Note recorded: dec-") {
		t.Errorf("unexpected note result: %s", resp.Result)
	}
}

func TestHandleMemoryHistory_Empty(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	srv := mcpserver.NewServer(idx)
	resp, err := srv.HandleMemoryHistory(ctx, mcpserver.MemoryHistoryRequest{
		Scope: "empty-scope",
	})
	if err != nil {
		t.Fatalf("HandleMemoryHistory failed: %v", err)
	}
	if !strings.Contains(resp.Result, "No retired or superseded entries found") {
		t.Errorf("expected empty message, got: %s", resp.Result)
	}
}

func TestShortTermRecallAndCoverageGaps_PullTools(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-short-term-test"
	inboxStore := inbox.NewStore(dataDir, repoID)

	// Write an unconsolidated inbox note for scope "payments" on branch "feat/checkout"
	note := &inbox.Note{
		Source:         inbox.SourceTurnNote,
		ConversationID: "conv-123",
		Branch:         "feat/checkout",
		Items: []inbox.NoteItem{
			{
				Type:     dossiers.EntryTypeGotcha,
				Title:    "Stripe webhook signature header",
				Why:      "Raw body is mutated by JSON middleware before signature check",
				Do:       "Read raw body bytes before unmarshaling",
				Anchors:  []string{"internal/payments/webhook.go"},
				Evidence: "Verified in webhook_test.go",
				Scope:    "payments",
			},
		},
	}
	if _, err := inboxStore.Write(note); err != nil {
		t.Fatalf("inboxStore.Write failed: %v", err)
	}

	// Add a suspect entry in scope "legacy" to trigger low coverage gap line
	if err := idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "got-legacy-1",
		Type:         dossiers.EntryTypeGotcha,
		DossierScope: "legacy",
		Text:         "Legacy adapter timeout",
		StaleSuspect: true,
	}); err != nil {
		t.Fatal(err)
	}

	srv := mcpserver.NewServer(
		idx,
		mcpserver.WithInboxStore(inboxStore),
		mcpserver.WithRepoContext(dataDir, repoID, repoRoot),
	)

	// 1. HandleMemoryBrief on "payments": should include 1-line A6 freshness header + unconsolidated note
	briefPayments, err := srv.HandleMemoryBrief(ctx, mcpserver.MemoryBriefRequest{Scope: "payments"})
	if err != nil {
		t.Fatalf("HandleMemoryBrief(payments) failed: %v", err)
	}
	for _, want := range []string{
		"memory: index current to op-log · HEAD matches last build · 1 notes not yet consolidated · 1 entries suspect",
		"Unconsolidated Inbox Notes (unconsolidated):",
		"[unconsolidated " + note.ID + "] (feat/checkout) gotcha: Stripe webhook signature header — Raw body is mutated by JSON middleware before signature check / Read raw body bytes before unmarshaling",
	} {
		if !strings.Contains(briefPayments.Result, want) {
			t.Errorf("HandleMemoryBrief(payments) missing %q in:\n%s", want, briefPayments.Result)
		}
	}

	// 2. HandleMemoryBrief on uncovered scope "unknown-mod": should include explicit SPEC §5.2 gap line
	briefUnknown, err := srv.HandleMemoryBrief(ctx, mcpserver.MemoryBriefRequest{Scope: "unknown-mod"})
	if err != nil {
		t.Fatalf("HandleMemoryBrief(unknown-mod) failed: %v", err)
	}
	wantNoMem := "No memory covers unknown-mod. Absence of memory is not evidence; read the code."
	if !strings.Contains(briefUnknown.Result, wantNoMem) {
		t.Errorf("HandleMemoryBrief(unknown-mod) missing %q in:\n%s", wantNoMem, briefUnknown.Result)
	}

	// 3. HandleMemoryBrief on low-coverage scope "legacy": should include low-coverage gap line
	briefLegacy, err := srv.HandleMemoryBrief(ctx, mcpserver.MemoryBriefRequest{Scope: "legacy"})
	if err != nil {
		t.Fatalf("HandleMemoryBrief(legacy) failed: %v", err)
	}
	wantLowCov := "Low memory coverage for legacy (score 0.00). Absence of memory is not evidence; read the code."
	if !strings.Contains(briefLegacy.Result, wantLowCov) {
		t.Errorf("HandleMemoryBrief(legacy) missing %q in:\n%s", wantLowCov, briefLegacy.Result)
	}

	// 4. HandleMemoryWhy on unconsolidated note ID: should return Status: unconsolidated
	whyResp, err := srv.HandleMemoryWhy(ctx, mcpserver.MemoryWhyRequest{EntryID: note.ID})
	if err != nil {
		t.Fatalf("HandleMemoryWhy(%s) failed: %v", note.ID, err)
	}
	for _, want := range []string{
		"memory: index current to op-log",
		"Note: " + note.ID,
		"Status: unconsolidated",
		"Branch: feat/checkout",
		"Title: Stripe webhook signature header",
		"Anchors: internal/payments/webhook.go",
	} {
		if !strings.Contains(whyResp.Result, want) {
			t.Errorf("HandleMemoryWhy(%s) missing %q in:\n%s", note.ID, want, whyResp.Result)
		}
	}

	// 5. HandleMemorySearch matching unconsolidated note
	searchResp, err := srv.HandleMemorySearch(ctx, mcpserver.MemorySearchRequest{Query: "Stripe webhook"})
	if err != nil {
		t.Fatalf("HandleMemorySearch failed: %v", err)
	}
	for _, want := range []string{
		"memory: index current to op-log",
		"[unconsolidated] " + note.ID + " (payments, branch: feat/checkout): Stripe webhook signature header",
	} {
		if !strings.Contains(searchResp.Result, want) {
			t.Errorf("HandleMemorySearch missing %q in:\n%s", want, searchResp.Result)
		}
	}
}

