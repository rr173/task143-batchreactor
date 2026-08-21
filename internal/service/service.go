// Package service orchestrates the batch-reactor kinetics engine: it wires
// the domain packages (kinetics, thermal, reactor, campaign, lifecycle) to
// the SQLite store, enforces business invariants inside transactions, and
// re-derives computed state on restart. The HTTP layer and the selfcheck call
// into Services; it is the only writer of derived tables.
package service

import (
	"sync"

	"task143-batchreactor/internal/clock"
	"task143-batchreactor/internal/idlib"
	"task143-batchreactor/internal/store"
)

// Services is the bundle of business services the HTTP layer depends on. It
// owns the store, an injected clock and a per-reactor mutex that serializes
// lifecycle transitions on the same reactor so concurrent requests don't
// interleave writes to the derived tables.
type Services struct {
	st  *store.Store
	clk clock.Clock
	mu  muMap
}

type muMap struct {
	sync.Mutex
	m map[string]*sync.Mutex
}

// NewWithClock builds Services over a store with the given clock.
func NewWithClock(st *store.Store, clk clock.Clock) *Services {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Services{st: st, clk: clk, mu: muMap{m: make(map[string]*sync.Mutex)}}
}

// New builds Services with the real clock.
func New(st *store.Store) *Services { return NewWithClock(st, clock.Real{}) }

// Store exposes the store for the selfcheck (read-only usage).
func (s *Services) Store() *store.Store { return s.st }

// Clock exposes the clock for the selfcheck.
func (s *Services) Clock() clock.Clock { return s.clk }

// reactorLock returns the mutex guarding a reactor's batch writes.
func (s *Services) reactorLock(reactorID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	mu, ok := s.mu.m[reactorID]
	if !ok {
		mu = &sync.Mutex{}
		s.mu.m[reactorID] = mu
	}
	return mu
}

// now returns the current epoch seconds.
func (s *Services) now() int64 { return s.clk.Epoch() }

// newID generates a new opaque id.
func newID(prefix string) string { return idlib.New(prefix) }

// Reconcile returns the reconcile sub-service.
func (s *Services) Reconcile() *Reconciler { return &Reconciler{svc: s} }
