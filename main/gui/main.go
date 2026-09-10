// The GUI runs the shared software renderer inside Ebitengine, keeping all
// geometry, editing and solving independent of the display server.
package main

import (
	"context"
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
	demoDrag                                            bool
}

type solved struct {
	result      solver.Result
	config      vehicle.Config
	err         error
	generation  int
	provisional bool
	sensitivity []solver.Sensitivity
	analysis    bool
}

type game struct {
	authoringOpen        bool
	importRequests       chan csvImport
	reference            *render.Reference
	referenceKey         string
	manualMode           bool
	manualDrag           *editor.LineDrag
	manualFailure        *track.Vec3
	opts                 options
	ed                   *editor.Editor
	solvedScene          track.Scene
	renderer             *render.Renderer
	result               solver.Result
	config               vehicle.Config
	texture              *ebiten.Image
	replies              chan solved
	busy                 bool
	setupOpen            bool
	setupPage            int
	polishNext           int
	customCar            *vehicle.Config
	provisional          bool
	cancelSolve          context.CancelFunc
	generation           int
	sensitivity          []solver.Sensitivity
	analyzing            bool
	oldResult            solver.Result
	oldConfig            vehicle.Config
	oldScene             track.Scene
	solveRequests        int
	rollback             func()
	playing              bool
	clock                float64
	status               string
	statusUntil          time.Time
	frame, draws         int
	captured             bool
	captureErr           error
	started              time.Time
	cameraDrag           *cameraGesture
	resetCamera          bool
	drag                 *editor.Drag
	scrubbing            bool
	charting             bool
	comparison           bool
	rate                 float64
	pointerCancelled     bool
	pointerX, pointerY   float64
	hover                int
	fileEditing          bool
	fileDraft            string
	demoIndex            int
	demoLog              []string
	nextDemo             int
	demoDragStep         int
	demoDragX, demoDragY float64
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
	flag.BoolVar(&o.demoDrag, "demo-drag", false, "verify pointer dragging only (use with --frames and --report)")
	flag.Parse()
	if o.demoDrag {
		o.demo = true
		demoActions = []string{"next", "next", "pointer-drag"}
	}
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
		s.VehicleConfig = nil
	}
	v, err := vehicle.Preset(s.Vehicle)
	if err != nil {
		log.Fatal(err)
	}
	ed, err := editor.New(s)
	if err != nil {
		log.Fatal(err)
	}
	v = ed.Vehicle()

	result, err := solver.Solve(s, v, solver.DefaultOptions())
	if err != nil {
		log.Fatal(err)
	}
	r, err := render.New(s, result, v, render.Options{Width: o.width, Height: o.height, View: o.view})
	if err != nil {
		log.Fatal(err)
	}
	g := &game{opts: o, ed: ed, solvedScene: s, renderer: r, result: result, config: v, playing: true, comparison: true, rate: 1, replies: make(chan solved, 1), started: time.Now(), nextDemo: 20}
	if err := g.syncReference(); err != nil {
		log.Fatal(err)
	}
	if s.Study != nil && s.Study.Manual != nil && s.Study.Manual.RoadDigest == track.RoadDigest(s) {
		evalOpts := solver.DefaultOptions()
		evalOpts.Spacing = s.Study.Manual.Spacing
		g.result, err = solver.Evaluate(s, v, s.Study.Manual.Offsets, evalOpts)
		if err != nil {
			log.Fatal(err)
		}
		g.manualMode = true
	}
	if err := g.rebuild(); err != nil {
		log.Fatal(err)
	}
	g.texture = ebiten.NewImage(o.width, o.height)
	attachInspection(g)
	ebiten.SetWindowSize(o.width, o.height)
	ebiten.SetWindowTitle("The Line · Racing geometry studio")
	if o.demo {
		ebiten.SetWindowTitle("The Line verification · closes automatically")
	}
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	// Finite runs also progress when the automation window loses focus.
	ebiten.SetRunnableOnUnfocused(true)
	runErr := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{InitUnfocused: o.demo})
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
	if err := g.syncReference(); err != nil {
		return err
	}
	opts := render.Options{Width: g.opts.width, Height: g.opts.height, View: g.opts.view, Setup: g.setupOpen, Authoring: g.authoringOpen, SetupPage: g.setupPage, Manual: g.manualMode, Reference: g.reference}
	opts.LineColor, opts.ChartChannel = g.renderer.LineColorMode(), g.renderer.ChartChannel()
	opts.Analysis = g.renderer.Analysis()
	if !g.resetCamera || g.busy {
		camera := g.renderer.Camera()
		opts.Camera = &camera
	}
	r, err := render.New(g.solvedScene, g.result, g.config, opts)
	if err != nil {
		return err
	}
	g.renderer = r
	if !g.busy {
		g.resetCamera = false
	}
	return nil
}

