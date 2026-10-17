/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/fake/fake.go
 * Description: Provides in-memory test doubles for harness.Harness,
 *              harness.TranscriptSource, harness.HookCodec, harness.Reporter,
 *              and harness.Housekeeper.
 *
 * Exported Types / Functions:
 * - FakeHarness: In-memory test implementation of harness.Harness.
 *
 *   Lifecycle:
 *   - New():            Constructs an initialized FakeHarness with default mocks.
 *   - NewFakeHarness(): Constructs an initialized FakeHarness with default mocks.
 *
 *   Own Public API:
 *   - Housekeeper():    Returns the mock Housekeeper attached to the harness.
 *
 *   Harness (harness.Harness):
 *   - Name():           Returns the configured harness name ("fake" by default).
 *   - Capabilities():   Returns the configured Capabilities struct.
 *   - Transcripts():    Returns the mock TranscriptSource.
 *   - Hooks():          Returns the mock HookCodec.
 *   - Reporter():       Returns the mock Reporter.
 *
 * - FakeTranscriptSource: In-memory mock implementation of harness.TranscriptSource.
 *
 *   Lifecycle:
 *   - NewFakeTranscriptSource(): Constructs an empty FakeTranscriptSource.
 *
 *   Own Public API:
 *   - SetDiscovered():           Overwrites the list of discovered transcript paths.
 *   - AddTranscript():           Registers parsed steps for a transcript path.
 *
 *   TranscriptSource (harness.TranscriptSource):
 *   - Discover():                Returns the configured list of discovered transcript paths.
 *   - Parse():                   Returns the registered steps for the requested path.
 *
 * - FakeHookCodec: In-memory mock implementation of harness.HookCodec.
 *
 *   Lifecycle:
 *   - NewFakeHookCodec(): Constructs an empty FakeHookCodec.
 *
 *   HookCodec (harness.HookCodec):
 *   - DecodeEvent():      Records and decodes a raw hook payload into a harness.Event.
 *   - EncodeInjection():  Records and encodes a harness.Injection into JSON bytes.
 *
 * - ReportEntry: Recorded metadata for a report started via FakeReporter.
 * - FakeReporter: In-memory mock implementation of harness.Reporter.
 *
 *   Lifecycle:
 *   - NewFakeReporter(): Constructs an empty FakeReporter.
 *
 *   Own Public API:
 *   - SetReport():       Updates the stored ReportStatus for a report ID.
 *
 *   Reporter (harness.Reporter):
 *   - StartReport():     Allocates a report ID and records the started report.
 *   - PollReport():      Returns the current ReportStatus for a report ID.
 *
 * - FakeHousekeeper: In-memory mock implementation of harness.Housekeeper.
 *
 *   Lifecycle:
 *   - NewFakeHousekeeper(): Constructs an empty FakeHousekeeper.
 *
 *   Own Public API:
 *   - SetResult():          Configures the canned Result for a pipeline Stage.
 *
 *   Housekeeper (harness.Housekeeper):
 *   - Run():                Records the stage invocation and returns the configured Result.
 */

package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// fakeSessionLogTokenRe matches <session-log-HEX> echo tokens in test prompts.
var fakeSessionLogTokenRe = regexp.MustCompile(`<session-log-([0-9a-fA-F]+)>`)

// FakeHarness is an in-memory test implementation of harness.Harness.
type FakeHarness struct {
	MockName         string                // Custom harness name returned by Name().
	MockCapabilities harness.Capabilities  // Feature support flags returned to callers inspecting adapter capabilities.
	MockTranscripts  *FakeTranscriptSource // Mock transcript source returned by Transcripts().
	MockHooks        *FakeHookCodec        // Mock hook codec returned by Hooks().
	MockReporter     *FakeReporter         // In-memory review report tracker returned by Reporter().
	MockHousekeeper  *FakeHousekeeper      // Deterministic LLM stage runner returned by Housekeeper().
}

// New creates an initialized FakeHarness with default mocks.
func New() *FakeHarness {
	return NewFakeHarness()
}

// NewFakeHarness creates an initialized FakeHarness with default mocks.
func NewFakeHarness() *FakeHarness {
	// --- Step 1: Construct FakeHarness with all capability flags enabled and fresh sub-mocks ---
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

// Name returns MockName if non-empty, or "fake" by default.
func (f *FakeHarness) Name() string {
	if f.MockName != "" {
		return f.MockName
	}
	return "fake"
}

// Capabilities returns the configured MockCapabilities for this fake harness.
func (f *FakeHarness) Capabilities() harness.Capabilities {
	return f.MockCapabilities
}

// Transcripts returns the underlying FakeTranscriptSource.
func (f *FakeHarness) Transcripts() harness.TranscriptSource {
	return f.MockTranscripts
}

// Hooks returns the underlying FakeHookCodec.
func (f *FakeHarness) Hooks() harness.HookCodec {
	return f.MockHooks
}

// Reporter returns the underlying FakeReporter.
func (f *FakeHarness) Reporter() harness.Reporter {
	return f.MockReporter
}

// Housekeeper returns the underlying FakeHousekeeper.
func (f *FakeHarness) Housekeeper() harness.Housekeeper {
	return f.MockHousekeeper
}

// FakeTranscriptSource provides an in-memory mock for transcript discovery and parsing.
type FakeTranscriptSource struct {
	mu              sync.RWMutex              // Guards concurrent access to paths and transcripts.
	DiscoveredPaths []string                  // Ordered slice of transcript paths returned by Discover.
	Transcripts     map[string][]harness.Step // Map of transcript path to parsed steps.
	DiscoverErr     error                     // Optional error returned by Discover.
	ParseErr        error                     // Optional error returned by Parse.
}

// NewFakeTranscriptSource constructs an empty FakeTranscriptSource.
func NewFakeTranscriptSource() *FakeTranscriptSource {
	return &FakeTranscriptSource{
		Transcripts: make(map[string][]harness.Step),
	}
}

// Discover returns a copy of DiscoveredPaths or DiscoverErr if set.
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

// Parse returns a copy of the registered steps for path or an error if not found.
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

// SetDiscovered replaces the list of paths returned by Discover.
func (f *FakeTranscriptSource) SetDiscovered(paths []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DiscoveredPaths = make([]string, len(paths))
	copy(f.DiscoveredPaths, paths)
}

// AddTranscript registers steps under path and appends path to DiscoveredPaths.
func (f *FakeTranscriptSource) AddTranscript(path string, steps []harness.Step) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Transcripts[path] = steps
	f.DiscoveredPaths = append(f.DiscoveredPaths, path)
}

