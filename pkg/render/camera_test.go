package render

import (
	"math"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestCameraProjectionAcrossViewsAndLetterboxing(t *testing.T) {
	for _, view := range []string{"2d", "3d"} {
		base := fixture(t, view)
		for _, size := range [][2]int{{1440, 900}, {1000, 700}, {800, 600}} {
			r, err := New(base.scene, base.result, base.vehicle, Options{View: view, Width: size[0], Height: size[1]})
			if err != nil {
				t.Fatal(err)
			}
			for _, yaw := range []float64{0, math.Pi / 2, math.Pi, -math.Pi / 3, 2 * math.Pi} {
				for _, elevation := range []float64{MinElevation, math.Pi / 4, MaxElevation} {
					c := r.Camera()
					c.Yaw, c.Elevation, c.Zoom = yaw, elevation, 3.25
					c.PanX, c.PanY = 51, -28
					c.Target = track.Vec3{X: 13, Y: 5, Z: 2}
					// Projection tests do not need the unrelated paint pass.
					if err := r.setCamera(c); err != nil {
						t.Fatal(err)
					}
					for _, p := range []track.Vec3{{}, {X: -91, Y: 83, Z: -4}, {X: 72, Y: 23, Z: 15}} {
						x, y := r.Project(p)
						got := r.Unproject(x, y, p.Z)
						if got.Sub(p).Length() > 1e-10 {
							t.Fatalf("%s %v yaw%.2f elevation%.2f: inverse %v != %v", view, size, yaw, elevation, got, p)
						}
					}
					x, y := r.Project(c.Target)
					wantX := (float64(r.viewport.Min.X+r.viewport.Max.X)/2+c.PanX)*r.displayScale + r.displayX
					wantY := (float64(r.viewport.Min.Y+r.viewport.Max.Y)/2+c.PanY)*r.displayScale + r.displayY
					if math.Hypot(x-wantX, y-wantY) > 1e-10 {
						t.Fatal("camera target did not remain at panned viewport centre")
					}
				}
			}
		}
	}
}

func TestOrthographicCameraIndependentAxisLengths(t *testing.T) {
	r := fixture(t, "3d")
	c := r.Camera()
	c.Yaw, c.Elevation, c.Zoom = 0, math.Pi/6, 4
	if err := r.setCamera(c); err != nil {
		t.Fatal(err)
	}
	originX, originY := r.Project(track.Vec3{})
	// Looking along -Y at 30 degrees: world X remains horizontal; world Y
	// foreshortens to one half, world Z to sqrt(3)/2. No inverse code is used.
	for _, tc := range []struct {
		p      track.Vec3
		dx, dy float64
	}{{track.Vec3{X: 1}, 4, 0}, {track.Vec3{Y: 1}, 0, -2}, {track.Vec3{Z: 1}, 0, -2 * math.Sqrt(3)}} {
		x, y := r.Project(tc.p)
		if math.Abs((x-originX)/r.displayScale-tc.dx) > 1e-10 || math.Abs((y-originY)/r.displayScale-tc.dy) > 1e-10 {
			t.Fatal("orthographic basis disagrees with analytical axis projection")
		}
	}
}

func TestCameraBoundsValidationAndReuse(t *testing.T) {
	r := fixture(t, "3d")
	initial := r.Camera()
	face, base := r.faces[12], r.base
	c := initial
	c.Elevation, c.Zoom, c.Yaw = -1, 1000, 6*math.Pi
	if err := r.SetCamera(c); err != nil {
		t.Fatal(err)
	}
	bounded := r.Camera()
	if bounded.Elevation != MinElevation || bounded.Zoom != MaxZoom || math.Abs(bounded.Yaw) > 1e-12 {
		t.Fatalf("camera bounds were not applied: %+v", bounded)
	}
	if r.faces[12] != face || r.base != base {
		t.Fatal("camera motion reallocated typography or cached image")
	}
	for _, invalid := range []Camera{{Zoom: 0}, {Zoom: 1, Yaw: math.NaN()}, {Zoom: 1, Elevation: math.Inf(1)}, {Zoom: 1, PanX: 1e20}} {
		if err := r.SetCamera(invalid); err == nil {
			t.Fatalf("accepted invalid camera: %+v", invalid)
		}
		if r.Camera() != bounded {
			t.Fatal("rejected camera changed the projection")
		}
	}
	r.ResetCamera()
	if r.Camera() != initial {
		t.Fatal("reset did not restore fitted camera")
	}
}

func TestCameraPreservedAcrossRendererRebuild(t *testing.T) {
	r := fixture(t, "3d")
	c := r.Camera()
	c.Yaw += 1.1
	c.PanX, c.PanY = -70, 42
	c.Zoom *= 1.4
	if err := r.SetCamera(c); err != nil {
		t.Fatal(err)
	}
	c = r.Camera()
	scene := r.scene
	scene.Points = append([]track.Point(nil), scene.Points...)
	scene.Points[3].X += 3
	rebuilt, err := New(scene, r.result, r.vehicle, Options{View: "3d", Width: 800, Height: 600, Camera: &c})
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Camera() != c {
		t.Fatal("rebuild refitted the supplied camera")
	}
	for _, p := range r.scene.Points {
		x, y := r.Project(p.Position())
		xx, yy := rebuilt.Project(p.Position())
		if x != xx || y != yy {
			t.Fatal("rebuild moved an unchanged world point")
		}
	}
}
