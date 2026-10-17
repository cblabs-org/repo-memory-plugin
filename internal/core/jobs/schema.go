/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/jobs/schema.go
 * Description: Defines the SQLite DDL schema and initialization routine for
 *               the durable background job table and its scheduler indexes.
 *
 * Exported Types / Functions:
 * - SchemaDDL:  SQLite DDL statement creating the jobs table and indexes.
 * - InitSchema(): Executes SchemaDDL against an open SQLite database connection.
 */

package jobs

import (
	"database/sql"
	"fmt"
)

// SchemaDDL defines the SQLite table and indexes for the durable job table (SPEC §4.3).
// Schema includes: id, kind, priority, idempotency_key, state, lease_expires, attempts, next_attempt_at, cost.
const SchemaDDL = `
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    idempotency_key TEXT,
    state TEXT NOT NULL DEFAULT 'queued',
    lease_expires TIMESTAMP,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP,
    cost TEXT,
    payload TEXT,
    last_error TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_idempotency_key
    ON jobs(idempotency_key)
    WHERE idempotency_key IS NOT NULL AND idempotency_key != '';

CREATE INDEX IF NOT EXISTS idx_jobs_scheduler
    ON jobs(state, priority DESC, next_attempt_at, created_at);

CREATE INDEX IF NOT EXISTS idx_jobs_lease_expires
    ON jobs(state, lease_expires)
    WHERE state = 'running';
`

// InitSchema creates the jobs table and associated indexes if they do not exist.
func InitSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("db cannot be nil")
	}
	if _, err := db.Exec(SchemaDDL); err != nil {
		return fmt.Errorf("initializing jobs schema: %w", err)
	}
	return nil
}
