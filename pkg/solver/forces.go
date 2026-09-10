package solver

import "math"

// instrument retains exactly the road state passed to the outgoing segment
// envelope. The final point uses incoming acceleration/state because it has no
// outgoing segment; this is explicitly a left-sided force observation.
func (e evaluator) instrument(nodes []Node, path []pathState, grades []float64) {
	if len(nodes) < 2 {
		return
	}
	for i := range nodes {
		j := min(i, len(nodes)-2)
		n := &nodes[i]
		n.Bank, n.Grade = path[i].bank, grades[j]
		n.Grip = math.Min(path[j].grip, path[j+1].grip)
		if i == len(nodes)-1 {
			n.Acceleration = nodes[i-1].Acceleration
		}
		cap := e.model.Parameters().MaxSpeed
		if i == 0 {
			cap = math.Min(cap, e.entry)
		} else if i == len(nodes)-1 {
			cap = math.Min(cap, e.exit)
		}
		n.Forces = e.model.Limits(n.Speed, n.Curvature, n.Bank, n.Grade, n.Grip).Tyres.Forces(n.Acceleration, n.Speed, cap)
	}
}

func (r Result) forceAt(n Node) Node {
	if r.model != nil && n.Forces.Available {
		cap := r.model.Parameters().MaxSpeed
		if len(r.Nodes) > 0 {
			if n.Station == r.Nodes[0].Station {
				cap = math.Min(cap, r.EntrySpeedCap)
			} else if n.Station == r.Nodes[len(r.Nodes)-1].Station {
				cap = math.Min(cap, r.ExitSpeedCap)
			}
		}
		n.Forces = r.model.Limits(n.Speed, n.Curvature, n.Bank, n.Grade, n.Grip).Tyres.Forces(n.Acceleration, n.Speed, cap)
	}
	return n
}

// Interpolating an archived result without its live Model preserves the
// exported envelope and labels, recomputing the force norm. A live Result
// replaces this approximation with a fresh Model.Limits call in forceAt.
func interpolateForces(a, b Node, f float64) (n Node) {
	n.Bank, n.Grade, n.Grip = lerp(a.Bank, b.Bank, f), a.Grade, a.Grip
	n.Forces = a.Forces
	if a.Forces.Available && b.Forces.Available {
		n.Forces.Lateral = lerp(a.Forces.Lateral, b.Forces.Lateral, f)
		n.Forces.Longitudinal = lerp(a.Forces.Longitudinal, b.Forces.Longitudinal, f)
		n.Forces.Capacity = lerp(a.Forces.Capacity, b.Forces.Capacity, f)
		n.Forces.Utilization = math.Hypot(n.Forces.Lateral, n.Forces.Longitudinal) / n.Forces.Capacity
	}
	return n
}
