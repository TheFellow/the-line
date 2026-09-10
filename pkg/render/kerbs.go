package render

import (
	"image"
	"image/color"
	"math"

	"github.com/TheFellow/the-line/pkg/track"
)

// drawKerbs uses explicit physical widths; unconfigured edges receive only the
// white road-limit line. The strips are projected onto the same authoritative
// triangle surface used by the trajectory, even on twisted bank transitions.
func (r *Renderer) drawKerbs(im *image.RGBA, a, b track.Sample) {
	col := color.RGBA{228, 230, 215, 255}
	if int(a.S/4)%2 == 0 {
		col = color.RGBA{199, 72, 66, 255}
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
			// Clip the strip against each ribbon triangle. A kerb crossing the
			// diagonal gets an inserted vertex at its authoritative height.
			ar, al := a.AtOffset(-a.RightLimit()), a.AtOffset(a.LeftLimit())
			br, bl := b.AtOffset(-b.RightLimit()), b.AtOffset(b.LeftLimit())
			for _, tri := range [][3]track.Vec3{{al, ar, bl}, {ar, br, bl}} {
				clipped := clipWorldTriangle(poly, tri)
				screen := make([]point, len(clipped))
				for i, p := range clipped {
					screen[i] = r.projected(p)
				}
				if len(screen) >= 3 {
					polygon(im, screen, col)
				}
			}
		} else {
			screen := make([]point, len(poly))
			for i, p := range poly {
				screen[i] = r.projected(p)
			}
			polygon(im, screen, col)
		}
	}
}

func clipWorldTriangle(poly []track.Vec3, tri [3]track.Vec3) []track.Vec3 {
	cross := func(a, b, p track.Vec3) float64 { return (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X) }
	sign := 1.
	if cross(tri[0], tri[1], tri[2]) < 0 {
		sign = -1
	}
	for j, a := range tri {
		b := tri[(j+1)%3]
		input := poly
		poly = nil
		if len(input) == 0 {
			break
		}
		last := input[len(input)-1]
		ld := sign * cross(a, b, last)
		for _, p := range input {
			pd := sign * cross(a, b, p)
			if (ld >= 0) != (pd >= 0) {
				f := ld / (ld - pd)
				poly = append(poly, last.Add(p.Sub(last).Mul(f)))
			}
			if pd >= 0 {
				poly = append(poly, p)
			}
			last, ld = p, pd
		}
	}
	den := cross(tri[0], tri[1], tri[2])
	if math.Abs(den) > 1e-12 {
		for i, p := range poly {
			wa := cross(tri[1], tri[2], p) / den
			wb := cross(tri[2], tri[0], p) / den
			poly[i].Z = wa*tri[0].Z + wb*tri[1].Z + (1-wa-wb)*tri[2].Z
		}
	}
	return poly
}
