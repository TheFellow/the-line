// Package render draws the same display-independent racing studio for exports and
// the live editor. Geometry is cached; Frame only paints the moving overlays.
package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"github.com/TheFellow/the-line/pkg/racecraft"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Options struct {
	Race          *racecraft.Result
	Analysis      bool
	Authoring     bool
	Width, Height int
	// View is "2d" (plan) or "3d" (an elevated orthographic projection).
	View                    string
	LineColor, ChartChannel Channel
	// Camera preserves an explicit framing; nil fits the road automatically.
	Camera    *Camera
	SetupPage int
	Setup     bool
	Reference *Reference
	Manual    bool
}

// State contains transient editor state; none of it changes the solved path.
type State struct {
	ManualHandles  []LineHandle
	ManualDragging bool
	ManualFailure  *track.Vec3
	Selected       int
	Playing        bool
	Status         string
	FPS            float64
	FilePath       string
	Hover          *int
	Drag           *DragPreview
	Comparison     bool
	Rate           float64
	SetupConfig    *vehicle.Config
	Sensitivity    []solver.Sensitivity
	Provisional    bool
}

// DragPreview is an uncommitted control position shown over the last solved road.
type DragPreview struct {
	Index    int
	Position track.Vec3
}

type point struct{ x, y float64 }

// Renderer owns a cached static scene. It is not safe for concurrent calls.
type Renderer struct {
	speedChartScale                  [2]float64
	perspective                      *perspectiveScene
	markers                          []solver.Marker
	scene                            track.Scene
	result                           solver.Result
	vehicle                          vehicle.Config
	opts                             Options
	base                             *image.RGBA
	frame                            *image.RGBA
	output                           *image.RGBA
	displayScale, displayX, displayY float64
	faces                            map[int]font.Face
	bold                             map[int]font.Face
	controls                         map[string]image.Rectangle
	viewport                         image.Rectangle
	camera                           Camera
	scale, ox, oy                    float64
}

var (
	bg     = color.RGBA{15, 21, 29, 255}
	panel  = color.RGBA{21, 29, 39, 255}
	ink    = color.RGBA{230, 236, 239, 255}
	muted  = color.RGBA{133, 151, 164, 255}
	faint  = color.RGBA{53, 69, 79, 255}
	accent = color.RGBA{178, 238, 135, 255}
)

// New prepares road geometry, typography, labels and editor controls once.
func New(scene track.Scene, result solver.Result, config vehicle.Config, opts Options) (*Renderer, error) {
	if opts.Width == 0 {
		opts.Width = 1440
	}
	if opts.Height == 0 {
		opts.Height = 900
	}
	if opts.View == "" {
		opts.View = "3d"
	}
	if opts.LineColor == "" {
		opts.LineColor = SpeedChannel
	}
	if opts.ChartChannel == "" {
		opts.ChartChannel = SpeedChannel
	}
	if !opts.LineColor.Valid() || !opts.ChartChannel.Valid() {
		return nil, fmt.Errorf("unknown line color or chart channel")
	}
	if opts.Width < 640 || opts.Height < 400 || opts.Width > 8192 || opts.Height > 8192 {
		return nil, fmt.Errorf("render size must be between 640x400 and 8192x8192")
	}
	if opts.View != "2d" && opts.View != "3d" && opts.View != "perspective" {
		return nil, fmt.Errorf("view must be 2d, 3d or perspective")
	}
	if opts.Race != nil && (opts.View == "perspective" || opts.Race.Duration <= 0) {
		return nil, fmt.Errorf("racecraft requires a valid experiment and a 2d or 3d view")
	}
	if len(result.Road) < 2 || len(result.Nodes) < 2 {
		return nil, fmt.Errorf("render requires a solved road")
	}
	outputWidth, outputHeight := opts.Width, opts.Height
	opts.Width, opts.Height = 1440, 900
	r := &Renderer{scene: scene, result: result, vehicle: config, opts: opts, faces: make(map[int]font.Face), bold: make(map[int]font.Face), controls: make(map[string]image.Rectangle)}
	r.output = image.NewRGBA(image.Rect(0, 0, outputWidth, outputHeight))
	r.displayScale = math.Min(float64(outputWidth)/1440, float64(outputHeight)/900)
	r.displayX = (float64(outputWidth) - 1440*r.displayScale) / 2
	r.displayY = (float64(outputHeight) - 900*r.displayScale) / 2
	r.base = image.NewRGBA(image.Rect(0, 0, opts.Width, opts.Height))
	r.frame = image.NewRGBA(r.base.Bounds())
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, err
	}
	for _, size := range []int{11, 12, 13, 14, 16, 18, 22, 26, 32, 40} {
		r.faces[size], err = opentype.NewFace(regular, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return nil, err
		}
		r.bold[size], err = opentype.NewFace(bold, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return nil, err
		}
	}
	r.viewport = image.Rect(28, 150, opts.Width-356, 565)
	if opts.Camera == nil {
		r.fit()
	} else if err := r.setCamera(*opts.Camera); err != nil {
		return nil, err
	}
	r.speedChartScale = speedChartBounds(result.Nodes, r.referenceNodes())
	r.markers = solver.DetectMarkers(result.Nodes)
	r.drawBase()
	return r, nil
}

