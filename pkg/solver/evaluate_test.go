package solver

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestEvaluateReproducesFinalLine(t *testing.T) {
	scene, _ := track.Preset("esses")
	car, _ := vehicle.Preset("road")
	opts := DefaultOptions()
	opts.Iterations = 1
	line, err := Solve(scene, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Spacing = line.Spacing
	start := time.Now()
	evaluated, err := Evaluate(scene, car, line.Offsets, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("same-line evaluation %s", time.Since(start))
	if evaluated.Duration != line.Duration {
		t.Fatalf("reevaluated %.17g != %.17g", evaluated.Duration, line.Duration)
	}
	for i := range line.Nodes {
		if evaluated.Nodes[i] != line.Nodes[i] {
			t.Fatalf("node %d changed", i)
		}
	}
	evaluated.Offsets[0] = 99
	if line.Offsets[0] == 99 {
		t.Fatal("aliased offsets")
	}
}

func TestSeedEvaluationAndBaseline(t *testing.T) {
	scene := straight()
	car, _ := vehicle.Preset("road")
	for _, spacing := range []float64{3, .5} {
		opts := Options{Spacing: spacing, Margin: .25}
		road, err := track.SampleRoad(scene, spacing)
		if err != nil {
			t.Fatal(err)
		}
		seed := make([]float64, len(road))
		for i, p := range road {
			seed[i] = 2 * math.Sin(p.S/35)
		}
		opts.Seed = seed
		want, err := Evaluate(scene, car, seed, opts)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Solve(scene, car, opts)
		if err != nil {
			t.Fatal(err)
		}
		if got.Duration != want.Duration {
			t.Fatal("zero-budget seed changed")
		}
		opts.Iterations = 1
		got, err = Solve(scene, car, opts)
		if err != nil {
			t.Fatal(err)
		}
		if got.Duration > want.Duration || got.Duration > got.CenterDuration {
			t.Fatalf("seed or baseline regression: %+v", got)
		}
	}
}

func TestEvaluateRejectsOffsetsAndCancellation(t *testing.T) {
	s := straight()
	car, _ := vehicle.Preset("road")
	opts := DefaultOptions()
	for _, offset := range [][]float64{{1}, {math.NaN()}} {
		if _, err := Evaluate(s, car, offset, opts); err == nil {
			t.Fatal("bad offsets accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SolveContext(ctx, s, car, opts); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := EvaluateContext(ctx, s, car, nil, opts); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	opts.Seed = []float64{0}
	opts.Iterations = -1
	if _, err := Solve(s, car, opts); err == nil {
		t.Fatal("negative iterations accepted")
	}
}

func TestSensitivitiesUseSameLine(t *testing.T) {
	scene := straight()
	car, _ := vehicle.Preset("road")
	opts := DefaultOptions()
	opts.Iterations = 0
	line, err := Solve(scene, car, opts)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Sensitivities(context.Background(), scene, car, line, opts.Margin)
	if err != nil {
		t.Fatal(err)
	}
	opts.Spacing = line.Spacing
	for _, row := range rows {
		value, _ := car.Value(row.Parameter)
		changed, _ := car.With(row.Parameter, value+row.Change)
		want, err := Evaluate(scene, changed, line.Offsets, opts)
		if err != nil {
			t.Fatal(err)
		}
		if row.Duration != want.Duration || row.Delta != want.Duration-line.Duration {
			t.Fatalf("sensitivity %+v differs from direct evaluation", row)
		}
	}
}

type cancellingModel struct {
	vehicle.Config
	cancel context.CancelFunc
	calls  int
}

func (m *cancellingModel) Limits(speed, curvature, bank, grade, grip float64) vehicle.Envelope {
	m.calls++
	if m.calls == 100 {
		m.cancel()
	}
	return m.Config.Limits(speed, curvature, bank, grade, grip)
}
func TestCancellationDuringProfile(t *testing.T) {
	car, _ := vehicle.Preset("road")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := &cancellingModel{Config: car, cancel: cancel}
	_, err := SolveContext(ctx, straight(), model, DefaultOptions())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
	if model.calls > 5000 {
		t.Fatalf("too much work after cancellation: %d calls", model.calls)
	}
}
