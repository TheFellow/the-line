package editor_test

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/TheFellow/the-line/internal/editor"
	"github.com/TheFellow/the-line/pkg/render"
	"github.com/TheFellow/the-line/pkg/solver"
	"github.com/TheFellow/the-line/pkg/track"
	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestPointerDragWithActualProjection(t *testing.T) {
	for _, view := range []string{"2d", "3d"} {
		for _, size := range [][2]int{{1440, 900}, {800, 600}} {
			t.Run(fmt.Sprintf("%s/%dx%d", view, size[0], size[1]), func(t *testing.T) {
				scene, _ := track.Preset("banked")
				road, err := track.SampleRoad(scene, 3)
				if err != nil {
					t.Fatal(err)
				}
				result := solver.Result{Road: road, Duration: 20, CenterDuration: 20}
				for _, s := range road {
					result.Nodes = append(result.Nodes, solver.Node{Position: s.Position, S: s.S, Time: s.S / 20, Speed: 20})
				}
				config, _ := vehicle.Preset(scene.Vehicle)
				r, err := render.New(scene, result, config, render.Options{Width: size[0], Height: size[1], View: view})
				if err != nil {
					t.Fatal(err)
				}
				ed, err := editor.New(scene)
				if err != nil {
					t.Fatal(err)
				}
				origin := scene.Points[3].Position()
				x, y := r.Project(origin)
				// Grab the handle off-centre, as a person clicking its label would do.
				drag := editor.BeginDrag(scene, r, x+6, y-4)
				if drag == nil || drag.Index != 3 {
					t.Fatalf("could not grab control: %+v", drag)
				}
				changed, err := drag.Commit(ed, r, x+6, y-4)
				if err != nil || changed || ed.CanUndo() {
					t.Fatal("click changed geometry or history")
				}
				target := origin.Add(track.Vec3{X: 4, Y: 3})
				endX, endY := r.Project(target)
				preview := drag.Position(r, endX+6, endY-4)
				if preview.Sub(target).Length() > 1e-8 {
					t.Fatalf("grab offset lost: %v versus %v", preview, target)
				}
				if !reflect.DeepEqual(ed.Scene(), scene) {
					t.Fatal("preview mutated scene")
				}
				changed, err = drag.Commit(ed, r, endX+6, endY-4)
				if err != nil || !changed {
					t.Fatalf("release failed: changed=%v err=%v", changed, err)
				}
				got := ed.Scene().Points[3].Position()
				if got.Sub(target).Length() > 1e-8 || math.Abs(got.Z-origin.Z) > 1e-8 {
					t.Fatalf("release=%v want %v", got, target)
				}
				if !ed.Undo() || !reflect.DeepEqual(ed.Scene(), scene) || ed.CanUndo() {
					t.Fatal("drag was not one undoable edit")
				}
				// A release onto the next control is rejected transactionally.
				badX, badY := r.Project(scene.Points[4].Position())
				if _, err = drag.Commit(ed, r, badX+6, badY-4); err == nil {
					t.Fatal("duplicate control accepted")
				}
				if !reflect.DeepEqual(ed.Scene(), scene) {
					t.Fatal("invalid drop changed scene")
				}
			})
		}
	}
}
