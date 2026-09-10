package racecraft

import (
	"context"
	"math"
)

// On every interval, distance can change at no more than the sum of the cars'
// maximum speeds. Endpoint distances minus V*dt/2 bound the interior, including
// trajectory vertices and acceleration changes. The global speed maxima are
// conservative because each segment has constant longitudinal acceleration.
func certify(ctx context.Context, r Result) (float64, bool) {
	speed := 0.
	for _, c := range r.Cars {
		v := 0.
		for _, n := range c.Path.Nodes {
			v = math.Max(v, n.Speed)
		}
		speed += v
	}
	radii := r.Cars[0].Radius + r.Cars[1].Radius
	required := radii + r.Config.Clearance
	distance := func(t float64) float64 {
		n := r.At(t)
		return math.Hypot(n[0].Position.X-n[1].Position.X, n[0].Position.Y-n[1].Position.Y)
	}
	minimum := math.Inf(1)
	intervals := 0
	var interval func(float64, float64, float64, float64) bool
	interval = func(a, b, da, db float64) bool {
		intervals++
		if intervals > 250000 || ctx.Err() != nil || da < required || db < required {
			return false
		}
		lower := math.Min(da, db) - speed*(b-a)/2
		if lower >= required {
			minimum = math.Min(minimum, lower-radii)
			return true
		}
		if b-a < 1e-6 {
			return false
		}
		m := (a + b) / 2
		dm := distance(m)
		return interval(a, m, da, dm) && interval(m, b, dm, db)
	}
	ok := interval(0, r.Duration, distance(0), distance(r.Duration))
	return minimum, ok
}
