package main

import "math"

func (g *game) playbackDuration() float64 {
	if g.comparison && len(g.result.CenterNodes) > 1 {
		return math.Max(g.result.Duration, g.result.CenterDuration)
	}
	return g.result.Duration
}

func (g *game) seekStation(x float64) {
	n, err := g.result.AtStation(g.renderer.ChartStation(x))
	if err != nil {
		g.recordError(err)
		return
	}
	g.clock = n.Time
	g.playing = false
}
