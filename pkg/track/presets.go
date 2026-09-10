package track

import "fmt"

func Presets() []string { return []string{"hairpin", "esses", "compound", "banked", "rally"} }

// Preset returns independent illustrative geometry, not surveyed real-world tracks.
func Preset(name string) (Scene, error) {
	s := Scene{Version: Version, Vehicle: "club", EntrySpeed: 38, ExitSpeed: 55}
	var xy [][2]float64
	switch name {
	case "hairpin":
		s.Name = "The Switchback"
		xy = [][2]float64{{-120, 0}, {-55, 0}, {0, 0}, {28, 14}, {38, 40}, {28, 66}, {0, 80}, {-55, 80}, {-120, 80}}
	case "esses":
		s.Name = "Rhythm Section"
		xy = [][2]float64{{-135, -35}, {-90, -35}, {-50, -15}, {-20, 30}, {20, 40}, {55, 5}, {80, -35}, {120, -45}, {165, -20}}
	case "compound":
		s.Name = "Double Apex"
		xy = [][2]float64{{-135, -65}, {-80, -65}, {-30, -55}, {5, -30}, {12, 5}, {-5, 40}, {8, 75}, {45, 90}, {105, 90}}
	case "banked":
		s.Name = "Highline Sweep"
		xy = [][2]float64{{-135, -65}, {-85, -65}, {-35, -55}, {10, -30}, {40, 10}, {45, 55}, {25, 95}, {-15, 120}, {-75, 125}}
	case "rally":
		s.Name = "Ridge to River"
		s.Vehicle = "rally"
		s.EntrySpeed = 28
		s.ExitSpeed = 40
		xy = [][2]float64{{-145, -35}, {-100, -35}, {-60, -10}, {-25, 25}, {15, 25}, {50, -10}, {80, -35}, {115, -20}, {145, 20}, {190, 35}}
	default:
		return Scene{}, fmt.Errorf("track: unknown preset %q", name)
	}
	for i, v := range xy {
		p := Point{X: v[0], Y: v[1], Width: 12, Surface: "asphalt"}
		if name == "banked" {
			p.Bank = -12
			p.Z = float64(i) * 1.6
		}
		if name == "rally" {
			p.Width = 9
			p.Z = []float64{0, 3, 8, 13, 14, 10, 5, 1, 0, 2}[i]
			if i >= 3 && i < 7 {
				p.Surface = "gravel"
			}
			if i >= 7 {
				p.Surface = "dirt"
			}
			p.Bank = []float64{0, 0, -4, -5, 0, 4, 3, -3, -3, 0}[i]
		}
		s.Points = append(s.Points, p)
	}
	return s, nil
}
