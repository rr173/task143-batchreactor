package report

import (
	"testing"

	"task143-batchreactor/internal/forecast"
	"task143-batchreactor/internal/model"
)

func safeRecipe(id string) model.Recipe {
	return model.Recipe{
		ID: id, Name: "ester", Product: "P1", K0: 2.1e6, Ea: 60000, Order: model.OrderFirst,
		CA0: 2000, DeltaHrx: -60000, Rho: 1000, Cp: 3000, T0: 333.15, JacketTemp: 333.15,
		ThermalLimit: 450.0, MinConversion: 0.95, Duration: 3600,
	}
}

func buildForecastsForTest(t *testing.T, batches []model.Batch, recipes map[string]model.Recipe, reactors map[string]model.Reactor) []model.BatchForecast {
	t.Helper()
	return forecast.BuildAll(batches, recipes, reactors, map[string]model.SafetyResult{}, 5000)
}

func TestAbortedBatchCountedAsTerminal(t *testing.T) {
	campaign := model.Campaign{ID: "c1", Name: "abort-campaign", Status: model.CampaignRunning}
	aborted := model.Batch{
		ID: "b-abort", CampaignID: "c1", ReactorID: "r1", RecipeID: "f1", Seq: 1,
		Status: model.BatchAborted, PlannedStart: 1000, StartedAt: 1000, EndedAt: 2000,
	}
	batches := []model.Batch{aborted}
	recipes := map[string]model.Recipe{"f1": safeRecipe("f1")}
	reactors := map[string]model.Reactor{"r1": {ID: "r1", Name: "R-1", Status: model.ReactorAvailable}}

	// BuildAll forecasts first so report can fold in their risk/next-action.
	forecasts := buildForecastsForTest(t, batches, recipes, reactors)
	rep := Build(Input{Campaign: campaign, Batches: batches, Recipes: recipes, Reactors: reactors, Forecasts: forecasts, Now: 5000})

	if rep.AbortedCount != 1 {
		t.Fatalf("AbortedCount: got %d want 1", rep.AbortedCount)
	}
	if rep.ActiveCount != 0 {
		t.Fatalf("ActiveCount: got %d want 0 (aborted must not count as active)", rep.ActiveCount)
	}
	if rep.DoneCount != 0 || rep.FaultCount != 0 {
		t.Fatalf("done/fault counts: got done=%d fault=%d want 0/0", rep.DoneCount, rep.FaultCount)
	}
	if rep.BatchCount != 1 {
		t.Fatalf("BatchCount: got %d want 1", rep.BatchCount)
	}
	if len(rep.ReactorLoads) != 1 {
		t.Fatalf("reactor loads: got %d want 1", len(rep.ReactorLoads))
	}
	load := rep.ReactorLoads[0]
	if load.ActiveCount != 0 {
		t.Fatalf("reactor active count: got %d want 0", load.ActiveCount)
	}
	if load.TerminalCount != 1 {
		t.Fatalf("reactor terminal count: got %d want 1", load.TerminalCount)
	}
	// An aborted batch is not a fault; it must not inflate FaultCount.
	if load.FaultCount != 0 {
		t.Fatalf("reactor fault count: got %d want 0", load.FaultCount)
	}
	// And it must not be escalated into CriticalBatches (only faulted is).
	if contains(rep.CriticalBatches, aborted.ID) {
		t.Fatalf("aborted batch %s must not appear in CriticalBatches %v", aborted.ID, rep.CriticalBatches)
	}
}

func TestFaultedBatchCountedAsCriticalAndTerminal(t *testing.T) {
	campaign := model.Campaign{ID: "c2", Name: "fault-campaign", Status: model.CampaignRunning}
	faulted := model.Batch{
		ID: "b-fault", CampaignID: "c2", ReactorID: "r1", RecipeID: "f1", Seq: 1,
		Status: model.BatchFaulted, PlannedStart: 1000, StartedAt: 1000, EndedAt: 2500, FaultReason: "runaway",
	}
	batches := []model.Batch{faulted}
	recipes := map[string]model.Recipe{"f1": safeRecipe("f1")}
	reactors := map[string]model.Reactor{"r1": {ID: "r1", Name: "R-1", Status: model.ReactorAvailable}}
	forecasts := buildForecastsForTest(t, batches, recipes, reactors)
	rep := Build(Input{Campaign: campaign, Batches: batches, Recipes: recipes, Reactors: reactors, Forecasts: forecasts, Now: 5000})

	if rep.FaultCount != 1 {
		t.Fatalf("FaultCount: got %d want 1", rep.FaultCount)
	}
	if rep.ActiveCount != 0 {
		t.Fatalf("ActiveCount: got %d want 0", rep.ActiveCount)
	}
	if !contains(rep.CriticalBatches, faulted.ID) {
		t.Fatalf("faulted batch %s must appear in CriticalBatches %v", faulted.ID, rep.CriticalBatches)
	}
	if len(rep.ReactorLoads) != 1 || rep.ReactorLoads[0].TerminalCount != 1 {
		t.Fatalf("reactor terminal count: got %+v", rep.ReactorLoads)
	}
}
