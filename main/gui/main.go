// The GUI runs the shared software renderer inside Ebitengine, keeping all
// geometry, editing and solving independent of the display server.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheFellow/the-line/internal/editor"
	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type options struct {
	preset, scene, vehicle, view, file, capture, report string
	frames, width, height                               int
	demo                                                bool
}

type solved struct {
	result solver.Result
	config vehicle.Config
	err    error
}

type game struct {
	opts         options
	ed           *editor.Editor
	solvedScene  track.Scene
	renderer     *render.Renderer
	result       solver.Result
	config       vehicle.Config
	texture      *ebiten.Image
	replies      chan solved
	busy         bool
	rollback     func() bool
	playing      bool
	clock        float64
	status       string
	statusUntil  time.Time
	frame, draws int
	captured     bool
	captureErr   error
	started      time.Time
	dragging     bool
	dragIndex    int
	dragX, dragY int
	dragPosition track.Vec3
	fileEditing  bool
	fileDraft    string
	demoIndex    int
	demoLog      []string
	nextDemo     int
}

func main() {
	var o options
	flag.StringVar(&o.preset, "preset", "esses", "starting corner sequence (hairpin, esses, compound, banked, rally)")
	flag.StringVar(&o.scene, "scene", "", "load a scene JSON instead of a preset")
	flag.StringVar(&o.vehicle, "vehicle", "", "override the scene vehicle preset")
	flag.StringVar(&o.view, "view", "3d", "initial view: 2d or 3d")
	flag.StringVar(&o.file, "file", "scene.json", "editor save/load path; click the path to edit it")
	flag.StringVar(&o.capture, "capture", "", "capture actual final Ebitengine Draw as PNG")
	flag.StringVar(&o.report, "report", "", "write JSON verification report on exit")
	flag.IntVar(&o.frames, "frames", 0, "exit after at least this many live frames (0 runs until closed)")
	flag.IntVar(&o.width, "width", 1440, "logical window width")
	flag.IntVar(&o.height, "height", 900, "logical window height")
	flag.BoolVar(&o.demo, "demo", false, "exercise select/edit/undo/redo/save/load/view actions before capture")
	flag.Parse()
	if o.frames < 0 {
		log.Fatal("frames must be nonnegative")
	}
	s, err := track.Preset(o.preset)
	if o.scene != "" {
		s, err = track.Load(o.scene)
		if o.file == "scene.json" {
			o.file = o.scene
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	if o.vehicle != "" {
		s.Vehicle = o.vehicle
	}
	v, err := vehicle.Preset(s.Vehicle)
	if err != nil {
		log.Fatal(err)
	}
	ed, err := editor.New(s)
	if err != nil {
		log.Fatal(err)
	}
	result, err := solver.Solve(s, v, solver.DefaultOptions())
	if err != nil {
		log.Fatal(err)
	}
	r, err := render.New(s, result, v, render.Options{Width: o.width, Height: o.height, View: o.view})
	if err != nil {
		log.Fatal(err)
	}
	g := &game{opts: o, ed: ed, solvedScene: s, renderer: r, result: result, config: v, playing: true, replies: make(chan solved, 1), started: time.Now(), nextDemo: 20}
	g.texture = ebiten.NewImage(o.width, o.height)
	ebiten.SetWindowSize(o.width, o.height)
	ebiten.SetWindowTitle("The Line · Racing geometry studio")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	// Finite runs also progress when the automation window loses focus.
	ebiten.SetRunnableOnUnfocused(true)
	runErr := ebiten.RunGame(g)
	if errors.Is(runErr, ebiten.Termination) {
		runErr = nil
	}
	if runErr != nil {
		g.recordError(runErr)
	}
	if o.demo && g.demoIndex < len(demoActions) {
		g.recordError(fmt.Errorf("demo ended after %d of %d actions", g.demoIndex, len(demoActions)))
	}
	if err := g.writeReport(); err != nil {
		log.Fatal(err)
	}
	if runErr != nil {
		log.Fatal(runErr)
	}
	if g.captureErr != nil {
		log.Fatal(g.captureErr)
	}
	for _, action := range g.demoLog {
		if strings.HasPrefix(action, "ERROR:") {
			log.Fatal("demo verification failed: " + action)
		}
	}
	fmt.Printf("Rendered %d frames, %d updates in %.2fs; Ebitengine %.1f FPS / %.1f TPS; run %.3fs\n", g.draws, g.frame, time.Since(g.started).Seconds(), ebiten.ActualFPS(), ebiten.ActualTPS(), g.result.Duration)
}

func (g *game) Layout(_, _ int) (int, int) { return g.opts.width, g.opts.height }

func (g *game) setStatus(s string) { g.status = s; g.statusUntil = time.Now().Add(7 * time.Second) }

// recordError keeps interactive failures visible and makes automated failures
// durable in the report. Ordinary rejected edits do not fail a later normal exit.
func (g *game) recordError(err error) {
	g.setStatus(err.Error())
	if g.opts.demo {
		g.demoLog = append(g.demoLog, "ERROR: "+err.Error())
	}
}

func (g *game) rebuild() error {
	r, err := render.New(g.solvedScene, g.result, g.config, render.Options{Width: g.opts.width, Height: g.opts.height, View: g.opts.view})
	if err != nil {
		return err
	}
	g.renderer = r
	return nil
}

func (g *game) queueSolve(rollback func() bool) {
	g.busy = true
	g.rollback = rollback
	g.setStatus("Computing a feasible line… playback continues")
	s := g.ed.Scene()
	go func() {
		v, err := vehicle.Preset(s.Vehicle)
		if err != nil {
			g.replies <- solved{err: err}
			return
		}
		r, err := solver.Solve(s, v, solver.DefaultOptions())
		g.replies <- solved{result: r, config: v, err: err}
	}()
}

func (g *game) Update() error {
	if g.captured {
		return ebiten.Termination
	}
	g.frame++
	select {
	case reply := <-g.replies:
		g.busy = false
		if reply.err != nil {
			if g.rollback != nil {
				g.rollback()
			}
			g.recordError(fmt.Errorf("edit reverted: %w", reply.err))
		} else {
			fraction := g.clock / math.Max(g.result.Duration, 1)
			g.result, g.config = reply.result, reply.config
			g.solvedScene = g.ed.Scene()
			g.clock = fraction * g.result.Duration
			if err := g.rebuild(); err != nil {
				return err
			}
			g.setStatus(fmt.Sprintf("Solved · %.2f s · %.2f s faster than centreline", g.result.Duration, g.result.CenterDuration-g.result.Duration))
		}
	default:
	}
	if g.playing {
		g.clock += 1.0 / 60
		if g.clock > g.result.Duration {
			g.clock = math.Mod(g.clock, g.result.Duration)
		}
	}
	if g.fileEditing {
		g.editPath()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.action("play")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.action("view")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyHome) {
		g.action("restart")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketLeft) {
		g.action("previous")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketRight) {
		g.action("next")
	}
	modifier := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
	if modifier {
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.action("save")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyO) {
			g.action("load")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyN) {
			g.action("new")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				g.action("redo")
			} else {
				g.action("undo")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			g.action("redo")
		}
	} else {
		if inpututil.IsKeyJustPressed(ebiten.KeyN) {
			g.action("add")
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDelete) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			g.action("delete")
		}
		for _, entry := range []struct {
			key    ebiten.Key
			action string
		}{{ebiten.KeyArrowLeft, "move-left"}, {ebiten.KeyArrowRight, "move-right"}, {ebiten.KeyArrowUp, "move-up"}, {ebiten.KeyArrowDown, "move-down"}} {
			if inpututil.IsKeyJustPressed(entry.key) {
				g.action(entry.action)
			}
		}
	}
	g.mouse()
	if g.opts.demo && !g.busy && g.frame >= g.nextDemo {
		g.runDemo()
	}
	return nil
}

