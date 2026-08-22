package campaign

import (
	"testing"

	"task143-batchreactor/internal/model"
)

func rcp(product string, peakT float64) model.Recipe {
	return model.Recipe{Product: product, T0: 300, DeltaHrx: -1, CA0: 1, Rho: 1, Cp: 1, K0: 1, Ea: 1, Duration: 1, ThermalLimit: peakT}
}

func vessel(maxT float64) model.Reactor {
	return model.Reactor{Volume: 1, MaxOperatingTemp: maxT, HeatTransferU: 1, HeatTransferArea: 1}
}

func TestPlanThermalIncompatRejected(t *testing.T) {
	// MTSR = 300 + (-(-1))·1/(1·1) = 301. Reactor max 290 → incompatible.
	in := PlanInput{
		Items: []model.CampaignItem{{CampaignID: "c", Seq: 1, RecipeID: "r1", ReactorID: "k1", BatchCount: 1}},
		Recipes:  map[string]model.Recipe{"r1": rcp("A", 400)},
		Reactors: map[string]model.Reactor{"k1": vessel(290)},
		Matrix:   func(string, string) model.CleaningSeverity { return model.CleaningNone },
		StartAt:  1000,
	}
	plan, err := Plan(in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Errors) == 0 {
		t.Fatal("expected thermal incompatibility error")
	}
	if len(plan.Batches) != 0 {
		t.Fatalf("no batches should be planned on incompatibility, got %d", len(plan.Batches))
	}
}

func TestPlanSameProductNoCleaning(t *testing.T) {
	in := PlanInput{
		Items: []model.CampaignItem{
			{CampaignID: "c", Seq: 1, RecipeID: "r1", ReactorID: "k1", BatchCount: 2},
		},
		Recipes:  map[string]model.Recipe{"r1": rcp("A", 400)},
		Reactors: map[string]model.Reactor{"k1": vessel(500)},
		Matrix:   func(string, string) model.CleaningSeverity { return model.CleaningHeavy },
		StartAt:  1000,
	}
	plan, err := Plan(in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(plan.Batches))
	}
	if plan.Batches[0].CleaningBefore != 0 || plan.Batches[1].CleaningBefore != 0 {
		t.Fatalf("same-product runs need no cleaning: got %.0f %.0f", plan.Batches[0].CleaningBefore, plan.Batches[1].CleaningBefore)
	}
}

