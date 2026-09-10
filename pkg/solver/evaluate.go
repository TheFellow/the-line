package solver

import (
	"context"
	"fmt"
	"math"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func (e evaluator) cancelled() error {
	cooperate()
	if e.ctx != nil {
		return e.ctx.Err()
	}
	return nil
}

// Evaluate computes a verified speed profile on supplied offsets without a
// search. Offsets correspond to SampleRoad(scene, opts.Spacing); nil selects
// the centreline. Coarser inputs are interpolated onto the final ≤0.5 m road.
func Evaluate(scene track.Scene, model vehicle.Model, offsets []float64, opts Options) (Result, error) {
	return EvaluateContext(context.Background(), scene, model, offsets, opts)
}

// EvaluateContext is Evaluate with cancellation. Iterations and Seed are ignored.
func EvaluateContext(ctx context.Context, scene track.Scene, model vehicle.Model, offsets []float64, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if model == nil {
		return Result{}, fmt.Errorf("vehicle model is required")
	}
	if err := model.Parameters().Validate(); err != nil {
		return Result{}, err
	}
	if !finite(opts.Spacing) || opts.Spacing <= 0 || !finite(opts.Margin) || opts.Margin < 0 {
		return Result{}, fmt.Errorf("spacing must be positive and margin nonnegative")
	}
	road, err := track.SampleRoad(scene, opts.Spacing)
	if err != nil {
		return Result{}, err
	}
	if len(road) < 3 {
		return Result{}, fmt.Errorf("road requires at least three samples; reduce spacing")
	}
	if offsets == nil {
		offsets = make([]float64, len(road))
	}
	if len(offsets) != len(road) {
		return Result{}, &lineError{fmt.Errorf("offsets: got %d, need %d road stations", len(offsets), len(road))}
	}
	if err := validatePeriodicOffsets(road, offsets); err != nil {
		return Result{}, &lineError{err}
	}
	clearance := model.Parameters().Width/2 + opts.Margin
	for i, offset := range offsets {
		if road[i].LeftLimit() <= clearance || road[i].RightLimit() <= clearance {
			return Result{}, fmt.Errorf("road at %.2f m is narrower than vehicle and clearance", road[i].S)
		}
		if !finite(offset) || offset > road[i].LeftLimit()-clearance || offset < -road[i].RightLimit()+clearance {
			return Result{}, &lineError{fmt.Errorf("offset at station %.2f m violates vehicle clearance", road[i].S)}
		}
	}
	spacing := math.Min(opts.Spacing, .5)
	if spacing < opts.Spacing {
		fine, err := track.SampleRoad(scene, spacing)
		if err != nil {
			return Result{}, err
		}
		offsets = interpolateOffsets(road, offsets, fine, clearance)
		road = fine
	}
	eval := evaluator{ctx: ctx, road: road, model: model, entry: scene.EntrySpeed, exit: scene.ExitSpeed, clearance: clearance, edges: indexRoadEdges(road, clearance)}
	isCenter := true
	for _, offset := range offsets {
		if offset != 0 {
			isCenter = false
			break
		}
	}
	// Establish baseline feasibility before attributing a profile failure to
	// a supplied line. This also distinguishes refined-road failures from seeds.
	baseline, err := eval.run(make([]float64, len(road)))
	if err != nil {
		return Result{}, fmt.Errorf("centreline infeasible: %w", err)
	}
	result := baseline
	if !isCenter {
		result, err = eval.run(offsets)
		if err != nil {
			return Result{}, &lineError{err}
		}
	}

	result.Offsets = append([]float64(nil), offsets...)
	result.Road = road
	result.CenterNodes = append([]Node(nil), baseline.Nodes...)
	result.CenterDuration = baseline.Duration
	result.CenterEntrySpeed = baseline.Nodes[0].Speed
	result.CenterExitSpeed = baseline.Nodes[len(baseline.Nodes)-1].Speed
	result.EntrySpeedCap = scene.EntrySpeed
	result.ExitSpeedCap = scene.ExitSpeed
	result.Spacing = spacing
	result.SearchSpacing = opts.Spacing
	result.CoarseDuration = result.Duration
	result.SelectedCoarseDuration = result.Duration
	result.SearchWorkers = 1
	result.Termination = "supplied line evaluated; no search"
	result.Candidates = 2
	if isCenter {
		result.Candidates = 1
	}
	return result, nil
}

// OffsetsAt samples a solved line at another road's normalized reference
// stations, for warm starts. It never mutates the trajectory or clamps width.
func (r Result) OffsetsAt(road []track.Sample) ([]float64, error) {
	if len(r.Nodes) < 2 || len(road) < 2 {
		return nil, fmt.Errorf("two road and trajectory stations required")
	}
	out := make([]float64, len(road))
	for i, sample := range road {
		n, err := r.AtStation(sample.S / road[len(road)-1].S * r.Nodes[len(r.Nodes)-1].Station)
		if err != nil {
			return nil, err
		}
		out[i] = n.Offset
	}
	return out, nil
}
