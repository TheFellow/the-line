package editor

import (
	"context"
	"errors"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// SolveSetup searches a setup's line, retrying a rejected warm start without
// its seed. Invalid scenes, models, baselines and cancellation return directly.
// onRetry, if non-nil, runs on the caller's goroutine before the fresh search.
func SolveSetup(ctx context.Context, scene track.Scene, model vehicle.Model, opts solver.Options, onRetry func()) (solver.Result, error) {
	result, err := solver.SolveContext(ctx, scene, model, opts)
	var seedErr *solver.SeedError
	if !errors.As(err, &seedErr) || ctx.Err() != nil {
		return result, err
	}
	if onRetry != nil {
		onRetry()
	}
	opts.Seed = nil
	return solver.SolveContext(ctx, scene, model, opts)
}
