// Package ops implements the append-only op log for repo knowledge entries.
//
// The op log is the v2 storage layer's source of truth. Operations target
// individual entry IDs (or dossiers during splits) and record additions, amendments,
// supersessions, retirements, moves, and splits.
//
// Ops are batched and written to .memory/ops/<yyyy-mm>/<batch-id>.json, ensuring
// git-friendly append-only logs that avoid merge conflicts.
package ops
