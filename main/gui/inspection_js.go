//go:build browsercheck && js && wasm

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"syscall/js"

	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

// This read-only bridge exists only in explicit browsercheck builds. It cannot
// mutate application state or inject input: verification uses browser mouse and
// keyboard events, which travel through Ebitengine's normal input processing.
var inspection struct {
	requested bool
	snapshot  string
	callback  js.Func
	nodes     *solver.Node
	digest    string
}

func attachInspection(g *game) {
	inspection.callback = js.FuncOf(func(this js.Value, args []js.Value) any {
		inspection.requested = true
		return inspection.snapshot
	})
	js.Global().Set("theLineSnapshot", inspection.callback)
	inspection.requested = true
	inspectFrame(g)
}
func inspectFrame(g *game) {
	if !inspection.requested {
		return
	}
	inspection.requested = false
	points := make([][2]float64, len(g.solvedScene.Points))
	for i, p := range g.solvedScene.Points {
		x, y := g.renderer.Project(p.Position())
		points[i] = [2]float64{x, y}
	}
	controls := map[string][4]int{}
	for key, r := range g.renderer.Controls() {
		controls[key] = [4]int{r.Min.X, r.Min.Y, r.Max.X, r.Max.Y}
	}
	project := func(p track.Vec3) [2]float64 { x, y := g.renderer.Project(p); return [2]float64{x, y} }
	if len(g.result.Nodes) > 0 && inspection.nodes != &g.result.Nodes[0] {
		inspection.nodes = &g.result.Nodes[0]
		h := fnv.New64a()
		var bits [8]byte
		for _, n := range g.result.Nodes {
			for _, v := range []float64{n.Position.X, n.Position.Y, n.Position.Z, n.Speed, n.Time} {
				binary.LittleEndian.PutUint64(bits[:], math.Float64bits(v))
				h.Write(bits[:])
			}
		}
		inspection.digest = fmt.Sprintf("%016x", h.Sum64())
	}
	var preview *track.Vec3
	if g.drag != nil {
		p := g.drag.Position(g.renderer, g.pointerX, g.pointerY)
		preview = &p
	}
	current := g.result.At(g.clock)
	center, _ := g.result.CenterAtStation(current.Station)
	viewport := g.renderer.RoadViewport()
	end := g.result.Nodes[len(g.result.Nodes)-1]
	centerEnd := g.result.CenterAt(math.Inf(1))
	data := struct {
		Scene             track.Scene       `json:"scene"`
		SolvedScene       track.Scene       `json:"solvedScene"`
		View              string            `json:"view"`
		Width             int               `json:"width"`
		Height            int               `json:"height"`
		Points            [][2]float64      `json:"points"`
		Controls          map[string][4]int `json:"controls"`
		Origin            [2]float64        `json:"origin"`
		AxisX             [2]float64        `json:"axisX"`
		AxisY             [2]float64        `json:"axisY"`
		Selected          int               `json:"selected"`
		Dragging          bool              `json:"dragging"`
		Pointer           [2]float64        `json:"pointer"`
		Preview           *track.Vec3       `json:"preview"`
		Busy              bool              `json:"busy"`
		CanUndo           bool              `json:"canUndo"`
		CanRedo           bool              `json:"canRedo"`
		Status            string            `json:"status"`
		Playing           bool              `json:"playing"`
		Time              float64           `json:"time"`
		Duration          float64           `json:"duration"`
		Nodes             int               `json:"nodes"`
		LineDigest        string            `json:"lineDigest"`
		ForceResidual     float64           `json:"forceResidual"`
		Updates           int               `json:"updates"`
		Camera            render.Camera     `json:"camera"`
		CameraDragging    bool              `json:"cameraDragging"`
		Charting          bool              `json:"charting"`
		Comparison        bool              `json:"comparison"`
		Rate              float64           `json:"rate"`
		PlaybackDuration  float64           `json:"playbackDuration"`
		CenterDuration    float64           `json:"centerDuration"`
		Current           solver.Node       `json:"current"`
		CenterCurrent     solver.Node       `json:"centerCurrent"`
		SameStationCenter solver.Node       `json:"sameStationCenter"`
		Delta             float64           `json:"delta"`
		RoadViewport      [4]int            `json:"roadViewport"`
		SolveRequests     int               `json:"solveRequests"`
		StationBounds     [2]float64        `json:"stationBounds"`
		EndNode           solver.Node       `json:"endNode"`
		CenterEndNode     solver.Node       `json:"centerEndNode"`
	}{
		Scene: g.ed.Scene(), SolvedScene: g.solvedScene, View: g.opts.view,
		Width: g.opts.width, Height: g.opts.height, Points: points, Controls: controls,
		Origin: project(track.Vec3{}), AxisX: project(track.Vec3{X: 1}), AxisY: project(track.Vec3{Y: 1}),
		Selected: g.ed.Selected(), Dragging: g.drag != nil, Pointer: [2]float64{g.pointerX, g.pointerY}, Preview: preview,
		Busy: g.busy, CanUndo: g.ed.CanUndo(), CanRedo: g.ed.CanRedo(), Status: g.status, Playing: g.playing,
		Time: g.clock, Duration: g.result.Duration, Nodes: len(g.result.Nodes), LineDigest: inspection.digest,
		ForceResidual: g.result.MaxForceResidual, Updates: g.frame,
		Camera: g.renderer.Camera(), CameraDragging: g.cameraDrag != nil, Charting: g.charting,
		Comparison: g.comparison, Rate: g.rate, PlaybackDuration: g.playbackDuration(), CenterDuration: g.result.CenterDuration,
		Current: current, CenterCurrent: g.result.CenterAt(g.clock), SameStationCenter: center, Delta: current.Time - center.Time,
		RoadViewport: [4]int{viewport.Min.X, viewport.Min.Y, viewport.Max.X, viewport.Max.Y}, SolveRequests: g.solveRequests,
		StationBounds: [2]float64{g.result.Nodes[0].Station, end.Station}, EndNode: end, CenterEndNode: centerEnd,
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	inspection.snapshot = string(encoded)
}
