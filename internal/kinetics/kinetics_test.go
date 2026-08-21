package kinetics

import (
	"math"
	"testing"

	"task143-batchreactor/internal/model"
)

// mildRecipe is a first-order exotherm with good jacket cooling: the
// trajectory is numerically stable and the reactant is consumed to near
// completion over 1 h.
func mildRecipe() model.Recipe {
	return model.Recipe{K0: 2.1e6, Ea: 60000, Order: model.OrderFirst, CA0: 2000,
		DeltaHrx: -60000, Rho: 1000, Cp: 3000, T0: 333.15, JacketTemp: 333.15,
		ThermalLimit: 450.0, MinConversion: 0.95, Duration: 3600}
}

func cooledReactor() model.Reactor {
	return model.Reactor{Volume: 2.0, HeatTransferU: 2000, HeatTransferArea: 10.0, MaxOperatingTemp: 600.0}
}

func TestArrheniusRateMonotonic(t *testing.T) {
	k1 := ArrheniusRate(1e6, 60000, 300)
	k2 := ArrheniusRate(1e6, 60000, 400)
	if k2 <= k1 {
		t.Fatalf("rate must increase with T: k(300)=%.4e k(400)=%.4e", k1, k2)
	}
	if k1 <= 0 {
		t.Fatalf("rate must be positive at finite T: %.4e", k1)
	}
}

func TestArrheniusRateNonPositiveTemp(t *testing.T) {
	if ArrheniusRate(1e6, 60000, 0) != 0 {
		t.Fatal("rate at T=0 must be 0")
	}
	if ArrheniusRate(1e6, 60000, -10) != 0 {
		t.Fatal("rate at negative T must be 0")
	}
}

func TestIntegrateConsumesReactant(t *testing.T) {
	out, err := Integrate(mildRecipe(), cooledReactor(), 3600, 1.0)
	if err != nil {
		t.Fatalf("integrate: %v", err)
	}
	if out.Conversion < 0.9 {
		t.Fatalf("expected near-full conversion, got %.4f", out.Conversion)
	}
	if out.PeakTemp < mildRecipe().T0 {
		t.Fatalf("peak must be ≥ T0: %.2f", out.PeakTemp)
	}
	if out.PeakTemp >= mildRecipe().ThermalLimit {
		t.Fatalf("cooled mild recipe must stay below thermal limit: peak %.2f", out.PeakTemp)
	}
}

func TestIntegrateDeterministic(t *testing.T) {
	r, rc := mildRecipe(), cooledReactor()
	a, _ := Integrate(r, rc, 3600, 1.0)
	b, _ := Integrate(r, rc, 3600, 1.0)
	if a.Conversion != b.Conversion || a.PeakTemp != b.PeakTemp {
		t.Fatal("integration must be deterministic")
	}
}

func TestIntegrateIsothermalMatchesAnalytic(t *testing.T) {
	// U=0, JacketTemp==T0 → isothermal path. For a first-order reaction
	// C_A(t)=C_A0·exp(-k·t); conversion = 1-exp(-k·t).
	r := mildRecipe()
	r.DeltaHrx = 0 // endotherm-neutral so heat balance is irrelevant
	rc := model.Reactor{Volume: 2.0, HeatTransferU: 0, HeatTransferArea: 0}
	out, err := Integrate(r, rc, 3600, 1.0)
	if err != nil {
		t.Fatalf("integrate: %v", err)
	}
	k := ArrheniusRate(r.K0, r.Ea, r.T0)
	want := 1 - math.Exp(-k*3600)
	if math.Abs(out.Conversion-want) > 1e-3 {
		t.Fatalf("isothermal conversion: got %.6f want %.6f", out.Conversion, want)
	}
	if !out.Isothermal {
		t.Fatal("expected isothermal flag")
	}
}

func TestIntegrateBadInput(t *testing.T) {
	r := mildRecipe()
	r.CA0 = 0
	if _, err := Integrate(r, cooledReactor(), 3600, 1.0); err != ErrBadInput {
		t.Fatalf("expected ErrBadInput, got %v", err)
	}
}

func TestIntegrateRunawayCappedNotNaN(t *testing.T) {
	// ΔT_ad ≥ 200K, fast kinetics → thermal runaway; the trajectory must not
	// produce NaN/Inf and conversion must be finite.
	r := model.Recipe{K0: 1e7, Ea: 60000, Order: model.OrderFirst, CA0: 2000,
		DeltaHrx: -300000, Rho: 1000, Cp: 3000, T0: 343.15, JacketTemp: 343.15,
		ThermalLimit: 420.0, MinConversion: 0.90, Duration: 1200}
	out, err := Integrate(r, cooledReactor(), 1200, 1.0)
	if err != nil {
		t.Fatalf("integrate runaway: %v", err)
	}
	if math.IsNaN(out.PeakTemp) || math.IsInf(out.PeakTemp, 0) {
		t.Fatalf("peak must be finite: %v", out.PeakTemp)
	}
	if math.IsNaN(out.Conversion) || out.Conversion < 0 || out.Conversion > 1 {
		t.Fatalf("conversion out of range: %v", out.Conversion)
	}
}
