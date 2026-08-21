package store

import (
	"context"
	"database/sql"
	"fmt"

	"task143-batchreactor/internal/model"
)

// --- Cleaning matrix ---

// SetCleaning sets or replaces a cross-contamination cell.
func (s *Store) SetCleaning(ctx context.Context, tx *sql.Tx, fromProduct, toProduct string, sev model.CleaningSeverity) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO cleaning_matrix(from_product,to_product,severity) VALUES(?,?,?) ON CONFLICT(from_product,to_product) DO UPDATE SET severity=excluded.severity`, fromProduct, toProduct, int(sev))
	if err != nil {
		return fmt.Errorf("set cleaning: %w", err)
	}
	return nil
}

// ListCleaning returns the whole matrix.
func (s *Store) ListCleaning(ctx context.Context) ([]model.CleaningEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT from_product,to_product,severity FROM cleaning_matrix ORDER BY from_product,to_product`)
	if err != nil {
		return nil, fmt.Errorf("list cleaning: %w", err)
	}
	defer rows.Close()
	var out []model.CleaningEntry
	for rows.Next() {
		var e model.CleaningEntry
		var sev int
		if err := rows.Scan(&e.FromProduct, &e.ToProduct, &sev); err != nil {
			return nil, err
		}
		e.Severity = model.CleaningSeverity(sev)
		out = append(out, e)
	}
	return out, rows.Err()
}

// CleaningSeverity returns the severity from one product to another. A missing
// pair means no cleaning (CleaningNone).
func (s *Store) CleaningSeverity(ctx context.Context, fromProduct, toProduct string) (model.CleaningSeverity, error) {
	var sev int
	err := s.db.QueryRowContext(ctx, `SELECT severity FROM cleaning_matrix WHERE from_product=? AND to_product=?`, fromProduct, toProduct).Scan(&sev)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.CleaningNone, nil
		}
		return model.CleaningNone, err
	}
	return model.CleaningSeverity(sev), nil
}
