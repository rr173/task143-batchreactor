package kinetics

import "errors"

// ErrBadInput is returned when recipe/reactor parameters are non-physical
// (zero/negative concentration, density, heat capacity or volume).
var ErrBadInput = errors.New("kinetics: non-physical recipe or reactor input")

// KineticsOutput is the raw trajectory summary produced by Integrate, before
// the thermal-safety package classifies it.
type KineticsOutput struct {
	Conversion  float64 // 0–1
	PeakTemp   float64 // K
	FinalTemp   float64 // K
	Duration    float64 // s
	QGenMax     float64 // W
	QRemAtPeak  float64 // W
	Steps       int
	Isothermal  bool
}
