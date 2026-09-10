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
	"github.com/TheFellow/the-line/pkg/vehicle"
)

// This read-only bridge exists only in explicit browsercheck builds. It cannot
// mutate application state or inject input: verification uses browser mouse and
// keyboard events, which travel through Ebitengine's normal input processing.
var inspection struct {
	requested  bool
	snapshot   string
	callback   js.Func
	nodes      *solver.Node
	digest     string
	pathDigest string
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
		pathHash := fnv.New64a()
		var bits [8]byte
		for _, n := range g.result.Nodes {
			for _, v := range []float64{n.Position.X, n.Position.Y, n.Position.Z, n.Offset} {
				binary.LittleEndian.PutUint64(bits[:], math.Float64bits(v))
				pathHash.Write(bits[:])
			}
			for _, v := range []float64{n.Position.X, n.Position.Y, n.Position.Z, n.Speed, n.Time} {
				binary.LittleEndian.PutUint64(bits[:], math.Float64bits(v))
				h.Write(bits[:])
			}
		}
		inspection.digest = fmt.Sprintf("%016x", h.Sum64())
		inspection.pathDigest = fmt.Sprintf("%016x", pathHash.Sum64())
	}
	var preview *track.Vec3
	if g.drag != nil {
		p := g.drag.Position(g.renderer, g.pointerX, g.pointerY)
		preview = &p
	}
	current := g.currentNode()
	center, _ := g.referenceTrajectory().AtStation(current.Station)
	viewport := g.renderer.RoadViewport()
	end := g.result.Nodes[len(g.result.Nodes)-1]
	centerEnd := g.referenceTrajectory().At(math.Inf(1))
	handles := g.manualHandles()
	manualPoints := make([][2]float64, len(handles))
	manualAxes := make([][2]float64, len(handles))
	manualControls := g.manualControls()
	for i, h := range handles {
		manualPoints[i] = project(h.Position)
		q := project(g.result.Road[manualControls[i].Index].AtOffset(h.Offset + 1))
		manualAxes[i] = [2]float64{q[0] - manualPoints[i][0], q[1] - manualPoints[i][1]}
	}
	referenceName := "Centreline"
	referenceStale := false
	referenceDigest := track.RoadDigest(g.solvedScene)
	if g.reference != nil {
		referenceName = g.reference.Name
		referenceStale = !g.reference.Compatible(g.solvedScene)
		referenceDigest = g.reference.RoadDigest
	}
	data := struct {
		PolishCandidates  int                  `json:"polishCandidates"`
		Analysis          bool                 `json:"analysis"`
		Sectors           []render.Sector      `json:"sectors"`
		Closed            bool                 `json:"closed"`
		StartNode         solver.Node          `json:"startNode"`
		SetupPage         int                  `json:"setupPage"`
		AuthoringOpen     bool                 `json:"authoringOpen"`
		LineColor         string               `json:"lineColor"`
		ChartChannel      string               `json:"chartChannel"`
		Markers           []solver.Marker      `json:"markers"`
		ForceCursor       [2]float64           `json:"forceCursor"`
		ChartCursor       [2]float64           `json:"chartCursor"`
		ManualFailure     *track.Vec3          `json:"manualFailure"`
		ManualAxes        [][2]float64         `json:"manualAxes"`
		ManualMode        bool                 `json:"manualMode"`
		ManualDragging    bool                 `json:"manualDragging"`
		ManualHandles     []render.LineHandle  `json:"manualHandles"`
		ManualPoints      [][2]float64         `json:"manualPoints"`
		ReferenceName     string               `json:"referenceName"`
		ReferenceStale    bool                 `json:"referenceStale"`
		ReferenceDigest   string               `json:"referenceDigest"`
		ReferenceDuration float64              `json:"referenceDuration"`
		SetupOpen         bool                 `json:"setupOpen"`
		Config            vehicle.Config       `json:"config"`
		RequestedConfig   vehicle.Config       `json:"requestedConfig"`
		Provisional       bool                 `json:"provisional"`
		Generation        int                  `json:"generation"`
		Analyzing         bool                 `json:"analyzing"`
		Sensitivity       []solver.Sensitivity `json:"sensitivity"`
		Scene             track.Scene          `json:"scene"`
		SolvedScene       track.Scene          `json:"solvedScene"`
		View              string               `json:"view"`
		Width             int                  `json:"width"`
		Height            int                  `json:"height"`
		Points            [][2]float64         `json:"points"`
		Controls          map[string][4]int    `json:"controls"`
		Origin            [2]float64           `json:"origin"`
		AxisX             [2]float64           `json:"axisX"`
		AxisY             [2]float64           `json:"axisY"`
		Selected          int                  `json:"selected"`
		Dragging          bool                 `json:"dragging"`
		Pointer           [2]float64           `json:"pointer"`
		Preview           *track.Vec3          `json:"preview"`
		Busy              bool                 `json:"busy"`
		CanUndo           bool                 `json:"canUndo"`
		CanRedo           bool                 `json:"canRedo"`
		Status            string               `json:"status"`
		Playing           bool                 `json:"playing"`
		Time              float64              `json:"time"`
		Duration          float64              `json:"duration"`
		Nodes             int                  `json:"nodes"`
		PathDigest        string               `json:"pathDigest"`
		LineDigest        string               `json:"lineDigest"`
		ForceResidual     float64              `json:"forceResidual"`
		Updates           int                  `json:"updates"`
		Camera            render.Camera        `json:"camera"`
		CameraDragging    bool                 `json:"cameraDragging"`
		Charting          bool                 `json:"charting"`
		Comparison        bool                 `json:"comparison"`
		Rate              float64              `json:"rate"`
		PlaybackDuration  float64              `json:"playbackDuration"`
		CenterDuration    float64              `json:"centerDuration"`
		Current           solver.Node          `json:"current"`
		CenterCurrent     solver.Node          `json:"centerCurrent"`
		SameStationCenter solver.Node          `json:"sameStationCenter"`
		Delta             float64              `json:"delta"`
		RoadViewport      [4]int               `json:"roadViewport"`
		SolveRequests     int                  `json:"solveRequests"`
		StationBounds     [2]float64           `json:"stationBounds"`
		EndNode           solver.Node          `json:"endNode"`
		CenterEndNode     solver.Node          `json:"centerEndNode"`
	}{
		Analysis: g.renderer.Analysis(), Sectors: g.renderer.Sectors(), Closed: g.result.Closed, StartNode: g.result.Nodes[0], SetupPage: g.setupPage, AuthoringOpen: g.authoringOpen,
		LineColor: string(g.renderer.LineColorMode()), ChartChannel: string(g.renderer.ChartChannel()),
		PolishCandidates: g.result.PolishCandidates,
		Markers:          g.renderer.Markers(), ForceCursor: g.renderer.ForceCursorWithState(g.clock, render.State{Playing: g.playing, Comparison: g.comparison}), ChartCursor: g.renderer.ChartCursorWithState(g.clock, render.State{Playing: g.playing, Comparison: g.comparison}),
		SetupOpen: g.setupOpen, Config: g.config, RequestedConfig: g.ed.Vehicle(), Provisional: g.provisional, Generation: g.generation, Analyzing: g.analyzing, Sensitivity: g.sensitivity,
		ManualFailure: g.manualFailure, ManualAxes: manualAxes, ManualMode: g.manualMode, ManualDragging: g.manualDrag != nil, ManualHandles: handles, ManualPoints: manualPoints, ReferenceName: referenceName, ReferenceStale: referenceStale, ReferenceDigest: referenceDigest, ReferenceDuration: g.referenceTrajectory().Duration,
		Scene: g.ed.Scene(), SolvedScene: g.solvedScene, View: g.opts.view,
		Width: g.opts.width, Height: g.opts.height, Points: points, Controls: controls,
		Origin: project(track.Vec3{}), AxisX: project(track.Vec3{X: 1}), AxisY: project(track.Vec3{Y: 1}),
		Selected: g.ed.Selected(), Dragging: g.drag != nil, Pointer: [2]float64{g.pointerX, g.pointerY}, Preview: preview,
		Busy: g.busy, CanUndo: g.ed.CanUndo(), CanRedo: g.ed.CanRedo(), Status: g.status, Playing: g.playing,
		Time: g.clock, Duration: g.result.Duration, Nodes: len(g.result.Nodes), LineDigest: inspection.digest, PathDigest: inspection.pathDigest,
		ForceResidual: g.result.MaxForceResidual, Updates: g.frame,
		Camera: g.renderer.Camera(), CameraDragging: g.cameraDrag != nil, Charting: g.charting,
		Comparison: g.comparison, Rate: g.rate, PlaybackDuration: g.playbackDuration(), CenterDuration: g.result.CenterDuration,
		Current: current, CenterCurrent: g.referenceTrajectory().LapAt(g.clock), SameStationCenter: center, Delta: current.Time - center.Time,
		RoadViewport: [4]int{viewport.Min.X, viewport.Min.Y, viewport.Max.X, viewport.Max.Y}, SolveRequests: g.solveRequests,
		StationBounds: [2]float64{g.result.Nodes[0].Station, end.Station}, EndNode: end, CenterEndNode: centerEnd,
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	inspection.snapshot = string(encoded)
}
