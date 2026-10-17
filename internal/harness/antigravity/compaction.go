/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/compaction.go
 * Description: Detects transcript CHECKPOINT compactions and /rewind step-count
 *              regressions, and builds post-compaction SystemMessage re-injection steps
 *              for entries lost prior to the checkpoint.
 *
 * Exported Types / Functions:
 * - CompactionResult:   Summary of whether a CHECKPOINT truncated prior injections.
 * - DetectCompaction(): Inspects transcript steps against ledger injections for checkpoint loss.
 * - DetectRewind():     Reports whether transcript step count regressed below lastKnownStepCount.
 * - BuildReinjection(): Ranks and formats surviving entries lost before a checkpoint into a SystemMessage step.
 */

package antigravity

import (
	"context"
	"fmt"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

// CompactionResult summarizes whether a transcript CHECKPOINT truncated prior injections.
type CompactionResult struct {
	// Detected is true when a CHECKPOINT step exists and prior injections precede it.
	Detected bool
	// CheckpointStepIdx is the resolved step index where the latest CHECKPOINT occurred.
	CheckpointStepIdx int
	// LostInjections lists ledger injection records whose StepIndex precedes the post-checkpoint resume point.
	LostInjections []delivery.InjectionRecord
}

// DetectCompaction inspects parsed transcript steps against ledger injections to
// identify entries lost before the latest CHECKPOINT boundary.
func DetectCompaction(steps []Step, ledger *delivery.Ledger) CompactionResult {
	// --- Step 1: Locate the latest CHECKPOINT step in the transcript ---
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

	// --- Step 2: Resolve the effective step index of the CHECKPOINT ---
	result.Detected = true
	result.CheckpointStepIdx = steps[checkpointIdx].StepIndex
	if result.CheckpointStepIdx == 0 && checkpointIdx > 0 {
		for i := checkpointIdx - 1; i >= 0; i-- {
			if !steps[i].IsCheckpoint && !steps[i].IsGap && steps[i].StepIndex > 0 {
				result.CheckpointStepIdx = steps[i].StepIndex + 1
				break
			}
		}
		if result.CheckpointStepIdx == 0 {
			result.CheckpointStepIdx = checkpointIdx
		}
	}

	// --- Step 3: Collect ledger injections whose step index precedes the first post-checkpoint step ---
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

// DetectRewind reports whether the transcript step count regressed below lastKnownStepCount.
func DetectRewind(steps []Step, lastKnownStepCount int) bool {
	return len(steps) < lastKnownStepCount
}

// BuildReinjection ranks and formats active entries from lost that are not already
// preserved in checkpointText, returning a persistent SystemMessage InjectStep
// (with Metadata omitted) within budget tokens.
func BuildReinjection(ctx context.Context, idx *index.Index, lost []delivery.InjectionRecord, checkpointText string, budget int) (*InjectStep, error) {
	if len(lost) == 0 {
		return nil, nil
	}

	// --- Step 1: Deduplicate lost entry IDs and filter out entries already in checkpointText or retired ---
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

	// --- Step 2: Rank surviving entries, clip to budget, and wrap as a SystemMessage ---
	ranked := delivery.RankEntries(valid, nil)
	selected := delivery.SelectWithinBudget(ranked, budget, 0)

	if len(selected) == 0 {
		return nil, nil
	}

	formatted := delivery.FormatEntries(selected)

	return &InjectStep{
		SystemMessage: &SystemMessageEntry{
			SystemMessage: formatted,
		},
	}, nil
}
