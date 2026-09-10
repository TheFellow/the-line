package solver

import (
	"fmt"
	"math"

	"github.com/TheFellow/the-line/pkg/vehicle"
)

func (e evaluator) run(offset []float64) (Result, error) {
	if err := e.cancelled(); err != nil {
		return Result{}, err
	}
	path, err := e.geometry(offset)
	if err != nil {
		return Result{}, err
	}
	n := len(path)
	speeds := make([]float64, n)
	grades := make([]float64, n-1)
	for i := range grades {
		a, b := path[i].node.Position, path[i+1].node.Position
		h := math.Hypot(b.X-a.X, b.Y-a.Y)
		if h < 1e-9 {
			return Result{}, fmt.Errorf("vertical or zero-length segment at station %.2f m", path[i].node.Station)
		}
		grades[i] = (b.Z - a.Z) / h
	}
	var config *vehicle.Config
	switch model := e.model.(type) {
	case vehicle.Config:
		config = &model
	case *vehicle.Config:
		config = model
	}
	segments := borrowEnvelopes(n - 1)
	defer releaseEnvelopes(segments)
	for i := range segments {
		segments[i].model, segments[i].config = e.model, config
		segments[i].a, segments[i].b, segments[i].grade = path[i], path[i+1], grades[i]
		segments[i].ready = [65]bool{}
	}
	maxSpeed := e.model.Parameters().MaxSpeed
	// Node limits include either adjacent segment's conservative road state.
	for i := range speeds {
		if i%32 == 0 {
			if err := e.cancelled(); err != nil {
				return Result{}, err
			}
		}
		cap := maxSpeed
		for j := max(0, i-1); j <= min(n-2, i); j++ {
			feasible := func(v float64) bool {
				a, b := path[j], path[j+1]
				grip := math.Min(a.grip, b.grip)
				for endpoint, p := range []pathState{a, b} {
					ok := false
					if config != nil {
						ok = config.FeasibleOn(v, segments[j].state(endpoint*64))
					} else {
						ok = e.model.Limits(v, p.node.Curvature, p.bank, grades[j], grip).Feasible
					}
					if !ok {
						return false
					}
				}
				return true
			}
			if !feasible(0) {
				return Result{}, fmt.Errorf("bank/grade exceeds stationary grip at station %.2f m", path[i].node.Station)
			}
			if !feasible(cap) {
				lo, hi := 0., cap
				for k := 0; k < 25; k++ {
					mid := (lo + hi) / 2
					if feasible(mid) {
						lo = mid
					} else {
						hi = mid
					}
				}
				cap = lo
			}
		}
		speeds[i] = cap * (1 - 1e-7)
	}
	if e.closed() {
		speeds[0] = math.Min(speeds[0], speeds[n-1])
		speeds[n-1] = speeds[0]
	} else {
		speeds[0] = math.Min(speeds[0], e.entry)
		speeds[n-1] = math.Min(speeds[n-1], e.exit)
	}
	converged := false
	denseDrive, denseBrake := make([]float64, n-1), make([]float64, n-1)
	resolutions := make([]int, n-1)
	for i := range resolutions {
		resolutions[i] = 5
	}
	for pass := 0; pass < 40; pass++ {
		if err := e.cancelled(); err != nil {
			return Result{}, err
		}
		change := 0.
		for i := 0; i < n-1; i++ {
			if i%32 == 0 {
				if err := e.cancelled(); err != nil {
					return Result{}, err
				}
			}
			ds := path[i+1].node.S - path[i].node.S
			residual := func(v float64) float64 {
				acc, _ := segments[i].limits(speeds[i], v, resolutions[i])
				return (v*v-speeds[i]*speeds[i])/(2*ds) - acc
			}
			if residual(speeds[i+1]) > 1e-8 {
				old := speeds[i+1]
				lo, ok := maximumSpeed(old, residual)
				if !ok {
					return Result{}, fmt.Errorf("insufficient driving force at station %.2f m", path[i].node.Station)
				}
				speeds[i+1] = lo
				change = math.Max(change, old-lo)
			}
		}
		if e.closed() {
			old := speeds[0]
			speeds[0] = math.Min(speeds[0], speeds[n-1])
			speeds[n-1] = speeds[0]
			change = math.Max(change, old-speeds[0])
		}
		for i := n - 2; i >= 0; i-- {
			if i%32 == 0 {
				if err := e.cancelled(); err != nil {
					return Result{}, err
				}
			}
			ds := path[i+1].node.S - path[i].node.S
			residual := func(v float64) float64 {
				_, brake := segments[i].limits(v, speeds[i+1], resolutions[i])
				return (v*v-speeds[i+1]*speeds[i+1])/(2*ds) - brake
			}
			if residual(speeds[i]) > 1e-8 {
				old := speeds[i]
				lo, ok := maximumSpeed(old, residual)
				if !ok {
					return Result{}, fmt.Errorf("insufficient braking force at station %.2f m", path[i].node.Station)
				}
				speeds[i] = lo
				change = math.Max(change, old-lo)
			}
		}
		if e.closed() {
			old := speeds[n-1]
			speeds[n-1] = math.Min(speeds[0], speeds[n-1])
			speeds[0] = speeds[n-1]
			change = math.Max(change, old-speeds[n-1])
		}
		if change < 1e-6 {
			refine := false
			for i := range resolutions {
				if i%32 == 0 {
					if err := e.cancelled(); err != nil {
						return Result{}, err
					}
				}
				ds := path[i+1].node.S - path[i].node.S
				a := (speeds[i+1]*speeds[i+1] - speeds[i]*speeds[i]) / (2 * ds)
				drive, brake := segments[i].limits(speeds[i], speeds[i+1], 65)
				denseDrive[i], denseBrake[i] = drive, brake
				if math.Max(a-drive, -a-brake) > 1e-6 && resolutions[i] < 65 {
					resolutions[i] = 65
					refine = true
				}
			}
			if !refine {
				converged = true
				break
			}
		}
	}
	if !converged {
		return Result{}, fmt.Errorf("speed envelope failed to converge in 40 passes")
	}
	nodes := make([]Node, n)
	t := 0.
	residual := 0.
	for i := range nodes {
		if i%32 == 0 {
			if err := e.cancelled(); err != nil {
				return Result{}, err
			}
		}
		nodes[i] = path[i].node
		nodes[i].Speed = speeds[i]
		nodes[i].Time = t
		if i == n-1 {
			break
		}
		ds := path[i+1].node.S - path[i].node.S
		sum := speeds[i] + speeds[i+1]
		if sum <= 1e-8 {
			return Result{}, fmt.Errorf("two stopped nodes span %.3f m; reduce spacing", ds)
		}
		a := (speeds[i+1]*speeds[i+1] - speeds[i]*speeds[i]) / (2 * ds)
		nodes[i].Acceleration = a
		// The convergence gate evaluated all 65 points at these exact final
		// speeds. Reuse those bounds; the independent verification suite uses
		// its own denser samples and separately reconstructed force balance.
		drive, brake := denseDrive[i], denseBrake[i]
		r := math.Max(a-drive, -a-brake)
		residual = math.Max(residual, r)
		if r > 2e-5 || !finite(r) {
			return Result{}, fmt.Errorf("force residual %.6g at station %.2f m", r, nodes[i].Station)
		}
		t += 2 * ds / sum
	}
	if !finite(t) || t <= 0 {
		return Result{}, fmt.Errorf("invalid traversal time")
	}
	e.instrument(nodes, path, grades)
	if e.closed() {
		end := nodes[n-1]
		nodes[n-1] = nodes[0]
		nodes[n-1].S, nodes[n-1].Station, nodes[n-1].Time = end.S, end.Station, end.Time
	}
	return Result{Closed: e.closed(), model: e.model, Nodes: nodes, Duration: t, Length: nodes[n-1].S, MaxForceResidual: residual}, nil
}

