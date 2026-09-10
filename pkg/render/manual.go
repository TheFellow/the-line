package render

import (
	"fmt"
	"image"
	"image/color"

	"github.com/TheFellow/the-line/pkg/track"
)

type LineHandle struct {
	Index    int        `json:"index"`
	Position track.Vec3 `json:"position"`
	Offset   float64    `json:"offset"`
	Station  float64    `json:"station"`
}

func (r *Renderer) studyButtons(im *image.RGBA) {
	r.button(im, "pin", image.Rect(395, 99, 514, 132), "PIN CURRENT", false)
	r.button(im, "unpin", image.Rect(521, 99, 650, 132), "CENTRELINE", false)
	label := "AUTHOR LINE"
	if r.opts.Manual {
		label = "GEOMETRY"
	}
	r.button(im, "manual", image.Rect(658, 99, 805, 132), label, r.opts.Manual)
}

func (r *Renderer) manualSidebar(im *image.RGBA) {
	x := r.opts.Width - 304
	r.text(im, x, 388, "AUTHOR A LINE", 13, accent, true)
	r.text(im, x, 414, "Drag diamond handles across the road.", 12, muted, false)
	r.text(im, x, 434, "Release to evaluate; Esc cancels.", 12, muted, false)
	r.text(im, x, 454, "Amber handles are uncommitted.", 12, muted, false)
	r.button(im, "manual-zero", image.Rect(x, 478, x+280, 512), "RESET LINE TO CENTRE", false)
	r.button(im, "manual-optimize", image.Rect(x, 524, x+280, 558), "OPTIMIZE FROM HERE", false)
	r.button(im, "manual-adopt", image.Rect(x, 570, x+280, 604), "ADOPT AS REFERENCE", false)
	r.button(im, "undo", image.Rect(x, 638, x+134, 672), "UNDO", false)
	r.button(im, "redo", image.Rect(x+145, 638, x+280, 672), "REDO", false)
}

func (r *Renderer) manualFrame(im *image.RGBA, state State) {
	for _, handle := range state.ManualHandles {
		p := r.projected(handle.Position)
		col := referenceColor
		if state.ManualDragging {
			col = color.RGBA{244, 184, 110, 255}
		}
		polygon(im, []point{{p.x, p.y - 9}, {p.x + 9, p.y}, {p.x, p.y + 9}, {p.x - 9, p.y}}, col)
		circle(im, p, 3, bg)
		r.text(im, int(p.x)+12, int(p.y)-9, fmt.Sprintf("L%d", handle.Index+1), 11, col, true)
	}
	if state.ManualFailure != nil {
		p := r.projected(*state.ManualFailure)
		circle(im, p, 17, color.RGBA{237, 108, 98, 120})
		circle(im, p, 6, color.RGBA{255, 128, 110, 255})
		r.text(im, int(p.x)+20, int(p.y), "REJECTED HERE", 11, color.RGBA{255, 128, 110, 255}, true)
	}
}

func (r *Renderer) referenceSummary(im *image.RGBA) {
	x := r.opts.Width - 304
	line(im, point{float64(x), 691}, point{float64(x + 280), 691}, 1, faint)
	r.text(im, x, 707, fmt.Sprintf("A · %s · %.2f s", truncate(r.vehicle.Name, 18), r.result.Duration), 12, accent, true)
	if !r.referenceCompatible() {
		r.text(im, x, 732, "B · "+truncate(r.referenceName(), 28), 12, referenceColor, true)
		r.text(im, x, 753, "STALE · DIFFERENT ROAD", 12, referenceColor, true)
		r.text(im, x, 776, "Pin current or select Centreline", 11, muted, false)
		return
	}
	name := r.referenceName()
	if r.opts.Reference == nil {
		name += " · " + r.vehicle.Name
	}
	r.text(im, x, 731, "B · "+truncate(name, 35), 11, referenceColor, true)
	r.text(im, x, 752, fmt.Sprintf("%.2f s · Δ %+.3f s", r.referenceDuration(), r.result.Duration-r.referenceDuration()), 16, ink, true)
	if r.result.Closed {
		r.text(im, x, 776, "Steady laps · ghosts wrap independently", 11, muted, false)
	} else {
		r.text(im, x, 776, "Speed caps; actual entry states can differ", 11, muted, false)
	}
}
