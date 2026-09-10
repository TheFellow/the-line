package main

import "math"

func (g *game) playbackDuration() float64 {
	if g.result.Closed {
		return g.result.Duration
	}
	if g.comparison && len(g.referenceTrajectory().Nodes) > 1 {
		return math.Max(g.result.Duration, g.referenceTrajectory().Duration)
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
