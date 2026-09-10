package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// Channel selects an absolute display scale shared by cars and trajectories.
type Channel string

const (
	SpeedChannel        Channel = "speed"
	GripChannel         Channel = "utilization"
	LateralChannel      Channel = "lateral_g"
	LongitudinalChannel Channel = "longitudinal_g"
)

func Channels() []Channel {
	return []Channel{SpeedChannel, GripChannel, LateralChannel, LongitudinalChannel}
}
func (c Channel) Valid() bool {
	for _, candidate := range Channels() {
		if c == candidate {
			return true
		}
	}
	return false
}
func (c Channel) next() Channel {
	for i, candidate := range Channels() {
		if c == candidate {
			return Channels()[(i+1)%4]
		}
	}
	return SpeedChannel
}
func (c Channel) label() string {
	switch c {
	case GripChannel:
		return "GRIP"
	case LateralChannel:
		return "LATERAL G"
	case LongitudinalChannel:
		return "LONG G"
	}
	return "SPEED"
}
func (c Channel) scale() (low, high float64, unit string) {
	switch c {
	case GripChannel:
		return 0, 1, "used / capacity"
	case LateralChannel:
		return -2, 2, "g · left +"
	case LongitudinalChannel:
		return -1.5, 1.5, "g · drive +"
	}
	return 0, 300, "km/h"
}
func (c Channel) value(n solver.Node) float64 {
	switch c {
	case GripChannel:
		return n.Forces.Utilization
	case LateralChannel:
		return n.Forces.Lateral / vehicle.Gravity
	case LongitudinalChannel:
		return n.Forces.Longitudinal / vehicle.Gravity
	}
	return n.Speed * 3.6
}
func (c Channel) color(n solver.Node) color.RGBA {
	if c != SpeedChannel && !n.Forces.Available {
		return muted
	}
	low, high, _ := c.scale()
	u := (c.value(n) - low) / (high - low)
	if c == GripChannel {
		u = 1 - u
	}
	return speedColor(u)
}

func (r *Renderer) LineColorMode() Channel { return r.opts.LineColor }
func (r *Renderer) ChartChannel() Channel  { return r.opts.ChartChannel }
func (r *Renderer) CycleLineColor()        { r.opts.LineColor = r.opts.LineColor.next(); r.drawBase() }
func (r *Renderer) CycleChartChannel() {
	r.opts.ChartChannel = r.opts.ChartChannel.next()
	r.drawBase()
}
func (r *Renderer) Markers() []solver.Marker { return append([]solver.Marker(nil), r.markers...) }

func (r *Renderer) instrumentationBase(im *image.RGBA) {
	fill(im, image.Rect(28, 572, 1090, 610), color.RGBA{19, 28, 37, 255})
	r.button(im, "line-color", image.Rect(39, 576, 234, 606), "COLOR: "+r.opts.LineColor.label(), false)
	low, high, unit := r.opts.LineColor.scale()
	r.text(im, 250, 588, fmt.Sprintf("%s  %g … %g %s", r.opts.LineColor.label(), low, high, unit), 11, ink, true)
	for x := 0; x < 150; x++ {
		u := float64(x) / 149
		if r.opts.LineColor == GripChannel {
			u = 1 - u
		}
		line(im, point{float64(250 + x), 596}, point{float64(250 + x), 600}, 1, speedColor(u))
	}
	r.text(im, 414, 601, "shared scale", 11, muted, false)
	r.text(im, 720, 595, "B brake · A speed apex · T tyre drive", 11, muted, false)
}

func markerStyle(kind string) (string, color.RGBA) {
	switch kind {
	case "brake":
		return "B", color.RGBA{255, 145, 113, 255}
	case "drive":
		return "T", accent
	default:
		return "A", color.RGBA{255, 211, 129, 255}
	}
}
func (r *Renderer) roadMarkers(im *image.RGBA) {
	for _, m := range r.markers {
		p := r.projected(m.Position)
		label, col := markerStyle(m.Kind)
		line(im, point{p.x - 4, p.y - 4}, point{p.x + 4, p.y + 4}, 2, col)
		line(im, point{p.x - 4, p.y + 4}, point{p.x + 4, p.y - 4}, 2, col)
		dx, dy := 12., -12.
		if m.Kind == "apex" {
			dy = 12
		}
		if m.Kind == "drive" {
			dx = -12
		}
		circle(im, point{p.x + dx, p.y + dy}, 8, bg)
		r.text(im, int(p.x+dx)-4, int(p.y+dy)+4, label, 11, col, true)
	}
}
func (r *Renderer) chartMarkers(im *image.RGBA) {
	rect := r.controls["chart"]
	for _, m := range r.markers {
		n, err := r.result.AtStation(m.Station)
		if err != nil {
			continue
		}
		p := r.chartPoint(n)
		label, col := markerStyle(m.Kind)
		y := rect.Min.Y + 18
		if m.Kind == "brake" {
			y += 12
		}
		if m.Kind == "drive" {
			y += 24
		}
		line(im, point{p.x, float64(rect.Min.Y)}, point{p.x, float64(rect.Min.Y + 6)}, 1, col)
		r.text(im, int(p.x)-3, y, label, 11, col, true)
	}
}

