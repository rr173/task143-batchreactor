// Package clock abstracts wall-clock time so business logic and the selfcheck
// can use a deterministic fake clock. Epoch seconds are the single time unit
// used by the store and service layers.
package clock

import "time"

// Clock returns the current epoch second.
type Clock interface {
	Epoch() int64
}

// Real is the production clock backed by time.Now.
type Real struct{}

// Epoch returns the current Unix second.
func (Real) Epoch() int64 { return time.Now().Unix() }

// Fake is a deterministic clock advanced under test control. It is not safe
// for concurrent use without external synchronization; the selfcheck runs
// scenarios sequentially.
type Fake struct {
	t int64
}

// NewFake builds a fake clock anchored at the given epoch second.
func NewFake(epoch int64) *Fake { return &Fake{t: epoch} }

// Epoch returns the current fake time.
func (f *Fake) Epoch() int64 { return f.t }

// Advance moves the fake clock forward by the given number of seconds.
func (f *Fake) Advance(seconds int64) { f.t += seconds }

// Set snaps the fake clock to a specific epoch second.
func (f *Fake) Set(epoch int64) { f.t = epoch }
