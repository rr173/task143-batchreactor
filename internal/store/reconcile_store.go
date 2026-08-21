package store

import (
	"context"
	"database/sql"
	"fmt"

	"task143-batchreactor/internal/model"
)

// statusFromEvent maps the latest lifecycle event_type back to a batch status.
// This is the authoritative recovery rule: the event log wins over the
// batches.status column, so a torn write between an UPDATE and an event
// append converges to the event.
func statusFromEvent(et string) model.BatchStatus {
	switch et {
	case "queued":
		return model.BatchQueued
	case "charging":
		return model.BatchCharging
	case "reacting":
		return model.BatchReacting
	case "cooling":
		return model.BatchCooling
	case "discharging":
		return model.BatchDischarging
	case "cleaning":
		return model.BatchCleaning
	case "done":
		return model.BatchDone
	case "faulted":
		return model.BatchFaulted
	case "aborted":
		return model.BatchAborted
	}
	return model.BatchQueued
}

// RecomputeBatch re-derives a batch's kinetics result from its stored recipe +
// reactor inputs and writes it to safety_results. It also corrects the
// reactor's busy/available status based on the batch's terminal state. The
// caller passes the recomputed result so the heavy math stays in the thermal
// package; this function only persists.
func (s *Store) RecomputeBatch(ctx context.Context, batchID string, res model.KineticsResult, verdict model.SafetyVerdict) error {
	sr := model.SafetyResult{
		BatchID: batchID, DeltaTad: res.DeltaTad, MTSR: res.MTSR,
		StoesselClass: res.StoesselClass, TMRSeconds: res.TMRSeconds,
		PeakTemp: res.PeakTemp, Conversion: res.Conversion, Verdict: verdict,
	}
	return s.InTx(ctx, func(tx *sql.Tx) error {
		if err := s.UpsertSafetyResult(ctx, tx, sr); err != nil {
			return err
		}
		// Persist the recomputed verdict/conversion back onto the batch row so
		// reads of batches.* stay consistent with safety_results.
		_, err := tx.ExecContext(ctx, `UPDATE batches SET peak_temp=?,conversion=?,safety_verdict=?,stoessel_class=? WHERE id=?`,
			res.PeakTemp, res.Conversion, string(verdict), res.StoesselClass, batchID)
		if err != nil {
			return fmt.Errorf("reconcile batch result: %w", err)
		}
		return nil
	})
}

// CorrectBatchStatus rewrites a batch's status from the latest lifecycle event
// in the log (the authoritative recovery source).
func (s *Store) CorrectBatchStatus(ctx context.Context, batchID string) error {
	return s.InTx(ctx, func(tx *sql.Tx) error {
		et, err := s.LatestEventType(ctx, tx, batchID)
		if err != nil {
			return err
		}
		if et == "" {
			return nil
		}
		st := statusFromEvent(et)
		_, err = tx.ExecContext(ctx, `UPDATE batches SET status=? WHERE id=?`, string(st), batchID)
		if err != nil {
			return fmt.Errorf("correct batch status: %w", err)
		}
		return nil
	})
}

// RefreshReactorStatus recomputes a reactor's busy/available status from its
// currently-active (non-terminal) batch.
func (s *Store) RefreshReactorStatus(ctx context.Context, reactorID string) error {
	return s.InTx(ctx, func(tx *sql.Tx) error {
		var n int
		err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM batches WHERE reactor_id=? AND status NOT IN ('done','faulted','aborted')`, reactorID).Scan(&n)
		if err != nil {
			return err
		}
		st := model.ReactorAvailable
		if n > 0 {
			st = model.ReactorBusy
		}
		_, err = tx.ExecContext(ctx, `UPDATE reactors SET status=? WHERE id=?`, string(st), reactorID)
		return err
	})
}
