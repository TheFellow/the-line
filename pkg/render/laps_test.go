package render

import (
	"image"
	"math"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestClosedPlaybackAndInspectionAgree(t *testing.T) {
	scene, err := track.Preset("club-loop")
	if err != nil {
		t.Fatal(err)
	}
	car, _ := vehicle.Preset("gt")
	current, err := solver.Evaluate(scene, car, nil, solver.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	slower := car
	slower.Grip *= .65
	reference, err := solver.Evaluate(scene, slower, nil, solver.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if reference.Duration <= current.Duration {
		t.Fatal("fixture reference must have a longer lap period")
	}
	pin := PinReference("Lower grip", scene, reference, slower)
	for _, view := range []string{"2d", "3d", "perspective"} {
		t.Run(view, func(t *testing.T) {
			r, err := New(scene, current, car, Options{Width: 1440, Height: 900, View: view, Reference: &pin})
			if err != nil {
				t.Fatal(err)
			}
			if r.PlaybackDuration(true) != current.Duration {
				t.Fatal("closed transport must cover one current lap")
			}
			for _, elapsed := range []float64{0, current.Duration, current.Duration * 1.35, reference.Duration * 2.2} {
				n := current.At(math.Mod(elapsed, current.Duration))
				ref := reference.At(math.Mod(elapsed, reference.Duration))
				if got := r.referenceAt(elapsed); !reflect.DeepEqual(got, ref) {
					t.Fatal("ghost did not use its own lap period")
				}
				cursor := r.ChartCursor(elapsed)
				chart := r.controls["chart"]
				wantX := float64(chart.Min.X) + n.Station/current.Nodes[len(current.Nodes)-1].Station*float64(chart.Dx())
				if math.Abs(cursor[0]-wantX) > 1e-9 {
					t.Fatal("chart cursor did not follow the wrapped road station")
				}
				scale := math.Max(2*vehicle.Gravity, math.Ceil(math.Max(n.Forces.Capacity, ref.Forces.Capacity)/vehicle.Gravity)*vehicle.Gravity)
				force := r.ForceCursor(elapsed)
				if math.Abs(force[0]-(410+n.Forces.Lateral/scale*31)) > 1e-9 || math.Abs(force[1]-(839-n.Forces.Longitudinal/scale*31)) > 1e-9 {
					t.Fatal("force cursor does not follow independently wrapped cars")
				}
				frame := r.Frame(elapsed).(*image.RGBA)
				if got := frame.RGBAAt(int(cursor[0]), int(cursor[1])); got != accent {
					t.Fatalf("chart query disagrees with rendered cursor: got %v", got)
				}
			}
			paused := State{Comparison: true, Selected: -1}
			end := r.ChartCursorWithState(current.Duration, paused)
			if end[0] != float64(r.controls["chart"].Max.X) {
				t.Fatal("paused end inspection wrapped to the beginning")
			}
			frame := r.FrameWithState(current.Duration, paused).(*image.RGBA)
			if frame.RGBAAt(int(end[0]), int(end[1])) != accent {
				t.Fatal("paused frame did not render the end inspection cursor")
			}
			if r.ChartCursorWithState(current.Duration*1.35, paused) != r.ChartCursor(current.Duration*1.35) {
				t.Fatal("pausing on a subsequent lap changed its inspected station")
			}
			var currentTotal, referenceTotal float64
			sectors := r.Sectors()
			if len(sectors) != len(scene.Points) {
				t.Fatal("closed sectors omitted the final-to-first control interval")
			}
			for _, sector := range sectors {
				currentTotal += sector.Current
				referenceTotal += sector.Reference
			}
			if math.Abs(currentTotal-current.Duration) > 1e-9 || math.Abs(referenceTotal-reference.Duration) > 1e-9 {
				t.Fatal("sector times do not cover both complete laps")
			}
		})
	}
}

func TestCenterReferencePreservesLiveForceQueries(t *testing.T) {
	scene, _ := track.Preset("banked")
	car, _ := vehicle.Preset("gt")
	car.LiftArea = 3
	run, err := solver.Evaluate(scene, car, nil, solver.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(scene, run, car, Options{View: "3d"})
	if err != nil {
		t.Fatal(err)
	}
	ref := r.referenceTrajectory()
	for i := 0; i+1 < len(run.CenterNodes); i += 7 {
		elapsed := (run.CenterNodes[i].Time + run.CenterNodes[i+1].Time) / 2
		if !reflect.DeepEqual(ref.At(elapsed), run.CenterAt(elapsed)) {
			t.Fatal("centreline conversion lost the live force model at an interpolated query")
		}
	}
}

func TestOpenFrameClampsClockToVisibleFinish(t *testing.T) {
	r := &Renderer{result: solver.Result{
		Duration: 10, CenterDuration: 15,
		CenterNodes: []solver.Node{{Time: 0}, {Time: 15}},
	}}
	for _, test := range []struct {
		elapsed    float64
		comparison bool
		want       float64
	}{{-2, true, 0}, {12, false, 10}, {12, true, 12}, {17, true, 15}} {
		if got := r.frameTime(test.elapsed, State{Comparison: test.comparison}); got != test.want {
			t.Fatalf("visible clock at %g, comparison %v = %g, want %g", test.elapsed, test.comparison, got, test.want)
		}
	}
}
