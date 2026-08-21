// Package kinetics integrates the coupled reaction-rate / heat-balance ODE for
// a liquid-phase batch reactor. The integrator is a fixed-step RK4 over
//
//	dC_A/dt = -k(T)·C_A^Order            (limiting-reactant consumption)
//	dT/dt   = (Q_gen - Q_rem) / (ρ·Cp·V)  (jacket heat balance)
//
// where k(T)=K0·exp(-Ea/(R·T)), Q_gen=(-ΔH_rx)·k(T)·C_A^Order·V (exotherm when
// ΔH_rx<0) and Q_rem=U·A·(T-T_j). The integration depends only on the recipe
// and reactor inputs, never on the wall clock or any random source, so the
// result is deterministic and recomputable on restart.
//
// A hard temperature ceiling (hardTceiling) caps the trajectory before a
// thermal runaway can overflow the float exponent and produce NaN/Inf; the
// caller (thermal.Classify) interprets a peaked-ceiling trajectory as a
// runaway via the recipe's thermal limit.
package kinetics

import (
	"math"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/units"
)

// hardTCeiling is the absolute temperature (K) at which integration stops to
// avoid float overflow; it is well above any plausible thermal limit.
const hardTCeiling = 5000.0

// State is the integrator's instantaneous state.
type State struct {
	CA   float64 // mol/m³
	T    float64 // K
	Time float64 // s
	QGen float64 // W (current heat generation)
	QRem float64 // W (current heat removal)
}

// heatFlux returns (Q_gen, Q_rem) at the given state — separated from
// derivatives so callers don't confuse the (dCA, dT) return with heat flux.
func heatFlux(s State, r model.Recipe, rc model.Reactor) (qGen, qRem float64) {
	if s.T <= 0 {
		s.T = r.T0
	}
	ca := s.CA
	if ca < 0 {
		ca = 0 // concentration cannot be negative; clamping in the derivative
		// evaluation prevents an RK4 intermediate from flipping the rate sign
		// and feeding positive feedback into the coupled ODE (which would
		// otherwise blow CA up and produce NaN).
	}
	k := ArrheniusRate(r.K0, r.Ea, s.T)
	var rate float64
	switch r.Order {
	case model.OrderSecond:
		rate = k * ca * ca
	default:
		rate = k * ca
	}
	qGen = (-r.DeltaHrx) * rate * rc.Volume // W; exotherm ΔH<0 → qGen>0
	qRem = rc.HeatTransferU * rc.HeatTransferArea * (s.T - r.JacketTemp)
	return qGen, qRem
}

// derivatives computes dCA/dt and dT/dt at the given (CA, T).
func derivatives(s State, r model.Recipe, rc model.Reactor) (dCA, dT float64) {
	qGen, qRem := heatFlux(s, r, rc)
	heatCap := r.Rho * r.Cp * rc.Volume
	if heatCap <= 0 {
		heatCap = 1
	}
	// rate of consumption of A = k·CA^Order = (-dCA/dt). Clamp CA to ≥0 so a
	// negative RK4 intermediate cannot flip the rate sign.
	ca := s.CA
	if ca < 0 {
		ca = 0
	}
	k := ArrheniusRate(r.K0, r.Ea, s.T)
	var rate float64
	switch r.Order {
	case model.OrderSecond:
		rate = k * ca * ca
	default:
		rate = k * ca
	}
	dCA = -rate
	dT = (qGen - qRem) / heatCap
	return dCA, dT
}

// ArrheniusRate returns k(T) = K0·exp(-Ea/(R·T)).
func ArrheniusRate(k0, ea, tKelvin float64) float64 {
	if tKelvin <= 0 {
		return 0
	}
	return k0 * math.Exp(-ea/(units.R*tKelvin))
}

