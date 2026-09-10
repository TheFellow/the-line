package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/TheFellow/the-line/pkg/solver"
)

var referenceColor = color.RGBA{115, 193, 225, 255}

// PlaybackDuration includes the slower reference finish when its ghost is shown.
func (r *Renderer) PlaybackDuration(comparison bool) float64 {
	if comparison && len(r.result.CenterNodes) > 1 {
		return math.Max(r.result.Duration, r.result.CenterDuration)
	}
	return r.result.Duration
}

// ChartStation converts a display pointer to the common road coordinate, not
// either car's travelled distance. Dragging outside the chart clamps to its ends.
func (r *Renderer) ChartStation(x float64) float64 {
	rect := r.displayRect(r.controls["chart"])
	first, last := r.result.Nodes[0].Station, r.result.Nodes[len(r.result.Nodes)-1].Station
	f := math.Max(0, math.Min(1, (x-float64(rect.Min.X))/float64(rect.Dx())))
	return first + f*(last-first)
}

func (r *Renderer) chartPoint(n solver.Node) point {
	rect := r.controls["chart"]
	first, last := r.result.Nodes[0].Station, r.result.Nodes[len(r.result.Nodes)-1].Station
	return point{float64(rect.Min.X) + (n.Station-first)/math.Max(last-first, 1e-9)*float64(rect.Dx()), float64(rect.Max.Y) - n.Speed*3.6/r.chartMaxSpeed()*float64(rect.Dy())}
}

func (r *Renderer) chartMaxSpeed() float64 {
	if r.comparisonMax > 0 {
		return r.comparisonMax
	}
	maxSpeed := 1.0
	for _, nodes := range [][]solver.Node{r.result.Nodes, r.result.CenterNodes} {
		for _, n := range nodes {
			maxSpeed = math.Max(maxSpeed, n.Speed*3.6)
		}
	}
	r.comparisonMax = math.Ceil(maxSpeed/50) * 50
	return r.comparisonMax
}

func (r *Renderer) comparisonBase(im *image.RGBA) {
	r.controls["chart"] = image.Rect(82, 646, 1078, 722)
	rect := r.controls["chart"]
	fill(im, image.Rect(28, 618, 1090, 754), color.RGBA{19, 28, 37, 255})
	r.text(im, 42, 636, "SPEED / ROAD STATION", 11, muted, true)
	line(im, point{222, 631}, point{246, 631}, 2, accent)
	r.text(im, 252, 636, "Optimized", 11, ink, false)
	for x := 333; x < 357; x += 8 {
		line(im, point{float64(x), 631}, point{float64(x + 4), 631}, 2, referenceColor)
	}
	r.text(im, 363, 636, "Centreline reference", 11, referenceColor, false)
	r.text(im, 43, 749, "km/h", 11, muted, false)
	maxSpeed := r.chartMaxSpeed()
	for j := 0; j <= 2; j++ {
		y := rect.Max.Y - j*rect.Dy()/2
		line(im, point{float64(rect.Min.X), float64(y)}, point{float64(rect.Max.X), float64(y)}, 1, faint)
		r.text(im, 44, y+4, fmt.Sprintf("%.0f", float64(j)*maxSpeed/2), 11, muted, false)
	}
	first, last := r.result.Nodes[0].Station, r.result.Nodes[len(r.result.Nodes)-1].Station
	for j := 0; j <= 4; j++ {
		x := rect.Min.X + j*rect.Dx()/4
		line(im, point{float64(x), float64(rect.Min.Y)}, point{float64(x), float64(rect.Max.Y)}, 1, color.RGBA{35, 48, 60, 255})
		r.text(im, x-8, 739, fmt.Sprintf("%.0f m", first+float64(j)*(last-first)/4), 11, muted, false)
	}
	plot := im.SubImage(rect.Inset(-1)).(*image.RGBA)
	for k, nodes := range [][]solver.Node{r.result.CenterNodes, r.result.Nodes} {
		for i := 1; i < len(nodes); i++ {
			if k == 0 && int(r.chartPoint(nodes[i]).x/7)%2 == 0 {
				continue
			}
			col := accent
			if k == 0 {
				col = referenceColor
			}
			line(plot, r.chartPoint(nodes[i-1]), r.chartPoint(nodes[i]), 2, col)
		}
	}
}

func (r *Renderer) comparisonFrame(im *image.RGBA, t float64, n solver.Node, state State) {
	rect := r.controls["chart"]
	cursor := r.chartPoint(n)
	line(im, point{cursor.x, float64(rect.Min.Y)}, point{cursor.x, float64(rect.Max.Y)}, 1, ink)
	circle(im, cursor, 3.5, accent)
	if center, err := r.result.CenterAtStation(n.Station); err == nil {
		circle(im, r.chartPoint(center), 3, referenceColor)
		delta := n.Time - center.Time
		col := accent
		if delta > 0 {
			col = color.RGBA{255, 145, 113, 255}
		}
		r.text(im, 568, 636, fmt.Sprintf("%+.3f s  at same road station", delta), 13, col, true)
	}
	r.text(im, 922, 636, "Drag chart to inspect", 11, muted, false)
	ghostLabel := "GHOST OFF"
	if state.Comparison {
		ghostLabel = "GHOST ON"
	}
	r.text(im, 752, r.opts.Height-65, ghostLabel, 12, referenceColor, true)
	rate := state.Rate
	if rate <= 0 {
		rate = 1
	}
	r.text(im, 922, r.opts.Height-65, fmt.Sprintf("SPEED %.2g×", rate), 12, ink, true)
	if state.Comparison {
		r.text(im, 736, r.opts.Height-41, "Ghost: shared elapsed time", 11, muted, false)
		scrub := r.controls["scrub"]
		finish := float64(scrub.Min.X) + float64(scrub.Dx())*r.result.Duration/r.PlaybackDuration(true)
		line(im, point{finish, float64(scrub.Min.Y - 3)}, point{finish, float64(scrub.Max.Y)}, 1, accent)
		r.text(im, int(finish)-80, scrub.Min.Y-7, "LINE FINISH", 11, accent, false)
		if t >= r.result.Duration && t < r.result.CenterDuration {
			r.text(im, 735, r.opts.Height-96, "Line finished · reference running", 11, referenceColor, false)
		}
	}
}

func (r *Renderer) ghost(im *image.RGBA, t float64) {
	n := r.result.CenterAt(t)
	before := r.result.CenterAt(math.Max(0, t-.08))
	after := r.result.CenterAt(math.Min(r.result.CenterDuration, t+.08))
	r.drawVehicle(im, n, before, after, true)
}
