package render

import (
	"image"
	"math"

	"github.com/TheFellow/the-line/pkg/track"
)

// perspectiveMarker identifies a car without enlarging its physical footprint.
// The current car has a green ring, the reference a dashed blue ring. These
// screen-space annotations never enter the depth buffer or reveal a hidden car.
func (r *Renderer) perspectiveMarker(im *image.RGBA, depths []depthSample, center track.Vec3, ghost bool) {
	camera := r.perspective.camera(center)
	if camera.Z < perspectiveNear {
		return
	}
	p := r.perspectiveScreen(camera)
	visible := func(x, y int) bool {
		if !image.Pt(x, y).In(r.viewport) {
			return false
		}
		surface := depths[(y-r.viewport.Min.Y)*r.viewport.Dx()+x-r.viewport.Min.X]
		// Compare at the car's projected center, correcting the covered pixel's
		// surface slope just as for the road's screen-space line overlays.
		occluder := surface.inverse + surface.dx*(p.x-float64(x)-.5) + surface.dy*(p.y-float64(y)-.5)
		return 1/camera.Z >= occluder-1e-10
	}
	if !visible(int(math.Floor(p.x)), int(math.Floor(p.y))) {
		return
	}
	const radius = 11.
	col := accent
	if ghost {
		col = referenceColor
	}
	bounds := image.Rect(int(math.Floor(p.x-radius-2)), int(math.Floor(p.y-radius-2)), int(math.Ceil(p.x+radius+2)), int(math.Ceil(p.y+radius+2))).Intersect(r.viewport)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !visible(x, y) {
				continue
			}
			dx, dy := float64(x)+.5-p.x, float64(y)+.5-p.y
			distance := math.Hypot(dx, dy)
			if distance < radius {
				halo := col
				halo.A = 20
				blend(im, x, y, halo, 1)
			}
			if ghost && int((math.Atan2(dy, dx)+math.Pi)*6/math.Pi)%2 == 0 {
				continue
			}
			coverage := math.Max(0, math.Min(1, 1.6-math.Abs(distance-radius)))
			blend(im, x, y, col, coverage)
		}
	}
}
