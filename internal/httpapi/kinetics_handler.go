package httpapi

import (
	"net/http"
)

// simulateRequest is the body of POST /api/kinetics/simulate.
type simulateRequest struct {
	RecipeID  string `json:"recipe_id"`
	ReactorID string `json:"reactor_id"`
}

// simulate: POST /api/kinetics/simulate — dry-run kinetics classification.
func (h *handlers) simulate(w http.ResponseWriter, r *http.Request) {
	var req simulateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	out, err := h.svc.Simulate(r.Context(), req.RecipeID, req.ReactorID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