// Controls returns a copy of the named click targets, in display pixels.
func (r *Renderer) Controls() map[string]image.Rectangle {
	out := make(map[string]image.Rectangle, len(r.controls))
	for k, v := range r.controls {
		out[k] = r.displayRect(v)
	}
	return out
}

func (r *Renderer) projected(p track.Vec3) point {
	if r.opts.View == "perspective" {
		return r.perspectivePoint(p)
	}
	q := r.raw(p)
	return point{q.x*r.scale + r.ox, q.y*r.scale + r.oy}
}

func (r *Renderer) displayRect(rect image.Rectangle) image.Rectangle {
	return image.Rect(int(float64(rect.Min.X)*r.displayScale+r.displayX), int(float64(rect.Min.Y)*r.displayScale+r.displayY), int(float64(rect.Max.X)*r.displayScale+r.displayX), int(float64(rect.Max.Y)*r.displayScale+r.displayY))
}

// Frame draws a playing scene with the first control point selected.
// The returned image is reused on the next call; copy it when retaining frames.
func (r *Renderer) Frame(t float64) image.Image {
	return r.FrameWithState(t, State{Selected: 0, Playing: true, Comparison: true, Rate: 1})
}

// FrameWithState overlays the vehicle, selection, playhead and live telemetry.
func (r *Renderer) FrameWithState(t float64, state State) image.Image {
	if r.opts.Race != nil {
		return r.raceFrame(t, state)
	}
	copy(r.frame.Pix, r.base.Pix)
	im := r.frame
	t = r.frameTime(t, state)
	n := r.frameNode(t, state)
	playheadTime := t
	if r.result.Closed {
		playheadTime = n.Time
	}
	roadImage := im.SubImage(r.viewport).(*image.RGBA)
	selected := state.Selected
	if r.opts.View != "perspective" && !r.opts.Analysis && !r.opts.Manual && selected >= 0 && selected < len(r.scene.Points) {
		p := r.scene.Points[selected]
		q := r.projected(track.Vec3{X: p.X, Y: p.Y, Z: p.Z})
		circle(roadImage, q, 11, color.RGBA{178, 238, 135, 35})
		circle(roadImage, q, 6, accent)
		circle(roadImage, q, 3, bg)
		if !r.opts.Setup {
			if r.opts.Authoring && !r.opts.Manual {
				r.authoringSelection(im, selected, p)
			} else {
				r.selection(im, selected, p)
			}
		}
	}
	if r.opts.View == "perspective" {
		r.perspectiveFrame(roadImage, t, n, state)
	} else {
		if state.Comparison && len(r.referenceNodes()) > 1 {
			r.ghost(roadImage, t)
		}
		r.car(roadImage, t, n)
		r.manualFrame(roadImage, state)
		if state.Hover != nil && *state.Hover >= 0 && *state.Hover < len(r.scene.Points) {
			q := r.projected(r.scene.Points[*state.Hover].Position())
			circle(roadImage, q, 12, accent)
			circle(roadImage, q, 9, bg)
			circle(roadImage, q, 4, ink)
		}
		if state.Drag != nil {
			r.drawDrag(roadImage, *state.Drag)
		}
	}
	if r.opts.Setup && !r.opts.Analysis {
		r.setupFrame(im, state)
	}
	path := state.FilePath
	if path == "" {
		path = "scene.json"
	}
	r.text(im, r.opts.Width-296, 225, truncate(path, 40), 11, muted, false)
	y := r.opts.Height - 93
	r.text(im, 40, y, "VELOCITY", 11, muted, false)
	r.text(im, 40, y+38, fmt.Sprintf("%03.0f", n.Speed*3.6), 32, ink, true)
	r.text(im, 111, y+36, "km/h", 13, muted, false)
	r.text(im, 186, y, "ELAPSED", 11, muted, false)
	r.text(im, 186, y+34, fmt.Sprintf("%05.2f", playheadTime), 26, ink, false)
	r.text(im, 268, y+32, "/ "+fmt.Sprintf("%.2f s", r.PlaybackDuration(state.Comparison)), 13, muted, false)
	r.forceWidget(im, t, n, state)
	play := r.controls["play"]
	label := "PAUSE"
	if !state.Playing {
		label = "PLAY"
	}
	r.text(im, play.Min.X+17, play.Min.Y+23, label, 12, accent, true)
	if state.FPS > 0 {
		r.text(im, r.opts.Width-172, r.opts.Height-6, fmt.Sprintf("LIVE  %.0f FPS", state.FPS), 11, muted, false)
	}
	scrub := r.controls["scrub"]
	x := float64(scrub.Min.X) + float64(scrub.Dx())*playheadTime/r.PlaybackDuration(state.Comparison)
	line(im, point{float64(scrub.Min.X), float64(scrub.Min.Y + 9)}, point{x, float64(scrub.Min.Y + 9)}, 3, accent)
	circle(im, point{x, float64(scrub.Min.Y + 9)}, 5, ink)
	r.comparisonFrame(im, t, n, state)
	r.presentationFrame(im, n)
	if state.Status != "" {
		r.text(im, 40, r.opts.Height-6, truncate(state.Status, 92), 12, color.RGBA{237, 189, 127, 255}, false)
	} else {
		hint := "SPACE  play / pause     TAB  change view     Drag a numbered handle; release to apply"
		if r.opts.View == "perspective" {
			hint = "SPACE play / pause    , / . frame step    SHIFT + , / . station step    TAB return to edit"
		}
		r.text(im, 40, r.opts.Height-6, hint, 12, muted, false)
	}
	return r.scaledFrame(im)
}

