package solver

import (
	"context"
	"fmt"
	"math"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Options controls sampling and the finite search budget. Margin is additional
// horizontal clearance beyond half the vehicle width (a circular approximation).
type Options struct {
	Spacing    float64
	Iterations int
	Margin     float64
	// Seed contains lateral offsets at SampleRoad(scene, Spacing) stations.
	Seed []float64
}

func DefaultOptions() Options { return Options{Spacing: 3, Iterations: 4, Margin: .25} }

type Node struct {
	Position track.Vec3 `json:"position"`
	// S is actual three-dimensional distance along this trajectory, in metres.
	S float64 `json:"s"`
	// Station is distance along the sampled reference centreline, in metres.
	// It is shared by optimized and centreline paths and varies linearly with
	// segment distance between road stations and inserted triangle crossings.
	Station      float64 `json:"station"`
	Time         float64 `json:"time"`
	Speed        float64 `json:"speed"`
	Curvature    float64 `json:"curvature"`
	Offset       float64 `json:"offset"`
	Acceleration float64 `json:"acceleration"`
	// Force state uses the outgoing segment; the terminal node uses the final
	// incoming segment. Grade is actual path rise/run; Grip is conservative.
	Bank   float64            `json:"bank_deg"`
	Grade  float64            `json:"grade"`
	Grip   float64            `json:"grip"`
	Forces vehicle.TyreForces `json:"tyre_forces"`
}

type Result struct {
	Closed bool `json:"closed,omitempty"`
	model  vehicle.Model
	Nodes  []Node `json:"nodes"`
	// Offsets are lateral offsets on Road; use Spacing to evaluate them exactly.
	Offsets []float64 `json:"offsets"`
	// CenterNodes is the verified centreline trajectory under the same model,
	// surfaces, clearance and requested endpoint caps as Nodes.
	CenterNodes      []Node         `json:"center_nodes"`
	Road             []track.Sample `json:"-"`
	Duration         float64        `json:"duration"`
	CenterDuration   float64        `json:"center_duration"`
	Length           float64        `json:"length"`
	Iterations       int            `json:"iterations"`
	Candidates       int            `json:"candidates"`
	Spacing          float64        `json:"spacing"`
	SearchSpacing    float64        `json:"search_spacing"`
	CoarseDuration   float64        `json:"coarse_duration"`
	EntrySpeedCap    float64        `json:"entry_speed_cap"`
	ExitSpeedCap     float64        `json:"exit_speed_cap"`
	CenterEntrySpeed float64        `json:"center_entry_speed"`
	CenterExitSpeed  float64        `json:"center_exit_speed"`
	Termination      string         `json:"termination"`
	MaxForceResidual float64        `json:"max_force_residual"`
}

// Solve treats open-road entry/exit speeds as upper bounds and leaves endpoint offsets
// and headings free. Baseline and candidate share these conditions, but their
// realized endpoint speeds can differ. Closed laps instead identify the endpoint
// state periodically and ignore these caps. Only feasible time improvements survive.
func Solve(scene track.Scene, model vehicle.Model, opts Options) (Result, error) {
	return SolveContext(context.Background(), scene, model, opts)
}

// SolveContext cancels between bounded numerical work units.
func SolveContext(ctx context.Context, scene track.Scene, model vehicle.Model, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if model == nil {
		return Result{}, fmt.Errorf("vehicle model is required")
	}
	if err := model.Parameters().Validate(); err != nil {
		return Result{}, err
	}
	if !finite(opts.Spacing) || opts.Spacing <= 0 || !finite(opts.Margin) || opts.Margin < 0 || opts.Iterations < 0 || opts.Iterations > 30 {
		return Result{}, fmt.Errorf("spacing must be positive, margin nonnegative, and iterations between 0 and 30")
	}
	if opts.Seed != nil && opts.Iterations == 0 {
		return EvaluateContext(ctx, scene, model, opts.Seed, opts)
	}
	road, err := track.SampleRoad(scene, opts.Spacing)
	if err != nil {
		return Result{}, err
	}
	n := len(road)
	if n < 3 {
		return Result{}, fmt.Errorf("road requires at least three samples; reduce spacing")
	}
	clearance := model.Parameters().Width/2 + opts.Margin
	bounds := make([]float64, n)
	centers := make([]float64, n)
	for i, s := range road {
		bounds[i] = (s.LeftLimit()+s.RightLimit())/2 - clearance
		centers[i] = (s.LeftLimit() - s.RightLimit()) / 2
		if s.LeftLimit() <= clearance || s.RightLimit() <= clearance {
			return Result{}, fmt.Errorf("road at %.1f m is narrower than vehicle and clearance", s.S)
		}
	}
	eval := evaluator{ctx: ctx, road: road, model: model, entry: scene.EntrySpeed, exit: scene.ExitSpeed, clearance: clearance}
	offsets := make([]float64, n)
	best, err := eval.run(offsets)
	if err != nil {
		return Result{}, fmt.Errorf("centreline infeasible: %w", err)
	}
	best.CenterDuration = best.Duration
	best.CenterEntrySpeed = best.Nodes[0].Speed
	best.CenterExitSpeed = best.Nodes[len(best.Nodes)-1].Speed
	baseline := best
	count := 1
	var incumbents [][]float64
	var seedResult *Result
	if opts.Seed != nil {
		r, err := EvaluateContext(ctx, scene, model, opts.Seed, opts)
		if err != nil {
			return Result{}, fmt.Errorf("seed: %w", err)
		}
		seedResult = &r
		seeded, err := eval.run(opts.Seed)
		if err != nil {
			return Result{}, fmt.Errorf("seed: %w", err)
		}
		if seeded.Duration < best.Duration {
			best = seeded
			copy(offsets, opts.Seed)
		}
		incumbents = append(incumbents, append([]float64(nil), opts.Seed...))
	}
	accept := func(candidate []float64) bool {
		count++
		r, e := eval.run(candidate)
		if e == nil && r.Duration < best.Duration-1e-7 {
			best = r
			copy(offsets, candidate)
			incumbents = append(incumbents, append([]float64(nil), candidate...))
			return true
		}
		return false
	}
	// A bounded geometric fairing seed gives coordinated corner-entry/exit moves.
	seed := append([]float64(nil), offsets...)
	for k := 0; k < 80; k++ {
		next := append([]float64(nil), seed...)
		for i := 1; i < n-1; i++ {
			p := road[i].AtOffset(seed[i])
			a := road[i-1].AtOffset(seed[i-1])
			b := road[i+1].AtOffset(seed[i+1])
			d := (.5*(a.X+b.X)-p.X)*road[i].Normal.X + (.5*(a.Y+b.Y)-p.Y)*road[i].Normal.Y
			next[i] = clamp(seed[i]+.65*d, centers[i]-.9*bounds[i], centers[i]+.9*bounds[i])
		}
		if scene.Closed {
			i := 0
			p, a, b := road[i].AtOffset(seed[i]), road[n-2].AtOffset(seed[n-2]), road[1].AtOffset(seed[1])
			d := (.5*(a.X+b.X)-p.X)*road[i].Normal.X + (.5*(a.Y+b.Y)-p.Y)*road[i].Normal.Y
			next[0] = clamp(seed[0]+.65*d, centers[0]-.9*bounds[0], centers[0]+.9*bounds[0])
			next[n-1] = next[0]
		}
		seed = next
	}
	if opts.Iterations > 0 {
		accept(seed)
	}
	completed := 0
	for iteration := 0; iteration < opts.Iterations; iteration++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		// Cosine bumps have zero derivative at their support edges. All candidates
		// are evaluated through the complete sequence, including braking downstream.
		for _, parts := range []int{4, 8, 16} {
			radius := road[n-1].S / float64(parts) * 1.35
			amp := math.Pow(.62, float64(iteration))
			for j := 0; j <= parts; j++ {
				center := float64(j) * road[n-1].S / float64(parts)
				for _, sign := range []float64{-1, 1} {
					candidate := append([]float64(nil), offsets...)
					for i := range candidate {
						delta := math.Abs(road[i].S - center)
						if scene.Closed {
							delta = math.Min(delta, road[n-1].S-delta)
						}
						u := delta / radius
						if u < 1 {
							latent := math.Atanh(clamp((candidate[i]-centers[i])/bounds[i], -.999, .999))
							candidate[i] = centers[i] + bounds[i]*math.Tanh(latent+sign*amp*(1+math.Cos(math.Pi*u))/2)
						}
					}
					if scene.Closed {
						candidate[n-1] = candidate[0]
					}
					accept(candidate)
				}
			}
		}
		completed++
		// A smaller next sweep can improve even when the current amplitude cannot.
	}
	coarseDuration := best.Duration
	// Search geometry is provisional. Export and compare on a denser road, so
	// coarse curvature samples cannot produce an optimistic displayed trajectory.
	finalSpacing := math.Min(opts.Spacing, .5)
	if finalSpacing < opts.Spacing {
		fine, err := track.SampleRoad(scene, finalSpacing)
		if err != nil {
			return Result{}, err
		}
		refined := eval
		refined.road = fine
		baseline, err = refined.run(make([]float64, len(fine)))
		if err != nil {
			return Result{}, fmt.Errorf("refined centreline infeasible: %w", err)
		}
		count++
		best = baseline
		best.Offsets = make([]float64, len(fine))
		// Coarse winners can exchange rank after curvature refinement. Retain a
		// bounded, evenly spaced history so an optimistic late winner cannot
		// discard an earlier physically faster line.
		checks := min(16, len(incumbents))
		for j := 0; j < checks; j++ {
			index := 0
			if checks > 1 {
				index = j * (len(incumbents) - 1) / (checks - 1)
			}
			candidateOffsets := interpolateOffsets(road, incumbents[index], fine, clearance)
			candidate, candidateErr := refined.run(candidateOffsets)
			count++
			if candidateErr == nil && candidate.Duration < best.Duration {
				best = candidate
				best.Offsets = candidateOffsets
			}
		}
		road = fine
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if seedResult != nil && seedResult.Duration < best.Duration {
		best = *seedResult
	}
	if best.Offsets == nil {
		best.Offsets = append([]float64(nil), offsets...)
	}
	best.CenterNodes = append([]Node(nil), baseline.Nodes...)
	best.CenterDuration = baseline.Duration
	best.CenterEntrySpeed = baseline.Nodes[0].Speed
	best.CenterExitSpeed = baseline.Nodes[len(baseline.Nodes)-1].Speed
	best.Road = road
	best.Candidates = count
	best.Iterations = completed
	best.Spacing = finalSpacing
	best.SearchSpacing = opts.Spacing
	best.CoarseDuration = coarseDuration
	best.EntrySpeedCap = scene.EntrySpeed
	best.ExitSpeedCap = scene.ExitSpeed
	best.Termination = "search budget exhausted"
	if completed < opts.Iterations || opts.Iterations == 0 {
		best.Termination = "no improving smooth offset found"
	}
	return best, nil
}

type pathState struct {
	node       Node
	bank, grip float64
}
type evaluator struct {
	ctx                    context.Context
	road                   []track.Sample
	model                  vehicle.Model
	entry, exit, clearance float64
}
