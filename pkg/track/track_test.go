package track

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPresetsAndSurfaceStations(t *testing.T) {
	for _, name := range Presets() {
		t.Run(name, func(t *testing.T) {
			scene, err := Preset(name)
			if err != nil {
				t.Fatal(err)
			}
			for _, spacing := range []float64{1, 2, 4} {
				samples, err := SampleRoad(scene, spacing)
				if err != nil {
					t.Fatal(err)
				}
				if samples[len(samples)-1].S < 200 {
					t.Fatal("preset too short")
				}
				for _, p := range scene.Points {
					found := false
					for _, s := range samples {
						if s.Position.Sub(p.Position()).Length() < 1e-6 && s.Surface == p.Surface {
							found = true
							break
						}
					}
					if !found {
						t.Fatalf("missing source transition %+v", p)
					}
				}
			}
		})
	}
}
func TestBankGeometry(t *testing.T) {
	s := Sample{Position: Vec3{Z: 3}, Normal: Vec3{Y: 1}, Bank: 15}
	p := s.AtOffset(4)
	if p.Y != 4 || math.Abs(p.Z-(3+4*math.Tan(math.Pi/12))) > 1e-12 {
		t.Fatalf("bank position %+v", p)
	}
}
func TestPersistence(t *testing.T) {
	scene, _ := Preset("rally")
	path := filepath.Join(t.TempDir(), "scene.json")
	if err := Save(path, scene); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, scene) {
		t.Fatal("round trip changed scene")
	}
	data, _ := os.ReadFile(path)
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["entry_speed_cap"]; !ok {
		t.Fatal("missing explicit cap key")
	}
	scene.Points[0].Width = 0
	if Save(path, scene) == nil {
		t.Fatal("accepted invalid overwrite")
	}
	if _, err := Load(path); err != nil {
		t.Fatal("failed save damaged prior file")
	}
}
func TestInvalidGeometry(t *testing.T) {
	base, _ := Preset("hairpin")
	cases := map[string]func(*Scene){"nan": func(s *Scene) { s.Points[0].X = math.NaN() }, "duplicate": func(s *Scene) { s.Points[1] = s.Points[0] }, "unknown surface": func(s *Scene) { s.Points[0].Surface = "moon" }, "fold": func(s *Scene) {
		for i := range s.Points {
			s.Points[i].Width = 80
		}
	}}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := base
			s.Points = append([]Point(nil), base.Points...)
			mutate(&s)
			if _, err := SampleRoad(s, 2); err == nil {
				t.Fatal("invalid geometry accepted")
			}
		})
	}
}

func TestSplineRejectsSelfIntersectionAndReversal(t *testing.T) {
	for name, xy := range map[string][][2]float64{
		"crossing":        {{-60, -40}, {60, 40}, {-60, 40}, {60, -40}},
		"reversal":        {{0, 0}, {50, 0}, {0, 0}},
		"short wide bend": {{0, 0}, {10, 0}, {11, 1}, {10, 10}},
	} {
		t.Run(name, func(t *testing.T) {
			s := Scene{Version: 1, Name: name}
			for _, p := range xy {
				s.Points = append(s.Points, Point{X: p[0], Y: p[1], Width: 12, Surface: "asphalt"})
			}
			for _, spacing := range []float64{.25, .5, 3} {
				if _, err := SampleRoad(s, spacing); err == nil {
					t.Fatalf("accepted invalid ribbon at %gm spacing", spacing)
				}
			}
		})
	}
}
