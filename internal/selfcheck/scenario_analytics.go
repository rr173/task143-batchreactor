package selfcheck

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"task143-batchreactor/internal/clock"
	"task143-batchreactor/internal/model"
)

// smokeAnalytics proves that the report API uses the same live campaign state
// as planning and lifecycle. It deliberately reads a queued batch, then moves
// it into charging and confirms both the per-batch and aggregate reports see
// the new operational phase without a process restart.
func smokeAnalytics(srv *httptest.Server, clk *clock.Fake) error {
	rid, err := createReactor(srv, safeReactor())
	if err != nil {
		return err
	}
	fid, err := createRecipe(srv, exothermicRecipe())
	if err != nil {
		return err
	}
	cid, err := createCampaign(srv, "analytics-campaign")
	if err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/items", cid), map[string]any{"recipe_id": fid, "reactor_id": rid, "batch_count": 1}, nil); err != nil {
		return err
	}
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/campaigns/%s/plan", cid), nil, nil); err != nil {
		return err
	}
	batch, err := firstBatchOf(srv, cid)
	if err != nil {
		return err
	}
	var before model.BatchForecast
	if err := mustDo(srv, "GET", fmt.Sprintf("/api/batches/%s/forecast", batch.ID), nil, &before); err != nil {
		return err
	}
	if before.Phase != model.ForecastWaiting {
		return fmt.Errorf("queued forecast phase = %s", before.Phase)
	}
	if _, err := advanceBatch(srv, batch.ID, model.BatchCharging); err != nil {
		return err
	}
	var report model.CampaignAnalytics
	if err := mustDo(srv, "GET", fmt.Sprintf("/api/campaigns/%s/analytics", cid), nil, &report); err != nil {
		return err
	}
	if report.BatchCount != 1 || report.ActiveCount != 1 || len(report.Forecasts) != 1 {
		return fmt.Errorf("analytics counts = batches:%d active:%d forecasts:%d", report.BatchCount, report.ActiveCount, len(report.Forecasts))
	}
	if report.Forecasts[0].Phase != model.ForecastCharge {
		return fmt.Errorf("charging forecast phase = %s", report.Forecasts[0].Phase)
	}
	var load model.ReactorLoad
	if err := mustDo(srv, "GET", fmt.Sprintf("/api/campaigns/%s/reactors/%s/load", cid, rid), nil, &load); err != nil {
		return err
	}
	if load.ActiveCount != 1 || load.BatchCount != 1 {
		return fmt.Errorf("reactor load = active:%d batches:%d", load.ActiveCount, load.BatchCount)
	}
	if err := expectCode(srv, "GET", "/api/campaigns/missing/analytics", nil, http.StatusNotFound); err != nil {
		return err
	}
	return nil
}
