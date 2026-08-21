package httpapi

import (
	"net/http"

	"task143-batchreactor/internal/model"
)

// createRecipe: POST /api/recipes
func (h *handlers) createRecipe(w http.ResponseWriter, r *http.Request) {
	var rc model.Recipe
	if !decodeJSON(w, r, &rc) {
		return
	}
	out, err := h.svc.CreateRecipe(r.Context(), &rc)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// listRecipes: GET /api/recipes
func (h *handlers) listRecipes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListRecipes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if out == nil {
		out = []model.Recipe{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"recipes": out})
}

// getRecipe: GET /api/recipes/{id}
func (h *handlers) getRecipe(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetRecipe(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// recipeSafety: GET /api/recipes/{id}/safety
func (h *handlers) recipeSafety(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.RecipeSafety(r.Context(), pathID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
