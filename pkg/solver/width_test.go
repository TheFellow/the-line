package solver

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestAsymmetricClearanceAndKerbGrip(t *testing.T) {
	car, _ := vehicle.Preset("road")
	scene := track.Scene{Version: 2, Name: "Asymmetric straight", EntrySpeed: 10, ExitSpeed: 30, Points: []track.Point{
		{WidthLeft: 3, WidthRight: 7, Surface: "asphalt", KerbLeft: track.Kerb{Width: 3, Grip: .5}},
		{X: 150, WidthLeft: 3, WidthRight: 7, Surface: "asphalt", KerbLeft: track.Kerb{Width: 3, Grip: .5}},
	}}
	opts := DefaultOptions()
	opts.Spacing = .5
	opts.Iterations = 0
	road, err := track.SampleRoad(scene, opts.Spacing)
	if err != nil {
		t.Fatal(err)
	}
	offsets := make([]float64, len(road))
	for i := range offsets {
		offsets[i] = -5
	}
	right, err := Evaluate(scene, car, offsets, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range right.Nodes {
		if math.Abs(n.Position.Y+5) > 1e-9 {
			t.Fatal("asymmetric right line moved")
		}
	}
	for i := range offsets {
		offsets[i] = 4
	}
	if _, err = Evaluate(scene, car, offsets, opts); err == nil {
		t.Fatal("excluded kerb permitted")
	}
	scene.KerbsCountAsRoad = true
	kerb, err := Evaluate(scene, car, offsets, opts)
	if err != nil {
		t.Fatal(err)
	}
	base := make([]float64, len(road))
	asphalt, err := Evaluate(scene, car, base, opts)
	if err != nil {
		t.Fatal(err)
	}
	if kerb.Duration <= asphalt.Duration {
		t.Fatalf("lower-grip kerb did not reduce acceleration: %g vs %g", kerb.Duration, asphalt.Duration)
	}
	for _, n := range kerb.Nodes {
		// Independent planar boundary check: straight legal edges at y=-7, y=6.
		if 6-n.Position.Y < car.Width/2+opts.Margin-1e-8 || n.Position.Y+7 < car.Width/2+opts.Margin-1e-8 {
			t.Fatal("edge clearance violated")
		}
	}
}

func TestManualZeroLineOnVaryingAsymmetricWidths(t *testing.T) {
	road := []track.Sample{{S: 0, WidthLeft: 3, WidthRight: 7}, {S: 25, WidthLeft: 4, WidthRight: 6}, {S: 50, WidthLeft: 7, WidthRight: 3}}
	offsets, err := ManualOffsets(road, []LineControl{{Index: 0}, {Index: 2}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range offsets {
		if o != 0 {
			t.Fatal("zero manual line left centreline")
		}
	}
}
