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
	CassetteDir     string
	PipelineVersion string
	Record          bool // If true, passes through to a real Housekeeper and saves the result
	RealHousekeeper harness.Housekeeper
}

func NewRecordedHousekeeper(cassetteDir, pipelineVersion string, record bool, realHousekeeper harness.Housekeeper) *RecordedHousekeeper {
	return &RecordedHousekeeper{
		CassetteDir:     cassetteDir,
		PipelineVersion: pipelineVersion,
		Record:          record,
		RealHousekeeper: realHousekeeper,
	}
}

func (r *RecordedHousekeeper) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return harness.Result{}, fmt.Errorf("failed to marshal input for cassette key: %w", err)
	}

	hash := sha256.Sum256(inputBytes)
	hashStr := hex.EncodeToString(hash[:])[:12]

	filename := fmt.Sprintf("%s_%s_%s.json", stage.String(), r.PipelineVersion, hashStr)
	path := filepath.Join(r.CassetteDir, filename)

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
