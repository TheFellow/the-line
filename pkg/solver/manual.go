package solver

import (
	"fmt"

	"github.com/TheFellow/the-line/pkg/track"
)

// LineControl places a horizontal lateral offset at an authoritative road
// sample index. First and last controls anchor the road endpoints. On a closed
// road they share the same offset and bracket at least three unique handles.
type LineControl struct {
	Index  int
	Offset float64
}

// ManualOffsets expands sparse lateral handles using the same bounded latent
// C2 cubic as search refinement. Invalid handles are rejected, never clamped.
func ManualOffsets(road []track.Sample, controls []LineControl, clearance float64) ([]float64, error) {
	if len(road) < 2 || len(controls) < 2 || !finite(clearance) || clearance < 0 {
		return nil, fmt.Errorf("manual line: two road samples and controls with nonnegative clearance required")
	}
	if controls[0].Index != 0 || controls[len(controls)-1].Index != len(road)-1 {
		return nil, fmt.Errorf("manual line: endpoint controls required")
	}
	coarse := make([]track.Sample, len(controls))
	offsets := make([]float64, len(controls))
	for i, c := range controls {
		if c.Index < 0 || c.Index >= len(road) || i > 0 && c.Index <= controls[i-1].Index {
			return nil, fmt.Errorf("manual line: control indices must increase")
		}
		left, right := road[c.Index].LeftLimit()-clearance, road[c.Index].RightLimit()-clearance
		if !finite(c.Offset) || left <= 0 || right <= 0 || c.Offset < -right || c.Offset > left {
			return nil, fmt.Errorf("manual line at station %.2f m violates vehicle clearance", road[c.Index].S)
		}
		coarse[i], offsets[i] = road[c.Index], c.Offset
	}
	if road[0].Closed && len(controls) < 4 {
		return nil, fmt.Errorf("manual closed line: at least three unique handles and a repeated seam required")
	}
	if err := validatePeriodicOffsets(coarse, offsets); err != nil {
		return nil, err
	}
	for _, p := range road {
		if p.LeftLimit() <= clearance || p.RightLimit() <= clearance {
			return nil, fmt.Errorf("manual line at station %.2f m has no vehicle clearance", p.S)
		}
	}
	return interpolateOffsets(coarse, offsets, road, clearance), nil
}
