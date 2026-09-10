package solver

import (
	"fmt"
	"github.com/TheFellow/the-line/pkg/track"
	"math"
)

func (e evaluator) geometry(offset []float64) ([]pathState, error) {
	n := len(e.road)
	if err := validatePeriodicOffsets(e.road, offset); err != nil {
		return nil, err
	}
	points := make([]track.Vec3, n)
	curves := make([]float64, n)
	for i, s := range e.road {
		points[i] = s.AtOffset(offset[i])
	}
	edges := e.edges
	if edges == nil {
		edges = indexRoadEdges(e.road, e.clearance)
	}
	for i := 0; i < n-1; i++ {
		if i%64 == 0 {
			if err := e.cancelled(); err != nil {
				return nil, err
			}
		}
		if edges.legal.contactGrip(points[i], points[i+1], math.Max(0, e.clearance-1e-7)) == 0 {
			return nil, fmt.Errorf("candidate violates side-edge clearance at station %.2f m", e.road[i].S)
		}
	}
	for i := 1; i < n-1; i++ {
		a, b, c := points[i-1], points[i], points[i+1]
		ab := math.Hypot(b.X-a.X, b.Y-a.Y)
		bc := math.Hypot(c.X-b.X, c.Y-b.Y)
		ac := math.Hypot(c.X-a.X, c.Y-a.Y)
		if ab*bc*ac < 1e-9 {
			return nil, fmt.Errorf("degenerate candidate at station %.2f m", e.road[i].S)
		}
		curves[i] = 2 * cross(b.X-a.X, b.Y-a.Y, c.X-b.X, c.Y-b.Y) / (ab * bc * ac)
	}
	curves[0] = curves[1]
	curves[n-1] = curves[n-2]
	if e.closed() {
		a, b, c := points[n-2], points[0], points[1]
		ab, bc, ac := math.Hypot(b.X-a.X, b.Y-a.Y), math.Hypot(c.X-b.X, c.Y-b.Y), math.Hypot(c.X-a.X, c.Y-a.Y)
		if ab*bc*ac < 1e-9 {
			return nil, fmt.Errorf("degenerate candidate at lap seam")
		}
		curves[0] = 2 * cross(b.X-a.X, b.Y-a.Y, c.X-b.X, c.Y-b.Y) / (ab * bc * ac)
		curves[n-1] = curves[0]
	}
	path := make([]pathState, 0, 2*n)
	add := func(p track.Vec3, station, k, o, bank, grip float64) {
		s := 0.
		if len(path) > 0 {
			prev := path[len(path)-1].node
			s = prev.S + distance(prev.Position, p)
		}
		path = append(path, pathState{node: Node{Position: p, S: s, Station: station, Curvature: k, Offset: o}, bank: bank, grip: grip})
	}
	for i := 0; i < n; i++ {
		s := e.road[i]
		add(points[i], s.S, curves[i], offset[i], s.Bank, s.GripAcross(offset[i], e.clearance))
		if i == n-1 {
			break
		}
		a, b := e.road[i], e.road[i+1]
		if a.KerbsCountAsRoad {
			// Evaluate both cross sections with the outgoing asphalt surface: this
			// adds kerb conservatism without moving existing road-surface transitions.
			end := b
			end.Grip = a.Grip
			path[len(path)-1].grip = min(a.GripAcross(offset[i], e.clearance), end.GripAcross(offset[i+1], e.clearance))
			path[len(path)-1].grip = min(path[len(path)-1].grip, edges.kerbs.contactGrip(points[i], points[i+1], e.clearance))
		}
		// The line lies in the convex, clearance-inset cell. Its height follows the
		// same two authoritative triangles as the renderer, including warped banks.
		polygon := []track.Vec3{a.AtOffset(a.LeftLimit() - e.clearance), a.AtOffset(-a.RightLimit() + e.clearance), b.AtOffset(-b.RightLimit() + e.clearance), b.AtOffset(b.LeftLimit() - e.clearance)}
		if !convex(polygon) {
			return nil, fmt.Errorf("clearance ribbon folds at station %.2f m", s.S)
		}
		for _, p := range []track.Vec3{points[i], points[i+1]} {
			if !inside(polygon, p) {
				return nil, fmt.Errorf("candidate leaves clearance ribbon at station %.2f m", s.S)
			}
			// Width is horizontal, but circular vehicle clearance is Euclidean:
			// tapered side edges can be closer than cross-section width implies.
			for _, side := range []float64{-1, 1} {
				edge0, edge1 := a.AtOffset(a.EdgeOffset(side)), b.AtOffset(b.EdgeOffset(side))
				dx, dy := edge1.X-edge0.X, edge1.Y-edge0.Y
				gap := math.Abs(cross(dx, dy, p.X-edge0.X, p.Y-edge0.Y)) / math.Hypot(dx, dy)
				if gap+1e-7 < e.clearance {
					return nil, fmt.Errorf("candidate violates side-edge clearance at station %.2f m", s.S)
				}
			}
		}
		d0, d1 := a.AtOffset(-a.RightLimit()), b.AtOffset(b.LeftLimit())
		p, q := points[i], points[i+1]
		dx, dy := q.X-p.X, q.Y-p.Y
		ex, ey := d1.X-d0.X, d1.Y-d0.Y
		den := cross(dx, dy, ex, ey)
		if math.Abs(den) > 1e-10 {
			t := cross(d0.X-p.X, d0.Y-p.Y, ex, ey) / den
			u := cross(d0.X-p.X, d0.Y-p.Y, dx, dy) / den
			if t > 1e-6 && t < 1-1e-6 && u >= 0 && u <= 1 {
				v := mix(p, q, t)
				v.Z = lerp(d0.Z, d1.Z, u)
				add(v, lerp(a.S, b.S, t), lerp(curves[i], curves[i+1], t), lerp(offset[i], offset[i+1], t), lerp(a.Bank, b.Bank, t), path[len(path)-1].grip)
			}
		}
	}
	return path, nil
}