func (g *game) queueSolve(rollback func()) { g.startSolve(rollback, false) }

func (g *game) Update() error {
	g.updateImport()
	defer inspectFrame(g)
	if g.captured {
		return ebiten.Termination
	}
	g.frame++
	if err := g.acceptReplies(); err != nil {
		return err
	}

	if g.playing {
		g.clock += g.rate / 60
		if !g.result.Closed && g.clock > g.playbackDuration() {
			g.clock = math.Mod(g.clock, g.playbackDuration())
		}
	}
	if g.opts.demo {
		if !g.busy && g.frame >= g.nextDemo {
			g.runDemo()
		}
		return nil
	}
	if pointerInterrupted() || !ebiten.IsFocused() {
		g.cancelPointer()
		return nil
	}
	if g.fileEditing {
		g.editPath()
		return nil
	}
	g.presentationKeys()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.drag != nil || g.manualDrag != nil || g.scrubbing || g.charting || g.cameraDrag != nil {
			g.cancelPointer()
			return nil
		}
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
			if inpututil.IsKeyJustPressed(entry.key) && g.opts.view != "perspective" {
				g.action(entry.action)
			}
		}
	}
	g.mouse()
	return nil
}

func (g *game) mouse() {
	x, y := ebiten.CursorPosition()
	g.pointerX, g.pointerY = float64(x), float64(y)
	if g.cameraMouse(float64(x), float64(y)) {
		ebiten.SetCursorShape(ebiten.CursorShapeMove)
		return
	}
	g.pointer(float64(x), float64(y), inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft), ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft))
	shape := ebiten.CursorShapeDefault
	if g.hover >= 0 {
		shape = ebiten.CursorShapePointer
	}
	if g.drag != nil {
		shape = ebiten.CursorShapeMove
	}
	ebiten.SetCursorShape(shape)
}

// cancelPointer discards an incomplete gesture and waits for a fresh press.
func (g *game) cancelPointer() {
	if g.drag != nil || g.manualDrag != nil || g.scrubbing || g.charting || g.cameraDrag != nil {
		g.setStatus("Gesture cancelled")
	}
	g.drag = nil
	g.manualDrag = nil
	g.cameraDrag = nil
	g.scrubbing = false
	g.charting = false
	g.hover = -1
	g.pointerCancelled = true
}

