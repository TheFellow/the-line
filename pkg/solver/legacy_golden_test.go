package solver

import (
	"context"
	"math"
	"testing"
)

// These values were independently captured by running the five original
// fixtures from an isolated git archive of 6620b93, before the once-over fixes,
// with each scene's default vehicle and DefaultOptions. They protect actual
// legacy coarse-stage results, not just the optional-dynamics feature gate.
// The default solver now improves this starting line at final resolution;
// keeping these exact values checks that this search change did not alter the
// vehicle physics or the original objective. Do not regenerate these goldens.
func TestLegacyPresetDurations(t *testing.T) {
	for _, tt := range []struct {
		name             string
		duration, centre uint64
	}{
		{"hairpin", 0x402991b4b8ea0fcf, 0x402a62a7df77ef3d},
		{"esses", 0x402c7b3604b07805, 0x402fb281486db94a},
		{"compound", 0x402affb68999c00c, 0x402c7e6954d8e00c},
		{"banked", 0x40242e5471418190, 0x4024b20b06f0ec17},
		{"rally", 0x40357413e345e49f, 0x403695dd182907c5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scene, car := fixture(t, tt.name)
			result, err := solveCoarseContext(context.Background(), scene, car, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			if math.Float64bits(result.Duration) != tt.duration {
				t.Errorf("optimized duration %.17g (0x%x), original %.17g (0x%x)", result.Duration, math.Float64bits(result.Duration), math.Float64frombits(tt.duration), tt.duration)
			}
			if math.Float64bits(result.CenterDuration) != tt.centre {
				t.Errorf("centreline duration %.17g (0x%x), original %.17g (0x%x)", result.CenterDuration, math.Float64bits(result.CenterDuration), math.Float64frombits(tt.centre), tt.centre)
			}
		})
	}
}
