package render

import (
	"bytes"
	"github.com/TheFellow/the-line/pkg/track"
	"image"
	"image/color"
	"math"
	"testing"
)

func TestPerspectivePinholeAndDepthOcclusion(t *testing.T) {
	r := fixture(t, "perspective")
	r.perspective = &perspectiveScene{forward: track.Vec3{Z: 1}, right: track.Vec3{X: 1}, up: track.Vec3{Y: 1}, focal: 200, depths: make([]depthSample, r.viewport.Dx()*r.viewport.Dy())}
	cx := float64(r.viewport.Min.X+r.viewport.Max.X) / 2
	near, far := r.perspectivePoint(track.Vec3{X: 1, Z: 5}), r.perspectivePoint(track.Vec3{X: 1, Z: 10})
	if math.Abs((near.x-cx)-2*(far.x-cx)) > 1e-10 {
		t.Fatal("pinhole projection must halve apparent width at twice the depth")
	}
	im := image.NewRGBA(r.base.Bounds())
	red, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}
	triangle := func(z float64) []track.Vec3 {
		return []track.Vec3{{X: -1, Y: -1, Z: z}, {X: 1, Y: -1, Z: z}, {X: 0, Y: 1, Z: z}}
	}
	r.perspectivePolygon(im, r.perspective.depths, triangle(2), red)
	r.perspectivePolygon(im, r.perspective.depths, triangle(5), blue)
	cy := (r.viewport.Min.Y + r.viewport.Max.Y) / 2
	if im.RGBAAt(int(cx), cy) != red {
		t.Fatal("far triangle overwrote near triangle")
	}
	clear(r.perspective.depths)
	r.perspectivePolygon(im, r.perspective.depths, triangle(5), blue)
	r.perspectivePolygon(im, r.perspective.depths, triangle(2), red)
	if im.RGBAAt(int(cx), cy) != red {
		t.Fatal("occlusion depends on submission order")
	}
}
func TestPerspectiveNearClipAndFrameIsolation(t *testing.T) {
	poly := clipNear([]track.Vec3{{X: -1, Z: -1}, {X: 1, Z: 1}, {Y: 1, Z: 1}})
	if len(poly) != 4 {
		t.Fatalf("near-clipped triangle has %d vertices", len(poly))
	}
	for _, p := range poly {
		if p.Z < perspectiveNear {
			t.Fatal("behind-camera point survived")
		}
	}
	if len(clipNear([]track.Vec3{{Z: -2}, {Z: -1}, {Z: 0}})) != 0 {
		t.Fatal("behind-camera triangle survived")
	}
	r := fixture(t, "perspective")
	first := append([]byte(nil), r.Frame(0).(*image.RGBA).Pix...)
	r.Frame(r.result.Duration * .5)
	if !bytes.Equal(first, r.Frame(0).(*image.RGBA).Pix) {
		t.Fatal("depth state leaked between frames")
	}
	if r.controls["watch"].Empty() || r.controls["station+"].Empty() {
		t.Fatal("watch and station controls missing")
	}
}

func TestPerspectiveRoadOverlayIsContinuousWithoutPiercingForeground(t *testing.T) {
	r := fixture(t, "perspective")
	r.perspective = &perspectiveScene{
		forward: track.Vec3{Z: 1}, right: track.Vec3{X: 1}, up: track.Vec3{Y: 1},
		focal: 200, depths: make([]depthSample, r.viewport.Dx()*r.viewport.Dy()),
	}
	// A plane with depth gradients along both screen axes. The line lies
	// exactly on it, while its three-pixel stroke covers neighboring depths.
	world := func(x, y, scale float64) track.Vec3 {
		z := scale / (.2 + .0005*x + .0008*y)
		return track.Vec3{X: x * z / 200, Y: -y * z / 200, Z: z}
	}
	quad := func(left, right, scale float64) []track.Vec3 {
		return []track.Vec3{world(left, -30, scale), world(right, -30, scale), world(right, 30, scale), world(left, 30, scale)}
	}
	im := image.NewRGBA(r.base.Bounds())
	road, stripe, foreground := color.RGBA{70, 70, 70, 255}, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}
	r.perspectivePolygon(im, r.perspective.depths, quad(-60, 60, 1), road)
	r.perspectivePolygon(im, r.perspective.depths, quad(10, 20, .5), foreground)
	r.perspectiveLine(im, r.perspective.depths, world(-50, 0, 1), world(50, 0, 1), 3, stripe)
	cx, cy := (r.viewport.Min.X+r.viewport.Max.X)/2, (r.viewport.Min.Y+r.viewport.Max.Y)/2
	for x := -48; x <= 48; x++ {
		for y := -1; y <= 1; y++ {
			want := stripe
			if x >= 10 && x < 20 {
				want = foreground
			}
			if got := im.RGBAAt(cx+x, cy+y); got != want {
				t.Fatalf("overlay at (%d,%d): got %v, want %v", x, y, got, want)
			}
		}
	}
}