// pointer is shared by real mouse events and the automated drag scenario.
func (g *game) pointer(x, y float64, pressed, held, released bool) {
	g.pointerX, g.pointerY = x, y
	g.hover = -1
	if g.pointerCancelled {
		if !held || pressed {
			g.pointerCancelled = false
		}
		if !pressed {
			return
		}
	}
	if x < 0 || y < 0 || x >= float64(g.opts.width) || y >= float64(g.opts.height) {
		if g.drag != nil || g.manualDrag != nil || g.scrubbing || g.charting || g.cameraDrag != nil {
			g.cancelPointer()
		}
		return
	}
	if g.scrubbing {
		g.scrub(int(x))
		if released || !held {
			g.scrubbing = false
		}
		return
	}
	if g.charting {
		g.seekStation(x)
		if released || !held {
			g.charting = false
		}
		return
	}
	for key, rect := range g.renderer.Controls() {
		if image.Pt(int(x), int(y)).In(rect) && g.drag == nil && g.manualDrag == nil {
			if pressed {
				if key == "scrub" {
					g.scrubbing = true
					g.scrub(int(x))
				} else if key == "chart" {
					g.charting = true
					g.seekStation(x)
				} else {
					g.action(key)
				}
			}
			return
		}
	}
	if g.opts.view == "perspective" {
		return
	}
	if g.manualPointer(x, y, pressed, held, released) {
		return
	}
	if g.drag == nil && !image.Pt(int(x), int(y)).In(g.renderer.RoadViewport()) {
		return
	}
	if !g.busy {
		g.hover = editor.Pick(g.ed.Scene(), g.renderer, x, y)
	}
	if pressed {
		if g.busy {
			g.setStatus("Computing the last edit — drag again when the line is ready")
			return
		}
		g.drag = editor.BeginDrag(g.ed.Scene(), g.renderer, x, y)
		if g.drag != nil {
			_ = g.ed.Select(g.drag.Index)
		} else {
			g.setStatus("Grab a numbered circular handle; release to apply the edit")
		}
	}
	if g.drag != nil && released {
		drag := g.drag
		g.drag = nil
		restore := g.ed.Checkpoint()
		changed, err := drag.Commit(g.ed, g.renderer, x, y)
		if err != nil {
			g.recordError(err)
		} else if changed {
			g.queueSolve(restore)
		}
	}
}

func (g *game) scrub(x int) {
	rect := g.renderer.Controls()["scrub"]
	g.clock = math.Max(0, math.Min(1, float64(x-rect.Min.X)/float64(rect.Dx()))) * g.playbackDuration()
	g.playing = false
}

