package solver

import (
	"context"
	"runtime"
	"sync"

	"github.com/TheFellow/the-line/pkg/vehicle"
)

type candidateEvaluation struct {
	result Result
	err    error
}

// Only independent final candidates run concurrently. The coordinate search
// remains sequential, and callers consume this slice in its original order.
// Replacement models have no thread-safety requirement, so remain sequential.
func evaluateCandidates(ctx context.Context, eval evaluator, offsets [][]float64, workers int) []candidateEvaluation {
	out := make([]candidateEvaluation, len(offsets))
	workers = searchWorkers(eval.model, workers, len(offsets))
	if workers <= 1 {
		for i := range offsets {
			if ctx.Err() != nil {
				break
			}
			out[i].result, out[i].err = eval.run(offsets[i])
		}
		return out
	}
	jobs := make(chan int)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					continue
				}
				out[i].result, out[i].err = eval.run(offsets[i])
			}
		}()
	}
	for i := range offsets {
		select {
		case jobs <- i:
		case <-ctx.Done():
			close(jobs)
			group.Wait()
			return out
		}
	}
	close(jobs)
	group.Wait()
	return out
}
func searchWorkers(model vehicle.Model, requested, candidates int) int {
	switch model.(type) {
	case vehicle.Config, *vehicle.Config:
	default:
		return 1
	}
	if runtime.GOOS == "js" {
		return 1
	}
	if requested == 0 {
		requested = min(4, runtime.GOMAXPROCS(0))
	}
	return min(max(1, requested), max(1, candidates))
}

func shortlistIndices(total, limit int) []int {
	checks := min(total, limit)
	indices := make([]int, checks)
	for j := range indices {
		if checks > 1 {
			indices[j] = j * (total - 1) / (checks - 1)
		}
	}
	return indices
}