// Integrate runs the coupled ODE from t=0 to t=duration (or until the reactant
// is exhausted or the trajectory hits the hard temperature ceiling). dt<=0
// falls back to units.DefaultDt. isothermal pins T to T0 (only dCA/dt is
// integrated, heat balance reported for diagnostics).
func Integrate(r model.Recipe, rc model.Reactor, duration, dt float64) (KineticsOutput, error) {
	if duration <= 0 {
		duration = r.Duration
	}
	if dt <= 0 {
		dt = units.DefaultDt * 2
	}
	if r.CA0 <= 0 || r.Rho <= 0 || r.Cp <= 0 || rc.Volume <= 0 {
		return KineticsOutput{}, ErrBadInput
	}
	steps := int(math.Ceil(duration / dt))
	if steps <= 0 {
		steps = 1
	}
	if steps > 5_000_000 {
		steps = 5_000_000
	}
	isothermal := r.JacketTemp == r.T0 && rc.HeatTransferU == 0

	s := State{CA: r.CA0, T: r.T0}
	var peak = r.T0
	var qGenMax, qRemAtPeak float64
	k0 := ArrheniusRate(r.K0, r.Ea, s.T)
	s.QGen, s.QRem = heatFlux(s, r, rc)

	for i := 0; i < steps; i++ {
		if isothermal {
			s.QGen, _ = heatFlux(s, r, rc)
			s.QRem = s.QGen
			dCA1 := -k0 * powCA(s.CA, r.Order)
			c2 := s.CA + 0.5*dt*dCA1
			dCA2 := -k0 * powCA(c2, r.Order)
			c3 := s.CA + 0.5*dt*dCA2
			dCA3 := -k0 * powCA(c3, r.Order)
			c4 := s.CA + dt*dCA3
			dCA4 := -k0 * powCA(c4, r.Order)
			s.CA += dt * (dCA1 + 2*dCA2 + 2*dCA3 + dCA4) / 6
			s.T = r.T0
		} else {
			dCA1, dT1 := derivatives(s, r, rc)
			s2 := State{CA: s.CA + 0.5*dt*dCA1, T: s.T + 0.5*dt*dT1, Time: s.Time + 0.5*dt}
			dCA2, dT2 := derivatives(s2, r, rc)
			s3 := State{CA: s.CA + 0.5*dt*dCA2, T: s.T + 0.5*dt*dT2, Time: s.Time + 0.5*dt}
			dCA3, dT3 := derivatives(s3, r, rc)
			s4 := State{CA: s.CA + dt*dCA3, T: s.T + dt*dT3, Time: s.Time + dt}
			dCA4, dT4 := derivatives(s4, r, rc)
			s.CA += dt * (dCA1 + 2*dCA2 + 2*dCA3 + dCA4) / 6
			s.T += dt * (dT1 + 2*dT2 + 2*dT3 + dT4) / 6
			s.QGen, s.QRem = heatFlux(s, r, rc)
		}
		s.Time += dt
		// Numerical safety: stop on NaN/Inf or a hard thermal runaway ceiling so
		// a divergent exotherm cannot overflow the float exponent.
		if math.IsNaN(s.CA) || math.IsNaN(s.T) || math.IsInf(s.CA, 0) || math.IsInf(s.T, 0) {
			break
		}
		if s.CA < 0 {
			s.CA = 0
		}
		if s.T < 0 {
			s.T = 0
		}
		if s.T > peak {
			peak = s.T
			qGenMax = s.QGen
			qRemAtPeak = s.QRem
		}
		if s.CA <= 0 {
			break
		}
		if s.T >= hardTCeiling {
			break
		}
	}
	if qGenMax == 0 {
		qGenMax = s.QGen
		qRemAtPeak = s.QRem
	}
	conv := (r.CA0 - s.CA) / r.CA0
	if math.IsNaN(conv) {
		conv = 1 // trajectory diverged → treat as fully consumed (runaway)
	}
	if conv < 0 {
		conv = 0
	}
	if conv > 1 {
		conv = 1
	}
	return KineticsOutput{
		Conversion:  conv,
		PeakTemp:    peak,
		FinalTemp:    s.T,
		Duration:     s.Time,
		QGenMax:      qGenMax,
		QRemAtPeak:   qRemAtPeak,
		Steps:        steps,
		Isothermal:   isothermal,
	}, nil
}

func powCA(ca float64, order model.ReactionOrder) float64 {
	switch order {
	case model.OrderSecond:
		return ca * ca
	default:
		return ca
	}
}
