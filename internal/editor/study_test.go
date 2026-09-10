package editor

import (
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestActiveLineSelectionSharesSetupAndGeometryHistory(t *testing.T) {
	scene, _ := track.Preset("esses")
	scene.Study = &track.Study{Version: 1, ActiveLine: track.LineManual,
		Manual: &track.ManualLine{Spacing: 3, RoadDigest: track.RoadDigest(scene), Offsets: []float64{0, 1, 0}},
	}
	ed, err := New(scene)
	if err != nil {
		t.Fatal(err)
	}
	optimized := ed.Scene()
	optimized.Study.ActiveLine = track.LineOptimized
	if err := ed.Replace(optimized); err != nil {
		t.Fatal(err)
	}
	gt, _ := vehicle.Preset("gt")
	if err := ed.SetVehicle(gt); err != nil {
		t.Fatal(err)
	}
	if !ed.Undo() || ed.Scene().Study.UsesManual() || ed.Vehicle() == gt {
		t.Fatal("undoing setup changed the selected optimized line")
	}
	if !ed.Undo() || !ed.Scene().Study.UsesManual() {
		t.Fatal("undoing optimization did not restore authored selection")
	}
	if !ed.Redo() || ed.Scene().Study.UsesManual() || !ed.Redo() || ed.Vehicle() != gt || ed.Scene().Study.UsesManual() {
		t.Fatal("redo did not restore optimized selection and GT setup")
	}
	if !reflect.DeepEqual(ed.Scene().Study.Manual, scene.Study.Manual) {
		t.Fatal("optimization or setup history discarded the authored hypothesis")
	}
}

func TestGeometryEditDeactivatesManualInOneTransaction(t *testing.T) {
	scene, _ := track.Preset("esses")
	scene.Study = &track.Study{Version: 1, Manual: &track.ManualLine{Spacing: 3, RoadDigest: track.RoadDigest(scene), Offsets: []float64{0, 1, 0}}}
	ed, err := New(scene)
	if err != nil {
		t.Fatal(err)
	}
	p := scene.Points[1]
	p.Bank++
	if err := ed.UpdatePoint(1, p); err != nil {
		t.Fatal(err)
	}
	changed := ed.Scene()
	if changed.Study.ActiveLine != track.LineOptimized || !reflect.DeepEqual(changed.Study.Manual, scene.Study.Manual) {
		t.Fatal("geometry edit must retain but deactivate the manual hypothesis")
	}
	if !ed.Undo() || !ed.Scene().Study.UsesManual() || ed.Scene().Points[1] != scene.Points[1] || ed.CanUndo() {
		t.Fatal("geometry and manual selection did not undo together")
	}
	if !ed.Redo() || ed.Scene().Study.UsesManual() || ed.Scene().Points[1] != p {
		t.Fatal("geometry and optimized selection did not redo together")
	}
	if !scene.Study.UsesManual() || scene.Study.ActiveLine != "" {
		t.Fatal("editor mutated caller's study")
	}
}