// action is shared by real click/key events and deterministic verification.
func (g *game) action(key string) {
	if g.presentationAction(key) || g.lapAction(key) {
		return
	}
	if g.opts.view == "perspective" {
		if key == "manual" {
			g.opts.view = "3d"
		} else if key == "add" || key == "delete" || strings.HasPrefix(key, "move-") || strings.HasPrefix(key, "width") || strings.HasPrefix(key, "bank") || strings.HasPrefix(key, "height") || strings.HasPrefix(key, "surface") || strings.HasPrefix(key, "road:") {
			g.setStatus("Choose Edit view to change road geometry")
			return
		}
	}
	if g.authoringAction(key) {
		return
	}
	if g.instrumentationAction(key) {
		return
	}
	if g.manualAction(key) {
		return
	}
	if g.setupAction(key) {
		return
	}
	s := g.ed.Scene()
	i := g.ed.Selected()
	switch key {
	case "comparison":
		g.comparison = !g.comparison
		g.clock = math.Min(g.clock, g.playbackDuration())
		return
	case "rate":
		switch g.rate {
		case .25:
			g.rate = .5
		case .5:
			g.rate = 1
		case 1:
			g.rate = 2
		default:
			g.rate = .25
		}
		return
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
	case "fit":
		g.cancelPointer()
		g.renderer.ResetCamera()
		g.setStatus("Camera fitted to the road")
		return
	case "view":
		if g.drag != nil || g.manualDrag != nil || g.scrubbing || g.charting || g.cameraDrag != nil {
			g.cancelPointer()
		}
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
	if g.drag != nil || g.manualDrag != nil || g.cameraDrag != nil {
		g.setStatus("Release the handle to apply the edit, or press Esc to cancel")
		return
	}
	if g.busy {
		g.setStatus("Finish computing this edit before changing the road again")
		return
	}
	var err error
	rollback := g.ed.Checkpoint()
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
	case "undo":
		if !g.ed.Undo() {
			g.setStatus("Nothing to undo")
			return
		}
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
			if p.WidthLeft != 0 || p.WidthRight != 0 || q.WidthLeft != 0 || q.WidthRight != 0 {
				p.WidthLeft, p.WidthRight = (p.LeftWidth()+q.LeftWidth())/2, (p.RightWidth()+q.RightWidth())/2
				p.Width = 0
			} else {
				p.Width = (p.Width + q.Width) / 2
			}
			p.KerbLeft.Width = (p.KerbLeft.Width + q.KerbLeft.Width) / 2
			p.KerbRight.Width = (p.KerbRight.Width + q.KerbRight.Width) / 2
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
		step := .5
		if key == "width-" {
			step = -step
		}
		if p.WidthLeft != 0 || p.WidthRight != 0 {
			p.WidthLeft += step / 2
			p.WidthRight += step / 2
		} else {
			p.Width += step
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
	if key == "load" || key == "new" || key == "preset" || key == "undo" || key == "redo" {
		loaded := g.ed.Scene()
		g.manualMode = loaded.Study != nil && loaded.Study.Manual != nil && loaded.Study.Manual.RoadDigest == track.RoadDigest(loaded)
	}
	g.resetCamera = key == "load" || key == "new" || key == "preset"
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

var demoActions = []string{"next", "next", "pointer-drag", "undo", "width+", "bank-", "height+", "move-up", "add", "undo", "redo", "delete", "surface+", "surface-", "save", "new", "vehicle", "preset", "load", "view", "view", "scrub-mid", "play"}

func (g *game) runDemo() {
	if g.demoIndex >= len(demoActions) {
		return
	}
	a := demoActions[g.demoIndex]
	if a == "pointer-drag" {
		switch g.demoDragStep {
		case 0:
			p := g.ed.Scene().Points[g.ed.Selected()].Position()
			x, y := g.renderer.Project(p)
			endX, endY := g.renderer.Project(p.Add(track.Vec3{X: 2, Y: 1}))
			g.pointer(x+6, y-4, true, true, false)
			if g.drag == nil {
				g.recordError(fmt.Errorf("pointer did not pick selected control"))
			} else {
				g.demoDragX, g.demoDragY = endX+6, endY-4
				g.demoDragStep = 1
				g.nextDemo = g.frame + 5
				return
			}
		case 1:
			g.pointer(g.demoDragX, g.demoDragY, false, true, false)
			g.demoDragStep = 2
			g.nextDemo = g.frame + 5
			return
		case 2:
			g.pointer(g.demoDragX, g.demoDragY, false, false, true)
			g.demoDragStep = 0
		}
	} else {
		g.action(a)
	}
	g.demoLog = append(g.demoLog, a)
	g.demoIndex++
	g.nextDemo = g.frame + 5
}

func (g *game) Draw(screen *ebiten.Image) {
	status := g.status
	if time.Now().After(g.statusUntil) && !g.busy {
		status = ""
	}
	if g.drag != nil {
		status = "Drag preview · release to apply · Esc to cancel"
	} else if g.hover >= 0 && !g.busy && status == "" {
		status = fmt.Sprintf("Drag handle %02d to reshape the road; release to compute the line", g.hover+1)
	}
	path := g.opts.file
	if g.fileEditing {
		path = g.fileDraft + "|"
		status = "Editing save / load path · ENTER confirm · ESC cancel"
	}
	setupCar := g.ed.Vehicle()
	state := render.State{ManualHandles: g.manualHandles(), ManualDragging: g.manualDrag != nil, ManualFailure: g.manualFailure, SetupConfig: &setupCar, Sensitivity: g.sensitivity, Provisional: g.provisional, Selected: g.ed.Selected(), Playing: g.playing, Status: status, FPS: ebiten.ActualFPS(), FilePath: path, Comparison: g.comparison, Rate: g.rate}
	if g.hover >= 0 && !g.busy {
		state.Hover = &g.hover
	}
	if g.drag != nil {
		state.Drag = &render.DragPreview{Index: g.drag.Index, Position: g.drag.Position(g.renderer, g.pointerX, g.pointerY)}
	}
	frame := g.renderer.FrameWithState(g.clock, state)
	g.texture.WritePixels(frame.(*image.RGBA).Pix)
	screen.DrawImage(g.texture, nil)
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
