package selfcheck

import (
	"context"
	"fmt"
	"net/http/httptest"

	"task143-batchreactor/internal/clock"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/service"
	"task143-batchreactor/internal/thermal"
)

// smokeRestartRecovery seeds a campaign + planned batches + one batch reacted
// to cooling, then closes the store (simulating a crash), reopens the same
// SQLite file and runs ReconcileAll. The recomputed kinetics result and the
// event-log-corrected batch status must match the pre-crash state.
func smokeRestartRecovery(dbPath string, clk clock.Clock) error {
	ctx := context.Background()

	// --- Phase 1: seed state via the real mux. ---
	srv, err := newServer(dbPath, clk)
	if err != nil {
		return err
	}
	rcp := exothermicRecipe()
	rc := safeReactor()
	rid, err := createReactor(srv, rc)
	if err != nil {
		srv.Close()
		return err
	}
	fid, err := createRecipe(srv, rcp)
	if err != nil {
		srv.Close()
		return err
	}
	cid, err := createCampaign(srv, "restart-campaign")
	if err != nil {
		srv.Close()
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid),
		map[string]any{"recipe_id": fid, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		srv.Close()
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/plan", cid), nil, nil); err != nil {
		srv.Close()
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/start", cid), nil, nil); err != nil {
		srv.Close()
		return err
	}
	batch, err := firstBatchOf(srv, cid)
	if err != nil {
		srv.Close()
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchCharging); err != nil {
		srv.Close()
		return err
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchReacting); err != nil {
		srv.Close()
		return err
	}
	before, err := advanceBatch(srv, batch.ID, model.BatchCooling)
	if err != nil {
		srv.Close()
		return err
	}
	srv.Close()

	// --- Phase 2: reopen the same file and reconcile. ---
	srv2, st, err := restartServer(dbPath, clk)
	if err != nil {
		return err
	}
	defer srv2.Close()
	svc := service.NewWithClock(st, clk)
	recomputed, corrected, err := svc.Reconcile().ReconcileAll(ctx)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}
	if recomputed < 1 {
		return fmt.Errorf("reconcile recomputed %d batches, want ≥1", recomputed)
	}
	if corrected < 0 {
		return fmt.Errorf("reconcile corrected %d (negative)", corrected)
	}

	// --- Phase 3: assert convergence. ---
	var after model.Batch
	if err := mustDo(srv2, "GET", fmt.Sprintf("/api/batches/%s", batch.ID), nil, &after); err != nil {
		return err
	}
	if after.Status != before.Status {
		return fmt.Errorf("status after restart: got %s want %s", after.Status, before.Status)
	}
	if !approxEq(after.PeakTemp, before.PeakTemp, 1e-3, 1e-6) {
		return fmt.Errorf("peak temp after restart: got %.3f want %.3f", after.PeakTemp, before.PeakTemp)
	}
	if !approxEq(after.Conversion, before.Conversion, 1e-6, 1e-6) {
		return fmt.Errorf("conversion after restart: got %.6f want %.6f", after.Conversion, before.Conversion)
	}

	// Independently verify the recomputed result matches a fresh classify.
	want, err := thermal.Classify(rcp, rc)
	if err != nil {
		return err
	}
	var sr model.SafetyResult
	if err := mustDo(srv2, "GET", fmt.Sprintf("/api/batches/%s/result", batch.ID), nil, &sr); err != nil {
		return err
	}
	if sr.StoesselClass != want.StoesselClass {
		return fmt.Errorf("reconciled stoessel: got %d want %d", sr.StoesselClass, want.StoesselClass)
	}
	if !approxEq(sr.Conversion, want.Conversion, 1e-6, 1e-6) {
		return fmt.Errorf("reconciled conversion: got %.6f want %.6f", sr.Conversion, want.Conversion)
	}
	// Ensure the store can be used by the selfcheck directly.
	_ = st.DB()
	return nil
}

// httptest import guard if future refactors drop direct usage.
var _ = httptest.NewServer