// FakeHookCodec provides an in-memory mock for encoding and decoding hook payloads.
type FakeHookCodec struct {
	mu                   sync.RWMutex                              // Guards recorded payloads and custom callbacks.
	DecodeFunc           func(payload []byte) (harness.Event, error) // Optional custom decoder callback.
	EncodeFunc           func(injection harness.Injection) ([]byte, error) // Optional custom encoder callback.
	LastDecodedPayload   []byte                                    // Raw bytes from the most recent DecodeEvent call.
	LastEncodedInjection harness.Injection                         // Injection struct from the most recent EncodeInjection call.
}

// NewFakeHookCodec constructs an empty FakeHookCodec.
func NewFakeHookCodec() *FakeHookCodec {
	return &FakeHookCodec{}
}

// DecodeEvent records payload and unmarshals it into a harness.Event (or delegates to DecodeFunc).
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

// EncodeInjection records injection and marshals it to JSON (or delegates to EncodeFunc).
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
	mu             sync.RWMutex                    // Guards concurrent access to report state.
	Reports        map[string]harness.ReportStatus // Active and completed report statuses keyed by ID.
	StartedReports []ReportEntry                   // Chronological log of reports started via StartReport.
	StartErr       error                           // Optional error returned by StartReport.
	PollErr        error                           // Optional error returned by PollReport.
	nextID         int                             // Monotonic counter for generating synthetic report IDs.
}

// ReportEntry records the arguments of a single StartReport call.
type ReportEntry struct {
	ID     string // Synthetic sequential report handle (e.g. "rep-001") assigned on start.
	Title  string // Report title passed to StartReport.
	Prompt string // Report prompt passed to StartReport.
}

// NewFakeReporter constructs an empty FakeReporter.
func NewFakeReporter() *FakeReporter {
	return &FakeReporter{
		Reports: make(map[string]harness.ReportStatus),
	}
}

// StartReport allocates a sequential report ID, records the ReportEntry, and marks the report active.
func (f *FakeReporter) StartReport(ctx context.Context, title string, prompt string) (string, error) {
	// --- Step 1: Check for injected error under lock ---
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.StartErr != nil {
		return "", f.StartErr
	}

	// --- Step 2: Allocate sequential ID and record active report entry ---
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

// PollReport returns the stored ReportStatus for reportID or an error if not found.
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

// SetReport updates the stored ReportStatus for status.ReportID.
func (f *FakeReporter) SetReport(status harness.ReportStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Reports[status.ReportID] = status
}

// FakeHousekeeper provides an in-memory mock for running housekeeping stages.
type FakeHousekeeper struct {
	mu         sync.RWMutex                                                          // Guards stage results and invocation history.
	Results    map[harness.Stage]harness.Result                                      // Canned results keyed by pipeline Stage.
	RunHistory []harness.Stage                                                       // Ordered log of stages passed to Run.
	Inputs     []any                                                                 // Ordered log of inputs passed to Run.
	RunFunc    func(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) // Optional custom stage runner callback.
	RunErr     error                                                                 // Optional error returned by Run.
}

// NewFakeHousekeeper constructs an empty FakeHousekeeper.
func NewFakeHousekeeper() *FakeHousekeeper {
	return &FakeHousekeeper{
		Results: make(map[harness.Stage]harness.Result),
	}
}

// Run records the stage and input, then delegates to RunFunc or returns the canned stage Result.
func (f *FakeHousekeeper) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	// --- Step 1: Record invocation history and snapshot configured result/callbacks ---
	f.mu.Lock()
	f.RunHistory = append(f.RunHistory, stage)
	f.Inputs = append(f.Inputs, input)
	runFn := f.RunFunc
	runErr := f.RunErr
	res, ok := f.Results[stage]
	f.mu.Unlock()

	// --- Step 2: Evaluate error override, custom callback, or default stage payload ---
	if runErr != nil {
		return harness.Result{}, runErr
	}
	if runFn != nil {
		return runFn(ctx, stage, input)
	}
	if !ok {
		return harness.Result{
			Payload: fmt.Sprintf(`{"status":"ok","stage":%q}`, stage.String()),
		}, nil
	}
	// --- Step 3: Substitute {{echo_token}} placeholder when present in input ---
	if s, isStr := input.(string); isStr && strings.Contains(res.Payload, "{{echo_token}}") {
		if m := fakeSessionLogTokenRe.FindStringSubmatch(s); len(m) == 2 {
			res.Payload = strings.ReplaceAll(res.Payload, "{{echo_token}}", m[1])
		}
	}
	return res, nil
}

// SetResult configures the canned Result returned when stage is executed.
func (f *FakeHousekeeper) SetResult(stage harness.Stage, result harness.Result) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Results[stage] = result
}
