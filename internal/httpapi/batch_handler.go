package httpapi

import (
	"net/http"

	"task143-batchreactor/internal/model"
)

// listBatches: GET /api/campaigns/{id}/batches
func (h *handlers) listBatches(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListBatchesByCampaign(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		out = []model.Batch{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": out})
}

// getBatch: GET /api/batches/{id}
func (h *handlers) getBatch(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetBatch(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// advanceRequest is the body of POST /api/batches/{id}/advance.
type advanceRequest struct {
	Target string `json:"target"`
}

// advanceBatch: POST /api/batches/{id}/advance
func (h *handlers) advanceBatch(w http.ResponseWriter, r *http.Request) {
	var body advanceRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	out, err := h.svc.AdvanceBatch(r.Context(), pathID(r), model.BatchStatus(body.Target))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// abortBatch: POST /api/batches/{id}/abort
func (h *handlers) abortBatch(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.AbortBatch(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// getBatchResult: GET /api/batches/{id}/result
func (h *handlers) getBatchResult(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetBatchResult(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// reactorTimeline: GET /api/reactors/{id}/timeline
func (h *handlers) reactorTimeline(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ReactorTimeline(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		out = []model.Batch{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": out})
}
