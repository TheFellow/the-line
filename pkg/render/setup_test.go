package render

import (
	"strings"
	"testing"

	"github.com/TheFellow/the-line/pkg/vehicle"
)

func TestSetupPagesExposeEachControlWithoutOverlap(t *testing.T) {
	base := fixture(t, "3d")
	for _, size := range [][2]int{{1440, 900}, {1000, 700}} {
		seen := map[string]bool{}
		for page := 0; page < SetupPages(); page++ {
			r, err := New(base.scene, base.result, base.vehicle, Options{Width: size[0], Height: size[1], View: "3d", Setup: true, SetupPage: page})
			if err != nil {
				t.Fatal(err)
			}
			controls := r.Controls()
			if controls["setup-page"].Empty() {
				t.Fatal("page navigation missing")
			}
			for key, rect := range controls {
				if !strings.HasPrefix(key, "setup:") {
					continue
				}
				if seen[key] {
					t.Fatalf("off-page control %s leaked", key)
				}
				seen[key] = true
				for other, otherRect := range controls {
					if other != key && rect.Overlaps(otherRect) {
						t.Fatalf("%s overlaps %s at %v", key, other, size)
					}
				}
			}
		}
		for _, f := range vehicle.SetupFields() {
			for _, suffix := range []string{"+", "-"} {
				if !seen["setup:"+f.Key+suffix] {
					t.Fatalf("missing %s%s", f.Key, suffix)
				}
			}
		}
	}
}
