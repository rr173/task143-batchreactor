package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task143-batchreactor/internal/model"
)

// --- Batch ---

// CreateBatch inserts a batch row.
func (s *Store) CreateBatch(ctx context.Context, tx *sql.Tx, b *model.Batch) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO batches(id,campaign_id,campaign_item_seq,reactor_id,recipe_id,seq,status,planned_start,started_at,ended_at,peak_temp,conversion,safety_verdict,stoessel_class,fault_reason) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, b.CampaignID, b.CampaignItemSeq, b.ReactorID, b.RecipeID, b.Seq, string(b.Status), b.PlannedStart, b.StartedAt, b.EndedAt, b.PeakTemp, b.Conversion, string(b.SafetyVerdict), b.StoesselClass, b.FaultReason)
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}
	return nil
}

// GetBatch returns one batch.
func (s *Store) GetBatch(ctx context.Context, id string) (*model.Batch, error) {
	return s.getBatch(ctx, s.db, id)
}

// GetBatchTx returns one batch inside a transaction.
func (s *Store) GetBatchTx(ctx context.Context, tx *sql.Tx, id string) (*model.Batch, error) {
	return s.getBatch(ctx, tx, id)
}

func (s *Store) getBatch(ctx context.Context, q DBTX, id string) (*model.Batch, error) {
	var b model.Batch
	var status, verdict string
	err := q.QueryRowContext(ctx, `SELECT id,campaign_id,campaign_item_seq,reactor_id,recipe_id,seq,status,planned_start,started_at,ended_at,peak_temp,conversion,safety_verdict,stoessel_class,fault_reason FROM batches WHERE id=?`, id).
		Scan(&b.ID, &b.CampaignID, &b.CampaignItemSeq, &b.ReactorID, &b.RecipeID, &b.Seq, &status, &b.PlannedStart, &b.StartedAt, &b.EndedAt, &b.PeakTemp, &b.Conversion, &verdict, &b.StoesselClass, &b.FaultReason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	b.Status = model.BatchStatus(status)
	b.SafetyVerdict = model.SafetyVerdict(verdict)
	return &b, nil
}

// ListBatchesByCampaign returns all batches of a campaign.
func (s *Store) ListBatchesByCampaign(ctx context.Context, campaignID string) ([]model.Batch, error) {
	return s.listBatches(ctx, s.db, `WHERE campaign_id=? ORDER BY reactor_id, seq`, campaignID)
}

// ListBatchesByReactor returns all batches of a reactor in seq order.
func (s *Store) ListBatchesByReactor(ctx context.Context, reactorID string) ([]model.Batch, error) {
	return s.listBatches(ctx, s.db, `WHERE reactor_id=? ORDER BY seq`, reactorID)
}

// ActiveBatchOnReactor returns the non-terminal batch currently on a reactor,
// if any (for occupancy checks during planning and charging).
func (s *Store) ActiveBatchOnReactor(ctx context.Context, reactorID string) (*model.Batch, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,campaign_id,campaign_item_seq,reactor_id,recipe_id,seq,status,planned_start,started_at,ended_at,peak_temp,conversion,safety_verdict,stoessel_class,fault_reason FROM batches WHERE reactor_id=? AND status NOT IN ('done','faulted','aborted') ORDER BY seq DESC LIMIT 1`, reactorID)
	var b model.Batch
	var status, verdict string
	err := row.Scan(&b.ID, &b.CampaignID, &b.CampaignItemSeq, &b.ReactorID, &b.RecipeID, &b.Seq, &status, &b.PlannedStart, &b.StartedAt, &b.EndedAt, &b.PeakTemp, &b.Conversion, &verdict, &b.StoesselClass, &b.FaultReason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	b.Status = model.BatchStatus(status)
	b.SafetyVerdict = model.SafetyVerdict(verdict)
	return &b, nil
}

// ListBatchesByReactorTx returns batches on a reactor inside a transaction.
func (s *Store) ListBatchesByReactorTx(ctx context.Context, tx *sql.Tx, reactorID string) ([]model.Batch, error) {
	return s.listBatches(ctx, tx, `WHERE reactor_id=? ORDER BY seq`, reactorID)
}

func (s *Store) listBatches(ctx context.Context, q DBTX, where string, args ...any) ([]model.Batch, error) {
	query := `SELECT id,campaign_id,campaign_item_seq,reactor_id,recipe_id,seq,status,planned_start,started_at,ended_at,peak_temp,conversion,safety_verdict,stoessel_class,fault_reason FROM batches ` + where
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list batches: %w", err)
	}
	defer rows.Close()
	var out []model.Batch
	for rows.Next() {
		var b model.Batch
		var status, verdict string
		if err := rows.Scan(&b.ID, &b.CampaignID, &b.CampaignItemSeq, &b.ReactorID, &b.RecipeID, &b.Seq, &status, &b.PlannedStart, &b.StartedAt, &b.EndedAt, &b.PeakTemp, &b.Conversion, &verdict, &b.StoesselClass, &b.FaultReason); err != nil {
			return nil, err
		}
		b.Status = model.BatchStatus(status)
		b.SafetyVerdict = model.SafetyVerdict(verdict)
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBatchStatus transitions a batch's status and timestamps.
func (s *Store) UpdateBatchStatus(ctx context.Context, tx *sql.Tx, id string, status model.BatchStatus, startedAt, endedAt int64) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE batches SET status=?, started_at=?, ended_at=? WHERE id=?`, string(status), startedAt, endedAt, id)
	if err != nil {
		return fmt.Errorf("update batch status: %w", err)
	}
	return nil
}

// SetBatchResult persists the computed kinetics + safety figures onto a batch.
func (s *Store) SetBatchResult(ctx context.Context, tx *sql.Tx, id string, res model.KineticsResult) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE batches SET peak_temp=?,conversion=?,safety_verdict=?,stoessel_class=? WHERE id=?`,
		res.PeakTemp, res.Conversion, string(res.Verdict), res.StoesselClass, id)
	if err != nil {
		return fmt.Errorf("set batch result: %w", err)
	}
	return nil
}

// SetBatchFault marks a batch faulted with a reason.
func (s *Store) SetBatchFault(ctx context.Context, tx *sql.Tx, id string, status model.BatchStatus, reason string, endedAt int64) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE batches SET status=?, fault_reason=?, ended_at=? WHERE id=?`, string(status), reason, endedAt, id)
	if err != nil {
		return fmt.Errorf("set batch fault: %w", err)
	}
	return nil
}
