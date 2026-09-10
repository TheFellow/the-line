package solver

import (
	"math"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

type constantModel struct {
	config                vehicle.Config
	acceleration, braking float64
}

func (m constantModel) Parameters() vehicle.Config { return m.config }
func (m constantModel) Limits(v, k, bank, grade, grip float64) vehicle.Envelope {
	return vehicle.Envelope{Acceleration: m.acceleration, Braking: m.braking, Feasible: true}
}
func fixture(t testing.TB, name string) (track.Scene, vehicle.Config) {
	t.Helper()
	s, e := track.Preset(name)
	if e != nil {
		t.Fatal(e)
	}
	v, e := vehicle.Preset(s.Vehicle)
	if e != nil {
		t.Fatal(e)
	}
	return s, v
}
func straight() track.Scene {
	return track.Scene{Version: 1, Name: "Straight oracle", Vehicle: "club", EntrySpeed: 20, ExitSpeed: 80, Points: []track.Point{{X: 0, Y: 0, Width: 12, Surface: "asphalt"}, {X: 200, Y: 0, Width: 12, Surface: "asphalt"}}}
}

func TestConstantAccelerationOracle(t *testing.T) {
	_, cfg := fixture(t, "hairpin")
	cfg.MaxSpeed = 80
	m := constantModel{cfg, 2, 4}
	s := straight()
	r, e := Solve(s, m, Options{Spacing: 2, Margin: .2})
	if e != nil {
		t.Fatal(e)
	}
	wantSpeed := math.Sqrt(s.EntrySpeed*s.EntrySpeed + 2*m.acceleration*200)
	wantTime := (wantSpeed - s.EntrySpeed) / m.acceleration
	if math.Abs(r.Duration-wantTime) > 1e-4 || math.Abs(r.Nodes[len(r.Nodes)-1].Speed-wantSpeed) > 1e-4 {
		t.Fatalf("duration=%g speed=%g; want %g %g", r.Duration, r.Nodes[len(r.Nodes)-1].Speed, wantTime, wantSpeed)
	}
	mid := r.At(r.Duration / 2)
	wantS := s.EntrySpeed*mid.Time + .5*m.acceleration*mid.Time*mid.Time
	if math.Abs(mid.S-wantS) > 1e-4 {
		t.Fatalf("animation station=%g; want %g", mid.S, wantS)
	}
	if r.At(-1) != r.Nodes[0] || r.At(r.Duration+1) != r.Nodes[len(r.Nodes)-1] {
		t.Fatal("At must clamp open endpoints")
	}
}

func TestLaunchStopAndRestrictiveCaps(t *testing.T) {
	_, cfg := fixture(t, "hairpin")
	cfg.MaxSpeed = 80
	m := constantModel{cfg, 2, 4}
	s := straight()
	s.EntrySpeed = 0
	s.ExitSpeed = 0
	r, e := Solve(s, m, Options{Spacing: 1, Margin: .2})
	if e != nil {
		t.Fatal(e)
	}
	peak := math.Sqrt(2 * 200 * m.acceleration * m.braking / (m.acceleration + m.braking))
	want := peak/m.acceleration + peak/m.braking
	if math.Abs(r.Duration-want)/want > .001 {
		t.Fatalf("launch/stop duration=%g want %g", r.Duration, want)
	}
	if r.Nodes[0].Speed != 0 || r.Nodes[len(r.Nodes)-1].Speed != 0 {
		t.Fatal("zero endpoint caps ignored")
	}
	s.EntrySpeed = 80
	s.ExitSpeed = 5
	r, e = Solve(s, m, Options{Spacing: 2, Margin: .2})
	if e != nil {
		t.Fatal(e)
	}
	wantEntry := math.Sqrt(25 + 2*4*200)
	if math.Abs(r.Nodes[0].Speed-wantEntry) > 1e-3 {
		t.Fatalf("entry cap not reduced for braking: %g want %g", r.Nodes[0].Speed, wantEntry)
	}
	if r.EntrySpeedCap != 80 || r.CenterEntrySpeed >= r.EntrySpeedCap {
		t.Fatal("boundary diagnostics omit slower realized entry")
	}
}

func TestPresetsImproveAndRemainFeasible(t *testing.T) {
	for _, name := range track.Presets() {
		t.Run(name, func(t *testing.T) {
			s, v := fixture(t, name)
			r, e := Solve(s, v, DefaultOptions())
			if e != nil {
				t.Fatal(e)
			}
			t.Logf("%s %.4fs baseline %.4fs improvement %.2f%% candidates %d nodes %d", name, r.Duration, r.CenterDuration, 100*(1-r.Duration/r.CenterDuration), r.Candidates, len(r.Nodes))
			if r.Duration > r.CenterDuration || r.Duration <= 0 {
				t.Fatal("invalid objective")
			}
			if r.Duration >= r.CenterDuration*.995 {
				t.Fatal("representative complex must improve by at least 0.5 percent")
			}
			verifyResult(t, r, s, v, DefaultOptions())
		})
	}
}

func verifyResult(t *testing.T, r Result, s track.Scene, v vehicle.Config, opts Options) {
	t.Helper()
	offsets := make([]float64, len(r.Road))
	cursor := 0
	for i, road := range r.Road {
		found := false
		for cursor < len(r.Nodes) {
			p := r.Nodes[cursor].Position
			dx, dy := p.X-road.Position.X, p.Y-road.Position.Y
			if math.Abs(dx*road.Normal.Y-dy*road.Normal.X) < 1e-6 {
				offsets[i] = dx*road.Normal.X + dy*road.Normal.Y
				if math.Abs(offsets[i])+v.Width/2+opts.Margin > road.Width/2+1e-6 {
					t.Fatal("clearance exceeded")
				}
				found = true
				cursor++
				break
			}
			cursor++
		}
		if !found {
			t.Fatalf("missing road station %d", i)
		}
	}
	eval := evaluator{road: r.Road, model: v, entry: s.EntrySpeed, exit: s.ExitSpeed, clearance: v.Width/2 + opts.Margin}
	p, e := eval.geometry(offsets)
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != len(r.Nodes) {
		t.Fatal("path surface reconstruction differs")
	}
	fine, err := track.SampleRoad(s, .25)
	if err != nil {
		t.Fatal(err)
	}
	denseEval := eval
	denseEval.road = fine
	dense, err := denseEval.run(interpolateOffsets(r.Road, offsets, fine, v.Width/2+opts.Margin))
	if err != nil {
		t.Fatal(err)
	}
	change := math.Abs(dense.Duration-r.Duration) / dense.Duration
	t.Logf("verified %.2fm to .25m: %.3f%% (%g to %g)", r.Spacing, change*100, r.Duration, dense.Duration)
	if change > .02 {
		t.Fatalf("verified path refinement exceeds 2 percent: %.3f%%", change*100)
	}
	for i, n := range r.Nodes {
		if !finite(n.Speed) || !finite(n.Time) || n.Speed < 0 {
			t.Fatal("invalid speed/time")
		}
		if distance(n.Position, p[i].node.Position) > 1e-6 {
			t.Fatal("line does not lie on authoritative surface")
		}
		if i == len(r.Nodes)-1 {
			break
		}
		next := r.Nodes[i+1]
		if next.Time <= n.Time || next.S <= n.S {
			t.Fatal("nonmonotonic trajectory")
		}
		a, b := n.Position, next.Position
		grade := (b.Z - a.Z) / math.Hypot(b.X-a.X, b.Y-a.Y)
		drive, brake := eval.limits(p[i], p[i+1], grade, n.Speed, next.Speed, 65)
		if n.Acceleration > drive+5e-5 || -n.Acceleration > brake+5e-5 {
			t.Fatalf("segment force violation at %d: a=%g limits %g %g", i, n.Acceleration, drive, brake)
		}
		for j := 0; j < 9; j++ {
			mid := r.At(n.Time + (next.Time-n.Time)*float64(j)/8)
			if mid.S < n.S-1e-6 || mid.S > next.S+1e-6 {
				t.Fatal("animation escaped segment")
			}
		}
	}
}

func TestDeterministic(t *testing.T) {
	s, v := fixture(t, "esses")
	opts := DefaultOptions()
	opts.Iterations = 1
	a, e := Solve(s, v, opts)
	if e != nil {
		t.Fatal(e)
	}
	b, e := Solve(s, v, opts)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("solver is nondeterministic")
	}
}

