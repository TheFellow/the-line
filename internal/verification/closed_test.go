package verification

import (
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
	"testing"
)

func TestClosedSeamSurfaceTransition(t *testing.T) {
	s, _ := track.Preset("club-loop")
	// The low-grip sector begins exactly on the lap seam. Its braking demand
	// must propagate into the previous lap's final approach.
	s.Points[0].Surface = "ice"
	car, _ := vehicle.Preset("road")
	opts := solver.DefaultOptions()
	opts.Iterations = 0
	r, err := solver.Solve(s, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	a, b := r.Nodes[0], r.Nodes[len(r.Nodes)-1]
	if a.Position != b.Position || a.Speed != b.Speed || a.Curvature != b.Curvature || a.Forces != b.Forces {
		t.Fatal("periodic physical seam differs")
	}
	if r.Nodes[len(r.Nodes)-2].Acceleration >= 0 {
		t.Fatal("seam low-grip sector did not cause approach braking")
	}
	clearance := checkClearance(t, r, car.Width/2+opts.Margin)
	residual := checkForces(t, r, car)
	t.Logf("seam surface transition %.6fs clearance %.6fm max force excess %.3g", r.Duration, clearance, residual)
}
