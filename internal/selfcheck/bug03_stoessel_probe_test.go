package selfcheck

import (
	"task143-batchreactor/internal/forecast"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/report"
	"task143-batchreactor/internal/thermal"
	"testing"
)

func TestBug03_StoesselFourRemainsMarginalAcrossReports(t *testing.T) {
	r := exothermicRecipe()
	r.ThermalLimit = 1000
	if got := thermal.Verdict(r, 350, 4); got != model.VerdictMarginal {
		t.Fatalf("class four verdict=%s, want marginal", got)
	}
	f := forecast.Build(forecast.Input{Batch: model.Batch{ID: "b", ReactorID: "r", RecipeID: "p", Status: model.BatchQueued}, Recipe: r, Reactor: safeReactor(), Result: &model.SafetyResult{PeakTemp: 350, Verdict: model.VerdictMarginal}, Now: 1})
	if f.Risk != model.RiskWatch {
		t.Fatalf("class four forecast=%s, want watch", f.Risk)
	}
	a := report.Build(report.Input{Campaign: model.Campaign{ID: "c"}, Batches: []model.Batch{{ID: "b", ReactorID: "r", RecipeID: "p", SafetyVerdict: model.VerdictMarginal}}, Recipes: map[string]model.Recipe{"p": r}, Reactors: map[string]model.Reactor{"r": safeReactor()}, Now: 1})
	if a.MarginalCount != 1 {
		t.Fatalf("class four report marginal=%d, want 1", a.MarginalCount)
	}
}
