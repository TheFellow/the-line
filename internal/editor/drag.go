package editor

import (
	"math"

	"github.com/TheFellow/the-line/pkg/track"
)

// Projection maps pointer coordinates to the horizontal plane through a control.
// The renderer implements this interface in both views and at any display size.
type Projection interface {
	Project(track.Vec3) (float64, float64)
	Unproject(x, y, z float64) track.Vec3
}

// Pick finds the nearest control within a forgiving display-space grab radius.
func Pick(scene track.Scene, projection Projection, x, y float64) int {
	best, distance := -1, 24.0
	for i, p := range scene.Points {
		px, py := projection.Project(p.Position())
		if d := math.Hypot(px-x, py-y); d < distance {
			best, distance = i, d
		}
	}
	return best
}

// Drag preserves the initial pointer-to-handle offset, so grabbing a label or
// edge of a handle does not snap the control to the cursor on release.
type Drag struct {
	Index          int
	StartX, StartY float64
	Origin         track.Vec3
}

func BeginDrag(scene track.Scene, projection Projection, x, y float64) *Drag {
	i := Pick(scene, projection, x, y)
	if i < 0 {
		return nil
	}
	return &Drag{Index: i, StartX: x, StartY: y, Origin: scene.Points[i].Position()}
}

func (d Drag) Position(projection Projection, x, y float64) track.Vec3 {
	start := projection.Unproject(d.StartX, d.StartY, d.Origin.Z)
	end := projection.Unproject(x, y, d.Origin.Z)
	return d.Origin.Add(end.Sub(start))
}

// Commit makes one undoable edit on release. A click leaves geometry and history
// untouched; invalid geometry is rejected by the editor transaction.
func (d Drag) Commit(e *Editor, projection Projection, x, y float64) (bool, error) {
	if math.Hypot(x-d.StartX, y-d.StartY) <= 2 {
		return false, nil
	}
	if err := e.MovePoint(d.Index, d.Position(projection, x, y)); err != nil {
		return false, err
	}
	return true, nil
}
