package solver

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestEdgeGridFindsRemoteCellFootprintContact(t *testing.T) {
	// The nearest point is in the interior of a later side segment, not an
	// adjacent cross section. Exercise grid boundaries and negative coordinates.
	for _, shift := range []float64{-100.1, -2, -.01, 0, 1.99, 200} {
		grid := edgeGrid{2, make(map[[2]int][]roadEdge)}
		grid.add(roadEdge{track.Vec3{X: shift - 3, Y: 1}, track.Vec3{X: shift + 3, Y: 1}, 0})
		a, b := track.Vec3{X: shift - .25}, track.Vec3{X: shift + .25}
		if grid.contactGrip(a, b, 1.0001) != 0 {
			t.Fatal("missed interior contact in neighboring cell")
		}
		if !math.IsInf(grid.contactGrip(a, b, .9999), 1) {
			t.Fatal("charged a clear footprint")
		}
		// Crossing segments are a collision even when all endpoint pairs are far.
		a, b = track.Vec3{X: shift, Y: -2}, track.Vec3{X: shift, Y: 3}
		if grid.contactGrip(a, b, .01) != 0 {
			t.Fatal("missed swept crossing")
		}
	}
}

func TestZeroWidthKerbDoesNotChargeDefaultGrip(t *testing.T) {
	road := []track.Sample{
		{Position: track.Vec3{}, Normal: track.Vec3{Y: 1}, Width: 12, KerbsCountAsRoad: true},
		{Position: track.Vec3{X: 10}, Normal: track.Vec3{Y: 1}, Width: 12, KerbsCountAsRoad: true, KerbLeft: track.Kerb{Width: 1, Surface: "asphalt"}},
	}
	edges := indexRoadEdges(road, 1)
	if got := edges.kerbs.contactGrip(track.Vec3{X: 2, Y: 5.1}, track.Vec3{X: 8, Y: 5.1}, 1); got != 1 {
		t.Fatalf("taper charged absent kerb: %g", got)
	}
}
