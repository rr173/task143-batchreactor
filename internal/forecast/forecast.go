// Package forecast derives deterministic, operator-facing batch forecasts from
// persisted lifecycle state and the same thermal inputs used by the kinetics
// engine. It does not write state or read the clock itself: callers pass an
// observation time, making a restart report reproducible and testable.
package forecast

import (
	"math"
	"sort"

	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/thermal"
)

// Input is the complete read snapshot needed to forecast one batch. Result may
// be nil before a batch leaves the reacting stage; in that case the recipe's
// deterministic thermal classification still supplies a conservative preview.
type Input struct {
	Batch   model.Batch
	Recipe  model.Recipe
	Reactor model.Reactor
	Result  *model.SafetyResult
	Now     int64
}

// Build returns a single operational forecast. The calculation never claims a
// physical completion time for a faulted or completed batch: terminal batches
// retain their recorded end timestamp and report a terminal phase instead.
func Build(in Input) model.BatchForecast {
	f := model.BatchForecast{
		BatchID: in.Batch.ID, ReactorID: in.Batch.ReactorID, RecipeID: in.Batch.RecipeID,
		Status: in.Batch.Status, PlannedStart: in.Batch.PlannedStart,
		EstimatedStart: estimatedStart(in.Batch), Reasons: []string{},
	}

	res := resultOrPreview(in)
	f.TMRSeconds = res.TMRSeconds
	f.ThermalHeadroom = headroom(in.Recipe, res.PeakTemp)
	f.ConversionGap = conversionGap(in.Recipe, res.Conversion)
	f.Phase = phaseFor(in.Batch.Status)
	f.EstimatedFinish = estimatedFinish(in, f.Phase)
	f.ScheduleSlip = scheduleSlip(in, f.EstimatedStart)
	f.Risk, f.Reasons = riskFor(in, res, f)
	f.NextAction = nextAction(in, f)
	return f
}

// BuildAll builds a stable, reactor/sequence ordered forecast list. A map is
// used for recipes and reactors because a campaign frequently reuses both; a
// missing reference is represented as a critical forecast rather than causing
// all other batches to disappear from the operations report.
func BuildAll(batches []model.Batch, recipes map[string]model.Recipe, reactors map[string]model.Reactor, results map[string]model.SafetyResult, now int64) []model.BatchForecast {
	out := make([]model.BatchForecast, 0, len(batches))
	for _, b := range batches {
		r, recipeOK := recipes[b.RecipeID]
		rc, reactorOK := reactors[b.ReactorID]
		if !recipeOK || !reactorOK {
			f := model.BatchForecast{BatchID: b.ID, ReactorID: b.ReactorID, RecipeID: b.RecipeID, Status: b.Status, PlannedStart: b.PlannedStart, EstimatedStart: estimatedStart(b), Phase: phaseFor(b.Status), Risk: model.RiskCritical, Reasons: []string{"batch references a missing recipe or reactor"}, NextAction: "hold batch and repair the campaign reference"}
			out = append(out, f)
			continue
		}
		var result *model.SafetyResult
		if v, ok := results[b.ID]; ok {
			copy := v
			result = &copy
		}
		out = append(out, Build(Input{Batch: b, Recipe: r, Reactor: rc, Result: result, Now: now}))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ReactorID != out[j].ReactorID {
			return out[i].ReactorID < out[j].ReactorID
		}
		if out[i].EstimatedStart != out[j].EstimatedStart {
			return out[i].EstimatedStart < out[j].EstimatedStart
		}
		return out[i].BatchID < out[j].BatchID
	})
	return out
}

func resultOrPreview(in Input) model.KineticsResult {
	if in.Result != nil {
		return model.KineticsResult{Conversion: in.Result.Conversion, PeakTemp: in.Result.PeakTemp, DeltaTad: in.Result.DeltaTad, MTSR: in.Result.MTSR, StoesselClass: in.Result.StoesselClass, TMRSeconds: in.Result.TMRSeconds, Verdict: in.Result.Verdict}
	}
	res, err := thermal.Classify(in.Recipe, in.Reactor)
	if err != nil {
		return model.KineticsResult{PeakTemp: in.Recipe.T0, TMRSeconds: math.Inf(1), Verdict: model.VerdictRunaway, StoesselClass: 5}
	}
	return res
}

func phaseFor(status model.BatchStatus) model.ForecastPhase {
	switch status {
	case model.BatchQueued:
		return model.ForecastWaiting
	case model.BatchCharging:
		return model.ForecastCharge
	case model.BatchReacting:
		return model.ForecastReact
	case model.BatchCooling:
		return model.ForecastCool
	case model.BatchDischarging:
		return model.ForecastDischarge
	case model.BatchCleaning:
		return model.ForecastCleaning
	default:
		return model.ForecastTerminal
	}
}

