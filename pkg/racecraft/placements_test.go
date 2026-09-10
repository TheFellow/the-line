package racecraft

import (
	"strings"
	"testing"
)

func TestPlacementScenarios(t *testing.T) {
	for _, name := range Scenarios() {
		for car := 0; car < 2; car++ {
			for _, variant := range []float64{0, .25, .5} {
				controls, intent, err := placements(DefaultConfig(name), car, variant, 101)
				if err != nil || len(controls) != 7 || intent == "" {
					t.Fatalf("%s car %d variant %g: controls=%v intent=%q err=%v", name, car, variant, controls, intent, err)
				}
			}
		}
	}
	if _, _, err := placements(DefaultConfig("unimplemented"), 0, 0, 101); err == nil || !strings.Contains(err.Error(), "unknown racecraft placements") {
		t.Fatalf("unknown placement: %v", err)
	}
}
