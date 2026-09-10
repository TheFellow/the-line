package render

import (
	"fmt"
	"image"
	"math"

	"github.com/TheFellow/the-line/pkg/track"
)

// Camera is an orthographic presentation transform, independent of road edits.
// Angles are radians. Zoom is canonical display pixels per world metre; pan is
// measured in the renderer's 1440×900 canonical display pixels. Target is the
// world point at the centre of the viewport before pan. Plan view ignores yaw
// and elevation, retaining them for the next elevated view.
type Camera struct {
	Yaw       float64    `json:"yaw"`
	Elevation float64    `json:"elevation"`
	Zoom      float64    `json:"zoom"`
	PanX      float64    `json:"panX"`
	PanY      float64    `json:"panY"`
	Target    track.Vec3 `json:"target"`
}

const (
	MinElevation = 20 * math.Pi / 180
	MaxElevation = 80 * math.Pi / 180
	MinZoom      = .02
	MaxZoom      = 100
)

// Camera returns a value copy, suitable for preserving the camera across a new
// Renderer after an edit. Changing a camera never modifies the scene or result.
func (r *Renderer) Camera() Camera { return r.camera }

// RoadViewport is the road's visible and interactive area, in display pixels.
// It excludes the heading, chart, timeline and sidebar, including letterboxing.
func (r *Renderer) RoadViewport() image.Rectangle { return r.displayRect(r.viewport) }

// SetCamera validates and bounds a camera, then repaints the cached road using
// existing typography. It deliberately does not refit on yaw or elevation changes.
func (r *Renderer) SetCamera(c Camera) error {
	if err := r.setCamera(c); err != nil {
		return err
	}
	r.drawBase()
	return nil
}

func (r *Renderer) setCamera(c Camera) error {
	for _, v := range []float64{c.Yaw, c.Elevation, c.Zoom, c.PanX, c.PanY, c.Target.X, c.Target.Y, c.Target.Z} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("camera values must be finite")
		}
	}
	if c.Zoom <= 0 {
		return fmt.Errorf("camera zoom must be positive")
	}
	c.Yaw = math.Remainder(c.Yaw, 2*math.Pi)
	c.Elevation = math.Max(MinElevation, math.Min(MaxElevation, c.Elevation))
	c.Zoom = math.Max(MinZoom, math.Min(MaxZoom, c.Zoom))
	// Keep projected coordinates representable even for malformed caller input.
	if math.Abs(c.PanX) > 1e6 || math.Abs(c.PanY) > 1e6 || math.Abs(c.Target.X) > 1e9 || math.Abs(c.Target.Y) > 1e9 || math.Abs(c.Target.Z) > 1e9 {
		return fmt.Errorf("camera pan or target is out of range")
	}
	r.camera = c
	r.scale = c.Zoom
	q := r.raw(c.Target)
	r.ox = float64(r.viewport.Min.X+r.viewport.Max.X)/2 + c.PanX - q.x*r.scale
	r.oy = float64(r.viewport.Min.Y+r.viewport.Max.Y)/2 + c.PanY - q.y*r.scale
	return nil
}

// ResetCamera restores the default orientation and fits the current road.
func (r *Renderer) ResetCamera() {
	r.fit()
	r.drawBase()
}

func (r *Renderer) raw(p track.Vec3) point {
	if r.opts.View == "2d" {
		return point{p.X, -p.Y}
	}
	s, c := math.Sincos(r.camera.Yaw)
	se, ce := math.Sincos(r.camera.Elevation)
	return point{c*p.X - s*p.Y, -se*(s*p.X+c*p.Y) - ce*p.Z}
}

// depth is signed distance toward the camera; paint smaller values first.
func (r *Renderer) depth(p track.Vec3) float64 {
	if r.opts.View == "2d" {
		return p.Z
	}
	s, c := math.Sincos(r.camera.Yaw)
	se, ce := math.Sincos(r.camera.Elevation)
	return -ce*(s*p.X+c*p.Y) + se*p.Z
}

func (r *Renderer) fit() {
	r.camera = Camera{Yaw: math.Pi / 6, Elevation: math.Pi / 5, Zoom: 1}
	lo, hi := point{math.Inf(1), math.Inf(1)}, point{math.Inf(-1), math.Inf(-1)}
	minZ, maxZ := math.Inf(1), math.Inf(-1)
	for _, s := range r.result.Road {
		for _, off := range []float64{-s.RightWidth() - s.KerbRight.Width, s.LeftWidth() + s.KerbLeft.Width} {
			world := s.AtOffset(off)
			p := r.raw(world)
			lo.x, lo.y = math.Min(lo.x, p.x), math.Min(lo.y, p.y)
			hi.x, hi.y = math.Max(hi.x, p.x), math.Max(hi.y, p.y)
			minZ, maxZ = math.Min(minZ, world.Z), math.Max(maxZ, world.Z)
		}
	}
	c := r.camera
	c.Zoom = math.Min(float64(r.viewport.Dx()-110)/math.Max(hi.x-lo.x, 1), float64(r.viewport.Dy()-75)/math.Max(hi.y-lo.y, 1))
	c.Target = r.inverseRaw((lo.x+hi.x)/2, (lo.y+hi.y)/2, (minZ+maxZ)/2)
	// Sampled roads and their bounds are validated before rendering.
	_ = r.setCamera(c)
}

func (r *Renderer) inverseRaw(x, y, z float64) track.Vec3 {
	if r.opts.View == "2d" {
		return track.Vec3{X: x, Y: -y, Z: z}
	}
	s, c := math.Sincos(r.camera.Yaw)
	se, ce := math.Sincos(r.camera.Elevation)
	forward := -(y + ce*z) / se
	return track.Vec3{X: c*x + s*forward, Y: -s*x + c*forward, Z: z}
}

// Project maps a world-space point to display coordinates.
func (r *Renderer) Project(p track.Vec3) (float64, float64) {
	q := r.projected(p)
	return q.x*r.displayScale + r.displayX, q.y*r.displayScale + r.displayY
}

// Unproject intersects a display coordinate with a constant-elevation plane.
// The bounded elevation keeps this inverse well conditioned for handle edits.
// Watch-only perspective has no editing inverse and returns NaN coordinates.
func (r *Renderer) Unproject(x, y, z float64) track.Vec3 {
	if r.opts.View == "perspective" {
		return track.Vec3{X: math.NaN(), Y: math.NaN(), Z: math.NaN()}
	}
	x = (x - r.displayX) / r.displayScale
	y = (y - r.displayY) / r.displayScale
	return r.inverseRaw((x-r.ox)/r.scale, (y-r.oy)/r.scale, z)
}
