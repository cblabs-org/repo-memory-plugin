with open("internal/core/index/index.go", "r") as f:
    code = f.read()

import re
code = re.sub(r'// InsertClaim inserts.*\}', '', code, flags=re.DOTALL)

claim_func = """
// InsertClaim inserts or updates a claim and its related tables in the index.
func (idx *Index) InsertClaim(ctx context.Context, claim *claims.Claim) error {
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT OR REPLACE INTO claims (id, type, title, status, relevant_when, apply, verified_at, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.ExecContext(ctx, query, claim.ID, claim.Type, claim.Title, claim.Status, claim.RelevantWhen, claim.Apply, claim.VerifiedAt, claim.Body)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM anchors WHERE claim_id = ?`, claim.ID)
	if err != nil {
		return err
	}
	for _, a := range claim.Anchors {
		_, err = tx.ExecContext(ctx, `INSERT INTO anchors (claim_id, kind, ref, fp) VALUES (?, ?, ?, ?)`, claim.ID, a.Kind, a.Ref, a.FP)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM relations WHERE source_id = ?`, claim.ID)
	if err != nil {
		return err
	}
	if claim.Relations != nil {
		for _, target := range claim.Relations.Supports {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "supports", target)
		}
		for _, target := range claim.Relations.Contradicts {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "contradicts", target)
		}
		for _, target := range claim.Relations.Supersedes {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "supersedes", target)
		}
		for _, target := range claim.Relations.Explains {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "explains", target)
		}
		for _, target := range claim.Relations.Refines {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "refines", target)
		}
		for _, target := range claim.Relations.DependsOn {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "depends_on", target)
		}
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM conditions WHERE claim_id = ?`, claim.ID)
	if err != nil {
		return err
	}
	for k, v := range claim.Conditions {
		_, err = tx.ExecContext(ctx, `INSERT INTO conditions (claim_id, key, result) VALUES (?, ?, ?)`, claim.ID, k, v)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
"""

with open("internal/core/index/index.go", "w") as f:
    f.write(code.strip() + "\n" + claim_func)
