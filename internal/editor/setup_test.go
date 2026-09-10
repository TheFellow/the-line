package editor

import (
	"path/filepath"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestSetupSharesSceneHistory(t *testing.T) {
	scene, _ := track.Preset("esses")
	ed, err := New(scene)
	if err != nil {
		t.Fatal(err)
	}
	original := ed.Vehicle()
	changed := original
	changed.Grip += .05
	if err := ed.SetVehicle(changed); err != nil {
		t.Fatal(err)
	}
	p := ed.Scene().Points[1]
	p.Z += .2
	if err := ed.UpdatePoint(1, p); err != nil {
		t.Fatal(err)
	}
	if !ed.Undo() || ed.Vehicle() != changed || ed.Scene().Points[1].Z != scene.Points[1].Z {
		t.Fatal("geometry undo altered setup")
	}
	if !ed.Undo() || ed.Vehicle() != original {
		t.Fatal("setup undo failed")
	}
	invalid := changed
	invalid.Mass = -1
	if err := ed.SetVehicle(invalid); err == nil {
		t.Fatal("invalid setup accepted")
	}
	if !ed.CanRedo() || ed.Vehicle() != original {
		t.Fatal("rejected setup changed history")
	}
	if !ed.Redo() || ed.Vehicle() != changed {
		t.Fatal("setup redo failed")
	}
	restore := ed.Checkpoint()
	if err := ed.SetVehicle(original); err != nil {
		t.Fatal(err)
	}
	restore()
	if ed.Vehicle() != changed || !ed.CanRedo() {
		t.Fatal("checkpoint lost setup/history")
	}
}

func TestStudyPersistsInlineSetup(t *testing.T) {
	scene, _ := track.Preset("hairpin")
	ed, err := New(scene)
	if err != nil {
		t.Fatal(err)
	}
	car := ed.Vehicle()
	car.Power += 25000
	car.Name = "Custom road"
	if err := ed.SetVehicle(car); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "study.json")
	if err := ed.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := New(scene)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.Load(path); err != nil {
		t.Fatal(err)
	}
	if loaded.Vehicle() != car {
		t.Fatalf("setup lost: %+v", loaded.Vehicle())
	}
	if !loaded.Undo() || loaded.Vehicle() == car {
		t.Fatal("load not undoable")
	}
}

func TestSetupSceneCopyCannotMutateEditor(t *testing.T) {
	scene, _ := track.Preset("esses")
	ed, _ := New(scene)
	car := ed.Vehicle()
	car.Grip += .05
	if err := ed.SetVehicle(car); err != nil {
		t.Fatal(err)
	}
	copy := ed.Scene()
	copy.VehicleConfig.Grip = 2
	if ed.Scene().VehicleConfig.Grip != car.Grip {
		t.Fatal("scene config pointer aliases editor")
	}
	if ed.Vehicle() != car {
		t.Fatal("vehicle snapshot altered")
	}
}
