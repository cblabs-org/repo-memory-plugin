/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/fake/recorded.go
 * Description: Implements a cassette-backed Housekeeper that replays or records
 *              stage execution results keyed by stage name, pipeline version, and
 *              SHA-256 input hash.
 *
 * Exported Types / Functions:
 * - RecordedHousekeeper: Cassette-backed implementation of harness.Housekeeper.
 *
 *   Lifecycle:
 *   - NewRecordedHousekeeper(): Constructs a RecordedHousekeeper for a cassette directory.
 *
 *   Housekeeper (harness.Housekeeper):
 *   - Run(): Replays a recorded stage Result from disk or records a live execution.
 */

package fake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// RecordedHousekeeper implements a cassette-based recorded response Housekeeper.
type RecordedHousekeeper struct {
	CassetteDir     string              // Directory path where JSON cassette files are stored.
	PipelineVersion string              // Version tag included in cassette filenames.
	Record          bool                // If true, delegates to RealHousekeeper and writes the result to disk.
	RealHousekeeper harness.Housekeeper // Underlying live Housekeeper used when Record is true.
}

// NewRecordedHousekeeper constructs a RecordedHousekeeper configured for cassette replay or recording.
func NewRecordedHousekeeper(cassetteDir, pipelineVersion string, record bool, realHousekeeper harness.Housekeeper) *RecordedHousekeeper {
	return &RecordedHousekeeper{
		CassetteDir:     cassetteDir,
		PipelineVersion: pipelineVersion,
		Record:          record,
		RealHousekeeper: realHousekeeper,
	}
}

// Run computes a deterministic cassette filename from stage, PipelineVersion, and the SHA-256
// hash of input, replaying the cached Result when Record is false or invoking RealHousekeeper
// and persisting the cassette when Record is true.
func (r *RecordedHousekeeper) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	// --- Step 1: Compute deterministic cassette path from stage, version, and input hash ---
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return harness.Result{}, fmt.Errorf("failed to marshal input for cassette key: %w", err)
	}

	hash := sha256.Sum256(inputBytes)
	hashStr := hex.EncodeToString(hash[:])[:12]

	filename := fmt.Sprintf("%s_%s_%s.json", stage.String(), r.PipelineVersion, hashStr)
	path := filepath.Join(r.CassetteDir, filename)

	// --- Step 2: Replay existing cassette from disk when not recording ---
	if !r.Record {
		data, err := os.ReadFile(path)
		if err != nil {
			return harness.Result{}, fmt.Errorf("failed to read cassette %s: %w", path, err)
		}
		var res harness.Result
		if err := json.Unmarshal(data, &res); err != nil {
			return harness.Result{}, fmt.Errorf("failed to unmarshal cassette %s: %w", path, err)
		}
		return res, nil
	}

	// --- Step 3: Execute live Housekeeper and persist cassette to disk ---
	if r.RealHousekeeper == nil {
		return harness.Result{}, fmt.Errorf("recording enabled but no RealHousekeeper provided")
	}

	res, err := r.RealHousekeeper.Run(ctx, stage, input)
	if err != nil {
		return harness.Result{}, err
	}

	if err := os.MkdirAll(r.CassetteDir, 0755); err != nil {
		return res, fmt.Errorf("failed to create cassette dir: %w", err)
	}

	resBytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return res, fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(path, resBytes, 0644); err != nil {
		return res, fmt.Errorf("failed to write cassette: %w", err)
	}

	return res, nil
}
