package track

import "fmt"

// These deliberately fictional studies isolate recognizable corner problems;
// their vertical geometry is not a claim of suspension or crest-unloading physics.
func archetype(name string) (Scene, error) {
	s := Scene{Version: Version, Vehicle: "gt", EntrySpeed: 40, ExitSpeed: 55, KerbsCountAsRoad: true}
	var xy [][2]float64
	var z, bank []float64
	switch name {
	case "decreasing-radius":
		s.Name = "Closing Spiral"
		xy = [][2]float64{{-180, -100}, {-100, -100}, {-20, -95}, {50, -70}, {90, -25}, {95, 20}, {70, 50}, {30, 55}, {-20, 55}}
	case "banked-bowl":
		s.Name = "Velodrome Bowl"
		xy = [][2]float64{{-150, -95}, {-80, -95}, {0, -85}, {65, -45}, {95, 20}, {75, 85}, {20, 125}, {-60, 135}, {-145, 135}}
		bank = []float64{0, -4, -10, -16, -16, -16, -10, -4, 0}
		z = []float64{8, 4, 0, -3, -4, -3, 0, 4, 8}
	case "compression":
		s.Name = "Valley Compression"
		xy = [][2]float64{{-180, -30}, {-115, -30}, {-50, -15}, {10, 20}, {65, 50}, {125, 55}, {190, 55}}
		z = []float64{18, 12, 2, -8, -5, 5, 14}
	case "chicane":
		s.Name = "Kerb Dance"
		xy = [][2]float64{{-180, -30}, {-110, -30}, {-60, -15}, {-15, 25}, {30, 25}, {75, -15}, {125, -30}, {190, -30}}
	case "long-double-apex":
		s.Name = "Patient Double Apex"
		xy = [][2]float64{{-180, -100}, {-105, -100}, {-30, -90}, {25, -55}, {45, 0}, {60, 65}, {100, 110}, {165, 125}, {230, 125}}
	case "blind-crest":
		s.Name = "Over the Brow"
		xy = [][2]float64{{-180, -50}, {-100, -50}, {-20, -45}, {50, -20}, {80, 30}, {55, 80}, {0, 105}, {-65, 105}}
		z = []float64{0, 8, 19, 16, 5, -3, -7, -8}
	default:
		return Scene{}, fmt.Errorf("track: unknown preset %q", name)
	}
	for i, v := range xy {
		p := Point{X: v[0], Y: v[1], WidthLeft: 6, WidthRight: 6, Surface: "asphalt", KerbLeft: Kerb{Width: 1.2, Surface: "wet", Grip: .85}, KerbRight: Kerb{Width: 1.2, Surface: "wet", Grip: .85}}
		if len(z) > 0 {
			p.Z = z[i]
		}
		if len(bank) > 0 {
			p.Bank = bank[i]
		}
		if name == "decreasing-radius" {
			p.WidthLeft = 5
			p.WidthRight = 7
		}
		s.Points = append(s.Points, p)
	}
	return s, nil
}
