package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TheFellow/the-line/internal/editor"
	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func (g *game) setupAction(key string) bool {
	if key == "setup-page" {
		g.setupPage = (g.setupPage + 1) % render.SetupPages()
		if err := g.rebuild(); err != nil {
			g.recordError(err)
		}
		return true
	}
	if key == "vehicle" {
		g.cycleVehicle()
		return true
	}
	if key == "setup" {
		g.setupOpen = !g.setupOpen
		g.cancelPointer()
		if err := g.rebuild(); err != nil {
			g.recordError(err)
		}
		return true
	}
	if key == "sensitivity" {
		if !g.busy && !g.analyzing {
			g.analyze()
		}
		return true
	}
	if key == "save-car" {
		if err := vehicle.Save(g.opts.file+".car.json", g.ed.Vehicle()); err != nil {
			g.recordError(err)
		} else {
			g.setStatus("Saved car " + g.opts.file + ".car.json")
		}
		return true
	}
	if !strings.HasPrefix(key, "setup:") && key != "load-car" && key != "reset-car" {
		return false
	}
	restore := g.ed.Checkpoint()
	car := g.ed.Vehicle()
	var err error
	switch key {
	case "load-car":
		car, err = vehicle.Load(g.opts.file + ".car.json")
	case "reset-car":
		car, err = vehicle.Preset(g.ed.Scene().Vehicle)
	default:
		fieldKey := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(key, "setup:"), "+"), "-")
		for _, field := range vehicle.SetupFields() {
			if field.Key == fieldKey {
				value, _ := car.Value(field.Key)
				step := field.Step
				if strings.HasSuffix(key, "-") {
					step = -step
				}
				if field.Key == "front_brake" && value == 0 && step > 0 {
					step = .5 // Start a fixed bias at balanced braking.
				}
				if field.Key == "lift_area" && value == 0 && step > 0 && car.AeroBalance == 0 {
					car.AeroBalance = car.FrontWeight
				}
				car, err = car.With(field.Key, value+step)
				car.Name = "Custom · " + g.ed.Scene().Vehicle
				break
			}
		}
	}
	if err == nil {
		err = g.ed.SetVehicle(car)
	}
	if err != nil {
		g.recordError(err)
		return true
	}
	if key != "reset-car" {
		saved := car
		g.customCar = &saved
	}
	g.startSolve(restore, true)
	return true
}

// A checkpoint belongs to one committed edit, including edits that supersede
// unfinished work. Recovering an unfinished predecessor resumes its solve;
// restoring an older displayed trajectory alone would lose that valid edit.
type solveCheckpoint struct {
	rollback func()
	previous *solveCheckpoint
	result   solver.Result
	config   vehicle.Config
	scene    track.Scene
}

func (g *game) checkpointSolve(rollback func()) {
	if rollback == nil { // Resuming a predecessor keeps its original checkpoint.
		return
	}
	var previous *solveCheckpoint
	if g.busy {
		previous = g.checkpoint
	}
	g.checkpoint = &solveCheckpoint{rollback: rollback, previous: previous,
		result: g.result, config: g.config, scene: g.solvedScene}
}

func (g *game) startSolve(rollback func(), progressive bool) {
	sceneNow := g.ed.Scene()
	if g.manualMode && !activeManual(sceneNow) {
		g.manualMode = false
		g.manualFailure = nil
	}
	if activeManual(sceneNow) {
		g.evaluateManual(rollback, sceneNow.Study.Manual.Offsets)
		return
	}
	if g.cancelSolve != nil {
		g.cancelSolve()
	}
	g.checkpointSolve(rollback)
	g.solveRequests++
	g.generation++
	generation := g.generation
	g.busy = true
	g.sensitivity = nil
	g.analyzing = false
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelSolve = cancel
	scene, config, current := g.ed.Scene(), g.ed.Vehicle(), g.result
	polish := g.polishNext
	g.polishNext = 0
	g.setStatus("Computing a feasible line… playback continues")
	send := func(reply solved) bool {
		reply.generation = generation
		reply.scene = scene
		select {
		case g.replies <- reply:
			return true
		case <-ctx.Done():
			return false
		}
	}
	go func() {
		opts := solver.DefaultOptions()
		opts.Polish = polish
		var provisional *solver.Result
		if progressive {
			evalOpts := opts
			evalOpts.Spacing = current.Spacing
			r, err := solver.EvaluateContext(ctx, scene, config, current.Offsets, evalOpts)
			if ctx.Err() != nil {
				return
			}
			// A valid setup can invalidate the old line (a wider car is a
			// common example). Only publish and seed from a feasible line;
			// otherwise search the new car's available road from scratch.
			if err == nil {
				provisional = &r
				if !send(solved{result: r, config: config, provisional: true}) {
					return
				}
				road, sampleErr := track.SampleRoad(scene, opts.Spacing)
				if sampleErr == nil {
					opts.Seed, _ = current.OffsetsAt(road)
				}
			} else if !send(solved{freshSearch: "Current line no longer fits; finding a fresh line…"}) {
				return
			}
		}
		r, err := editor.SolveSetup(ctx, scene, config, opts, func() {
			send(solved{freshSearch: "Warm-start line no longer fits; finding a fresh line…"})
		})
		if provisional != nil && (err != nil || r.Duration > provisional.Duration) && ctx.Err() == nil {
			search := r
			r = *provisional
			if err != nil {
				r.Termination = "retained verified current line; search failed: " + err.Error()
			} else {
				r.Iterations = search.Iterations
				r.Candidates = search.Candidates
				r.FineCandidates = search.FineCandidates
				r.SearchWorkers = search.SearchWorkers
				r.PolishCandidates = search.PolishCandidates
				r.RefineCandidates = search.RefineCandidates
				r.Termination = "retained faster verified current line"
			}
			err = nil
		}
		send(solved{result: r, config: config, err: err})
	}()
}

