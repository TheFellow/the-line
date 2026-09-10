package solver

import (
	"fmt"
	"math"
	"sort"
)

// At queries the optimized trajectory at elapsed seconds, using constant
// acceleration on each validated straight segment. Open paths clamp at their
// endpoints, including infinite times; NaN selects the entry. An empty
// trajectory returns a zero Node. Wrapping is a presentation decision.
func (r Result) At(t float64) Node { return r.forceAt(atTime(r.Nodes, t)) }

// CenterAt is At for the verified centreline reference, at the same elapsed
// seconds rather than the same road station or fraction of traversal time.
func (r Result) CenterAt(t float64) Node { return r.forceAt(atTime(r.CenterNodes, t)) }

// AtStation queries the optimized trajectory at reference-road metres, clamped
// to the open endpoints. Position and Station vary linearly with segment
// distance; speed and elapsed time follow constant-acceleration kinematics.
// Nonfinite queries and empty trajectories return an error.
func (r Result) AtStation(station float64) (Node, error) {
	n, err := atStation(r.Nodes, station)
	return r.forceAt(n), err
}

// CenterAtStation is AtStation for the verified centreline reference. Subtract
// its Time from AtStation's Time at the same station to measure time gained:
// a negative optimized-minus-reference delta means the optimized line is ahead.
func (r Result) CenterAtStation(station float64) (Node, error) {
	n, err := atStation(r.CenterNodes, station)
	return r.forceAt(n), err
}

func atTime(nodes []Node, t float64) Node {
	if len(nodes) == 0 {
		return Node{}
	}
	if math.IsNaN(t) || t <= nodes[0].Time {
		return nodes[0]
	}
	last := nodes[len(nodes)-1]
	if t >= last.Time {
		return last
	}
	hi := sort.Search(len(nodes), func(i int) bool { return nodes[i].Time >= t })
	if nodes[hi].Time == t {
		return nodes[hi]
	}
	a, b := nodes[hi-1], nodes[hi]
	dt := t - a.Time
	f := clamp((a.Speed*dt+.5*a.Acceleration*dt*dt)/(b.S-a.S), 0, 1)
	return interpolateNode(a, b, f, t, math.Max(0, a.Speed+a.Acceleration*dt))
}

func atStation(nodes []Node, station float64) (Node, error) {
	if !finite(station) {
		return Node{}, fmt.Errorf("road station must be finite")
	}
	if len(nodes) == 0 {
		return Node{}, fmt.Errorf("trajectory is empty")
	}
	if station <= nodes[0].Station {
		return nodes[0], nil
	}
	last := nodes[len(nodes)-1]
	if station >= last.Station {
		return last, nil
	}
	hi := sort.Search(len(nodes), func(i int) bool { return nodes[i].Station >= station })
	if nodes[hi].Station == station {
		return nodes[hi], nil
	}
	a, b := nodes[hi-1], nodes[hi]
	f := (station - a.Station) / (b.Station - a.Station)
	speed := math.Sqrt(lerp(a.Speed*a.Speed, b.Speed*b.Speed, f))
	// Average speed avoids cancellation in (v-v0)/a near constant speed,
	// and is finite for launches and stops without an epsilon denominator.
	dt := 2 * f * (b.S - a.S) / (a.Speed + speed)
	return interpolateNode(a, b, f, a.Time+dt, speed), nil
}

func interpolateNode(a, b Node, f, t, speed float64) Node {
	forces := interpolateForces(a, b, f)
	return Node{
		Position: mix(a.Position, b.Position, f), S: lerp(a.S, b.S, f),
		Station: lerp(a.Station, b.Station, f), Time: t, Speed: speed,
		Curvature: lerp(a.Curvature, b.Curvature, f), Offset: lerp(a.Offset, b.Offset, f),
		Acceleration: a.Acceleration,
		Bank:         forces.Bank, Grade: forces.Grade, Grip: forces.Grip, Forces: forces.Forces,
	}
}
