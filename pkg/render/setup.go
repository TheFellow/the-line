package render

import (
	"fmt"
	"github.com/TheFellow/the-line/pkg/vehicle"
	"image"
)

func (r *Renderer) setupSidebar(im *image.RGBA) {
	x := r.opts.Width - 304
	r.text(im, x, 253, "SETUP WORKBENCH", 11, accent, true)
	r.button(im, "setup", image.Rect(x+175, 240, x+280, 266), "← GEOMETRY", false)
	for i, field := range vehicle.SetupFields() {
		y := 291 + i*31
		r.button(im, "setup:"+field.Key+"-", image.Rect(x+194, y-19, x+232, y+7), "−", false)
		r.button(im, "setup:"+field.Key+"+", image.Rect(x+242, y-19, x+280, y+7), "+", false)
	}
	r.text(im, x, 573, "Axle fractions: static traction only", 11, muted, false)
	r.text(im, x, 589, "Illustrative · quasi-static model", 11, muted, false)
	r.button(im, "save-car", image.Rect(x, 600, x+86, 630), "SAVE CAR", false)
	r.button(im, "load-car", image.Rect(x+94, 600, x+180, 630), "LOAD CAR", false)
	r.button(im, "reset-car", image.Rect(x+188, 600, x+280, 630), "RESET", false)
	r.button(im, "undo", image.Rect(x, 638, x+86, 668), "UNDO", false)
	r.button(im, "redo", image.Rect(x+94, 638, x+180, 668), "REDO", false)
	r.button(im, "sensitivity", image.Rect(x+188, 638, x+280, 668), "ANALYZE", false)
	r.text(im, x, 687, "TIME SENSITIVITY · ON THIS LINE", 11, muted, true)
	r.text(im, x, 779, "Car file: scene path + .car.json", 11, muted, false)
}

func (r *Renderer) setupFrame(im *image.RGBA, state State) {
	x := r.opts.Width - 304
	car := r.vehicle
	if state.SetupConfig != nil {
		car = *state.SetupConfig
	}
	for i, field := range vehicle.SetupFields() {
		value, _ := car.Value(field.Key)
		label := fmt.Sprintf("%s  %.2f %s", field.Label, value*field.DisplayScale, field.Unit)
		if field.Key == "mass" || field.Key == "power" || field.Key == "max_speed" {
			label = fmt.Sprintf("%s  %.0f %s", field.Label, value*field.DisplayScale, field.Unit)
		}
		r.text(im, x, 292+i*31, label, 12, ink, false)
	}
	if len(state.Sensitivity) == 0 {
		r.text(im, x, 710, "Analyze when ready; fixed path only", 11, muted, false)
	}
	for i, row := range state.Sensitivity {
		label := map[string]string{"mass": "+50 kg", "power": "+25 kW", "grip": "+0.05 grip", "drag_area": "+0.1 m² drag"}[row.Parameter]
		r.text(im, x, 706+i*16, fmt.Sprintf("%-13s  %+.3f s", label, row.Delta), 11, ink, false)
	}
	if state.Provisional {
		r.text(im, 44, 167, "PROVISIONAL · CURRENT LINE / NEW SETUP", 12, accent, true)
	}
}
