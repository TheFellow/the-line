package render

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestPinnedReferenceSetupDeltaPersistenceAndStaleSafety(t *testing.T) {
	scene, _ := track.Preset("hairpin")
	car, _ := vehicle.Preset("road")
	opts := solver.DefaultOptions()
	opts.Iterations = 1
	original, err := solver.Solve(scene, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	pin := PinReference("Before setup", scene, original, car)
	stored := StoreReference(pin.Name, scene, original, car)
	restored, err := RestoreReference(stored)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Name != pin.Name || restored.Trajectory.Duration != pin.Trajectory.Duration {
		t.Fatalf("reference changed on reload: %g != %g", restored.Trajectory.Duration, pin.Trajectory.Duration)
	}
	car.Grip *= 1.1
	opts.Spacing = original.Spacing
	changed, err := solver.Evaluate(scene, car, original.Offsets, opts)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(scene, changed, car, Options{Reference: &pin})
	if err != nil {
		t.Fatal(err)
	}
	end := changed.Nodes[len(changed.Nodes)-1]
	referenceEnd, err := r.referenceAtStation(end.Station)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs((end.Time-referenceEnd.Time)-(changed.Duration-original.Duration)) > 1e-12 {
		t.Fatal("same-station exit delta differs from total duration delta")
	}
	original.Nodes[0].Speed = 123
	if pin.Trajectory.Nodes[0].Speed == 123 {
		t.Fatal("pin aliases source nodes")
	}
	scene.Points[1].Bank++
	if pin.Compatible(scene) {
		t.Fatal("road edit left pin comparable")
	}
	r.scene = scene
	if _, err := r.referenceAtStation(end.Station); err == nil {
		t.Fatal("stale station comparison succeeded")
	}
	if len(r.referenceNodes()) != 0 || r.PlaybackDuration(true) != changed.Duration {
		t.Fatal("stale ghost remained active")
	}
}
