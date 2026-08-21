package store

import (
	"context"
	"database/sql"
	"fmt"
)

// --- Batch event log ---

// AppendEvent appends one lifecycle event.
func (s *Store) AppendEvent(ctx context.Context, tx *sql.Tx, batchID string, eventType string, at int64, payload string) error {
	var q DBTX = s.db
	if tx != nil {
		q = tx
	}
	_, err := q.ExecContext(ctx, `INSERT INTO batch_events(batch_id,event_type,at,payload) VALUES(?,?,?,?)`, batchID, eventType, at, payload)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

// ListEvents returns the events of a batch in time order.
func (s *Store) ListEvents(ctx context.Context, batchID string) ([]int64, []string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,event_type FROM batch_events WHERE batch_id=? ORDER BY id`, batchID)
	if err != nil {
		return nil, nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	var ids []int64
	var types []string
	for rows.Next() {
		var id int64
		var et string
		if err := rows.Scan(&id, &et); err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
		types = append(types, et)
	}
	return ids, types, rows.Err()
}

// LatestEventType returns the most recent event_type for a batch, used by the
// reconcile path to correct a batch's status from the authoritative event log.
func (s *Store) LatestEventType(ctx context.Context, tx *sql.Tx, batchID string) (string, error) {
	q := DBTX(s.db)
	if tx != nil {
		q = tx
	}
	var et string
	err := q.QueryRowContext(ctx, `SELECT event_type FROM batch_events WHERE batch_id=? ORDER BY id DESC LIMIT 1`, batchID).Scan(&et)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return et, nil
}

// AllBatchIDs returns every batch id, used by the restart reconcile pass.
func (s *Store) AllBatchIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM batches`)
	if err != nil {
		return nil, fmt.Errorf("all batch ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
