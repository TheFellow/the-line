package editor

import (
	"context"
	"errors"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestSolveSetupRetriesOnlyRejectedSeed(t *testing.T) {
	scene, _ := track.Preset("esses")
	car, _ := vehicle.Preset("road")
	opts := solver.DefaultOptions()
	opts.Iterations = 0
	road, err := track.SampleRoad(scene, opts.Spacing)
	if err != nil {
		t.Fatal(err)
	}
	opts.Seed = make([]float64, len(road))
	for i := range opts.Seed {
		opts.Seed[i] = 100 // Deliberately outside this road, with a valid shape.
	}
	retries := 0
	result, err := SolveSetup(context.Background(), scene, car, opts, func() { retries++ })
	if err != nil {
		t.Fatal(err)
	}
	if retries != 1 || result.Duration <= 0 || result.Duration != result.CenterDuration || result.MaxForceResidual > .0005 {
		t.Fatalf("fresh centreline was not verified: retries=%d duration=%g centre=%g residual=%g", retries, result.Duration, result.CenterDuration, result.MaxForceResidual)
	}
	if opts.Seed[0] != 100 {
		t.Fatal("retry mutated the caller's seed")
	}
	// Clearing the seed is insufficient to fix this car: both validation and
	// baseline failures must reach the caller without repeating the search.
	for _, failure := range []struct {
		name  string
		width float64
	}{{"invalid car", 100}, {"infeasible road", 4}} {
		t.Run(failure.name, func(t *testing.T) {
			badCar := car
			badCar.Width = failure.width
			narrow := scene
			narrow.Points = append([]track.Point(nil), scene.Points...)
			for i := range narrow.Points {
				narrow.Points[i].Width = 3
			}
			retries = 0
			if _, err := SolveSetup(context.Background(), narrow, badCar, opts, func() { retries++ }); err == nil || retries != 0 {
				t.Fatalf("non-seed failure retried: err=%v retries=%d", err, retries)
			}
		})
	}
}

func TestSolveSetupCancellationAndOptionalNotification(t *testing.T) {
	scene, _ := track.Preset("banked")
	car, _ := vehicle.Preset("road")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := SolveSetup(ctx, scene, car, solver.DefaultOptions(), func() { t.Fatal("cancelled solve retried") })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	opts := solver.DefaultOptions()
	opts.Iterations = 0
	opts.Seed = []float64{100} // Invalid shape also identifies a rejected seed.
	if _, err := SolveSetup(context.Background(), scene, car, opts, nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if _, err := SolveSetup(ctx, scene, car, opts, cancel); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation before retry lost: %v", err)
	}
}