func (r *Renderer) forceScale(n, reference solver.Node) float64 {
	return math.Max(2*vehicle.Gravity, math.Ceil(math.Max(n.Forces.Capacity, reference.Forces.Capacity)/vehicle.Gravity)*vehicle.Gravity)
}
func forcePoint(n solver.Node, scale float64) point {
	return point{410 + n.Forces.Lateral/scale*31, 839 - n.Forces.Longitudinal/scale*31}
}

// ForceCursor returns the actual demand-dot display coordinates for headless
// interaction verification. It uses the same time, force query and projection
// as the widget; the reference is sampled at shared elapsed time.
func (r *Renderer) ForceCursor(t float64) [2]float64 {
	n, ref := r.result.At(t), r.referenceAt(t)
	p := forcePoint(n, r.forceScale(n, ref))
	return [2]float64{r.displayX + p.x*r.displayScale, r.displayY + p.y*r.displayScale}
}

func (r *Renderer) ChartCursor(t float64) [2]float64 {
	p := r.chartPoint(r.result.At(t))
	return [2]float64{r.displayX + p.x*r.displayScale, r.displayY + p.y*r.displayScale}
}

func (r *Renderer) forceWidget(im *image.RGBA, t float64, n solver.Node, state State) {
	r.text(im, 379, 801, "MODEL TYRE FORCES", 11, muted, true)
	if !n.Forces.Available {
		r.text(im, 379, 834, "Model does not expose force channels", 12, muted, false)
		return
	}
	ref := r.referenceAt(t)
	scale := r.forceScale(n, ref)
	line(im, point{377, 839}, point{443, 839}, 1, faint)
	line(im, point{410, 806}, point{410, 872}, 1, faint)
	shape := make([]point, 0, 130)
	for i := 0; i <= 64; i++ {
		lat := -n.Forces.Capacity + 2*n.Forces.Capacity*float64(i)/64
		drive, _ := n.Forces.LongitudinalBounds(lat)
		shape = append(shape, point{410 + lat/scale*31, 839 - drive/scale*31})
	}
	for i := 64; i >= 0; i-- {
		lat := -n.Forces.Capacity + 2*n.Forces.Capacity*float64(i)/64
		_, brake := n.Forces.LongitudinalBounds(lat)
		shape = append(shape, point{410 + lat/scale*31, 839 + brake/scale*31})
	}
	polygon(im, shape, color.RGBA{57, 83, 79, 160})
	for i, p := range shape {
		line(im, p, shape[(i+1)%len(shape)], 1, accent)
	}
	if state.Comparison && r.referenceCompatible() && ref.Forces.Available {
		p := forcePoint(ref, scale)
		for i := 0; i < 20; i++ {
			a, b := float64(i)*math.Pi/10, float64(i+1)*math.Pi/10
			line(im, point{p.x + 4*math.Cos(a), p.y + 4*math.Sin(a)}, point{p.x + 4*math.Cos(b), p.y + 4*math.Sin(b)}, 1, referenceColor)
		}
	}
	circle(im, forcePoint(n, scale), 3, ink)
	r.text(im, 452, 818, fmt.Sprintf("%s · %s", n.Forces.Phase, limitLabel(n.Forces.Limit)), 12, accent, true)
	r.text(im, 452, 837, fmt.Sprintf("LAT %+.2fg   LONG %+.2fg", n.Forces.Lateral/vehicle.Gravity, n.Forces.Longitudinal/vehicle.Gravity), 12, ink, false)
	r.text(im, 452, 855, fmt.Sprintf("Grip %.1f%% · axes ±%.0fg", 100*n.Forces.Utilization, scale/vehicle.Gravity), 11, muted, false)
	r.text(im, 452, 870, "Illustrative · quasi-static · outline: ref", 11, muted, false)
}
func limitLabel(s string) string {
	switch s {
	case "speed_cap":
		return "speed cap"
	case "power":
		return "power limit"
	case "grip":
		return "grip limit"
	case "brake":
		return "brake limit"
	case "none":
		return "profile constrained"
	default:
		return "unavailable"
	}
}
