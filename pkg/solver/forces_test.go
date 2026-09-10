package solver

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestExportedForcesAndInterpolatedQueries(t *testing.T) {
	for _, name := range []string{"hairpin", "banked", "rally"} {
		t.Run(name, func(t *testing.T) {
			s, c := fixture(t, name)
			opts := DefaultOptions()
			opts.Iterations = 1
			r, err := Solve(s, c, opts)
			if err != nil {
				t.Fatal(err)
			}
			for _, nodes := range [][]Node{r.Nodes, r.CenterNodes} {
				for i, n := range nodes {
					j := min(i, len(nodes)-2)
					a, b := nodes[j], nodes[j+1]
					grade := (b.Position.Z - a.Position.Z) / math.Hypot(b.Position.X-a.Position.X, b.Position.Y-a.Position.Y)
					accel := (b.Speed*b.Speed - a.Speed*a.Speed) / (2 * (b.S - a.S))
					if math.Abs(n.Grade-grade) > 1e-12 || math.Abs(n.Acceleration-accel) > 1e-9 {
						t.Fatal("channels do not use outgoing segment; terminal must use incoming")
					}
					checkForceSample(t, n, c)
				}
			}
			for i := 1; i < 30; i++ {
				n := r.At(float64(i) * r.Duration / 30)
				checkForceSample(t, n, c)
				same, err := r.AtStation(n.Station)
				if err != nil {
					t.Fatal(err)
				}
				if math.Abs(same.Forces.Utilization-n.Forces.Utilization) > 1e-8 {
					t.Fatal("force dot and station chart disagree")
				}
				checkForceSample(t, r.CenterAt(float64(i)*r.CenterDuration/30), c)
			}
		})
	}
}

func checkForceSample(t *testing.T, n Node, c vehicle.Config) {
	t.Helper()
	e := c.Limits(n.Speed, n.Curvature, n.Bank, n.Grade, n.Grip)
	if !n.Forces.Available {
		t.Fatal("missing force channels")
	}
	if math.Abs(n.Forces.Lateral-e.Tyres.Lateral) > 1e-9 || math.Abs(n.Forces.Capacity-e.Tyres.Capacity) > 1e-9 {
		t.Fatal("channels differ from envelope")
	}
	resistance := .5*vehicle.AirDensity*c.DragArea*n.Speed*n.Speed/c.Mass + vehicle.Gravity*n.Grade/math.Sqrt(1+n.Grade*n.Grade)
	if math.Abs(n.Forces.Longitudinal-n.Acceleration-resistance) > 1e-9 {
		t.Fatal("longitudinal channel violates force balance")
	}
	if n.Forces.Utilization > 1+2e-5 || math.IsNaN(n.Forces.Utilization) {
		t.Fatalf("invalid combined utilization %g", n.Forces.Utilization)
	}
}

func TestMarkerRefinementAndStraightAbsence(t *testing.T) {
	s, c := fixture(t, "hairpin")
	opts := DefaultOptions()
	opts.Iterations = 1
	r, err := Solve(s, c, opts)
	if err != nil {
		t.Fatal(err)
	}
	coarse := DetectMarkers(r.Nodes)
	if len(coarse) < 2 {
		t.Fatalf("hairpin lacks apex/drive markers: %+v", coarse)
	}
	fineRoad, err := track.SampleRoad(s, .25)
	if err != nil {
		t.Fatal(err)
	}
	offsets := interpolateOffsets(r.Road, r.Offsets, fineRoad, c.Width/2+opts.Margin)
	opts.Spacing = .25
	fine, err := Evaluate(s, c, offsets, opts)
	if err != nil {
		t.Fatal(err)
	}
	fineMarkers := DetectMarkers(fine.Nodes)
	for _, a := range coarse {
		nearest := math.Inf(1)
		for _, b := range fineMarkers {
			if a.Kind == b.Kind {
				nearest = math.Min(nearest, math.Abs(a.Station-b.Station))
			}
		}
		if nearest > r.Spacing+1e-6 {
			t.Fatalf("%s marker shifted %g m under refinement", a.Kind, nearest)
		}
	}
	s = straight()
	opts.Iterations = 0
	r, err = Solve(s, c, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := DetectMarkers(r.Nodes); len(got) != 0 {
		t.Fatalf("straight got corner markers: %+v", got)
	}
}
