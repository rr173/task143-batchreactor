package selfcheck

import (
	"testing"

	"task143-batchreactor/internal/kinetics"
	"task143-batchreactor/internal/model"
	"task143-batchreactor/internal/thermal"
	"task143-batchreactor/internal/units"
)

func TestBug06_DefaultIntegrationStepIsShared(t *testing.T) {
	if units.DefaultDt != 1 { t.Errorf("default step = %v", units.DefaultDt) }
	r := model.Recipe{K0: .2, Ea: 1, Order: model.OrderFirst, CA0: 1, DeltaHrx: -10, Rho: 1000, Cp: 1000, T0: 300, JacketTemp: 300, Duration: 4}
	rc := model.Reactor{Volume: 1, HeatTransferArea: 1, HeatTransferU: 1}
	out, err := kinetics.Integrate(r, rc, r.Duration, 0)
	if err != nil || out.Steps != 4 { t.Errorf("default integration = %+v, %v", out, err) }
	res, err := thermal.Classify(r, rc)
	if err != nil || res.Steps != out.Steps { t.Errorf("thermal integration steps = %d, bare = %d, %v", res.Steps, out.Steps, err) }
}
