package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"task143-batchreactor/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func seedReactorAndRecipe(t *testing.T, st *Store) {
	ctx := context.Background()
	if err := st.CreateReactor(ctx, nil, &model.Reactor{ID: "r1", Name: "R", Volume: 1, HeatTransferU: 1, HeatTransferArea: 1, MaxOperatingTemp: 500, Status: model.ReactorAvailable, CreatedAt: 1}); err != nil {
		t.Fatalf("create reactor: %v", err)
	}
	if err := st.CreateRecipe(ctx, nil, &model.Recipe{ID: "f1", Name: "X", Product: "P", K0: 1, Ea: 1, Order: model.OrderFirst, CA0: 1, DeltaHrx: -1, Rho: 1, Cp: 1, T0: 300, JacketTemp: 300, ThermalLimit: 400, MinConversion: 0.9, Duration: 1, CreatedAt: 1}); err != nil {
		t.Fatalf("create recipe: %v", err)
	}
}

func TestCreateAndGetReactor(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	r := &model.Reactor{ID: "r1", Name: "R-1", Volume: 2, HeatTransferU: 100, HeatTransferArea: 4, MaxOperatingTemp: 500, Material: "SS", Status: model.ReactorAvailable, CreatedAt: 1}
	if err := st.CreateReactor(ctx, nil, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := st.GetReactor(ctx, "r1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "R-1" || got.Status != model.ReactorAvailable {
		t.Fatalf("got %+v", got)
	}
}

func TestGetReactorNotFound(t *testing.T) {
	st := openTestStore(t)
	if _, err := st.GetReactor(context.Background(), "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRecipeCRUD(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	r := &model.Recipe{ID: "f1", Name: "X", Product: "P", K0: 1, Ea: 1, Order: model.OrderFirst, CA0: 1, DeltaHrx: -1, Rho: 1, Cp: 1, T0: 300, JacketTemp: 300, ThermalLimit: 400, MinConversion: 0.9, Duration: 100, CreatedAt: 1}
	if err := st.CreateRecipe(ctx, nil, r); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := st.GetRecipe(ctx, "f1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Order != model.OrderFirst || got.Product != "P" {
		t.Fatalf("got %+v", got)
	}
}

func TestRecipeTxVariant(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	r := &model.Recipe{ID: "f1", Name: "X", Product: "P", K0: 1, Ea: 1, Order: model.OrderFirst, CA0: 1, DeltaHrx: -1, Rho: 1, Cp: 1, T0: 300, JacketTemp: 300, ThermalLimit: 400, MinConversion: 0.9, Duration: 1, CreatedAt: 1}
	err := st.InTx(ctx, func(tx *sql.Tx) error {
		return st.CreateRecipe(ctx, tx, r)
	})
	if err != nil {
		t.Fatalf("create in tx: %v", err)
	}
	got, err := st.GetRecipe(ctx, "f1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "f1" {
		t.Fatalf("got %+v", got)
	}
}

func TestCleaningMatrix(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	if err := st.SetCleaning(ctx, nil, "A", "B", model.CleaningMedium); err != nil {
		t.Fatalf("set: %v", err)
	}
	sev, err := st.CleaningSeverity(ctx, "A", "B")
	if err != nil || sev != model.CleaningMedium {
		t.Fatalf("get: sev %v err %v", sev, err)
	}
	sev, _ = st.CleaningSeverity(ctx, "A", "C")
	if sev != model.CleaningNone {
		t.Fatalf("missing pair → none, got %v", sev)
	}
	// Upsert replaces.
	st.SetCleaning(ctx, nil, "A", "B", model.CleaningLight)
	sev, _ = st.CleaningSeverity(ctx, "A", "B")
	if sev != model.CleaningLight {
		t.Fatalf("upsert: got %v", sev)
	}
}

func TestCampaignAndItems(t *testing.T) {
	st := openTestStore(t)
	seedReactorAndRecipe(t, st)
	ctx := context.Background()
	if err := st.CreateCampaign(ctx, nil, &model.Campaign{ID: "c1", Name: "C", Status: model.CampaignDraft, CreatedAt: 1}); err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if err := st.AddCampaignItem(ctx, nil, &model.CampaignItem{CampaignID: "c1", Seq: 1, RecipeID: "f1", ReactorID: "r1", BatchCount: 2, Status: model.ItemPending}); err != nil {
		t.Fatalf("add item: %v", err)
	}
	items, err := st.ListCampaignItems(ctx, "c1")
	if err != nil || len(items) != 1 {
		t.Fatalf("list items: %v len %d", err, len(items))
	}
	if items[0].BatchCount != 2 {
		t.Fatalf("batch count: %d", items[0].BatchCount)
	}
}

func TestBatchAndEventsTx(t *testing.T) {
	st := openTestStore(t)
	seedReactorAndRecipe(t, st)
	ctx := context.Background()
	st.CreateCampaign(ctx, nil, &model.Campaign{ID: "c1", Name: "C", Status: model.CampaignRunning, CreatedAt: 1})
	b := &model.Batch{ID: "b1", CampaignID: "c1", CampaignItemSeq: 1, ReactorID: "r1", RecipeID: "f1", Seq: 1, Status: model.BatchQueued, PlannedStart: 1}
	err := st.InTx(ctx, func(tx *sql.Tx) error {
		if err := st.CreateBatch(ctx, tx, b); err != nil {
			return err
		}
		if err := st.AppendEvent(ctx, tx, "b1", "queued", 1, ""); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	ids, types, err := st.ListEvents(ctx, "b1")
	if err != nil || len(ids) != 1 || types[0] != "queued" {
		t.Fatalf("events: ids %v types %v err %v", ids, types, err)
	}
	et, _ := st.LatestEventType(ctx, nil, "b1")
	if et != "queued" {
		t.Fatalf("latest event: %s", et)
	}
	got, err := st.GetBatch(ctx, "b1")
	if err != nil || got.Status != model.BatchQueued {
		t.Fatalf("get batch: %v %+v", err, got)
	}
}

func TestActiveBatchOnReactor(t *testing.T) {
	st := openTestStore(t)
	seedReactorAndRecipe(t, st)
	ctx := context.Background()
	st.CreateCampaign(ctx, nil, &model.Campaign{ID: "c1", Name: "C", Status: model.CampaignRunning, CreatedAt: 1})
	st.CreateBatch(ctx, nil, &model.Batch{ID: "b1", CampaignID: "c1", CampaignItemSeq: 1, ReactorID: "r1", RecipeID: "f1", Seq: 1, Status: model.BatchReacting, PlannedStart: 1})
	active, err := st.ActiveBatchOnReactor(ctx, "r1")
	if err != nil || active == nil || active.ID != "b1" {
		t.Fatalf("active batch: %v %+v", err, active)
	}
}

func TestSafetyResultUpsert(t *testing.T) {
	st := openTestStore(t)
	seedReactorAndRecipe(t, st)
	ctx := context.Background()
	st.CreateCampaign(ctx, nil, &model.Campaign{ID: "c1", Name: "C", Status: model.CampaignRunning, CreatedAt: 1})
	st.CreateBatch(ctx, nil, &model.Batch{ID: "b1", CampaignID: "c1", CampaignItemSeq: 1, ReactorID: "r1", RecipeID: "f1", Seq: 1, Status: model.BatchCooling, PlannedStart: 1})
	sr := model.SafetyResult{BatchID: "b1", DeltaTad: 40, MTSR: 373, StoesselClass: 2, TMRSeconds: 468, PeakTemp: 342, Conversion: 0.975, Verdict: model.VerdictSafe}
	if err := st.UpsertSafetyResult(ctx, nil, sr); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := st.GetSafetyResult(ctx, "b1")
	if err != nil || got.StoesselClass != 2 {
		t.Fatalf("get: %v %+v", err, got)
	}
	// Upsert replaces.
	sr.StoesselClass = 4
	st.UpsertSafetyResult(ctx, nil, sr)
	got, _ = st.GetSafetyResult(ctx, "b1")
	if got.StoesselClass != 4 {
		t.Fatalf("upsert replace: %d", got.StoesselClass)
	}
}