func TestCircleCurvatureAndBankSymmetry(t *testing.T) {
	const radius = 50.
	road := make([]track.Sample, 101)
	offset := make([]float64, len(road))
	for i := range road {
		theta := float64(i) * math.Pi / 200
		road[i] = track.Sample{Position: track.Vec3{X: radius * math.Cos(theta), Y: radius * math.Sin(theta)}, Normal: track.Vec3{X: -math.Cos(theta), Y: -math.Sin(theta)}, Width: 10, Bank: -10, Grip: 1}
	}
	e := evaluator{road: road, clearance: 1}
	p, err := e.geometry(offset)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range p {
		if math.Abs(s.node.Curvature-1/radius) > 1e-9 {
			t.Fatalf("circle curvature=%g want %g", s.node.Curvature, 1/radius)
		}
	}
	_, v := fixture(t, "banked")
	e.model = v
	e.entry = 80
	e.exit = 80
	r, err := e.run(offset)
	if err != nil {
		t.Fatal(err)
	}
	for i := range road {
		road[i].Position.Y = -road[i].Position.Y
		road[i].Normal.X = -road[i].Normal.X
		road[i].Bank = -road[i].Bank
	}
	r2, err := e.run(offset)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.Duration-r2.Duration) > 2e-6 {
		t.Fatalf("bank/curvature reflection differs: %g %g", r.Duration, r2.Duration)
	}
}

