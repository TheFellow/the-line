package solver

import (
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
	"math"
	"testing"
)

func lapCircle() track.Scene {
	s := track.Scene{Version: track.Version, Name: "circle", Closed: true, EntrySpeed: 0, ExitSpeed: 0}
	for i := 0; i < 12; i++ {
		a := 2 * math.Pi * float64(i) / 12
		s.Points = append(s.Points, track.Point{X: 80 * math.Cos(a), Y: 80 * math.Sin(a), Width: 10, Surface: "asphalt"})
	}
	return s
}
func TestClosedCirclePeriodicProfile(t *testing.T) {
	s := lapCircle()
	car, _ := vehicle.Preset("road")
	opts := DefaultOptions()
	opts.Iterations = 0
	r, err := Solve(s, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Closed {
		t.Fatal("missing closed result")
	}
	a, b := r.Nodes[0], r.Nodes[len(r.Nodes)-1]
	b.S, b.Station, b.Time = 0, 0, 0
	if a != b {
		t.Fatalf("seam state differs: %#v %#v", a, b)
	}
	if a.Speed < 10 {
		t.Fatal("open zero speed caps applied to closed lap")
	}
	for _, n := range r.Nodes {
		if math.Abs(n.Curvature-1.0/80) > .00035 {
			t.Fatalf("circle curvature %g differs from oracle", n.Curvature)
		}
	}
	if r.LapAt(r.Duration+.3).Position.Sub(r.At(.3).Position).Length() > 1e-9 {
		t.Fatal("lap does not wrap")
	}
	rotated := s
	rotated.Points = append(append([]track.Point(nil), s.Points[3:]...), s.Points[:3]...)
	q, err := Solve(rotated, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(q.Duration-r.Duration) > 1e-7 {
		t.Fatalf("circle start changes lap time: %.12f %.12f", r.Duration, q.Duration)
	}
}
func TestClosedOffsetsAndRefinement(t *testing.T) {
	s, _ := track.Preset("club-loop")
	car, _ := vehicle.Preset("gt")
	opts := DefaultOptions()
	opts.Iterations = 1
	r, err := Solve(s, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	if r.Duration > r.CenterDuration {
		t.Fatal("regressed baseline")
	}
	q, err := Evaluate(s, car, r.Offsets, Options{Spacing: r.Spacing, Margin: opts.Margin})
	if err != nil {
		t.Fatal(err)
	}
	if r.Duration != q.Duration {
		t.Fatalf("reevaluation changed duration %.12f %.12f", r.Duration, q.Duration)
	}
	fine, err := track.SampleRoad(s, .25)
	if err != nil {
		t.Fatal(err)
	}
	offsets := interpolateOffsets(r.Road, r.Offsets, fine, car.Width/2+opts.Margin)
	q, err = Evaluate(s, car, offsets, Options{Spacing: .25, Margin: opts.Margin})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(q.Duration/r.Duration-1) > .01 {
		t.Fatalf("lap refinement >1%%: %g %g", r.Duration, q.Duration)
	}
	t.Logf("lap %.6f reference %.6f refined %.6f", r.Duration, r.CenterDuration, q.Duration)
	bad := append([]float64(nil), r.Offsets...)
	bad[len(bad)-1] += .1
	if _, err := Evaluate(s, car, bad, Options{Spacing: r.Spacing, Margin: opts.Margin}); err == nil {
		t.Fatal("nonperiodic seam accepted")
	}
}

func TestClosedStartStationInvariance(t *testing.T) {
	s, _ := track.Preset("club-loop")
	car, _ := vehicle.Preset("gt")
	opts := DefaultOptions()
	opts.Iterations = 1
	first, err := Solve(s, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, shift := range []int{2, 5} {
		rotated := s
		rotated.Points = append(append([]track.Point(nil), s.Points[shift:]...), s.Points[:shift]...)
		r, err := Solve(rotated, car, opts)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(r.CenterDuration-first.CenterDuration) > 1e-7 {
			t.Fatal("baseline depends on start station")
		}
		if math.Abs(r.Duration/first.Duration-1) > .01 {
			t.Fatal("heuristic rotated lap exceeds 1% refinement tolerance")
		}
	}
}
func TestClosedManualControls(t *testing.T) {
	road, err := track.SampleRoad(lapCircle(), 3)
	if err != nil {
		t.Fatal(err)
	}
	controls := []LineControl{{Index: 0, Offset: 1}, {Index: len(road) / 3, Offset: -1}, {Index: 2 * len(road) / 3, Offset: 2}, {Index: len(road) - 1, Offset: 1}}
	offsets, err := ManualOffsets(road, controls, 1.2)
	if err != nil {
		t.Fatal(err)
	}
	if offsets[0] != offsets[len(offsets)-1] {
		t.Fatal("manual seam opens")
	}
	controls[len(controls)-1].Offset = 0
	if _, err := ManualOffsets(road, controls, 1.2); err == nil {
		t.Fatal("mismatched manual seam accepted")
	}
}

func TestClosedConstantSpeedOracle(t *testing.T) {
	s := lapCircle()
	car, _ := vehicle.Preset("road")
	car.MaxSpeed = 25
	r, err := Solve(s, constantModel{car, 2, 4}, Options{Spacing: .5, Margin: .25})
	if err != nil {
		t.Fatal(err)
	}
	// Constant positive drive and brake and a uniform speed ceiling imply that
	// every point of a steady lap can attain that ceiling, regardless of seam.
	if math.Abs(r.Duration-r.Length/car.MaxSpeed) > 1e-5 {
		t.Fatalf("constant-speed lap: got %g want %g", r.Duration, r.Length/car.MaxSpeed)
	}
}
