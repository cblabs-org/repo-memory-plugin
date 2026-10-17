package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

var entryTagRe = regexp.MustCompile(`\[repo-memory ([^\]]+)\]`)

type probePhaseMetrics struct {
	Phase            string
	Channel          string
	EntryCount       int
	SeedCount        int
	DocSeedCount     int
	LineCount        int
	TokenEstimate    int
	LatencyMs        float64
	InjectedEntryIDs []string
	PersistedInCtx   int
}

func extractEntryIDsFromText(text string) []string {
	matches := entryTagRe.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var ids []string
	for _, m := range matches {
		id := strings.TrimSpace(m[1])
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// setupEvalV2ProbeEnv prepares an isolated enrolled workspace and dataDir backed by
// evaluation/eval-v2/seed/index.db and evaluation/eval-v2/template when present on disk,
// or falls back to a deterministic synthetic esp32 fixture in clean CI environments.
func setupEvalV2ProbeEnv(t *testing.T) (workspaceRoot string, dataDir string, idxPath string, usingRealSeed bool) {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	seedDB := filepath.Join(repoRoot, "evaluation", "eval-v2", "seed", "index.db")
	templateDir := filepath.Join(repoRoot, "evaluation", "eval-v2", "template")

	dataDir = t.TempDir()
	idxPath = filepath.Join(dataDir, "esp32-eval-probe", "index.db")
	if err := os.MkdirAll(filepath.Dir(idxPath), 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(seedDB); err == nil {
		if _, err := os.Stat(templateDir); err == nil {
			// Copy index.db to isolated temp dataDir so probe mutations never touch evaluation/eval-v2/seed/index.db.
			if err := copyFile(seedDB, idxPath); err != nil {
				t.Fatalf("copy seed index.db: %v", err)
			}
			// Create an isolated enrolled workspace that mirrors the template file paths via symlinks
			// without modifying evaluation/eval-v2/template.
			workspaceRoot = t.TempDir()
			entries, err := os.ReadDir(templateDir)
			if err != nil {
				t.Fatalf("readdir templateDir: %v", err)
			}
			for _, ent := range entries {
				if ent.Name() == ".memory" || ent.Name() == ".git" {
					continue
				}
				src := filepath.Join(templateDir, ent.Name())
				dst := filepath.Join(workspaceRoot, ent.Name())
				if err := os.Symlink(src, dst); err != nil {
					t.Fatalf("symlink %s: %v", ent.Name(), err)
				}
			}
			if err := config.CreateEnrolledMarker(workspaceRoot); err != nil {
				t.Fatal(err)
			}
			if err := config.AddToRegistry(dataDir, []string{workspaceRoot}, "esp32-eval-probe", workspaceRoot); err != nil {
				t.Fatal(err)
			}
			return workspaceRoot, dataDir, idxPath, true
		}
	}

	// Deterministic CI fallback fixture when gitignored evaluation/eval-v2/{seed,template} are absent.
	workspaceRoot = t.TempDir()
	if err := config.CreateEnrolledMarker(workspaceRoot); err != nil {
		t.Fatal(err)
	}
	if err := config.AddToRegistry(dataDir, []string{workspaceRoot}, "esp32-eval-probe", workspaceRoot); err != nil {
		t.Fatal(err)
	}

	filesToCreate := []string{
		"platformio.ini",
		"modules/Application/Application.hpp",
		"modules/Application/Application.cpp",
		"modules/DataManager/DataManager.hpp",
		"modules/U8G2Display/U8G2Display.hpp",
		"modules/WebServer/WebServer.hpp",
		"modules/MCPServer/MCPServer.hpp",
		"modules/EventBus/EventBus.hpp",
		"modules/ConfigManager/ConfigManager.hpp",
		"include/interfaces/IDataSource.hpp",
		"include/interfaces/IMCPContributor.hpp",
		"include/interfaces/IWebContributor.hpp",
		"include/interfaces/IDisplayBoard.hpp",
	}
	for _, rel := range filesToCreate {
		abs := filepath.Join(workspaceRoot, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("// synthetic esp32 fixture\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	syntheticEntries := []dossiers.Entry{
		{
			ID:           "docseed-u8g2-contract",
			Type:         dossiers.EntryTypeContract,
			Source:       "doc-seed",
			DossierScope: "modules/U8G2Display",
			Text:         "U8G2 boards implement the page-buffered IDisplayBoard interface and register on the screen carousel.",
			Anchors:      []string{"modules/U8G2Display/U8G2Display.hpp", "include/interfaces/IDisplayBoard.hpp"},
		},
		{
			ID:           "docseed-mcp-invariant",
			Type:         dossiers.EntryTypeInvariant,
			Source:       "doc-seed",
			DossierScope: "modules/MCPServer",
			Text:         "MCPServer registers contributor tools via IMCPContributor during begin() on the mcp server.",
			Anchors:      []string{"modules/MCPServer/MCPServer.hpp", "include/interfaces/IMCPContributor.hpp"},
		},
		{
			ID:           "docseed-web-invariant",
			Type:         dossiers.EntryTypeInvariant,
			Source:       "doc-seed",
			DossierScope: "modules/WebServer",
			Text:         "Portal top-level navigation icons and status pages register via IWebContributor and ITabRegistry.",
			Anchors:      []string{"modules/WebServer/WebServer.hpp", "include/interfaces/IWebContributor.hpp"},
		},
		{
			ID:           "docseed-datamanager-contract",
			Type:         dossiers.EntryTypeContract,
			Source:       "doc-seed",
			DossierScope: "modules/DataManager",
			Text:         "External HTTP/API data sources (temperature, bitcoin price, coinmarketcap) implement IDataSource::fetchUpdate(FetchContext&).",
			Anchors:      []string{"modules/DataManager/DataManager.hpp", "include/interfaces/IDataSource.hpp"},
		},
		{
			ID:           "docseed-app-invariant",
			Type:         dossiers.EntryTypeInvariant,
			Source:       "doc-seed",
			DossierScope: "modules/Application",
			Text:         "Application.cpp wires module registration across DataManager, U8G2Display, WebServer, and MCPServer.",
			Anchors:      []string{"modules/Application/Application.cpp", "modules/Application/Application.hpp"},
		},
		{
			ID:           "seed-portal-decision",
			Type:         dossiers.EntryTypeDecision,
			Source:       "seed",
			DossierScope: "root",
			Text:         "REST API coverage added policy to GET /api/display/status and portal module status routes.",
		},
	}
	for _, ent := range syntheticEntries {
		if err := idx.InsertEntry(ctx, ent); err != nil {
			t.Fatal(err)
		}
	}
	idx.Close()
	return workspaceRoot, dataDir, idxPath, false
}

func TestEvalV2ReadSideMicroProbe(t *testing.T) {
	ctx := context.Background()
	workspaceRoot, dataDir, idxPath, usingRealSeed := setupEvalV2ProbeEnv(t)
	os.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Cleanup(func() { os.Unsetenv("REPO_MEMORY_DATA_DIR") })

	// Add a deterministic gate: edit entry anchored to platformio.ini in the isolated index copy
	// to test Phase 3 PreToolUse gate enforcement alongside MomentAboutToEdit.
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	gateEntry := dossiers.Entry{
		ID:           "docseed-gate-platformio",
		Type:         dossiers.EntryTypeInvariant,
		Source:       "doc-seed",
		DossierScope: "root",
		Text:         "Do not modify platformio.ini partition tables or build_flags without verifying native test compatibility gate: edit",
		Anchors:      []string{"platformio.ini"},
	}
	if err := idx.InsertEntry(ctx, gateEntry); err != nil {
		t.Fatalf("insert gate entry: %v", err)
	}
	idx.Close()

	repoRoot, _ := filepath.Abs(filepath.Join("..", "..", ".."))
	task1PromptBytes, err := os.ReadFile(filepath.Join(repoRoot, "evaluation", "eval-v2", "tasks", "task-1-temperature-module.txt"))
	if err != nil {
		task1PromptBytes = []byte("Write a new module for a Waterproof DS18B20-Compatible Temperature Sensor. We need a new u8g2 screen for this added to the carousel and a new page on the portal to show the temperature status, and configure the update interval from between 1 sec to 15 mins. It should be a top-level icon. Write data from this module into the on-device mcp server too.")
	}
	task2PromptBytes, err := os.ReadFile(filepath.Join(repoRoot, "evaluation", "eval-v2", "tasks", "task-2-bitcoin-price-module.txt"))
	if err != nil {
		task2PromptBytes = []byte("Build a module to get bitcoin prices using https://pro-api.coinmarketcap.com/public-api/v1/simple/price?ids=1 once per hour. Display them on a u8g2 screen, in the portal and on the mcp server.")
	}

	tasks := []struct {
		name         string
		convID       string
		prompt       string
		touchedFiles []string
	}{
		{
			name:   "Task 1 — DS18B20 Temperature Sensor Module",
			convID: "probe-task-1-temperature",
			prompt: strings.TrimSpace(string(task1PromptBytes)),
			touchedFiles: []string{
				"modules/U8G2Display/U8G2Display.hpp",
				"modules/WebServer/WebServer.hpp",
				"modules/MCPServer/MCPServer.hpp",
				"modules/Application/Application.cpp",
				"include/interfaces/IMCPContributor.hpp",
			},
		},
		{
			name:   "Task 2 — Bitcoin Price Ticker Module",
			convID: "probe-task-2-bitcoin",
			prompt: strings.TrimSpace(string(task2PromptBytes)),
			touchedFiles: []string{
				"modules/DataManager/DataManager.hpp",
				"include/interfaces/IDataSource.hpp",
				"modules/U8G2Display/U8G2Display.hpp",
				"modules/WebServer/WebServer.hpp",
				"modules/MCPServer/MCPServer.hpp",
				"modules/Application/Application.cpp",
			},
		},
	}

	readIdx, err := index.NewReadOnlyIndex(idxPath)
	if err != nil {
		t.Fatalf("open readonly index: %v", err)
	}
	defer readIdx.Close()

	classifyEntryIDs := func(ids []string) (seedCnt, docSeedCnt, entryTokens int, details []string) {
		for _, id := range ids {
			if id == "docseed-gate-platformio" {
				continue
			}
			ent, err := readIdx.GetEntry(ctx, id)
			if err != nil || ent == nil {
				continue
			}
			if ent.Source == "doc-seed" {
				docSeedCnt++
			} else {
				seedCnt++
			}
			tok := len(ent.Text) / 4
			if tok == 0 {
				tok = 1
			}
			entryTokens += tok
			snippet := ent.Text
			if len(snippet) > 90 {
				snippet = snippet[:90] + "..."
			}
			details = append(details, fmt.Sprintf("%s (%s/%s/%s scope=%s): %s", ent.ID, ent.Source, ent.Type, ent.EffectiveKind(), ent.DossierScope, snippet))
		}
		return seedCnt, docSeedCnt, entryTokens, details
	}

	for _, tc := range tasks {
		t.Run(tc.name, func(t *testing.T) {
			transcriptDir := filepath.Join(t.TempDir(), "logs")
			if err := os.MkdirAll(transcriptDir, 0755); err != nil {
				t.Fatal(err)
			}
			transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")

			marshalStep := func(m map[string]any) string {
				b, err := json.Marshal(m)
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			}
			writeTranscript := func(lines ...string) {
				content := strings.Join(lines, "\n") + "\n"
				if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			var metrics []probePhaseMetrics

			// =========================================================================
			// Phase 1: Invocation 0 (MomentTaskStart, invocationNum == 0, initialNumSteps == 1)
			// =========================================================================
			step0 := marshalStep(map[string]any{
				"step_index": 0,
				"type":       "USER_INPUT",
				"content":    tc.prompt,
			})
			writeTranscript(step0)

			payloadPhase1, _ := json.Marshal(PreInvocationPayload{
				ConversationID:  tc.convID,
				InvocationNum:   0,
				InitialNumSteps: 1,
				UserPrompt:      tc.prompt,
				TranscriptPath:  transcriptPath,
				WorkspacePaths:  []string{workspaceRoot},
			})

			t0 := time.Now()
			out1, err := HandlePreInvocation(ctx, payloadPhase1)
			lat1 := float64(time.Since(t0).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("Phase 1 HandlePreInvocation failed: %v", err)
			}

			var resp1 InjectStepsResponse
			if err := json.Unmarshal(out1, &resp1); err != nil {
				t.Fatalf("Phase 1 unmarshal: %v (raw: %s)", err, string(out1))
			}
			if len(resp1.InjectSteps) != 1 {
				t.Fatalf("Phase 1 expected 1 injectStep, got %d", len(resp1.InjectSteps))
			}
			p1Step := resp1.InjectSteps[0]
			if p1Step.EphemeralMessage != "" {
				t.Errorf("Phase 1 expected empty EphemeralMessage, got %q", p1Step.EphemeralMessage)
			}
			if p1Step.SystemMessage == nil || p1Step.SystemMessage.SystemMessage == "" {
				t.Fatalf("Phase 1 expected non-empty SystemMessage")
			}

			p1Text := p1Step.SystemMessage.SystemMessage
			p1Lines := len(strings.Split(strings.TrimSpace(p1Text), "\n"))
			p1RawTokens := len(p1Text) / 4
			p1IDs := extractEntryIDsFromText(p1Text)
			p1Seed, p1DocSeed, p1Tokens, p1Details := classifyEntryIDs(p1IDs)
			if p1Lines > 180 {
				t.Errorf("Phase 1 lines %d exceeded 180-line cap", p1Lines)
			}
			if p1Tokens > 500 {
				t.Errorf("Phase 1 entry tokens %d exceeded 500-token task-start budget", p1Tokens)
			}
			if p1RawTokens > 1200 {
				t.Errorf("Phase 1 formatted payload tokens %d exceeded 1200-token total cap", p1RawTokens)
			}
			if p1DocSeed == 0 {
				t.Errorf("Phase 1 expected doc-seed entries to be surfaced on MomentTaskStart, got 0 (IDs: %v)", p1IDs)
			}

			metrics = append(metrics, probePhaseMetrics{
				Phase:            "Phase 1: Invocation 0 (MomentTaskStart)",
				Channel:          "systemMessage",
				EntryCount:       len(p1IDs),
				SeedCount:        p1Seed,
				DocSeedCount:     p1DocSeed,
				LineCount:        p1Lines,
				TokenEstimate:    p1Tokens,
				LatencyMs:        lat1,
				InjectedEntryIDs: p1IDs,
				PersistedInCtx:   len(p1IDs),
			})
			for _, d := range p1Details {
				t.Logf("[Phase 1 TaskStart] %s", d)
			}

			// =========================================================================
			// Phase 2: Invocation 1 (MomentFirstEntry & MomentFirstTouch, invocationNum == 1, initialNumSteps == 5)
			// =========================================================================
			step1 := marshalStep(map[string]any{
				"step_index": 1,
				"type":       "SYSTEM_MESSAGE",
				"source":     "SYSTEM_SDK",
				"content":    p1Text,
			})
			var toolCalls []map[string]any
			for _, rel := range tc.touchedFiles {
				toolCalls = append(toolCalls, map[string]any{
					"name": "view_file",
					"args": map[string]any{
						"AbsolutePath": filepath.Join(workspaceRoot, rel),
					},
				})
			}
			step2 := marshalStep(map[string]any{
				"step_index": 2,
				"type":       "PLANNER_RESPONSE",
				"tool_calls": toolCalls,
			})
			step3 := marshalStep(map[string]any{
				"step_index": 3,
				"type":       "TOOL_RESPONSE",
				"content":    "// viewed subsystem headers and Application.cpp",
			})
			step4 := marshalStep(map[string]any{
				"step_index": 4,
				"type":       "PLANNER_RESPONSE",
				"thinking":   "Reviewed subsystem headers for module implementation.",
				"content":    "Subsystem headers inspected.",
			})
			writeTranscript(step0, step1, step2, step3, step4)

			payloadPhase2, _ := json.Marshal(PreInvocationPayload{
				ConversationID:  tc.convID,
				InvocationNum:   1,
				InitialNumSteps: 5,
				TranscriptPath:  transcriptPath,
				WorkspacePaths:  []string{workspaceRoot},
			})

			t1 := time.Now()
			out2, err := HandlePreInvocation(ctx, payloadPhase2)
			lat2 := float64(time.Since(t1).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("Phase 2 HandlePreInvocation failed: %v", err)
			}

			var resp2 InjectStepsResponse
			if err := json.Unmarshal(out2, &resp2); err != nil {
				t.Fatalf("Phase 2 unmarshal: %v (raw: %s)", err, string(out2))
			}

			var p2SysMsgs []string
			for _, st := range resp2.InjectSteps {
				if st.EphemeralMessage != "" {
					t.Errorf("Phase 2 expected empty EphemeralMessage, got %q", st.EphemeralMessage)
				}
				if st.SystemMessage != nil && st.SystemMessage.SystemMessage != "" {
					p2SysMsgs = append(p2SysMsgs, st.SystemMessage.SystemMessage)
				}
			}
			p2Combined := strings.Join(p2SysMsgs, "\n")
			p2IDs := extractEntryIDsFromText(p2Combined)
			if len(p2IDs) == 0 {
				t.Fatalf("Phase 2 expected MomentFirstTouch + MomentFirstEntry entries, got 0")
			}

			// Verify InjectedSet suppressed duplicate Phase 1 task-start entries
			p1Set := make(map[string]bool)
			for _, id := range p1IDs {
				p1Set[id] = true
			}
			for _, id := range p2IDs {
				if p1Set[id] {
					t.Errorf("Phase 2 expected InjectedSet to suppress duplicate task-start entry %s", id)
				}
			}

			// Verify Invocation 0 SystemMessage remains persisted in transcript_full.jsonl
			rawTranscriptP2, err := os.ReadFile(transcriptPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range p1IDs {
				if !strings.Contains(string(rawTranscriptP2), id) {
					t.Errorf("Phase 2 expected Invocation 0 entry %s to remain persisted in transcript_full.jsonl", id)
				}
			}

			p2Seed, p2DocSeed, p2Tokens, p2Details := classifyEntryIDs(p2IDs)
			p2Lines := len(strings.Split(strings.TrimSpace(p2Combined), "\n"))

			metrics = append(metrics, probePhaseMetrics{
				Phase:            "Phase 2: Invocation 1 (FirstTouch + FirstEntry)",
				Channel:          "systemMessage",
				EntryCount:       len(p2IDs),
				SeedCount:        p2Seed,
				DocSeedCount:     p2DocSeed,
				LineCount:        p2Lines,
				TokenEstimate:    p2Tokens,
				LatencyMs:        lat2,
				InjectedEntryIDs: p2IDs,
				PersistedInCtx:   len(p1IDs) + len(p2IDs),
			})
			for _, d := range p2Details {
				t.Logf("[Phase 2 FirstTouch/FirstEntry] %s", d)
			}

			// Append Phase 2 SystemMessage to transcript and verify Invocation 2 deduplication
			step5 := marshalStep(map[string]any{
				"step_index": 5,
				"type":       "SYSTEM_MESSAGE",
				"source":     "SYSTEM_SDK",
				"content":    p2Combined,
			})
			step6 := marshalStep(map[string]any{
				"step_index": 6,
				"type":       "PLANNER_RESPONSE",
				"thinking":   "Planning module header and source structure.",
				"content":    "Ready to implement.",
			})
			writeTranscript(step0, step1, step2, step3, step4, step5, step6)

			payloadDedup, _ := json.Marshal(PreInvocationPayload{
				ConversationID:  tc.convID,
				InvocationNum:   2,
				InitialNumSteps: 7,
				TranscriptPath:  transcriptPath,
				WorkspacePaths:  []string{workspaceRoot},
			})
			tDedup := time.Now()
			outDedup, err := HandlePreInvocation(ctx, payloadDedup)
			latDedup := float64(time.Since(tDedup).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("Phase 2b Dedup HandlePreInvocation failed: %v", err)
			}
			if string(outDedup) != "{}" {
				t.Errorf("Phase 2b expected deduplicated '{}', got %s", string(outDedup))
			}
			metrics = append(metrics, probePhaseMetrics{
				Phase:          "Phase 2b: Invocation 2 (Mid-Turn Dedup)",
				Channel:        "none (dedup)",
				EntryCount:     0,
				LineCount:      0,
				TokenEstimate:  0,
				LatencyMs:      latDedup,
				PersistedInCtx: len(p1IDs) + len(p2IDs),
			})

			// =========================================================================
			// Phase 3: Mid-Turn Edit Pointer (MomentAboutToEdit) + PreToolUse gate: edit
			// =========================================================================
			step7 := marshalStep(map[string]any{
				"step_index": 7,
				"type":       "PLANNER_RESPONSE",
				"thinking":   "Now I will edit modules/Application/Application.cpp to register the new module.",
				"content":    "Editing Application.cpp.",
			})
			writeTranscript(step0, step1, step2, step3, step4, step5, step6, step7)

			payloadPhase3, _ := json.Marshal(PreInvocationPayload{
				ConversationID:  tc.convID,
				InvocationNum:   3,
				InitialNumSteps: 8,
				TranscriptPath:  transcriptPath,
				WorkspacePaths:  []string{workspaceRoot},
			})

			t3 := time.Now()
			out3, err := HandlePreInvocation(ctx, payloadPhase3)
			lat3 := float64(time.Since(t3).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("Phase 3 HandlePreInvocation failed: %v", err)
			}
			var resp3 InjectStepsResponse
			if err := json.Unmarshal(out3, &resp3); err != nil {
				t.Fatalf("Phase 3 unmarshal: %v (raw: %s)", err, string(out3))
			}
			if len(resp3.InjectSteps) != 1 {
				t.Fatalf("Phase 3 expected 1 about-to-edit step, got %d: %+v", len(resp3.InjectSteps), resp3.InjectSteps)
			}
			p3Step := resp3.InjectSteps[0]
			if p3Step.SystemMessage != nil {
				t.Errorf("Phase 3 expected nil SystemMessage on about-to-edit, got %+v", p3Step.SystemMessage)
			}
			wantEphemeral := "[repo-memory] Check injected entries above before editing Application.cpp"
			if p3Step.EphemeralMessage != wantEphemeral {
				t.Errorf("Phase 3 got EphemeralMessage %q, want %q", p3Step.EphemeralMessage, wantEphemeral)
			}
			p3Lines := len(strings.Split(strings.TrimSpace(p3Step.EphemeralMessage), "\n"))
			p3Tokens := len(p3Step.EphemeralMessage) / 4
			if p3Lines > 35 || p3Tokens > 50 {
				t.Errorf("Phase 3 about-to-edit exceeded caps: lines=%d (<=35), tokens=%d (<=50)", p3Lines, p3Tokens)
			}

			metrics = append(metrics, probePhaseMetrics{
				Phase:            "Phase 3a: Invocation 3 (MomentAboutToEdit)",
				Channel:          "ephemeral",
				EntryCount:       1,
				LineCount:        p3Lines,
				TokenEstimate:    p3Tokens,
				LatencyMs:        lat3,
				InjectedEntryIDs: []string{"about-to-edit:modules/Application/Application.cpp"},
				PersistedInCtx:   len(p1IDs) + len(p2IDs),
			})

			// Phase 3b: PreToolUse gate: edit on platformio.ini
			gatePayload := fmt.Sprintf(`{
				"conversationId": %q,
				"toolName": "replace_file_content",
				"toolCall": {
					"name": "replace_file_content",
					"args": {"TargetFile": %q}
				},
				"workspacePaths": [%q]
			}`, tc.convID, filepath.Join(workspaceRoot, "platformio.ini"), workspaceRoot)

			tGate := time.Now()
			outGate1, err := HandlePreToolUse(ctx, []byte(gatePayload))
			latGate := float64(time.Since(tGate).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("Phase 3b HandlePreToolUse failed: %v", err)
			}
			var gateResp1 PreToolUseResponse
			if err := json.Unmarshal(outGate1, &gateResp1); err != nil {
				t.Fatalf("Phase 3b unmarshal: %v", err)
			}
			if gateResp1.Decision == "allow" {
				t.Fatalf("Phase 3b expected gate block (force_ask/deny) on first edit to platformio.ini, got allow")
			}
			gateLines := len(strings.Split(strings.TrimSpace(gateResp1.Reason), "\n"))
			gateTokens := len(gateResp1.Reason) / 4
			if gateLines > 35 {
				t.Errorf("Phase 3b gate reason lines %d exceeded 35-line cap", gateLines)
			}

			// Acknowledge via PostToolUse and verify subsequent PreToolUse returns allow
			postGatePayload := fmt.Sprintf(`{
				"conversationId": %q,
				"toolName": "replace_file_content",
				"toolCall": {
					"name": "replace_file_content",
					"args": {"TargetFile": %q}
				},
				"toolResult": {"status": "success", "output": ""},
				"workspacePaths": [%q]
			}`, tc.convID, filepath.Join(workspaceRoot, "platformio.ini"), workspaceRoot)
			if _, err := HandlePostToolUse(ctx, []byte(postGatePayload)); err != nil {
				t.Fatalf("Phase 3b HandlePostToolUse failed: %v", err)
			}
			outGate2, err := HandlePreToolUse(ctx, []byte(gatePayload))
			if err != nil {
				t.Fatalf("Phase 3b retry HandlePreToolUse failed: %v", err)
			}
			var gateResp2 PreToolUseResponse
			if err := json.Unmarshal(outGate2, &gateResp2); err != nil {
				t.Fatalf("Phase 3b retry unmarshal: %v", err)
			}
			if gateResp2.Decision != "allow" {
				t.Errorf("Phase 3b expected decision 'allow' after gate acknowledgment, got %q", gateResp2.Decision)
			}

			metrics = append(metrics, probePhaseMetrics{
				Phase:            "Phase 3b: PreToolUse (gate: edit -> retry allow)",
				Channel:          gateResp1.Decision + " -> allow",
				EntryCount:       1,
				DocSeedCount:     1,
				LineCount:        gateLines,
				TokenEstimate:    gateTokens,
				LatencyMs:        latGate,
				InjectedEntryIDs: []string{"docseed-gate-platformio"},
				PersistedInCtx:   len(p1IDs) + len(p2IDs),
			})

			// =========================================================================
			// Phase 4: Post-CHECKPOINT Compaction (BuildReinjection)
			// =========================================================================
			step9 := marshalStep(map[string]any{
				"step_index": 9,
				"type":       "SYSTEM",
				"content":    "Compacted trajectory summary: agent inspected headers and prepared module skeleton.",
			})
			writeTranscript(step0, step1, step2, step3, step4, step5, step6, step7, "CHECKPOINT", step9)

			payloadPhase4, _ := json.Marshal(PreInvocationPayload{
				ConversationID:  tc.convID,
				InvocationNum:   0,
				InitialNumSteps: 10,
				TranscriptPath:  transcriptPath,
				WorkspacePaths:  []string{workspaceRoot},
			})

			t4 := time.Now()
			out4, err := HandlePreInvocation(ctx, payloadPhase4)
			lat4 := float64(time.Since(t4).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("Phase 4 HandlePreInvocation failed: %v", err)
			}
			var resp4 InjectStepsResponse
			if err := json.Unmarshal(out4, &resp4); err != nil {
				t.Fatalf("Phase 4 unmarshal: %v (raw: %s)", err, string(out4))
			}
			if len(resp4.InjectSteps) == 0 {
				t.Fatalf("Phase 4 expected post-CHECKPOINT BuildReinjection step, got empty")
			}
			p4Step := resp4.InjectSteps[0]
			if p4Step.EphemeralMessage != "" {
				t.Errorf("Phase 4 expected empty EphemeralMessage on BuildReinjection, got %q", p4Step.EphemeralMessage)
			}
			if p4Step.SystemMessage == nil || p4Step.SystemMessage.SystemMessage == "" {
				t.Fatalf("Phase 4 expected non-empty SystemMessage on BuildReinjection")
			}
			p4Text := p4Step.SystemMessage.SystemMessage
			p4IDs := extractEntryIDsFromText(p4Text)
			p4Seed, p4DocSeed, p4Tokens, _ := classifyEntryIDs(p4IDs)
			p4Lines := len(strings.Split(strings.TrimSpace(p4Text), "\n"))
			if p4Tokens > 1000 {
				t.Errorf("Phase 4 BuildReinjection tokens %d exceeded 1000-token budget", p4Tokens)
			}

			loadedP4, err := LoadLedger(dataDir, "esp32-eval-probe", tc.convID)
			if err != nil {
				t.Fatalf("Phase 4 LoadLedger: %v", err)
			}
			if loadedP4.LastHandledCheckpointStep < 0 {
				t.Errorf("Phase 4 expected LastHandledCheckpointStep >= 0, got %d", loadedP4.LastHandledCheckpointStep)
			}

			metrics = append(metrics, probePhaseMetrics{
				Phase:            "Phase 4: Post-CHECKPOINT (BuildReinjection)",
				Channel:          "systemMessage",
				EntryCount:       len(p4IDs),
				SeedCount:        p4Seed,
				DocSeedCount:     p4DocSeed,
				LineCount:        p4Lines,
				TokenEstimate:    p4Tokens,
				LatencyMs:        lat4,
				InjectedEntryIDs: p4IDs,
				PersistedInCtx:   len(p4IDs),
			})

			// =========================================================================
			// Phase 5: Transcript Rewind (DetectRewind)
			// =========================================================================
			writeTranscript(step0, step1, step2)

			payloadPhase5, _ := json.Marshal(PreInvocationPayload{
				ConversationID:  tc.convID,
				InvocationNum:   1,
				InitialNumSteps: 3,
				TranscriptPath:  transcriptPath,
				WorkspacePaths:  []string{workspaceRoot},
			})

			t5 := time.Now()
			out5, err := HandlePreInvocation(ctx, payloadPhase5)
			lat5 := float64(time.Since(t5).Microseconds()) / 1000.0
			if err != nil {
				t.Fatalf("Phase 5 HandlePreInvocation failed: %v", err)
			}
			var resp5 InjectStepsResponse
			if err := json.Unmarshal(out5, &resp5); err != nil {
				t.Fatalf("Phase 5 unmarshal: %v (raw: %s)", err, string(out5))
			}
			var p5SysMsgs []string
			for _, st := range resp5.InjectSteps {
				if st.EphemeralMessage != "" {
					t.Errorf("Phase 5 expected empty EphemeralMessage after rewind, got %q", st.EphemeralMessage)
				}
				if st.SystemMessage != nil && st.SystemMessage.SystemMessage != "" {
					p5SysMsgs = append(p5SysMsgs, st.SystemMessage.SystemMessage)
				}
			}
			p5Combined := strings.Join(p5SysMsgs, "\n")
			p5IDs := extractEntryIDsFromText(p5Combined)
			if len(p5IDs) == 0 {
				t.Fatalf("Phase 5 expected DetectRewind to re-arm dedup and re-inject entries via SystemMessage, got 0")
			}
			p5Seed, p5DocSeed, p5Tokens, _ := classifyEntryIDs(p5IDs)
			p5Lines := len(strings.Split(strings.TrimSpace(p5Combined), "\n"))

			loadedP5, err := LoadLedger(dataDir, "esp32-eval-probe", tc.convID)
			if err != nil {
				t.Fatalf("Phase 5 LoadLedger: %v", err)
			}
			if loadedP5.LastHandledCheckpointStep != -1 {
				t.Errorf("Phase 5 expected LastHandledCheckpointStep reset to -1 after rewind past checkpoint, got %d", loadedP5.LastHandledCheckpointStep)
			}

			metrics = append(metrics, probePhaseMetrics{
				Phase:            "Phase 5: Post-/rewind (DetectRewind Re-Arm)",
				Channel:          "systemMessage",
				EntryCount:       len(p5IDs),
				SeedCount:        p5Seed,
				DocSeedCount:     p5DocSeed,
				LineCount:        p5Lines,
				TokenEstimate:    p5Tokens,
				LatencyMs:        lat5,
				InjectedEntryIDs: p5IDs,
				PersistedInCtx:   len(p1IDs) + len(p5IDs),
			})

			// Log structured summary table for this task
			t.Logf("\n=== %s (realSeed=%v) ===", tc.name, usingRealSeed)
			t.Logf("%-46s | %-18s | %7s | %4s | %8s | %5s | %6s | %8s | %9s",
				"Phase", "Channel", "Entries", "Seed", "Doc-Seed", "Lines", "Tokens", "Lat (ms)", "InContext")
			t.Logf("%s", strings.Repeat("-", 128))
			for _, m := range metrics {
				t.Logf("%-46s | %-18s | %7d | %4d | %8d | %5d | %6d | %8.2f | %9d",
					m.Phase, m.Channel, m.EntryCount, m.SeedCount, m.DocSeedCount, m.LineCount, m.TokenEstimate, m.LatencyMs, m.PersistedInCtx)
			}
		})
	}

	// =========================================================================
	// Quantitative Comparison: v2.1 Baseline (PA-only + EphemeralMessage) vs v2.2 Combined
	// =========================================================================
	t.Run("BaselineComparison_v2_1_vs_v2_2", func(t *testing.T) {
		v21DataDir := t.TempDir()
		v21IdxPath := filepath.Join(v21DataDir, "esp32-v21", "index.db")
		if err := copyFile(idxPath, v21IdxPath); err != nil {
			t.Fatalf("copy index for v2.1 baseline: %v", err)
		}
		v21Idx, err := index.NewIndex(v21IdxPath)
		if err != nil {
			t.Fatalf("open v2.1 index: %v", err)
		}
		// Strip doc-seed entries to simulate v2.1 PA-only index (256 seed entries)
		docSeedEntries, err := v21Idx.ListEntriesBySource(ctx, "doc-seed")
		if err != nil {
			t.Fatalf("list doc-seed entries: %v", err)
		}
		for _, de := range docSeedEntries {
			if err := v21Idx.DeleteEntry(ctx, de.ID); err != nil {
				t.Fatalf("delete doc-seed entry %s: %v", de.ID, err)
			}
		}
		seedEntries, err := v21Idx.ListEntriesBySource(ctx, "seed")
		if err != nil {
			t.Fatalf("list seed entries: %v", err)
		}

		// Compare TaskStart + FirstTouch + FirstEntry under v2.1 vs v2.2
		v21Ledger := delivery.NewLedger("v21-sim")
		v21Eng := delivery.NewEngine(v21Idx, workspaceRoot, delivery.WithInjectedSet(v21Ledger.InjectedSet))
		v22Ledger := delivery.NewLedger("v22-sim")
		v22Idx, err := index.NewIndex(idxPath)
		if err != nil {
			t.Fatalf("open v2.2 index: %v", err)
		}
		defer v22Idx.Close()
		defer v21Idx.Close()
		v22Eng := delivery.NewEngine(v22Idx, workspaceRoot, delivery.WithInjectedSet(v22Ledger.InjectedSet))

		for _, tc := range tasks {
			terms := ExtractPromptTerms(tc.prompt)
			v21Start, _ := v21Eng.TaskStartEntries(ctx, terms, nil)
			v22Start, _ := v22Eng.TaskStartEntries(ctx, terms, nil)

			var v21TouchCount, v22TouchCount int
			for _, rel := range tc.touchedFiles {
				r21, _ := v21Eng.AnchorEntries(ctx, rel)
				r22, _ := v22Eng.AnchorEntries(ctx, rel)
				v21TouchCount += len(r21)
				v22TouchCount += len(r22)
			}
			scopes := []string{"modules/U8G2Display", "modules/WebServer", "modules/MCPServer", "modules/Application", "modules/DataManager", "include/interfaces"}
			var v21DossierCount, v22DossierCount int
			for _, sc := range scopes {
				d21, _ := v21Eng.DossierEntries(ctx, sc)
				d22, _ := v22Eng.DossierEntries(ctx, sc)
				v21DossierCount += len(d21)
				v22DossierCount += len(d22)
			}

			// In v2.1, EphemeralMessage vanishes after 1 invocation while InjectedSet blocks re-injection,
			// so 0 entries remain in context at Invocation 2+ when the agent actually edits code.
			v21SurvivingAtEdit := 0
			v22SurvivingAtEdit := len(v22Start) + v22TouchCount + v22DossierCount

			t.Logf("[v2.1 vs v2.2 Comparison] %s: corpus=(v2.1:%d seed-only vs v2.2:%d combined) | TaskStart=(v2.1:%d vs v2.2:%d) | FirstTouch=(v2.1:%d vs v2.2:%d) | FirstEntry=(v2.1:%d vs v2.2:%d) | SurvivingAtCodeEdit(Inv>=2)=(v2.1:%d vs v2.2:%d)",
				tc.name, len(seedEntries), len(seedEntries)+len(docSeedEntries)-1,
				len(v21Start), len(v22Start),
				v21TouchCount, v22TouchCount,
				v21DossierCount, v22DossierCount,
				v21SurvivingAtEdit, v22SurvivingAtEdit)

			if v22SurvivingAtEdit <= v21SurvivingAtEdit {
				t.Errorf("expected v2.2 surviving entries at edit (%d) > v2.1 (%d)", v22SurvivingAtEdit, v21SurvivingAtEdit)
			}
			v21Ledger.RearmDedup()
			v22Ledger.RearmDedup()
		}
	})
}
