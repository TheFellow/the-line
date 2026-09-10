package solver_test

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestTaperedAsphaltChargesKerbUnderCircularFootprint(t *testing.T) {
	car, _ := vehicle.Preset("road")
	car.Width = 2
	scene := track.Scene{Version: 2, Name: "Tapered asphalt", EntrySpeed: 1, ExitSpeed: 1, KerbsCountAsRoad: true}
	for i, x := range []float64{0, 10, 60} {
		left := 2.0
		if i == 0 {
			left = 8
		}
		scene.Points = append(scene.Points, track.Point{X: x, WidthLeft: left, WidthRight: 6, Surface: "asphalt", KerbLeft: track.Kerb{Width: 2, Surface: "ice"}})
	}
	opts := solver.DefaultOptions()
	opts.Spacing = .5
	road, err := track.SampleRoad(scene, opts.Spacing)
	if err != nil {
		t.Fatal(err)
	}
	offsets := make([]float64, len(road))
	for i, sample := range road {
		// Every normal cross-section has more than the 1.25 m safety
		// clearance, yet the taper brings asphalt's sloping edge closer.
		offsets[i] = sample.LeftWidth() - 1.26
		if sample.GripAcross(offsets[i], car.Width/2+opts.Margin) != 1 {
			t.Fatal("fixture must evade the old cross-section-only check")
		}
	}
	result, err := solver.Evaluate(scene, car, offsets, opts)
	if err != nil {
		t.Fatal(err)
	}
	contacts := 0
	for _, node := range result.Nodes {
		// This straight centreline makes the asphalt edge's equation y=m*x+b.
		// Use signed line distance only when its perpendicular foot lies on
		// the segment, independently of production's segment-contact routine.
		for i := 0; i < len(road)-1; i++ {
			a, b := road[i], road[i+1]
			m := (b.LeftWidth() - a.LeftWidth()) / (b.Position.X - a.Position.X)
			intercept := a.LeftWidth() - m*a.Position.X
			xFoot := (node.Position.X + m*(node.Position.Y-intercept)) / (1 + m*m)
			gap := math.Abs(m*node.Position.X-node.Position.Y+intercept) / math.Sqrt(1+m*m)
			if xFoot > a.Position.X && xFoot < b.Position.X && gap < car.Width/2 {
				contacts++
				if node.Grip != .16 {
					t.Fatalf("station %.3f: body touches ice kerb at %.6f m, exported grip %g", node.Station, gap, node.Grip)
				}
			}
		}
	}
	if contacts == 0 {
		t.Fatal("fixture never touched kerb with physical vehicle radius")
	}
	// A decorative kerb excluded from the legal road must not alter the old
	// asphalt-only evaluation or its geometry.
	scene.KerbsCountAsRoad = false
	excluded, err := solver.Evaluate(scene, car, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	for i := range scene.Points {
		scene.Points[i].KerbLeft = track.Kerb{}
	}
	bare, err := solver.Evaluate(scene, car, nil, opts)
	if err != nil {
		t.Fatal(err)
	}
	if excluded.Duration != bare.Duration {
		t.Fatalf("excluded kerbs changed time: %.15g vs %.15g", excluded.Duration, bare.Duration)
	}
	for i, node := range excluded.Nodes {
		if node != bare.Nodes[i] {
			t.Fatalf("excluded kerb changed trajectory at node %d", i)
		}
	}
}