func (g *game) mouse() {
	x, y := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for key, rect := range g.renderer.Controls() {
			if image.Pt(x, y).In(rect) {
				if key == "scrub" {
					g.scrub(x)
				} else {
					g.action(key)
				}
				return
			}
		}
		if !g.busy {
			s := g.ed.Scene()
			best, distance := -1, 18.0
			for i, p := range s.Points {
				px, py := g.renderer.Project(p.Position())
				d := math.Hypot(px-float64(x), py-float64(y))
				if d < distance {
					best, distance = i, d
				}
			}
			if best >= 0 {
				_ = g.ed.Select(best)
				g.dragging = true
				g.dragIndex = best
				g.dragX, g.dragY = x, y
				g.dragPosition = s.Points[best].Position()
			}
		}
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && !g.dragging {
		if rect := g.renderer.Controls()["scrub"]; image.Pt(x, y).In(rect) {
			g.scrub(x)
		}
	}
	if g.dragging && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		g.dragging = false
		if math.Hypot(float64(x-g.dragX), float64(y-g.dragY)) > 2 {
			p := g.renderer.Unproject(float64(x), float64(y), g.dragPosition.Z)
			if err := g.ed.MovePoint(g.dragIndex, p); err != nil {
				g.recordError(err)
			} else {
				g.queueSolve(g.ed.Undo)
			}
		}
	}
}

