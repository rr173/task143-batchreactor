package service

import (
	"context"
	"fmt"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/store"
)

// CreateReactor validates and persists a reactor.
func (s *Services) CreateReactor(ctx context.Context, r *model.Reactor) (*model.Reactor, error) {
	if r.Name == "" {
		return nil, fmt.Errorf("%w: reactor name required", store.ErrInvariant)
	}
	if r.Volume <= 0 || r.HeatTransferArea <= 0 {
		return nil, fmt.Errorf("%w: volume and area must be > 0", store.ErrInvariant)
	}
	r.ID = newID("rct")
	r.Status = model.ReactorAvailable
	r.CreatedAt = s.now()
	if err := s.st.CreateReactor(ctx, nil, r); err != nil {
		return nil, err
	}
	return r, nil
}

// ListReactors returns all reactors.
func (s *Services) ListReactors(ctx context.Context) ([]model.Reactor, error) {
	return s.st.ListReactors(ctx)
}

// GetReactor returns one reactor.
func (s *Services) GetReactor(ctx context.Context, id string) (*model.Reactor, error) {
	return s.st.GetReactor(ctx, id)
}

// UpdateReactorStatus sets a reactor's status, only allowing the maintenance
// toggle (available↔maintenance) directly; busy is derived by the lifecycle.
func (s *Services) UpdateReactorStatus(ctx context.Context, id string, status model.ReactorStatus) (*model.Reactor, error) {
	if status != model.ReactorAvailable && status != model.ReactorMaintenance {
		return nil, fmt.Errorf("%w: only available/maintenance can be set directly", store.ErrInvariant)
	}
	r, err := s.st.GetReactor(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.Status == model.ReactorBusy && status == model.ReactorAvailable {
		return nil, fmt.Errorf("%w: cannot free a busy reactor directly", store.ErrReactorBusy)
	}
	if err := s.st.UpdateReactorStatus(ctx, nil, id, status); err != nil {
		return nil, err
	}
	return s.st.GetReactor(ctx, id)
}
