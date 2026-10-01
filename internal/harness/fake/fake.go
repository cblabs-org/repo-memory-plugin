package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// FakeHarness is an in-memory test implementation of harness.Harness.
type FakeHarness struct {
	MockName         string
	MockCapabilities harness.Capabilities
	MockTranscripts  *FakeTranscriptSource
	MockHooks        *FakeHookCodec
	MockReporter     *FakeReporter
	MockHousekeeper  *FakeHousekeeper
}

// New creates an initialized FakeHarness with default mocks.
func New() *FakeHarness {
	return NewFakeHarness()
}

// NewFakeHarness creates an initialized FakeHarness with default mocks.
func NewFakeHarness() *FakeHarness {
	return &FakeHarness{
		MockName: "fake",
		MockCapabilities: harness.Capabilities{
			InjectBeforeModelCall: true,
			InjectAtPrompt:        true,
			InjectAfterTool:       true,
			InjectAtSubagentStart: true,
			SoftContextBeforeTool: true,
			BlockWithReason:       true,
			AsyncPush:             true,
			HookPayloadHasToolIO:  true,
		},
		MockTranscripts: NewFakeTranscriptSource(),
		MockHooks:       NewFakeHookCodec(),
		MockReporter:    NewFakeReporter(),
		MockHousekeeper: NewFakeHousekeeper(),
	}
}

func (f *FakeHarness) Name() string {
	if f.MockName != "" {
		return f.MockName
	}
	return "fake"
}

func (f *FakeHarness) Capabilities() harness.Capabilities {
	return f.MockCapabilities
}

func (f *FakeHarness) Transcripts() harness.TranscriptSource {
	return f.MockTranscripts
}

func (f *FakeHarness) Hooks() harness.HookCodec {
	return f.MockHooks
}

func (f *FakeHarness) Reporter() harness.Reporter {
	return f.MockReporter
}

func (f *FakeHarness) Housekeeper() harness.Housekeeper {
	return f.MockHousekeeper
}

// FakeTranscriptSource provides an in-memory mock for transcript discovery and parsing.
type FakeTranscriptSource struct {
	mu              sync.RWMutex
	DiscoveredPaths []string
	Transcripts     map[string][]harness.Step
	DiscoverErr     error
	ParseErr        error
}

func NewFakeTranscriptSource() *FakeTranscriptSource {
	return &FakeTranscriptSource{
		Transcripts: make(map[string][]harness.Step),
	}
}

func (f *FakeTranscriptSource) Discover(ctx context.Context, location string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.DiscoverErr != nil {
		return nil, f.DiscoverErr
	}
	paths := make([]string, len(f.DiscoveredPaths))
	copy(paths, f.DiscoveredPaths)
	return paths, nil
}

func (f *FakeTranscriptSource) Parse(ctx context.Context, path string) ([]harness.Step, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.ParseErr != nil {
		return nil, f.ParseErr
	}
	steps, ok := f.Transcripts[path]
	if !ok {
		return nil, fmt.Errorf("transcript not found for path: %s", path)
	}
	out := make([]harness.Step, len(steps))
	copy(out, steps)
	return out, nil
}

func (f *FakeTranscriptSource) SetDiscovered(paths []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DiscoveredPaths = make([]string, len(paths))
	copy(f.DiscoveredPaths, paths)
}

func (f *FakeTranscriptSource) AddTranscript(path string, steps []harness.Step) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Transcripts[path] = steps
	f.DiscoveredPaths = append(f.DiscoveredPaths, path)
}

// FakeHookCodec provides an in-memory mock for encoding and decoding hook payloads.
type FakeHookCodec struct {
	mu                   sync.RWMutex
	DecodeFunc           func(payload []byte) (harness.Event, error)
	EncodeFunc           func(injection harness.Injection) ([]byte, error)
	LastDecodedPayload   []byte
	LastEncodedInjection harness.Injection
}

func NewFakeHookCodec() *FakeHookCodec {
	return &FakeHookCodec{}
}

func (f *FakeHookCodec) DecodeEvent(payload []byte) (harness.Event, error) {
	f.mu.Lock()
	f.LastDecodedPayload = payload
	decodeFn := f.DecodeFunc
	f.mu.Unlock()

	if decodeFn != nil {
		return decodeFn(payload)
	}

	var ev harness.Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		return harness.Event{}, err
	}
	return ev, nil
}

func (f *FakeHookCodec) EncodeInjection(injection harness.Injection) ([]byte, error) {
	f.mu.Lock()
	f.LastEncodedInjection = injection
	encodeFn := f.EncodeFunc
	f.mu.Unlock()

	if encodeFn != nil {
		return encodeFn(injection)
	}

	return json.Marshal(injection)
}

// FakeReporter provides an in-memory mock for review reports.
type FakeReporter struct {
	mu             sync.RWMutex
	Reports        map[string]harness.ReportStatus
	StartedReports []ReportEntry
	StartErr       error
	PollErr        error
	nextID         int
}

type ReportEntry struct {
	ID     string
	Title  string
	Prompt string
}

func NewFakeReporter() *FakeReporter {
	return &FakeReporter{
		Reports: make(map[string]harness.ReportStatus),
	}
}

func (f *FakeReporter) StartReport(ctx context.Context, title string, prompt string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.StartErr != nil {
		return "", f.StartErr
	}
	f.nextID++
	id := fmt.Sprintf("rep-%03d", f.nextID)
	f.StartedReports = append(f.StartedReports, ReportEntry{
		ID:     id,
		Title:  title,
		Prompt: prompt,
	})
	f.Reports[id] = harness.ReportStatus{
		ReportID: id,
		Active:   true,
	}
	return id, nil
}

func (f *FakeReporter) PollReport(ctx context.Context, reportID string) (harness.ReportStatus, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.PollErr != nil {
		return harness.ReportStatus{}, f.PollErr
	}
	st, ok := f.Reports[reportID]
	if !ok {
		return harness.ReportStatus{}, fmt.Errorf("report not found: %s", reportID)
	}
	return st, nil
}

func (f *FakeReporter) SetReport(status harness.ReportStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Reports[status.ReportID] = status
}

// FakeHousekeeper provides an in-memory mock for running housekeeping stages.
type FakeHousekeeper struct {
	mu         sync.RWMutex
	Results    map[harness.Stage]harness.Result
	RunHistory []harness.Stage
	RunErr     error
}

func NewFakeHousekeeper() *FakeHousekeeper {
	return &FakeHousekeeper{
		Results: make(map[harness.Stage]harness.Result),
	}
}

func (f *FakeHousekeeper) Run(ctx context.Context, stage harness.Stage) (harness.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RunHistory = append(f.RunHistory, stage)
	if f.RunErr != nil {
		return harness.Result{}, f.RunErr
	}
	res, ok := f.Results[stage]
	if !ok {
		return harness.Result{
			Payload: fmt.Sprintf(`{"status":"ok","stage":%q}`, stage.String()),
		}, nil
	}
	return res, nil
}

func (f *FakeHousekeeper) SetResult(stage harness.Stage, result harness.Result) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Results[stage] = result
}
