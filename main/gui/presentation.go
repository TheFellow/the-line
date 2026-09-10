package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (g *game) presentationKeys() {
	if ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyAlt) {
		return
	}
	prefix := "frame"
	if ebiten.IsKeyPressed(ebiten.KeyShift) {
		prefix = "station"
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyComma) {
		g.presentationAction(prefix + "-")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPeriod) {
		g.presentationAction(prefix + "+")
	}
}
func (g *game) presentationAction(key string) bool {
	switch key {
	case "polish":
		if g.busy {
			g.setStatus("Wait for the current refinement to finish")
			return true
		}
		g.cancelPointer()
		g.manualMode = false
		g.polishNext = 1
		g.startSolve(g.ed.Checkpoint(), true)
		return true
	case "watch":
		g.cancelPointer()
		if g.opts.view == "perspective" {
			g.opts.view = "3d"
		} else {
			g.opts.view = "perspective"
		}
		if err := g.rebuild(); err != nil {
			g.recordError(err)
		}
		return true
	case "analysis":
		g.renderer.SetAnalysis(!g.renderer.Analysis())
		return true
	case "frame-", "frame+", "station-", "station+":
		g.cancelPointer()
		g.playing = false
		direction := 1.
		if key[len(key)-1] == '-' {
			direction = -1
		}
		if key[:5] == "frame" {
			g.clock = math.Max(0, g.clock+direction/60)
			if !g.result.Closed {
				g.clock = math.Min(g.playbackDuration(), g.clock)
			}
		} else {
			n, err := g.result.AtStation(g.currentNode().Station + direction*5)
			if err != nil {
				g.recordError(err)
			} else {
				g.clock = n.Time
			}
		}
		return true
	}
	return false
}
