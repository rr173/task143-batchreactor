package httpapi

import (
	"net/http"

	"task143-batchreactor/internal/model"
)

// createCampaign: POST /api/campaigns
func (h *handlers) createCampaign(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	out, err := h.svc.CreateCampaign(r.Context(), body.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// listCampaigns: GET /api/campaigns
func (h *handlers) listCampaigns(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListCampaigns(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		out = []model.Campaign{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"campaigns": out})
}

// getCampaign: GET /api/campaigns/{id}
func (h *handlers) getCampaign(w http.ResponseWriter, r *http.Request) {
	c, items, err := h.svc.GetCampaignDetail(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []model.CampaignItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"campaign": c, "items": items})
}

// addCampaignItem: POST /api/campaigns/{id}/items
func (h *handlers) addCampaignItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RecipeID  string `json:"recipe_id"`
		ReactorID string `json:"reactor_id"`
		BatchCount int   `json:"batch_count"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	out, err := h.svc.AddCampaignItem(r.Context(), pathID(r), body.RecipeID, body.ReactorID, body.BatchCount)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// planCampaign: POST /api/campaigns/{id}/plan
func (h *handlers) planCampaign(w http.ResponseWriter, r *http.Request) {
	plan, err := h.svc.PlanCampaign(r.Context(), pathID(r))
	if err != nil {
		// Even on a scheduling violation we return the plan with its errors so
		// the caller can see every problem; the status reflects the rejection.
		if plan != nil {
			writeJSON(w, http.StatusUnprocessableEntity, plan)
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// startCampaign: POST /api/campaigns/{id}/start
func (h *handlers) startCampaign(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.StartCampaign(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// fullReport: GET /api/campaigns/{id}/full-report
func (h *handlers) fullReport(w http.ResponseWriter, r *http.Request) {
	c, items, batches, err := h.svc.CampaignFullReport(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []model.CampaignItem{}
	}
	if batches == nil {
		batches = []model.Batch{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaign": c, "items": items, "batches": batches,
	})
}
