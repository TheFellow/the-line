package editor

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

type bankedProjector struct{ yaw, pitch, scale float64 }

func (p bankedProjector) Project(v track.Vec3) (float64, float64) {
	return p.scale * (v.X*math.Cos(p.yaw) - v.Y*math.Sin(p.yaw)), p.scale * ((v.X*math.Sin(p.yaw)+v.Y*math.Cos(p.yaw))*math.Sin(p.pitch) - v.Z*math.Cos(p.pitch))
}
func TestManualDragBankedOrbitAndGrabOffset(t *testing.T) {
	road := []track.Sample{{Normal: track.Vec3{Y: 1}, Bank: 20}}
	controls := []solver.LineControl{{Index: 0, Offset: 1}}
	for _, yaw := range []float64{0, .7, 2.5, 4.2} {
		for _, scale := range []float64{1, 2, 5} {
			p := bankedProjector{yaw: yaw, pitch: .6, scale: scale}
			x, y := p.Project(road[0].AtOffset(1))
			dx, dy := p.Project(road[0].AtOffset(3))
			drag := BeginLineDrag(road, controls, p, x+3, y-2)
			if drag == nil {
				t.Fatal("handle not picked")
			}
			if got := drag.Offset(dx+3, dy-2); math.Abs(got-3) > 1e-10 {
				t.Fatalf("banked camera drag: got %g want 3", got)
			}
			if got := drag.Offset(x+3, y-2); got != 1 {
				t.Fatal("click-only moved handle")
			}
		}
	}
}

func TestClosedManualSeamPicksUniqueHandle(t *testing.T) {
	s, _ := track.Preset("club-loop")
	road, err := track.SampleRoad(s, 3)
	if err != nil {
		t.Fatal(err)
	}
	controls := ManualControls(road, nil, 2)
	if len(controls) < 4 {
		t.Fatal("periodic manual controls require three unique handles")
	}
	p := bankedProjector{yaw: .7, pitch: .6, scale: 2}
	x, y := p.Project(road[0].Position)
	d := BeginLineDrag(road, controls, p, x, y)
	if d == nil || d.Control != 0 {
		t.Fatal("duplicate hidden seam captured drag")
	}
}
