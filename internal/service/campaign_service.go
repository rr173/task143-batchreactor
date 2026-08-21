package service

import (
	"context"
	"database/sql"
	"fmt"

	"task143-batchreactor/internal/campaign"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/store"
)

// CreateCampaign validates and persists an empty campaign in draft.
func (s *Services) CreateCampaign(ctx context.Context, name string) (*model.Campaign, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: campaign name required", store.ErrInvariant)
	}
	c := &model.Campaign{ID: newID("cpg"), Name: name, Status: model.CampaignDraft, CreatedAt: s.now()}
	if err := s.st.CreateCampaign(ctx, nil, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ListCampaigns returns all campaigns.
func (s *Services) ListCampaigns(ctx context.Context) ([]model.Campaign, error) {
	return s.st.ListCampaigns(ctx)
}

// GetCampaign returns one campaign (without items).
func (s *Services) GetCampaign(ctx context.Context, id string) (*model.Campaign, error) {
	return s.st.GetCampaign(ctx, id)
}

// GetCampaignDetail returns a campaign with its items.
func (s *Services) GetCampaignDetail(ctx context.Context, id string) (*model.Campaign, []model.CampaignItem, error) {
	c, err := s.st.GetCampaign(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.st.ListCampaignItems(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return c, items, nil
}

// AddCampaignItem appends a (recipe,reactor,count) line to a draft campaign.
func (s *Services) AddCampaignItem(ctx context.Context, campaignID string, recipeID, reactorID string, batchCount int) (*model.CampaignItem, error) {
	if batchCount <= 0 {
		return nil, fmt.Errorf("%w: batch_count must be > 0", store.ErrInvariant)
	}
	c, err := s.st.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if c.Status != model.CampaignDraft {
		return nil, fmt.Errorf("%w: campaign not draft (status=%s)", store.ErrStateConflict, c.Status)
	}
	// Validate references exist.
	if _, err := s.st.GetRecipe(ctx, recipeID); err != nil {
		return nil, err
	}
	if _, err := s.st.GetReactor(ctx, reactorID); err != nil {
		return nil, err
	}
	items, err := s.st.ListCampaignItems(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	it := &model.CampaignItem{CampaignID: campaignID, Seq: len(items) + 1, RecipeID: recipeID, ReactorID: reactorID, BatchCount: batchCount, Status: model.ItemPending}
	if err := s.st.AddCampaignItem(ctx, nil, it); err != nil {
		return nil, err
	}
	return it, nil
}

// PlanCampaign resolves a draft campaign's items into a concrete schedule,
// applying the thermal-compatibility and sequence-dependent-cleaning
// constraints. On success it persists the planned batches (in queued) and
// marks the campaign planned. Any scheduling violation is returned in the
// plan's Errors and aborts persistence (the campaign stays draft).
func (s *Services) PlanCampaign(ctx context.Context, campaignID string) (*model.CampaignPlan, error) {
	c, err := s.st.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if c.Status != model.CampaignDraft {
		return nil, fmt.Errorf("%w: campaign not draft (status=%s)", store.ErrStateConflict, c.Status)
	}
	items, err := s.st.ListCampaignItems(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: campaign has no items", store.ErrInvariant)
	}

	// Hydrate recipes and reactors.
	recipes := map[string]model.Recipe{}
	reactors := map[string]model.Reactor{}
	for _, it := range items {
		if _, ok := recipes[it.RecipeID]; !ok {
			r, err := s.st.GetRecipe(ctx, it.RecipeID)
			if err != nil {
				return nil, err
			}
			recipes[it.RecipeID] = *r
		}
		if _, ok := reactors[it.ReactorID]; !ok {
			rc, err := s.st.GetReactor(ctx, it.ReactorID)
			if err != nil {
				return nil, err
			}
			reactors[it.ReactorID] = *rc
		}
	}

	plan, err := campaign.Plan(campaign.PlanInput{
		Items: items, Recipes: recipes, Reactors: reactors,
		Matrix: s.matrixLookup(ctx), StartAt: s.now(),
	})
	if err != nil {
		return nil, err
	}
	if len(plan.Errors) > 0 {
		// Surface the first violation as a typed error while still returning
		// the full plan so the caller can report all errors.
		return &plan, fmt.Errorf("%w: %s", store.ErrThermalIncompatible, plan.Errors[0].Reason)
	}

	// Persist planned batches and flip campaign to planned, atomically.
	if err := s.persistPlan(ctx, campaignID, plan); err != nil {
		return nil, err
	}
	if err := s.st.UpdateCampaignStatus(ctx, nil, campaignID, model.CampaignPlanned); err != nil {
		return nil, err
	}
	return &plan, nil
}

// persistPlan creates the queued batch rows for a plan and appends a queued
// event per batch, inside a single transaction.
func (s *Services) persistPlan(ctx context.Context, campaignID string, plan model.CampaignPlan) error {
	// Find each item's seq by matching plan items to plan batches.
	return s.st.InTx(ctx, func(tx *sql.Tx) error {
		for _, pb := range plan.Batches {
			itemSeq := itemSeqForBatch(plan.Items, pb)
			b := &model.Batch{
				ID: newID("btch"), CampaignID: campaignID, CampaignItemSeq: itemSeq,
				ReactorID: pb.ReactorID, RecipeID: pb.RecipeID, Seq: pb.Seq,
				Status: model.BatchQueued, PlannedStart: pb.PlannedStart,
			}
			if err := s.st.CreateBatch(ctx, tx, b); err != nil {
				return err
			}
			if err := s.st.AppendEvent(ctx, tx, b.ID, "queued", b.PlannedStart, ""); err != nil {
				return err
			}
		}
		return nil
	})
}

// itemSeqForBatch finds the campaign item seq that produced a planned batch.
func itemSeqForBatch(items []model.PlannedItem, pb model.PlannedBatch) int {
	for _, it := range items {
		if it.RecipeID == pb.RecipeID && it.ReactorID == pb.ReactorID {
			return it.Seq
		}
	}
	return 0
}

// StartCampaign flips a planned campaign to running. Batches are already
// queued from PlanCampaign; starting just changes the campaign status so the
// operator can begin advancing batches.
func (s *Services) StartCampaign(ctx context.Context, campaignID string) (*model.Campaign, error) {
	c, err := s.st.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if c.Status != model.CampaignPlanned {
		return nil, fmt.Errorf("%w: campaign must be planned before start (status=%s)", store.ErrStateConflict, c.Status)
	}
	if err := s.st.UpdateCampaignStatus(ctx, nil, campaignID, model.CampaignRunning); err != nil {
		return nil, err
	}
	return s.st.GetCampaign(ctx, campaignID)
}
