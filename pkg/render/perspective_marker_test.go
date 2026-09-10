package render

import (
	"bytes"
	"image"
	"image/color"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestPerspectiveMarkersIdentifyCarsWithoutChangingDepth(t *testing.T) {
	r := fixture(t, "perspective")
	r.perspective = &perspectiveScene{
		forward: track.Vec3{Z: 1}, right: track.Vec3{X: 1}, up: track.Vec3{Y: 1}, focal: 200,
		depths: make([]depthSample, r.viewport.Dx()*r.viewport.Dy()),
	}
	var images [][]byte
	for _, ghost := range []bool{false, true} {
		im := image.NewRGBA(r.base.Bounds())
		fill(im, r.viewport, bg)
		before := append([]byte(nil), im.Pix...)
		r.perspectiveMarker(im, r.perspective.depths, track.Vec3{Z: 100}, ghost)
		if bytes.Equal(before, im.Pix) {
			t.Fatal("distant car has no identification marker")
		}
		cx, cy := (r.viewport.Min.X+r.viewport.Max.X)/2, (r.viewport.Min.Y+r.viewport.Max.Y)/2
		if im.RGBAAt(cx+20, cy) != bg {
			t.Fatal("identification marker escaped its restrained screen-space bounds")
		}
		images = append(images, append([]byte(nil), im.Pix...))
	}
	if bytes.Equal(images[0], images[1]) {
		t.Fatal("current and reference identification markers are indistinguishable")
	}
	for _, depth := range r.perspective.depths {
		if depth != (depthSample{}) {
			t.Fatal("identification marker changed the physical footprint's depth buffer")
		}
	}
}

func TestPerspectiveMarkerRespectsFullAndPartialOcclusion(t *testing.T) {
	r := fixture(t, "perspective")
	r.perspective = &perspectiveScene{
		forward: track.Vec3{Z: 1}, right: track.Vec3{X: 1}, up: track.Vec3{Y: 1}, focal: 200,
		depths: make([]depthSample, r.viewport.Dx()*r.viewport.Dy()),
	}
	cx, cy := (r.viewport.Min.X+r.viewport.Max.X)/2, (r.viewport.Min.Y+r.viewport.Max.Y)/2
	occluder := color.RGBA{60, 60, 90, 255}
	for _, fullyHidden := range []bool{true, false} {
		im := image.NewRGBA(r.base.Bounds())
		fill(im, r.viewport, occluder)
		for y := r.viewport.Min.Y; y < r.viewport.Max.Y; y++ {
			for x := r.viewport.Min.X; x < r.viewport.Max.X; x++ {
				depth := 0.
				if fullyHidden || x > cx+4 {
					depth = 1. / 50 // Foreground is twice as near as the car.
				}
				r.perspective.depths[(y-r.viewport.Min.Y)*r.viewport.Dx()+x-r.viewport.Min.X] = depthSample{inverse: depth}
			}
		}
		before := append([]byte(nil), im.Pix...)
		depths := append([]depthSample(nil), r.perspective.depths...)
		r.perspectiveMarker(im, r.perspective.depths, track.Vec3{Z: 100}, false)
		if fullyHidden && !bytes.Equal(before, im.Pix) {
			t.Fatal("halo revealed a fully hidden car")
		}
		if !fullyHidden {
			if bytes.Equal(before, im.Pix) {
				t.Fatal("visible portion of marker was lost")
			}
			if im.RGBAAt(cx+11, cy) != occluder {
				t.Fatal("halo painted through a nearer foreground surface")
			}
		}
		if !reflect.DeepEqual(depths, r.perspective.depths) {
			t.Fatal("annotation changed scene occlusion")
		}
	}
}