func (g *game) scrub(x int) {
	rect := g.renderer.Controls()["scrub"]
	g.clock = math.Max(0, math.Min(1, float64(x-rect.Min.X)/float64(rect.Dx()))) * g.result.Duration
	g.playing = false
}

// action is shared by real click/key events and deterministic verification.
func (g *game) action(key string) {
	s := g.ed.Scene()
	i := g.ed.Selected()
	switch key {
	case "play":
		g.playing = !g.playing
		return
	case "scrub-mid":
		rect := g.renderer.Controls()["scrub"]
		g.scrub((rect.Min.X + rect.Max.X) / 2)
		return
	case "restart":
		g.clock = 0
		return
	case "view":
		if g.opts.view == "3d" {
			g.opts.view = "2d"
		} else {
			g.opts.view = "3d"
		}
		if err := g.rebuild(); err != nil {
			g.recordError(err)
		}
		return
	case "previous":
		_ = g.ed.Select((i + len(s.Points) - 1) % len(s.Points))
		return
	case "next":
		_ = g.ed.Select((i + 1) % len(s.Points))
		return
	case "path":
		g.fileEditing = true
		g.fileDraft = g.opts.file
		return
	}
	if g.busy {
		g.setStatus("Finish computing this edit before changing the road again")
		return
	}
	var err error
	rollback := g.ed.Undo
	p := s.Points[i]
	switch key {
	case "save":
		err = g.ed.Save(g.opts.file)
		if err != nil {
			g.recordError(err)
		} else {
			g.setStatus("Saved " + g.opts.file)
		}
		return
	case "load":
		err = g.ed.Load(g.opts.file)
	case "new":
		err = g.ed.Replace(track.Scene{Version: track.Version, Name: "Untitled sequence", Vehicle: s.Vehicle, EntrySpeed: 30, ExitSpeed: 40, Points: []track.Point{{X: -90, Width: 12, Surface: "asphalt"}, {Width: 12, Surface: "asphalt"}, {X: 60, Y: 40, Width: 12, Surface: "asphalt"}, {X: 100, Y: 90, Width: 12, Surface: "asphalt"}}})
	case "preset":
		names := track.Presets()
		next := 0
		for j, name := range names {
			candidate, _ := track.Preset(name)
			if candidate.Name == s.Name {
				next = (j + 1) % len(names)
			}
		}
		var scene track.Scene
		scene, err = track.Preset(names[next])
		if err == nil {
			err = g.ed.Replace(scene)
		}
	case "vehicle":
		names := vehicle.Presets()
		next := 0
		for j, name := range names {
			if name == s.Vehicle {
				next = (j + 1) % len(names)
			}
		}
		s.Vehicle = names[next]
		err = g.ed.Replace(s)
	case "undo":
		if !g.ed.Undo() {
			g.setStatus("Nothing to undo")
			return
		}
		rollback = g.ed.Redo
	case "redo":
		if !g.ed.Redo() {
			g.setStatus("Nothing to redo")
			return
		}
	case "add":
		if i+1 < len(s.Points) {
			q := s.Points[i+1]
			p.X = (p.X + q.X) / 2
			p.Y = (p.Y + q.Y) / 2
			p.Z = (p.Z + q.Z) / 2
			p.Width = (p.Width + q.Width) / 2
			p.Bank = (p.Bank + q.Bank) / 2
		} else {
			q := s.Points[i-1]
			p.X += .5 * (p.X - q.X)
			p.Y += .5 * (p.Y - q.Y)
			p.Z += .5 * (p.Z - q.Z)
		}
		err = g.ed.InsertPoint(i, p)
	case "delete":
		err = g.ed.DeletePoint(i)
	case "width-", "width+":
		if key == "width-" {
			p.Width -= .5
		} else {
			p.Width += .5
		}
		err = g.ed.UpdatePoint(i, p)
	case "bank-", "bank+":
		if key == "bank-" {
			p.Bank -= 1
		} else {
			p.Bank += 1
		}
		err = g.ed.UpdatePoint(i, p)
	case "height-", "height+":
		if key == "height-" {
			p.Z -= .5
		} else {
			p.Z += .5
		}
		err = g.ed.UpdatePoint(i, p)
	case "surface-", "surface+":
		surfaces := track.Surfaces()
		for j, v := range surfaces {
			if v.Name == p.Surface {
				step := 1
				if key == "surface-" {
					step = -1
				}
				p.Surface = surfaces[(j+len(surfaces)+step)%len(surfaces)].Name
				break
			}
		}
		err = g.ed.UpdatePoint(i, p)
	case "move-left", "move-right", "move-up", "move-down":
		switch key {
		case "move-left":
			p.X -= 1
		case "move-right":
			p.X += 1
		case "move-up":
			p.Y += 1
		case "move-down":
			p.Y -= 1
		}
		err = g.ed.MovePoint(i, p.Position())
	default:
		return
	}
	if err != nil {
		g.recordError(err)
		return
	}
	g.queueSolve(rollback)
}

