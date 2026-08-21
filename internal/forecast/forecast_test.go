package forecast

import (
	"math"
	"testing"

	"task143-batchreactor/internal/model"
)

// TestConversionGapBelowSpec reports the shortfall when conversion is strictly
// below the release spec.
func TestConversionGapBelowSpec(t *testing.T) {
	r := model.Recipe{MinConversion: 0.95}
	if got := conversionGap(r, 0.80); math.Abs(got-0.15) > 1e-9 {
		t.Fatalf("conversionGap below spec: got %v want 0.15", got)
	}
}

// TestConversionGapAtSpec is zero when conversion lands exactly on the spec.
// The release spec is a floor, so meeting it exactly is a pass — reporting a
// gap would manufacture a phantom quality shortfall for an on-spec batch.
func TestConversionGapAtSpec(t *testing.T) {
	r := model.Recipe{MinConversion: 0.95}
	if got := conversionGap(r, 0.95); got != 0 {
		t.Fatalf("conversionGap at spec must be 0: got %v", got)
	}
}

// TestConversionGapAboveSpec is zero when conversion exceeds the spec.
func TestConversionGapAboveSpec(t *testing.T) {
	r := model.Recipe{MinConversion: 0.95}
	if got := conversionGap(r, 0.99); got != 0 {
		t.Fatalf("conversionGap above spec must be 0: got %v", got)
	}
}
