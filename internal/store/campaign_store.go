package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task143-batchreactor/internal/model"
)

// --- Campaign & campaign items ---

// CreateCampaign inserts a campaign row.
func (s *Store) CreateCampaign(ctx context.Context, tx *sql.Tx, c *model.Campaign) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO campaigns(id,name,status,created_at) VALUES(?,?,?,?)`, c.ID, c.Name, string(c.Status), c.CreatedAt)
	if err != nil {
		return fmt.Errorf("create campaign: %w", err)
	}
	return nil
}

// ListCampaigns returns all campaigns.
func (s *Store) ListCampaigns(ctx context.Context) ([]model.Campaign, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,status,created_at FROM campaigns ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()
	var out []model.Campaign
	for rows.Next() {
		var c model.Campaign
		var st string
		if err := rows.Scan(&c.ID, &c.Name, &st, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Status = model.CampaignStatus(st)
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCampaign returns one campaign.
func (s *Store) GetCampaign(ctx context.Context, id string) (*model.Campaign, error) {
	var c model.Campaign
	var st string
	err := s.db.QueryRowContext(ctx, `SELECT id,name,status,created_at FROM campaigns WHERE id=?`, id).Scan(&c.ID, &c.Name, &st, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c.Status = model.CampaignStatus(st)
	return &c, nil
}

// UpdateCampaignStatus sets a campaign's status.
func (s *Store) UpdateCampaignStatus(ctx context.Context, tx *sql.Tx, id string, status model.CampaignStatus) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE campaigns SET status=? WHERE id=?`, string(status), id)
	if err != nil {
		return fmt.Errorf("update campaign status: %w", err)
	}
	return nil
}

// AddCampaignItem inserts one campaign item.
func (s *Store) AddCampaignItem(ctx context.Context, tx *sql.Tx, it *model.CampaignItem) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO campaign_items(campaign_id,seq,recipe_id,reactor_id,batch_count,status) VALUES(?,?,?,?,?,?)`,
		it.CampaignID, it.Seq, it.RecipeID, it.ReactorID, it.BatchCount, string(it.Status))
	if err != nil {
		return fmt.Errorf("add campaign item: %w", err)
	}
	return nil
}

// ListCampaignItems returns the items of a campaign in Seq order.
func (s *Store) ListCampaignItems(ctx context.Context, campaignID string) ([]model.CampaignItem, error) {
	return s.listCampaignItems(ctx, s.db, campaignID)
}

// ListCampaignItemsTx returns the items of a campaign inside a transaction.
func (s *Store) ListCampaignItemsTx(ctx context.Context, tx *sql.Tx, campaignID string) ([]model.CampaignItem, error) {
	return s.listCampaignItems(ctx, tx, campaignID)
}

func (s *Store) listCampaignItems(ctx context.Context, q DBTX, campaignID string) ([]model.CampaignItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT campaign_id,seq,recipe_id,reactor_id,batch_count,status FROM campaign_items WHERE campaign_id=? ORDER BY seq`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list campaign items: %w", err)
	}
	defer rows.Close()
	var out []model.CampaignItem
	for rows.Next() {
		var it model.CampaignItem
		var st string
		if err := rows.Scan(&it.CampaignID, &it.Seq, &it.RecipeID, &it.ReactorID, &it.BatchCount, &st); err != nil {
			return nil, err
		}
		it.Status = model.ItemStatus(st)
		out = append(out, it)
	}
	return out, rows.Err()
}

// UpdateCampaignItemStatus sets an item's status.
func (s *Store) UpdateCampaignItemStatus(ctx context.Context, tx *sql.Tx, campaignID string, seq int, status model.ItemStatus) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE campaign_items SET status=? WHERE campaign_id=? AND seq=?`, string(status), campaignID, seq)
	if err != nil {
		return fmt.Errorf("update campaign item status: %w", err)
	}
	return nil
}
