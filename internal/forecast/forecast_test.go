package forecast

import (
	"math"
	"testing"

	"task143-batchreactor/internal/model"
)

// safeInputs is a minimal, well-behaved recipe+reactor pair whose thermal
// classification is safe and whose headroom/conversion gaps do not add risk
// reasons, so the only risk driver under test is the schedule slip.
func safeInputs() (model.Recipe, model.Reactor) {
	// ΔT_ad = 4 K (class 1) with a near-isothermal trajectory: peak stays
	// far below the thermal limit, so the verdict is safe and the headroom is
	// large. The recipe alone would still compute a finite TMR, so callers that
	// need the slip to be the *only* risk driver pass an explicit safe result
	// (see safeResult).
	r := model.Recipe{
		ID: "r1", Name: "safe", Product: "P", K0: 2.1e6, Ea: 60000,
		Order: model.OrderFirst, CA0: 2000, DeltaHrx: -6000, Rho: 1000, Cp: 3000,
		T0: 333.15, JacketTemp: 333.15, ThermalLimit: 450.0, MinConversion: 0.50, Duration: 3600,
	}
	rc := model.Reactor{
		ID: "k1", Name: "R-1", Volume: 2.0, HeatTransferU: 2000, HeatTransferArea: 10.0,
		MaxOperatingTemp: 600.0, MaxPressure: 12.0, Material: "SS316L",
	}
	return r, rc
}

// safeResult is a clean SafetyResult that makes the thermal reasons vanish so
// the schedule slip is the sole risk driver in tests that isolate slip behavior.
func safeResult(batchID string) *model.SafetyResult {
	return &model.SafetyResult{
		BatchID: batchID, Conversion: 0.99, PeakTemp: 335.0,
		StoesselClass: 1, TMRSeconds: math.Inf(1), Verdict: model.VerdictSafe,
	}
}

// containsReason reports whether the forecast surfaced the late-start reason.
func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}

// TestScheduleSlipLateStartPositive asserts that a batch which started after its
// planned slot reports a positive slip, a forecast window shifted by the same
// amount, and the "started after its planned slot" watch reason — all three
// describing the same lateness consistently. This is the regression guard for
// the sign-flipped slip that made the three pieces disagree.
func TestScheduleSlipLateStartPositive(t *testing.T) {
	r, rc := safeInputs()
	const planned = 10000
	const slip = 1200 // started 20 minutes late
	b := model.Batch{
		ID: "b1", ReactorID: "k1", RecipeID: "r1", Status: model.BatchCharging,
		PlannedStart: planned, StartedAt: planned + slip,
	}
	f := Build(Input{Batch: b, Recipe: r, Reactor: rc, Result: safeResult(b.ID), Now: planned + slip})

	if f.ScheduleSlip != slip {
		t.Fatalf("schedule slip: got %d, want %d (positive lateness)", f.ScheduleSlip, slip)
	}
	// The forecast window must be shifted by exactly the slip: the start
	// equals the late StartedAt, not the planned slot.
	if f.EstimatedStart != planned+slip {
		t.Fatalf("estimated start: got %d, want %d", f.EstimatedStart, planned+slip)
	}
	if f.ScheduleSlip != f.EstimatedStart-f.PlannedStart {
		t.Fatalf("slip %d disagrees with window shift %d", f.ScheduleSlip, f.EstimatedStart-f.PlannedStart)
	}
	// A late start must surface the watch reason — the operator-facing signal
	// that corresponds to the positive slip and the shifted window.
	if !containsReason(f.Reasons, "batch started after its planned slot") {
		t.Fatalf("late start missing reason; got %v", f.Reasons)
	}
	if f.Risk != model.RiskWatch {
		t.Fatalf("late start risk: got %s, want watch", f.Risk)
	}
}

// TestScheduleSlipOnTimeZero asserts an on-time (or not-yet-started) batch
// reports zero slip and no late-start reason, so the late-start signal only
// fires when there is real lateness.
func TestScheduleSlipOnTimeZero(t *testing.T) {
	r, rc := safeInputs()
	// Not started yet: estimatedStart falls back to PlannedStart → no slip.
	b := model.Batch{
		ID: "b2", ReactorID: "k1", RecipeID: "r1", Status: model.BatchQueued,
		PlannedStart: 5000,
	}
	f := Build(Input{Batch: b, Recipe: r, Reactor: rc, Now: 5000})
	if f.ScheduleSlip != 0 {
		t.Fatalf("queued slip: got %d, want 0", f.ScheduleSlip)
	}
	if containsReason(f.Reasons, "batch started after its planned slot") {
		t.Fatalf("queued batch should not surface late-start reason; got %v", f.Reasons)
	}
	// Started exactly on time → zero slip, no late-start reason.
	b2 := model.Batch{
		ID: "b3", ReactorID: "k1", RecipeID: "r1", Status: model.BatchCharging,
		PlannedStart: 5000, StartedAt: 5000,
	}
	f2 := Build(Input{Batch: b2, Recipe: r, Reactor: rc, Now: 5000})
	if f2.ScheduleSlip != 0 {
		t.Fatalf("on-time slip: got %d, want 0", f2.ScheduleSlip)
	}
	if containsReason(f2.Reasons, "batch started after its planned slot") {
		t.Fatalf("on-time batch should not surface late-start reason; got %v", f2.Reasons)
	}
}

// TestScheduleSlipMissingPlannedStartZero asserts a batch without a planned
// start never reports a slip (there is no slot to be late against).
func TestScheduleSlipMissingPlannedStartZero(t *testing.T) {
	r, rc := safeInputs()
	b := model.Batch{
		ID: "b4", ReactorID: "k1", RecipeID: "r1", Status: model.BatchCharging,
		StartedAt: 9000,
	}
	f := Build(Input{Batch: b, Recipe: r, Reactor: rc, Now: 9000})
	if f.ScheduleSlip != 0 {
		t.Fatalf("missing planned start slip: got %d, want 0", f.ScheduleSlip)
	}
}
