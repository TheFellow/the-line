package editor

import (
	"math"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

// ManualControls uses sparse, stable road indices. Dense persisted offsets
// preserve the exact evaluated line; these controls recover its authoring handles.
func ManualControls(road []track.Sample, offsets []float64, count int) []solver.LineControl {
	minimum := 2
	if len(road) > 0 && road[0].Closed {
		minimum = 4
	}
	count = min(len(road), max(minimum, count))
	controls := make([]solver.LineControl, count)
	for i := range controls {
		index := i * (len(road) - 1) / (count - 1)
		controls[i].Index = index
		if len(offsets) == len(road) {
			controls[i].Offset = offsets[index]
		}
	}
	return controls
}

type LineProjector interface {
	Project(track.Vec3) (float64, float64)
}

// LineDrag captures one lateral degree of freedom. Projecting the full banked
// cross-section includes its z component, unlike a fixed-height geometry drag.
type LineDrag struct {
	Control      int
	StartOffset  float64
	x, y, dx, dy float64
}

func BeginLineDrag(road []track.Sample, controls []solver.LineControl, p LineProjector, x, y float64) *LineDrag {
	nearest := 15.0
	var drag *LineDrag
	for i, c := range controls {
		if len(road) > 0 && road[0].Closed && c.Index == len(road)-1 {
			continue
		}
		sample := road[c.Index]
		px, py := p.Project(sample.AtOffset(c.Offset))
		distance := math.Hypot(x-px, y-py)
		if distance > nearest {
			continue
		}
		qx, qy := p.Project(sample.AtOffset(c.Offset + 1))
		dx, dy := qx-px, qy-py
		if dx*dx+dy*dy < .01 {
			continue
		}
		nearest = distance
		drag = &LineDrag{Control: i, StartOffset: c.Offset, x: x, y: y, dx: dx, dy: dy}
	}
	return drag
}

func (d LineDrag) Offset(x, y float64) float64 {
	return d.StartOffset + ((x-d.x)*d.dx+(y-d.y)*d.dy)/(d.dx*d.dx+d.dy*d.dy)
}
