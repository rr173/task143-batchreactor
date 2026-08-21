// Package units holds physical constants and small numeric helpers shared by
// the kinetics and thermal packages. Centralizing R and the rounding policy
// keeps the locked interpretation in one place.
package units

import "math"

// R is the universal gas constant, J/(mol·K).
const R = 8.314

// DefaultDt is the default RK4 integration step in seconds when a caller does
// not specify one.
const DefaultDt = 2.0

// KelvinZeroC is 0°C in Kelvin.
const KelvinZeroC = 273.15

// RoundHalfUp rounds v to the given number of decimal places using half-up
// rounding (the schoolbook rule). It avoids the banker's-rounding surprise of
// math.Round at .5 boundaries by adding half an ulp before truncation.
func RoundHalfUp(v float64, places int) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	pow := math.Pow(10, float64(places))
	scaled := v * pow
	if scaled < 0 {
		return math.Ceil(scaled-0.5) / pow
	}
	return math.Floor(scaled+0.5) / pow
}

// Clamp restricts v to [lo, hi].
func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ApproxEq reports whether two floats are equal within an absolute and relative
// tolerance; used by tests comparing derived kinetics figures.
func ApproxEq(a, b, absTol, relTol float64) bool {
	if a == b {
		return true
	}
	diff := math.Abs(a - b)
	if diff <= absTol {
		return true
	}
	denom := math.Max(math.Abs(a), math.Abs(b))
	if denom == 0 {
		return false
	}
	return diff/denom <= relTol
}
