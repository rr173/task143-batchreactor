package selfcheck

import (
	"fmt"
	"net/http/httptest"

	"task143-batchreactor/internal/model"
)

// createReactor posts a reactor and returns its id.
func createReactor(srv *httptest.Server, rc model.Reactor) (string, error) {
	var out model.Reactor
	if err := mustDo(srv, "POST", "/api/reactors", rc, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// createRecipe posts a recipe and returns its id.
func createRecipe(srv *httptest.Server, r model.Recipe) (string, error) {
	var out model.Recipe
	if err := mustDo(srv, "POST", "/api/recipes", r, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// createCampaign posts a campaign and returns its id.
func createCampaign(srv *httptest.Server, name string) (string, error) {
	var out model.Campaign
	if err := mustDo(srv, "POST", "/api/campaigns", map[string]any{"name": name}, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// advanceBatch advances a batch to the target status and returns the updated
// batch. It fails the scenario on a non-2xx response.
func advanceBatch(srv *httptest.Server, id string, target model.BatchStatus) (model.Batch, error) {
	var out model.Batch
	if err := mustDo(srv, "POST", fmt.Sprintf("/api/batches/%s/advance", id),
		map[string]any{"target": string(target)}, &out); err != nil {
		return model.Batch{}, err
	}
	return out, nil
}

// firstBatchOf returns the first batch of a campaign (lowest reactor seq).
func firstBatchOf(srv *httptest.Server, campaignID string) (model.Batch, error) {
	var resp struct {
		Batches []model.Batch `json:"batches"`
	}
	if err := mustDo(srv, "GET", fmt.Sprintf("/api/campaigns/%s/batches", campaignID), nil, &resp); err != nil {
		return model.Batch{}, err
	}
	if len(resp.Batches) == 0 {
		return model.Batch{}, fmt.Errorf("campaign %s has no batches", campaignID)
	}
	return resp.Batches[0], nil
}
