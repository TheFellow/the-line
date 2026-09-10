package track

import (
	"math"
	"testing"
)

func TestNaturalSplineAnalyticalArch(t *testing.T) {
	// Equal chords sqrt(2), natural endpoint second derivatives zero. On the
	// first span, y(t) = 3t/2 - t^3/2 and x(t) = t independently of the solver.
	s := newCenterSpline([]Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 0}})
	for _, u := range []float64{0, .25, .5, .75, 1} {
		p, d, dd := s.at(0, u)
		want := Vec3{X: u, Y: 1.5*u - .5*u*u*u}
		if p.Sub(want).Length() > 1e-12 || math.Abs(d.X-1/math.Sqrt2) > 1e-12 || math.Abs(dd.Y+1.5*u) > 1e-12 {
			t.Fatalf("t=%g: position %+v, derivative %+v, second %+v", u, p, d, dd)
		}
	}
}

func TestNaturalSplineContinuity(t *testing.T) {
	for _, name := range Presets() {
		scene, _ := Preset(name)
		s := newCenterSpline(scene.Points)
		for i := 0; i < len(scene.Points)-2; i++ {
			p, d, dd := s.at(i, 1)
			q, e, ee := s.at(i+1, 0)
			if p.Sub(q).Length() > 1e-12 || d.Sub(e).Length() > 1e-12 || dd.Sub(ee).Length() > 1e-12 {
				t.Fatalf("%s knot %d is not C2", name, i+1)
			}
		}
		_, _, first := s.at(0, 0)
		_, _, last := s.at(len(s.spans)-1, 1)
		if first.Length() != 0 || last.Length() != 0 {
			t.Fatalf("%s has non-natural endpoints", name)
		}
	}
}

func TestSplineStraightAndBoundedCrossSection(t *testing.T) {
	scene := Scene{Version: 1, Name: "linear grade", Points: []Point{
		{X: 0, Z: 0, Width: 8, Bank: -5, Surface: "asphalt"},
		{X: 50, Z: 5, Width: 16, Bank: 5, Surface: "wet"},
		{X: 150, Z: 15, Width: 12, Bank: -2, Surface: "gravel"},
	}}
	road, err := SampleRoad(scene, .25)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range road {
		if math.Abs(p.Position.Z-.1*p.Position.X) > 1e-12 || math.Abs(p.Grade-.1) > 1e-12 || p.Normal != (Vec3{Y: 1}) {
			t.Fatalf("straight grade changed: %+v", p)
		}
		if p.Width < 8 || p.Width > 16 || p.Bank < -5 || p.Bank > 5 {
			t.Fatalf("cross section overshot: %+v", p)
		}
		if p.Position.X == 50 {
			if p.Width != 16 || p.Bank != 5 || p.Surface != "wet" {
				t.Fatal("control point attributes changed")
			}
			// A C1 interpolant with zero knot slope converges to zero from each
			// side; bound finite-difference slopes on this quarter-metre mesh.
			for _, q := range []Sample{road[i-1], road[i+1]} {
				if math.Abs((q.Bank-p.Bank)/(q.S-p.S)) > .004 || math.Abs((q.Width-p.Width)/(q.S-p.S)) > .003 {
					t.Fatal("cross-section knot has a first-derivative step")
				}
			}
		}
	}
}

// Measured curvature uses only sampled positions and circumcircles, independent
// of the spline derivative implementation. C2 guarantees continuity, not a bound
// on the curvature gradient of arbitrary user-authored roads.
func TestPresetSampledCurvature(t *testing.T) {
	for _, name := range Presets() {
		t.Run(name, func(t *testing.T) {
			scene, _ := Preset(name)
			road, err := SampleRoad(scene, .5)
			if err != nil {
				t.Fatal(err)
			}
			curvature := make([]float64, len(road))
			for i := 1; i < len(road)-1; i++ {
				a, b, c := road[i-1].Position, road[i].Position, road[i+1].Position
				curvature[i] = 2 * cross(a, b, c) / (math.Hypot(b.X-a.X, b.Y-a.Y) * math.Hypot(c.X-b.X, c.Y-b.Y) * math.Hypot(c.X-a.X, c.Y-a.Y))
			}
			largest, nearKnot, station := 0., 0., 0.
			for i := 2; i < len(road)-1; i++ {
				jump := math.Abs(curvature[i] - curvature[i-1])
				if jump > largest {
					largest, station = jump, road[i].S
				}
				for _, p := range scene.Points[1 : len(scene.Points)-1] {
					if road[i].Position.Sub(p.Position()).Length() < .51 || road[i-1].Position.Sub(p.Position()).Length() < .51 {
						nearKnot = math.Max(nearKnot, jump)
					}
				}
			}
			t.Logf("largest adjacent curvature difference %.8f 1/m at %.2fm; within .5m of knots %.8f 1/m", largest, station, nearKnot)
			if largest > .0015 || nearKnot > .001 {
				t.Fatal("preset curvature exceeds declared mesh-gradient bounds")
			}
		})
	}
}
