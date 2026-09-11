package solver

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestConcurrentSolveEvaluateSharedInputs(t *testing.T) {
	scene, car := fixture(t, "hairpin")
	road, err := track.SampleRoad(scene, 3)
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.Iterations = 1
	opts.Workers = 1
	opts.Seed = make([]float64, len(road))
	originalScene := scene
	originalScene.Points = append([]track.Point(nil), scene.Points...)
	originalCar := car
	originalSeed := append([]float64(nil), opts.Seed...)
	wantSolve, err := Solve(scene, &car, opts)
	if err != nil {
		t.Fatal(err)
	}
	evalOpts := opts
	evalOpts.Spacing = wantSolve.Spacing
	offsets := append([]float64(nil), wantSolve.Offsets...)
	wantEval, err := Evaluate(scene, &car, offsets, evalOpts)
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		kind   string
		result Result
		err    error
	}
	results := make(chan outcome, 6)
	start := make(chan struct{})
	for _, workers := range []int{1, 2, 4} {
		go func(workers int) {
			<-start
			options := opts
			options.Workers = workers
			result, err := Solve(scene, &car, options)
			results <- outcome{"solve", result, err}
		}(workers)
		go func() {
			<-start
			result, err := Evaluate(scene, &car, offsets, evalOpts)
			results <- outcome{"evaluate", result, err}
		}()
	}
	close(start)
	for range 6 {
		got := <-results
		if got.err != nil {
			t.Errorf("concurrent %s: %v", got.kind, got.err)
			continue
		}
		want := wantSolve
		if got.kind == "evaluate" {
			want = wantEval
		}
		if got.result.Duration != want.Duration || got.result.CenterDuration != want.CenterDuration || !reflect.DeepEqual(got.result.Nodes, want.Nodes) || !reflect.DeepEqual(got.result.CenterNodes, want.CenterNodes) || !reflect.DeepEqual(got.result.Offsets, want.Offsets) {
			t.Errorf("concurrent %s differs from independent serial result", got.kind)
		}
	}
	if !reflect.DeepEqual(scene, originalScene) || car != originalCar || !reflect.DeepEqual(opts.Seed, originalSeed) || !reflect.DeepEqual(offsets, wantSolve.Offsets) {
		t.Fatal("shared input changed during solve/evaluation")
	}
}

// Pausing the first context check inside each evaluator lets the test cancel
// after workers actually start, instead of racing a timer against scheduling.
// The dispatcher receives the ordinary context and cannot consume these gates.
type gatedEvaluationContext struct {
	context.Context
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (c *gatedEvaluationContext) Err() error {
	if c.calls.Add(1) <= 4 {
		c.started <- struct{}{}
		<-c.release
	}
	return c.Context.Err()
}

func TestParallelFinalistsCancelAfterWorkersStart(t *testing.T) {
	eval, offsets := finalistFixture(t)
	if searchWorkers(eval.model, 4, len(offsets)) != 4 {
		t.Skip("four-worker pool is unavailable on this platform")
	}
	ctx, cancel := context.WithCancel(context.Background())
	gate := &gatedEvaluationContext{Context: ctx, started: make(chan struct{}, 4), release: make(chan struct{})}
	var release sync.Once
	cleanup := func() { cancel(); release.Do(func() { close(gate.release) }) }
	defer cleanup()
	eval.ctx = gate
	done := make(chan []candidateEvaluation, 1)
	go func() { done <- evaluateCandidates(ctx, eval, offsets, 4) }()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for range 4 {
		select {
		case <-gate.started:
		case <-timeout.C:
			t.Fatal("four workers did not start")
		}
	}
	cleanup()
	select {
	case results := <-done:
		if len(results) != len(offsets) {
			t.Fatal("cancellation changed indexed result shape")
		}
		cancelled := 0
		for i, result := range results {
			if errors.Is(result.err, context.Canceled) {
				cancelled++
				continue
			}
			if result.err != nil || len(result.result.Nodes) > 0 {
				t.Errorf("candidate %d completed after gated cancellation: %v", i, result.err)
			}
		}
		if cancelled != 4 {
			t.Fatalf("cancelled %d active workers, want 4", cancelled)
		}
	case <-timeout.C:
		t.Fatal("cancelled workers were not joined")
	}
}

func TestDistinctFinalistBatchMatchesSerial(t *testing.T) {
	eval, offsets := finalistFixture(t)
	serial := evaluateCandidates(context.Background(), eval, offsets, 1)
	parallel := evaluateCandidates(context.Background(), eval, offsets, 4)
	streamed := make([]candidateEvaluation, len(offsets))
	next := 0
	visitCandidates(eval, offsets, 4, func(index int, result candidateEvaluation) {
		if index != next {
			t.Errorf("streamed result %d, expected %d", index, next)
		}
		streamed[index] = result
		next++
	})
	if next != len(offsets) {
		t.Fatal("stream dropped candidates")
	}
	durations := make(map[float64]bool)
	for i, want := range serial {
		got := parallel[i]
		if want.err != nil || got.err != nil {
			t.Fatalf("candidate %d: serial %v, parallel %v", i, want.err, got.err)
		}
		if want.result.Duration != got.result.Duration || !reflect.DeepEqual(want.result.Nodes, got.result.Nodes) {
			t.Fatalf("candidate %d changed under parallel evaluation", i)
		}
		if streamed[i].err != nil || !reflect.DeepEqual(want.result.Nodes, streamed[i].result.Nodes) {
			t.Fatalf("streamed candidate %d changed", i)
		}
		durations[want.result.Duration] = true
	}
	if len(durations) != len(offsets) {
		t.Fatalf("fixture only exercises %d distinct durations for %d candidates", len(durations), len(offsets))
	}
}

func TestWindowedPollCancellationJoinsWorkers(t *testing.T) {
	eval, offsets := finalistFixture(t)
	if searchWorkers(eval.model, 4, len(offsets)) != 4 {
		t.Skip("four workers unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	gate := &gatedEvaluationContext{Context: ctx, started: make(chan struct{}, 4), release: make(chan struct{})}
	var release sync.Once
	cleanup := func() { cancel(); release.Do(func() { close(gate.release) }) }
	defer cleanup()
	eval.ctx = gate
	done := make(chan struct{})
	go func() {
		visitCandidates(eval, offsets, 4, func(int, candidateEvaluation) { t.Error("gated cancelled candidate was published") })
		close(done)
	}()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for range 4 {
		select {
		case <-gate.started:
		case <-timeout.C:
			t.Fatal("poll workers did not start")
		}
	}
	cleanup()
	select {
	case <-done:
	case <-timeout.C:
		t.Fatal("poll workers did not stop after cancellation")
	}
}
