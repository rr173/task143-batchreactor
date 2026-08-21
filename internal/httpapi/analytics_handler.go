package httpapi

import (
	"net/http"

	"task143-batchreactor/internal/model"
)

// campaignAnalytics: GET /api/campaigns/{id}/analytics. The endpoint is a
// read-only operational snapshot: it combines lifecycle progress, forecast
// risk and reactor utilization without changing campaign state.
func (h *handlers) campaignAnalytics(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.CampaignAnalytics(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if out.CriticalBatches == nil {
		out.CriticalBatches = []string{}
	}
	if out.ReactorLoads == nil {
		out.ReactorLoads = []model.ReactorLoad{}
	}
	if out.Forecasts == nil {
		out.Forecasts = []model.BatchForecast{}
	}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	writeJSON(w, http.StatusOK, out)
}

// batchForecast: GET /api/batches/{id}/forecast. It is intentionally separate
// from the batch row so consumers can refresh a forecast frequently without
// needing to understand result-table availability or lifecycle timestamps.
func (h *handlers) batchForecast(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.BatchForecast(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if out.Reasons == nil {
		out.Reasons = []string{}
	}
	writeJSON(w, http.StatusOK, out)
}

// reactorCampaignLoad: GET /api/campaigns/{id}/reactors/{reactorID}/load.
// It exposes one reactor section of the same campaign report used by the
// aggregate endpoint, avoiding conflicting utilization calculations.
func (h *handlers) reactorCampaignLoad(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ReactorLoad(r.Context(), pathID(r), r.PathValue("reactorID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