func convex(p []track.Vec3) bool {
	sign := 0.
	for i := range p {
		a, b, c := p[i], p[(i+1)%4], p[(i+2)%4]
		v := cross(b.X-a.X, b.Y-a.Y, c.X-b.X, c.Y-b.Y)
		if math.Abs(v) < 1e-9 {
			return false
		}
		if sign == 0 {
			sign = v
		}
		if sign*v < 0 {
			return false
		}
	}
	return true
}
func inside(poly []track.Vec3, p track.Vec3) bool {
	sign := 0.
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		v := cross(b.X-a.X, b.Y-a.Y, p.X-a.X, p.Y-a.Y)
		if math.Abs(v) < 1e-6 {
			continue
		}
		if sign == 0 {
			sign = v
		}
		if sign*v < 0 {
			return false
		}
	}
	return true
}
func cross(ax, ay, bx, by float64) float64 { return ax*by - ay*bx }
func finite(x float64) bool                { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func clamp(x, lo, hi float64) float64      { return math.Max(lo, math.Min(x, hi)) }
func lerp(a, b, t float64) float64         { return a + (b-a)*t }
func mix(a, b track.Vec3, t float64) track.Vec3 {
	return track.Vec3{X: lerp(a.X, b.X, t), Y: lerp(a.Y, b.Y, t), Z: lerp(a.Z, b.Z, t)}
}
func distance(a, b track.Vec3) float64 {
	return math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y) + (a.Z-b.Z)*(a.Z-b.Z))
}

// interpolateOffsets refines the smooth lateral controls, then evaluates fresh
// road positions and curvature. Latent interpolation keeps widths bounded.
func interpolateOffsets(coarse []track.Sample, offsets []float64, fine []track.Sample, clearance float64) []float64 {
	allZero := true
	for _, offset := range offsets {
		allZero = allZero && offset == 0
	}
	if allZero {
		return make([]float64, len(fine))
	}
	if coarse[0].Closed {
		return periodicOffsets(coarse, offsets, fine, clearance)
	}
	latent := make([]float64, len(coarse))
	for i := range latent {
		latent[i] = math.Atanh(clamp((offsets[i]-(coarse[i].LeftLimit()-coarse[i].RightLimit())/2)/((coarse[i].LeftLimit()+coarse[i].RightLimit())/2-clearance), -.999999, .999999))
	}
	// Natural cubic controls are C2. Re-fitting Catmull-Rom derivatives at each
	// resolution creates artificial curvature jumps and is not a refinement oracle.
	second := make([]float64, len(coarse))
	work := make([]float64, len(coarse))
	for i := 1; i < len(coarse)-1; i++ {
		left, right := coarse[i].S-coarse[i-1].S, coarse[i+1].S-coarse[i].S
		ratio := left / (left + right)
		pivot := ratio*second[i-1] + 2
		second[i] = (ratio - 1) / pivot
		work[i] = (6*((latent[i+1]-latent[i])/right-(latent[i]-latent[i-1])/left)/(left+right) - ratio*work[i-1]) / pivot
	}
	for i := len(coarse) - 2; i >= 0; i-- {
		second[i] = second[i]*second[i+1] + work[i]
	}
	result := make([]float64, len(fine))
	j := 0
	for i, p := range fine {
		station := p.S * coarse[len(coarse)-1].S / fine[len(fine)-1].S
		for j < len(coarse)-2 && coarse[j+1].S < station {
			j++
		}
		ds := coarse[j+1].S - coarse[j].S
		b := clamp((station-coarse[j].S)/ds, 0, 1)
		a := 1 - b
		value := a*latent[j] + b*latent[j+1] + ((a*a*a-a)*second[j]+(b*b*b-b)*second[j+1])*ds*ds/6
		result[i] = (p.LeftLimit()-p.RightLimit())/2 + ((p.LeftLimit()+p.RightLimit())/2-clearance)*math.Tanh(value)
	}
	return result
}
