package racecraft

import (
	"math"
	"strings"

	"github.com/TheFellow/the-line/pkg/track"
	"testing"
)

func TestPlacementScenarios(t *testing.T) {
	road := make([]track.Sample, 101)
	for i := range road {
		road[i].S = float64(i) * 2
	}
	for _, name := range Scenarios() {
		for car := 0; car < 2; car++ {
			for _, variant := range []float64{0, .25, .5} {
				controls, intent, err := placements(DefaultConfig(name), car, variant, road)
				if err != nil || len(controls) != 7 || intent == "" {
					t.Fatalf("%s car %d variant %g: controls=%v intent=%q err=%v", name, car, variant, controls, intent, err)
				}
			}
		}
	}
	if _, _, err := placements(DefaultConfig("unimplemented"), 0, 0, road); err == nil || !strings.Contains(err.Error(), "unknown racecraft placements") {
		t.Fatalf("unknown placement: %v", err)
	}
}

func TestPlacementsUseRoadStation(t *testing.T) {
	// Many short source spans retain dense samples at the start of the road;
	// fractions of the sample count would shift tactical points toward that end.
	scene := track.Scene{Version: 1, Name: "Nonuniform stations", EntrySpeed: 20, ExitSpeed: 30}
	for x := 0.; x <= 30; x++ {
		scene.Points = append(scene.Points, track.Point{X: x, Width: 20, Surface: "asphalt"})
	}
	scene.Points = append(scene.Points, track.Point{X: 203, Width: 20, Surface: "asphalt"})
	road, err := track.SampleRoad(scene, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		car       int
		variant   float64
		fractions []float64
	}{
		{"over-under", 0, 0, []float64{0, .25, .4, .60, .70, .82, 1}},
		{"over-under", 1, 0, []float64{0, .25, .38, .46, .72, .86, 1}},
		{"over-under", 1, .25, []float64{0, .25, .38, .48, .74, .86, 1}},
		{"over-under", 1, .5, []float64{0, .25, .38, .50, .76, .86, 1}},
		{"pass-repass", 1, .5, []float64{0, .25, .4, .64, .74, .82, 1}},
		{"defend", 0, 0, []float64{0, .25, .4, .52, .64, .78, 1}},
	} {
		controls, _, err := placements(DefaultConfig(tc.name), tc.car, tc.variant, road)
		if err != nil {
			t.Fatal(err)
		}
		for i, control := range controls {
			target := tc.fractions[i] * road[len(road)-1].S
			delta := math.Abs(road[control.Index].S - target)
			if delta > 1.01 {
				t.Errorf("%s car%d variant%g control%d at %g, target %g", tc.name, tc.car, tc.variant, i, road[control.Index].S, target)
			}
			for _, sample := range road {
				if math.Abs(sample.S-target) < delta-1e-9 {
					t.Errorf("%s car%d variant%g control%d is not nearest target station", tc.name, tc.car, tc.variant, i)
					break
				}
			}
		}
	}
}