func (r *Renderer) scaledFrame(im *image.RGBA) image.Image {
	if r.output.Bounds() == im.Bounds() {
		return im
	}
	fill(r.output, r.output.Bounds(), bg)
	xdraw.ApproxBiLinear.Scale(r.output, r.displayRect(im.Bounds()), im, im.Bounds(), draw.Src, nil)
	return r.output
}

func (r *Renderer) drawBase() {
	if r.opts.Race != nil {
		r.raceBase()
		return
	}
	im := r.base
	fill(im, im.Bounds(), bg)
	w, h := r.opts.Width, r.opts.Height
	fill(im, image.Rect(w-328, 80, w, h-116), panel)
	line(im, point{float64(w - 328), 80}, point{float64(w - 328), float64(h - 116)}, 1, faint)
	// Fine survey grid gives the world a quiet sense of scale.
	for x := r.viewport.Min.X + 12; x < r.viewport.Max.X; x += 32 {
		for y := r.viewport.Min.Y + 10; y < r.viewport.Max.Y; y += 32 {
			circle(im, point{float64(x), float64(y)}, .7, color.RGBA{48, 65, 71, 150})
		}
	}
	r.text(im, 34, 44, "THE LINE", 26, ink, true)
	r.text(im, 192, 43, "/  RACING GEOMETRY STUDIO", 12, muted, false)
	r.text(im, 35, 71, "Explore the corner. Find the flow.", 12, muted, false)
	r.text(im, w-312, 40, "QUALIFYING / TIME SEARCH", 11, accent, true)
	topology := "open sequence"
	if r.result.Closed {
		topology = "continuous lap"
	}
	r.text(im, w-312, 62, "3D road · friction envelope · "+topology, 11, muted, false)
	line(im, point{28, 83}, point{float64(w - 28), 83}, 1, faint)
	r.text(im, 42, 119, r.scene.Name, 22, ink, true)
	view := "ELEVATED / 3D"
	if r.opts.View == "perspective" {
		view = "TRACKSIDE / PERSPECTIVE"
	}
	if r.opts.View == "2d" {
		view = "PLAN / 2D"
	}
	r.text(im, 43, 142, view+"    ·    "+fmt.Sprintf("line %.0f m", r.result.Length), 11, muted, false)
	r.drawRoad(im)
	r.instrumentationBase(im)
	r.comparisonBase(im)
	r.sidebar(im)
	r.studyButtons(im)
	fill(im, image.Rect(0, h-116, w, h), color.RGBA{18, 25, 34, 255})
	line(im, point{0, float64(h - 116)}, point{float64(w), float64(h - 116)}, 1, faint)
	r.controls["scrub"] = image.Rect(40, h-129, w-355, h-111)
	line(im, point{40, float64(h - 120)}, point{float64(w - 355), float64(h - 120)}, 3, faint)
	r.button(im, "play", image.Rect(w-290, h-89, w-198, h-51), "", true)
	r.button(im, "restart", image.Rect(w-187, h-89, w-42, h-51), "RESTART", false)
	r.button(im, "comparison", image.Rect(736, h-87, 897, h-53), "", false)
	r.button(im, "rate", image.Rect(908, h-87, 1039, h-53), "", false)
	r.presentationBase(im)
	r.button(im, "race-mode", image.Rect(870, 25, 1085, 63), "RACECRAFT MODE", false)
}

