package render

import (
	"math"
	"testing"
)

func TestWatchViewCannotOfferGeometryDragCoordinates(t *testing.T) {
	r := fixture(t, "perspective")
	p := r.scene.Points[len(r.scene.Points)/2].Position()
	x, y := r.Project(p)
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		t.Fatal("visible geometry has no finite perspective projection")
	}
	unprojected := r.Unproject(x, y, p.Z)
	if !math.IsNaN(unprojected.X) || !math.IsNaN(unprojected.Y) || !math.IsNaN(unprojected.Z) {
		t.Fatal("watch-only view exposed an orthographic inverse as a geometry drag coordinate")
	}
	for _, key := range []string{"add", "delete", "width+", "width-", "bank+", "bank-", "height+", "height-", "authoring", "previous", "next"} {
		if _, found := r.Controls()[key]; found {
			t.Fatalf("watch-only panel retained hidden geometry control %q", key)
		}
	}
	for _, key := range []string{"watch", "analysis", "frame-", "frame+", "station-", "station+", "chart", "play"} {
		if r.Controls()[key].Empty() {
			t.Fatalf("watch-only view lost study control %q", key)
		}
	}
}
