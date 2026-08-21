package store

import (
	"context"
	"database/sql"
	"fmt"

	"task143-batchreactor/internal/model"
)

// --- Safety results (derived table) ---

// UpsertSafetyResult stores or replaces the derived safety result for a batch.
func (s *Store) UpsertSafetyResult(ctx context.Context, tx *sql.Tx, res model.SafetyResult) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO safety_results(batch_id,delta_t_ad,mtsr,stoessel_class,tmr_seconds,peak_temp,conversion,verdict) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(batch_id) DO UPDATE SET delta_t_ad=excluded.delta_t_ad,mtsr=excluded.mtsr,stoessel_class=excluded.stoessel_class,tmr_seconds=excluded.tmr_seconds,peak_temp=excluded.peak_temp,conversion=excluded.conversion,verdict=excluded.verdict`,
		res.BatchID, res.DeltaTad, res.MTSR, res.StoesselClass, res.TMRSeconds, res.PeakTemp, res.Conversion, string(res.Verdict))
	if err != nil {
		return fmt.Errorf("upsert safety result: %w", err)
	}
	return nil
}

// GetSafetyResult returns the derived safety result for a batch.
func (s *Store) GetSafetyResult(ctx context.Context, batchID string) (*model.SafetyResult, error) {
	var r model.SafetyResult
	var verdict string
	err := s.db.QueryRowContext(ctx, `SELECT batch_id,delta_t_ad,mtsr,stoessel_class,tmr_seconds,peak_temp,conversion,verdict FROM safety_results WHERE batch_id=?`, batchID).
		Scan(&r.BatchID, &r.DeltaTad, &r.MTSR, &r.StoesselClass, &r.TMRSeconds, &r.PeakTemp, &r.Conversion, &verdict)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.Verdict = model.SafetyVerdict(verdict)
	return &r, nil
}
