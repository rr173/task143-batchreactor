package selfcheck

import (
	"math"
	"testing"

	"task143-batchreactor/internal/forecast"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/report"
	"task143-batchreactor/internal/thermal"
)

func TestBug05_NonExothermicRecipeStaysNonCritical(t *testing.T) {
	rcp := model.Recipe{DeltaHrx: 100, K0: 1e8, Ea: 40000, Rho: 1000, Cp: 3000, CA0: 10, T0: 320, ThermalLimit: 500}
	if !math.IsInf(thermal.TMR(rcp, rcp.T0), 1) { t.Error("endothermic recipe has a finite TMR") }
	f := forecast.Build(forecast.Input{Batch: model.Batch{ID: "endo", Status: model.BatchQueued}, Recipe: rcp, Result: &model.SafetyResult{TMRSeconds: 0, PeakTemp: 320, Verdict: model.VerdictSafe}})
	if f.Risk != model.RiskNormal { t.Errorf("non-exothermic forecast risk = %s", f.Risk) }
	r := report.Build(report.Input{Batches: []model.Batch{{ID: "endo", SafetyVerdict: model.VerdictSafe}}})
	if r.SafeCount != 1 || r.MarginalCount != 0 { t.Errorf("safe count = %d, marginal = %d", r.SafeCount, r.MarginalCount) }
}
