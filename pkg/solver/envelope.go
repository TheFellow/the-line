package solver

import (
	"github.com/TheFellow/the-line/pkg/vehicle"
	"math"
	"sync"
)

// A segment retains only speed-independent road projections. The adaptive
// five-point pass is a subset of the mandatory 65-point final check, so every
// reused state has precisely the same interpolation fraction as before caching.
type segmentEnvelope struct {
	model  vehicle.Model
	config *vehicle.Config
	a, b   pathState
	grade  float64
	states [65]vehicle.RoadState
	ready  [65]bool
}

func (s *segmentEnvelope) state(j int) vehicle.RoadState {
	if !s.ready[j] {
		f := float64(j) / 64
		s.states[j] = vehicle.PrepareRoad(lerp(s.a.node.Curvature, s.b.node.Curvature, f), lerp(s.a.bank, s.b.bank, f), s.grade, math.Min(s.a.grip, s.b.grip))
		s.ready[j] = true
	}
	return s.states[j]
}
func (s *segmentEnvelope) limits(v0, v1 float64, samples int) (float64, float64) {
	drive, brake := math.Inf(1), math.Inf(1)
	// A constant speed, curvature and bank has one identical envelope at
	// every requested sample, including the dense verification grid.
	if s.config != nil && v0 == v1 && s.a.node.Curvature == s.b.node.Curvature && s.a.bank == s.b.bank {
		a, b, ok := s.config.BoundsOn(v0, s.state(0))
		if !ok {
			return math.Inf(-1), math.Inf(-1)
		}
		return a, b
	}
	for j := 0; j < samples; j++ {
		f := float64(j) / float64(samples-1)
		v := math.Sqrt(lerp(v0*v0, v1*v1, f))
		var acceleration, braking float64
		var feasible bool
		if s.config != nil {
			acceleration, braking, feasible = s.config.BoundsOn(v, s.state(j*64/(samples-1)))
		} else {
			env := s.model.Limits(v, lerp(s.a.node.Curvature, s.b.node.Curvature, f), lerp(s.a.bank, s.b.bank, f), s.grade, math.Min(s.a.grip, s.b.grip))
			acceleration, braking, feasible = env.Acceleration, env.Braking, env.Feasible
		}
		if !feasible {
			return math.Inf(-1), math.Inf(-1)
		}
		drive = math.Min(drive, acceleration)
		brake = math.Min(brake, braking)
	}
	return drive, brake
}

var envelopePool sync.Pool

func borrowEnvelopes(n int) []segmentEnvelope {
	if p, ok := envelopePool.Get().(*[]segmentEnvelope); ok && cap(*p) >= n {
		return (*p)[:n]
	}
	return make([]segmentEnvelope, n)
}
func releaseEnvelopes(segments []segmentEnvelope) {
	// Do not retain a custom model or context via the cache.
	for i := range segments {
		segments[i].model = nil
		segments[i].config = nil
	}
	envelopePool.Put(&segments)
}
