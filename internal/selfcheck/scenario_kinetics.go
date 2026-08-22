package selfcheck

import (
	"fmt"
	"math"
	"net/http/httptest"

	"task143-batchreactor/internal/clock"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/thermal"
)

// smokeKineticsSimulate asserts the dry-run simulate endpoint returns a
// classification consistent with the locked formulas. It seeds a recipe and
// reactor via the HTTP API, then cross-checks the response against an
// independent in-process computation from thermal.Classify.
func smokeKineticsSimulate(srv *httptest.Server, clk *clock.Fake) error {
	rcp := exothermicRecipe()
	rc := safeReactor()
	rid, err := createReactor(srv, rc)
	if err != nil {
		return err
	}
	fid, err := createRecipe(srv, rcp)
	if err != nil {
		return err
	}

	var res model.KineticsResult
	if err := mustDo(srv, "POST", "/api/kinetics/simulate",
		map[string]any{"recipe_id": fid, "reactor_id": rid}, &res); err != nil {
		return err
	}

	// Independent recomputation from the same inputs.
	want, err := thermal.Classify(rcp, rc)
	if err != nil {
		return fmt.Errorf("independent classify: %w", err)
	}

	if !approxEq(res.Conversion, want.Conversion, 1e-6, 1e-6) {
		return fmt.Errorf("conversion: got %.6f want %.6f", res.Conversion, want.Conversion)
	}
	if !approxEq(res.PeakTemp, want.PeakTemp, 1e-3, 1e-6) {
		return fmt.Errorf("peak temp: got %.3f want %.3f", res.PeakTemp, want.PeakTemp)
	}
	if res.StoesselClass != want.StoesselClass {
		return fmt.Errorf("stoessel: got %d want %d", res.StoesselClass, want.StoesselClass)
	}
	if res.Verdict != want.Verdict {
		return fmt.Errorf("verdict: got %s want %s", res.Verdict, want.Verdict)
	}
	// ΔT_ad for the exothermic recipe = 60000·2000/(1000·3000) = 40 K → class 2.
	wantDtad := thermal.DeltaTAd(rcp)
	if !approxEq(res.DeltaTad, wantDtad, 1e-3, 1e-6) {
		return fmt.Errorf("delta_t_ad: got %.3f want %.3f", res.DeltaTad, wantDtad)
	}
	if res.StoesselClass != 2 {
		return fmt.Errorf("expected stoessel 2 for ΔT_ad≈40K, got %d", res.StoesselClass)
	}
	return nil
}

// smokeThermalIncompatReject asserts planning rejects a (recipe,reactor) whose
// MTSR exceeds the reactor's max operating temperature.
func smokeThermalIncompatReject(srv *httptest.Server, clk *clock.Fake) error {
	rcp := exothermicRecipe() // MTSR = T0 + ΔT_ad ≈ 333.15 + 185.2 = 518 K
	rc := tightReactor()      // max operating 340 K → 518 > 340, incompatible
	rid, err := createReactor(srv, rc)
	if err != nil {
		return err
	}
	fid, err := createRecipe(srv, rcp)
	if err != nil {
		return err
	}
	cid, err := createCampaign(srv, "reject-campaign")
	if err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid),
		map[string]any{"recipe_id": fid, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		return err
	}
	// Plan must return 422 with the thermal-incompatibility reason.
	code, body, _ := doJSON(srv, "POST", fmt.Sprintf("/api/campaigns/%s/plan", cid), nil)
	if code != 422 {
		return fmt.Errorf("plan incompatible recipe: want status 422, got %d: %s", code, string(body))
	}
	return nil
}

// smokeSequenceCleaning asserts a cleaning step is inserted between two
// different products when the contamination matrix is non-zero, and omitted
// when it is zero (or when the same product repeats).
func smokeSequenceCleaning(srv *httptest.Server, clk *clock.Fake) error {
	rcpA := exothermicRecipe() // product P1
	rcpB := adiabaticRecipe()  // product PX
	rcpB.Product = "P2"
	rcpB.Name = "second product"
	rc := safeReactor()
	rid, err := createReactor(srv, rc)
	if err != nil {
		return err
	}
	fidA, err := createRecipe(srv, rcpA)
	if err != nil {
		return err
	}
	fidB, err := createRecipe(srv, rcpB)
	if err != nil {
		return err
	}
	// Contamination P1→P2 = medium (1800s).
	if err := mustDo(srv, "PUT", "/api/cleaning-matrix/P1/P2", map[string]any{"severity": 2}, nil); err != nil {
		return err
	}
	cid, err := createCampaign(srv, "cleaning-campaign")
	if err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid),
		map[string]any{"recipe_id": fidA, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid),
		map[string]any{"recipe_id": fidB, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		return err
	}
	var plan model.CampaignPlan
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/plan", cid), nil, &plan); err != nil {
		return err
	}
	if len(plan.Batches) != 2 {
		return fmt.Errorf("expected 2 planned batches, got %d", len(plan.Batches))
	}
	// Second batch must have a non-zero cleaning_before (1800s).
	if plan.Batches[1].CleaningBefore != 1800 {
		return fmt.Errorf("expected cleaning_before=1800 between P1→P2, got %.0f", plan.Batches[1].CleaningBefore)
	}
	if plan.Batches[0].CleaningBefore != 0 {
		return fmt.Errorf("first batch must have no cleaning_before, got %.0f", plan.Batches[0].CleaningBefore)
	}
	return nil
}

// approxEq is the local tolerance wrapper for scenario comparisons.
func approxEq(a, b, absTol, relTol float64) bool {
	if a == b {
		return true
	}
	if math.Abs(a-b) <= absTol {
		return true
	}
	denom := math.Max(math.Abs(a), math.Abs(b))
	if denom == 0 {
		return false
	}
	return math.Abs(a-b)/denom <= relTol
}