func (r *Renderer) drawRoad(im *image.RGBA) {
	if r.opts.View == "perspective" {
		r.perspectiveRoad(im)
		return
	}
	im = im.SubImage(r.viewport).(*image.RGBA)
	road := r.result.Road
	if r.opts.View == "3d" {
		// Ground footprints and elevation ties make height legible without a
		// perspective camera or any distortion of the authoritative road mesh.
		for i := 1; i < len(road); i++ {
			a, b := road[i-1], road[i]
			var footprint []point
			for _, p := range []track.Vec3{a.AtOffset(-a.RightWidth() - a.KerbRight.Width - 2), a.AtOffset(a.LeftWidth() + a.KerbLeft.Width + 2), b.AtOffset(b.LeftWidth() + b.KerbLeft.Width + 2), b.AtOffset(-b.RightWidth() - b.KerbRight.Width - 2)} {
				p.Z = 0
				footprint = append(footprint, r.projected(p))
			}
			polygon(im, footprint, color.RGBA{25, 38, 40, 255})
		}
		for i, p := range r.scene.Points {
			if i%3 != 2 || math.Abs(p.Z) < 2 {
				continue
			}
			a := r.projected(p.Position())
			ground := p.Position()
			ground.Z = 0
			b := r.projected(ground)
			d := math.Hypot(a.x-b.x, a.y-b.y)
			for t := 0.0; t < d; t += 7 {
				u, v := t/d, math.Min(1, (t+3)/d)
				line(im, point{a.x + (b.x-a.x)*u, a.y + (b.y-a.y)*u}, point{a.x + (b.x-a.x)*v, a.y + (b.y-a.y)*v}, 1, muted)
			}
			line(im, point{b.x - 4, b.y}, point{b.x + 4, b.y}, 1, muted)
			r.text(im, int(b.x)+8, int(b.y)+5, fmt.Sprintf("%+.1f m", p.Z), 11, muted, false)
		}
	}
	// Low, offset shadow separates raised road from the terrain.
	for i := 1; i < len(road); i++ {
		a, b := road[i-1], road[i]
		p := []point{r.projected(a.AtOffset(-a.RightWidth() - a.KerbRight.Width - 1.4)), r.projected(a.AtOffset(a.LeftWidth() + a.KerbLeft.Width + 1.4)), r.projected(b.AtOffset(b.LeftWidth() + b.KerbLeft.Width + 1.4)), r.projected(b.AtOffset(-b.RightWidth() - b.KerbRight.Width - 1.4))}
		for j := range p {
			p[j].y += 8
		}
		polygon(im, p, color.RGBA{6, 11, 16, 200})
	}
	order := make([]int, len(road)-1)
	for i := range order {
		order[i] = i + 1
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		return r.depth(road[a-1].Position.Add(road[a].Position)) < r.depth(road[b-1].Position.Add(road[b].Position))
	})
	for _, i := range order {
		a, b := road[i-1], road[i]
		al, ar := r.projected(a.AtOffset(a.LeftLimit())), r.projected(a.AtOffset(-a.RightLimit()))
		bl, br := r.projected(b.AtOffset(b.LeftLimit())), r.projected(b.AtOffset(-b.RightLimit()))
		col := color.RGBA{65, 75, 80, 255}
		switch a.Surface {
		case "gravel":
			col = color.RGBA{109, 97, 77, 255}
		case "dirt":
			col = color.RGBA{103, 79, 62, 255}
		case "wet", "wet-asphalt":
			col = color.RGBA{55, 73, 86, 255}
		case "ice":
			col = color.RGBA{110, 146, 158, 255}
		}
		polygon(im, []point{al, ar, bl}, col)
		polygon(im, []point{ar, br, bl}, col)
		r.drawKerbs(im, a, b)
		line(im, al, bl, 1.1, color.RGBA{238, 235, 213, 210})
		line(im, ar, br, 1.1, color.RGBA{238, 235, 213, 210})
		if int(a.S/5)%2 == 0 {
			line(im, r.projected(a.Position), r.projected(b.Position), 1, color.RGBA{196, 203, 196, 115})
		}
	}
	for i := 1; r.opts.Race == nil && i < len(r.result.Nodes); i++ {
		a, b := r.result.Nodes[i-1], r.result.Nodes[i]
		line(im, r.projected(a.Position), r.projected(b.Position), 8, color.RGBA{52, 235, 182, 27})
	}
	for i := 1; r.opts.Race == nil && i < len(r.result.Nodes); i++ {
		a, b := r.result.Nodes[i-1], r.result.Nodes[i]
		line(im, r.projected(a.Position), r.projected(b.Position), 2.8, r.opts.LineColor.color(b))
	}
	if r.opts.Race == nil {
		r.roadMarkers(im)
	}
	for _, idx := range []int{0, len(road) - 1} {
		s := road[idx]
		for j := 0; j < 10; j++ {
			a := -s.RightLimit() + float64(j)*(s.LeftLimit()+s.RightLimit())/10
			b := a + (s.LeftLimit()+s.RightLimit())/10
			col := ink
			if j%2 == 1 {
				col = bg
			}
			line(im, r.projected(s.AtOffset(a)), r.projected(s.AtOffset(b)), 4, col)
		}
	}
	if !r.opts.Manual && r.opts.Race == nil {
		for i, p := range r.scene.Points {
			q := r.projected(track.Vec3{X: p.X, Y: p.Y, Z: p.Z})
			circle(im, q, 7, bg)
			circle(im, q, 5, ink)
			circle(im, q, 3, panel)
			r.text(im, int(q.x)+11, int(q.y)-10, fmt.Sprintf("%02d", i+1), 12, ink, true)
		}
	}
	start, end := r.projected(road[0].Position), r.projected(road[len(road)-1].Position)
	if r.result.Closed {
		r.text(im, int(start.x)+13, int(start.y)+25, "START / FINISH", 11, accent, true)
	} else {
		r.text(im, int(start.x)+13, int(start.y)+25, "IN", 11, accent, true)
		r.text(im, int(end.x)+13, int(end.y)+25, "OUT", 11, accent, true)
	}
}

