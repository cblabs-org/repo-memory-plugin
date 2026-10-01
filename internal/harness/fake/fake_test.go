package fake_test

import (
	"context"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

func TestFakeHarnessBasics(t *testing.T) {
	h := fake.NewFakeHarness()

	if h.Name() != "fake" {
		t.Fatalf("expected harness name 'fake', got %q", h.Name())
	}

	caps := h.Capabilities()
	if !caps.InjectBeforeModelCall || !caps.BlockWithReason {
		t.Fatalf("expected default capabilities to be populated, got %+v", caps)
	}

	if h.Transcripts() == nil {
		t.Fatal("expected non-nil Transcripts source")
	}
	if h.Hooks() == nil {
		t.Fatal("expected non-nil Hooks codec")
	}
	if h.Reporter() == nil {
		t.Fatal("expected non-nil Reporter")
	}
	if h.Housekeeper() == nil {
		t.Fatal("expected non-nil Housekeeper")
	}
}

func TestFakeTranscriptSource(t *testing.T) {
	ctx := context.Background()
	ts := fake.NewFakeTranscriptSource()

	ts.AddTranscript("/path/to/transcript.jsonl", []harness.Step{
		{
			Index: 1,
			Kind:  harness.StepKindUser,
			Text:  "hello",
		},
		{
			Index: 2,
			Kind:  harness.StepKindAgent,
			Text:  "hi there",
		},
	})

	paths, err := ts.Discover(ctx, "/path/to")
	if err != nil {
		t.Fatalf("unexpected Discover error: %v", err)
	}
	if len(paths) != 1 || paths[0] != "/path/to/transcript.jsonl" {
		t.Fatalf("unexpected discovered paths: %v", paths)
	}

	steps, err := ts.Parse(ctx, "/path/to/transcript.jsonl")
	if err != nil {
		t.Fatalf("unexpected Parse error: %v", err)
	}
	if len(steps) != 2 || steps[0].Text != "hello" || steps[1].Text != "hi there" {
		t.Fatalf("unexpected steps: %+v", steps)
	}

	_, err = ts.Parse(ctx, "/nonexistent")
	if err == nil {
		t.Fatal("expected error parsing non-existent transcript")
	}
}

func TestFakeHookCodec(t *testing.T) {
	codec := fake.NewFakeHookCodec()

	raw := []byte(`{"type":"pre_invocation","conversation_id":"c123","invocation_num":1}`)
	ev, err := codec.DecodeEvent(raw)
	if err != nil {
		t.Fatalf("unexpected DecodeEvent error: %v", err)
	}
	if ev.Type != harness.EventPreInvocation || ev.ConversationID != "c123" || ev.InvocationNum != 1 {
		t.Fatalf("unexpected decoded event: %+v", ev)
	}

	inj := harness.Injection{
		Decision:         harness.DecisionAllow,
		EphemeralMessage: "context delivered",
	}
	data, err := codec.EncodeInjection(inj)
	if err != nil {
		t.Fatalf("unexpected EncodeInjection error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty encoded injection bytes")
	}
}

func TestFakeReporter(t *testing.T) {
	ctx := context.Background()
	rep := fake.NewFakeReporter()

	repID, err := rep.StartReport(ctx, "Weekly Review", "Please review pending claims")
	if err != nil {
		t.Fatalf("unexpected StartReport error: %v", err)
	}
	if repID == "" {
		t.Fatal("expected non-empty report ID")
	}

	st, err := rep.PollReport(ctx, repID)
	if err != nil {
		t.Fatalf("unexpected PollReport error: %v", err)
	}
	if !st.Active || st.ReportID != repID {
		t.Fatalf("unexpected report status: %+v", st)
	}

	st.Active = false
	st.Messages = []harness.Step{
		{Index: 1, Kind: harness.StepKindUser, Text: "Looks good"},
	}
	rep.SetReport(st)

	polled, err := rep.PollReport(ctx, repID)
	if err != nil {
		t.Fatalf("unexpected PollReport error after update: %v", err)
	}
	if polled.Active || len(polled.Messages) != 1 {
		t.Fatalf("unexpected updated report status: %+v", polled)
	}
}

func TestFakeHousekeeper(t *testing.T) {
	ctx := context.Background()
	hk := fake.NewFakeHousekeeper()

	res, err := hk.Run(ctx, harness.StageTriage)
	if err != nil {
		t.Fatalf("unexpected Run error: %v", err)
	}
	if res.Payload == "" {
		t.Fatal("expected non-empty default payload")
	}

	custom := harness.Result{
		Payload: `{"claims":["clm-1"]}`,
		Usage: harness.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}
	hk.SetResult(harness.StageExtract, custom)

	resExtract, err := hk.Run(ctx, harness.StageExtract)
	if err != nil {
		t.Fatalf("unexpected Run error for extract: %v", err)
	}
	if resExtract.Payload != custom.Payload || resExtract.Usage.InputTokens != 100 {
		t.Fatalf("unexpected extract result: %+v", resExtract)
	}
}
