// Package editor applies transactional scene edits independently of a windowing
// system. Invalid edits leave both the current scene and undo history unchanged.
package editor

import (
	"fmt"

	"github.com/TheFellow/the-line/pkg/track"
)

type snapshot struct {
	scene    track.Scene
	selected int
}
type Editor struct {
	current    snapshot
	undo, redo []snapshot
}

func clone(s track.Scene) track.Scene { s.Points = append([]track.Point(nil), s.Points...); return s }
func valid(s track.Scene) error       { _, err := track.SampleRoad(s, 2); return err }
func New(scene track.Scene) (*Editor, error) {
	if err := valid(scene); err != nil {
		return nil, err
	}
	return &Editor{current: snapshot{clone(scene), 0}}, nil
}

// Scene returns an independent copy; mutating it cannot bypass validation.
func (e *Editor) Scene() track.Scene { return clone(e.current.scene) }
func (e *Editor) Selected() int      { return e.current.selected }
func (e *Editor) Select(index int) error {
	if err := e.index(index); err != nil {
		return err
	}
	e.current.selected = index
	return nil
}
func (e *Editor) index(i int) error {
	if i < 0 || i >= len(e.current.scene.Points) {
		return fmt.Errorf("editor: control point %d is out of range", i)
	}
	return nil
}
func (e *Editor) commit(s track.Scene, selected int) error {
	if err := valid(s); err != nil {
		return err
	}
	e.undo = append(e.undo, e.current)
	if len(e.undo) > 100 {
		e.undo = append([]snapshot(nil), e.undo[len(e.undo)-100:]...)
	}
	e.redo = nil
	e.current = snapshot{clone(s), max(0, min(selected, len(s.Points)-1))}
	return nil
}

// Replace commits a new or loaded scene as one undoable operation.
func (e *Editor) Replace(scene track.Scene) error { return e.commit(scene, 0) }
func (e *Editor) UpdatePoint(index int, p track.Point) error {
	if err := e.index(index); err != nil {
		return err
	}
	s := e.Scene()
	s.Points[index] = p
	return e.commit(s, index)
}
func (e *Editor) MovePoint(index int, position track.Vec3) error {
	if err := e.index(index); err != nil {
		return err
	}
	p := e.current.scene.Points[index]
	p.X, p.Y, p.Z = position.X, position.Y, position.Z
	return e.UpdatePoint(index, p)
}

// InsertPoint inserts after the given index; -1 inserts at the start.
func (e *Editor) InsertPoint(after int, p track.Point) error {
	if after < -1 || after >= len(e.current.scene.Points) {
		return fmt.Errorf("editor: insertion index %d is out of range", after)
	}
	s := e.Scene()
	s.Points = append(s.Points, track.Point{})
	copy(s.Points[after+2:], s.Points[after+1:])
	s.Points[after+1] = p
	return e.commit(s, after+1)
}
func (e *Editor) DeletePoint(index int) error {
	if err := e.index(index); err != nil {
		return err
	}
	if len(e.current.scene.Points) <= 2 {
		return fmt.Errorf("editor: a road needs at least two control points")
	}
	s := e.Scene()
	s.Points = append(s.Points[:index], s.Points[index+1:]...)
	return e.commit(s, min(index, len(s.Points)-1))
}
func (e *Editor) Undo() bool {
	if len(e.undo) == 0 {
		return false
	}
	e.redo = append(e.redo, e.current)
	e.current = e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	return true
}
func (e *Editor) Redo() bool {
	if len(e.redo) == 0 {
		return false
	}
	e.undo = append(e.undo, e.current)
	e.current = e.redo[len(e.redo)-1]
	e.redo = e.redo[:len(e.redo)-1]
	return true
}
func (e *Editor) CanUndo() bool { return len(e.undo) > 0 }
func (e *Editor) CanRedo() bool { return len(e.redo) > 0 }

// Checkpoint captures the scene, selection, and undo/redo history. Call the
// returned function to discard provisional edits after an asynchronous operation
// fails. Restoring does not add an undo entry, and the checkpoint can be reused.
// Capture and restoration must run on the editor's owning goroutine.
func (e *Editor) Checkpoint() func() {
	saved := e.copyState()
	return func() { *e = saved.copyState() }
}

func (e *Editor) copyState() Editor {
	copySnapshot := func(s snapshot) snapshot {
		return snapshot{scene: clone(s.scene), selected: s.selected}
	}
	copyHistory := func(history []snapshot) []snapshot {
		out := make([]snapshot, len(history))
		for i, s := range history {
			out[i] = copySnapshot(s)
		}
		return out
	}
	return Editor{current: copySnapshot(e.current), undo: copyHistory(e.undo), redo: copyHistory(e.redo)}
}

func (e *Editor) Load(path string) error {
	s, err := track.Load(path)
	if err != nil {
		return err
	}
	return e.Replace(s)
}
func (e *Editor) Save(path string) error { return track.Save(path, e.current.scene) }
