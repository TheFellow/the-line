package editor

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func editorForTest(t *testing.T) *Editor {
	t.Helper()
	s, _ := track.Preset("esses")
	e, err := New(s)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func TestTransactionAndOwnership(t *testing.T) {
	e := editorForTest(t)
	before := e.Scene()
	bad := before.Points[2]
	bad.Width = -1
	if e.UpdatePoint(2, bad) == nil {
		t.Fatal("invalid edit accepted")
	}
	if !reflect.DeepEqual(e.Scene(), before) || e.CanUndo() {
		t.Fatal("failed edit changed state")
	}
	copy := e.Scene()
	copy.Points[0].X = 900
	if !reflect.DeepEqual(e.Scene(), before) {
		t.Fatal("scene leaked writable ownership")
	}
}
func TestUndoRedoAndBranch(t *testing.T) {
	e := editorForTest(t)
	before := e.Scene()
	p := before.Points[2]
	p.Bank = 4
	p.Z = 2
	p.Width = 13
	p.Surface = "wet"
	if err := e.UpdatePoint(2, p); err != nil {
		t.Fatal(err)
	}
	after := e.Scene()
	if e.Selected() != 2 {
		t.Fatal("selection not updated")
	}
	if !e.Undo() || !reflect.DeepEqual(e.Scene(), before) {
		t.Fatal("undo failed")
	}
	if !e.Redo() || !reflect.DeepEqual(e.Scene(), after) {
		t.Fatal("redo failed")
	}
	e.Undo()
	p = before.Points[3]
	p.Bank = -3
	if err := e.UpdatePoint(3, p); err != nil {
		t.Fatal(err)
	}
	if e.Redo() {
		t.Fatal("stale branch redo survived")
	}
}
func TestAddDeleteMoveSaveLoad(t *testing.T) {
	e := editorForTest(t)
	s := e.Scene()
	p := s.Points[len(s.Points)-1]
	p.X += 45
	p.Y += 20
	if err := e.InsertPoint(len(s.Points)-1, p); err != nil {
		t.Fatal(err)
	}
	index := e.Selected()
	position := p.Position()
	position.Z = 1
	if err := e.MovePoint(index, position); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "custom.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	saved := track.Migrate(e.Scene())
	if err := e.DeletePoint(index); err != nil {
		t.Fatal(err)
	}
	if err := e.Load(path); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.Scene(), saved) {
		t.Fatal("load did not restore edited scene")
	}
	if !e.Undo() {
		t.Fatal("load is not undoable")
	}
}
func TestInvalidIndicesAndMinimum(t *testing.T) {
	e := editorForTest(t)
	if e.Select(-1) == nil || e.DeletePoint(500) == nil || e.InsertPoint(-2, track.Point{}) == nil {
		t.Fatal("bad index accepted")
	}
	s := track.Scene{Version: 1, Name: "New straight", Vehicle: "road", EntrySpeed: 20, ExitSpeed: 30, Points: []track.Point{{X: -50, Width: 12, Surface: "asphalt"}, {X: 50, Width: 12, Surface: "asphalt"}}}
	if err := e.Replace(s); err != nil {
		t.Fatal(err)
	}
	if e.DeletePoint(0) == nil {
		t.Fatal("deleted below two points")
	}
	if !e.Undo() {
		t.Fatal("new scene was not undoable")
	}
}