func (r *Renderer) drawDrag(im *image.RGBA, preview DragPreview) {
	if preview.Index < 0 || preview.Index >= len(r.scene.Points) {
		return
	}
	scene := r.scene
	scene.Points = append([]track.Point(nil), scene.Points...)
	p := &scene.Points[preview.Index]
	p.X, p.Y, p.Z = preview.Position.X, preview.Position.Y, preview.Position.Z
	col := color.RGBA{255, 196, 104, 255}
	road, err := track.SampleRoad(scene, 4)
	if err != nil {
		col = color.RGBA{255, 125, 115, 255}
	} else {
		for i := 1; i < len(road); i++ {
			for _, side := range []float64{-1, 1} {
				a, b := road[i-1], road[i]
				line(im, r.projected(a.AtOffset(a.EdgeOffset(side))), r.projected(b.AtOffset(b.EdgeOffset(side))), 2, col)
			}
		}
	}
	q := r.projected(preview.Position)
	origin := r.projected(r.scene.Points[preview.Index].Position())
	line(im, origin, q, 2, col)
	for _, i := range []int{preview.Index - 1, preview.Index + 1} {
		if i >= 0 && i < len(scene.Points) {
			line(im, q, r.projected(scene.Points[i].Position()), 1, col)
		}
	}
	circle(im, q, 11, col)
	circle(im, q, 8, bg)
	circle(im, q, 4, col)
	label := fmt.Sprintf("MOVE %02d · release to apply", preview.Index+1)
	if err != nil {
		label = "Invalid shape · Esc to cancel"
	}
	r.text(im, int(q.x)+16, int(q.y)-17, label, 13, col, true)
}

