package main

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/TheFellow/the-line/pkg/racecraft"
	"github.com/TheFellow/the-line/pkg/render"
)

type raceReply struct {
	result       racecraft.Result
	err          error
	resetExample bool
}

func (g *game) racePath() string {
	if g.opts.raceFile != "" {
		return g.opts.raceFile
	}
	return "racecraft.json"
}

func (g *game) startRace(e racecraft.Experiment, resetExample bool) {
	g.cancelRace()
	// Detach any qualifying chooser. Its callback can finish without editing
	// the retained study, even if the user returns before the read completes.
	g.importRequests = nil
	g.cancelPointer()
	g.busy = true
	g.setStatus("Planning two cars and checking continuous body clearance...")
	g.raceReplies = make(chan raceReply, 1)
	replies := g.raceReplies
	ctx, cancel := context.WithCancel(context.Background())
	g.cancelRacePlan = cancel
	go func() {
		r, err := racecraft.Plan(ctx, e.Scene, e.Vehicle, e.Config)
		replies <- raceReply{r, err, resetExample}
	}()
}

// Replies belong to one buffered channel per plan; abandoning it makes stale
// completions harmless and never blocks the planner while it exits.
func (g *game) cancelRace() {
	if g.cancelRacePlan != nil {
		g.cancelRacePlan()
		g.cancelRacePlan = nil
	}
	if g.raceReplies != nil {
		g.raceReplies = nil
		g.busy = false
	}
}

func (g *game) acceptRace() {
	if g.raceReplies == nil {
		return
	}
	select {
	case reply := <-g.raceReplies:
		g.cancelRace()
		if reply.err != nil {
			g.setStatus("Race edit rejected: " + reply.err.Error())
			return
		}
		view := g.opts.view
		if view == "perspective" {
			view = "3d"
		}
		resetCamera := g.race == nil || g.race.Config.Scenario != reply.result.Config.Scenario
		r, err := g.newRaceRenderer(&reply.result, view, resetCamera)
		if err != nil {
			g.recordError(err)
			return
		}
		// Commit all mode state only after both planning and rendering succeed.
		g.cancelPointer()
		if g.race == nil {
			g.raceReturnView = g.opts.view
			g.raceReturnRenderer = g.renderer
		}
		g.race = &reply.result
		g.renderer = r
		g.opts.view = view
		g.clock = 0
		g.resetCamera = false
		status := fmt.Sprintf("%s · %d completed passes · change placement and replay", reply.result.Config.Scenario, len(reply.result.Events))
		if reply.resetExample {
			status = "Loaded next example · reset road, vehicle and controls · " + reply.result.Config.Scenario
		}
		g.setStatus(status)
	default:
	}
}

func (g *game) raceAction(key string) bool {
	if key == "race-mode" {
		planning := g.raceReplies != nil
		if planning {
			g.cancelRace()
		}
		if g.busy {
			g.setStatus("Wait for the current plan to finish")
			return true
		}
		if g.race != nil {
			g.cancelPointer()
			g.race = nil
			g.clock = 0
			g.opts.view = g.raceReturnView
			g.renderer = g.raceReturnRenderer
			g.raceReturnRenderer = nil
			g.resetCamera = false
			g.setStatus("Qualifying study restored")
			return true
		}
		if planning {
			g.cancelPointer()
			g.setStatus("Race planning cancelled · qualifying study restored")
			return true
		}
		e, err := racecraft.Example("over-under")
		if err != nil {
			g.recordError(err)
			return true
		}
		g.startRace(e, false)
		return true
	}
	if g.race == nil {
		if g.raceReplies != nil {
			g.setStatus("Race planning in progress · R cancels and returns to qualifying")
			return true
		}
		return false
	}
	// Editing the qualifying study stays in qualifying mode. Race controls only
	// change a copy of the last certified experiment, committed after planning.
	switch key {
	case "play", "restart", "rate", "view", "fit", "scrub-mid":
		return false
	case "frame-", "frame+", "station-", "station+":
		delta := 1. / 60
		if strings.HasSuffix(key, "-") {
			delta = -delta
		}
		g.clock = math.Max(0, math.Min(g.race.Duration, g.clock+delta))
		g.playing = false
		return true
	}
	if g.busy {
		g.setStatus("Wait for the current race plan to finish · R cancels and returns to qualifying")
		return true
	}
	resetExample := false
	e := racecraft.Experiment{Config: g.race.Config, Scene: g.race.Scene, Vehicle: g.race.Vehicle}
	switch key {
	case "race-scenario":
		resetExample = true
		names := racecraft.Scenarios()
		for i, name := range names {
			if name == e.Config.Scenario {
				e, _ = racecraft.Example(names[(i+1)%len(names)])
				break
			}
		}
	case "race-save", "save":
		if err := racecraft.Save(g.racePath(), e); err != nil {
			g.recordError(err)
		} else {
			g.setStatus("Saved race experiment to " + g.racePath())
		}
		return true
	case "race-load", "load":
		var err error
		e, err = racecraft.Load(g.racePath())
		if err != nil {
			g.recordError(err)
			return true
		}
	default:
		delta := 1.
		if strings.HasSuffix(key, "-") {
			delta = -delta
		}
		switch strings.TrimRight(key, "+-") {
		case "race-gap":
			e.Config.Gap += delta
		case "race-overspeed":
			e.Config.Overspeed += delta
		case "race-separation":
			e.Config.Separation += delta * .25
		case "race-clearance":
			e.Config.Clearance += delta * .1
			e.Config.Clearance = math.Round(e.Config.Clearance*10) / 10
		default:
			g.setStatus("Qualifying controls are inactive in race mode · R returns to qualifying")
			return true
		}
	}
	g.startRace(e, resetExample)
	return true
}

func (g *game) newRaceRenderer(race *racecraft.Result, view string, resetCamera bool) (*render.Renderer, error) {
	opts := render.Options{Width: g.opts.width, Height: g.opts.height, View: view, Race: race}
	if !resetCamera {
		camera := g.renderer.Camera()
		opts.Camera = &camera
	}
	return render.New(race.Scene, race.Cars[0].Path, race.Vehicle, opts)
}

func (g *game) rebuildRace() error {
	r, err := g.newRaceRenderer(g.race, g.opts.view, g.resetCamera)
	if err != nil {
		return err
	}
	g.renderer = r
	g.resetCamera = false
	return nil
}
