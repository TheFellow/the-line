package solver

import (
	"math"

	"github.com/TheFellow/the-line/pkg/track"
)

type kerbEdge struct {
	a, b track.Vec3
	grip float64
}

// segmentKerbGrip measures circular-footprint contact with the actual asphalt
// edges, including neighboring road cells. A cross-section offset alone misses
// contact where width tapers or the path approaches an edge obliquely. The grid
// keeps this geometric check local even on long, finely sampled laps.
// The existing profile's minimum-adjacent-grip rule remains conservative at
// categorical surface boundaries. Excluded kerbs never affect the force model.
func segmentKerbGrip(road []track.Sample, points []track.Vec3, radius float64) []float64 {
	if !road[0].KerbsCountAsRoad {
		return nil
	}
	cellSize := math.Max(2, 2*radius)
	grid := make(map[[2]int][]kerbEdge)
	bounds := func(a, b track.Vec3, pad float64) (int, int, int, int) {
		return int(math.Floor((min(a.X, b.X) - pad) / cellSize)), int(math.Floor((max(a.X, b.X) + pad) / cellSize)),
			int(math.Floor((min(a.Y, b.Y) - pad) / cellSize)), int(math.Floor((max(a.Y, b.Y) + pad) / cellSize))
	}
	for i := 0; i < len(road)-1; i++ {
		a, b := road[i], road[i+1]
		for _, side := range []float64{-1, 1} {
			ak, bk, aw, bw := a.KerbLeft, b.KerbLeft, a.LeftWidth(), b.LeftWidth()
			if side < 0 {
				ak, bk, aw, bw = a.KerbRight, b.KerbRight, a.RightWidth(), b.RightWidth()
			}
			if ak.Width == 0 && bk.Width == 0 {
				continue
			}
			edge := kerbEdge{a.AtOffset(side * aw), b.AtOffset(side * bw), min(ak.Friction(), bk.Friction())}
			x0, x1, y0, y1 := bounds(edge.a, edge.b, 0)
			for x := x0; x <= x1; x++ {
				for y := y0; y <= y1; y++ {
					grid[[2]int{x, y}] = append(grid[[2]int{x, y}], edge)
				}
			}
		}
	}
	grips := make([]float64, len(points)-1)
	for i := range grips {
		grips[i] = math.Inf(1)
		a, b := points[i], points[i+1]
		x0, x1, y0, y1 := bounds(a, b, radius)
		for x := x0; x <= x1; x++ {
			for y := y0; y <= y1; y++ {
				for _, edge := range grid[[2]int{x, y}] {
					if edge.grip < grips[i] && segmentsNear(a, b, edge.a, edge.b, radius) {
						grips[i] = edge.grip
					}
				}
			}
		}
	}
	return grips
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