func (g *game) editPath() {
	if (ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)) && inpututil.IsKeyJustPressed(ebiten.KeyA) {
		g.fileDraft = ""
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 32 {
			g.fileDraft += string(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		rr := []rune(g.fileDraft)
		if len(rr) > 0 {
			g.fileDraft = string(rr[:len(rr)-1])
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.fileEditing = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && strings.TrimSpace(g.fileDraft) != "" {
		g.opts.file = g.fileDraft
		g.fileEditing = false
		g.setStatus("Save / load path: " + g.opts.file)
	}
}

var demoActions = []string{"next", "next", "width+", "bank-", "height+", "move-up", "add", "undo", "redo", "delete", "surface+", "surface-", "save", "new", "vehicle", "preset", "load", "view", "view", "scrub-mid", "play"}

func (g *game) runDemo() {
	if g.demoIndex >= len(demoActions) {
		return
	}
	a := demoActions[g.demoIndex]
	g.action(a)
	g.demoLog = append(g.demoLog, a)
	g.demoIndex++
	g.nextDemo = g.frame + 5
}

func (g *game) Draw(screen *ebiten.Image) {
	status := g.status
	if time.Now().After(g.statusUntil) && !g.busy {
		status = ""
	}
	if g.dragging {
		status = "Release to move this control point and recompute the line"
	}
	path := g.opts.file
	if g.fileEditing {
		path = g.fileDraft + "|"
		status = "Editing save / load path · ENTER confirm · ESC cancel"
	}
	frame := g.renderer.FrameWithState(g.clock, render.State{Selected: g.ed.Selected(), Playing: g.playing, Status: status, FPS: ebiten.ActualFPS(), FilePath: path})
	g.texture.WritePixels(frame.(*image.RGBA).Pix)
	screen.DrawImage(g.texture, nil)
	if g.dragging {
		x, y := ebiten.CursorPosition() // A visible ghost makes the release destination unambiguous.
		for dx := -9; dx <= 9; dx++ {
			screen.Set(x+dx, y, image.White.At(0, 0))
		}
		for dy := -9; dy <= 9; dy++ {
			screen.Set(x, y+dy, image.White.At(0, 0))
		}
	}
	g.draws++
	done := g.opts.frames > 0 && g.draws >= g.opts.frames && !g.busy && (!g.opts.demo || g.demoIndex == len(demoActions))
	if done && !g.captured {
		if g.opts.capture != "" {
			capture := image.NewRGBA(screen.Bounds())
			screen.ReadPixels(capture.Pix)
			g.captureErr = savePNG(g.opts.capture, capture)
			if g.captureErr != nil {
				g.recordError(fmt.Errorf("capture: %w", g.captureErr))
			}
		}
		g.captured = true
	}
}

func savePNG(path string, im image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, im); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (g *game) writeReport() error {
	if g.opts.report == "" {
		return nil
	}
	report := struct {
		Frames      int         `json:"draw_frames"`
		Updates     int         `json:"updates"`
		WallSeconds float64     `json:"wall_seconds"`
		FPS         float64     `json:"fps"`
		TPS         float64     `json:"tps"`
		Time        float64     `json:"animation_time"`
		Duration    float64     `json:"estimated_duration"`
		View        string      `json:"view"`
		Actions     []string    `json:"actions"`
		Scene       track.Scene `json:"scene"`
	}{g.draws, g.frame, time.Since(g.started).Seconds(), ebiten.ActualFPS(), ebiten.ActualTPS(), g.clock, g.result.Duration, g.opts.view, g.demoLog, g.ed.Scene()}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(g.opts.report), 0755); err != nil {
		return err
	}
	return os.WriteFile(g.opts.report, append(b, '\n'), 0644)
}
