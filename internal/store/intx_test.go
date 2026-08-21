package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"task143-batchreactor/internal/model"
)

// TestInTxRealClosure exercises the real *sql.Tx closure path: writes done via
// the tx variant inside InTx commit and are visible after.
func TestInTxRealClosure(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "real.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()

	if err := st.InTx(ctx, func(tx *sql.Tx) error {
		// Writes inside the closure must use the tx, never s.db, or they
		// deadlock under SetMaxOpenConns(1).
		r := &model.Reactor{ID: "r1", Name: "R", Volume: 1, HeatTransferU: 1, HeatTransferArea: 1, MaxOperatingTemp: 500, Status: model.ReactorAvailable, CreatedAt: 1}
		if err := st.CreateReactor(ctx, tx, r); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE reactors SET material=? WHERE id=?`, "SS", "r1"); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("InTx: %v", err)
	}
	got, err := st.GetReactor(ctx, "r1")
	if err != nil {
		t.Fatalf("get after tx: %v", err)
	}
	if got.Material != "SS" {
		t.Fatalf("material not committed: %q", got.Material)
	}
}

// TestInTxRollbackOnUserError asserts a failing closure rolls back.
func TestInTxRollbackOnUserError(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "rb.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()

	rerr := errors.New("boom")
	err = st.InTx(ctx, func(tx *sql.Tx) error {
		if err := st.CreateReactor(ctx, tx, &model.Reactor{ID: "r1", Name: "R", Volume: 1, HeatTransferU: 1, HeatTransferArea: 1, MaxOperatingTemp: 500, Status: model.ReactorAvailable, CreatedAt: 1}); err != nil {
			return err
		}
		return rerr
	})
	if !errors.Is(err, rerr) {
		t.Fatalf("expected user error propagated, got %v", err)
	}
	if _, err := st.GetReactor(ctx, "r1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rolled-back reactor must be absent: %v", err)
	}
}

// TestInTxRollbackOnPanic asserts a panicking closure rolls back and re-panics.
func TestInTxRollbackOnPanic(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "panic.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate")
		}
		got, gerr := st.GetReactor(ctx, "r1")
		if !errors.Is(gerr, ErrNotFound) {
			t.Fatalf("rolled-back reactor must be absent after panic: got=%v err=%v", got, gerr)
		}
	}()
	_ = st.InTx(ctx, func(tx *sql.Tx) error {
		_ = st.CreateReactor(ctx, tx, &model.Reactor{ID: "r1", Name: "R", Volume: 1, HeatTransferU: 1, HeatTransferArea: 1, MaxOperatingTemp: 500, Status: model.ReactorAvailable, CreatedAt: 1})
		panic("kaboom")
	})
}
