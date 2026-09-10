package racecraft_test

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/TheFellow/the-line/pkg/racecraft"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestExamples(t *testing.T) {
	v, _ := vehicle.Preset("road")
	for _, name := range racecraft.Scenarios() {
		t.Run(name, func(t *testing.T) {
			s, _ := racecraft.Scene(name)
			c := racecraft.DefaultConfig(name)
			r, err := racecraft.Plan(context.Background(), s, v, c)
			if err != nil {
				t.Fatal(err)
			}
			again, err := racecraft.Plan(context.Background(), s, v, c)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(r)
			b, _ := json.Marshal(again)
			if string(a) != string(b) {
				t.Fatal("nondeterministic replay")
			}
			checkFirstFinish(t, r)
			if r.MinClearance < c.Clearance {
				t.Fatal("uncertified clearance")
			}
			start := r.At(0)
			if math.Abs(start[0].Station-start[1].Station-c.Gap) > 1e-8 {
				t.Fatal("changed initial gap")
			}
			for tm := 0.; tm < r.Duration; tm += .0037 {
				n := r.At(tm)
				d := math.Hypot(n[0].Position.X-n[1].Position.X, n[0].Position.Y-n[1].Position.Y)
				if r.MinClearance > d-r.Cars[0].Radius-r.Cars[1].Radius+1e-9 {
					t.Fatal("certified lower bound exceeds sampled body clearance")
				}
				if d < r.Cars[0].Radius+r.Cars[1].Radius+c.Clearance {
					t.Fatalf("body overlap at %g", tm)
				}
			}
			for _, car := range r.Cars {
				for i, n := range car.Path.Nodes {
					// Reconstruct the outgoing segment's kinematics and check against the
					// public force envelope, independently of racecraft's collision/planner code.
					if i+1 == len(car.Path.Nodes) {
						break
					}
					next := car.Path.Nodes[i+1]
					ds := math.Sqrt(math.Pow(next.Position.X-n.Position.X, 2) + math.Pow(next.Position.Y-n.Position.Y, 2) + math.Pow(next.Position.Z-n.Position.Z, 2))
					acceleration := (next.Speed*next.Speed - n.Speed*n.Speed) / (2 * ds)
					if math.Abs(next.Time-n.Time-2*ds/(n.Speed+next.Speed)) > 1e-8 {
						t.Fatal("time inconsistent with distance")
					}
					envelope := v.Limits(n.Speed, n.Curvature, n.Bank, n.Grade, n.Grip)
					if !envelope.Feasible || acceleration > envelope.Acceleration+.001 || acceleration < -envelope.Braking-.001 {
						t.Fatal("force envelope exceeded")
					}
					// Full disc clearance to all physical side segments, including neighboring
					// cells. The road's open endpoint cross sections are deliberately not walls.
					for j := 0; j+1 < len(car.Path.Road); j++ {
						for _, sign := range []float64{-1, 1} {
							p, q := car.Path.Road[j], car.Path.Road[j+1]
							left, right := p.AtOffset(p.LeftLimit()), q.AtOffset(q.LeftLimit())
							if sign < 0 {
								left, right = p.AtOffset(-p.RightLimit()), q.AtOffset(-q.RightLimit())
							}
							if segmentDistance(n.Position, next.Position, left, right) < car.Radius+.2499 {
								t.Fatalf("body leaves road at %.2f", n.Station)
							}
						}
					}
				}
			}
			switch name {
			case "over-under":
				if len(r.Events) != 1 || r.Events[0].Leader != 1 {
					t.Fatalf("missing exit pass: %+v", r.Events)
				}
			case "pass-repass":
				if len(r.Events) != 2 || r.Events[0].Leader != 1 || r.Events[1].Leader != 0 {
					t.Fatalf("missing repass: %+v", r.Events)
				}
			case "defend":
				if len(r.Events) != 0 {
					t.Fatal("defence failed")
				}
			}
		})
	}
}
func distance(p, a, b track.Vec3) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	u := math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p.X-a.X-u*dx, p.Y-a.Y-u*dy)
}

