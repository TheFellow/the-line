package editor

import (
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/pkg/track"
)

func TestCheckpointRestoresSceneSelectionAndHistory(t *testing.T) {
	e := editorForTest(t)
	initial := e.Scene()
	edit := func(index int, width float64) track.Scene {
		t.Helper()
		p := e.Scene().Points[index]
		p.Width = width
		if err := e.UpdatePoint(index, p); err != nil {
			t.Fatal(err)
		}
		return e.Scene()
	}
	first := edit(1, 13)
	second := edit(2, 14)
	if !e.Undo() {
		t.Fatal("could not prepare redo branch")
	}
	if err := e.Select(3); err != nil {
		t.Fatal(err)
	}
	restore := e.Checkpoint()

	// A provisional edit replaces the existing redo branch. Restoration must
	// recover that branch and must never make the rejected edit redoable.
	edit(0, 15)
	restore()
	if !reflect.DeepEqual(e.Scene(), first) || e.Selected() != 3 {
		t.Fatal("checkpoint did not restore scene and selection")
	}
	if !e.Redo() || !reflect.DeepEqual(e.Scene(), second) || e.Selected() != 2 {
		t.Fatal("checkpoint did not preserve the original redo branch")
	}
	if e.Redo() {
		t.Fatal("rejected edit remained redoable")
	}
	if !e.Undo() || !reflect.DeepEqual(e.Scene(), first) || e.Selected() != 3 {
		t.Fatal("undo lost the restored selection")
	}
	if !e.Undo() || !reflect.DeepEqual(e.Scene(), initial) || e.Selected() != 0 || e.Undo() {
		t.Fatal("checkpoint did not preserve the original undo history")
	}

	// Exercise new edits and both history stacks after restoration, then reuse
	// the checkpoint. Neither history may share writable storage with it.
	edit(0, 16)
	edit(1, 17)
	e.Undo()
	e.Undo()
	e.Redo()
	if err := e.Select(0); err != nil {
		t.Fatal(err)
	}
	restore()
	if !reflect.DeepEqual(e.Scene(), first) || e.Selected() != 3 {
		t.Fatal("later operations mutated the captured checkpoint")
	}
	if !e.Redo() || !reflect.DeepEqual(e.Scene(), second) || e.Redo() {
		t.Fatal("later operations mutated captured redo history")
	}
	restore()
	if !e.Undo() || !reflect.DeepEqual(e.Scene(), initial) || e.Undo() {
		t.Fatal("later operations mutated captured undo history")
	}
}

func TestCheckpointDoesNotMakeRejectedEditRedoable(t *testing.T) {
	e := editorForTest(t)
	before := e.Scene()
	restore := e.Checkpoint()
	p := before.Points[2]
	p.Width = 13
	if err := e.UpdatePoint(2, p); err != nil {
		t.Fatal(err)
	}
	restore()
	if !reflect.DeepEqual(e.Scene(), before) || e.CanUndo() || e.CanRedo() {
		t.Fatal("restoring a provisional edit added history")
	}
}
