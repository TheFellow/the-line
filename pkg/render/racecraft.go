package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/TheFellow/the-line/pkg/racecraft"
	"github.com/TheFellow/the-line/pkg/track"
)

var raceColors = [2]color.RGBA{{178, 238, 135, 255}, {255, 181, 102, 255}}

func (r *Renderer) raceBase() {
	im := r.base
	fill(im, im.Bounds(), bg)
	r.text(im, 34, 44, "THE LINE", 26, ink, true)
	r.text(im, 192, 43, "/  RACECRAFT LAB", 12, muted, false)
	r.text(im, 35, 71, "Two cars. One road. Every position matters.", 12, muted, false)
	r.button(im, "race-mode", image.Rect(870, 25, 1085, 63), "QUALIFYING MODE", false)
	r.text(im, 42, 119, r.scene.Name+" / "+r.opts.Race.Config.Scenario, 22, ink, true)
	r.text(im, 43, 142, "A  GREEN     B  AMBER     ·     "+r.opts.View+" / shared race clock", 11, muted, false)
	r.drawRoad(im)
	roadImage := im.SubImage(r.viewport).(*image.RGBA)
	for i, car := range r.opts.Race.Cars {
		for j := 1; j < len(car.Path.Nodes); j++ {
			line(roadImage, r.projected(car.Path.Nodes[j-1].Position), r.projected(car.Path.Nodes[j].Position), 2, raceColors[i])
		}
	}
	fill(im, image.Rect(1112, 80, 1440, 784), panel)
	r.text(im, 1130, 110, "RACE EXPERIMENT", 16, ink, true)
	r.button(im, "race-scenario", image.Rect(1130, 129, 1418, 166), "NEXT: "+r.opts.Race.Config.Scenario, false)
	for i, car := range r.opts.Race.Cars {
		r.text(im, 1130, 194+i*25, car.Name+"  "+car.Intent, 12, raceColors[i], false)
	}
	c := r.opts.Race.Config
	for i, field := range []struct {
		key, label string
		value      float64
	}{{"gap", "Starting gap / m", c.Gap}, {"overspeed", "B entry advantage / m/s", c.Overspeed}, {"separation", "Lateral separation / m", c.Separation}, {"clearance", "Body clearance / m", c.Clearance}} {
		y := 254 + i*70
		r.text(im, 1130, y, field.label, 12, muted, false)
		r.text(im, 1247, y+30, fmt.Sprintf("%.1f", field.value), 18, ink, true)
		r.button(im, "race-"+field.key+"-", image.Rect(1130, y+8, 1180, y+42), "-", false)
		r.button(im, "race-"+field.key+"+", image.Rect(1368, y+8, 1418, y+42), "+", false)
	}
	r.text(im, 1130, 552, fmt.Sprintf("Body gap certified >= %.2f m", r.opts.Race.MinClearance), 12, accent, false)
	r.text(im, 1130, 577, "Complete pass = 4.4 m ahead", 12, muted, false)
	r.text(im, 1130, 601, "Stops at first finish; replay together", 11, muted, false)
	r.text(im, 1130, 625, "Entry speeds are caps; grip can", 11, muted, false)
	r.text(im, 1130, 642, "reduce the requested advantage.", 11, muted, false)
	r.text(im, 1130, 672, "Offline tactical estimate", 12, muted, false)
	r.button(im, "view", image.Rect(1130, 695, 1268, 732), "2D / 3D", false)
	r.button(im, "fit", image.Rect(1280, 695, 1418, 732), "FIT", false)
	r.button(im, "race-save", image.Rect(1130, 743, 1268, 777), "SAVE RACE", false)
	r.button(im, "race-load", image.Rect(1280, 743, 1418, 777), "LOAD RACE", false)
	r.raceCharts(im)
	fill(im, image.Rect(0, 784, 1440, 900), panel)
	r.controls["scrub"] = image.Rect(40, 770, 1085, 790)
	line(im, point{40, 780}, point{1085, 780}, 3, faint)
	r.button(im, "play", image.Rect(1130, 805, 1240, 843), "", true)
	r.button(im, "restart", image.Rect(1252, 805, 1418, 843), "RESTART", false)
	r.button(im, "rate", image.Rect(1130, 851, 1418, 884), "", false)
}

