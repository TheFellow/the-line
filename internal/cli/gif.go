package cli

import (
	"image"
	"image/color"
	"sort"
)

// gifQuantizer builds a palette from the shared studio frame, then uses a compact
// lookup table for animation. The static road already contains the complete speed
// ramp. Avoiding a 256-colour search per pixel keeps long exports practical.
type gifQuantizer struct {
	palette color.Palette
	lookup  [32768]uint8
}
type colorBin struct {
	count, r, g, b int64
	key            int
}

func newGIFQuantizer(im image.Image) *gifQuantizer {
	histogram := make([]colorBin, 32768)
	bounds := im.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := im.At(x, y).RGBA()
			rr, gg, bb := int64(r>>8), int64(g>>8), int64(b>>8)
			key := int((rr>>3)<<10 | (gg>>3)<<5 | bb>>3)
			h := &histogram[key]
			h.count++
			h.r += rr
			h.g += gg
			h.b += bb
			h.key = key
		}
	}
	bins := make([]colorBin, 0, 32768)
	for _, h := range histogram {
		if h.count > 0 {
			bins = append(bins, h)
		}
	}
	boxes := [][]colorBin{bins}
	channel := func(h colorBin, c int) int { return (h.key >> uint(10-c*5)) & 31 }
	for len(boxes) < 256 {
		best, axis := -1, 0
		var bestScore int64
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			var population int64
			low, high := [3]int{31, 31, 31}, [3]int{}
			for _, h := range box {
				population += h.count
				for c := 0; c < 3; c++ {
					v := channel(h, c)
					low[c] = min(low[c], v)
					high[c] = max(high[c], v)
				}
			}
			for c := 0; c < 3; c++ {
				score := population * int64(high[c]-low[c])
				if score > bestScore {
					best, bestScore, axis = i, score, c
				}
			}
		}
		if best < 0 {
			break
		}
		box := boxes[best]
		sort.Slice(box, func(i, j int) bool {
			a, b := channel(box[i], axis), channel(box[j], axis)
			if a == b {
				return box[i].key < box[j].key
			}
			return a < b
		})
		var population int64
		for _, h := range box {
			population += h.count
		}
		var sum int64
		split := 1
		for split < len(box) {
			sum += box[split-1].count
			if sum >= population/2 {
				break
			}
			split++
		}
		split = min(split, len(box)-1)
		boxes[best] = box[:split]
		boxes = append(boxes, box[split:])
	}
	q := &gifQuantizer{}
	for _, box := range boxes {
		var n, r, g, b int64
		for _, h := range box {
			n += h.count
			r += h.r
			g += h.g
			b += h.b
		}
		q.palette = append(q.palette, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255})
	}
	for key := range q.lookup {
		r, g, b := (key>>10)*8+4, ((key>>5)&31)*8+4, (key&31)*8+4
		bestDistance, best := int(^uint(0)>>1), 0
		for i, c := range q.palette {
			cc := c.(color.RGBA)
			dr, dg, db := r-int(cc.R), g-int(cc.G), b-int(cc.B)
			d := dr*dr + dg*dg + db*db
			if d < bestDistance {
				bestDistance, best = d, i
			}
		}
		q.lookup[key] = uint8(best)
	}
	return q
}
func (q *gifQuantizer) frame(im image.Image) *image.Paletted {
	out := image.NewPaletted(im.Bounds(), q.palette)
	if rgba, ok := im.(*image.RGBA); ok {
		for y := 0; y < rgba.Rect.Dy(); y++ {
			src := rgba.Pix[y*rgba.Stride:]
			dst := out.Pix[y*out.Stride:]
			for x := 0; x < rgba.Rect.Dx(); x++ {
				i := x * 4
				key := int(src[i]>>3)<<10 | int(src[i+1]>>3)<<5 | int(src[i+2]>>3)
				dst[x] = q.lookup[key]
			}
		}
		return out
	}
	for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
		for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
			r, g, b, _ := im.At(x, y).RGBA()
			key := int(r>>11)<<10 | int(g>>11)<<5 | int(b>>11)
			out.SetColorIndex(x, y, q.lookup[key])
		}
	}
	return out
}