func (r *Renderer) sidebar(im *image.RGBA) {
	x := r.opts.Width - 304
	r.text(im, x, 115, "SEQUENCE", 11, muted, true)
	r.button(im, "preset", image.Rect(x, 130, x+155, 165), "NEXT SCENE →", false)
	label := "OPEN ROAD"
	if r.scene.Closed {
		label = "CLOSED LAP"
	}
	r.button(im, "closed", image.Rect(x+163, 130, x+280, 165), label, r.scene.Closed)
	r.button(im, "new", image.Rect(x, 175, x+85, 209), "NEW", false)
	r.button(im, "load", image.Rect(x+96, 175, x+181, 209), "LOAD", false)
	r.button(im, "save", image.Rect(x+192, 175, x+280, 209), "SAVE", false)
	r.controls["path"] = image.Rect(x, 211, x+280, 233)
	line(im, point{float64(x), 235}, point{float64(x + 280), 235}, 1, faint)
	r.button(im, "setup", image.Rect(x+175, 240, x+280, 266), "SETUP →", false)
	if r.opts.Setup {
		r.setupSidebar(im)
	} else {
		r.text(im, x, 251, "ILLUSTRATIVE CAR", 11, muted, true)
		r.text(im, x, 276, truncate(r.vehicle.Name, 28), 18, ink, true)
		drive := "AWD"
		if r.vehicle.FrontDrive == 0 {
			drive = "RWD"
		}
		if r.vehicle.FrontDrive == 1 {
			drive = "FWD"
		}
		r.text(im, x, 298, fmt.Sprintf("%.0f kg · %.0f kW · %.0f kW/t", r.vehicle.Mass, r.vehicle.Power/1000, r.vehicle.Power/r.vehicle.Mass), 12, muted, false)
		r.button(im, "vehicle", image.Rect(x, 312, x+280, 346), "CHANGE VEHICLE  →", false)
		r.text(im, x, 359, fmt.Sprintf("%s · tyre grip ×%.2f · quasi-static", drive, r.vehicle.Grip), 11, muted, false)
		line(im, point{float64(x), 363}, point{float64(x + 280), 363}, 1, faint)
		if r.opts.Manual {
			r.manualSidebar(im)
		} else if r.opts.Authoring {
			r.authoringSidebar(im)
		} else {
			r.button(im, "authoring", image.Rect(x, 372, x+174, 402), "ROAD LIMITS →", false)
			r.button(im, "previous", image.Rect(x+187, 372, x+229, 402), "‹", false)
			r.button(im, "next", image.Rect(x+239, 372, x+280, 402), "›", false)
			for i, key := range []string{"width", "bank", "height", "surface"} {
				y := 441 + i*37
				r.button(im, key+"-", image.Rect(x+191, y-21, x+229, y+9), "−", false)
				r.button(im, key+"+", image.Rect(x+239, y-21, x+280, y+9), "+", false)
			}
			r.button(im, "add", image.Rect(x, 593, x+134, 627), "ADD POINT", false)
			r.button(im, "delete", image.Rect(x+145, 593, x+280, 627), "DELETE", false)
			r.button(im, "undo", image.Rect(x, 638, x+134, 672), "UNDO", false)
			r.button(im, "redo", image.Rect(x+145, 638, x+280, 672), "REDO", false)
		}
		r.referenceSummary(im)
	}
	r.button(im, "fit", image.Rect(r.opts.Width-620, 99, r.opts.Width-500, 132), "FIT / RESET", false)
	r.button(im, "view", image.Rect(r.opts.Width-490, 99, r.opts.Width-356, 132), "SWITCH 2D / 3D", false)
	hint := "Right / Alt-drag orbit · Shift-drag pan · Wheel zoom"
	if r.opts.View == "2d" {
		hint = "Shift-drag pan · Wheel zoom · Tab for elevated orbit"
	}
	if r.opts.View == "perspective" {
		hint = "Watch only · frame / station controls below · Tab to edit"
	}
	r.text(im, 570, 145, hint, 11, muted, false)
}

