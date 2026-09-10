package render

import (
	"bytes"
	"image"
	"math"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestComparisonUsesCommonStationAndSpeedAxes(t *testing.T) {
	s, result, config := comparisonFixture(t)
	beforeNodes := append([]solver.Node(nil), result.Nodes...)
	beforeCenter := append([]solver.Node(nil), result.CenterNodes...)
	beforeRoad := append([]track.Sample(nil), result.Road...)
	lastStation := result.Nodes[len(result.Nodes)-1].Station
	lastDistance := result.Nodes[len(result.Nodes)-1].S
	// Pick a real point for which travelled-distance alignment would visibly
	// disagree with reference-road alignment. This fixture must exercise that
	// distinction instead of letting an all-centreline result pass vacuously.
	candidate := result.Nodes[0]
	maxDifference := 0.0
	for _, n := range result.Nodes {
		difference := math.Abs(n.Station/lastStation - n.S/lastDistance)
		if difference > maxDifference {
			candidate, maxDifference = n, difference
		}
	}
	if maxDifference < .003 {
		t.Fatal("esses fixture does not distinguish station from path distance")
	}
	for _, size := range [][2]int{{1440, 900}, {1000, 700}, {800, 600}} {
		r, err := New(s, result, config, Options{View: "3d", Width: size[0], Height: size[1]})
		if err != nil {
			t.Fatal(err)
		}
		chart := r.Controls()["chart"]
		for _, fraction := range []float64{0, .25, .5, .75, 1} {
			station := r.ChartStation(float64(chart.Min.X) + fraction*float64(chart.Dx()))
			if math.Abs(station-fraction*lastStation) > 1e-10 {
				t.Fatalf("%v: chart coordinate %.2f selected %.6fm", size, fraction, station)
			}
		}
		if r.ChartStation(float64(chart.Min.X)-100) != 0 || r.ChartStation(float64(chart.Max.X)+100) != lastStation {
			t.Fatal("chart drags outside the strip must clamp to open endpoints")
		}
		center, err := result.CenterAtStation(candidate.Station)
		if err != nil {
			t.Fatal(err)
		}
		a, b := r.chartPoint(candidate), r.chartPoint(center)
		if math.Abs(a.x-b.x) > 1e-10 {
			t.Fatal("same road station plotted in different chart columns")
		}
		canonical := r.controls["chart"]
		wrongDistanceX := float64(canonical.Min.X) + candidate.S/lastDistance*float64(canonical.Dx())
		if math.Abs(a.x-wrongDistanceX) < 2 {
			t.Fatal("chart plotted travelled metres instead of shared road station")
		}
		// Equal physical speed must occupy the same row regardless of which
		// trajectory supplies the node. Zero is the visible common lower axis.
		center.Speed = candidate.Speed
		if r.chartPoint(center).y != a.y {
			t.Fatal("same speed plotted against inconsistent scales")
		}
		zero := candidate
		zero.Speed = 0
		if r.chartPoint(zero).y != float64(canonical.Max.Y) {
			t.Fatal("speed chart does not use zero as the shared baseline")
		}
		for _, nodes := range [][]solver.Node{result.Nodes, result.CenterNodes} {
			for _, n := range nodes {
				p := r.chartPoint(n)
				if p.y < float64(canonical.Min.Y) || p.y > float64(canonical.Max.Y) {
					t.Fatal("shared speed axis clips a verified trajectory")
				}
			}
		}
		if r.PlaybackDuration(false) != result.Duration || r.PlaybackDuration(true) != result.CenterDuration {
			t.Fatal("comparison playback did not retain the reference finish")
		}
	}
	if !reflect.DeepEqual(beforeNodes, result.Nodes) || !reflect.DeepEqual(beforeCenter, result.CenterNodes) || !reflect.DeepEqual(beforeRoad, result.Road) {
		t.Fatal("presentation modified numerical trajectories")
	}
}

func TestGhostFollowsReferenceAndStaysInsideRoadViewport(t *testing.T) {
	s, result, config := comparisonFixture(t)
	for _, view := range []string{"2d", "3d"} {
		r, err := New(s, result, config, Options{View: view})
		if err != nil {
			t.Fatal(err)
		}
		time := result.Duration * .5
		optimized, center := result.At(time), result.CenterAt(time)
		x, y := r.Project(center.Position)
		ox, oy := r.Project(optimized.Position)
		if math.Hypot(x-ox, y-oy) < 40 {
			t.Fatalf("%s: midpoint fixture does not visibly separate cars", view)
		}
		without := append([]byte(nil), r.FrameWithState(time, State{Selected: -1}).(*image.RGBA).Pix...)
		with := r.FrameWithState(time, State{Selected: -1, Comparison: true}).(*image.RGBA)
		changes, nearReference := 0, 0
		for yy := r.viewport.Min.Y; yy < r.viewport.Max.Y; yy++ {
			for xx := r.viewport.Min.X; xx < r.viewport.Max.X; xx++ {
				i := with.PixOffset(xx, yy)
				if bytes.Equal(without[i:i+4], with.Pix[i:i+4]) {
					continue
				}
				changes++
				if math.Hypot(float64(xx)-x, float64(yy)-y) <= 25 {
					nearReference++
				} else {
					t.Fatalf("%s: ghost drawn away from same-time reference at %d,%d", view, xx, yy)
				}
			}
		}
		if changes < 30 || nearReference < 30 {
			t.Fatalf("%s: ghost is not visibly drawn at the reference position", view)
		}
		// Zoom and pan put the reference entirely outside the road viewport.
		// The ghost must disappear there, even though its world point exists.
		beforePan := append([]byte(nil), with.Pix...)
		camera := r.Camera()
		camera.Zoom *= 4
		camera.PanX = 1600
		if err := r.SetCamera(camera); err != nil {
			t.Fatal(err)
		}
		canvas := r.FrameWithState(time, State{Selected: -1, Comparison: true}).(*image.RGBA)
		for yy := 0; yy < canvas.Bounds().Max.Y; yy++ {
			for xx := 0; xx < canvas.Bounds().Max.X; xx++ {
				if image.Pt(xx, yy).In(r.viewport) {
					continue
				}
				i := canvas.PixOffset(xx, yy)
				if !bytes.Equal(beforePan[i:i+4], canvas.Pix[i:i+4]) {
					t.Fatalf("%s: clipped ghost wrote into the interface at %d,%d", view, xx, yy)
				}
			}
		}
	}
}

func comparisonFixture(t *testing.T) (track.Scene, solver.Result, vehicle.Config) {
	t.Helper()
	s, err := track.Preset("esses")
	if err != nil {
		t.Fatal(err)
	}
	v, err := vehicle.Preset("road")
	if err != nil {
		t.Fatal(err)
	}
	result, err := solver.Solve(s, v, solver.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	return s, result, v
}
