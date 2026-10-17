/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/spend.go
 * Description: Token spend guard enforcement, quota floor verification, and rolling
 *              token-estimator calibration persistence (Addendum A §6.2–§6.3).
 *
 * Exported Types / Functions:
 * - MaxCalibrationSamples:              Rolling window size for token estimator calibration.
 * - SpendGuardState:                    Persisted per-job timestamps and calibration ratios.
 *   - CalibrationMultiplier():          Returns the rolling median calibration multiplier.
 * - EstimateTokensCalibrated():         Estimates token count using calibrated byte-to-token ratio.
 * - SpendGuardsPath():                  Returns the filesystem path to spend_guards.json.
 * - LoadSpendGuardState():              Reads spend_guards.json from disk.
 * - SaveSpendGuardState():              Atomically writes spend_guards.json to disk.
 * - PipelineExecutor.CheckSpendGuard(): Evaluates whether a background job is allowed under spend guards.
 * - PipelineExecutor.RecordSpendRun():  Records job execution timestamp and token calibration ratio.
 */

package tick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// MaxCalibrationSamples is the rolling window size for token estimator calibration (Addendum A §6.3).
const MaxCalibrationSamples = 10

// SpendGuardState persists per-job last-run timestamps and rolling token calibration ratios
// at <dataDir>/<repoID>/state/spend_guards.json (Addendum A §6.2–§6.3).
type SpendGuardState struct {
	LastRunAt         map[string]string `json:"last_run_at,omitempty"`        // RFC3339 last-run timestamp keyed by job kind.
	CalibrationRatios []float64         `json:"calibration_ratios,omitempty"` // Rolling actual/estimated token ratios (up to MaxCalibrationSamples).
}

// CalibrationMultiplier returns the rolling median of CalibrationRatios over the last
// MaxCalibrationSamples runs, or 1.0 when no samples have been recorded yet.
func (s SpendGuardState) CalibrationMultiplier() float64 {
	if len(s.CalibrationRatios) == 0 {
		return 1.0
	}
	cp := make([]float64, len(s.CalibrationRatios))
	copy(cp, s.CalibrationRatios)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2.0
}

// EstimateTokensCalibrated estimates the token count of text using len(bytes)/4
// multiplied by the rolling median calibration factor (Addendum A §6.3).
func EstimateTokensCalibrated(text string, st SpendGuardState) int {
	if len(text) == 0 {
		return 0
	}
	base := float64(len(text)) / 4.0
	if base < 1.0 {
		base = 1.0
	}
	est := int(math.Round(base * st.CalibrationMultiplier()))
	if est < 1 {
		est = 1
	}
	return est
}

// SpendGuardsPath returns <dataDir>/<repoID>/state/spend_guards.json.
func SpendGuardsPath(dataDir, repoID string) string {
	return filepath.Join(dataDir, repoID, "state", "spend_guards.json")
}

// LoadSpendGuardState reads <dataDir>/<repoID>/state/spend_guards.json.
// If the file does not exist, an empty SpendGuardState with initialized map is returned without error.
func LoadSpendGuardState(dataDir, repoID string) (SpendGuardState, error) {
	// --- Step 1: Initialize default state and validate paths ---
	st := SpendGuardState{
		LastRunAt: make(map[string]string),
	}
	if dataDir == "" || repoID == "" {
		return st, nil
	}

	// --- Step 2: Read spend_guards.json from disk ---
	p := SpendGuardsPath(dataDir, repoID)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, fmt.Errorf("reading spend_guards.json: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return st, nil
	}

	// --- Step 3: Unmarshal JSON and ensure LastRunAt map is non-nil ---
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("parsing spend_guards.json: %w", err)
	}
	if st.LastRunAt == nil {
		st.LastRunAt = make(map[string]string)
	}
	return st, nil
}

// SaveSpendGuardState atomically writes SpendGuardState to <dataDir>/<repoID>/state/spend_guards.json.
func SaveSpendGuardState(dataDir, repoID string, st SpendGuardState) error {
	// --- Step 1: Validate inputs and ensure parent directory exists ---
	if dataDir == "" || repoID == "" {
		return nil
	}
	p := SpendGuardsPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	if st.LastRunAt == nil {
		st.LastRunAt = make(map[string]string)
	}

	// --- Step 2: Serialize state to formatted JSON ---
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling spend_guards.json: %w", err)
	}

	// --- Step 3: Write to a temporary file and atomically rename ---
	tmp := fmt.Sprintf("%s.%d.tmp", p, time.Now().UnixNano())
	if err := os.WriteFile(tmp, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing temp spend_guards.json: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming spend_guards.json: %w", err)
	}
	return nil
}

