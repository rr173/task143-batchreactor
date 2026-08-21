package selfcheck

import (
	"testing"

	"task143-batchreactor/internal/model"
)

func TestBug29_CleaningChangeoverContract29(t *testing.T) {
	if got := model.CleaningLight.CleaningDuration(); got != 600 {
		t.Fatalf("light-product changeover duration = %v, want 600", got)
	}
}
