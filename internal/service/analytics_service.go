package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"task143-batchreactor/internal/forecast"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/report"
	"task143-batchreactor/internal/store"
)

// CampaignAnalytics assembles the restart-safe operations report for one
// campaign. It uses only persisted inputs and the service clock; no report
// calculation may mutate a batch, which keeps reads safe while an operator is
// advancing a different reactor.
func (s *Services) CampaignAnalytics(ctx context.Context, campaignID string) (*model.CampaignAnalytics, error) {
	campaign, err := s.st.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	batches, err := s.st.ListBatchesByCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	recipes, reactors, results, warnings, err := s.analyticsInputs(ctx, batches)
	if err != nil {
		return nil, err
	}
	cleaning := s.cleaningSeconds(ctx, batches, recipes)
	forecasts := forecast.BuildAll(batches, recipes, reactors, results, cleaning, s.now())
	analytics := report.Build(report.Input{Campaign: *campaign, Batches: batches, Recipes: recipes, Reactors: reactors, Forecasts: forecasts, Cleaning: cleaning, Now: s.now()})
	analytics.Warnings = append(analytics.Warnings, warnings...)
	return &analytics, nil
}

// BatchForecast returns a current forecast for one batch. The service verifies
// the references first so the endpoint never silently projects a zero-value
// recipe or reactor when a campaign row has been corrupted.
func (s *Services) BatchForecast(ctx context.Context, batchID string) (*model.BatchForecast, error) {
	b, err := s.st.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	r, err := s.st.GetRecipe(ctx, b.RecipeID)
	if err != nil {
		return nil, fmt.Errorf("forecast recipe: %w", err)
	}
	rc, err := s.st.GetReactor(ctx, b.ReactorID)
	if err != nil {
		return nil, fmt.Errorf("forecast reactor: %w", err)
	}
	result, err := s.st.GetSafetyResult(ctx, b.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	// The cleaning changeover depends on the immediately preceding batch's
	// product on the same reactor, so load the whole reactor sequence to derive
	// it (consistent with how planning and the campaign report compute it).
	reactorBatches, err := s.st.ListBatchesByReactor(ctx, b.ReactorID)
	if err != nil {
		return nil, err
	}
	reactorRecipes := map[string]model.Recipe{b.RecipeID: *r}
	for _, rb := range reactorBatches {
		if _, ok := reactorRecipes[rb.RecipeID]; ok {
			continue
		}
		if rr, err := s.st.GetRecipe(ctx, rb.RecipeID); err == nil {
			reactorRecipes[rb.RecipeID] = *rr
		}
	}
	cleaning := s.cleaningSeconds(ctx, reactorBatches, reactorRecipes)
	returnPtr := forecast.Build(forecast.Input{Batch: *b, Recipe: *r, Reactor: *rc, Result: result, CleaningSeconds: cleaning[b.ID], Now: s.now()})
	return &returnPtr, nil
}

// ReactorLoad returns the report section for one vessel. It intentionally uses
// the campaign snapshot so load values have the same denominator and time span
// as the campaign summary instead of mixing a global reactor query with a
// campaign-local schedule.
func (s *Services) ReactorLoad(ctx context.Context, campaignID, reactorID string) (*model.ReactorLoad, error) {
	analytics, err := s.CampaignAnalytics(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	for i := range analytics.ReactorLoads {
		if analytics.ReactorLoads[i].ReactorID == reactorID {
			copy := analytics.ReactorLoads[i]
			return &copy, nil
		}
	}
	if _, err := s.st.GetReactor(ctx, reactorID); err != nil {
		return nil, err
	}
	return &model.ReactorLoad{ReactorID: reactorID}, nil
}

// analyticsInputs hydrates report dependencies once per unique id. Missing
// results are normal before reacting has completed; missing recipe/reactor
// references are retained as warnings and forecast.BuildAll will mark their
// affected batches critical instead of making the whole report unavailable.
func (s *Services) analyticsInputs(ctx context.Context, batches []model.Batch) (map[string]model.Recipe, map[string]model.Reactor, map[string]model.SafetyResult, []string, error) {
	recipes := make(map[string]model.Recipe)
	reactors := make(map[string]model.Reactor)
	results := make(map[string]model.SafetyResult)
	warnings := make([]string, 0)
	for _, b := range batches {
		if _, ok := recipes[b.RecipeID]; !ok {
			r, err := s.st.GetRecipe(ctx, b.RecipeID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					warnings = append(warnings, "missing recipe "+b.RecipeID+" for batch "+b.ID)
				} else {
					return nil, nil, nil, nil, err
				}
			} else {
				recipes[b.RecipeID] = *r
			}
		}
		if _, ok := reactors[b.ReactorID]; !ok {
			rc, err := s.st.GetReactor(ctx, b.ReactorID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					warnings = append(warnings, "missing reactor "+b.ReactorID+" for batch "+b.ID)
				} else {
					return nil, nil, nil, nil, err
				}
			} else {
				reactors[b.ReactorID] = *rc
			}
		}
		result, err := s.st.GetSafetyResult(ctx, b.ID)
		if err == nil {
			results[b.ID] = *result
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, nil, nil, nil, err
		}
	}
	return recipes, reactors, results, warnings, nil
}

// cleaningSeconds derives each batch's mandatory changeover wait (seconds) from
// the stored contamination matrix. It mirrors campaign.Plan's rule: within a
// reactor, ordered by seq, a batch whose product differs from the immediately
// preceding batch's product waits the severity-derived cleaning time; same
// product or the first batch on a reactor needs none. The result is keyed by
// batch id so forecast.BuildAll can attach the right wait to each forecast. A
// missing recipe is treated as no cleaning rather than dropping the batch.
func (s *Services) cleaningSeconds(ctx context.Context, batches []model.Batch, recipes map[string]model.Recipe) map[string]float64 {
	byReactor := map[string][]model.Batch{}
	for _, b := range batches {
		byReactor[b.ReactorID] = append(byReactor[b.ReactorID], b)
	}
	out := make(map[string]float64, len(batches))
	lookup := s.matrixLookup(ctx)
	for _, group := range byReactor {
		sort.Slice(group, func(i, j int) bool { return group[i].Seq < group[j].Seq })
		var prevProduct string
		for _, b := range group {
			cur, ok := recipes[b.RecipeID]
			if !ok || prevProduct == "" || prevProduct == cur.Product {
				out[b.ID] = 0
			} else {
				out[b.ID] = lookup(prevProduct, cur.Product).CleaningDuration()
			}
			if ok {
				prevProduct = cur.Product
			}
		}
	}
	return out
}
