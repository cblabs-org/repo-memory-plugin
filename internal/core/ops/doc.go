/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/ops/doc.go
 * Description: Package documentation for the append-only repository memory
 *              operation log storage layer.
 *
 * Exported Types / Functions:
 * - (None — package documentation file)
 */

// Package ops implements the append-only op log for repo knowledge entries.
//
// The op log is the v2 storage layer's source of truth. Operations target
// individual entry IDs (or dossiers during splits) and record additions, amendments,
// supersessions, retirements, moves, and splits.
//
// Ops are batched and written to .memory/ops/<yyyy-mm>/<batch-id>.json, ensuring
// git-friendly append-only logs that avoid merge conflicts.
package ops
