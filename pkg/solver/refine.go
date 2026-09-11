package solver

import (
	"math"
	"sync"
)

// refineLine searches the objective actually used for exports. Each poll is
// independent; only verified improvements, consumed in fixed order, survive.
// Combining successful directions lets an entry/apex/exit change move together.
func refineLine(eval evaluator, initial Result, rounds, workers int) (Result, int, error) {
	if rounds == 0 {
		return initial, 0, nil
	}
	best := initial
	count := 0
	n := len(eval.road)
	length := eval.road[n-1].S
	centers, bounds := make([]float64, n), make([]float64, n)
	for i, p := range eval.road {
		centers[i] = (p.LeftLimit() - p.RightLimit()) / 2
		bounds[i] = (p.LeftLimit()+p.RightLimit())/2 - eval.clearance
	}
	var directions [][]float64
	for _, parts := range []int{16, 32} {
		end := parts
		if !eval.closed() {
			end++
		}
		for j := 0; j < end; j++ {
			direction := make([]float64, n)
			center, radius := float64(j)*length/float64(parts), length/float64(parts)*1.5
			for i, p := range eval.road {
				distance := math.Abs(p.S - center)
				if eval.closed() {
					distance = math.Min(distance, length-distance)
				}
				if u := distance / radius; u < 1 {
					// The bump and its first two derivatives vanish at its edges.
					v := 1 - u*u
					direction[i] = v * v * v / bounds[i]
				}
			}
			directions = append(directions, direction)
		}
	}
	for round := 0; round < rounds; round++ {
		if err := eval.cancelled(); err != nil {
			return Result{}, count, err
		}
		latent := make([]float64, n)
		for i, offset := range best.Offsets {
			latent[i] = math.Atanh(clamp((offset-centers[i])/bounds[i], -.999999, .999999))
		}
		candidate := func(direction []float64, amplitude float64) []float64 {
			offsets := make([]float64, n)
			for i := range offsets {
				offsets[i] = centers[i] + bounds[i]*math.Tanh(latent[i]+amplitude*direction[i])
			}
			if eval.closed() {
				offsets[n-1] = offsets[0]
			}
			return offsets
		}
		// Settle broad corner combinations before spending evaluations on smaller
		// local changes. Both scales still use the final road and timing objective.
		wideCount := 16
		if !eval.closed() {
			wideCount++
		}
		wideRounds := min(3, max(1, rounds/2))
		active := directions[:wideCount]
		scaleRound := round
		if round >= wideRounds {
			active = directions[wideCount:]
			scaleRound -= wideRounds
		}
		// Keep the poll large enough to detect useful steps at nonsmooth changes
		// of the active braking/cornering constraints, then tighten it.
		step := .2 * math.Pow(.5, float64(scaleRound/3))
		poll := make([][]float64, 0, 2*len(active))
		for _, direction := range active {
			poll = append(poll, candidate(direction, -step), candidate(direction, step))
		}
		base := best.Duration
		combined := make([]float64, n)
		accept := func(result Result, offsets []float64, err error) {
			if err == nil && len(result.Nodes) > 0 && result.Duration < best.Duration-1e-7 {
				best = result
				best.Offsets = offsets
			}
		}
		visitCandidates(eval, poll, workers, func(j int, evaluation candidateEvaluation) {
			count++
			accept(evaluation.result, poll[j], evaluation.err)
			if evaluation.err != nil || len(evaluation.result.Nodes) == 0 {
				return
			}
			gain := base - evaluation.result.Duration
			if gain <= 0 {
				return
			}
			sign := -1.
			if j%2 == 1 {
				sign = 1
			}
			for i, value := range active[j/2] {
				combined[i] += sign * gain * value
			}
		})
		largest := 0.
		for i, value := range combined {
			largest = math.Max(largest, math.Abs(value)*bounds[i])
		}
		if largest > 0 {
			for i := range combined {
				combined[i] /= largest
			}
			proposals := [][]float64{candidate(combined, .25), candidate(combined, .5), candidate(combined, 1), candidate(combined, 2)}
			visitCandidates(eval, proposals, workers, func(j int, evaluation candidateEvaluation) {
				count++
				accept(evaluation.result, proposals[j], evaluation.err)
			})
		}
	}
	if err := eval.cancelled(); err != nil {
		return Result{}, count, err
	}
	return best, count, nil
}

