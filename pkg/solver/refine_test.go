package solver

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Challenge the finished line with edits outside the optimizer's direction
// family: physical cosine displacements and the actual authoring cubic, rather
// than its compact polynomial changes in bounded latent coordinates.
func TestClubLoopResistsIndependentEdits(t *testing.T) {
	for _, name := range []string{"road", "gt"} {
		t.Run(name, func(t *testing.T) {
			scene, _ := track.Preset("club-loop")
			car, _ := vehicle.Preset(name)
			opts := DefaultOptions()
			result, err := Solve(scene, car, opts)
			if err != nil {
				t.Fatal(err)
			}
			gain := result.BeforeRefineDuration - result.Duration
			if gain < .05 || result.RefineCandidates == 0 {
				t.Fatalf("final-resolution search did not improve the identical-condition baseline: %g s", gain)
			}
			opts.Spacing = result.Spacing
			again, err := Evaluate(scene, car, result.Offsets, opts)
			if err != nil || again.Duration != result.Duration {
				t.Fatalf("same-line timing changed: %v", err)
			}
			var edits [][]float64
			length := result.Road[len(result.Road)-1].S
			for j := 0; j < 32; j++ {
				for _, amplitude := range []float64{-.2, .2} {
					offsets := append([]float64(nil), result.Offsets...)
					center, radius := float64(j)*length/32, length/32*1.35
					for i, p := range result.Road {
						d := math.Abs(p.S - center)
						d = math.Min(d, length-d)
						if d < radius {
							offsets[i] += amplitude * (1 + math.Cos(math.Pi*d/radius)) / 2
						}
					}
					offsets[len(offsets)-1] = offsets[0]
					edits = append(edits, offsets)
				}
			}
			controls := make([]LineControl, min(25, 2*len(scene.Points)-1))
			for i := range controls {
				index := i * (len(result.Road) - 1) / (len(controls) - 1)
				controls[i] = LineControl{Index: index, Offset: result.Offsets[index]}
			}
			for i := 0; i < len(controls)-1; i++ {
				for _, delta := range []float64{-.5, .5} {
					for _, paired := range []bool{false, true} {
						trial := append([]LineControl(nil), controls...)
						trial[i].Offset += delta
						if paired {
							trial[(i+1)%(len(trial)-1)].Offset -= delta
						}
						trial[len(trial)-1].Offset = trial[0].Offset
						offsets, err := ManualOffsets(result.Road, trial, car.Width/2+opts.Margin)
						if err == nil {
							edits = append(edits, offsets)
						}
					}
				}
			}
			// These public-API evaluations are independent. Keep the challenging
			// edit family broad without serially repeating its baseline profiles.
			times := make([]float64, len(edits))
			jobs := make(chan int)
			var workers sync.WaitGroup
			for range 4 {
				workers.Add(1)
				go func() {
					defer workers.Done()
					for i := range jobs {
						r, err := Evaluate(scene, car, edits[i], opts)
						if err == nil {
							times[i] = r.Duration
						}
					}
				}()
			}
			for i := range edits {
				jobs <- i
			}
			close(jobs)
			workers.Wait()
			bestEdit, feasible := result.Duration, 0
			for _, duration := range times {
				if duration > 0 {
					feasible++
					bestEdit = math.Min(bestEdit, duration)
				}
			}
			if feasible < 32 {
				t.Fatalf("only %d feasible independent edits", feasible)
			}
			t.Logf("before %.9f s, after %.9f s; %d feasible independent edits, maximum gain %.9f s", result.BeforeRefineDuration, result.Duration, feasible, result.Duration-bestEdit)
			if result.Duration-bestEdit > .01 {
				t.Fatalf("a small independent edit still gains %.6f s", result.Duration-bestEdit)
			}
			verifyResult(t, result, scene, car, opts)
		})
	}
}

func TestChicaneFineSearchImprovesAndRefines(t *testing.T) {
	for _, name := range []string{"chicane", "closed-chicane"} {
		t.Run(name, func(t *testing.T) {
			preset := name
			if name == "closed-chicane" {
				preset = "club-loop"
			}
			scene, car := fixture(t, preset)
			if name == "closed-chicane" {
				// Pull the middle of the lower straight inward by 50 m to make
				// a substantial change of direction within a periodic lap.
				scene.Points[1].Y += 50
			}
			opts := DefaultOptions()
			r, err := Solve(scene, car, opts)
			if err != nil {
				t.Fatal(err)
			}
			if r.Duration >= r.BeforeRefineDuration-.02 {
				t.Fatalf("chicane fine search gain too small: %.9f to %.9f", r.BeforeRefineDuration, r.Duration)
			}
			t.Logf("%s %.9f to %.9f s", name, r.BeforeRefineDuration, r.Duration)
			verifyResult(t, r, scene, car, opts)
		})
	}
}

func TestFineSearchCancellationPreservesInput(t *testing.T) {
	scene, car := fixture(t, "esses")
	initial, err := Evaluate(scene, car, nil, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	offsets := append([]float64(nil), initial.Offsets...)
	nodes := append([]Node(nil), initial.Nodes...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := &cancellingModel{Config: car, cancel: cancel}
	clearance := car.Width/2 + DefaultOptions().Margin
	eval := evaluator{ctx: ctx, road: initial.Road, model: model, entry: scene.EntrySpeed, exit: scene.ExitSpeed, clearance: clearance, edges: indexRoadEdges(initial.Road, clearance)}
	_, _, err = refineLine(eval, initial, 2, 4)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if !reflect.DeepEqual(offsets, initial.Offsets) || !reflect.DeepEqual(nodes, initial.Nodes) {
		t.Fatal("fine search mutated its input")
	}
}
