package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
)

// Sector is the time between successive geometry controls, at shared road stations.
// Delta is current minus reference: a negative value is time gained.
type Sector struct {
	Number                    int `json:"number"`
	Start, End                float64
	Current, Reference, Delta float64
}

// SectorSplits compares trajectories at each original control-point station.
// SampleRoad retains these exact positions; unrelated or incomplete roads fail
// rather than silently comparing nearest points on different geometry.
func SectorSplits(scene track.Scene, road []track.Sample, current, reference solver.Result) ([]Sector, error) {
	if len(road) < 2 || len(current.Nodes) < 2 || len(reference.Nodes) < 2 {
		return nil, fmt.Errorf("sectors require two trajectories and a sampled road")
	}
	stations := make([]float64, 0, len(scene.Points)+1)
	cursor := 0
	for i, p := range scene.Points {
		found := -1
		for j := cursor; j < len(road); j++ {
			if road[j].Position.Sub(p.Position()).Length() < 1e-6 {
				found = j
				break
			}
		}
		if found < 0 {
			return nil, fmt.Errorf("sector control %d is absent from sampled road", i+1)
		}
		stations = append(stations, road[found].S)
		cursor = found + 1
	}
	if scene.Closed {
		stations = append(stations, road[len(road)-1].S)
	}
	out := make([]Sector, 0, len(stations)-1)
	for i := 1; i < len(stations); i++ {
		a, err := current.AtStation(stations[i-1])
		if err != nil {
			return nil, err
		}
		b, err := current.AtStation(stations[i])
		if err != nil {
			return nil, err
		}
		c, err := reference.AtStation(stations[i-1])
		if err != nil {
			return nil, err
		}
		d, err := reference.AtStation(stations[i])
		if err != nil {
			return nil, err
		}
		out = append(out, Sector{Number: i, Start: stations[i-1], End: stations[i], Current: b.Time - a.Time, Reference: d.Time - c.Time, Delta: (b.Time - a.Time) - (d.Time - c.Time)})
	}
	return out, nil
}
func (r *Renderer) Sectors() []Sector {
	if !r.referenceCompatible() {
		return nil
	}
	sectors, _ := SectorSplits(r.scene, r.result.Road, r.result, r.referenceTrajectory())
	return sectors
}
func (r *Renderer) Analysis() bool { return r.opts.Analysis }
func (r *Renderer) SetAnalysis(enabled bool) {
	r.opts.Analysis = enabled
	r.controls = make(map[string]image.Rectangle)
	r.drawBase()
}

func (r *Renderer) presentationBase(im *image.RGBA) {
	label := "WATCH"
	if r.opts.View == "perspective" {
		label = "EDIT VIEW"
	}
	r.button(im, "watch", image.Rect(530, 576, 618, 606), label, r.opts.View == "perspective")
	r.button(im, "analysis", image.Rect(625, 576, 711, 606), "SECTORS", r.opts.Analysis)
	// Transport has its own compact row above the telemetry, leaving the scrub strip clear.
	for i, b := range []struct{ key, label string }{{"frame-", "‹ FRAME"}, {"frame+", "FRAME ›"}, {"station-", "‹ 5 m"}, {"station+", "5 m ›"}} {
		x := 736 + i*86
		r.button(im, b.key, image.Rect(x, 849, x+80, 870), b.label, false)
	}
	if !r.opts.Analysis && r.opts.View != "perspective" {
		return
	}
	x := r.opts.Width - 304
	area := image.Rect(x-5, 368, x+290, 782)
	fill(im, area, panel)
	// Hidden controls must not remain interactive under this panel.
	for k, rect := range r.controls {
		if rect.Overlaps(area) {
			delete(r.controls, k)
		}
	}
	if !r.opts.Analysis {
		r.text(im, x, 389, "TRACKSIDE / WATCH ONLY", 13, accent, true)
		for i, text := range []string{"Perspective reveals bank and elevation.", "Cars share elapsed time, not station.", "The line uses the same absolute scale.", "", "SPACE plays or pauses.", "Comma / period step one frame.", "SHIFT + comma / period step 5 metres.", "Drag the chart to inspect a road station.", "SECTORS shows each corner's time gain.", "EDIT VIEW returns to geometry editing."} {
			r.text(im, x, 423+i*24, text, 12, muted, false)
		}
		r.referenceSummary(im)
		return
	}
	r.text(im, x, 389, "SECTORS / CONTROL POINTS", 13, accent, true)
	r.text(im, x, 411, "A current · B reference · Δ A − B", 11, muted, false)
	r.text(im, x, 435, "SECTOR       A / B SECONDS          Δ", 11, muted, true)
	sectors := r.Sectors()
	if len(sectors) == 0 {
		r.text(im, x, 464, "Reference stale · different road", 12, referenceColor, false)
		return
	}
	// Keep nine rows visible so numerical search diagnostics have their own area.
	r.text(im, x, 689, fmt.Sprintf("%d sectors · negative Δ is time gained", len(sectors)), 11, muted, false)
	fineCandidates := r.result.FineCandidates + r.result.RefineCandidates + r.result.PolishCandidates
	r.text(im, x, 706, fmt.Sprintf("%d candidates · %d fine · %d workers", r.result.Candidates, fineCandidates, r.result.SearchWorkers), 11, muted, false)
	r.text(im, x, 723, fmt.Sprintf("Final search gained %.3f s", r.result.BeforeRefineDuration-r.result.Duration), 11, muted, false)
	r.button(im, "polish", image.Rect(x, 732, x+280, 757), "REFINE CURRENT LINE", false)
	r.text(im, x, 777, truncate(r.result.Termination, 45), 11, muted, false)
}
func (r *Renderer) presentationFrame(im *image.RGBA, n solver.Node) {
	// Signed, fixed ±2 second scale, independent of vehicle or sequence.
	left, right, zero := 186., 355., 270.5
	r.text(im, 186, 856, "DELTA / ±2 s", 11, muted, true)
	line(im, point{left, 866}, point{right, 866}, 4, faint)
	line(im, point{zero, 860}, point{zero, 872}, 1, ink)
	ref, err := r.referenceAtStation(n.Station)
	if err == nil {
		delta := n.Time - ref.Time
		col := accent
		if delta > 0 {
			col = color.RGBA{255, 145, 113, 255}
		}
		end := zero + math.Max(-1, math.Min(1, delta/2))*(right-left)/2
		line(im, point{zero, 866}, point{end, 866}, 4, col)
		r.text(im, 286, 856, fmt.Sprintf("%+.3fs", delta), 11, col, true)
	}
	if !r.opts.Analysis {
		return
	}
	sectors := r.Sectors()
	selected := 0
	for i, s := range sectors {
		if n.Station >= s.Start {
			selected = i
		}
	}
	start := max(0, min(selected-4, len(sectors)-9))
	end := min(len(sectors), start+9)
	x := r.opts.Width - 304
	for i := start; i < end; i++ {
		s := sectors[i]
		y := 458 + (i-start)*26
		col := accent
		if s.Delta > 0 {
			col = color.RGBA{255, 145, 113, 255}
		}
		if i == selected {
			fill(im, image.Rect(x-3, y-17, x+283, y+5), color.RGBA{35, 48, 55, 255})
		}
		r.text(im, x, y, fmt.Sprintf("%02d", s.Number), 12, ink, true)
		r.text(im, x+37, y, fmt.Sprintf("%5.2f / %5.2f", s.Current, s.Reference), 12, muted, false)
		r.text(im, x+198, y, fmt.Sprintf("%+.3f", s.Delta), 12, col, true)
	}
}
