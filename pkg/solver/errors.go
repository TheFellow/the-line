package solver

import (
	"context"
	"errors"
)

// SeedError identifies a rejected warm-start line. Callers may retry without
// the seed; geometry, vehicle and centreline failures are not seed failures.
// Unwrap preserves cancellation and any underlying validation error.
type SeedError struct{ Err error }

func (e *SeedError) Error() string { return "seed: " + e.Err.Error() }
func (e *SeedError) Unwrap() error { return e.Err }

// lineError marks failures caused by supplied offsets, rather than road/model
// validation or the independently required centreline baseline.
type lineError struct{ err error }

func (e *lineError) Error() string { return e.err.Error() }
func (e *lineError) Unwrap() error { return e.err }
func seedFailure(err error) error {
	var line *lineError
	if errors.As(err, &line) && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return &SeedError{Err: err}
	}
	return err
}