func (r *Renderer) selection(im *image.RGBA, idx int, p track.Point) {
	x := r.opts.Width - 304
	r.text(im, x, 414, fmt.Sprintf("Point %02d of %02d", idx+1, len(r.scene.Points)), 13, accent, false)
	values := []string{fmt.Sprintf("Width   %.1f m", p.LeftWidth()+p.RightWidth()), fmt.Sprintf("Bank    %+.1f°", p.Bank), fmt.Sprintf("Height  %+.1f m", p.Z), "Surface  " + p.Surface}
	for i, v := range values {
		r.text(im, x, 445+i*37, truncate(v, 25), 13, ink, false)
	}
	r.text(im, x, 582, "Bank + raises the left edge", 11, muted, false)
}

func (r *Renderer) car(im *image.RGBA, t float64, n solver.Node) {
	carTime := t
	if !r.result.Closed {
		carTime = math.Min(t, r.result.Duration)
	}
	a, b := r.currentAt(carTime-.08), r.currentAt(carTime+.08)
	r.drawVehicle(im, n, a, b, false)
}

func (r *Renderer) drawVehicle(im *image.RGBA, n, a, b solver.Node, ghost bool) {
	p := r.projected(n.Position)
	pa, pb := r.projected(a.Position), r.projected(b.Position)
	dx, dy := pb.x-pa.x, pb.y-pa.y
	d := math.Hypot(dx, dy)
	if d < 1e-8 {
		dx, dy, d = 1, 0, 1
	}
	dx /= d
	dy /= d
	length := math.Max(13, math.Min(24, 4.5*r.scale))
	width := length * .44
	shape := func(long, wide, shift float64) []point {
		var out []point
		for _, v := range []point{{long / 2, wide / 2}, {long / 2, -wide / 2}, {-long / 2, -wide / 2}, {-long / 2, wide / 2}} {
			out = append(out, point{p.x + dx*v.x - dy*v.y, p.y + dy*v.x + dx*v.y + shift})
		}
		return out
	}
	if ghost {
		body := shape(length+4, width+4, 0)
		for i := range body {
			line(im, body[i], body[(i+1)%len(body)], 2, referenceColor)
		}
		circle(im, p, length*.85, color.RGBA{115, 193, 225, 22})
		return
	}
	circle(im, p, length*.95, color.RGBA{205, 249, 163, 23})
	polygon(im, shape(length+3, width+3, 3), color.RGBA{0, 0, 0, 170})
	polygon(im, shape(length, width, 0), ink)
	polygon(im, shape(length*.43, width*.77, 0), color.RGBA{35, 57, 63, 255})
	nose := point{p.x + dx*length*.39, p.y + dy*length*.39}
	line(im, point{nose.x - dy*width*.43, nose.y + dx*width*.43}, point{nose.x + dy*width*.43, nose.y - dx*width*.43}, 2, accent)
}

func (r *Renderer) button(im *image.RGBA, key string, rect image.Rectangle, label string, active bool) {
	r.controls[key] = rect
	col := color.RGBA{31, 43, 53, 255}
	if active {
		col = color.RGBA{43, 64, 49, 255}
	}
	fill(im, rect, col)
	line(im, point{float64(rect.Min.X), float64(rect.Max.Y - 1)}, point{float64(rect.Max.X), float64(rect.Max.Y - 1)}, 1, faint)
	if label != "" {
		r.text(im, rect.Min.X+12, rect.Min.Y+rect.Dy()/2+5, label, 12, ink, false)
	}
}