// Conservative segment envelope uses the lowest capacities across the represented
// constant-acceleration segment and minimum adjacent grip at surface boundaries.
func (e evaluator) limits(a, b pathState, grade, v0, v1 float64, samples int) (float64, float64) {
	drive, brake := math.Inf(1), math.Inf(1)
	grip := math.Min(a.grip, b.grip)
	for j := 0; j < samples; j++ {
		f := float64(j) / float64(samples-1)
		v := math.Sqrt(lerp(v0*v0, v1*v1, f))
		env := e.model.Limits(v, lerp(a.node.Curvature, b.node.Curvature, f), lerp(a.bank, b.bank, f), grade, grip)
		if !env.Feasible {
			return math.Inf(-1), math.Inf(-1)
		}
		drive = math.Min(drive, env.Acceleration)
		brake = math.Min(brake, env.Braking)
	}
	return drive, brake
}

// maximumSpeed uses a safeguarded Illinois secant in squared speed. Constant
// force cases are linear in this coordinate, so their roots need one evaluation.
func maximumSpeed(cap float64, residual func(float64) float64) (float64, bool) {
	lo, hi := 0., cap*cap
	flo, fhi := residual(0), residual(cap)
	if flo > 0 || !finite(flo) {
		return 0, false
	}
	side := 0
	for k := 0; k < 40; k++ {
		x := (lo*fhi - hi*flo) / (fhi - flo)
		if !finite(x) || x <= lo || x >= hi {
			x = (lo + hi) / 2
		}
		fx := residual(math.Sqrt(x))
		if fx <= 0 {
			lo = x
			flo = fx
			if side == -1 {
				fhi /= 2
			}
			side = -1
		} else {
			hi = x
			fhi = fx
			if side == 1 {
				flo /= 2
			}
			side = 1
		}
		if fx <= 0 && fx >= -1e-8 {
			return math.Sqrt(x), true
		}
		if hi-lo < 1e-9 {
			break
		}
	}
	return math.Sqrt(lo), true
}
