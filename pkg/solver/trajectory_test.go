package solver

import (
	"math"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestStationKinematics(t *testing.T) {
	// The reference road covers 20 m while this sloped trajectory covers 10 m.
	// Independent motion laws use actual distance d, never reference metres.
	for _, motion := range []struct {
		name         string
		entry, exit  float64
		acceleration float64
		timeAt       func(float64) float64
		speedAt      func(float64) float64
	}{
		{"launch", 0, math.Sqrt(40), 2, func(d float64) float64 { return math.Sqrt(d) }, func(d float64) float64 { return 2 * math.Sqrt(d) }},
		{"stop", math.Sqrt(40), 0, -2, func(d float64) float64 { return math.Sqrt(10) - math.Sqrt(10-d) }, func(d float64) float64 { return 2 * math.Sqrt(10-d) }},
		{"cruise", 5, 5, 0, func(d float64) float64 { return d / 5 }, func(float64) float64 { return 5 }},
	} {
		t.Run(motion.name, func(t *testing.T) {
			nodes := []Node{
				{Position: track.Vec3{}, S: 7, Station: 100, Time: 3, Speed: motion.entry, Acceleration: motion.acceleration},
				{Position: track.Vec3{X: 6, Z: 8}, S: 17, Station: 120, Time: 3 + motion.timeAt(10), Speed: motion.exit},
			}
			r := Result{Nodes: nodes, CenterNodes: append([]Node(nil), nodes...)}
			for _, d := range []float64{0, 1e-8, .1, 1, 5, 9, 9.99999999, 10} {
				station := 100 + 2*d
				for _, query := range []func(float64) (Node, error){r.AtStation, r.CenterAtStation} {
					n, err := query(station)
					if err != nil {
						t.Fatal(err)
					}
					for label, pair := range map[string][2]float64{
						"time": {n.Time, 3 + motion.timeAt(d)}, "speed": {n.Speed, motion.speedAt(d)},
						"distance": {n.S, 7 + d}, "station": {n.Station, station},
						"x": {n.Position.X, .6 * d}, "height": {n.Position.Z, .8 * d},
					} {
						if math.Abs(pair[0]-pair[1]) > 1e-8 {
							t.Fatalf("distance %.12g %s: got %.12g want %.12g", d, label, pair[0], pair[1])
						}
					}
					for _, at := range []func(float64) Node{r.At, r.CenterAt} {
						back := at(n.Time)
						if math.Abs(back.Station-station) > 1e-8 || math.Abs(back.Speed-n.Speed) > 1e-8 {
							t.Fatalf("time query disagrees at station %g: %+v vs %+v", station, back, n)
						}
					}
				}
			}
		})
	}
}

func TestTrajectoryQueryBounds(t *testing.T) {
	nodes := []Node{{Station: 4, Time: 0, Speed: 2}, {Station: 14, S: 10, Time: 5, Speed: 2}}
	r := Result{Nodes: nodes, CenterNodes: nodes}
	for _, query := range []func(float64) (Node, error){r.AtStation, r.CenterAtStation} {
		for _, invalid := range []float64{math.NaN(), math.Inf(-1), math.Inf(1)} {
			if _, err := query(invalid); err == nil {
				t.Fatal("nonfinite station accepted")
			}
		}
		for _, tc := range []struct {
			station float64
			want    Node
		}{{-1, nodes[0]}, {4, nodes[0]}, {14, nodes[1]}, {50, nodes[1]}} {
			got, err := query(tc.station)
			if err != nil || got != tc.want {
				t.Fatalf("station %g: %+v %v", tc.station, got, err)
			}
		}
	}
	for _, at := range []func(float64) Node{r.At, r.CenterAt} {
		if at(math.NaN()) != nodes[0] || at(math.Inf(-1)) != nodes[0] || at(math.Inf(1)) != nodes[1] {
			t.Fatal("time endpoint policy differs")
		}
	}
	empty := Result{}
	if _, err := empty.AtStation(0); err == nil {
		t.Fatal("empty optimized trajectory accepted")
	}
	if _, err := empty.CenterAtStation(0); err == nil {
		t.Fatal("empty reference trajectory accepted")
	}
	if empty.At(0) != (Node{}) || empty.CenterAt(0) != (Node{}) {
		t.Fatal("empty time query differs")
	}
}

func TestRetainedBaseline(t *testing.T) {
	scene, car := fixture(t, "banked")
	for _, spacing := range []float64{3, .5} {
		for _, iterations := range []int{0, 1} {
			r, err := Solve(scene, car, Options{Spacing: spacing, Iterations: iterations, Margin: .25})
			if err != nil {
				t.Fatal(err)
			}
			if len(r.CenterNodes) < len(r.Road) {
				t.Fatal("missing reference anchors")
			}
			first, last := r.CenterNodes[0], r.CenterNodes[len(r.CenterNodes)-1]
			if first.Speed != r.CenterEntrySpeed || last.Speed != r.CenterExitSpeed || last.Time != r.CenterDuration {
				t.Fatal("baseline endpoint diagnostics disagree")
			}
			for _, nodes := range [][]Node{r.Nodes, r.CenterNodes} {
				if nodes[0].Station != r.Road[0].S || nodes[len(nodes)-1].Station != r.Road[len(r.Road)-1].S {
					t.Fatal("reference bounds differ")
				}
				if len(nodes) <= len(r.Road) {
					t.Fatal("fixture did not exercise diagonal crossings")
				}
				for i := 1; i < len(nodes); i++ {
					if nodes[i].Station <= nodes[i-1].Station {
						t.Fatal("nonmonotonic reference station")
					}
				}
			}
			if iterations == 0 && !reflect.DeepEqual(r.Nodes, r.CenterNodes) {
				t.Fatal("disabled search differs from baseline")
			}
			for i := 0; i <= 100; i++ {
				station := last.Station * float64(i) / 100
				n, err := r.AtStation(station)
				if err != nil {
					t.Fatal(err)
				}
				c, err := r.CenterAtStation(station)
				if err != nil {
					t.Fatal(err)
				}
				if iterations == 0 && math.Abs(n.Time-c.Time) > 1e-10 {
					t.Fatal("disabled search has nonzero delta")
				}
				if i == 100 && math.Abs((n.Time-c.Time)-(r.Duration-r.CenterDuration)) > 1e-10 {
					t.Fatal("terminal delta differs")
				}
			}
			// The baseline remains independently owned even when selected unchanged.
			r.Nodes[0].Speed = -1
			if r.CenterNodes[0].Speed < 0 {
				t.Fatal("selected trajectory aliases baseline")
			}
		}
	}
}
