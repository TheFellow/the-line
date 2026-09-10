package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"math"
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
			g.clock = math.Max(0, math.Min(g.playbackDuration(), g.currentNode().Time+direction/60))
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
