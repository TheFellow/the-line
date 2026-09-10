package solver

import (
	"errors"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// A previous car's feasible optimized line is only a warm-start suggestion.
// A wider replacement car can invalidate it without invalidating the road or
// setup. Presentation must then request an unseeded solve, not reject the car.
func TestWiderVehicleCanSolveAfterPreviousLineBecomesInfeasible(t *testing.T) {
	scene, roadCar := fixture(t, "esses")
	gt, err := vehicle.Preset("gt")
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	current, err := Solve(scene, roadCar, opts)
	if err != nil {
		t.Fatal(err)
	}
	evalOpts := opts
	evalOpts.Spacing = current.Spacing
	if _, err := Evaluate(scene, gt, current.Offsets, evalOpts); err == nil {
		t.Fatal("fixture must expose the previous car's insufficient clearance")
	}
	coarse, err := track.SampleRoad(scene, opts.Spacing)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := current.OffsetsAt(coarse)
	if err != nil {
		t.Fatal(err)
	}
	seededOpts := opts
	seededOpts.Seed = seed
	if _, err := Solve(scene, gt, seededOpts); err == nil {
		t.Fatal("unsafe old-car seed unexpectedly accepted")
	} else {
		var seedError *SeedError
		if !errors.As(err, &seedError) {
			t.Fatalf("seed failure lost its type: %v", err)
		}
	}
	fresh, err := Solve(scene, gt, opts)
	if err != nil {
		t.Fatalf("wider car has a feasible fresh solution: %v", err)
	}
	if fresh.Duration > fresh.CenterDuration {
		t.Fatal("fresh search regressed from feasible centreline")
	}
	verifyResult(t, fresh, scene, gt, opts)
	t.Logf("road %.6f s cannot fit GT; unseeded GT %.6f s (centreline %.6f s)", current.Duration, fresh.Duration, fresh.CenterDuration)
}

func TestInvalidRoadIsNotASeedFailure(t *testing.T) {
	scene, car := fixture(t, "hairpin")
	for i := range scene.Points {
		scene.Points[i].Width = 3
	}
	car.Width = 3
	opts := DefaultOptions()
	opts.Seed = []float64{100}
	_, err := Solve(scene, car, opts)
	var seedError *SeedError
	if err == nil || errors.As(err, &seedError) {
		t.Fatalf("invalid road would trigger pointless retry: %v", err)
	}
}
