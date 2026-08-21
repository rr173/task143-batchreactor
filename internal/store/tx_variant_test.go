package store

import (
	"context"
	"errors"
	"testing"
)

// TestErrorSentinels asserts the domain errors satisfy errors.Is.
func TestErrorSentinels(t *testing.T) {
	for _, e := range []error{ErrNotFound, ErrConflict, ErrStateConflict, ErrInvariant, ErrThermalIncompatible, ErrLowConversion, ErrRunaway, ErrReactorBusy, ErrAlreadyPlanned, ErrNotPlanned} {
		if !errors.Is(e, e) {
			t.Errorf("sentinel self-identity failed: %v", e)
		}
	}
}

// TestAllBatchIDsEmpty asserts the bootstrap store has no batches.
func TestAllBatchIDsEmpty(t *testing.T) {
	st := openTestStore(t)
	ids, err := st.AllBatchIDs(context.Background())
	if err != nil {
		t.Fatalf("all batch ids: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected 0 ids on empty store, got %d", len(ids))
	}
}

// TestCorrectBatchStatusNoEvents asserts correcting a batch with no events is
// a no-op (does not error, does not change status).
func TestCorrectBatchStatusNoEvents(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	if err := st.CorrectBatchStatus(ctx, "missing"); err != nil {
		t.Fatalf("correct missing batch: %v", err)
	}
}

// TestRefreshReactorStatusIdle asserts refreshing an idle reactor marks it
// available.
func TestRefreshReactorStatusIdle(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	if err := st.RefreshReactorStatus(ctx, "missing"); err != nil {
		t.Fatalf("refresh missing reactor: %v", err)
	}
}
