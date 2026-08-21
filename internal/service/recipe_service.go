package service

import (
	"context"
	"fmt"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/store"
)

// CreateRecipe validates and persists a recipe. Physical sanity checks keep the
// kinetics integrator from dividing by zero or producing NaNs.
func (s *Services) CreateRecipe(ctx context.Context, r *model.Recipe) (*model.Recipe, error) {
	if r.Name == "" || r.Product == "" {
		return nil, fmt.Errorf("%w: recipe name and product required", store.ErrInvariant)
	}
	if r.K0 <= 0 || r.Ea < 0 {
		return nil, fmt.Errorf("%w: k0 must be > 0 and ea >= 0", store.ErrInvariant)
	}
	if r.Order != model.OrderFirst && r.Order != model.OrderSecond {
		return nil, fmt.Errorf("%w: reaction order must be 1 or 2", store.ErrInvariant)
	}
	if r.CA0 <= 0 || r.Rho <= 0 || r.Cp <= 0 {
		return nil, fmt.Errorf("%w: ca0, rho, cp must be > 0", store.ErrInvariant)
	}
	if r.T0 <= 0 || r.Duration <= 0 {
		return nil, fmt.Errorf("%w: t0 and duration must be > 0", store.ErrInvariant)
	}
	if r.MinConversion < 0 || r.MinConversion > 1 {
		return nil, fmt.Errorf("%w: min_conversion must be in [0,1]", store.ErrInvariant)
	}
	r.ID = newID("rcp")
	r.CreatedAt = s.now()
	if err := s.st.CreateRecipe(ctx, nil, r); err != nil {
		return nil, err
	}
	return r, nil
}

// ListRecipes returns all recipes.
func (s *Services) ListRecipes(ctx context.Context) ([]model.Recipe, error) {
	return s.st.ListRecipes(ctx)
}

// GetRecipe returns one recipe.
func (s *Services) GetRecipe(ctx context.Context, id string) (*model.Recipe, error) {
	return s.st.GetRecipe(ctx, id)
}
