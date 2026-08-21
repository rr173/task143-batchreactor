package selfcheck

import (
	"task143-batchreactor/internal/forecast"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/report"
	"task143-batchreactor/internal/thermal"
	"testing"
)

func TestBug02_ThermalLimitEqualityEscalatesEveryView(t *testing.T) {
	r := exothermicRecipe()
	r.ThermalLimit = 450
	if got := thermal.Verdict(r, 450, 2); got != model.VerdictRunaway {
		t.Fatalf("limit equality verdict=%s, want runaway", got)
	}
	f := forecast.Build(forecast.Input{Batch: model.Batch{ID: "b", ReactorID: "r", RecipeID: "p", Status: model.BatchReacting}, Recipe: r, Reactor: safeReactor(), Result: &model.SafetyResult{PeakTemp: 450, Verdict: model.VerdictSafe}, Now: 1})
	if f.Risk != model.RiskCritical {
		t.Fatalf("zero-headroom forecast=%s, want critical", f.Risk)
	}
	a := report.Build(report.Input{Campaign: model.Campaign{ID: "c"}, Batches: []model.Batch{{ID: "b", ReactorID: "r", RecipeID: "p", SafetyVerdict: model.VerdictRunaway}}, Recipes: map[string]model.Recipe{"p": r}, Reactors: map[string]model.Reactor{"r": safeReactor()}, Now: 1})
	if a.RunawayCount != 1 || a.MarginalCount != 0 {
		t.Fatalf("runaway/marginal=%d/%d, want 1/0", a.RunawayCount, a.MarginalCount)
	}
}