func TestControlsAndRejection(t *testing.T) {
	s, _ := racecraft.Scene("over-under")
	v, _ := vehicle.Preset("road")
	c := racecraft.DefaultConfig("over-under")
	base, err := racecraft.Plan(context.Background(), s, v, c)
	if err != nil {
		t.Fatal(err)
	}
	c.Gap = 14
	r, err := racecraft.Plan(context.Background(), s, v, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 0 {
		t.Fatal("outcome forced despite larger gap")
	}
	c = racecraft.DefaultConfig("over-under")
	c.Overspeed = 0
	r, err = racecraft.Plan(context.Background(), s, v, c)
	if err != nil {
		t.Fatal(err)
	}
	if r.At(0)[1].Speed == base.At(0)[1].Speed {
		t.Fatal("overspeed ignored")
	}
	c = racecraft.DefaultConfig("defend")
	c.Separation = 5.5
	r, err = racecraft.Plan(context.Background(), s, v, c)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(r.Cars[0].Path.Offsets, base.Cars[0].Path.Offsets) {
		t.Fatal("placement ignored")
	}
	c.Gap = 0
	c.Separation = 2
	if _, err = racecraft.Plan(context.Background(), s, v, c); err == nil {
		t.Fatal("accepted initially overlapping cars")
	}
	c.Gap = math.NaN()
	if c.Validate() == nil {
		t.Fatal("accepted NaN")
	}
	if _, err := racecraft.Scene("unknown"); err == nil {
		t.Fatal("unknown scenario silently selected a road")
	}
	c = racecraft.DefaultConfig("unknown")
	if c.Validate() == nil {
		t.Fatal("accepted unknown scenario")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = racecraft.Plan(ctx, s, v, racecraft.DefaultConfig("defend")); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
	s.Closed = true
	if _, err = racecraft.Plan(context.Background(), s, v, racecraft.DefaultConfig("defend")); err == nil {
		t.Fatal("accepted periodic race")
	}
}

func TestOccupiedLineAdaptsPlacementAndArrival(t *testing.T) {
	e, _ := racecraft.Example("over-under")
	e.Config.Gap = 3.25
	e.Config.Separation = 5
	r, err := racecraft.Plan(context.Background(), e.Scene, e.Vehicle, e.Config)
	if err != nil {
		t.Fatal(err)
	}
	if r.Candidates != 2 {
		t.Fatalf("expected occupied first choice to trigger adaptation, got %d candidates", r.Candidates)
	}
	if r.Cars[1].Path.EntrySpeedCap >= e.Scene.EntrySpeed+e.Config.Overspeed {
		t.Fatal("attacker failed to give room on arrival")
	}
	if r.Cars[1].Path.Nodes[0].Offset >= -e.Config.Separation/2 {
		t.Fatal("attacker failed to widen placement")
	}
	for tm := 0.; tm < r.Duration; tm += .0023 {
		n := r.At(tm)
		if math.Hypot(n[0].Position.X-n[1].Position.X, n[0].Position.Y-n[1].Position.Y) < r.Cars[0].Radius+r.Cars[1].Radius+e.Config.Clearance {
			t.Fatal("adapted plan overlaps")
		}
	}
}

func segmentDistance(a, b, c, d track.Vec3) float64 {
	cross := func(p, q, r track.Vec3) float64 { return (q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X) }
	if cross(a, b, c)*cross(a, b, d) < 0 && cross(c, d, a)*cross(c, d, b) < 0 {
		return 0
	}
	return math.Min(math.Min(distance(a, c, d), distance(b, c, d)), math.Min(distance(c, a, b), distance(d, a, b)))
}

func TestEntrySpeedCaps(t *testing.T) {
	v, err := vehicle.Preset("road")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		entry, advantage float64
		wantB            float64
		fallback         bool
	}{
		{name: "zero", entry: 0, wantB: 0},
		{name: "fractional", entry: .25, wantB: .25},
		{name: "fractional after advantage", entry: 2, advantage: -1.5, wantB: .5},
		{name: "negative effective cap", entry: 2, advantage: -3, wantB: 0},
		{name: "negative fallback cap", entry: .5, wantB: 0, fallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := track.Scene{Version: 1, Name: "Entry caps", Vehicle: "road", EntrySpeed: tc.entry, ExitSpeed: 30,
				Points: []track.Point{{Width: 20, Surface: "asphalt"}, {X: 200, Width: 20, Surface: "asphalt"}}}
			c := racecraft.DefaultConfig("defend")
			c.Overspeed = tc.advantage
			wantCandidates := 1
			if tc.fallback {
				// Initial body overlap rejects the first two lane placements.
				c.Gap, c.Separation = 0, 4.5
				wantCandidates = 3
			}
			r, err := racecraft.Plan(context.Background(), s, v, c)
			if err != nil {
				t.Fatal(err)
			}
			if r.Candidates != wantCandidates {
				t.Fatalf("candidates = %d, want %d", r.Candidates, wantCandidates)
			}
			for i, wantCap := range []float64{tc.entry, tc.wantB} {
				path := r.Cars[i].Path
				if path.EntrySpeedCap != wantCap {
					t.Errorf("car %d entry cap = %g, want %g", i, path.EntrySpeedCap, wantCap)
				}
				// On this unconstrained straight, the initial speed should reach the
				// cap, including a standstill launch when the effective cap is zero.
				if math.Abs(path.Nodes[0].Speed-wantCap) > 1e-9 {
					t.Errorf("car %d entry speed = %g, want %g", i, path.Nodes[0].Speed, wantCap)
				}
				if path.ExitSpeedCap != s.ExitSpeed || path.Nodes[len(path.Nodes)-1].Speed > s.ExitSpeed+1e-9 {
					t.Errorf("car %d exit cap was not respected", i)
				}
			}
			if r.At(0)[1].Speed > tc.wantB+1e-9 {
				t.Errorf("B starts above its effective cap: %g > %g", r.At(0)[1].Speed, tc.wantB)
			}
		})
	}
}

