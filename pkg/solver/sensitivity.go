package solver

import (
	"context"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Sensitivity is a one-sided finite difference on the same fixed path, not a
// prediction about separately optimized lines. Delta is changed minus base time.
type Sensitivity struct {
	Parameter string  `json:"parameter"`
	Change    float64 `json:"change"`
	Duration  float64 `json:"duration"`
	Delta     float64 `json:"delta"`
}

func Sensitivities(ctx context.Context, scene track.Scene, config vehicle.Config, line Result, margin float64) ([]Sensitivity, error) {
	opts := Options{Spacing: line.Spacing, Margin: margin}
	base, err := EvaluateContext(ctx, scene, config, line.Offsets, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Sensitivity, 0, 4)
	for _, change := range []Sensitivity{{Parameter: "mass", Change: 50}, {Parameter: "power", Change: 25000}, {Parameter: "grip", Change: .05}, {Parameter: "drag_area", Change: .1}} {
		value, _ := config.Value(change.Parameter)
		car, err := config.With(change.Parameter, value+change.Change)
		if err != nil {
			return nil, err
		}
		result, err := EvaluateContext(ctx, scene, car, line.Offsets, opts)
		if err != nil {
			return nil, err
		}
		change.Duration = result.Duration
		change.Delta = result.Duration - base.Duration
		out = append(out, change)
	}
	return out, nil
}
