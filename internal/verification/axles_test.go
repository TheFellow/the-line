package verification

import (
	"math"
	"sort"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Reconstruct Newton's law and both axle circles directly from exported road,
// geometry and speeds. No production envelope or solver helper is consulted.
func TestEnhancedAxlesIndependentTrajectory(t *testing.T) {
	for _, preset := range []string{"esses", "banked"} {
		t.Run(preset, func(t *testing.T) {
			scene, _ := track.Preset(preset)
			c, _ := vehicle.Preset("gt")
			c.FrontBrake = .62
			c.LiftArea = 2.5
			c.AeroBalance = .45
			c.CGHeight = .3
			c.Wheelbase = 2.7
			c.LoadSensitivity = .08
			opts := solver.DefaultOptions()
			opts.Iterations = 0
			result, err := solver.Solve(scene, c, opts)
			if err != nil {
				t.Fatal(err)
			}
			for _, nodes := range [][]solver.Node{result.Nodes, result.CenterNodes} {
				for i := 0; i < len(nodes)-1; i++ {
					a, b := nodes[i], nodes[i+1]
					ds := b.Position.Sub(a.Position).Length()
					acceleration := (b.Speed*b.Speed - a.Speed*a.Speed) / (2 * ds)
					grade := (b.Position.Z - a.Position.Z) / math.Hypot(b.Position.X-a.Position.X, b.Position.Y-a.Position.Y)
					cg := 1 / math.Sqrt(1+grade*grade)
					for j := 0; j <= 16; j++ {
						u := float64(j) / 16
						station := a.Station*(1-u) + b.Station*u
						cell := sort.Search(len(result.Road), func(k int) bool { return result.Road[k].S > station }) - 1
						cell = max(0, min(cell, len(result.Road)-2))
						ra, rb := result.Road[cell], result.Road[cell+1]
						f := (station - ra.S) / (rb.S - ra.S)
						bank := (ra.Bank*(1-f) + rb.Bank*f) * math.Pi / 180
						v2 := a.Speed*a.Speed*(1-u) + b.Speed*b.Speed*u
						k := a.Curvature*(1-u) + b.Curvature*u
						lateral := v2*cg*cg*k*math.Cos(bank) + vehicle.Gravity*cg*math.Sin(bank)
						ground := vehicle.Gravity*cg*math.Cos(bank) - v2*cg*cg*k*math.Sin(bank)
						aero := .5 * vehicle.AirDensity * c.LiftArea * v2 / c.Mass
						drag := .5 * vehicle.AirDensity * c.DragArea * v2 / c.Mass
						longitudinal := acceleration + drag + vehicle.Gravity*grade*cg
						transfer := longitudinal * c.CGHeight / c.Wheelbase
						normalF := ground*c.FrontWeight + aero*c.AeroBalance - transfer
						normalR := ground*(1-c.FrontWeight) + aero*(1-c.AeroBalance) + transfer
						if normalF <= 0 || normalR <= 0 {
							t.Fatalf("axle lift-off at node %d", i)
						}
						cf := ra.Grip * c.Grip * normalF * math.Pow(normalF/(vehicle.Gravity*c.FrontWeight), -c.LoadSensitivity)
						cr := ra.Grip * c.Grip * normalR * math.Pow(normalR/(vehicle.Gravity*(1-c.FrontWeight)), -c.LoadSensitivity)
						q := c.FrontDrive
						if longitudinal < 0 {
							q = c.FrontBrake
						}
						excess := math.Max(math.Hypot(longitudinal*q, lateral*c.FrontWeight)-cf, math.Hypot(longitudinal*(1-q), lateral*(1-c.FrontWeight))-cr)
						if excess > 5e-4 {
							t.Fatalf("node %d sample %d axle excess %.9g", i, j, excess)
						}
						if longitudinal > 0 && math.Sqrt(v2) > 1e-6 && longitudinal > c.Power/(c.Mass*math.Sqrt(v2))+5e-4 {
							t.Fatal("power exceeded")
						}
						if longitudinal < -c.Brake-5e-4 {
							t.Fatal("brake cap exceeded")
						}
					}
				}
			}
		})
	}
}