func (r *Renderer) text(im *image.RGBA, x, y int, s string, size int, c color.RGBA, bold bool) {
	f := r.faces[size]
	if bold {
		f = r.bold[size]
	}
	d := font.Drawer{Dst: im, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

func truncate(s string, n int) string {
	rr := []rune(s)
	if len(rr) > n {
		return string(rr[:n-1]) + "…"
	}
	return s
}

func speedColor(t float64) color.RGBA {
	t = math.Max(0, math.Min(1, t))
	stops := []color.RGBA{{255, 115, 96, 255}, {255, 192, 101, 255}, {190, 236, 127, 255}, {78, 222, 186, 255}, {91, 190, 242, 255}}
	x := t * float64(len(stops)-1)
	i := int(x)
	if i == len(stops)-1 {
		return stops[i]
	}
	u := x - float64(i)
	a, b := stops[i], stops[i+1]
	return color.RGBA{uint8(float64(a.R)*(1-u) + float64(b.R)*u), uint8(float64(a.G)*(1-u) + float64(b.G)*u), uint8(float64(a.B)*(1-u) + float64(b.B)*u), 255}
}

func fill(im *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(im, r, image.NewUniform(c), image.Point{}, draw.Src)
}

func blend(im *image.RGBA, x, y int, c color.RGBA, coverage float64) {
	if !image.Pt(x, y).In(im.Bounds()) {
		return
	}
	a := float64(c.A) / 255 * math.Max(0, math.Min(1, coverage))
	if a <= 0 {
		return
	}
	i := im.PixOffset(x, y)
	inv := 1 - a
	im.Pix[i] = uint8(float64(c.R)*a + float64(im.Pix[i])*inv)
	im.Pix[i+1] = uint8(float64(c.G)*a + float64(im.Pix[i+1])*inv)
	im.Pix[i+2] = uint8(float64(c.B)*a + float64(im.Pix[i+2])*inv)
	im.Pix[i+3] = 255
}

func circle(im *image.RGBA, p point, r float64, c color.RGBA) {
	for y := int(math.Floor(p.y - r - 1)); y <= int(math.Ceil(p.y+r+1)); y++ {
		for x := int(math.Floor(p.x - r - 1)); x <= int(math.Ceil(p.x+r+1)); x++ {
			d := math.Hypot(float64(x)+.5-p.x, float64(y)+.5-p.y)
			blend(im, x, y, c, r+.5-d)
		}
	}
}

func line(im *image.RGBA, a, b point, width float64, c color.RGBA) {
	dx, dy := b.x-a.x, b.y-a.y
	den := dx*dx + dy*dy
	rad := width / 2
	x0, x1 := int(math.Floor(math.Min(a.x, b.x)-rad-1)), int(math.Ceil(math.Max(a.x, b.x)+rad+1))
	y0, y1 := int(math.Floor(math.Min(a.y, b.y)-rad-1)), int(math.Ceil(math.Max(a.y, b.y)+rad+1))
	x0 = max(x0, im.Bounds().Min.X)
	x1 = min(x1, im.Bounds().Max.X-1)
	y0 = max(y0, im.Bounds().Min.Y)
	y1 = min(y1, im.Bounds().Max.Y-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			px, py := float64(x)+.5, float64(y)+.5
			t := 0.0
			if den > 0 {
				t = math.Max(0, math.Min(1, ((px-a.x)*dx+(py-a.y)*dy)/den))
			}
			dist := math.Hypot(px-a.x-t*dx, py-a.y-t*dy)
			blend(im, x, y, c, rad+.5-dist)
		}
	}
}

func polygon(im *image.RGBA, pts []point, c color.RGBA) {
	if len(pts) < 3 {
		return
	}
	miny, maxy := pts[0].y, pts[0].y
	for _, p := range pts {
		miny = math.Min(miny, p.y)
		maxy = math.Max(maxy, p.y)
	}
	for y := max(im.Bounds().Min.Y, int(math.Floor(miny))); y <= min(im.Bounds().Max.Y-1, int(math.Ceil(maxy))); y++ {
		scan := float64(y) + .5
		var xs []float64
		for i, a := range pts {
			b := pts[(i+1)%len(pts)]
			if (a.y <= scan && b.y > scan) || (b.y <= scan && a.y > scan) {
				xs = append(xs, a.x+(scan-a.y)*(b.x-a.x)/(b.y-a.y))
			}
		}
		sort.Float64s(xs)
		for j := 0; j+1 < len(xs); j += 2 {
			for x := max(im.Bounds().Min.X, int(math.Ceil(xs[j]-.5))); x <= min(im.Bounds().Max.X-1, int(math.Floor(xs[j+1]-.5))); x++ {
				blend(im, x, y, c, 1)
			}
		}
	}
}