func TestPlanningFailureCauses(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		edit       func(*track.Scene, *racecraft.Config)
	}{
		{"wide placement", "road fit", func(s *track.Scene, c *racecraft.Config) { c.Separation = 6.75 }},
		{"narrow road", "vehicle clearance", func(s *track.Scene, c *racecraft.Config) {
			s.Points = []track.Point{{Width: 4, Surface: "asphalt"}, {X: 200, Width: 4, Surface: "asphalt"}}
		}},
		{"short road", "road length", func(s *track.Scene, c *racecraft.Config) {
			s.Points = []track.Point{{Width: 20, Surface: "asphalt"}, {X: 8, Width: 20, Surface: "asphalt"}}
			c.Gap = 0
		}},
		{"infeasible bank", "stationary grip", func(s *track.Scene, c *racecraft.Config) {
			s.Points = []track.Point{{Width: 20, Surface: "ice", Bank: 20}, {X: 200, Width: 20, Surface: "ice", Bank: 20}}
		}},
		{"initial overlap", "body-clearance certification", func(s *track.Scene, c *racecraft.Config) {
			c.Gap, c.Separation = 0, 2
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := racecraft.Scene("over-under")
			v, _ := vehicle.Preset("road")
			c := racecraft.DefaultConfig("over-under")
			tc.edit(&s, &c)
			_, err := racecraft.Plan(context.Background(), s, v, c)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want cause %q", err, tc.want)
			}
			if tc.name != "initial overlap" && strings.Contains(err.Error(), "collision") {
				t.Fatalf("non-collision failure misattributed: %v", err)
			}
			if tc.name == "infeasible bank" && !strings.Contains(err.Error(), "solver feasibility") {
				t.Fatalf("missing solver context: %v", err)
			}
		})
	}
}

func checkFirstFinish(t *testing.T, r racecraft.Result) {
	t.Helper()
	end := r.At(r.Duration)
	before := r.At(r.Duration - 1e-4)
	finished := 0
	for i, car := range r.Cars {
		last := car.Path.Nodes[len(car.Path.Nodes)-1]
		if math.Abs(end[i].Station-last.Station) < 1e-8 {
			finished++
		}
		if before[i].Station >= last.Station {
			t.Fatalf("car %d finished before the shared end", i)
		}
		if r.Duration+car.StartTime < car.Path.Duration-1e-8 && end[i].Station >= last.Station {
			t.Fatalf("car %d parked at its endpoint before finishing", i)
		}
	}
	if finished == 0 {
		t.Fatal("neither car finished at the shared end")
	}
	if got := r.At(r.Duration + 10); got != end {
		t.Fatal("replay continues past its certified first finish")
	}
	distance := math.Hypot(end[0].Position.X-end[1].Position.X, end[0].Position.Y-end[1].Position.Y)
	if r.MinClearance > distance-r.Cars[0].Radius-r.Cars[1].Radius+1e-9 {
		t.Fatal("certified lower bound exceeds finish body clearance")
	}
}

func TestCustomBankedNonuniformRoadAndVehicles(t *testing.T) {
	s := track.Scene{Version: 1, Name: "Banked custom straight", EntrySpeed: 20, ExitSpeed: 35}
	for x := 0.; x <= 30; x++ {
		s.Points = append(s.Points, track.Point{X: x, Width: 20, Bank: 5, Surface: "asphalt"})
	}
	s.Points = append(s.Points, track.Point{X: 203, Width: 20, Bank: 5, Surface: "asphalt"})
	for _, name := range vehicle.Presets() {
		t.Run(name, func(t *testing.T) {
			v, err := vehicle.Preset(name)
			if err != nil {
				t.Fatal(err)
			}
			c := racecraft.DefaultConfig("defend")
			r, err := racecraft.Plan(context.Background(), s, v, c)
			if err != nil {
				t.Fatal(err)
			}
			checkFirstFinish(t, r)
			start := r.At(0)
			if math.Abs(start[0].Station-start[1].Station-c.Gap) > 1e-8 {
				t.Fatal("custom road changed initial gap")
			}
			// The retained scene cap applies at station zero. A has already
			// accelerated over the gap when the shared experiment starts.
			if start[0].Speed <= s.EntrySpeed || start[1].Speed > s.EntrySpeed+1e-9 {
				t.Fatalf("unexpected shared start speeds: A=%g B=%g", start[0].Speed, start[1].Speed)
			}
			for tm := 0.; tm <= r.Duration; tm = math.Min(tm+.007, r.Duration) {
				n := r.At(tm)
				gap := math.Hypot(n[0].Position.X-n[1].Position.X, n[0].Position.Y-n[1].Position.Y) - r.Cars[0].Radius - r.Cars[1].Radius
				if gap < c.Clearance || r.MinClearance > gap+1e-9 {
					t.Fatalf("invalid certified clearance at %g: certified=%g sampled=%g", tm, r.MinClearance, gap)
				}
				if tm == r.Duration {
					break
				}
			}
		})
	}
}
