package racecraft

import (
	"context"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

func constantPath(x, y, speed, duration float64) solver.Result {
	return solver.Result{Duration: duration, Nodes: []solver.Node{
		{Position: track.Vec3{X: x, Y: y}, Speed: speed},
		{Position: track.Vec3{X: x + speed*duration, Y: y}, Speed: speed, S: speed * duration, Time: duration},
	}}
}

func TestGrazingContactRejectsUnresolvedInterval(t *testing.T) {
	// The moving disc grazes the stationary disc at t=1. Endpoints clear,
	// but no positive-width Lipschitz interval can certify the tangent point.
	r := Result{Duration: 2, Cars: [2]Car{
		{Radius: 2, Path: constantPath(-1, 4, 1, 2)},
		{Radius: 2, Path: constantPath(0, 0, 0, 2)},
	}}
	if _, ok := certify(context.Background(), r); ok {
		t.Fatal("accepted unresolved tangent contact")
	}
}

func TestCertificationBudgetRejectsUnresolvedRemainder(t *testing.T) {
	// Parallel motion keeps a true 10 µm body gap. With a relative-speed
	// bound of 2 m/s, certification needs intervals <=10 µs: depth 17,
	// 131072 leaves / 262143 visits. This exceeds the 250000-visit budget
	// before reaching the 1 µs resolution floor. Safety requires rejection.
	r := Result{Duration: 1, Cars: [2]Car{
		{Radius: 2, Path: constantPath(0, 0, 1, 1)},
		{Radius: 2, Path: constantPath(0, 4.00001, 1, 1)},
	}}
	if _, ok := certify(context.Background(), r); ok {
		t.Fatal("accepted pair without certifying the whole interval")
	}
	// Four times as much clearance needs only depth 15 and fits the budget.
	r.Cars[1].Path = constantPath(0, 4.00004, 1, 1)
	if _, ok := certify(context.Background(), r); !ok {
		t.Fatal("rejected the resolvable nearby control")
	}
}
