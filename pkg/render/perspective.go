package render

import (
	"image"
	"image/color"
	"math"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

// The trackside camera uses a true pinhole projection with a near plane and a
// reciprocal-depth buffer. It is intentionally independent of editable camera
// state: there is no inverse projection for watch-only geometry.
type perspectiveScene struct {
	eye, right, up, forward track.Vec3
	focal                   float64
	depths                  []depthSample
	frameDepths             []depthSample
}

// depthSample retains the reciprocal-depth plane at a pixel center. Lines
// cover neighboring pixels too; comparing on this plane avoids self-occlusion
// caused by their different sample locations on a sloping road.
type depthSample struct {
	inverse, dx, dy float64
}

const perspectiveNear = .2

func dot(a, b track.Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (r *Renderer) preparePerspective() {
	if r.perspective != nil {
		return
	}
	lo, hi := track.Vec3{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}, track.Vec3{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, s := range r.result.Road {
		for _, off := range []float64{-s.RightWidth() - s.KerbRight.Width, s.LeftWidth() + s.KerbLeft.Width} {
			p := s.AtOffset(off)
			lo.X = math.Min(lo.X, p.X)
			lo.Y = math.Min(lo.Y, p.Y)
			lo.Z = math.Min(lo.Z, p.Z)
			hi.X = math.Max(hi.X, p.X)
			hi.Y = math.Max(hi.Y, p.Y)
			hi.Z = math.Max(hi.Z, p.Z)
		}
	}
	target := lo.Add(hi).Mul(.5)
	// Align the long axis with the wide viewport. This is a fixed trackside
	// composition, not a moving camera that can conceal line differences.
	var xx, xy, yy float64
	for _, sample := range r.result.Road {
		d := sample.Position.Sub(target)
		xx += d.X * d.X
		xy += d.X * d.Y
		yy += d.Y * d.Y
	}
	yaw, elevation := -.5*math.Atan2(2*xy, xx-yy), .67
	sy, cy := math.Sincos(yaw)
	se, ce := math.Sincos(elevation)
	forward := track.Vec3{X: sy * ce, Y: cy * ce, Z: -se}
	right := track.Vec3{X: cy, Y: -sy}
	up := track.Vec3{X: sy * se, Y: cy * se, Z: ce}
	focal := float64(r.viewport.Dy()) / (2 * math.Tan(25*math.Pi/180))
	// Fit against each projected bound, including depth, rather than a bounding
	// sphere: wide roads remain large enough to study in the cinematic viewport.
	distance := 20.
	for _, s := range r.result.Road {
		for _, off := range []float64{-s.RightWidth() - s.KerbRight.Width, s.LeftWidth() + s.KerbLeft.Width} {
			d := s.AtOffset(off).Sub(target)
			needed := math.Max(math.Abs(dot(d, right))*focal/(float64(r.viewport.Dx())*.44), math.Abs(dot(d, up))*focal/(float64(r.viewport.Dy())*.40))
			distance = math.Max(distance, needed-dot(d, forward)+8)
		}
	}
	r.perspective = &perspectiveScene{eye: target.Sub(forward.Mul(distance)), right: right, up: up, forward: forward, focal: focal, depths: make([]depthSample, r.viewport.Dx()*r.viewport.Dy())}
}
func (p *perspectiveScene) camera(v track.Vec3) track.Vec3 {
	d := v.Sub(p.eye)
	return track.Vec3{X: dot(d, p.right), Y: dot(d, p.up), Z: dot(d, p.forward)}
}
func (r *Renderer) perspectivePoint(v track.Vec3) point {
	r.preparePerspective()
	return r.perspectiveScreen(r.perspective.camera(v))
}
func (r *Renderer) perspectiveScreen(v track.Vec3) point {
	if v.Z < perspectiveNear {
		return point{-1e6, -1e6}
	}
	return point{float64(r.viewport.Min.X+r.viewport.Max.X)/2 + r.perspective.focal*v.X/v.Z, float64(r.viewport.Min.Y+r.viewport.Max.Y)/2 - r.perspective.focal*v.Y/v.Z}
}

// clipNear preserves winding and interpolates camera coordinates at z=near.
func clipNear(input []track.Vec3) []track.Vec3 {
	if len(input) == 0 {
		return nil
	}
	out := make([]track.Vec3, 0, len(input)+1)
	prev := input[len(input)-1]
	for _, v := range input {
		if (prev.Z >= perspectiveNear) != (v.Z >= perspectiveNear) {
			u := (perspectiveNear - prev.Z) / (v.Z - prev.Z)
			q := prev.Add(v.Sub(prev).Mul(u))
			q.Z = perspectiveNear
			out = append(out, q)
		}
		if v.Z >= perspectiveNear {
			out = append(out, v)
		}
		prev = v
	}
	return out
}
func (r *Renderer) perspectivePolygon(im *image.RGBA, depths []depthSample, world []track.Vec3, col color.RGBA) {
	vertices := make([]track.Vec3, len(world))
	for i, v := range world {
		vertices[i] = r.perspective.camera(v)
	}
	vertices = clipNear(vertices)
	for i := 1; i+1 < len(vertices); i++ {
		r.perspectiveTriangle(im, depths, [3]track.Vec3{vertices[0], vertices[i], vertices[i+1]}, col)
	}
}
func (r *Renderer) perspectiveTriangle(im *image.RGBA, depths []depthSample, v [3]track.Vec3, col color.RGBA) {
	a, b, c := r.perspectiveScreen(v[0]), r.perspectiveScreen(v[1]), r.perspectiveScreen(v[2])
	cross := func(a, b, p point) float64 { return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x) }
	area := cross(a, b, c)
	if math.Abs(area) < 1e-9 {
		return
	}
	gradientX := ((b.y-c.y)/v[0].Z + (c.y-a.y)/v[1].Z + (a.y-b.y)/v[2].Z) / area
	gradientY := ((c.x-b.x)/v[0].Z + (a.x-c.x)/v[1].Z + (b.x-a.x)/v[2].Z) / area
	bounds := image.Rect(int(math.Floor(math.Min(a.x, math.Min(b.x, c.x)))), int(math.Floor(math.Min(a.y, math.Min(b.y, c.y)))), int(math.Ceil(math.Max(a.x, math.Max(b.x, c.x))))+1, int(math.Ceil(math.Max(a.y, math.Max(b.y, c.y))))+1).Intersect(r.viewport)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			q := point{float64(x) + .5, float64(y) + .5}
			wa, wb := cross(b, c, q)/area, cross(c, a, q)/area
			wc := 1 - wa - wb
			if wa < -1e-8 || wb < -1e-8 || wc < -1e-8 {
				continue
			}
			depth := wa/v[0].Z + wb/v[1].Z + wc/v[2].Z
			idx := (y-r.viewport.Min.Y)*r.viewport.Dx() + x - r.viewport.Min.X
			if depth >= depths[idx].inverse-1e-10 {
				depths[idx] = depthSample{inverse: depth, dx: gradientX, dy: gradientY}
				im.SetRGBA(x, y, col)
			}
		}
	}
}
func (r *Renderer) perspectiveLine(im *image.RGBA, depths []depthSample, a, b track.Vec3, width float64, col color.RGBA) {
	ca, cb := r.perspective.camera(a), r.perspective.camera(b)
	if ca.Z < perspectiveNear && cb.Z < perspectiveNear {
		return
	}
	if ca.Z < perspectiveNear {
		ca = ca.Add(cb.Sub(ca).Mul((perspectiveNear - ca.Z) / (cb.Z - ca.Z)))
		ca.Z = perspectiveNear
	}
	if cb.Z < perspectiveNear {
		cb = cb.Add(ca.Sub(cb).Mul((perspectiveNear - cb.Z) / (ca.Z - cb.Z)))
		cb.Z = perspectiveNear
	}
	pa, pb := r.perspectiveScreen(ca), r.perspectiveScreen(cb)
	// Clip in screen space before stepping, bounding work near the near plane.
	lo, hi := 0., 1.
	dx, dy := pb.x-pa.x, pb.y-pa.y
	for _, edge := range [][2]float64{{-dx, pa.x - float64(r.viewport.Min.X)}, {dx, float64(r.viewport.Max.X) - pa.x}, {-dy, pa.y - float64(r.viewport.Min.Y)}, {dy, float64(r.viewport.Max.Y) - pa.y}} {
		p, q := edge[0], edge[1]
		if p == 0 {
			if q < 0 {
				return
			}
			continue
		}
		u := q / p
		if p < 0 {
			lo = math.Max(lo, u)
		} else {
			hi = math.Min(hi, u)
		}
	}
	if lo > hi {
		return
	}
	steps := max(1, int(math.Ceil(math.Hypot(dx, dy)*(hi-lo)*1.5)))
	radius := max(0, int(width/2))
	for i := 0; i <= steps; i++ {
		u := lo + (hi-lo)*float64(i)/float64(steps)
		sampleX, sampleY := pa.x+dx*u, pa.y+dy*u
		x, y := int(math.Floor(sampleX)), int(math.Floor(sampleY))
		depth := (1-u)/ca.Z + u/cb.Z
		for oy := -radius; oy <= radius; oy++ {
			for ox := -radius; ox <= radius; ox++ {
				px, py := x+ox, y+oy
				if !image.Pt(px, py).In(r.viewport) {
					continue
				}
				idx := (py-r.viewport.Min.Y)*r.viewport.Dx() + px - r.viewport.Min.X
				surface := depths[idx]
				// Evaluate the occluding plane at the line sample, not the
				// covered pixel center. This is exact for a planar triangle.
				occluder := surface.inverse + surface.dx*(sampleX-float64(px)-.5) + surface.dy*(sampleY-float64(py)-.5)
				if depth >= occluder-1e-10 {
					im.SetRGBA(px, py, col)
				}
			}
		}
	}
}
func (r *Renderer) perspectiveRoad(im *image.RGBA) {
	r.preparePerspective()
	clear(r.perspective.depths)
	depths := r.perspective.depths
	for y := r.viewport.Min.Y; y < r.viewport.Max.Y; y++ {
		u := float64(y-r.viewport.Min.Y) / float64(r.viewport.Dy())
		fill(im, image.Rect(r.viewport.Min.X, y, r.viewport.Max.X, y+1), color.RGBA{uint8(19 + 9*u), uint8(32 + 10*u), uint8(42 + 4*u), 255})
	}
	road := r.result.Road
	for i := 1; i < len(road); i++ {
		a, b := road[i-1], road[i]
		col := color.RGBA{67, 77, 84, 255}
		switch a.Surface {
		case "gravel":
			col = color.RGBA{112, 100, 79, 255}
		case "dirt":
			col = color.RGBA{100, 78, 64, 255}
		case "wet":
			col = color.RGBA{57, 77, 94, 255}
		case "ice":
			col = color.RGBA{125, 159, 176, 255}
		}
		al, ar, bl, br := a.AtOffset(a.LeftLimit()), a.AtOffset(-a.RightLimit()), b.AtOffset(b.LeftLimit()), b.AtOffset(-b.RightLimit())
		r.perspectivePolygon(im, depths, []track.Vec3{al, ar, bl}, col)
		r.perspectivePolygon(im, depths, []track.Vec3{ar, br, bl}, col)
		kerb := color.RGBA{228, 230, 215, 255}
		if int(a.S/4)%2 == 0 {
			kerb = color.RGBA{199, 72, 66, 255}
		}
		for _, side := range []float64{-1, 1} {
			aw, bw, ak, bk := a.LeftWidth(), b.LeftWidth(), a.KerbLeft.Width, b.KerbLeft.Width
			if side < 0 {
				aw, bw, ak, bk = a.RightWidth(), b.RightWidth(), a.KerbRight.Width, b.KerbRight.Width
			}
			if ak == 0 && bk == 0 {
				continue
			}
			poly := []track.Vec3{a.AtOffset(side * aw), b.AtOffset(side * bw), b.AtOffset(side * (bw + bk)), a.AtOffset(side * (aw + ak))}
			if a.KerbsCountAsRoad {
				for _, tri := range [][3]track.Vec3{{al, ar, bl}, {ar, br, bl}} {
					r.perspectivePolygon(im, depths, clipWorldTriangle(poly, tri), kerb)
				}
			} else {
				r.perspectivePolygon(im, depths, poly, kerb)
			}
		}
	}
	lift := track.Vec3{Z: .04}
	for i := 1; i < len(road); i++ {
		a, b := road[i-1], road[i]
		for _, side := range []float64{-1, 1} {
			r.perspectiveLine(im, depths, a.AtOffset(a.EdgeOffset(side)).Add(lift), b.AtOffset(b.EdgeOffset(side)).Add(lift), 1, ink)
		}
		if int(a.S/5)%2 == 0 {
			r.perspectiveLine(im, depths, a.Position.Add(lift), b.Position.Add(lift), 1, muted)
		}
	}
	for i := 1; i < len(r.result.Nodes); i++ {
		a, b := r.result.Nodes[i-1], r.result.Nodes[i]
		r.perspectiveLine(im, depths, a.Position.Add(lift), b.Position.Add(lift), 3, r.opts.LineColor.color(b))
	}
	r.text(im, r.viewport.Min.X+16, r.viewport.Min.Y+25, "TRACKSIDE / PERSPECTIVE", 12, ink, true)
	r.text(im, r.viewport.Min.X+16, r.viewport.Min.Y+44, "Watch only · transport and plots remain interactive", 11, muted, false)
}
func (r *Renderer) perspectiveCar(im *image.RGBA, depths []depthSample, n, a, b solver.Node, ghost bool) {
	forward := b.Position.Sub(a.Position)
	forward.Z = 0
	if forward.Length() < 1e-6 {
		forward = track.Vec3{X: 1}
	}
	forward = forward.Mul(1 / forward.Length())
	side := track.Vec3{X: -forward.Y, Y: forward.X, Z: math.Tan(n.Bank * math.Pi / 180)}
	forward.Z = n.Grade
	center := n.Position.Add(track.Vec3{Z: .45})
	width := r.vehicle.Width * .5
	if ghost && r.opts.Reference != nil {
		width = r.opts.Reference.Vehicle.Width * .5
	}
	corners := make([]track.Vec3, 4)
	for i, p := range [][2]float64{{2, width}, {2, -width}, {-2, -width}, {-2, width}} {
		corners[i] = center.Add(forward.Mul(p[0])).Add(side.Mul(p[1]))
	}
	col := ink
	if ghost {
		col = referenceColor
	}
	r.perspectiveMarker(im, depths, center, ghost)
	r.perspectivePolygon(im, depths, corners, col)
	nose := center.Add(forward.Mul(2.1)).Add(track.Vec3{Z: .03})
	r.perspectiveLine(im, depths, nose.Add(side.Mul(width)), nose.Sub(side.Mul(width)), 2, accent)
	roof := center.Add(track.Vec3{Z: .25})
	r.perspectiveLine(im, depths, roof.Sub(forward.Mul(.5)), roof.Add(forward.Mul(.6)), 2, bg)
}
func (r *Renderer) perspectiveFrame(im *image.RGBA, t float64, n solver.Node, state State) {
	if len(r.perspective.frameDepths) != len(r.perspective.depths) {
		r.perspective.frameDepths = make([]depthSample, len(r.perspective.depths))
	}
	depths := r.perspective.frameDepths
	copy(depths, r.perspective.depths)
	if state.Comparison && r.referenceCompatible() {
		headingTime := t
		if !r.result.Closed {
			headingTime = math.Max(0, math.Min(t, r.referenceDuration()))
		}
		a, b := r.referenceAt(headingTime-.08), r.referenceAt(headingTime+.08)
		r.perspectiveCar(im, depths, r.referenceAt(t), a, b, true)
	}
	headingTime := t
	if !r.result.Closed {
		headingTime = math.Max(0, math.Min(t, r.result.Duration))
	}
	r.perspectiveCar(im, depths, n, r.currentAt(headingTime-.08), r.currentAt(headingTime+.08), false)
}
