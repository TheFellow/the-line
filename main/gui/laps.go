package main

import "github.com/TheFellow/the-line/pkg/solver"

func (g *game) currentNode() solver.Node {
	if g.result.Closed && !g.playing && g.clock <= g.result.Duration {
		return g.result.At(g.clock)
	}
	return g.result.LapAt(g.clock)
}

func (g *game) lapAction(key string) bool {
	if key != "closed" {
		return false
	}
	if g.busy {
		g.setStatus("Finish the current solve before changing road topology")
		return true
	}
	g.cancelPointer()
	restore := g.ed.Checkpoint()
	scene := g.ed.Scene()
	scene.Closed = !scene.Closed
	if err := g.ed.Replace(scene); err != nil {
		g.recordError(err)
		return true
	}
	g.manualMode = false
	g.resetCamera = true
	g.queueSolve(restore)
	return true
}
