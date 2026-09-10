package racecraft

import (
	"context"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"testing"
)

func TestBetweenFrameCollision(t *testing.T) {
	// Both endpoints are safe, but the cars cross in the middle in 10 ms.
	path := func(x, y, dx, dy float64) solver.Result {
		return solver.Result{Duration: .01, Nodes: []solver.Node{{Position: track.Vec3{X: x, Y: y}, Speed: 2000}, {Position: track.Vec3{X: x + dx, Y: y + dy}, Speed: 2000, S: 20, Time: .01}}}
	}
	r := Result{Duration: .01, Cars: [2]Car{{Radius: 2, Path: path(-10, 0, 20, 0)}, {Radius: 2, Path: path(0, -10, 0, 20)}}}
	if _, ok := certify(context.Background(), r); ok {
		t.Fatal("tunneling accepted")
	}
	r.Cars[1].Path = path(-10, 5, 20, 0)
	if clearance, ok := certify(context.Background(), r); !ok || clearance < 0 {
		t.Fatal("safe parallel pair rejected")
	}
}
