package service

import (
	"context"
	"fmt"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/store"
	"task143-batchreactor/internal/thermal"
)

// SetCleaning sets or replaces a cross-contamination cell.
func (s *Services) SetCleaning(ctx context.Context, fromProduct, toProduct string, sev model.CleaningSeverity) error {
	if fromProduct == "" || toProduct == "" {
		return fmt.Errorf("%w: product names required", store.ErrInvariant)
	}
	if sev < model.CleaningNone || sev > model.CleaningHeavy {
		return fmt.Errorf("%w: severity out of range", store.ErrInvariant)
	}
	return s.st.SetCleaning(ctx, nil, fromProduct, toProduct, sev)
}

// ListCleaning returns the whole matrix.
func (s *Services) ListCleaning(ctx context.Context) ([]model.CleaningEntry, error) {
	return s.st.ListCleaning(ctx)
}

// matrixLookup builds a campaign.MatrixLookup backed by the store.
func (s *Services) matrixLookup(ctx context.Context) func(string, string) model.CleaningSeverity {
	return func(from, to string) model.CleaningSeverity {
		sev, err := s.st.CleaningSeverity(ctx, from, to)
		if err != nil {
			return model.CleaningNone
		}
		return sev
	}
}

// Simulate runs the kinetics + thermal classification for a recipe+reactor
// without persisting anything. It is the dry-run used by the simulate endpoint
// and (persisted) by the batch reacting stage.
func (s *Services) Simulate(ctx context.Context, recipeID, reactorID string) (*model.KineticsResult, error) {
	r, err := s.st.GetRecipe(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	rc, err := s.st.GetReactor(ctx, reactorID)
	if err != nil {
		return nil, err
	}
	res, err := thermal.Classify(*r, *rc)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// RecipeSafety returns the thermal-safety summary for a recipe (Stoessel class,
// ΔT_ad, MTSR, TMR) independent of a specific reactor — useful for the safety
// endpoint that screens a recipe before assignment.
func (s *Services) RecipeSafety(ctx context.Context, recipeID string) (*model.KineticsResult, error) {
	r, err := s.st.GetRecipe(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	// Use a nominal reactor for the trajectory (the classification is recipe-
	// dominant; the reactor only contributes heat removal).
	rc := model.Reactor{Volume: r.Rho * 1, HeatTransferU: 0, HeatTransferArea: 0}
	res, err := thermal.Classify(*r, rc)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