func TestPlanSequenceDependentCleaning(t *testing.T) {
	in := PlanInput{
		Items: []model.CampaignItem{
			{CampaignID: "c", Seq: 1, RecipeID: "r1", ReactorID: "k1", BatchCount: 1},
			{CampaignID: "c", Seq: 2, RecipeID: "r2", ReactorID: "k1", BatchCount: 1},
		},
		Recipes: map[string]model.Recipe{
			"r1": rcp("A", 400),
			"r2": rcp("B", 400),
		},
		Reactors: map[string]model.Reactor{"k1": vessel(500)},
		Matrix:   func(from, to string) model.CleaningSeverity {
			if from == "A" && to == "B" {
				return model.CleaningMedium
			}
			return model.CleaningNone
		},
		StartAt: 1000,
	}
	plan, err := Plan(in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Batches[1].CleaningBefore != model.CleaningMedium.CleaningDuration() {
		t.Fatalf("expected medium cleaning 1800s before second batch, got %.0f", plan.Batches[1].CleaningBefore)
	}
	if plan.Batches[0].CleaningBefore != 0 {
		t.Fatalf("first batch needs no cleaning, got %.0f", plan.Batches[0].CleaningBefore)
	}
	if plan.Batches[1].CleaningProduct != "A" {
		t.Fatalf("cleaning product should be A, got %s", plan.Batches[1].CleaningProduct)
	}
}

func TestPlanMultiReactorIndependent(t *testing.T) {
	in := PlanInput{
		Items: []model.CampaignItem{
			{CampaignID: "c", Seq: 1, RecipeID: "r1", ReactorID: "k1", BatchCount: 1},
			{CampaignID: "c", Seq: 2, RecipeID: "r2", ReactorID: "k2", BatchCount: 1},
		},
		Recipes: map[string]model.Recipe{
			"r1": rcp("A", 400),
			"r2": rcp("B", 400),
		},
		Reactors: map[string]model.Reactor{"k1": vessel(500), "k2": vessel(500)},
		Matrix:   func(string, string) model.CleaningSeverity { return model.CleaningHeavy },
		StartAt:  1000,
	}
	plan, err := Plan(in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	// Different reactors → no cleaning between them.
	for _, b := range plan.Batches {
		if b.CleaningBefore != 0 {
			t.Fatalf("different reactors need no cleaning: %.0f", b.CleaningBefore)
		}
	}
}

func TestCleaningSeverityDuration(t *testing.T) {
	if model.CleaningNone.CleaningDuration() != 0 {
		t.Error("none → 0")
	}
	if model.CleaningLight.CleaningDuration() != 600 {
		t.Error("light → 600")
	}
	if model.CleaningMedium.CleaningDuration() != 1800 {
		t.Error("medium → 1800")
	}
	if model.CleaningHeavy.CleaningDuration() != 3600 {
		t.Error("heavy → 3600")
	}
}

// TestPlanLightCleaningVisible guards against the light-cleaning regression:
// a light (600s) changeover between two products must be recorded on the
// second batch AND advance its planned start by the cleaning duration. If the
// start is not advanced, the running view (timeline/forecast) collapses the
// gap and the batches look like a no-wait switch — the exact defect this
// regression test pins.
func TestPlanLightCleaningVisible(t *testing.T) {
	const dur = float64(3600) // distinct recipe duration so offsets are unambiguous
	recipe := func(product string) model.Recipe {
		r := rcp(product, 400)
		r.Duration = dur
		return r
	}
	in := PlanInput{
		Items: []model.CampaignItem{
			{CampaignID: "c", Seq: 1, RecipeID: "r1", ReactorID: "k1", BatchCount: 1},
			{CampaignID: "c", Seq: 2, RecipeID: "r2", ReactorID: "k1", BatchCount: 1},
		},
		Recipes: map[string]model.Recipe{
			"r1": recipe("A"),
			"r2": recipe("B"),
		},
		Reactors: map[string]model.Reactor{"k1": vessel(500)},
		Matrix: func(from, to string) model.CleaningSeverity {
			if from == "A" && to == "B" {
				return model.CleaningLight
			}
			return model.CleaningNone
		},
		StartAt: 1000,
	}
	plan, err := Plan(in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(plan.Batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(plan.Batches))
	}
	// Light cleaning is a real changeover step: it must be recorded, not 0.
	if plan.Batches[1].CleaningBefore != model.CleaningLight.CleaningDuration() {
		t.Fatalf("light cleaning must be visible on the second batch: got %.0f want %.0f",
			plan.Batches[1].CleaningBefore, model.CleaningLight.CleaningDuration())
	}
	if plan.Batches[1].CleaningProduct != "A" {
		t.Fatalf("cleaning product should be A, got %s", plan.Batches[1].CleaningProduct)
	}
	// The cleaning step must push the second batch's start later than a direct
	// handoff would. A no-wait switch would place batch 2 at batch1.start+dur.
	wantStart := in.StartAt + int64(dur) + int64(model.CleaningLight.CleaningDuration())
	if plan.Batches[1].PlannedStart != wantStart {
		t.Fatalf("planned start must include the cleaning gap: got %d want %d",
			plan.Batches[1].PlannedStart, wantStart)
	}
	if plan.Batches[0].PlannedStart != in.StartAt {
		t.Fatalf("first batch starts at StartAt: got %d want %d",
			plan.Batches[0].PlannedStart, in.StartAt)
	}
}
