package solver

import (
	"errors"
	"math"
	"strings"
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

// This replacement envelope makes a tighter curvature infeasible at every
// speed, so refining an otherwise accepted coarse road can reject the baseline.
type curvatureGate struct {
	constantModel
	maxCurvature float64
}

func (m curvatureGate) Limits(v, k, bank, grade, grip float64) vehicle.Envelope {
	e := m.constantModel.Limits(v, k, bank, grade, grip)
	e.Feasible = math.Abs(k) <= m.maxCurvature
	return e
}

func TestRefinedBaselineFailureIsNotASeedFailure(t *testing.T) {
	scene, car := fixture(t, "hairpin")
	opts := DefaultOptions()
	coarse, _ := track.SampleRoad(scene, opts.Spacing)
	fine, _ := track.SampleRoad(scene, .5)
	maxCurvature := func(road []track.Sample) float64 {
		peak := 0.0
		for i := 1; i < len(road)-1; i++ {
			a, b, c := road[i-1].Position, road[i].Position, road[i+1].Position
			ab, bc, ac := math.Hypot(b.X-a.X, b.Y-a.Y), math.Hypot(c.X-b.X, c.Y-b.Y), math.Hypot(c.X-a.X, c.Y-a.Y)
			k := 2 * math.Abs((b.X-a.X)*(c.Y-b.Y)-(b.Y-a.Y)*(c.X-b.X)) / (ab * bc * ac)
			peak = math.Max(peak, k)
		}
		return peak
	}
	low, high := maxCurvature(coarse), maxCurvature(fine)
	if high <= low {
		t.Fatal("fixture does not distinguish refined curvature")
	}
	model := curvatureGate{constantModel{car, 2, 4}, (low + high) / 2}
	opts.Seed = make([]float64, len(coarse))
	for _, iterations := range []int{0, 1} {
		opts.Iterations = iterations
		_, err := Solve(scene, model, opts)
		var seedError *SeedError
		if err == nil || errors.As(err, &seedError) || !strings.Contains(err.Error(), "centreline infeasible") {
			t.Fatalf("refined baseline treated as rejected seed: %v", err)
		}
	}
}

func TestZeroBudgetSeedRejectionKeepsErrorType(t *testing.T) {
	scene, car := fixture(t, "hairpin")
	opts := DefaultOptions()
	opts.Iterations = 0
	opts.Seed = []float64{100}
	_, err := Solve(scene, car, opts)
	var seedError *SeedError
	if !errors.As(err, &seedError) {
		t.Fatalf("untyped rejected seed: %v", err)
	}
}