func TestRefinement(t *testing.T) {
	for _, name := range []string{"hairpin", "banked", "rally"} {
		t.Run(name, func(t *testing.T) {
			s, v := fixture(t, name)
			opts := DefaultOptions()
			opts.Iterations = 0
			opts.Spacing = .5
			a, e := Solve(s, v, opts)
			if e != nil {
				t.Fatal(e)
			}
			opts.Spacing /= 2
			b, e := Solve(s, v, opts)
			if e != nil {
				t.Fatal(e)
			}
			delta := math.Abs(a.Duration-b.Duration) / b.Duration
			t.Logf("spacing .5m %.4fs, .25m %.4fs: %.3f%%", a.Duration, b.Duration, 100*delta)
			if delta > .01 {
				t.Fatalf("baseline refinement exceeds 1 percent: %.3f%%", 100*delta)
			}
		})
	}
}

func TestRejectUnusableRoadAndOptions(t *testing.T) {
	s, v := fixture(t, "hairpin")
	opts := DefaultOptions()
	opts.Margin = 10
	if _, e := Solve(s, v, opts); e == nil {
		t.Fatal("vehicle wider than road accepted")
	}
	opts = DefaultOptions()
	opts.Spacing = math.NaN()
	if _, e := Solve(s, v, opts); e == nil {
		t.Fatal("NaN spacing accepted")
	}
	s = straight()
	for i := range s.Points {
		s.Points[i].Bank = 35
		s.Points[i].Surface = "ice"
	}
	if _, e := Solve(s, v, DefaultOptions()); e == nil {
		t.Fatal("stationary cross-slope sliding accepted")
	}
}

func BenchmarkSolveEsses(b *testing.B) {
	s, v := fixture(b, "esses")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, e := Solve(s, v, DefaultOptions()); e != nil {
			b.Fatal(e)
		}
	}
}

func TestLowGripTransitionBrakesBeforeBoundary(t *testing.T) {
	_, v := fixture(t, "hairpin")
	s := straight()
	s.EntrySpeed = 80
	s.ExitSpeed = 5
	s.Points = []track.Point{{X: 0, Width: 12, Surface: "asphalt"}, {X: 80, Width: 12, Surface: "asphalt"}, {X: 100, Width: 12, Surface: "ice"}, {X: 200, Width: 12, Surface: "ice"}}
	opts := DefaultOptions()
	opts.Iterations = 0
	r, e := Solve(s, v, opts)
	if e != nil {
		t.Fatal(e)
	}
	dry := s
	dry.Points = append([]track.Point(nil), s.Points...)
	for i := range dry.Points {
		dry.Points[i].Surface = "asphalt"
	}
	baseline, e := Solve(dry, v, opts)
	if e != nil {
		t.Fatal(e)
	}
	speedAt := func(result Result, x float64) float64 {
		for _, n := range result.Nodes {
			if n.Position.X >= x {
				return n.Speed
			}
		}
		return 0
	}
	if speedAt(r, 90) >= speedAt(baseline, 90)-1 {
		t.Fatal("low grip downstream must cause earlier braking")
	}
	if r.Duration <= baseline.Duration {
		t.Fatal("ice unexpectedly faster than asphalt")
	}
	// Removing the downstream braking requirement permits a faster arrival at
	// the transition: the complete sequence constrains the earlier road section.
	prefix := s
	prefix.Points = append([]track.Point(nil), s.Points[:2]...)
	prefix.ExitSpeed = 80
	alone, e := Solve(prefix, v, opts)
	if e != nil {
		t.Fatal(e)
	}
	if speedAt(r, 80) >= alone.Nodes[len(alone.Nodes)-1].Speed {
		t.Fatal("downstream segment did not constrain upstream speed")
	}
}
