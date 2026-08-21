// Package thermal turns a kinetics trajectory into the thermal-safety
// classification used by the engine's boundary checks. It computes the
// adiabatic temperature rise, the Stoessel criticality class, the
// Frank-Kamenetskii time-to-maximum-rate (TMR) and the final safety verdict,
// all as pure functions of the stored recipe and reactor inputs so they are
// recomputable on restart.
package thermal

import (
	"math"

	"task143-batchreactor/internal/kinetics"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/units"
)

// DeltaTAd is the adiabatic temperature rise, ΔT_ad = (-ΔH_rx)·C_A0 / (ρ·Cp), K.
func DeltaTAd(r model.Recipe) float64 {
	if r.Rho <= 0 || r.Cp <= 0 {
		return 0
	}
	return (-r.DeltaHrx) * r.CA0 / (r.Rho * r.Cp)
}

// MTSR is the maximum temperature of the synthesis reaction under adiabatic
// full conversion, MTSR = T0 + ΔT_ad, K.
func MTSR(r model.Recipe) float64 {
	return r.T0 + DeltaTAd(r)
}

// StoesselClass returns the criticality class from ΔT_ad (K):
//
//	ΔT_ad < 10            → 1
//	10 ≤ ΔT_ad < 50       → 2
//	50 ≤ ΔT_ad < 150      → 3
//	150 ≤ ΔT_ad < 200     → 4
//	ΔT_ad ≥ 200           → 5
func StoesselClass(deltaTad float64) int {
	switch {
	case deltaTad < 10:
		return 1
	case deltaTad < 50:
		return 2
	case deltaTad < 150:
		return 3
	case deltaTad < 200:
		return 4
	default:
		return 5
	}
}

// TMR returns the Frank-Kamenetskii time-to-maximum-rate (seconds) at the given
// temperature, using the zero-order approximation
//
//	TMR(T) = ρ·Cp·R·T² / ((-ΔH_rx)·k(T)·C_A0^Order·Ea)
//
// where k(T)=K0·exp(-Ea/(R·T)). An endotherm (ΔH_rx≥0), zero Ea or a non-physical
// input yields +Inf (no runaway concern). TMR is an informational hazard
// metric; the runaway verdict is decided from ΔT_ad class and the trajectory's
// peak temperature against the recipe's thermal limit (see Verdict).
func TMR(r model.Recipe, tKelvin float64) float64 {
	if r.DeltaHrx >= 0 || r.Ea <= 0 || r.Rho <= 0 || r.Cp <= 0 || r.CA0 <= 0 || tKelvin <= 0 {
		return math.Inf(1)
	}
	k := kinetics.ArrheniusRate(r.K0, r.Ea, tKelvin)
	var rate float64
	switch r.Order {
	case model.OrderSecond:
		rate = k * r.CA0 * r.CA0
	default:
		rate = k * r.CA0
	}
	if rate <= 0 {
		return math.Inf(1)
	}
	return (r.Rho * r.Cp * units.R * tKelvin * tKelvin) / ((-r.DeltaHrx) * rate * r.Ea)
}

// Classify produces the full thermal-safety result for a recipe+reactor by
// running the kinetics integration and then applying the Stoessel +
// thermal-limit rules. It is the single authoritative computation used by the
// simulate endpoint, the batch reacting stage and the restart reconcile.
func Classify(r model.Recipe, rc model.Reactor) (model.KineticsResult, error) {
	dtAd := DeltaTAd(r)
	mtsr := MTSR(r)
	class := StoesselClass(dtAd)
	tmr := TMR(r, r.T0)

	out, err := kinetics.Integrate(r, rc, r.Duration, units.DefaultDt*2)
	if err != nil {
		return model.KineticsResult{}, err
	}

	verdict := Verdict(r, out.PeakTemp, class)

	return model.KineticsResult{
		Conversion:    out.Conversion,
		PeakTemp:      out.PeakTemp,
		Duration:      out.Duration,
		QGenMax:       out.QGenMax,
		QRemAtPeak:    out.QRemAtPeak,
		DeltaTad:      dtAd,
		MTSR:          mtsr,
		StoesselClass: class,
		TMRSeconds:    tmr,
		Verdict:       verdict,
		Steps:         out.Steps,
		Isothermal:    out.Isothermal,
	}, nil
}

// Verdict maps a peak temperature and Stoessel class to a safety verdict:
//
//	PeakTemp ≥ ThermalLimit or class==5 → runaway
//	PeakTemp ≥ ThermalLimit-10 or class==4 → marginal
//	else → safe
//
// A non-positive ThermalLimit disables the proximity rule (class alone decides).
func Verdict(r model.Recipe, peakTemp float64, class int) model.SafetyVerdict {
	if class == 5 {
		return model.VerdictRunaway
	}
	if r.ThermalLimit > 0 {
		if peakTemp >= r.ThermalLimit {
			return model.VerdictRunaway
		}
		if peakTemp >= r.ThermalLimit-10 {
			return model.VerdictMarginal
		}
	}
	if class == 4 {
		return model.VerdictMarginal
	}
	return model.VerdictSafe
}

// IsRunaway reports whether a result indicates thermal runaway.
func IsRunaway(res model.KineticsResult) bool {
	return res.Verdict == model.VerdictRunaway
}

// IsThermallyCompatible reports whether a recipe's MTSR fits a reactor's maximum
// operating temperature. This is the assignment-time guard that prevents
// running a recipe whose adiabatic ceiling exceeds the vessel rating.
func IsThermallyCompatible(r model.Recipe, rc model.Reactor) bool {
	if rc.MaxOperatingTemp <= 0 {
		return true
	}
	return MTSR(r) <= rc.MaxOperatingTemp
}
