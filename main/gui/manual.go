package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"regexp"
	"strconv"

	"github.com/TheFellow/the-line/internal/editor"
	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

func (g *game) syncReference() error {
	var saved *track.PinnedLine
	if s := g.ed.Scene().Study; s != nil {
		saved = s.Reference
	}
	encoded, _ := json.Marshal(saved)
	key := string(encoded)
	if key == g.referenceKey {
		return nil
	}
	ref, err := render.RestoreReference(saved)
	if err != nil {
		return err
	}
	g.reference, g.referenceKey = ref, key
	return nil
}

func (g *game) referenceTrajectory() solver.Result {
	if g.reference != nil {
		if !g.reference.Compatible(g.solvedScene) {
			return solver.Result{}
		}
		return g.reference.Trajectory
	}
	return g.result.CenterTrajectory()
}

func (g *game) manualControls() []solver.LineControl {
	scene := g.ed.Scene()
	var offsets []float64
	if scene.Study != nil && scene.Study.Manual != nil && scene.Study.Manual.RoadDigest == track.RoadDigest(scene) {
		offsets = scene.Study.Manual.Offsets
	}
	return editor.ManualControls(g.result.Road, offsets, min(25, 2*len(scene.Points)-1))
}

func (g *game) manualHandles() []render.LineHandle {
	if !g.manualMode {
		return nil
	}
	controls := g.manualControls()
	if g.manualDrag != nil {
		controls[g.manualDrag.Control].Offset = g.manualDrag.Offset(g.pointerX, g.pointerY)
	}
	count := len(controls)
	if len(g.result.Road) > 0 && g.result.Road[0].Closed {
		count-- // One visible handle owns both endpoints of a periodic line.
	}
	handles := make([]render.LineHandle, count)
	for i, c := range controls {
		if i == count {
			break
		}
		handles[i] = render.LineHandle{Index: i, Position: g.result.Road[c.Index].AtOffset(c.Offset), Offset: c.Offset, Station: g.result.Road[c.Index].S}
	}
	return handles
}

func (g *game) manualAction(key string) bool {
	if key != "pin" && key != "unpin" && key != "manual" && key != "manual-zero" && key != "manual-optimize" && key != "manual-adopt" {
		return false
	}
	if g.busy || g.manualDrag != nil || g.drag != nil {
		g.setStatus("Finish or cancel the current edit first")
		return true
	}
	g.cancelPointer()
	scene := g.ed.Scene()
	if scene.Study == nil {
		scene.Study = &track.Study{Version: 1}
	}
	switch key {
	case "pin", "manual-adopt":
		name := "Pinned · " + g.config.Name
		if key == "manual-adopt" {
			name = "Manual · " + g.config.Name
		}
		scene.Study.Reference = render.StoreReference(name, g.solvedScene, g.result, g.config)
		if err := g.ed.Replace(scene); err != nil {
			g.recordError(err)
			return true
		}
		g.solvedScene = g.ed.Scene()
		g.setStatus("Pinned reference · future setup edits compare with this run")
	case "unpin":
		scene.Study.Reference = nil
		if err := g.ed.Replace(scene); err != nil {
			g.recordError(err)
			return true
		}
		g.solvedScene = g.ed.Scene()
		g.setStatus("Reference reset to the current car's centreline")
	case "manual-optimize":
		g.manualMode = false
		g.startSolve(g.ed.Checkpoint(), true)
	case "manual":
		g.manualMode = !g.manualMode
		if g.manualMode {
			g.playing = false
			if scene.Study.Manual == nil || scene.Study.Manual.RoadDigest != track.RoadDigest(scene) {
				g.commitManual(make([]float64, len(g.result.Road)))
			} else {
				g.evaluateManual(g.ed.Checkpoint(), scene.Study.Manual.Offsets)
			}
		}
	case "manual-zero":
		g.commitManual(make([]float64, len(g.result.Road)))
	}
	if err := g.rebuild(); err != nil {
		g.recordError(err)
	}
	return true
}

func (g *game) commitManual(offsets []float64) {
	restore := g.ed.Checkpoint()
	scene := g.ed.Scene()
	if scene.Study == nil {
		scene.Study = &track.Study{Version: 1}
	}
	scene.Study.Manual = &track.ManualLine{Spacing: g.result.Spacing, RoadDigest: track.RoadDigest(scene), Offsets: append([]float64(nil), offsets...)}
	if err := g.ed.Replace(scene); err != nil {
		g.recordError(err)
		return
	}
	g.evaluateManual(restore, offsets)
}

func (g *game) evaluateManual(rollback func(), offsets []float64) {
	if g.cancelSolve != nil {
		g.cancelSolve()
	}
	if !g.busy {
		g.rollback = rollback
		g.oldResult, g.oldConfig, g.oldScene = g.result, g.config, g.solvedScene
	}
	g.generation++
	g.solveRequests++
	g.busy = true
	g.provisional = false
	g.manualFailure = nil
	generation := g.generation
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelSolve = cancel
	scene, config := g.ed.Scene(), g.ed.Vehicle()
	opts := solver.DefaultOptions()
	opts.Spacing = scene.Study.Manual.Spacing
	g.setStatus("Evaluating your authored line…")
	go func() {
		result, err := solver.EvaluateContext(ctx, scene, config, offsets, opts)
		select {
		case g.replies <- solved{generation: generation, result: result, config: config, err: err}:
		case <-ctx.Done():
		}
	}()
}

var manualStationPattern = regexp.MustCompile(`station ([0-9.]+)`)

func (g *game) markManualFailure(err error) {
	if !g.manualMode {
		return
	}
	match := manualStationPattern.FindStringSubmatch(err.Error())
	if len(match) == 2 {
		station, e := strconv.ParseFloat(match[1], 64)
		if e == nil {
			n, e := g.result.AtStation(station)
			if e == nil {
				g.manualFailure = &n.Position
			}
		}
	}
}

// manualPointer exclusively owns ordinary left drags in authoring mode.
func (g *game) manualPointer(x, y float64, pressed, held, released bool) bool {
	if !g.manualMode && g.manualDrag == nil {
		return false
	}
	if g.busy {
		return true
	}
	controls := g.manualControls()
	if g.manualDrag == nil {
		if !image.Pt(int(x), int(y)).In(g.renderer.RoadViewport()) {
			return false
		}
		if pressed {
			g.manualDrag = editor.BeginLineDrag(g.result.Road, controls, g.renderer, x, y)
			g.playing = false
			g.manualFailure = nil
		}
		return true
	}
	if released || !held {
		drag := g.manualDrag
		g.manualDrag = nil
		offset := drag.Offset(x, y)
		if math.Abs(offset-drag.StartOffset) < 1e-8 {
			return true
		}
		controls[drag.Control].Offset = offset
		if g.result.Closed {
			controls[len(controls)-1].Offset = controls[0].Offset
		}
		offsets, err := solver.ManualOffsets(g.result.Road, controls, g.config.Width/2+solver.DefaultOptions().Margin)
		if err != nil {
			g.markManualFailure(err)
			g.recordError(fmt.Errorf("manual line rejected: %w", err))
			return true
		}
		g.commitManual(offsets)
	}
	return true
}
