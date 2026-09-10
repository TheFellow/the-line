package render

import (
	"fmt"
	"image"

	"github.com/TheFellow/the-line/pkg/track"
)

func (r *Renderer) authoringSidebar(im *image.RGBA) {
	x := r.opts.Width - 304
	r.button(im, "authoring", image.Rect(x, 372, x+174, 402), "← GEOMETRY", true)
	r.button(im, "previous", image.Rect(x+187, 372, x+229, 402), "‹", false)
	r.button(im, "next", image.Rect(x+239, 372, x+280, 402), "›", false)
	for i, key := range []string{"left", "right", "kerb-left", "kerb-right", "grip-left", "grip-right"} {
		y := 435 + i*29
		r.button(im, "road:"+key+"-", image.Rect(x+194, y-18, x+232, y+7), "−", false)
		r.button(im, "road:"+key+"+", image.Rect(x+242, y-18, x+280, y+7), "+", false)
	}
	label := "KERBS EXCLUDED"
	if r.scene.KerbsCountAsRoad {
		label = "KERBS COUNT AS ROAD"
	}
	r.button(im, "road:limits", image.Rect(x, 592, x+280, 622), label, r.scene.KerbsCountAsRoad)
	r.button(im, "import-csv", image.Rect(x, 628, x+280, 655), "IMPORT CENTRELINE CSV", false)
	r.button(im, "undo", image.Rect(x, 660, x+134, 687), "UNDO", false)
	r.button(im, "redo", image.Rect(x+145, 660, x+280, 687), "REDO", false)
}
func (r *Renderer) authoringSelection(im *image.RGBA, idx int, p track.Point) {
	x := r.opts.Width - 304
	r.text(im, x, 416, fmt.Sprintf("Point %02d · horizontal metres", idx+1), 11, muted, false)
	values := []string{
		fmt.Sprintf("Left width  %.1f m", p.LeftWidth()),
		fmt.Sprintf("Right width  %.1f m", p.RightWidth()),
		fmt.Sprintf("Left kerb  %.1f m", p.KerbLeft.Width),
		fmt.Sprintf("Right kerb  %.1f m", p.KerbRight.Width),
		fmt.Sprintf("Left kerb grip  %.2f", p.KerbLeft.Friction()),
		fmt.Sprintf("Right kerb grip  %.2f", p.KerbRight.Friction()),
	}
	for i, label := range values {
		r.text(im, x, 439+i*29, label, 12, ink, false)
	}
}
