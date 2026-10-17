/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/ops/validate.go
 * Description: Validates individual knowledge operations and OpBatch containers
 *              against type-specific schema and field invariants before reading
 *              or persisting op logs.
 *
 * Exported Types / Functions:
 * - ValidateOp():    Validates a single Op against its type-specific field rules.
 * - ValidateBatch(): Validates an OpBatch header and all contained operations.
 */

package ops

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateOp validates a single op against its type-specific rules.
func ValidateOp(op Op) error {
	// --- Step 1: Dispatch validation rules by operation type ---
	switch op.Type {
	case OpAdd:
		// --- Step 2: Validate OpAdd required entry_id, dossier_scope, and fields.text ---
		if strings.TrimSpace(op.EntryID) == "" {
			return errors.New("add: entry_id is required")
		}
		if strings.TrimSpace(op.DossierScope) == "" {
			return errors.New("add: dossier_scope is required")
		}
		if op.Fields == nil {
			return errors.New("add: fields is required")
		}
		textVal, ok := op.Fields["text"]
		if !ok {
			return errors.New("add: fields.text is required")
		}
		textStr, ok := textVal.(string)
		if !ok || strings.TrimSpace(textStr) == "" {
			return errors.New("add: fields.text must be a non-empty string")
		}

	case OpAmend:
		// --- Step 3: Validate OpAmend required entry_id and non-empty fields map ---
		if strings.TrimSpace(op.EntryID) == "" {
			return errors.New("amend: entry_id is required")
		}
		if op.Fields == nil || len(op.Fields) == 0 {
			return errors.New("amend: fields is required")
		}

	case OpSupersede:
		// --- Step 4: Validate OpSupersede required entry_id, fields.new_entry_id, and fields.text ---
		if strings.TrimSpace(op.EntryID) == "" {
			return errors.New("supersede: entry_id is required")
		}
		if op.Fields == nil {
			return errors.New("supersede: fields is required")
		}
		newIDVal, ok := op.Fields["new_entry_id"]
		if !ok {
			return errors.New("supersede: fields.new_entry_id is required")
		}
		newID, ok := newIDVal.(string)
		if !ok || strings.TrimSpace(newID) == "" {
			return errors.New("supersede: fields.new_entry_id must be a non-empty string")
		}
		textVal, ok := op.Fields["text"]
		if !ok {
			return errors.New("supersede: fields.text is required")
		}
		textStr, ok := textVal.(string)
		if !ok || strings.TrimSpace(textStr) == "" {
			return errors.New("supersede: fields.text must be a non-empty string")
		}

	case OpRetire:
		// --- Step 5: Validate OpRetire required entry_id and fields.reason ---
		if strings.TrimSpace(op.EntryID) == "" {
			return errors.New("retire: entry_id is required")
		}
		if op.Fields == nil {
			return errors.New("retire: fields is required")
		}
		reasonVal, ok := op.Fields["reason"]
		if !ok {
			return errors.New("retire: fields.reason is required")
		}
		reasonStr, ok := reasonVal.(string)
		if !ok || strings.TrimSpace(reasonStr) == "" {
			return errors.New("retire: fields.reason must be a non-empty string")
		}

	case OpMove:
		// --- Step 6: Validate OpMove required entry_id and fields.to_dossier ---
		if strings.TrimSpace(op.EntryID) == "" {
			return errors.New("move: entry_id is required")
		}
		if op.Fields == nil {
			return errors.New("move: fields is required")
		}
		toDossierVal, ok := op.Fields["to_dossier"]
		if !ok {
			return errors.New("move: fields.to_dossier is required")
		}
		toDossier, ok := toDossierVal.(string)
		if !ok || strings.TrimSpace(toDossier) == "" {
			return errors.New("move: fields.to_dossier must be a non-empty string")
		}

	case OpSplit:
		// --- Step 7: Validate OpSplit required dossier_scope, fields.new_dossiers, and fields.entry_moves ---
		if strings.TrimSpace(op.DossierScope) == "" {
			return errors.New("split: dossier_scope is required")
		}
		if op.Fields == nil {
			return errors.New("split: fields is required")
		}

		rawNewDossiers, ok := op.Fields["new_dossiers"]
		if !ok {
			return errors.New("split: fields.new_dossiers is required")
		}
		var newDossiersCount int
		switch v := rawNewDossiers.(type) {
		case []string:
			newDossiersCount = len(v)
			for _, s := range v {
				if strings.TrimSpace(s) == "" {
					return errors.New("split: fields.new_dossiers must contain non-empty scope names")
				}
			}
		case []any:
			newDossiersCount = len(v)
			for _, item := range v {
				s, ok := item.(string)
				if !ok || strings.TrimSpace(s) == "" {
					return errors.New("split: fields.new_dossiers must contain non-empty string scope names")
				}
			}
		default:
			return errors.New("split: fields.new_dossiers must be a list of scope names")
		}
		if newDossiersCount == 0 {
			return errors.New("split: fields.new_dossiers cannot be empty")
		}

		rawEntryMoves, ok := op.Fields["entry_moves"]
		if !ok {
			return errors.New("split: fields.entry_moves is required")
		}
		var movesCount int
		switch v := rawEntryMoves.(type) {
		case map[string]string:
			movesCount = len(v)
			for k, s := range v {
				if strings.TrimSpace(k) == "" || strings.TrimSpace(s) == "" {
					return errors.New("split: fields.entry_moves contains empty entry ID or target scope")
				}
			}
		case map[string]any:
			movesCount = len(v)
			for k, val := range v {
				s, ok := val.(string)
				if !ok || strings.TrimSpace(k) == "" || strings.TrimSpace(s) == "" {
					return fmt.Errorf("split: fields.entry_moves[%q] must be a non-empty string target scope", k)
				}
			}
		default:
			return errors.New("split: fields.entry_moves must be a map of entry ID to scope name")
		}
		if movesCount == 0 {
			return errors.New("split: fields.entry_moves cannot be empty")
		}

	case "":
		return errors.New("op type is required")

	default:
		return fmt.Errorf("unknown op type %q", op.Type)
	}

	return nil
}

// ValidateBatch validates an entire OpBatch.
func ValidateBatch(batch OpBatch) error {
	// --- Step 1: Validate batch-level metadata and non-empty operation slice ---
	if batch.Version < 1 {
		return errors.New("batch version must be >= 1")
	}
	if strings.TrimSpace(batch.BatchID) == "" {
		return errors.New("batch_id is required")
	}
	if batch.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	if len(batch.Ops) == 0 {
		return errors.New("batch must contain at least one op")
	}

	// --- Step 2: Validate each individual Op in the batch ---
	for i, op := range batch.Ops {
		if err := ValidateOp(op); err != nil {
			return fmt.Errorf("op[%d]: %w", i, err)
		}
	}
	return nil
}
