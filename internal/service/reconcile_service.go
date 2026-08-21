package service

import (
	"context"
	"database/sql"
	"fmt"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/store"
	"task143-batchreactor/internal/thermal"
)

// Reconciler re-derives state on restart. ReconcileAll iterates every batch,
// recomputes its kinetics result from the stored recipe+reactor (deterministic)
// and corrects the batch status from the lifecycle event log; it then refreshes
// each reactor's busy/available status. The whole pass depends only on stored
// inputs, so a crashed-and-restarted process converges to the same state.
type Reconciler struct{ svc *Services }

// ReconcileAll recomputes every derived figure and corrects statuses.
func (r *Reconciler) ReconcileAll(ctx context.Context) (int, int, error) {
	ids, err := r.svc.st.AllBatchIDs(ctx)
	if err != nil {
		return 0, 0, err
	}
	recomputed := 0
	corrected := 0
	for _, id := range ids {
		b, err := r.svc.st.GetBatch(ctx, id)
		if err != nil {
			return recomputed, corrected, err
		}
		rcp, err := r.svc.st.GetRecipe(ctx, b.RecipeID)
		if err != nil {
			return recomputed, corrected, err
		}
		rc, err := r.svc.st.GetReactor(ctx, b.ReactorID)
		if err != nil {
			return recomputed, corrected, err
		}
		res, err := thermal.Classify(*rcp, *rc)
		if err != nil {
			return recomputed, corrected, err
		}
		// Recompute keeps the persisted verdict consistent with the recipe+reactor
		// inputs. A batch still in reacting (crashed mid-stage) keeps its recomputed
		// result but is NOT force-faulted: the operator re-advances it.
		verdict := res.Verdict
		if b.Status == model.BatchQueued || b.Status == model.BatchCharging {
			// Not yet reacted: no verdict, but persist the recomputed figures for
			// the safety preview.
			verdict = res.Verdict
		}
		if err := r.svc.st.RecomputeBatch(ctx, id, res, verdict); err != nil {
			return recomputed, corrected, err
		}
		recomputed++

		// Correct status from the event log.
		before := b.Status
		latest, err := r.svc.st.LatestEventType(ctx, nil, id)
		if err != nil {
			return recomputed, corrected, err
		}
		want := eventStatusFromString(latest)
		if want != "" && want != before {
			if err := setStatus(ctx, r.svc.st, id, want); err != nil {
				return recomputed, corrected, err
			}
			corrected++
		}
		// Refresh reactor status from active batches.
		_ = r.svc.st.RefreshReactorStatus(ctx, b.ReactorID)
	}
	return recomputed, corrected, nil
}

// setStatus rewrites a batch status directly (recovery-only path).
func setStatus(ctx context.Context, st *store.Store, id string, st2 model.BatchStatus) error {
	return st.InTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE batches SET status=? WHERE id=?`, string(st2), id)
		return err
	})
}

// eventStatusFromString maps an event_type to a batch status.
func eventStatusFromString(et string) model.BatchStatus {
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

// CampaignFullReport assembles a campaign with its items, batches and the
// reactor timeline for the full-report endpoint.
func (s *Services) CampaignFullReport(ctx context.Context, campaignID string) (model.Campaign, []model.CampaignItem, []model.Batch, error) {
	c, items, err := s.GetCampaignDetail(ctx, campaignID)
	if err != nil {
		return model.Campaign{}, nil, nil, err
	}
	batches, err := s.st.ListBatchesByCampaign(ctx, campaignID)
	if err != nil {
		return model.Campaign{}, nil, nil, err
	}
	if c == nil {
		return model.Campaign{}, nil, nil, fmt.Errorf("%w: campaign nil", store.ErrInvariant)
	}
	return *c, items, batches, nil
}
