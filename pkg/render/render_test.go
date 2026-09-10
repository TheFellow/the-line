package render

import (
	"bytes"
	"image"
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func fixture(t *testing.T, view string) *Renderer {
	t.Helper()
	s, err := track.Preset("banked")
	if err != nil {
		t.Fatal(err)
	}
	v, err := vehicle.Preset("road")
	if err != nil {
		t.Fatal(err)
	}
	road, err := track.SampleRoad(s, 3)
	if err != nil {
		t.Fatal(err)
	}
	result := solver.Result{Road: road}
	for _, sample := range road {
		result.Nodes = append(result.Nodes, solver.Node{Position: sample.Position, S: sample.S, Time: sample.S / 20, Speed: 20})
	}
	result.Length = road[len(road)-1].S
	result.Duration = result.Length / 20
	result.CenterDuration = result.Duration
	r, err := New(s, result, v, Options{Width: 800, Height: 600, View: view})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestProjectionRoundTripAtElevation(t *testing.T) {
	for _, view := range []string{"2d", "3d"} {
		r := fixture(t, view)
		for _, p := range r.scene.Points {
			v := track.Vec3{X: p.X, Y: p.Y, Z: p.Z}
			x, y := r.Project(v)
			got := r.Unproject(x, y, v.Z)
			if math.Hypot(got.X-v.X, got.Y-v.Y) > 1e-6 || got.Z != v.Z {
				t.Fatalf("%s projection round trip: %#v != %#v", view, got, v)
			}
		}
	}
}

func TestFrameAnimationAndViews(t *testing.T) {
	r := fixture(t, "3d")
	first := append([]byte(nil), r.Frame(0).(*image.RGBA).Pix...)
	second := r.Frame(r.result.Duration * .5).(*image.RGBA)
	if second.Bounds() != image.Rect(0, 0, 800, 600) {
		t.Fatal(second.Bounds())
	}
	if bytes.Equal(first, second.Pix) {
		t.Fatal("car/time overlay did not advance")
	}
	// Render again at the same time: animation must be deterministic and cannot
	// progressively paint into the cached road image.
	if !bytes.Equal(first, r.Frame(0).(*image.RGBA).Pix) {
		t.Fatal("frame contaminated cached scene")
	}
	plan := fixture(t, "2d").Frame(0).(*image.RGBA)
	if bytes.Equal(first, plan.Pix) {
		t.Fatal("plan and elevated views are identical")
	}
	for key, rect := range r.Controls() {
		if !rect.In(second.Bounds()) {
			t.Fatalf("%s control outside display: %v", key, rect)
		}
	}
}

func TestDragPreviewDoesNotReplaceSolvedScene(t *testing.T) {
	for _, view := range []string{"2d", "3d"} {
		r := fixture(t, view)
		before := append([]byte(nil), r.Frame(0).(*image.RGBA).Pix...)
		p := r.scene.Points[3].Position().Add(track.Vec3{X: 8, Y: 6})
		frame := r.FrameWithState(0, State{Selected: 3, Drag: &DragPreview{Index: 3, Position: p}}).(*image.RGBA)
		x, y := r.Project(p)
		changed := false
		for yy := int(y) - 5; yy <= int(y)+5; yy++ {
			for xx := int(x) - 5; xx <= int(x)+5; xx++ {
				offset := frame.PixOffset(xx, yy)
				if !bytes.Equal(before[offset:offset+4], frame.Pix[offset:offset+4]) {
					changed = true
				}
			}
		}
		if !changed {
			t.Fatalf("%s: no visible handle at drag destination", view)
		}
		if !bytes.Equal(before, r.Frame(0).(*image.RGBA).Pix) {
			t.Fatalf("%s: preview modified the solved scene", view)
		}
	}
}
