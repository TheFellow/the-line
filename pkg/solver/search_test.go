package solver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

type unpreparedModel struct{ vehicle.Config }

func (m unpreparedModel) Limits(v, k, b, g, mu float64) vehicle.Envelope {
	return m.Config.Limits(v, k, b, g, mu)
}

func TestSearchWorkersPreserveFixtureTrajectories(t *testing.T) {
	for _, name := range []string{"hairpin", "esses", "compound", "banked", "rally"} {
		t.Run(name, func(t *testing.T) {
			scene, car := fixture(t, name)
			opts := DefaultOptions()
			// One full-budget fixture exercises every final-search round. The
			// other geometries need both search scales, not repeated long solves.
			// Default-budget quality on every preset is checked separately.
			if name != "esses" {
				opts.Iterations = 1
			}
			opts.Workers = 1
			sequential, err := Solve(scene, car, opts)
			if err != nil {
				t.Fatal(err)
			}
			opts.Workers = 3
			concurrent, err := Solve(scene, car, opts)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(sequential.Nodes, concurrent.Nodes) || !reflect.DeepEqual(sequential.CenterNodes, concurrent.CenterNodes) || !reflect.DeepEqual(sequential.Offsets, concurrent.Offsets) || sequential.Duration != concurrent.Duration || sequential.Candidates != concurrent.Candidates {
				t.Fatal("concurrent evaluation changed the sequential fixture")
			}
			// Verify the general envelope on the complete final trajectory. Worker
			// determinism above uses the built-in model in both runs, so a much
			// slower custom envelope need not repeat hundreds of search probes.
			general, err := Evaluate(scene, unpreparedModel{car}, sequential.Offsets, Options{Spacing: sequential.Spacing, Margin: opts.Margin})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(general.Nodes, sequential.Nodes) || !reflect.DeepEqual(general.CenterNodes, sequential.CenterNodes) {
				t.Fatal("prepared evaluation changed the general-envelope trajectory")
			}
			t.Logf("%s unchanged %.12f s, %d finalists", name, concurrent.Duration, concurrent.FineCandidates)
		})
	}
}
func TestPolishPreservesVerifiedUnpolishedFallback(t *testing.T) {
	scene, car := fixture(t, "esses")
	opts := DefaultOptions()
	opts.Iterations = 1
	original, err := Solve(scene, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Polish = 1
	polished, err := Solve(scene, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	if polished.Duration > original.Duration || polished.PolishCandidates != 34 {
		t.Fatalf("polish regressed %g to %g (%d candidates)", original.Duration, polished.Duration, polished.PolishCandidates)
	}
	t.Logf("one polish sweep %.9f to %.9f s", original.Duration, polished.Duration)
}
func TestSearchCancellationAndOptions(t *testing.T) {
	scene, car := fixture(t, "esses")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SolveContext(ctx, scene, car, DefaultOptions()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, opts := range []Options{{Spacing: 3, Workers: 33}, {Spacing: 3, Polish: 4}} {
		if _, err := Solve(scene, car, opts); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
	if searchWorkers(unpreparedModel{car}, 8, 16) != 1 {
		t.Fatal("custom model concurrently evaluated without a thread safety contract")
	}
}

func TestIndependentFinalistsWorkerDeterminism(t *testing.T) {
	scene, car := fixture(t, "hairpin")
	road, err := track.SampleRoad(scene, 3)
	if err != nil {
		t.Fatal(err)
	}
	eval := evaluator{ctx: context.Background(), road: road, model: car, entry: scene.EntrySpeed, exit: scene.ExitSpeed, clearance: car.Width/2 + .25}
	offsets := make([][]float64, 6)
	for i := range offsets {
		offsets[i] = make([]float64, len(road))
	}
	sequential := evaluateCandidates(context.Background(), eval, offsets, 1)
	concurrent := evaluateCandidates(context.Background(), eval, offsets, 3)
	for i := range sequential {
		if sequential[i].err != nil || concurrent[i].err != nil || !reflect.DeepEqual(sequential[i].result.Nodes, concurrent[i].result.Nodes) {
			t.Fatalf("candidate %d differs", i)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eval.ctx = ctx
	if got := evaluateCandidates(ctx, eval, offsets, 3); len(got) != len(offsets) {
		t.Fatal("cancelled result shape")
	}
}
