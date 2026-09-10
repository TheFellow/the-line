// Package verification checks exported trajectories without solver internals.
package verification

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestPresetVehicleMatrix(t *testing.T) {
	for _, preset := range track.Presets() {
		for _, car := range vehicle.Presets() {
			t.Run(preset+"/"+car, func(t *testing.T) {
				scene, err := track.Preset(preset)
				if err != nil {
					t.Fatal(err)
				}
				cfg, err := vehicle.Preset(car)
				if err != nil {
					t.Fatal(err)
				}
				opts := solver.DefaultOptions()
				// One full search iteration exercises geometry changes for every pairing;
				// the solver package separately checks the full default search budget.
				opts.Iterations = 1
				result, err := solver.Solve(scene, cfg, opts)
				if err != nil {
					t.Fatal(err)
				}
				if result.Duration > result.CenterDuration+1e-7 {
					t.Fatal("optimization regressed against its baseline")
				}
				for _, trajectory := range []struct {
					name     string
					nodes    []solver.Node
					duration float64
				}{{"optimized", result.Nodes, result.Duration}, {"reference", result.CenterNodes, result.CenterDuration}} {
					t.Run(trajectory.name, func(t *testing.T) {
						if len(trajectory.nodes) < 2 {
							t.Fatal("missing trajectory")
						}
						for i, node := range trajectory.nodes {
							for _, value := range []float64{node.Position.X, node.Position.Y, node.Position.Z, node.S, node.Station, node.Time, node.Speed, node.Curvature, node.Acceleration} {
								if math.IsNaN(value) || math.IsInf(value, 0) {
									t.Fatalf("nonfinite output at node %d", i)
								}
							}
							if i > 0 && node.Station <= trajectory.nodes[i-1].Station {
								t.Fatal("reference station is not strictly increasing")
							}
						}
						first, last := trajectory.nodes[0], trajectory.nodes[len(trajectory.nodes)-1]
						if first.Station != result.Road[0].S || last.Station != result.Road[len(result.Road)-1].S {
							t.Fatal("road station bounds differ")
						}
						if last.Time != trajectory.duration {
							t.Fatal("duration does not match last node")
						}
						if !scene.Closed && (first.Speed > scene.EntrySpeed+1e-7 || last.Speed > scene.ExitSpeed+1e-7) {
							t.Fatal("endpoint cap exceeded")
						}
						checked := result
						checked.Nodes = trajectory.nodes
						minClearance := checkClearance(t, checked, cfg.Width/2+opts.Margin)
						residual := checkForces(t, checked, cfg)
						t.Logf("time %.6fs clearance %.6fm max force excess %.3g m/s²", trajectory.duration, minClearance, residual)
					})
				}
			})
		}
	}
}

func pointDistance(p, a, b track.Vec3) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	u := ((p.X-a.X)*dx + (p.Y-a.Y)*dy) / (dx*dx + dy*dy)
	u = math.Max(0, math.Min(1, u))
	return math.Hypot(p.X-a.X-u*dx, p.Y-a.Y-u*dy)
}
func side(a, b, p track.Vec3) float64 { return (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X) }
func segmentDistance(a, b, c, d track.Vec3) float64 {
	if side(a, b, c)*side(a, b, d) < 0 && side(c, d, a)*side(c, d, b) < 0 {
		return 0
	}
	return math.Min(math.Min(pointDistance(a, c, d), pointDistance(b, c, d)), math.Min(pointDistance(c, a, b), pointDistance(d, a, b)))
}
func checkClearance(t *testing.T, r solver.Result, required float64) float64 {
	t.Helper()
	least := math.Inf(1)
	for i := 0; i < len(r.Nodes)-1; i++ {
		for j := 0; j < len(r.Road)-1; j++ {
			for _, sign := range []float64{-1, 1} {
				a, b := r.Road[j], r.Road[j+1]
				d := segmentDistance(r.Nodes[i].Position, r.Nodes[i+1].Position, a.AtOffset(physicalEdge(a, sign)), b.AtOffset(physicalEdge(b, sign)))
				least = math.Min(least, d)
				if d < required-1e-6 {
					t.Fatalf("path segment %d is %.9fm from road side %d, requires %.9fm circular clearance", i, d, j, required)
				}
			}
		}
	}
	return least
}

func triangleHeight(p, a, b, c track.Vec3) (float64, bool) {
	area := side(a, b, c)
	u, v, w := side(b, c, p)/area, side(c, a, p)/area, side(a, b, p)/area
	return u*a.Z + v*b.Z + w*c.Z, u >= -1e-8 && v >= -1e-8 && w >= -1e-8
}

