package selfcheck

import (
	"strings"
	"testing"

	"task143-batchreactor/internal/forecast"
	"task143-batchreactor/internal/lifecycle"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/report"
)

func TestBug04_ConversionReleaseBoundaryRemainsVisible(t *testing.T) {
	b := model.Batch{ID: "short", Status: model.BatchDischarging, Conversion: .85}
	if _, err := lifecycle.NextStatus(b, model.BatchCleaning, &lifecycle.TransitionContext{MinConversion: .9}); err == nil { t.Error("short conversion was released") }
	f := forecast.Build(forecast.Input{Batch: b, Recipe: model.Recipe{MinConversion: .9, ThermalLimit: 500}, Reactor: model.Reactor{}, Result: &model.SafetyResult{Conversion: .85, PeakTemp: 400, Verdict: model.VerdictSafe, TMRSeconds: 999999}})
	if f.ConversionGap <= 0 || f.Risk != model.RiskWatch { t.Errorf("forecast hid short conversion: %+v", f) }
	r := report.Build(report.Input{Batches: []model.Batch{b}, Forecasts: []model.BatchForecast{f}})
	if !strings.Contains(strings.Join(r.Warnings, " "), "watch batch short") { t.Errorf("report hid watch warning: %+v", r.Warnings) }
}
