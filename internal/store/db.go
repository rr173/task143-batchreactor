// Package store is the SQLite persistence layer for the batch-reactor kinetics
// engine. It owns the schema, the connection and the transaction primitives;
// the service layer layers business rules on top.
//
// All write operations run inside a caller-begun IMMEDIATE transaction; Open
// sets _txlock=immediate so every BEGIN is BEGIN IMMEDIATE, serializing
// writers. A single pooled connection (SetMaxOpenConns(1)) avoids "database is
// locked" from interleaved write transactions on extra connections. With a
// single pooled connection, any read or write that happens inside an InTx
// transaction MUST use the *sql.Tx (via the DBTX switch), never s.db —
// reaching for s.db while the tx holds the only connection deadlocks.
//
// Derived figures (kinetics results, safety verdicts) are recomputable from
// the stored authoritative inputs (recipe, reactor, batch config); ReconcileAll
// re-runs them so a crashed-and-restarted process converges to the same state.
package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; CGO_ENABLED=0 compatible
)

// Store wraps a *sql.DB connection to the batchreactor database.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite file at path, applies the schema and tunes
// pragmatic options for durability and concurrency.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_txlock=immediate",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DBTX is the union of *sql.DB and *sql.Tx query/exec methods. Every store
// method that may run inside an InTx transaction takes a tx and switches
// between it and s.db. With SetMaxOpenConns(1) a method that reaches for s.db
// while a transaction holds the single connection deadlocks — pass the *sql.Tx
// instead.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// BeginTx starts an IMMEDIATE transaction. Callers must Commit or Rollback.
func (s *Store) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}

// InTx runs fn inside a single IMMEDIATE transaction. On error the transaction
// is rolled back; on success it is committed. This is the write path used by
// every mutating service method so business invariants are enforced atomically.
func (s *Store) InTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Ping verifies the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// DB exposes the underlying *sql.DB for the selfcheck (read-only usage).
func (s *Store) DB() *sql.DB { return s.db }
