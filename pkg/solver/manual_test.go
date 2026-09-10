package solver

import (
	"strings"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestManualZeroEqualsCentreline(t *testing.T) {
	scene, _ := track.Preset("hairpin")
	car, _ := vehicle.Preset("road")
	road, err := track.SampleRoad(scene, .5)
	if err != nil {
		t.Fatal(err)
	}
	controls := []LineControl{{Index: 0}, {Index: len(road) / 2}, {Index: len(road) - 1}}
	offsets, err := ManualOffsets(road, controls, car.Width/2+.2)
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.Spacing = .5
	result, err := Evaluate(scene, car, offsets, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Duration != result.CenterDuration {
		t.Fatalf("zero manual time %g != centreline %g", result.Duration, result.CenterDuration)
	}
	controls[1].Offset = 100
	_, err = ManualOffsets(road, controls, car.Width/2+.2)
	if err == nil || !strings.Contains(err.Error(), "station") || !strings.Contains(err.Error(), "clearance") {
		t.Fatalf("missing useful infeasibility: %v", err)
	}
}

func TestManualConstantOffsetsAndBounds(t *testing.T) {
	road := []track.Sample{{S: 0, Width: 12}, {S: 25, Width: 12}, {S: 50, Width: 12}, {S: 75, Width: 12}, {S: 100, Width: 12}}
	offsets, err := ManualOffsets(road, []LineControl{{0, 2}, {2, 2}, {4, 2}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range offsets {
		if offset < 1.999999999 || offset > 2.000000001 {
			t.Fatalf("constant offset was not preserved: %g", offset)
		}
	}
}
