package solver

import (
	"math"

	"github.com/TheFellow/the-line/pkg/track"
)

type roadEdge struct {
	a, b track.Vec3
	grip float64
}

// edgeGrid indexes side segments in plan. Both legal-boundary clearance and
// asphalt/kerb contact query the same exact circular swept-footprint geometry.
type edgeGrid struct {
	cellSize float64
	cells    map[[2]int][]roadEdge
}

func (g edgeGrid) bounds(a, b track.Vec3, pad float64) (int, int, int, int) {
	return int(math.Floor((min(a.X, b.X) - pad) / g.cellSize)), int(math.Floor((max(a.X, b.X) + pad) / g.cellSize)),
		int(math.Floor((min(a.Y, b.Y) - pad) / g.cellSize)), int(math.Floor((max(a.Y, b.Y) + pad) / g.cellSize))
}

func (g edgeGrid) add(edge roadEdge) {
	x0, x1, y0, y1 := g.bounds(edge.a, edge.b, 0)
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			g.cells[[2]int{x, y}] = append(g.cells[[2]int{x, y}], edge)
		}
	}
}

func (g edgeGrid) contactGrip(a, b track.Vec3, radius float64) float64 {
	grip := math.Inf(1)
	x0, x1, y0, y1 := g.bounds(a, b, radius)
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for _, edge := range g.cells[[2]int{x, y}] {
				if edge.grip < grip && segmentsNear(a, b, edge.a, edge.b, radius) {
					grip = edge.grip
				}
			}
		}
	}
	return grip
}

type roadEdges struct{ legal, kerbs edgeGrid }

func indexRoadEdges(road []track.Sample, radius float64) *roadEdges {
	newGrid := func() edgeGrid { return edgeGrid{math.Max(2, 2*radius), make(map[[2]int][]roadEdge)} }
	out := &roadEdges{newGrid(), newGrid()}
	for i := 0; i < len(road)-1; i++ {
		if i%64 == 0 {
			cooperate()
		}
		a, b := road[i], road[i+1]
		for _, side := range []float64{-1, 1} {
			out.legal.add(roadEdge{a.AtOffset(a.EdgeOffset(side)), b.AtOffset(b.EdgeOffset(side)), 0})
			if !a.KerbsCountAsRoad {
				continue
			}
			ak, bk, aw, bw := a.KerbLeft, b.KerbLeft, a.LeftWidth(), b.LeftWidth()
			if side < 0 {
				ak, bk, aw, bw = a.KerbRight, b.KerbRight, a.RightWidth(), b.RightWidth()
			}
			grip := math.Inf(1)
			if ak.Width > 0 {
				grip = ak.Friction()
			}
			if bk.Width > 0 {
				grip = min(grip, bk.Friction())
			}
			if !math.IsInf(grip, 1) {
				out.kerbs.add(roadEdge{a.AtOffset(side * aw), b.AtOffset(side * bw), grip})
			}
		}
	}
	return out
}

func segmentsNear(a, b, c, d track.Vec3, radius float64) bool {
	abc, abd := cross(b.X-a.X, b.Y-a.Y, c.X-a.X, c.Y-a.Y), cross(b.X-a.X, b.Y-a.Y, d.X-a.X, d.Y-a.Y)
	cda, cdb := cross(d.X-c.X, d.Y-c.Y, a.X-c.X, a.Y-c.Y), cross(d.X-c.X, d.Y-c.Y, b.X-c.X, b.Y-c.Y)
	if abc*abd < 0 && cda*cdb < 0 {
		return true
	}
	pointNear := func(p, a, b track.Vec3) bool {
		dx, dy := b.X-a.X, b.Y-a.Y
		u := 0.0
		if length2 := dx*dx + dy*dy; length2 > 0 {
			u = clamp(((p.X-a.X)*dx+(p.Y-a.Y)*dy)/length2, 0, 1)
		}
		return math.Hypot(p.X-a.X-u*dx, p.Y-a.Y-u*dy) <= radius
	}
	return pointNear(a, c, d) || pointNear(b, c, d) || pointNear(c, a, b) || pointNear(d, a, b)
}
