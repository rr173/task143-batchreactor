package campaign

import (
	"fmt"
	"testing"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/report"
)

// TestPlanChangeoverAdvancesCursor is a regression guard for the
// "short cleaning compressed to instant" bug: a non-zero cleaning severity
// must (a) set CleaningBefore, (b) advance the timeline cursor by that
// duration so the next batch's PlannedStart reflects the changeover, and
// (c) surface as a positive gap in the report timeline.
func TestPlanChangeoverAdvancesCursor(t *testing.T) {
	rcp := func(product string) model.Recipe {
		return model.Recipe{Product: product, Duration: 3600, ThermalLimit: 600}
	}
	in := PlanInput{
		Items: []model.CampaignItem{
			{CampaignID: "c", Seq: 1, RecipeID: "rA", ReactorID: "k1", BatchCount: 1},
			{CampaignID: "c", Seq: 2, RecipeID: "rB", ReactorID: "k1", BatchCount: 1},
			{CampaignID: "c", Seq: 3, RecipeID: "rB", ReactorID: "k1", BatchCount: 1},
		},
		Recipes:  map[string]model.Recipe{"rA": rcp("A"), "rB": rcp("B")},
		Reactors: map[string]model.Reactor{"k1": {MaxOperatingTemp: 700, HeatTransferU: 1, HeatTransferArea: 1}},
		Matrix: func(from, to string) model.CleaningSeverity {
			if from == "A" && to == "B" {
				return model.CleaningLight // 600 s
			}
			return model.CleaningNone
		},
		StartAt: 1000,
	}
	plan, err := Plan(in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	// (a) CleaningBefore carries the 600s changeover.
	if got := plan.Batches[1].CleaningBefore; got != 600 {
		t.Fatalf("CleaningBefore before product switch = %.0f, want 600", got)
	}
	// (b) The cursor advanced: B1 starts after A's duration PLUS the 600s step.
	aEnd := plan.Batches[0].PlannedStart + 3600
	if got := plan.Batches[1].PlannedStart - aEnd; got != 600 {
		t.Fatalf("changeover interval = %d, want 600", got)
	}
	// Same-product B1→B2 has no cleaning, so B2 starts right after B1.
	b1End := plan.Batches[1].PlannedStart + 3600
	if got := plan.Batches[2].PlannedStart - b1End; got != 0 {
		t.Fatalf("same-product handoff = %d, want 0", got)
	}

	// (c) The report timeline surfaces the changeover as a positive gap.
	var batches []model.Batch
	expected := map[string]int64{}
	for _, b := range plan.Batches {
		batches = append(batches, model.Batch{ID: fmt.Sprintf("b%d", b.Seq), ReactorID: b.ReactorID, RecipeID: b.RecipeID, PlannedStart: b.PlannedStart, Status: model.BatchQueued})
		expected[batches[len(batches)-1].ID] = 3600
	}
	wins := report.Windows(batches, expected, in.StartAt)
	gaps := report.Gaps(wins)
	var changeover int64 = -1
	for _, g := range gaps {
		if g.BeforeID == "b1" && g.AfterID == "b2" {
			changeover = g.Seconds
		}
	}
	if changeover != 600 {
		t.Fatalf("timeline gap b1→b2 = %d, want 600", changeover)
	}
}
