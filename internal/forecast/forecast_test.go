package forecast

import (
	"testing"

	"task143-batchreactor/internal/model"
)

// mildRecipe is a safe, well-behaved exotherm (Stoessel 2, verdict safe). It is
// the fixture for asserting terminal batches stop attracting live thermal
// attention even when their recipe would otherwise read safe/normal.
func mildRecipe() model.Recipe {
	return model.Recipe{
		ID: "f1", Name: "ester", Product: "P1", K0: 2.1e6, Ea: 60000, Order: model.OrderFirst,
		CA0: 2000, DeltaHrx: -60000, Rho: 1000, Cp: 3000, T0: 333.15, JacketTemp: 333.15,
		ThermalLimit: 450.0, MinConversion: 0.95, Duration: 3600,
	}
}

func mildReactor() model.Reactor {
	return model.Reactor{ID: "r1", Name: "R-1", Volume: 2.0, HeatTransferU: 2000, HeatTransferArea: 10.0, MaxOperatingTemp: 600.0}
}

func TestAbortedForecastIsTerminal(t *testing.T) {
	b := model.Batch{
		ID: "b1", ReactorID: "r1", RecipeID: "f1", Status: model.BatchAborted,
		PlannedStart: 1000, StartedAt: 1000, EndedAt: 2000,
	}
	f := Build(Input{Batch: b, Recipe: mildRecipe(), Reactor: mildReactor(), Now: 5000})
	if f.Phase != model.ForecastTerminal {
		t.Fatalf("aborted phase: got %s want terminal", f.Phase)
	}
	if f.Risk != model.RiskNormal {
		t.Fatalf("aborted risk: got %s want normal (production is over)", f.Risk)
	}
	if len(f.Reasons) != 0 {
		t.Fatalf("aborted must carry no live thermal reasons, got %v", f.Reasons)
	}
	// A terminal batch must report its recorded end, not a future estimate.
	if f.EstimatedFinish != b.EndedAt {
		t.Fatalf("aborted estimated finish: got %d want recorded EndedAt %d", f.EstimatedFinish, b.EndedAt)
	}
	if f.NextAction == "" {
		t.Fatal("aborted next action must be a terminal wrap-up, not empty")
	}
}

func TestDoneForecastIsTerminal(t *testing.T) {
	b := model.Batch{
		ID: "b2", ReactorID: "r1", RecipeID: "f1", Status: model.BatchDone,
		PlannedStart: 1000, StartedAt: 1000, EndedAt: 3000, Conversion: 0.97,
	}
	f := Build(Input{Batch: b, Recipe: mildRecipe(), Reactor: mildReactor(), Now: 5000})
	if f.Phase != model.ForecastTerminal {
		t.Fatalf("done phase: got %s want terminal", f.Phase)
	}
	if f.Risk != model.RiskNormal {
		t.Fatalf("done risk: got %s want normal", f.Risk)
	}
	if f.EstimatedFinish != b.EndedAt {
		t.Fatalf("done estimated finish: got %d want recorded EndedAt %d", f.EstimatedFinish, b.EndedAt)
	}
}

func TestFaultedForecastStaysCritical(t *testing.T) {
	b := model.Batch{
		ID: "b3", ReactorID: "r1", RecipeID: "f1", Status: model.BatchFaulted,
		PlannedStart: 1000, StartedAt: 1000, EndedAt: 2500, FaultReason: "thermal runaway",
	}
	f := Build(Input{Batch: b, Recipe: mildRecipe(), Reactor: mildReactor(), Now: 5000})
	if f.Phase != model.ForecastTerminal {
		t.Fatalf("faulted phase: got %s want terminal", f.Phase)
	}
	if f.Risk != model.RiskCritical {
		t.Fatalf("faulted risk: got %s want critical", f.Risk)
	}
	if f.EstimatedFinish != b.EndedAt {
		t.Fatalf("faulted estimated finish: got %d want recorded EndedAt %d", f.EstimatedFinish, b.EndedAt)
	}
}

func TestActiveForecastStillSurfacesThermalPreview(t *testing.T) {
	// Guard against the terminal short-circuit being over-broad: a reacting
	// batch must not be treated as terminal. The mild recipe's TMR (~468 s) is
	// below the 8-hour response window, so the live preview correctly flags it
	// critical — what matters for this test is only that the batch is still
	// read as live (reacting phase), not terminal.
	b := model.Batch{
		ID: "b4", ReactorID: "r1", RecipeID: "f1", Status: model.BatchReacting,
		PlannedStart: 1000, StartedAt: 1000,
	}
	f := Build(Input{Batch: b, Recipe: mildRecipe(), Reactor: mildReactor(), Now: 1200})
	if f.Phase != model.ForecastReact {
		t.Fatalf("reacting phase: got %s want react", f.Phase)
	}
	if f.Risk != model.RiskCritical {
		t.Fatalf("reacting risk: got %s want critical (mild recipe TMR < 8h)", f.Risk)
	}
	if len(f.Reasons) == 0 {
		t.Fatal("reacting batch must still carry its live thermal preview reasons")
	}
}
