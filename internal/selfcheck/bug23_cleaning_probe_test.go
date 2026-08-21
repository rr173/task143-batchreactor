package selfcheck

import (
	"testing"

	"task143-batchreactor/internal/model"
)

func TestBug23_CleaningChangeoverContract23(t *testing.T) {
	if got := model.CleaningLight.CleaningDuration(); got != 600 {
		t.Fatalf("light-product changeover duration = %v, want 600", got)
	}
}
