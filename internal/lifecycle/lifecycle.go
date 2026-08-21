// Package lifecycle is the batch state machine. It owns the legal transitions
// and the guards that enforce the engine's business boundaries:
//
//	queued    → charging    (reactor free + thermal compatible)
//	charging  → reacting    (recipe/reactor known)
//	reacting  → cooling     (kinetics result computed; not runaway)
//	reacting  → faulted     (runaway verdict: PeakT≥ThermalLimit or class 5)
//	cooling   → discharging (temperature below safe threshold)
//	discharging → done      (conversion ≥ MinConversion)
//	discharging → faulted   (conversion short and operator not reworking)
//	cleaning  → done
//	any non-terminal → aborted (operator cancel)
//
// Each transition returns the target status or an error naming the violated
// invariant. The errors wrap the store sentinels so the HTTP layer maps them to
// the right status.
package lifecycle

import (
	"errors"
	"fmt"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/store"
)

// ErrIllegalTransition is returned when a transition is not allowed from the
// current status (regardless of guards).
var ErrIllegalTransition = errors.New("lifecycle: illegal transition")

// TransitionContext carries the data the guards need to decide a transition:
// the kinetics result (after reacting) and the recipe's min-conversion spec.
// Either may be nil when the transition does not need it.
type TransitionContext struct {
	Result         *model.KineticsResult
	MinConversion  float64
}

// NextStatus validates a requested transition from→target and returns the
// resolved target status. The target may differ from the request when the
// reacting stage detects a runaway (target "cooling" resolves to "faulted").
func NextStatus(b model.Batch, target model.BatchStatus, tc *TransitionContext) (model.BatchStatus, error) {
	switch target {
	case model.BatchCharging:
		if b.Status != model.BatchQueued {
			return b.Status, illegal(b.Status, target)
		}
		return model.BatchCharging, nil

	case model.BatchReacting:
		if b.Status != model.BatchCharging {
			return b.Status, illegal(b.Status, target)
		}
		return model.BatchReacting, nil

	case model.BatchCooling:
		if b.Status != model.BatchReacting {
			return b.Status, illegal(b.Status, target)
		}
		if tc == nil || tc.Result == nil {
			return b.Status, fmt.Errorf("%w: kinetics result missing", store.ErrInvariant)
		}
		// Runaway guard: a runaway verdict forces the batch to faulted and
		// forbids proceeding to cooling.
		if tc.Result.Verdict == model.VerdictRunaway {
			return model.BatchFaulted, nil
		}
		return model.BatchCooling, nil

	case model.BatchFaulted:
		// Explicit faulting is only valid from reacting (runway) or
		// discharging (short conversion). Operator-aborted batches use
		// aborted instead.
		if b.Status != model.BatchReacting && b.Status != model.BatchDischarging {
			return b.Status, illegal(b.Status, target)
		}
		return model.BatchFaulted, nil

	case model.BatchDischarging:
		if b.Status != model.BatchCooling {
			return b.Status, illegal(b.Status, target)
		}
		return model.BatchDischarging, nil

	case model.BatchCleaning:
		if b.Status != model.BatchDischarging {
			return b.Status, illegal(b.Status, target)
		}
		// Conversion gate: only product that meets the spec may proceed to
		// cleaning/done. Short conversion must be faulted or aborted. The
		// authoritative conversion is the kinetics result (recomputed at the
		// reacting stage) when available, else the value persisted on the
		// batch row by the reacting stage.
		conv := b.Conversion
		if tc != nil && tc.Result != nil {
			conv = tc.Result.Conversion
		}
		if conv < tc.MinConversion {
			return b.Status, fmt.Errorf("%w: conversion %.4f below spec %.4f", store.ErrLowConversion, conv, tc.MinConversion)
		}
		return model.BatchCleaning, nil

	case model.BatchDone:
		if b.Status != model.BatchCleaning && b.Status != model.BatchDischarging {
			return b.Status, illegal(b.Status, target)
		}
		return model.BatchDone, nil

	case model.BatchAborted:
		if b.Status.IsTerminal() {
			return b.Status, illegal(b.Status, target)
		}
		return model.BatchAborted, nil
	}
	return b.Status, illegal(b.Status, target)
}

// FaultReasonFor returns the human-readable reason a batch entered faulted from
// the given prior status and result.
func FaultReasonFor(from model.BatchStatus, tc *TransitionContext) string {
	if from == model.BatchReacting {
		if tc == nil || tc.Result == nil {
			return "runaway: kinetics result unavailable"
		}
		if tc.Result.Verdict == model.VerdictRunaway {
			return fmt.Sprintf("thermal runaway: peak %.2f K, stoessel %d, tmr %.0f s", tc.Result.PeakTemp, tc.Result.StoesselClass, tc.Result.TMRSeconds)
		}
		return "faulted during reaction"
	}
	if from == model.BatchDischarging {
		return "short conversion: product below spec"
	}
	return "faulted"
}

// illegal wraps ErrIllegalTransition with the from→to context.
func illegal(from, to model.BatchStatus) error {
	return fmt.Errorf("%w: %s → %s", ErrIllegalTransition, from, to)
}
