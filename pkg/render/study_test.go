package render

import (
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"math"
	"testing"
)

func TestSectorSplitsAnalyticalSpeeds(t *testing.T) {
	scene := track.Scene{Points: []track.Point{{X: 0}, {X: 20}, {X: 50}}}
	road := []track.Sample{{Position: track.Vec3{X: 0}, S: 0}, {Position: track.Vec3{X: 20}, S: 20}, {Position: track.Vec3{X: 50}, S: 50}}
	trajectory := func(speed float64) solver.Result {
		r := solver.Result{}
		for _, s := range road {
			r.Nodes = append(r.Nodes, solver.Node{Position: s.Position, Station: s.S, S: s.S, Time: s.S / speed, Speed: speed})
		}
		r.Duration = 50 / speed
		return r
	}
	got, err := SectorSplits(scene, road, trajectory(10), trajectory(5))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{-2, -3} {
		if math.Abs(got[i].Delta-want) > 1e-12 {
			t.Fatalf("sector %d delta %g want %g", i, got[i].Delta, want)
		}
	}
	if got[0].Current+got[1].Current != 5 || got[0].Reference+got[1].Reference != 10 {
		t.Fatal("splits do not sum to complete duration")
	}
	scene.Points[1].X = 21
	if _, err = SectorSplits(scene, road, trajectory(10), trajectory(5)); err == nil {
		t.Fatal("different geometry silently accepted")
	}
}
func TestSectorSplitsClosedSeam(t *testing.T) {
	scene := track.Scene{Closed: true, Points: []track.Point{{X: 0}, {X: 10}, {X: 10, Y: 10}}}
	road := []track.Sample{{Position: track.Vec3{}, S: 0}, {Position: track.Vec3{X: 10}, S: 10}, {Position: track.Vec3{X: 10, Y: 10}, S: 20}, {Position: track.Vec3{}, S: 40}}
	a, b := solver.Result{}, solver.Result{}
	for _, p := range road {
		a.Nodes = append(a.Nodes, solver.Node{S: p.S, Station: p.S, Time: p.S / 10, Speed: 10})
		b.Nodes = append(b.Nodes, solver.Node{S: p.S, Station: p.S, Time: p.S / 5, Speed: 5})
	}
	got, err := SectorSplits(scene, road, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2].End != 40 || got[2].Delta != -2 {
		t.Fatalf("closing sector: %+v", got)
	}
}