// Compare the exported acceleration with Newton's law and the declared static
// axle friction circles, using road attributes reconstructed independently of
// the solver's evaluator. The authoritative triangle crossings split segments.
func checkForces(t *testing.T, r solver.Result, c vehicle.Config) float64 {
	t.Helper()
	anchors := make([]int, len(r.Road))
	cursor := 0
	for i, road := range r.Road {
		found := false
		for cursor < len(r.Nodes) {
			p := r.Nodes[cursor].Position.Sub(road.Position)
			if math.Abs(p.X*road.Normal.Y-p.Y*road.Normal.X) < 1e-5 && math.Hypot(p.X, p.Y) <= math.Max(physicalEdge(road, 1), -physicalEdge(road, -1))+1e-5 {
				anchors[i] = cursor
				cursor++
				found = true
				break
			}
			cursor++
		}
		if !found {
			t.Fatalf("cannot locate road station %d in exported trajectory", i)
		}
	}
	maxExcess := 0.
	for cell := 0; cell < len(r.Road)-1; cell++ {
		first, last := anchors[cell], anchors[cell+1]
		pa, pb := r.Nodes[first].Position, r.Nodes[last].Position
		dx, dy := pb.X-pa.X, pb.Y-pa.Y
		fraction := func(p track.Vec3) float64 { return ((p.X-pa.X)*dx + (p.Y-pa.Y)*dy) / (dx*dx + dy*dy) }
		for i := first; i < last; i++ {
			a, b := r.Nodes[i], r.Nodes[i+1]
			ra, rb := r.Road[cell], r.Road[cell+1]
			rightA, leftA := ra.AtOffset(physicalEdge(ra, -1)), ra.AtOffset(physicalEdge(ra, 1))
			rightB, leftB := rb.AtOffset(physicalEdge(rb, -1)), rb.AtOffset(physicalEdge(rb, 1))
			for _, p := range []track.Vec3{a.Position, a.Position.Add(b.Position).Mul(.5), b.Position} {
				z, inside := triangleHeight(p, rightA, rightB, leftB)
				if !inside {
					z, inside = triangleHeight(p, rightA, leftB, leftA)
				}
				if !inside || math.Abs(z-p.Z) > 1e-5 {
					t.Fatalf("segment %d position is outside its authoritative road surface", i)
				}
			}
			ds := b.Position.Sub(a.Position).Length()
			accel := (b.Speed*b.Speed - a.Speed*a.Speed) / (2 * ds)
			if math.Abs(accel-a.Acceleration) > 1e-7 {
				t.Fatalf("segment %d exported acceleration does not match kinematics", i)
			}
			duration := 2 * ds / (a.Speed + b.Speed)
			if math.Abs(duration-(b.Time-a.Time)) > 1e-7 {
				t.Fatalf("segment %d time does not match kinematics", i)
			}
			grade := (b.Position.Z - a.Position.Z) / math.Hypot(b.Position.X-a.Position.X, b.Position.Y-a.Position.Y)
			gradeCos := 1 / math.Sqrt(1+grade*grade)
			for j := 0; j <= 128; j++ {
				f := float64(j) / 128
				u := fraction(a.Position)*(1-f) + fraction(b.Position)*f
				wantStation := r.Road[cell].S*(1-u) + r.Road[cell+1].S*u
				if math.Abs(a.Station*(1-f)+b.Station*f-wantStation) > 1e-7 {
					t.Fatalf("segment %d station disagrees with reconstructed road fraction", i)
				}
				bank := (r.Road[cell].Bank*(1-u) + r.Road[cell+1].Bank*u) * math.Pi / 180
				k := a.Curvature*(1-f) + b.Curvature*f
				v := math.Sqrt(a.Speed*a.Speed*(1-f) + b.Speed*b.Speed*f)
				normal := vehicle.Gravity*gradeCos - v*v*gradeCos*gradeCos*k*math.Tan(bank)
				normal *= math.Cos(bank)
				lateral := v*v*gradeCos*gradeCos*k*math.Cos(bank) + vehicle.Gravity*gradeCos*math.Sin(bank)
				// Surface identity belongs to the outgoing source station. The production
				// solver may apply a still lower adjacent value conservatively.
				mu := ra.Grip
				if ra.KerbsCountAsRoad {
					offset := r.Nodes[first].Offset*(1-u) + r.Nodes[last].Offset*u
					left := ra.WidthLeft*(1-u) + rb.WidthLeft*u
					right := ra.WidthRight*(1-u) + rb.WidthRight*u
					if offset+c.Width/2 > left {
						mu = math.Min(mu, kerbFriction(ra.KerbLeft))
					}
					if offset-c.Width/2 < -right {
						mu = math.Min(mu, kerbFriction(ra.KerbRight))
					}
				}
				capacity := mu * c.Grip * normal
				if normal <= 0 || math.Abs(lateral) > capacity+1e-5 {
					t.Fatalf("segment %d has infeasible lateral force", i)
				}
				remaining := math.Sqrt(math.Max(0, capacity*capacity-lateral*lateral))
				drive := math.Inf(1)
				if c.FrontDrive > 0 {
					drive = math.Min(drive, remaining*c.FrontWeight/c.FrontDrive)
				}
				if c.FrontDrive < 1 {
					drive = math.Min(drive, remaining*(1-c.FrontWeight)/(1-c.FrontDrive))
				}
				if v > 0 {
					drive = math.Min(drive, c.Power/(c.Mass*v))
				}
				resistance := .5*vehicle.AirDensity*c.DragArea*v*v/c.Mass + vehicle.Gravity*grade*gradeCos
				tyreDemand := accel + resistance
				excess := math.Max(tyreDemand-drive, -tyreDemand-math.Min(c.Brake, remaining))
				maxExcess = math.Max(maxExcess, excess)
				if excess > 5e-4 {
					t.Fatalf("segment %d sample %d tyre force exceeds envelope by %.8g m/s²", i, j, excess)
				}
			}
		}
	}
	return maxExcess
}

// Reconstruct legal edge offsets from exported physical fields; do not use the
// solver's offset bounds or track's convenience boundary/footprint functions.
func physicalEdge(s track.Sample, side float64) float64 {
	left, right := s.WidthLeft, s.WidthRight
	if left == 0 && right == 0 {
		left, right = s.Width/2, s.Width/2
	}
	if s.KerbsCountAsRoad {
		left += s.KerbLeft.Width
		right += s.KerbRight.Width
	}
	if side > 0 {
		return left
	}
	return -right
}
func kerbFriction(k track.Kerb) float64 {
	if k.Width == 0 {
		return math.Inf(1)
	}
	if k.Grip > 0 {
		return k.Grip
	}
	if k.Surface == "" {
		return .8
	}
	for _, s := range track.Surfaces() {
		if s.Name == k.Surface {
			return s.Grip
		}
	}
	return 0
}
