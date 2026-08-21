// Package httpapi wires the batch-reactor kinetics services to HTTP routes.
// It owns the mux, the request/response JSON shape and the error → status
// mapping. The self-check smoke test and the production binary share the same
// mux (via NewMux) so a single code path is exercised end-to-end.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"task143-batchreactor/internal/lifecycle"
	"task143-batchreactor/internal/service"
	"task143-batchreactor/internal/store"
)

// Version is the API version, surfaced in the health check.
const Version = "v1.0.0"

// Services is the bundle of business services the handlers depend on.
type Services struct {
	Svc *service.Services
}

// NewMux builds the HTTP mux from services and an embedded frontend filesystem.
func NewMux(svc Services, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()
	h := &handlers{svc: svc.Svc}

	// Reactors.
	mux.HandleFunc("POST /api/reactors", h.createReactor)
	mux.HandleFunc("GET /api/reactors", h.listReactors)
	mux.HandleFunc("GET /api/reactors/{id}", h.getReactor)
	mux.HandleFunc("PATCH /api/reactors/{id}", h.updateReactor)

	// Recipes.
	mux.HandleFunc("POST /api/recipes", h.createRecipe)
	mux.HandleFunc("GET /api/recipes", h.listRecipes)
	mux.HandleFunc("GET /api/recipes/{id}", h.getRecipe)

	// Cleaning matrix.
	mux.HandleFunc("PUT /api/cleaning-matrix/{fromID}/{toID}", h.setCleaning)
	mux.HandleFunc("GET /api/cleaning-matrix", h.listCleaning)

	// Kinetics.
	mux.HandleFunc("POST /api/kinetics/simulate", h.simulate)
	mux.HandleFunc("GET /api/recipes/{id}/safety", h.recipeSafety)

	// Campaigns.
	mux.HandleFunc("POST /api/campaigns", h.createCampaign)
	mux.HandleFunc("GET /api/campaigns", h.listCampaigns)
	mux.HandleFunc("GET /api/campaigns/{id}", h.getCampaign)
	mux.HandleFunc("POST /api/campaigns/{id}/items", h.addCampaignItem)
	mux.HandleFunc("POST /api/campaigns/{id}/plan", h.planCampaign)
	mux.HandleFunc("POST /api/campaigns/{id}/start", h.startCampaign)
	mux.HandleFunc("GET /api/campaigns/{id}/analytics", h.campaignAnalytics)
	mux.HandleFunc("GET /api/campaigns/{id}/reactors/{reactorID}/load", h.reactorCampaignLoad)

	// Batches.
	mux.HandleFunc("GET /api/campaigns/{id}/batches", h.listBatches)
	mux.HandleFunc("GET /api/batches/{id}", h.getBatch)
	mux.HandleFunc("POST /api/batches/{id}/advance", h.advanceBatch)
	mux.HandleFunc("POST /api/batches/{id}/abort", h.abortBatch)
	mux.HandleFunc("GET /api/batches/{id}/result", h.getBatchResult)
	mux.HandleFunc("GET /api/batches/{id}/forecast", h.batchForecast)

	// Reports.
	mux.HandleFunc("GET /api/reactors/{id}/timeline", h.reactorTimeline)
	mux.HandleFunc("GET /api/campaigns/{id}/full-report", h.fullReport)
	mux.HandleFunc("GET /api/health", health)

	// Frontend.
	if frontend != nil {
		mux.Handle("GET /", http.FileServer(http.FS(frontend)))
	}
	return mux
}

// handlers holds the service reference for the routes.
type handlers struct {
	svc *service.Services
}

// health is the liveness probe.
func health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
}

// writeJSON serializes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError maps a domain error to an HTTP status and writes a JSON body.
func writeError(w http.ResponseWriter, err error) {
	status := errorStatus(err)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// decodeJSON reads a JSON body into v. Returns 400 on a decode error. An empty
// body (EOF) is allowed and leaves v untouched.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		return true
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

// errorStatus maps a store/service error to an HTTP status code.
func errorStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrConflict),
		errors.Is(err, store.ErrStateConflict),
		errors.Is(err, lifecycle.ErrIllegalTransition):
		return http.StatusConflict
	case errors.Is(err, store.ErrThermalIncompatible),
		errors.Is(err, store.ErrLowConversion),
		errors.Is(err, store.ErrRunaway),
		errors.Is(err, store.ErrReactorBusy),
		errors.Is(err, store.ErrAlreadyPlanned),
		errors.Is(err, store.ErrNotPlanned),
		errors.Is(err, store.ErrInvariant):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// pathID extracts the {id} path variable from the request.
func pathID(r *http.Request) string { return r.PathValue("id") }

// authBearer is unused but kept for parity with sibling tasks.
func authBearer(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
