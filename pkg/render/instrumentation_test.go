package render

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestInstrumentationSharedScalesAndCursors(t *testing.T) {
	s, _ := track.Preset("hairpin")
	c, _ := vehicle.Preset("gt")
	run, err := solver.Solve(s, c, solver.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	markers := solver.DetectMarkers(run.Nodes)
	if len(markers) < 2 {
		t.Fatal("full-budget fixture has no corner markers")
	}
	for _, view := range []string{"2d", "3d"} {
		for _, size := range [][2]int{{1440, 900}, {1000, 700}} {
			r, err := New(s, run, c, Options{View: view, Width: size[0], Height: size[1]})
			if err != nil {
				t.Fatal(err)
			}
			for name, control := range r.Controls() {
				if control.Overlaps(r.RoadViewport()) {
					t.Fatalf("%s HUD control covers the interactive road viewport", name)
				}
			}
			n := run.At(run.Duration * .45)
			x := r.chartPoint(n).x
			for _, channel := range Channels() {
				if r.LineColorMode() != channel || r.ChartChannel() != channel {
					t.Fatal("channel cycle order changed")
				}
				low, high, _ := r.ChartScale()
				p := r.chartPoint(n)
				wantY := 722 - (channel.value(n)-low)/(high-low)*76
				if p.x != x || math.Abs(p.y-wantY) > 1e-10 {
					t.Fatal("chart channel moved station or used inconsistent scale")
				}
				before := run.Duration
				r.FrameWithState(n.Time, State{Selected: -1, Comparison: true})
				if run.Duration != before {
					t.Fatal("instrumentation changed numerical result")
				}
				if _, ok := r.Controls()["line-color"]; !ok {
					t.Fatal("missing line color control")
				}
				if _, ok := r.Controls()["chart-channel"]; !ok {
					t.Fatal("missing chart channel control")
				}
				r.CycleLineColor()
				r.CycleChartChannel()
			}
			for _, elapsed := range []float64{0, markers[0].Time, run.Duration} {
				n := run.At(elapsed)
				ref := run.CenterAt(elapsed)
				dot := r.ForceCursor(elapsed)
				scale := math.Max(2*vehicle.Gravity, math.Ceil(math.Max(n.Forces.Capacity, ref.Forces.Capacity)/vehicle.Gravity)*vehicle.Gravity)
				wantX := r.displayX + (410+n.Forces.Lateral/scale*31)*r.displayScale
				wantY := r.displayY + (839-n.Forces.Longitudinal/scale*31)*r.displayScale
				if math.Abs(dot[0]-wantX) > 1e-10 || math.Abs(dot[1]-wantY) > 1e-10 {
					t.Fatal("force widget disagrees with current car")
				}
			}
			extreme := run.Nodes[0]
			extreme.Speed = 200
			if r.chartPoint(extreme).y != float64(r.controls["chart"].Min.Y) {
				t.Fatal("clipped speed cursor escaped chart into the road")
			}
		}
	}
}

func TestFixedColorScaleDoesNotDependOnRunExtrema(t *testing.T) {
	n := solver.Node{Speed: 30, Forces: vehicle.TyreForces{TyreEnvelope: vehicle.TyreEnvelope{Available: true, Lateral: vehicle.Gravity}, Longitudinal: -.5 * vehicle.Gravity, Utilization: .8}}
	for _, channel := range Channels() {
		first := channel.color(n)
		other := n
		other.Station = 200
		other.Time = 20
		other.Curvature = .08
		if channel.color(other) != first {
			t.Fatal("same physical channel value changed color across trajectory contexts")
		}
	}
	if SpeedChannel.color(solver.Node{Speed: 0}) == SpeedChannel.color(solver.Node{Speed: 300 / 3.6}) {
		t.Fatal("speed color scale is not legible")
	}
}

func TestSpeedChartFitsBothFullTracesAndRemainsStable(t *testing.T) {
	bounds := speedChartBounds([]solver.Node{{Speed: 70 / 3.6}, {Speed: 150 / 3.6}}, []solver.Node{{Speed: 60 / 3.6}, {Speed: 170 / 3.6}})
	if bounds[0] >= 60 || bounds[1] <= 170 || bounds[1]-bounds[0] >= 300 {
		t.Fatalf("unhelpful shared scale: %v", bounds)
	}
	s, result, car := comparisonFixture(t)
	r, err := New(s, result, car, Options{})
	if err != nil {
		t.Fatal(err)
	}
	low, high, _ := r.ChartScale()
	for _, elapsed := range []float64{0, result.Duration / 2, result.Duration} {
		r.FrameWithState(elapsed, State{Comparison: true})
		a, b, _ := r.ChartScale()
		if a != low || b != high {
			t.Fatal("scrubbing changed the comparison scale")
		}
	}
	flat := speedChartBounds([]solver.Node{{Speed: 30}, {Speed: 30}})
	if flat[0] >= 108 || flat[1] <= 108 {
		t.Fatal("constant-speed trace lacks headroom")
	}
}
