package service

import (
	"context"
	"database/sql"
	"fmt"

	"task143-batchreactor/internal/lifecycle"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/thermal"
	"task143-batchreactor/internal/store"
)

// ListBatchesByCampaign returns all batches of a campaign.
func (s *Services) ListBatchesByCampaign(ctx context.Context, campaignID string) ([]model.Batch, error) {
	return s.st.ListBatchesByCampaign(ctx, campaignID)
}

// GetBatch returns one batch.
func (s *Services) GetBatch(ctx context.Context, id string) (*model.Batch, error) {
	return s.st.GetBatch(ctx, id)
}

// GetBatchResult returns the derived safety result for a batch, recomputing it
// if the stored result is missing (so a freshly reacted batch is always
// readable even before the reconcile pass).
func (s *Services) GetBatchResult(ctx context.Context, batchID string) (*model.SafetyResult, error) {
	sr, err := s.st.GetSafetyResult(ctx, batchID)
	if err == nil {
		return sr, nil
	}
	if err != store.ErrNotFound {
		return nil, err
	}
	// Recompute on demand.
	b, err := s.st.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	r, err := s.st.GetRecipe(ctx, b.RecipeID)
	if err != nil {
		return nil, err
	}
	rc, err := s.st.GetReactor(ctx, b.ReactorID)
	if err != nil {
		return nil, err
	}
	res, err := thermal.Classify(*r, *rc)
	if err != nil {
		return nil, err
	}
	if err := s.st.RecomputeBatch(ctx, batchID, res, res.Verdict); err != nil {
		return nil, err
	}
	return s.st.GetSafetyResult(ctx, batchID)
}

// AdvanceBatch performs one lifecycle transition on a batch. It serializes on
// the reactor's mutex, validates the transition, and on the reacting→cooling
// step runs the kinetics classification (persisting the result and, on a
// runaway verdict, faulting the batch instead).
func (s *Services) AdvanceBatch(ctx context.Context, batchID string, target model.BatchStatus) (*model.Batch, error) {
	b, err := s.st.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if b.Status.IsTerminal() {
		return nil, fmt.Errorf("%w: batch %s is terminal (%s)", store.ErrStateConflict, batchID, b.Status)
	}
	mu := s.reactorLock(b.ReactorID)
	mu.Lock()
	defer mu.Unlock()

	r, err := s.st.GetRecipe(ctx, b.RecipeID)
	if err != nil {
		return nil, err
	}
	rc, err := s.st.GetReactor(ctx, b.ReactorID)
	if err != nil {
		return nil, err
	}

	var res *model.KineticsResult
	// Run kinetics when leaving the reacting stage.
	if b.Status == model.BatchReacting {
		classified, err := thermal.Classify(*r, *rc)
		if err != nil {
			return nil, err
		}
		res = &classified
	}

	tc := &lifecycle.TransitionContext{Result: res, MinConversion: r.MinConversion}
	next, err := lifecycle.NextStatus(*b, target, tc)
	if err != nil {
		return nil, err
	}

	// Reactor occupancy guard: charging a batch requires the reactor free.
	if target == model.BatchCharging && next == model.BatchCharging {
		active, err := s.st.ActiveBatchOnReactor(ctx, b.ReactorID)
		if err != nil {
			return nil, err
		}
		if active != nil && active.ID != b.ID {
			return nil, fmt.Errorf("%w: reactor %s has active batch %s", store.ErrReactorBusy, b.ReactorID, active.ID)
		}
	}

	err = s.st.InTx(ctx, func(tx *sql.Tx) error {
		now := s.now()
		startedAt, endedAt := b.StartedAt, b.EndedAt
		switch next {
		case model.BatchCharging:
			startedAt = now
		case model.BatchDone, model.BatchFaulted, model.BatchAborted:
			endedAt = now
		}
		// Persist the kinetics result if it was computed.
		if res != nil {
			if err := s.st.SetBatchResult(ctx, tx, b.ID, *res); err != nil {
				return err
			}
			sr := model.SafetyResult{
				BatchID: b.ID, DeltaTad: res.DeltaTad, MTSR: res.MTSR,
				StoesselClass: res.StoesselClass, TMRSeconds: res.TMRSeconds,
				PeakTemp: res.PeakTemp, Conversion: res.Conversion, Verdict: res.Verdict,
			}
			if err := s.st.UpsertSafetyResult(ctx, tx, sr); err != nil {
				return err
			}
		}
		if next == model.BatchFaulted {
			reason := lifecycle.FaultReasonFor(b.Status, tc)
			if err := s.st.SetBatchFault(ctx, tx, b.ID, model.BatchFaulted, reason, now); err != nil {
				return err
			}
		} else {
			if err := s.st.UpdateBatchStatus(ctx, tx, b.ID, next, startedAt, endedAt); err != nil {
				return err
			}
		}
		// Append the lifecycle event (authoritative for recovery).
		et := eventForStatus(next)
		if err := s.st.AppendEvent(ctx, tx, b.ID, string(et), now, ""); err != nil {
			return err
		}
		// Reactor status follows the batch: charging/reacting/cooling/discharging
		// → busy; done/faulted/aborted → available.
		if next.IsTerminal() {
			_ = s.st.UpdateReactorStatus(ctx, tx, b.ReactorID, model.ReactorAvailable)
		} else if next == model.BatchCharging {
			_ = s.st.UpdateReactorStatus(ctx, tx, b.ReactorID, model.ReactorBusy)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.st.GetBatch(ctx, batchID)
}

// AbortBatch cancels a non-terminal batch.
func (s *Services) AbortBatch(ctx context.Context, batchID string) (*model.Batch, error) {
	return s.AdvanceBatch(ctx, batchID, model.BatchAborted)
}

// eventForStatus maps a resolved batch status to the event_type written to the
// event log.
func eventForStatus(st model.BatchStatus) model.EventType {
	switch st {
	case model.BatchCharging:
		return model.EventCharging
	case model.BatchReacting:
		return model.EventReacting
	case model.BatchCooling:
		return model.EventCooling
	case model.BatchDischarging:
		return model.EventDischarging
	case model.BatchCleaning:
		return model.EventCleaning
	case model.BatchDone:
		return model.EventDone
	case model.BatchFaulted:
		return model.EventFaulted
	case model.BatchAborted:
		return model.EventAborted
	}
	return model.EventQueued
}

// ReactorTimeline returns the batches scheduled on a reactor in sequence order,
// for the frontend Gantt view.
func (s *Services) ReactorTimeline(ctx context.Context, reactorID string) ([]model.Batch, error) {
	return s.st.ListBatchesByReactor(ctx, reactorID)
}
