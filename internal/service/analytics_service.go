package service

import (
	"context"
	"errors"
	"fmt"

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
	forecasts := forecast.BuildAll(batches, recipes, reactors, results, s.now())
	analytics := report.Build(report.Input{Campaign: *campaign, Batches: batches, Recipes: recipes, Reactors: reactors, Forecasts: forecasts, Now: s.now()})
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
	returnPtr := forecast.Build(forecast.Input{Batch: *b, Recipe: *r, Reactor: *rc, Result: result, Now: s.now()})
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
