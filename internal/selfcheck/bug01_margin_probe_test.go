package selfcheck

import (
	"testing"

	"task143-batchreactor/internal/forecast"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/report"
	"task143-batchreactor/internal/thermal"
)

func TestBug01_MarginalThermalMarginRemainsVisibleAcrossOperations(t *testing.T) {
	r := exothermicRecipe()
	r.ThermalLimit = 450
	if got := thermal.Verdict(r, 442, 2); got != model.VerdictMarginal {
		t.Fatalf("8K thermal margin verdict = %s, want marginal", got)
	}
	b := model.Batch{ID: "b", ReactorID: "r", RecipeID: "p", Status: model.BatchQueued}
	f := forecast.Build(forecast.Input{Batch: b, Recipe: r, Reactor: safeReactor(), Result: &model.SafetyResult{PeakTemp: 442, Verdict: model.VerdictMarginal}, Now: 1})
	if f.Risk != model.RiskWatch {
		t.Fatalf("marginal forecast risk = %s, want watch", f.Risk)
	}
	a := report.Build(report.Input{Campaign: model.Campaign{ID: "c"}, Batches: []model.Batch{{ID: "b", ReactorID: "r", RecipeID: "p", SafetyVerdict: model.VerdictMarginal}}, Recipes: map[string]model.Recipe{"p": r}, Reactors: map[string]model.Reactor{"r": safeReactor()}, Forecasts: []model.BatchForecast{f}, Now: 1})
	if a.MarginalCount != 1 || a.SafeCount != 0 {
		t.Fatalf("report counts marginal=%d safe=%d, want 1/0", a.MarginalCount, a.SafeCount)
	}
}
