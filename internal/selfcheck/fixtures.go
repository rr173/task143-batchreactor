package selfcheck

import "task143-batchreactor/internal/model"

// exothermicRecipe is a moderate, well-behaved first-order exotherm whose
// adiabatic temperature rise is ~40 K (Stoessel class 2) and whose jacket
// cooling keeps the trajectory peak well below the thermal limit. It reaches
// near-full conversion in its 1 h duration, so the lifecycle completes and the
// conversion gate passes. This is the "safe" workhorse recipe.
//
// ΔT_ad = (-ΔH)·CA0/(ρ·Cp) = 60000·2000/(1000·3000) = 40 K.
// k(T0)=K0·exp(-Ea/(R·T0)) ≈ 8.3e-4 1/s → 1-exp(-k·3600) ≈ 0.95+ with the
// modest self-heat (peak ≈ 343 K ≪ 450 K thermal limit).
func exothermicRecipe() model.Recipe {
	return model.Recipe{
		Name: "ester A→P", Product: "P1", K0: 2.1e6, Ea: 60000, Order: model.OrderFirst,
		CA0: 2000, DeltaHrx: -60000, Rho: 1000, Cp: 3000, T0: 333.15, JacketTemp: 333.15,
		ThermalLimit: 450.0, MinConversion: 0.95, Duration: 3600,
	}
}

// adiabaticRecipe is a separate product used to exercise sequence-dependent
// cleaning; its parameters are mild and safe.
func adiabaticRecipe() model.Recipe {
	return model.Recipe{
		Name: "second product", Product: "P2", K0: 1.0e6, Ea: 58000, Order: model.OrderFirst,
		CA0: 1500, DeltaHrx: -40000, Rho: 1000, Cp: 3000, T0: 323.15, JacketTemp: 323.15,
		ThermalLimit: 440.0, MinConversion: 0.90, Duration: 3600,
	}
}

// runawayRecipe has ΔT_ad ≥ 200 K (Stoessel class 5) and kinetics fast enough
// that the trajectory peak blows past the thermal limit, so the reacting stage
// must fault the batch and forbid discharge.
//
// ΔT_ad = 300000·2000/(1000·3000) = 200 K → class 5.
func runawayRecipe() model.Recipe {
	return model.Recipe{
		Name: "hot epoxide", Product: "RH", K0: 1.0e7, Ea: 60000, Order: model.OrderFirst,
		CA0: 2000, DeltaHrx: -300000, Rho: 1000, Cp: 3000, T0: 343.15, JacketTemp: 343.15,
		ThermalLimit: 420.0, MinConversion: 0.90, Duration: 1200,
	}
}

// shortConversionRecipe completes reaction well short of spec: a slow rate and
// a very short duration give conversion ≈ 0.3 ≪ 0.80. ΔT_ad is small and the
// trajectory stays mild, so the verdict is safe — only the conversion gate
// rejects the discharging→cleaning step.
func shortConversionRecipe() model.Recipe {
	return model.Recipe{
		Name: "slow amide", Product: "P2", K0: 1.2e7, Ea: 60000, Order: model.OrderFirst,
		CA0: 2000, DeltaHrx: -20000, Rho: 1000, Cp: 3000, T0: 313.15, JacketTemp: 313.15,
		ThermalLimit: 450.0, MinConversion: 0.80, Duration: 300,
	}
}

// safeReactor is a generously-cooled, generously-rated vessel compatible with
// the exothermic and adiabatic recipes.
func safeReactor() model.Reactor {
	return model.Reactor{
		Name: "R-1", Volume: 2.0, HeatTransferU: 2000, HeatTransferArea: 10.0,
		MaxOperatingTemp: 600.0, MaxPressure: 12.0, Material: "SS316L",
	}
}

// tightReactor has a low max operating temperature so the exothermic recipe's
// MTSR (≈ 373 K) exceeds it (340 K) → planning must reject with
// ErrThermalIncompatible.
func tightReactor() model.Reactor {
	return model.Reactor{
		Name: "R-tight", Volume: 1.0, HeatTransferU: 2000, HeatTransferArea: 5.0,
		MaxOperatingTemp: 340.0, MaxPressure: 8.0, Material: "Glass-lined",
	}
}
