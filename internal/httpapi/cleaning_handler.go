package httpapi

import (
	"net/http"

	"task143-batchreactor/internal/model"
)

// setCleaning: PUT /api/cleaning-matrix/{fromID}/{toID}
func (h *handlers) setCleaning(w http.ResponseWriter, r *http.Request) {
	from := r.PathValue("fromID")
	to := r.PathValue("toID")
	var body struct {
		Severity int `json:"severity"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.SetCleaning(r.Context(), from, to, model.CleaningSeverity(body.Severity)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"from_product": from, "to_product": to, "status": "set"})
}

// listCleaning: GET /api/cleaning-matrix
func (h *handlers) listCleaning(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListCleaning(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		out = []model.CleaningEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"matrix": out})
}
