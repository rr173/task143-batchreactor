// Package campaign resolves a campaign's items into a concrete schedule of
// batches per reactor, applying the two hard scheduling constraints:
//
//  1. thermal compatibility — a recipe's MTSR must not exceed the reactor's
//     max operating temperature;
//  2. sequence-dependent cleaning — when two different recipes run back-to-back
//     on the same reactor, the cross-contamination matrix decides whether a
//     cleaning step is inserted and how long it lasts.
//
// Planning is a pure function of the stored items, the cleaning matrix and the
// recipe/reactor inputs, so it recomputes identically on restart.
package campaign

import (
	"sort"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/reactor"
)

// MatrixLookup returns the cleaning severity from one product to another. A
// missing pair means no cleaning (CleaningNone).
type MatrixLookup func(fromProduct, toProduct string) model.CleaningSeverity

// PlanInput bundles everything Plan needs.
type PlanInput struct {
	Items    []model.CampaignItem
	Recipes  map[string]model.Recipe     // keyed by recipe ID
	Reactors map[string]model.Reactor    // keyed by reactor ID
	Matrix   MatrixLookup
	StartAt  int64 // epoch seconds; batches are planned relative to this
}

// Plan computes the resolved schedule. It groups items by reactor (preserving
// per-reactor order by Seq), checks thermal compatibility for every
// (recipe,reactor), and inserts cleaning gaps when the previous batch on the
// same reactor ran a different product with a non-zero contamination severity.
// Cleaning is assessed on the previous scheduled batch's product (the actual
// immediately-preceding run on that reactor), so re-runs of the same product
// need no cleaning.
func Plan(in PlanInput) (model.CampaignPlan, error) {
	plan := model.CampaignPlan{CampaignID: campaignIDOf(in.Items)}

	// Group items by reactor, preserving the campaign Seq order within each.
	byReactor := map[string][]model.CampaignItem{}
	reactorOrder := []string{}
	for _, it := range in.Items {
		if _, ok := byReactor[it.ReactorID]; !ok {
			reactorOrder = append(reactorOrder, it.ReactorID)
		}
		byReactor[it.ReactorID] = append(byReactor[it.ReactorID], it)
	}
	sort.Strings(reactorOrder)

	for _, rid := range reactorOrder {
		rc, ok := in.Reactors[rid]
		if !ok {
			for _, it := range byReactor[rid] {
				plan.Errors = append(plan.Errors, model.PlanError{Seq: it.Seq, RecipeID: it.RecipeID, Reactor: rid, Reason: "unknown reactor"})
			}
			continue
		}
		items := byReactor[rid]
		// Order items on this reactor by Seq.
		sort.Slice(items, func(i, j int) bool { return items[i].Seq < items[j].Seq })

		// Track the reactor timeline cursor and the product of the immediately
		// preceding batch, starting empty (first batch needs no cleaning).
		var cursor int64 = in.StartAt
		var prevProduct string
		var seq int
		for _, it := range items {
			r, ok := in.Recipes[it.RecipeID]
			if !ok {
				plan.Errors = append(plan.Errors, model.PlanError{Seq: it.Seq, RecipeID: it.RecipeID, Reactor: rid, Reason: "unknown recipe"})
				continue
			}
			// Thermal compatibility check (hard scheduling constraint #1).
			if !reactor.CompatibleWith(r, rc) {
				plan.Errors = append(plan.Errors, model.PlanError{Seq: it.Seq, RecipeID: it.RecipeID, Reactor: rc.Name, Reason: reactor.IncompatibilityReason(r, rc)})
				continue
			}
			// Sequence-dependent cleaning (hard scheduling constraint #2).
			// The cross-contamination matrix turns a product changeover into a
			// real cleaning step: its graded duration is recorded on the first
			// batch that runs the new product, and it advances the reactor
			// timeline cursor so the following batch's planned start honors
			// the changeover instead of implying a direct handoff.
			var cleaningBefore float64
			cleaningProduct := ""
			if prevProduct != "" && prevProduct != r.Product {
				sev := in.Matrix(prevProduct, r.Product)
				cleaningBefore = sev.CleaningDuration()
				cleaningProduct = prevProduct
				cursor += int64(cleaningBefore)
			}
			for b := 0; b < it.BatchCount; b++ {
				seq++
				plan.Batches = append(plan.Batches, model.PlannedBatch{
					ReactorID: rid, RecipeID: it.RecipeID, Seq: seq,
					PlannedStart: cursor, CleaningBefore: cleaningBefore, CleaningProduct: cleaningProduct,
				})
				// Subsequent batches of the SAME product need no cleaning.
				cleaningBefore = 0
				cleaningProduct = ""
				cursor = nextCursor(cursor, r)
			}
			prevProduct = r.Product
			plan.Items = append(plan.Items, model.PlannedItem{
				Seq: it.Seq, RecipeID: it.RecipeID, ReactorID: rid, BatchCount: it.BatchCount,
			})
		}
	}
	return plan, nil
}

func campaignIDOf(items []model.CampaignItem) string {
	for _, it := range items {
		return it.CampaignID
	}
	return ""
}

// nextCursor advances the reactor timeline by the recipe's nominal duration.
// Real batch duration is set during reacting; the plan only needs an ordering
// estimate for planned-start sequencing within a reactor.
func nextCursor(cur int64, r model.Recipe) int64 {
	return cur + int64(r.Duration)
}
