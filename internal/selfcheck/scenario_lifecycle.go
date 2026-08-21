package selfcheck

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"task143-batchreactor/internal/clock"
	"task143-batchreactor/internal/model"
)

// smokeLifecycleFlow exercises the full happy-path batch lifecycle: create a
// campaign with the exothermic recipe, plan it, start it, then advance the
// batch queued→charging→reacting→cooling→discharging→cleaning→done. The
// reacting stage computes a safe verdict, so cooling is reached; conversion
// meets spec, so discharging→cleaning→done succeeds.
func smokeLifecycleFlow(srv *httptest.Server, clk *clock.Fake) error {
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
	cid, err := createCampaign(srv, "lifecycle-campaign")
	if err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid),
		map[string]any{"recipe_id": fid, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/plan", cid), nil, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/start", cid), nil, nil); err != nil {
		return err
	}
	batch, err := firstBatchOf(srv, cid)
	if err != nil {
		return err
	}
	// Reactor must be available until charging starts.
	advance := func(target model.BatchStatus) (model.Batch, error) {
		return advanceBatch(srv, batch.ID, target)
	}
	if _, err := advance(model.BatchCharging); err != nil {
		return fmt.Errorf("charging: %w", err)
	}
	if _, err := advance(model.BatchReacting); err != nil {
		return fmt.Errorf("reacting: %w", err)
	}
	if _, err := advance(model.BatchCooling); err != nil {
		return fmt.Errorf("cooling: %w", err)
	}
	if _, err := advance(model.BatchDischarging); err != nil {
		return fmt.Errorf("discharging: %w", err)
	}
	if _, err := advance(model.BatchCleaning); err != nil {
		return fmt.Errorf("cleaning: %w", err)
	}
	final, err := advance(model.BatchDone)
	if err != nil {
		return fmt.Errorf("done: %w", err)
	}
	if final.Status != model.BatchDone {
		return fmt.Errorf("final status: got %s want done", final.Status)
	}
	if final.Conversion < rcp.MinConversion {
		return fmt.Errorf("conversion %.4f below spec %.4f after done", final.Conversion, rcp.MinConversion)
	}
	// Reactor freed after a terminal batch.
	var rc2 model.Reactor
	if err := mustDo(srv, "GET", fmt.Sprintf("/api/reactors/%s", rid), nil, &rc2); err != nil {
		return err
	}
	if rc2.Status != model.ReactorAvailable {
		return fmt.Errorf("reactor status after done: got %s want available", rc2.Status)
	}
	return nil
}

// smokeThermalRunawayFault asserts a runaway recipe (ΔT_ad≥200 / class 5) makes
// the reacting stage fault the batch and forbids discharge.
func smokeThermalRunawayFault(srv *httptest.Server, clk *clock.Fake) error {
	rcp := runawayRecipe()
	rc := safeReactor()
	rid, err := createReactor(srv, rc)
	if err != nil {
		return err
	}
	fid, err := createRecipe(srv, rcp)
	if err != nil {
		return err
	}
	cid, err := createCampaign(srv, "runaway-campaign")
	if err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid),
		map[string]any{"recipe_id": fid, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/plan", cid), nil, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/start", cid), nil, nil); err != nil {
		return err
	}
	batch, err := firstBatchOf(srv, cid)
	if err != nil {
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchCharging); err != nil {
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchReacting); err != nil {
		return err
	}
	// Requesting cooling must resolve to faulted (runaway guard).
	got, err := advanceBatch(srv, batch.ID, model.BatchCooling)
	if err != nil {
		return fmt.Errorf("cooling after runaway: %w", err)
	}
	if got.Status != model.BatchFaulted {
		return fmt.Errorf("runaway batch status: got %s want faulted", got.Status)
	}
	if got.SafetyVerdict != model.VerdictRunaway {
		return fmt.Errorf("runaway verdict: got %s want runaway", got.SafetyVerdict)
	}
	// Discharge must be rejected from a faulted batch (illegal transition).
	if err := expectCode(srv, "POST", fmt.Sprintf("/api/batches/%s/advance", batch.ID),
		map[string]any{"target": string(model.BatchDischarging)}, http.StatusConflict); err != nil {
		return err
	}
	return nil
}

// smokeConversionGate asserts a batch whose conversion is below spec cannot
// proceed to cleaning (discharging→cleaning rejected with 422).
func smokeConversionGate(srv *httptest.Server, clk *clock.Fake) error {
	rcp := shortConversionRecipe()
	rc := safeReactor()
	rid, err := createReactor(srv, rc)
	if err != nil {
		return err
	}
	fid, err := createRecipe(srv, rcp)
	if err != nil {
		return err
	}
	cid, err := createCampaign(srv, "short-campaign")
	if err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid),
		map[string]any{"recipe_id": fid, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/plan", cid), nil, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/start", cid), nil, nil); err != nil {
		return err
	}
	batch, err := firstBatchOf(srv, cid)
	if err != nil {
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchCharging); err != nil {
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchReacting); err != nil {
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchCooling); err != nil {
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchDischarging); err != nil {
		return err
	}
	// cleaning (discharging→cleaning) must be rejected: conversion below spec.
	if err := expectCode(srv, "POST", fmt.Sprintf("/api/batches/%s/advance", batch.ID),
		map[string]any{"target": string(model.BatchCleaning)}, http.StatusUnprocessableEntity); err != nil {
		return err
	}
	return nil
}

// smokeFrontend asserts the embedded page is served and contains the expected
// anchor marker so a missing/empty embed is caught.
func smokeFrontend(srv *httptest.Server, clk *clock.Fake) error {
	code, body, _ := doJSON(srv, "GET", "/", nil)
	if code != http.StatusOK {
		return fmt.Errorf("frontend GET /: want 200, got %d", code)
	}
	if len(body) == 0 {
		return fmt.Errorf("frontend GET /: empty body")
	}
	bs := string(body)
	if !contains(bs, "batch-reactor") && !contains(bs, "Batch Reactor") {
		return fmt.Errorf("frontend GET /: missing page marker")
	}
	return nil
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
