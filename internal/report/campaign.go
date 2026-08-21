package report

import (
	"sort"

	"task143-batchreactor/internal/model"
)

// Input is the normalized read snapshot for a campaign report. Maps make it
// explicit that recipe/reactor/result lookup belongs to the report boundary,
// not to hidden database calls inside aggregation helpers.
type Input struct {
	Campaign  model.Campaign
	Batches   []model.Batch
	Recipes   map[string]model.Recipe
	Reactors  map[string]model.Reactor
	Forecasts []model.BatchForecast
	Now       int64
}

// Build creates a campaign-level operational summary. The result contains
// every batch even when a reference is missing; warnings and critical forecasts
// preserve that data-quality problem for the operator rather than dropping it.
func Build(in Input) model.CampaignAnalytics {
	out := model.CampaignAnalytics{CampaignID: in.Campaign.ID, CampaignName: in.Campaign.Name, CampaignStatus: in.Campaign.Status, GeneratedAt: in.Now, CriticalBatches: []string{}, ReactorLoads: []model.ReactorLoad{}, Forecasts: cloneForecasts(in.Forecasts), Warnings: []string{}}
	loads := map[string]*model.ReactorLoad{}
	expected := make(map[string]int64, len(in.Batches))
	for _, b := range in.Batches {
		out.BatchCount++
		countStatus(&out, b.Status)
		countVerdict(&out, b.SafetyVerdict)
		if b.Status == model.BatchFaulted {
			out.CriticalBatches = append(out.CriticalBatches, b.ID)
		}
		load := ensureLoad(loads, b.ReactorID, in.Reactors)
		load.BatchCount++
		if b.Status.IsTerminal() {
			load.TerminalCount++
		} else {
			load.ActiveCount++
		}
		if b.Status == model.BatchFaulted {
			load.FaultCount++
		}
		if r, ok := in.Recipes[b.RecipeID]; ok {
			d := r.Duration
			if d < 0 {
				d = 0
			}
			load.PlannedSeconds += d
			expected[b.ID] = int64(d)
		} else {
			out.Warnings = append(out.Warnings, "batch "+b.ID+" references a missing recipe")
		}
	}
	for _, f := range out.Forecasts {
		if f.Risk == model.RiskCritical && !contains(out.CriticalBatches, f.BatchID) {
			out.CriticalBatches = append(out.CriticalBatches, f.BatchID)
		}
		if f.Risk == model.RiskCritical {
			out.Warnings = append(out.Warnings, "watch batch "+f.BatchID+": "+f.NextAction)
		}
	}
	windows := Windows(in.Batches, expected, in.Now)
	out.EarliestStart, out.LatestFinish = Span(windows)
	for _, g := range Overlaps(windows) {
		out.Warnings = append(out.Warnings, "overlap on reactor "+g.ReactorID+" between "+g.BeforeID+" and "+g.AfterID)
	}
	for _, w := range windows {
		load := loads[w.ReactorID]
		if load == nil {
			continue
		}
		if load.FirstPlannedStart == 0 || w.Start < load.FirstPlannedStart {
			load.FirstPlannedStart = w.Start
		}
		if w.End > load.LastPlannedFinish {
			load.LastPlannedFinish = w.End
		}
	}
	for _, l := range loads {
		span := l.LastPlannedFinish - l.FirstPlannedStart
		if span > 0 {
			l.Utilization = l.PlannedSeconds / float64(span)
			if l.Utilization > 1 {
				l.Utilization = 1
			}
		}
		out.ReactorLoads = append(out.ReactorLoads, *l)
	}
	sort.Strings(out.CriticalBatches)
	sort.Strings(out.Warnings)
	sort.SliceStable(out.ReactorLoads, func(i, j int) bool { return out.ReactorLoads[i].ReactorID < out.ReactorLoads[j].ReactorID })
	return out
}

func ensureLoad(loads map[string]*model.ReactorLoad, id string, reactors map[string]model.Reactor) *model.ReactorLoad {
	if l := loads[id]; l != nil {
		return l
	}
	l := &model.ReactorLoad{ReactorID: id}
	if rc, ok := reactors[id]; ok {
		l.ReactorName, l.Status = rc.Name, rc.Status
	} else {
		l.Status = model.ReactorMaintenance
	}
	loads[id] = l
	return l
}

func countStatus(out *model.CampaignAnalytics, status model.BatchStatus) {
	switch status {
	case model.BatchQueued:
		out.QueuedCount++
	case model.BatchDone:
		out.DoneCount++
	case model.BatchFaulted:
		out.FaultCount++
	case model.BatchAborted:
		out.AbortedCount++
	default:
		out.ActiveCount++
	}
}

func countVerdict(out *model.CampaignAnalytics, verdict model.SafetyVerdict) {
	switch verdict {
	case model.VerdictSafe:
		out.SafeCount++
	case model.VerdictMarginal:
		out.MarginalCount++
	case model.VerdictRunaway:
		out.RunawayCount++
	}
}

func cloneForecasts(in []model.BatchForecast) []model.BatchForecast {
	out := make([]model.BatchForecast, len(in))
	for i, f := range in {
		out[i] = f
		out[i].Reasons = append([]string{}, f.Reasons...)
	}
	return out
}

func contains(items []string, item string) bool {
	for _, value := range items {
		if value == item {
			return true
		}
	}
	return false
}
