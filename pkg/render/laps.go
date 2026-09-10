package render

import (
	"math"

	"github.com/TheFellow/the-line/pkg/solver"
)

func (r *Renderer) currentAt(t float64) solver.Node { return r.result.LapAt(t) }

func (r *Renderer) frameTime(t float64, state State) float64 {
	t = math.Max(0, t)
	if !r.result.Closed {
		t = math.Min(t, r.PlaybackDuration(state.Comparison))
	}
	return t
}

// frameNode keeps road, chart and force inspection on the same lap. Pausing at
// the first lap's exact endpoint deliberately selects the end of the chart.
func (r *Renderer) frameNode(t float64, state State) solver.Node {
	t = r.frameTime(t, state)
	if r.result.Closed && !state.Playing && t <= r.result.Duration {
		return r.result.At(t)
	}
	return r.currentAt(t)
}
