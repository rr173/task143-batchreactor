package httpapi

import (
	"net/http"

	"task143-batchreactor/internal/model"
)

// createReactor: POST /api/reactors
func (h *handlers) createReactor(w http.ResponseWriter, r *http.Request) {
	var rc model.Reactor
	if !decodeJSON(w, r, &rc) {
		return
	}
	out, err := h.svc.CreateReactor(r.Context(), &rc)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// listReactors: GET /api/reactors
func (h *handlers) listReactors(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListReactors(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		out = []model.Reactor{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"reactors": out})
}

// getReactor: GET /api/reactors/{id}
func (h *handlers) getReactor(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetReactor(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// updateReactor: PATCH /api/reactors/{id}
func (h *handlers) updateReactor(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	out, err := h.svc.UpdateReactorStatus(r.Context(), pathID(r), model.ReactorStatus(body.Status))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
