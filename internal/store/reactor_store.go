package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"task143-batchreactor/internal/model"
)

// --- Reactor ---

// CreateReactor inserts a reactor row.
func (s *Store) CreateReactor(ctx context.Context, tx *sql.Tx, r *model.Reactor) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO reactors(id,name,volume,heat_transfer_u,heat_transfer_area,max_operating_temp,max_pressure,material,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Name, r.Volume, r.HeatTransferU, r.HeatTransferArea, r.MaxOperatingTemp, r.MaxPressure, r.Material, string(r.Status), r.CreatedAt)
	if err != nil {
		return fmt.Errorf("create reactor: %w", err)
	}
	return nil
}

// ListReactors returns all reactors.
func (s *Store) ListReactors(ctx context.Context) ([]model.Reactor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,volume,heat_transfer_u,heat_transfer_area,max_operating_temp,max_pressure,material,status,created_at FROM reactors ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list reactors: %w", err)
	}
	defer rows.Close()
	var out []model.Reactor
	for rows.Next() {
		var r model.Reactor
		var st string
		if err := rows.Scan(&r.ID, &r.Name, &r.Volume, &r.HeatTransferU, &r.HeatTransferArea, &r.MaxOperatingTemp, &r.MaxPressure, &r.Material, &st, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Status = model.ReactorStatus(st)
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetReactor returns one reactor.
func (s *Store) GetReactor(ctx context.Context, id string) (*model.Reactor, error) {
	return s.getReactor(ctx, s.db, id)
}

// GetReactorTx returns one reactor inside a transaction.
func (s *Store) GetReactorTx(ctx context.Context, tx *sql.Tx, id string) (*model.Reactor, error) {
	return s.getReactor(ctx, tx, id)
}

func (s *Store) getReactor(ctx context.Context, q DBTX, id string) (*model.Reactor, error) {
	var r model.Reactor
	var st string
	err := q.QueryRowContext(ctx, `SELECT id,name,volume,heat_transfer_u,heat_transfer_area,max_operating_temp,max_pressure,material,status,created_at FROM reactors WHERE id=?`, id).
		Scan(&r.ID, &r.Name, &r.Volume, &r.HeatTransferU, &r.HeatTransferArea, &r.MaxOperatingTemp, &r.MaxPressure, &r.Material, &st, &r.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	r.Status = model.ReactorStatus(st)
	return &r, nil
}

// UpdateReactorStatus sets a reactor's operational status.
func (s *Store) UpdateReactorStatus(ctx context.Context, tx *sql.Tx, id string, status model.ReactorStatus) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `UPDATE reactors SET status=? WHERE id=?`, string(status), id)
	if err != nil {
		return fmt.Errorf("update reactor status: %w", err)
	}
	return nil
}
