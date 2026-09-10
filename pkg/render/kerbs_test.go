package render

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestKerbClippingUsesAuthoritativeTrianglePlane(t *testing.T) {
	// A deliberately twisted cell: its diagonal is z=0→4. A strip crossing
	// that diagonal must gain a vertex on that edge, rather than floating at
	// the bilinear ribbon height.
	tri := [3]track.Vec3{{X: 0, Y: 4, Z: 2}, {X: 0, Y: -4, Z: -2}, {X: 10, Y: 4, Z: 6}}
	poly := []track.Vec3{{X: 0, Y: 2, Z: 1}, {X: 10, Y: 2, Z: 3}, {X: 10, Y: 4, Z: 6}, {X: 0, Y: 4, Z: 2}}
	out := clipWorldTriangle(poly, tri)
	if len(out) < 3 {
		t.Fatal("kerb lost its visible triangle")
	}
	for _, p := range out {
		// Independently derive this plane from its three vertices: z=.4x+.5y.
		if math.Abs(p.Z-(.4*p.X+.5*p.Y)) > 1e-12 {
			t.Fatal("kerb vertex floats above road triangle")
		}
		if p.X < 0 || p.X > 10 || p.Y < 2 || p.Y > 4 || p.Y < .8*p.X-4-1e-12 {
			t.Fatal("kerb clipping escaped triangle or strip")
		}
	}
}
