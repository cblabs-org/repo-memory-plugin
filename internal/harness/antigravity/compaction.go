package antigravity

import (
	"context"
	"fmt"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

type CompactionResult struct {
	Detected          bool
	CheckpointStepIdx int // step index where CHECKPOINT was found
	LostInjections    []delivery.InjectionRecord
}

func DetectCompaction(steps []Step, ledger *delivery.Ledger) CompactionResult {
	var result CompactionResult
	
	checkpointIdx := -1
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].IsCheckpoint {
			checkpointIdx = i
			break
		}
	}

	if checkpointIdx == -1 {
		return result
	}

	result.Detected = true
	result.CheckpointStepIdx = steps[checkpointIdx].StepIndex

	var nextStepIdx = 999999999
	for i := checkpointIdx + 1; i < len(steps); i++ {
		if !steps[i].IsCheckpoint && !steps[i].IsGap {
			nextStepIdx = steps[i].StepIndex
			break
		}
	}

	for _, inj := range ledger.Injections {
		if inj.StepIndex < nextStepIdx {
			result.LostInjections = append(result.LostInjections, inj)
		}
	}
	
	if len(result.LostInjections) == 0 {
		result.Detected = false
	}

	return result
}

func DetectRewind(steps []Step, lastKnownStepCount int) bool {
	return len(steps) < lastKnownStepCount
}

func BuildReinjection(ctx context.Context, idx *index.Index, lost []delivery.InjectionRecord, checkpointText string, budget int) (*InjectStep, error) {
	if len(lost) == 0 {
		return nil, nil
	}

	var valid []dossiers.Entry
	seen := make(map[string]bool)

	for _, inj := range lost {
		if seen[inj.EntryID] {
			continue
		}
		seen[inj.EntryID] = true

		tag := fmt.Sprintf("[repo-memory %s]", inj.EntryID)
		if strings.Contains(checkpointText, tag) {
			continue
		}

		ent, err := idx.GetEntry(ctx, inj.EntryID)
		if err != nil || ent == nil || ent.Retired {
			continue
		}

		valid = append(valid, *ent)
	}

	if len(valid) == 0 {
		return nil, nil
	}

	ranked := delivery.RankEntries(valid, nil)
	selected := delivery.SelectWithinBudget(ranked, budget)
	
	if len(selected) == 0 {
		return nil, nil
	}
	
	formatted := delivery.FormatEntries(selected)

	return &InjectStep{
		EphemeralMessage: formatted,
	}, nil
}