func (r *Renderer) raceCharts(im *image.RGBA) {
	r.text(im, 42, 592, "SPEED / SHARED TIME   ·   km/h", 11, muted, true)
	r.text(im, 42, 697, "POSITION GAP   ·   A ahead + / B ahead -", 11, muted, true)
	for _, y := range []float64{607, 637, 667, 715, 755} {
		line(im, point{82, y}, point{1078, y}, 1, faint)
	}
	line(im, point{82, 735}, point{1078, 735}, 1, muted)
	speedMax, gapMax := 200., 20.
	for _, car := range r.opts.Race.Cars {
		for _, n := range car.Path.Nodes {
			speedMax = math.Max(speedMax, math.Ceil(n.Speed*3.6/50)*50)
		}
	}
	for i := 0; i <= 400; i++ {
		n := r.opts.Race.At(r.opts.Race.Duration * float64(i) / 400)
		gapMax = math.Max(gapMax, math.Ceil(math.Abs(n[0].Station-n[1].Station)/10)*10)
	}
	r.text(im, 44, 610, fmt.Sprintf("%.0f", speedMax), 11, muted, false)
	r.text(im, 44, 640, fmt.Sprintf("%.0f", speedMax/2), 11, muted, false)
	r.text(im, 57, 670, "0", 11, muted, false)
	r.text(im, 44, 718, fmt.Sprintf("+%.0f", gapMax), 11, muted, false)
	r.text(im, 57, 738, "0", 11, muted, false)
	r.text(im, 44, 758, fmt.Sprintf("-%.0f", gapMax), 11, muted, false)
	race := r.opts.Race
	for i := 1; i <= 400; i++ {
		ta, tb := race.Duration*float64(i-1)/400, race.Duration*float64(i)/400
		a, b := race.At(ta), race.At(tb)
		x1, x2 := 82+996*ta/race.Duration, 82+996*tb/race.Duration
		for car := 0; car < 2; car++ {
			line(im, point{x1, 667 - a[car].Speed*3.6/speedMax*60}, point{x2, 667 - b[car].Speed*3.6/speedMax*60}, 1.7, raceColors[car])
		}
		gapA, gapB := a[0].Station-a[1].Station, b[0].Station-b[1].Station
		line(im, point{x1, 735 - gapA/gapMax*20}, point{x2, 735 - gapB/gapMax*20}, 1.8, ink)
	}
	for _, e := range race.Events {
		x := 82 + 996*e.Time/race.Duration
		line(im, point{x, 607}, point{x, 756}, 1, raceColors[e.Leader])
		r.text(im, int(x)-65, 684, fmt.Sprintf("%s %s %.1fs", race.Cars[e.Leader].Name, e.Kind, e.Time), 11, raceColors[e.Leader], false)
	}
	r.text(im, 82, 768, "0 s", 11, muted, false)
	r.text(im, 1025, 768, fmt.Sprintf("%.1f s", race.Duration), 11, muted, false)
}

func (r *Renderer) raceFrame(t float64, state State) image.Image {
	copy(r.frame.Pix, r.base.Pix)
	im := r.frame
	race := r.opts.Race
	t = math.Max(0, math.Min(t, race.Duration))
	if math.IsNaN(t) {
		t = 0
	}
	nodes := race.At(t)
	before, after := race.At(t-.01), race.At(t+.01)
	for i, n := range nodes {
		delta := after[i].Position.Sub(before[i].Position)
		d := math.Hypot(delta.X, delta.Y)
		if d < 1e-9 {
			d = 1
			delta.X = 1
		}
		dx, dy := delta.X/d, delta.Y/d
		// Project physical body vertices, never enlarge the car in screen pixels.
		body := func(length, width float64) []point {
			out := []point{}
			for _, q := range []point{{length / 2, width / 2}, {length / 2, -width / 2}, {-length / 2, -width / 2}, {-length / 2, width / 2}} {
				out = append(out, r.projected(track.Vec3{X: n.Position.X + dx*q.x - dy*q.y, Y: n.Position.Y + dy*q.x + dx*q.y, Z: n.Position.Z}))
			}
			return out
		}
		road := im.SubImage(r.viewport).(*image.RGBA)
		polygon(road, body(racecraft.BodyLength, race.Vehicle.Width), raceColors[i])
		polygon(road, body(1.9, race.Vehicle.Width*.75), bg)
		p := r.projected(n.Position)
		r.text(road, int(p.x)+9, int(p.y)-9, race.Cars[i].Name, 13, raceColors[i], true)
		r.text(im, 42+i*250, 824, fmt.Sprintf("%s  %5.1f km/h", race.Cars[i].Name, n.Speed*3.6), 22, raceColors[i], true)
		r.text(im, 42+i*250, 850, fmt.Sprintf("station %.1f m", n.Station), 12, muted, false)
	}
	gap := nodes[0].Station - nodes[1].Station
	lead := "A"
	if gap < 0 {
		lead = "B"
	}
	r.text(im, 570, 824, fmt.Sprintf("%s nose ahead  %.1f m", lead, math.Abs(gap)), 22, ink, true)
	r.text(im, 570, 850, fmt.Sprintf("%.2f / %.2f s", t, race.Duration), 14, muted, false)
	x := 82 + 996*t/race.Duration
	line(im, point{x, 603}, point{x, 757}, 1, accent)
	x = 40 + 1045*t/race.Duration
	line(im, point{40, 780}, point{x, 780}, 3, accent)
	circle(im, point{x, 780}, 5, ink)
	label := "PAUSE"
	if !state.Playing {
		label = "PLAY"
	}
	r.text(im, 1150, 830, label, 12, accent, true)
	r.text(im, 1150, 875, fmt.Sprintf("PLAYBACK  %.2gx", state.Rate), 12, ink, false)
	status := state.Status
	if status == "" {
		path := state.FilePath
		if path == "" {
			path = "racecraft.json"
		}
		status = "SPACE play / pause    TAB view    , / . step    R qualifying    Save/load: " + path
	}
	r.text(im, 40, 893, truncate(status, 135), 12, muted, false)
	return r.scaledFrame(im)
}
