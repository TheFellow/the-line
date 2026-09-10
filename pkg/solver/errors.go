package solver

// SeedError identifies a rejected warm-start line. Callers may retry without
// the seed; geometry, vehicle and centreline failures are not seed failures.
// Unwrap preserves cancellation and any underlying validation error.
type SeedError struct{ Err error }

func (e *SeedError) Error() string { return "seed: " + e.Err.Error() }
func (e *SeedError) Unwrap() error { return e.Err }
