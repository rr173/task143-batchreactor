// Package reactor holds domain rules that are local to a single reactor: the
// thermal-compatibility check used at campaign-planning time and the occupancy
// check that prevents two batches from sharing a reactor simultaneously.
//
// The heavy kinetics/thermal math lives in the kinetics and thermal packages;
// this package composes those results with the operational rules.
package reactor

import (
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/thermal"
)

// CompatibleWith reports whether a recipe can be assigned to a reactor. It
// wraps thermal.IsThermallyCompatible so callers don't import both packages.
func CompatibleWith(r model.Recipe, rc model.Reactor) bool {
	return thermal.IsThermallyCompatible(r, rc)
}

// IncompatibilityReason returns a human-readable explanation when a recipe is
// not thermally compatible with a reactor, or "" when compatible.
func IncompatibilityReason(r model.Recipe, rc model.Reactor) string {
	if CompatibleWith(r, rc) {
		return ""
	}
	return "MTSR exceeds reactor max operating temperature"
}

// Occupancy is the lightweight view of a reactor's current batch occupancy.
type Occupancy struct {
	ReactorID  string
	ActiveBatch string
	Busy       bool
}

// Free reports whether the reactor has no active (non-terminal) batch.
func (o Occupancy) Free() bool { return !o.Busy }
