package racecraft

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
)

func TestCompletedPassHysteresis(t *testing.T) {
	// A travels at 10 m/s. B brakes from 20 to 0 over two seconds,
	// then accelerates back to 20. Its lead is 10t-5t² in the first
	// phase; A has the same lead two seconds later. Both maxima are 5 m.
	r := Result{Duration: 4, Cars: [2]Car{
		{Path: solver.Result{Duration: 4, Nodes: []solver.Node{{Speed: 10}, {S: 40, Station: 40, Speed: 10, Time: 4}}}},
		{Path: solver.Result{Duration: 4, Nodes: []solver.Node{
			{Speed: 20, Acceleration: -10},
			{S: 20, Station: 20, Time: 2, Speed: 0, Acceleration: 10},
			{S: 40, Station: 40, Time: 4, Speed: 20},
		}}},
	}}
	got := events(r)
	if len(got) != 2 || got[0].Leader != 1 || got[0].Kind != "pass" || got[1].Leader != 0 || got[1].Kind != "repass" {
		t.Fatalf("completed order changes: %+v", got)
	}
	root := 1 - math.Sqrt(.12) // Analytic crossing of the 4.4 m threshold.
	for i, event := range got {
		want := root + 2*float64(i)
		if event.Time < want || event.Time-want > .020000001 {
			t.Errorf("event at %g, analytic threshold at %g", event.Time, want)
		}
	}
	// The nominal initial leader is A even for a side-by-side start.
	// Nose-ahead excursions below a body length do not count as passes.
	r.Duration = .4 // B leads by 3.2 m.
	if got := events(r); len(got) != 0 {
		t.Fatalf("counted an incomplete pass: %+v", got)
	}
	// The final instant is checked even when it falls between sample ticks.
	r.Duration = root + .001
	if got := events(r); len(got) != 1 || math.Abs(got[0].Time-r.Duration) > 1e-10 {
		t.Fatalf("missed final-instant pass: %+v", got)
	}
}