// Keep workers busy when neighboring candidates take different amounts of work,
// while consuming results in index order. Stop dispatching when completed
// trajectories accumulate behind a slower earlier candidate. Rejections contain
// no dense nodes and can keep flowing; live dense-trajectory memory is bounded
// by the worker count, rather than the number of directions in the poll.
func visitCandidates(eval evaluator, offsets [][]float64, workers int, visit func(int, candidateEvaluation)) {
	workers = searchWorkers(eval.model, workers, len(offsets))
	if workers == 1 {
		for i, offset := range offsets {
			if eval.ctx.Err() != nil {
				return
			}
			result, err := eval.run(offset)
			visit(i, candidateEvaluation{result: result, err: err})
		}
		return
	}
	type completion struct {
		index      int
		evaluation candidateEvaluation
	}
	jobs := make(chan int)
	completed := make(chan completion)
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				if eval.ctx.Err() != nil {
					return
				}
				result, err := eval.run(offsets[index])
				select {
				case completed <- completion{index, candidateEvaluation{result: result, err: err}}:
				case <-eval.ctx.Done():
					return
				}
			}
		}()
	}
	defer func() { close(jobs); group.Wait() }()
	pending := make(map[int]candidateEvaluation)
	sent, next, densePending := 0, 0, 0
	for next < len(offsets) {
		dispatch := jobs
		if sent == len(offsets) || densePending >= workers {
			dispatch = nil
		}
		select {
		case dispatch <- sent:
			sent++
		case outcome := <-completed:
			pending[outcome.index] = outcome.evaluation
			if len(outcome.evaluation.result.Nodes) > 0 {
				densePending++
			}
			for {
				result, ready := pending[next]
				if !ready {
					break
				}
				visit(next, result)
				if len(result.result.Nodes) > 0 {
					densePending--
				}
				delete(pending, next)
				next++
			}
		case <-eval.ctx.Done():
			return
		}
	}
}

// Extra user-requested polish starts from the completed search, so increasing
// this budget cannot discard an already faster verified result.
func polishLine(eval evaluator, best Result, sweeps int) (Result, int, error) {
	count := 0
	n := len(eval.road)
	length := eval.road[n-1].S
	for sweep := 0; sweep < sweeps; sweep++ {
		for j := 0; j <= 16; j++ {
			center, radius := float64(j)*length/16, length/16*1.35
			for _, sign := range []float64{-1, 1} {
				if err := eval.cancelled(); err != nil {
					return Result{}, count, err
				}
				offsets := append([]float64(nil), best.Offsets...)
				for i, p := range eval.road {
					distance := math.Abs(p.S - center)
					if eval.closed() {
						distance = math.Min(distance, length-distance)
					}
					if u := distance / radius; u < 1 {
						center := (p.LeftLimit() - p.RightLimit()) / 2
						bound := (p.LeftLimit()+p.RightLimit())/2 - eval.clearance
						latent := math.Atanh(clamp((offsets[i]-center)/bound, -.999999, .999999))
						offsets[i] = center + bound*math.Tanh(latent+sign*.08*math.Pow(.5, float64(sweep))*(1+math.Cos(math.Pi*u))/2)
					}
				}
				if eval.closed() {
					offsets[n-1] = offsets[0]
				}
				result, err := eval.run(offsets)
				count++
				if err == nil && result.Duration < best.Duration-1e-7 {
					best = result
					best.Offsets = offsets
				}
			}
		}
	}
	if err := eval.cancelled(); err != nil {
		return Result{}, count, err
	}
	return best, count, nil
}
