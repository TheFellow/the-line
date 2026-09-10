package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func (g *game) setupAction(key string) bool {
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

func (g *game) startSolve(rollback func(), progressive bool) {
	if g.cancelSolve != nil {
		g.cancelSolve()
	}
	if !g.busy {
		g.rollback = rollback
		g.oldResult = g.result
		g.oldConfig = g.config
		g.oldScene = g.solvedScene
	}
	g.solveRequests++
	g.generation++
	generation := g.generation
	g.busy = true
	g.sensitivity = nil
	g.analyzing = false
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelSolve = cancel
	scene, config, current := g.ed.Scene(), g.ed.Vehicle(), g.result
	g.setStatus("Computing a feasible line… playback continues")
	send := func(reply solved) bool {
		reply.generation = generation
		select {
		case g.replies <- reply:
			return true
		case <-ctx.Done():
			return false
		}
	}
	go func() {
		opts := solver.DefaultOptions()
		var provisional *solver.Result
		if progressive {
			evalOpts := opts
			evalOpts.Spacing = current.Spacing
			r, err := solver.EvaluateContext(ctx, scene, config, current.Offsets, evalOpts)
			if err != nil {
				send(solved{err: err})
				return
			}
			provisional = &r
			if !send(solved{result: r, config: config, provisional: true}) {
				return
			}
			road, err := track.SampleRoad(scene, opts.Spacing)
			if err != nil {
				send(solved{err: err})
				return
			}
			opts.Seed, err = current.OffsetsAt(road)
			if err != nil {
				send(solved{err: err})
				return
			}
		}
		r, err := solver.SolveContext(ctx, scene, config, opts)
		if provisional != nil && (err != nil || r.Duration > provisional.Duration) && ctx.Err() == nil {
			r = *provisional
			r.Termination = "retained faster verified current line"
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
				if g.rollback != nil {
					g.rollback()
				}
				g.result, g.config, g.solvedScene = g.oldResult, g.oldConfig, g.oldScene
				if err := g.rebuild(); err != nil {
					return err
				}
				g.recordError(fmt.Errorf("edit reverted: %w", reply.err))
				g.rollback = nil
				continue
			}
			fraction := g.clock / g.playbackDuration()
			g.result, g.config = reply.result, reply.config
			g.solvedScene = g.ed.Scene()
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
			} else {
				g.setStatus(fmt.Sprintf("Solved · %.2f s · %.2f s faster than centreline", g.result.Duration, g.result.CenterDuration-g.result.Duration))
				g.rollback = nil
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
