package lifecycle

import (
	"errors"
	"testing"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/store"
)

func TestQueuedToCharging(t *testing.T) {
	b := model.Batch{Status: model.BatchQueued}
	got, err := NextStatus(b, model.BatchCharging, nil)
	if err != nil || got != model.BatchCharging {
		t.Fatalf("queued→charging: got %s err %v", got, err)
	}
}

func TestIllegalTransition(t *testing.T) {
	b := model.Batch{Status: model.BatchQueued}
	_, err := NextStatus(b, model.BatchCooling, nil)
	if !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("queued→cooling must be illegal: %v", err)
	}
}

func TestRunawayForcesFault(t *testing.T) {
	b := model.Batch{Status: model.BatchReacting}
	tc := &TransitionContext{Result: &model.KineticsResult{Verdict: model.VerdictRunaway}, MinConversion: 0.5}
	got, err := NextStatus(b, model.BatchCooling, tc)
	if err != nil || got != model.BatchFaulted {
		t.Fatalf("runaway during reacting→cooling must resolve to faulted: got %s err %v", got, err)
	}
}

func TestConversionGateRejects(t *testing.T) {
	b := model.Batch{Status: model.BatchDischarging, Conversion: 0.3}
	tc := &TransitionContext{Result: &model.KineticsResult{Conversion: 0.3, Verdict: model.VerdictSafe}, MinConversion: 0.8}
	_, err := NextStatus(b, model.BatchCleaning, tc)
	if !errors.Is(err, store.ErrLowConversion) {
		t.Fatalf("conversion below spec must reject cleaning: %v", err)
	}
}

func TestConversionGateAllowsSpec(t *testing.T) {
	b := model.Batch{Status: model.BatchDischarging, Conversion: 0.9}
	tc := &TransitionContext{Result: &model.KineticsResult{Conversion: 0.9, Verdict: model.VerdictSafe}, MinConversion: 0.8}
	got, err := NextStatus(b, model.BatchCleaning, tc)
	if err != nil || got != model.BatchCleaning {
		t.Fatalf("conversion at spec must allow cleaning: got %s err %v", got, err)
	}
}

func TestConversionGateAllowsExactSpec(t *testing.T) {
	// A batch whose conversion lands exactly on MinConversion meets the release
	// spec and must be allowed to proceed to cleaning. The gate must not turn
	// the equality case into a block.
	b := model.Batch{Status: model.BatchDischarging, Conversion: 0.80}
	tc := &TransitionContext{Result: &model.KineticsResult{Conversion: 0.80, Verdict: model.VerdictSafe}, MinConversion: 0.80}
	got, err := NextStatus(b, model.BatchCleaning, tc)
	if err != nil || got != model.BatchCleaning {
		t.Fatalf("conversion exactly at spec must allow cleaning: got %s err %v", got, err)
	}
}

func TestConversionGateFallsBackToBatchConversion(t *testing.T) {
	// No recomputed result (e.g. advancing discharging after a restart that
	// didn't re-run the reacting guard): the gate uses the persisted batch
	// conversion.
	b := model.Batch{Status: model.BatchDischarging, Conversion: 0.2}
	tc := &TransitionContext{MinConversion: 0.8}
	_, err := NextStatus(b, model.BatchCleaning, tc)
	if !errors.Is(err, store.ErrLowConversion) {
		t.Fatalf("gate must fall back to batch conversion: %v", err)
	}
}

func TestAbortedFromAnyNonTerminal(t *testing.T) {
	for _, st := range []model.BatchStatus{model.BatchQueued, model.BatchCharging, model.BatchReacting, model.BatchCooling, model.BatchDischarging, model.BatchCleaning} {
		b := model.Batch{Status: st}
		got, err := NextStatus(b, model.BatchAborted, nil)
		if err != nil || got != model.BatchAborted {
			t.Fatalf("%s→aborted: got %s err %v", st, got, err)
		}
	}
}

func TestAbortedFromTerminalIllegal(t *testing.T) {
	for _, st := range []model.BatchStatus{model.BatchDone, model.BatchFaulted, model.BatchAborted} {
		b := model.Batch{Status: st}
		_, err := NextStatus(b, model.BatchAborted, nil)
		if !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("%s→aborted must be illegal: %v", st, err)
		}
	}
}

func TestFaultReasonForRunaway(t *testing.T) {
	tc := &TransitionContext{Result: &model.KineticsResult{Verdict: model.VerdictRunaway, PeakTemp: 500, StoesselClass: 5, TMRSeconds: 10}}
	got := FaultReasonFor(model.BatchReacting, tc)
	if got == "" {
		t.Fatal("runaway fault reason must be non-empty")
	}
}
