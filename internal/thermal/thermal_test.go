package thermal

import (
	"math"
	"testing"

	"task143-batchreactor/internal/model"
)

func mildRecipe() model.Recipe {
	return model.Recipe{K0: 2.1e6, Ea: 60000, Order: model.OrderFirst, CA0: 2000,
		DeltaHrx: -60000, Rho: 1000, Cp: 3000, T0: 333.15, JacketTemp: 333.15,
		ThermalLimit: 450.0, MinConversion: 0.95, Duration: 3600}
}

func cooledReactor() model.Reactor {
	return model.Reactor{Volume: 2.0, HeatTransferU: 2000, HeatTransferArea: 10.0, MaxOperatingTemp: 600.0}
}

func TestDeltaTAd(t *testing.T) {
	r := mildRecipe()
	// (-ΔH)·CA0/(ρ·Cp) = 60000·2000/(1000·3000) = 40.
	if got := DeltaTAd(r); math.Abs(got-40.0) > 1e-9 {
		t.Fatalf("DeltaTAd: got %.6f want 40", got)
	}
}

func TestMTSR(t *testing.T) {
	r := mildRecipe()
	want := r.T0 + 40.0
	if got := MTSR(r); math.Abs(got-want) > 1e-6 {
		t.Fatalf("MTSR: got %.6f want %.6f", got, want)
	}
}

func TestStoesselClass(t *testing.T) {
	cases := []struct{ dtad, tol float64; want int }{
		{5, 1e-9, 1}, {9.999, 1e-9, 1},
		{10, 1e-9, 2}, {49.999, 1e-9, 2},
		{50, 1e-9, 3}, {149.999, 1e-9, 3},
		{150, 1e-9, 4}, {199.999, 1e-9, 4},
		{200, 1e-9, 5}, {500, 1e-9, 5},
	}
	for _, c := range cases {
		if got := StoesselClass(c.dtad); got != c.want {
			t.Errorf("StoesselClass(%.3f)=%d want %d", c.dtad, got, c.want)
		}
	}
}

func TestTMRPositiveForExotherm(t *testing.T) {
	r := mildRecipe()
	tmr := TMR(r, r.T0)
	if math.IsNaN(tmr) || math.IsInf(tmr, 0) || tmr <= 0 {
		t.Fatalf("TMR must be finite positive for an exotherm: %v", tmr)
	}
}

func TestTMREndothermInf(t *testing.T) {
	r := mildRecipe()
	r.DeltaHrx = 5000 // endotherm → no runaway concern
	if tmr := TMR(r, r.T0); !math.IsInf(tmr, 1) {
		t.Fatalf("TMR for endotherm must be +Inf, got %v", tmr)
	}
}

func TestVerdictRunawayClass5(t *testing.T) {
	r := mildRecipe()
	if got := Verdict(r, 350, 5); got != model.VerdictRunaway {
		t.Fatalf("class 5 → runaway, got %s", got)
	}
}

func TestVerdictPeakAtLimit(t *testing.T) {
	r := mildRecipe()
	if got := Verdict(r, r.ThermalLimit, 2); got != model.VerdictRunaway {
		t.Fatalf("peak≥limit → runaway, got %s", got)
	}
	if got := Verdict(r, r.ThermalLimit-5, 2); got != model.VerdictMarginal {
		t.Fatalf("peak within 10K of limit → marginal, got %s", got)
	}
}

func TestVerdictSafe(t *testing.T) {
	r := mildRecipe()
	if got := Verdict(r, 340, 2); got != model.VerdictSafe {
		t.Fatalf("cool low-class trajectory → safe, got %s", got)
	}
}

func TestIsThermallyCompatible(t *testing.T) {
	r := mildRecipe() // MTSR = 373 K
	rc := model.Reactor{MaxOperatingTemp: 400}
	if !IsThermallyCompatible(r, rc) {
		t.Fatal("MTSR 373 ≤ 400 → compatible")
	}
	rc.MaxOperatingTemp = 340
	if IsThermallyCompatible(r, rc) {
		t.Fatal("MTSR 373 > 340 → incompatible")
	}
	rc.MaxOperatingTemp = 0
	if !IsThermallyCompatible(r, rc) {
		t.Fatal("unrated reactor (0) → compatible")
	}
}

func TestClassifyMildRecipeSafe(t *testing.T) {
	res, err := Classify(mildRecipe(), cooledReactor())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if res.Verdict != model.VerdictSafe {
		t.Fatalf("mild recipe verdict: got %s want safe", res.Verdict)
	}
	if res.StoesselClass != 2 {
		t.Fatalf("stoessel: got %d want 2", res.StoesselClass)
	}
	if res.Conversion < 0.9 {
		t.Fatalf("conversion too low: %.4f", res.Conversion)
	}
}
