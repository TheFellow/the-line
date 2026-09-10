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
	result racecraft.Result
	err    error
}

func (g *game) racePath() string {
	if g.opts.raceFile != "" {
		return g.opts.raceFile
	}
	return "racecraft.json"
}

func (g *game) startRace(e racecraft.Experiment) {
	g.cancelPointer()
	g.busy = true
	g.setStatus("Planning two cars and checking continuous body clearance...")
	g.raceReplies = make(chan raceReply, 1)
	replies := g.raceReplies
	go func() {
		r, err := racecraft.Plan(context.Background(), e.Scene, e.Vehicle, e.Config)
		replies <- raceReply{r, err}
	}()
}
func (g *game) acceptRace() {
	if g.raceReplies == nil {
		return
	}
	select {
	case reply := <-g.raceReplies:
		g.raceReplies = nil
		g.busy = false
		if reply.err != nil {
			g.setStatus("Race edit rejected: " + reply.err.Error())
			return
		}
		old := g.race
		g.race = &reply.result
		if old == nil {
			g.raceReturnView = g.opts.view
			g.raceReturnRenderer = g.renderer
		}
		if g.opts.view == "perspective" {
			g.opts.view = "3d"
		}
		g.clock = 0
		g.resetCamera = old == nil || old.Config.Scenario != reply.result.Config.Scenario
		if err := g.rebuild(); err != nil {
			g.race = old
			g.recordError(err)
			return
		}
		g.setStatus(fmt.Sprintf("%s · %d completed passes · change placement and replay", reply.result.Config.Scenario, len(reply.result.Events)))
	default:
	}
}

func (g *game) raceAction(key string) bool {
	if key == "race-mode" {
		if g.busy {
			g.setStatus("Wait for the current plan to finish")
			return true
		}
		if g.race != nil {
			g.race = nil
			g.clock = 0
			g.opts.view = g.raceReturnView
			g.renderer = g.raceReturnRenderer
			g.raceReturnRenderer = nil
			g.resetCamera = false
			return true
		}
		e, err := racecraft.Example("over-under")
		if err != nil {
			g.recordError(err)
			return true
		}
		g.startRace(e)
		return true
	}
	if g.race == nil {
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
		g.setStatus("Wait for the current race plan to finish")
		return true
	}
	e := racecraft.Experiment{Config: g.race.Config, Scene: g.race.Scene, Vehicle: g.race.Vehicle}
	switch key {
	case "race-scenario":
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
			return true
		}
	}
	g.startRace(e)
	return true
}

func (g *game) rebuildRace() error {
	opts := render.Options{Width: g.opts.width, Height: g.opts.height, View: g.opts.view, Race: g.race}
	if !g.resetCamera {
		camera := g.renderer.Camera()
		opts.Camera = &camera
	}
	r, err := render.New(g.race.Scene, g.race.Cars[0].Path, g.race.Vehicle, opts)
	if err != nil {
		return err
	}
	g.renderer = r
	g.resetCamera = false
	return nil
}
