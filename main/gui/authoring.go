package main

import (
	"strings"

	"github.com/TheFellow/the-line/pkg/track"
)

type csvImport struct {
	data, name string
	err        error
}

func (g *game) authoringAction(key string) bool {
	if key == "authoring" {
		g.authoringOpen = !g.authoringOpen
		g.cancelPointer()
		if err := g.rebuild(); err != nil {
			g.recordError(err)
		}
		return true
	}
	if key == "import-csv" {
		g.cancelPointer()
		g.importRequests = make(chan csvImport, 1)
		g.chooseCSV(g.importRequests)
		return true
	}
	if !strings.HasPrefix(key, "road:") {
		return false
	}
	restore := g.ed.Checkpoint()
	s := track.Migrate(g.ed.Scene())
	i := g.ed.Selected()
	p := &s.Points[i]
	step := .5
	if strings.HasSuffix(key, "-") {
		step = -step
	}
	switch strings.TrimSuffix(strings.TrimSuffix(key, "+"), "-") {
	case "road:left":
		p.WidthLeft += step
	case "road:right":
		p.WidthRight += step
	case "road:kerb-left":
		p.KerbLeft.Width += step
	case "road:kerb-right":
		p.KerbRight.Width += step
	case "road:grip-left":
		p.KerbLeft.Grip = p.KerbLeft.Friction() + step/10
	case "road:grip-right":
		p.KerbRight.Grip = p.KerbRight.Friction() + step/10
	case "road:limits":
		s.KerbsCountAsRoad = !s.KerbsCountAsRoad
	default:
		return false
	}
	if err := g.ed.Replace(s); err != nil {
		g.recordError(err)
		return true
	}
	_ = g.ed.Select(i)
	g.queueSolve(restore)
	return true
}

func (g *game) updateImport() {
	if g.importRequests == nil || g.busy || g.race != nil {
		return
	}
	select {
	case imported := <-g.importRequests:
		if imported.err != nil {
			g.recordError(imported.err)
			return
		}
		scene, err := track.ImportCSV(strings.NewReader(imported.data), imported.name)
		if err != nil {
			g.recordError(err)
			return
		}
		restore := g.ed.Checkpoint()
		if err = g.ed.Replace(scene); err != nil {
			g.recordError(err)
			return
		}
		g.resetCamera = true
		g.queueSolve(restore)
	default:
	}
}