func (g *game) acceptReplies() error {
	for {
		select {
		case reply := <-g.replies:
			if reply.generation != g.generation {
				continue
			}
			if reply.freshSearch != "" {
				g.setStatus(reply.freshSearch)
				continue
			}
			if reply.analysis {
				g.analyzing = false
				if reply.err != nil {
					g.recordError(reply.err)
				} else {
					g.sensitivity = reply.sensitivity
				}
				continue
			}
			if reply.err != nil {
				g.busy = false
				g.provisional = false
				checkpoint := g.checkpoint
				if checkpoint != nil {
					checkpoint.rollback()
					g.result, g.config, g.solvedScene = checkpoint.result, checkpoint.config, checkpoint.scene
					g.checkpoint = checkpoint.previous
				}
				g.manualMode = g.manualMode && activeManual(g.ed.Scene())
				if err := g.rebuild(); err != nil {
					return err
				}
				g.markManualFailure(reply.err)
				g.recordError(fmt.Errorf("edit reverted: %w", reply.err))
				if g.checkpoint != nil {
					g.startSolve(nil, true)
					g.setStatus("Latest edit reverted; finishing the preceding edit…")
				}
				continue
			}
			fraction := g.clock / g.playbackDuration()
			g.result, g.config = reply.result, reply.config
			g.solvedScene = reply.scene
			g.provisional = reply.provisional
			g.busy = reply.provisional
			g.clock = fraction * g.playbackDuration()
			if g.resetCamera {
				g.cancelPointer()
			}
			if err := g.rebuild(); err != nil {
				return err
			}
			if reply.provisional {
				g.setStatus("Provisional · current line with new setup; refining…")
				return nil
			} else {
				g.setStatus(fmt.Sprintf("Solved · %.2f s · %+.2f s versus centreline", g.result.Duration, g.result.Duration-g.result.CenterDuration))
				g.checkpoint = nil
				g.resetCamera = false
				if g.setupOpen {
					g.analyze()
				}
			}
		default:
			return nil
		}
	}
}

func (g *game) analyze() {
	g.analyzing = true
	generation := g.generation
	scene, car, line := g.solvedScene, g.config, g.result
	ctx := context.Background()
	if g.cancelSolve != nil {
		g.cancelSolve()
	}
	ctx, cancel := context.WithCancel(ctx)
	g.cancelSolve = cancel
	go func() {
		rows, err := solver.Sensitivities(ctx, scene, car, line, solver.DefaultOptions().Margin)
		select {
		case g.replies <- solved{generation: generation, analysis: true, sensitivity: rows, err: err}:
		case <-ctx.Done():
		}
	}()
}

// cycleVehicle includes the most recently edited custom car after the presets.
func (g *game) cycleVehicle() {
	if g.busy {
		g.setStatus("Finish computing this edit before changing the car")
		return
	}
	scene := g.ed.Scene()
	current := g.ed.Vehicle()
	preset, _ := vehicle.Preset(scene.Vehicle)
	restore := g.ed.Checkpoint()
	if current != preset {
		saved := current
		g.customCar = &saved
		scene.Vehicle = "road"
	} else {
		names := vehicle.Presets()
		next := 0
		for i, name := range names {
			if name == scene.Vehicle {
				next = i + 1
			}
		}
		if next == len(names) && g.customCar != nil {
			if err := g.ed.SetVehicle(*g.customCar); err != nil {
				g.recordError(err)
				return
			}
			g.startSolve(restore, true)
			return
		}
		scene.Vehicle = names[next%len(names)]
	}
	scene.VehicleConfig = nil
	if err := g.ed.Replace(scene); err != nil {
		g.recordError(err)
		return
	}
	g.startSolve(restore, true)
}