func estimatedStart(b model.Batch) int64 {
	if b.StartedAt > 0 {
		return b.StartedAt
	}
	return b.PlannedStart
}

func estimatedFinish(in Input, phase model.ForecastPhase) int64 {
	b := in.Batch
	if b.Status.IsTerminal() {
		return b.EndedAt
	}
	start := estimatedStart(b)
	if start == 0 {
		start = in.Now
	}
	duration := int64(math.Ceil(in.Recipe.Duration))
	if duration < 1 {
		duration = 1
	}
	switch phase {
	case model.ForecastWaiting:
		return start + duration
	case model.ForecastCharge:
		return max64(in.Now, start) + duration
	case model.ForecastReact:
		return max64(in.Now, start+duration/2) + duration/2
	case model.ForecastCool:
		return max64(in.Now, start+duration) + max64(60, duration/10)
	case model.ForecastDischarge:
		return max64(in.Now, start+duration) + 60
	case model.ForecastCleaning:
		return max64(in.Now, start) + 600
	default:
		return b.EndedAt
	}
}

func scheduleSlip(in Input, start int64) int64 {
	if in.Batch.PlannedStart <= 0 || start <= in.Batch.PlannedStart {
		return 0
	}
	return start - in.Batch.PlannedStart
}

func headroom(recipe model.Recipe, peak float64) float64 {
	if recipe.ThermalLimit <= 0 || peak <= 0 {
		return 0
	}
	return recipe.ThermalLimit - peak
}

func conversionGap(recipe model.Recipe, actual float64) float64 {
	if recipe.MinConversion <= actual {
		return 0
	}
	return recipe.MinConversion - actual
}

func riskFor(in Input, res model.KineticsResult, f model.BatchForecast) (model.RiskBand, []string) {
	reasons := make([]string, 0, 4)
	if in.Batch.Status == model.BatchFaulted {
		return model.RiskCritical, []string{"batch is faulted: " + in.Batch.FaultReason}
	}
	if res.Verdict == model.VerdictRunaway {
		reasons = append(reasons, "thermal classification is runaway")
	}
	if res.StoesselClass >= 5 {
		reasons = append(reasons, "Stoessel class 5 requires escalation")
	}
	if f.ThermalHeadroom <= 0 && in.Recipe.ThermalLimit > 0 {
		reasons = append(reasons, "predicted peak reaches the thermal limit")
	}
	if !math.IsInf(res.TMRSeconds, 1) && res.TMRSeconds >= 0 && res.TMRSeconds < 8*3600 {
		reasons = append(reasons, "TMR is below the eight-hour response window")
	}
	if len(reasons) > 0 {
		return model.RiskCritical, reasons
	}
	if res.Verdict == model.VerdictMarginal {
		reasons = append(reasons, "thermal classification is marginal")
	}
	if f.ThermalHeadroom > 0 && f.ThermalHeadroom < 10 {
		reasons = append(reasons, "thermal headroom is below 10 K")
	}
	if f.ConversionGap > 0 {
		reasons = append(reasons, "conversion remains below the release specification")
	}
	if f.ScheduleSlip > 0 {
		reasons = append(reasons, "batch started after its planned slot")
	}
	if len(reasons) > 0 {
		return model.RiskWatch, reasons
	}
	return model.RiskNormal, reasons
}

func nextAction(in Input, f model.BatchForecast) string {
	if f.Risk == model.RiskCritical {
		if in.Batch.Status.IsTerminal() {
			return "review the fault record before releasing the reactor"
		}
		return "hold the next transition and obtain a process-safety review"
	}
	switch in.Batch.Status {
	case model.BatchQueued:
		return "verify feed readiness before charging"
	case model.BatchCharging:
		return "complete charging and begin the controlled reaction"
	case model.BatchReacting:
		return "monitor peak temperature and prepare cooling"
	case model.BatchCooling:
		return "confirm cooling before opening the discharge step"
	case model.BatchDischarging:
		if f.ConversionGap > 0 {
			return "do not complete discharge; rework or abort the batch"
		}
		return "complete discharge and record the release"
	case model.BatchCleaning:
		return "complete cleaning before the next product"
	case model.BatchDone:
		return "release the reactor for the next scheduled batch"
	case model.BatchAborted:
		return "review abort cause and return reactor to a safe state"
	default:
		return "no lifecycle action is pending"
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