// effectiveSpendConfig resolves the effective SpendConfig by layering:
// DefaultSpendConfig -> MachineConfig.Spend -> disk RepoConfig.Spend -> in-memory p.cfg.Spend.
func (p *PipelineExecutor) effectiveSpendConfig() config.SpendConfig {
	eff := config.DefaultSpendConfig()
	if p.dataDir != "" {
		if mCfg, err := config.LoadMachineConfig(p.dataDir); err == nil {
			eff = config.MergeSpendConfig(eff, mCfg.Spend)
		}
	}
	if p.repoRoot != "" {
		if rCfg, err := config.LoadRepoConfig(p.repoRoot); err == nil {
			eff = config.MergeSpendConfig(eff, rCfg.Spend)
		}
	}
	eff = config.MergeSpendConfig(eff, p.cfg.Spend)
	return eff
}

// checkQuotaWithFloor reports whether remaining model quota across flash (or all) buckets meets floor.
func (p *PipelineExecutor) checkQuotaWithFloor(ctx context.Context, floor float64) bool {
	// --- Step 1: Resolve floor threshold and quota checker function ---
	if floor <= 0 {
		floor = MinFallbackQuotaFraction
	}
	checker := p.QuotaChecker
	if checker == nil {
		checker = telemetry.CheckQuota
	}

	// --- Step 2: Query live quota status ---
	qs, err := checker(ctx)
	if err != nil {
		if errors.Is(err, harness.ErrQuotaExhausted) {
			return false
		}
		return true
	}
	if qs == nil {
		return true
	}

	// --- Step 3: Collect quota buckets and prioritize flash model buckets ---
	var allBuckets []telemetry.QuotaBucket
	allBuckets = append(allBuckets, qs.Buckets...)
	for _, g := range qs.Groups {
		allBuckets = append(allBuckets, g.Buckets...)
	}
	if len(allBuckets) == 0 {
		return true
	}

	var flashBuckets []telemetry.QuotaBucket
	for _, b := range allBuckets {
		if strings.Contains(strings.ToLower(b.ID), "flash") || strings.Contains(strings.ToLower(b.Name), "flash") {
			flashBuckets = append(flashBuckets, b)
		}
	}

	targetBuckets := flashBuckets
	if len(targetBuckets) == 0 {
		targetBuckets = allBuckets
	}

	// --- Step 4: Verify all target buckets meet the minimum remaining fraction ---
	for _, b := range targetBuckets {
		if b.RemainingFraction < floor {
			return false
		}
	}
	return true
}

// CheckSpendGuard evaluates whether a background LLM job of jobKind is allowed to run
// under the configured A5 spend guards (min_interval, quota_floor) and returns the
// effective maxItems batch cap.
func (p *PipelineExecutor) CheckSpendGuard(jobKind string, force bool) (allowed bool, reason string, maxItems int) {
	// --- Step 1: Resolve effective spend configuration for jobKind ---
	eff := p.effectiveSpendConfig()
	jobCfg, ok := eff.Jobs[jobKind]
	if !ok {
		def := config.DefaultSpendConfig()
		jobCfg = def.Jobs[jobKind]
	}
	maxItems = jobCfg.MaxItems

	// --- Step 2: Enforce min_interval and quota_floor when not forced ---
	if !force {
		if jobCfg.MinInterval != "" && p.dataDir != "" && p.repoID != "" {
			if minDur, err := time.ParseDuration(jobCfg.MinInterval); err == nil && minDur > 0 {
				if st, err := LoadSpendGuardState(p.dataDir, p.repoID); err == nil {
					if lastStr := strings.TrimSpace(st.LastRunAt[jobKind]); lastStr != "" {
						if lastTime, err := time.Parse(time.RFC3339, lastStr); err == nil {
							if p.now().Sub(lastTime) < minDur {
								return false, "min_interval not elapsed", maxItems
							}
						}
					}
				}
			}
		}

		if p.QuotaChecker != nil {
			if !p.checkQuotaWithFloor(context.Background(), eff.QuotaFloor) {
				return false, "below quota_floor", maxItems
			}
		}
	}

	return true, "", maxItems
}

// RecordSpendRun updates LastRunAt[jobKind] in spend_guards.json and appends
// actualTokens/estimatedTokens to the rolling 10-sample calibration window when both > 0.
func (p *PipelineExecutor) RecordSpendRun(jobKind string, estimatedTokens, actualTokens int) {
	// --- Step 1: Guard against unconfigured paths and acquire executor lock ---
	if p.dataDir == "" || p.repoID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	// --- Step 2: Load existing state and record job completion timestamp ---
	st, err := LoadSpendGuardState(p.dataDir, p.repoID)
	if err != nil {
		st = SpendGuardState{LastRunAt: make(map[string]string)}
	}
	if jobKind != "" {
		st.LastRunAt[jobKind] = p.now().UTC().Format(time.RFC3339)
	}

	// --- Step 3: Append calibration sample and persist bounded window ---
	if estimatedTokens > 0 && actualTokens > 0 {
		ratio := float64(actualTokens) / float64(estimatedTokens)
		st.CalibrationRatios = append(st.CalibrationRatios, ratio)
		if len(st.CalibrationRatios) > MaxCalibrationSamples {
			st.CalibrationRatios = st.CalibrationRatios[len(st.CalibrationRatios)-MaxCalibrationSamples:]
		}
	}
	_ = SaveSpendGuardState(p.dataDir, p.repoID, st)
}
